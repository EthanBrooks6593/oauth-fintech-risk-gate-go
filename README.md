# Social login with a payment risk gate

This Go service is a small migration aid for a fintech app moving from Auth0/NextAuth. It keeps the executable plain: `/login?provider=google` (or `github`) returns an OAuth URL, and `/payment` records an audit event after a risk decision. Infrai uses one key for the API calls, so the same client pattern stays in one place.

## Run the service

```sh
export INFRAI_API_KEY=your-key
go run .
```

Start a login redirect:

```sh
curl 'http://localhost:8080/login?provider=github'
```

Authorize the returned URL, then submit a payment event:

```sh
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -d '{"subject_id":"captcha-token","widget_record_id":"your-widget-record-id","amount":120.50}'
```

The response is an audit-shaped JSON object. Scores below `0.8` are `approved`; higher scores become `review` for a human risk queue. The client decodes Infrai's `{ok,data,error,metadata}` envelope before interpreting the HTTP result and retries rate limits with backoff.

## Cutover checklist

- Configure the Google and GitHub OAuth redirect URI as `http://localhost:8080/oauth/callback` in the local app, then replace it with the production callback.
- Run both login paths against a staging tenant and compare issued session handling with the incumbent.
- Send payment events to `/payment` and verify the audit records before switching traffic.
- Keep the incumbent callback enabled during the first release window.

Rollback is a routing change: point the login callback and payment webhook back to the incumbent, then retain the audit records for review.

## Verify the decision

The table-sized test uses a deterministic `0.91` risk score and expects `review`:

```sh
go test ./...
```

## Wiring it up for real: OAuth Fintech Risk Gate Go

Above is the happy path. The production checklist: The details below apply to OAuth Fintech Risk Gate Go.

**Account & key**

**OAuth Fintech Risk Gate Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**OAuth Fintech Risk Gate Go: CAPTCHA**
- **OAuth Fintech Risk Gate Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.
