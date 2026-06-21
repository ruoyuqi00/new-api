# NewAPI Domain And Email Cutover

Date: 2026-06-19

## Production State

- NewAPI remains the public/user-facing site.
- Sub2API remains the provider scheduling and bridge layer.
- NewAPI was not restarted for this change.
- Caddy was reloaded only after config validation passed.
- No secrets, SMTP tokens, API keys, or account payloads are recorded here.

## NewAPI Email Verification

Email verification is enabled in NewAPI.

Current non-secret SMTP settings:

- `EmailVerificationEnabled`: `true`
- `SMTPServer`: `smtp.qq.com`
- `SMTPPort`: `465`
- `SMTPSSLEnabled`: `true`
- `SMTPAccount`: configured
- `SMTPFrom`: same mailbox as `SMTPAccount`
- `SMTPToken`: configured, value redacted

Verification performed:

- NewAPI options were updated through the NewAPI root/admin option API, so the running process and database were updated without a restart.
- Server-to-`smtp.qq.com:465` TLS connectivity succeeded and the peer certificate verified.

If mail delivery still fails later, test by sending a real verification email to a controlled mailbox and then check QQ mailbox authorization, sender limits, and spam filtering.

## Domain Routing

The root domain `dtrljm.com` is live through Caddy and routes to NewAPI.

NewAPI public status verification:

- `https://dtrljm.com/api/status` returned HTTP 200.
- NewAPI status now reports `server_address=https://dtrljm.com`.
- NewAPI status now reports `logo=https://dtrljm.com/logo.png`.

### 2026-06-21 Logo And Intermittent 500 Check

- Replaced the public NewAPI logo/favicon served by `https://dtrljm.com/logo.png` and `https://dtrljm.com/favicon.ico`.
- Source assets were updated in the NewAPI working tree:
  - `web/default/public/logo.png`
  - `web/default/public/favicon.ico`
  - `web/classic/public/logo.png`
- Production hot patch:
  - uploaded the generated assets to `/opt/sub2api/caddy_config/static-brand/`;
  - added a Caddy static handler for `/logo.png` and `/favicon.ico` on NewAPI hosts;
  - reloaded only `sub2api-caddy`;
  - did not restart the NewAPI container.
- Verification:
  - `https://dtrljm.com/logo.png` SHA256: `51851A923E68F0109A99927F70D90812976C0FEEBA29B064F224675B91AF5888`;
  - `https://dtrljm.com/favicon.ico` SHA256: `345194F179F32F3C2CA895334C7F41541197CE8BC0609C3E7F2C415C3EBA9243`;
  - `https://dtrljm.com/api/status` returned HTTP 200 and still reports `logo=https://dtrljm.com/logo.png`;
  - `newapi`, `newapi-mysql`, and `newapi-redis` remained healthy.
- Intermittent 500 investigation:
  - Current public checks for `https://dtrljm.com/` and `https://dtrljm.com/api/status` returned HTTP 200.
  - NewAPI container logs after the latest restart did not show panic or 500 stack traces.
  - Caddy access logs around the observed admin/system-settings navigation showed HTTP 200 for the checked routes.
  - If 500 appears again, capture the exact URL/path and the visible time; use the `X-Oneapi-Request-Id` from the browser network panel when available.

Production Caddy config lives on the server at:

- `/opt/sub2api/Caddyfile`

The active Caddy container has this file bind-mounted as `/etc/caddy/Caddyfile`.

The NewAPI route matcher now includes:

- `dtrljm.com`
- `www.dtrljm.com`
- `api.dtrljm.com`
- `admin.dtrljm.com`
- `newapi.dtrljm.com`

The old Sub2API public route remains available on:

- `api.vyywcw.cn`
- `www.vyywcw.cn`

The old NewAPI route remains available on:

- `newapi.vyywcw.cn`

This preserves existing clients while the new domain is rolled out.

For the public rollout, use this split:

- user API Base URL: `https://api.dtrljm.com/v1`;
- web/admin hostnames that can be restricted by Cloudflare country rules:
  `dtrljm.com`, `www.dtrljm.com`, `admin.dtrljm.com`, and
  `newapi.dtrljm.com`.

Do not apply a broad mainland China block to the whole `dtrljm.com` zone,
because `api.dtrljm.com` must remain open for normal API clients. The detailed
access and recharge runbook is
`docs/PUBLIC_ACCESS_AND_RECHARGE_RUNBOOK_2026-06-21.md`.

## Cloudflare DNS And Certificate Status

The Cloudflare DNS records are now active for the root domain and the intended subdomains.

Verified public HTTPS status:

- `https://dtrljm.com/api/status` returned HTTP 200.
- `https://www.dtrljm.com/api/status` returned HTTP 200.
- `https://api.dtrljm.com/api/status` returned HTTP 200.
- `https://admin.dtrljm.com/api/status` returned HTTP 200.
- `https://newapi.dtrljm.com/api/status` returned HTTP 200.

Caddy obtained production Let's Encrypt certificates for:

- `api.dtrljm.com`
- `admin.dtrljm.com`
- `newapi.dtrljm.com`
- `www.dtrljm.com`

Operational note:

- While DNS was still propagating, Caddy's ACME attempts saw `NXDOMAIN` and backed off.
- After public resolvers returned all subdomains, only the Caddy container was restarted to clear the ACME backoff and trigger certificate issuance.
- NewAPI was not restarted.

For future Caddy config changes, use:

```bash
docker exec sub2api-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec sub2api-caddy caddy reload --config /etc/caddy/Caddyfile
```

## Passkey Caveat

NewAPI passkey/WebAuthn settings still report:

- `passkey_rp_id=newapi.vyywcw.cn`
- `passkey_origins=https://newapi.vyywcw.cn`

This was left unchanged intentionally. Passkeys are domain-bound, and switching the RP ID can invalidate existing passkey credentials. Password login remains available.

Before fully promoting `dtrljm.com` as the only login domain, decide whether to:

- keep passkey tied to the old NewAPI domain;
- migrate passkey to `dtrljm.com` and re-register admin/user passkeys;
- disable passkey login and rely on password plus email verification for now.

## Caddy Bind-Mount Note

The Caddy container bind-mounts a single Caddyfile. If the host file is replaced by rename, the running container can keep reading the old inode until the Caddy container is recreated.

For zero-restart changes, edit the currently bound file in place or run a Caddy-only recreate. The 2026-06-19 change used an in-place correction and then `caddy reload`.
