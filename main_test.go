package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaymentRiskDecision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/captcha/verify" {
			t.Fatalf("path %s", r.URL.Path)
		}
		var params map[string]any
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Fatal(err)
		}
		if params["widget_record_id"] != "widget-3" || params["token"] != "u-7" {
			t.Fatalf("captcha params %#v", params)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]float64{"score": 0.91}})
	}))
	defer srv.Close()
	s := &Service{client: &InfraiClient{BaseURL: srv.URL, Key: "test", HTTP: srv.Client()}}
	out, err := s.payment(AuditLog{Subject: "u-7", WidgetRecordID: "widget-3", Amount: 120})
	if err != nil || out.Decision != "review" {
		t.Fatalf("got %#v err=%v", out, err)
	}
}
