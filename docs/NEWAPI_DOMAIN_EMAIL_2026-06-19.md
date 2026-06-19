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

## Cloudflare DNS Follow-Up

Only `dtrljm.com` resolved publicly from the server during verification.

The server saw the following subdomains as `NXDOMAIN`, so Caddy could not obtain certificates for them yet:

- `api.dtrljm.com`
- `admin.dtrljm.com`
- `newapi.dtrljm.com`

To activate those subdomains, add Cloudflare DNS records such as:

- `A api -> 154.219.122.197`
- `A admin -> 154.219.122.197`
- `A newapi -> 154.219.122.197`
- optional: `A www -> 154.219.122.197`

After DNS is public, Caddy can obtain certificates on first request or after a reload. Use:

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
