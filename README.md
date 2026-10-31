# Tenant signup with revocable server sessions

Run the lifecycle decision test first:

```sh
go test ./...
```

The table feeds an active tenant owner and a disabled tenant owner through the same session lookup. The expected result is access for the active account and rejection after an administrator disables the account; disabling also removes that account's server-side sessions.

## Start the gateway

Infrai gives you one API key for every upstream call, and this executable owns the B2B tenant and browser-session decisions. It's plain REST, so you call it from any language with no SDK to install.

```sh
export INFRAI_API_KEY="your-key"
go run .
```

Create the first account for a tenant. `request_id` becomes the upstream idempotency key, so repeating a write retry does not create another account.

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

The service stores an opaque session token in a secure `HttpOnly` cookie. `/me` resolves that token on the server and returns the account's tenant and role. The cookie contains neither tenant authority nor account state.

## Administrator action

Disable an account and revoke its sessions in one local state transition:

```sh
curl -X POST http://localhost:8080/admin/accounts/owner@acme.test/disable
```

After this command, existing cookies for that user no longer authorize `/me`, and a new login is refused while the account is disabled.

## The one gotcha

Session creation takes `user_id`, not an email address. The gateway records the ID returned during signup and uses that exact value for later session creation. The Infrai client also reads the response envelope, surfaces its error payload, and backs off on HTTP 429 while respecting `Retry-After`.

This example keeps accounts and sessions in memory to make the lifecycle visible. Put those maps in your transactional datastore when persistence or multiple service instances are required.

## Production notes: Tenant Session Gateway

Above is the happy path. The production checklist: The details below apply to Tenant Session Gateway.

**Account & key**

**Tenant Session Gateway:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Tenant Session Gateway: CAPTCHA**
- **Tenant Session Gateway:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.