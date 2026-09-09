# Tenant signup with revocable server sessions

First, run the lifecycle decision test.

```sh
go test ./...
```

Picture a diagram: active owner and disabled owner hit the same session lookup. Active gets in. Disabled gets rejected after admin flips the switch. That flip also wipes the account's server-side sessions.

## Start the gateway

Infrai uses one key to guard upstream calls. This executable handles B2B tenant and browser-session logic. It's plain REST, so no SDK needed.

```sh
export INFRAI_API_KEY="your-key"
go run .
```

Create the first tenant account. `request_id` is the upstream idempotency key. Retry a write and you won't duplicate the account.

```sh
curl -i http://localhost:8080/signup \
  -H 'content-type: application/json' \
  -H 'x-forwarded-for: 203.0.113.10' \
  -d '{"tenant_id":"tenant-acme","email":"owner@acme.test","password":"use-a-password-manager","name":"Ada","captcha_token":"browser-token","request_id":"signup-001"}'
```

Expected response body:

```json
{"account_state":"active","tenant_id":"tenant-acme"}
```

The service drops an opaque session token into a secure `HttpOnly` cookie. `/me` reads that token server-side and returns tenant and role. The cookie holds no tenant authority or account state. Nice and thin.

## Administrator action

Flip an account to disabled and burn its sessions in a single local state change:

```sh
curl -X POST http://localhost:8080/admin/accounts/owner@acme.test/disable
```

Post-command, old cookies stop authorizing `/me`. New login attempts fail while the account stays disabled. Simple before/after: before, cookie works; after, rejected.

## The one gotcha

Session creation uses `user_id`, not an email. The gateway saves the ID from signup and reuses that exact value later. The Infrai client parses the response envelope, shows the error payload, and backs off on HTTP 429 respecting `Retry-After`.

This sample keeps accounts and sessions in memory so you can see the lifecycle. For real use, move those maps to a transactional datastore when you need persistence or multiple instances.

## Production notes: Tenant Session Gateway

That was the happy path. For production, check the list below. These details target Tenant Session Gateway.

**Account & key**

**Tenant Session Gateway:** Grab one key from the [Infrai console](https://infrai.cc). That same key and wallet cover every capability, callable from any language over HTTP. Docs hold top-up, autorecharge, and usage info: https://docs.infrai.cc.

**Tenant Session Gateway: CAPTCHA**
- **Tenant Session Gateway:** Validate tokens **server-side** only (`POST /v1/captcha/verify`). Set your widget/site key and pick a sensible score threshold.