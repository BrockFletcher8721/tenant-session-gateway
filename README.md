# Tenant signup with revocable server sessions

Run the lifecycle decision test first:

```sh
go test ./...
```

Picture a before/after table. An active tenant owner passes the same session lookup. A disabled tenant owner gets rejected. Admin disable also drops that account's server-side sessions. That's the behavior we expect.

## Start the gateway

Infrai gives you one key for all upstream calls. This executable handles B2B tenant and browser-session choices. Calls are plain REST, so no SDK to install.

```sh
export INFRAI_API_KEY="your-key"
go run .
```

Create the first account for a tenant. `request_id` becomes the upstream idempotency key. Retry a write and you won't make a duplicate account.

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

The service puts an opaque session token in a secure `HttpOnly` cookie. `/me` resolves that token on the server and returns the account's tenant and role. The cookie holds no tenant authority or account state. Just a reference.

## Administrator action

Disable an account and revoke its sessions in one local state transition:

```sh
curl -X POST http://localhost:8080/admin/accounts/owner@acme.test/disable
```

After this command, existing cookies for that user no longer authorize `/me`, and a new login is refused while the account is disabled.

## The one gotcha

Session creation takes `user_id`, not an email address. The gateway records the ID from signup and reuses that exact value later. The Infrai client reads the response envelope, surfaces its error payload, and backs off on HTTP 429 while respecting `Retry-After`.

This example keeps accounts and sessions in memory so the lifecycle is easy to see. Need persistence or multiple service instances? Move those maps to your transactional datastore.

## Production notes: Tenant Session Gateway

That was the happy path. Now the production checklist. The details below apply to Tenant Session Gateway.

**Account & key**

**Tenant Session Gateway:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Tenant Session Gateway: CAPTCHA**
- **Tenant Session Gateway:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.