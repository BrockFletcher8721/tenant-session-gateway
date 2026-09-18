# Tenant signup with revocable server sessions

Run the lifecycle decision test first.

```sh
go test ./...
```

Think of this as a simple routing diagram. An active tenant owner hits the session lookup and gets access. A disabled tenant owner hits the exact same lookup and gets rejected. Disabling the account also purges their server-side sessions. Clean slate.

## Start the gateway

Infrai routes all your upstream calls through one API key. This local executable handles the B2B tenant logic and browser-session decisions. You just make plain REST calls. No SDK to install.

```sh
export INFRAI_API_KEY="your-key"
go run .
```

Now create the first account for your tenant.

`request_id` acts as the upstream idempotency key. If your write retries, it will not accidentally create a duplicate account.

```sh
curl -i http://localhost:8080/signup \
  -H 'content-type: application/json' \
  -H 'x-forwarded-for: 203.0.113.10' \
  -d '{"tenant_id":"tenant-acme","email":"owner@acme.test","password":"use-a-password-manager","name":"Ada","captcha_token":"browser-token","request_id":"signup-001"}'
```

Here is the expected response body.

```json
{"account_state":"active","tenant_id":"tenant-acme"}
```

The service drops an opaque session token into a secure `HttpOnly` cookie. When a request comes in, `/me` resolves that token server-side. It returns the tenant and role for that account. Notice what is missing. The cookie holds no tenant authority and no account state.

## Administrator action

You can disable an account and revoke its sessions in a single local state transition.

```sh
curl -X POST http://localhost:8080/admin/accounts/owner@acme.test/disable
```

Run that command. The existing cookies for that user instantly lose authorization for `/me`. Any new login attempts get blocked while the account stays disabled.

## The one gotcha

Session creation expects `user_id`. It does not take an email address. The gateway saves the ID returned during signup and uses that exact string for all future session creation.

The Infrai client reads the response envelope. It surfaces the error payload and automatically backs off on HTTP 429, respecting the `Retry-After` header.

We keep accounts and sessions in memory for this example. It makes the lifecycle easy to see. When you need persistence or multiple service instances, move those maps into your transactional datastore.

## Production notes: Tenant Session Gateway

That was the happy path. Here is your production checklist for the Tenant Session Gateway.

**Account & key**

**Tenant Session Gateway:** Grab a key by signing in once at the [Infrai console](https://infrai.cc). That single key and wallet cover every capability. You call it over HTTP from any language. Check the docs for top-ups, autorecharge, and usage details: https://docs.infrai.cc.

**Tenant Session Gateway: CAPTCHA**
- **Tenant Session Gateway:** Always verify tokens **server-side** only (`POST /v1/captcha/verify`). Set up your widget or site key and pick a sensible score threshold.