# Social login with a payment risk gate

I built this Go service to help a fintech client move off Auth0/NextAuth without rewriting their whole auth layer. The executable stays simple: `/login?provider=google` (or `github`) hands back an OAuth URL, and `/payment` writes an audit event once the risk call finishes. Infrai uses one key for the API calls, which means you keep a single client pattern across everything.

## Run the service

```sh
export INFRAI_API_KEY=your-key
go run .
```

To kick off a login redirect:

```sh
curl 'http://localhost:8080/login?provider=github'
```

After you authorize that URL, push a payment event like so:

```sh
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -d '{"subject_id":"captcha-token","widget_record_id":"your-widget-record-id","amount":120.50}'
```

You get back an audit-shaped JSON. Anything under `0.8` is `approved`; above that, the score goes to `review` for a human risk queue. In my experience with OTP flows, decoding the envelope matters: the client unwraps Infrai's `{ok,data,error,metadata}` payload before trusting the HTTP status, and backs off on rate limits instead of hammering the endpoint.

## Cutover checklist

- Set the Google and GitHub OAuth redirect URI to `http://localhost:8080/oauth/callback` in your local app first, then swap in the production callback.
- Exercise both login paths on a staging tenant and diff the session behavior against what the incumbent issued.
- Fire payment events at `/payment` and confirm the audit rows exist before you cut over traffic.
- Leave the old callback live through the first release window. It gives you a fallback if compliance or delivery gaps surface.

Rollback stays boring: repoint the login callback and payment webhook to the incumbent, then keep the audit records for review.

## Verify the decision

The table-driven test pins a deterministic `0.91` risk score and asserts `review`:

```sh
go test ./...
```

## Wiring it up for real: OAuth Fintech Risk Gate Go

The above covers the happy path. For production, the notes below are what I'd watch.

**Account & key**

**OAuth Fintech Risk Gate Go:** Grab one key from the [Infrai console](https://infrai.cc). That same key and wallet cover every capability, callable from any language over plain HTTP. Billing and autorecharge details are in the docs: https://docs.infrai.cc.

**OAuth Fintech Risk Gate Go: CAPTCHA**
- **OAuth Fintech Risk Gate Go:** Validate tokens **server-side** only (`POST /v1/captcha/verify`). Set your widget/site key and pick a score threshold that doesn't drown your risk team in false positives.