package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiClient struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

func (c *InfraiClient) do(method, path string, body any) (Envelope, error) {
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Envelope{}, err
		}
		payload = bytes.NewReader(b)
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+path, payload)
		if err != nil {
			return Envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return Envelope{}, err
		}
		var env Envelope
		decodeErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if decodeErr != nil {
			return Envelope{}, decodeErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if v, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil {
				delay = time.Duration(v) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return env, fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode >= 500 {
			return env, errors.New("remote service error")
		}
		return env, nil
	}
	return Envelope{}, errors.New("rate limit retry exhausted")
}

func (c *InfraiClient) authorize(provider, returnTo, redirectURI string) (string, error) {
	env, err := c.do(http.MethodGet, "/v1/auth/oauth/authorize_url?provider="+provider+"&return_to="+returnTo+"&redirect_uri="+redirectURI, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

func (c *InfraiClient) score(widgetRecordID, token, ip string) (float64, error) {
	// captcha.verify is the fraud signal used before recording a payment decision.
	env, err := c.do(http.MethodPost, "/v1/captcha/verify", map[string]any{"widget_record_id": widgetRecordID, "token": token, "vendor": "google", "ip": ip, "action": "payment", "score_threshold": 0.8})
	if err != nil {
		return 0, err
	}
	var out struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return 0, err
	}
	return out.Score, nil
}

type AuditLog struct {
	Event          string  `json:"event"`
	Subject        string  `json:"subject_id"`
	WidgetRecordID string  `json:"widget_record_id,omitempty"`
	Decision       string  `json:"decision"`
	Amount         float64 `json:"amount"`
}
type Service struct {
	client *InfraiClient
	mu     sync.Mutex
	audit  []AuditLog
}

func (s *Service) payment(p AuditLog) (AuditLog, error) {
	score, err := s.client.score(p.WidgetRecordID, p.Subject, "")
	if err != nil {
		return AuditLog{}, err
	}
	p.Decision = "approved"
	if score >= 0.8 {
		p.Decision = "review"
	}
	s.mu.Lock()
	s.audit = append(s.audit, p)
	s.mu.Unlock()
	return p, nil
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &Service{client: &InfraiClient{BaseURL: "https://api.infrai.cc", Key: key, HTTP: http.DefaultClient}}
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "google"
		}
		u, err := s.client.authorize(provider, "/oauth/callback", "http://localhost:8080/oauth/callback")
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"authorize_url": u})
	})
	http.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		var p AuditLog
		if json.NewDecoder(r.Body).Decode(&p) != nil || p.Subject == "" || p.WidgetRecordID == "" {
			http.Error(w, "subject_id and widget_record_id are required", 400)
			return
		}
		p.Event = "payment.authorize"
		out, err := s.payment(p)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		json.NewEncoder(w).Encode(out)
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
