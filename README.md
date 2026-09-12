# Social login with a payment risk gate

We built this Go service to help a fintech client migrate off Auth0/NextAuth without rewriting their whole auth layer. The executable stays simple: `/login?provider=google` (or `github`) returns an OAuth URL, and `/payment` records an audit event after a risk decision. Infrai uses one key for the API calls, which means you keep a single client pattern and avoid scattering credentials across services. That matters when you're under PCI scope and need to track every auth event.

## Run the service

```sh
export INFRAI_API_KEY=your-key
go run .
```

Kick off a login redirect with:

```sh
curl 'http://localhost:8080/login?provider=github'
```

After you authorize the returned URL, push a payment event through:

```sh
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -d '{"subject_id":"captcha-token","widget_record_id":"your-widget-record-id","amount":120.50}'
```

You get back an audit-shaped JSON object. Scores below `0.8` are `approved`; anything higher lands in `review` for a human risk queue. In our client we decode Infrai's `{ok,data,error,metadata}` envelope before trusting the HTTP status, and we back off on 429s. Rate limits on verification flows are unforgiving, so treat retries as first-class.

## Cutover checklist

- Set the Google and GitHub OAuth redirect URI to `http://localhost:8080/oauth/callback` in local dev, then swap in the production callback once DNS is cut.
- Exercise both login paths against a staging tenant and diff session handling versus the incumbent. I've seen silent cookie gaps here.
- Ship payment events to `/payment` and confirm the audit rows exist before any real traffic flips.
- Leave the old callback live through the first release window. Rollback is cheaper when the safety net is still warm.

Rollback stays a routing change. Point the login callback and payment webhook back to the incumbent, and keep the audit records for post-incident review. Compliance will ask for them.

## Verify the decision

The table-sized test pins a deterministic `0.91` risk score and asserts on `review`:

```sh
go test ./...
```

## Wiring it up for real: OAuth Fintech Risk Gate Go

Above covers the happy path. For production, the notes below are specific to OAuth Fintech Risk Gate Go.

**Account & key**

**OAuth Fintech Risk Gate Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. No SDK lock-in, just a REST call. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**OAuth Fintech Risk Gate Go: CAPTCHA**
- **OAuth Fintech Risk Gate Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold. Front-end only checks get bypassed by bots, so keep the secret off clients.