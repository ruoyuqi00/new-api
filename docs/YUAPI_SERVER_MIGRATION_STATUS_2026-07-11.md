# YuAPI Server Migration Status - 2026-07-11

This document records the production migration from the previous server to the
new Debian server and contains no passwords, API keys, database credentials, or
Cloudflare secrets.

## Cutover State

The final database cutover completed on 2026-07-11.

New production entry points:

```text
https://yuaiapi.com
https://global.yuaiapi.com
https://api.yuaiapi.com
```

The new server runs Debian 12 with Docker Engine and Docker Compose Plugin.
The active production containers are:

```text
newapi          newapi:yucore-ui-20260711-18d07497   healthy, restart=0
newapi-mysql    mysql:8.4                            healthy, restart=0
newapi-redis    redis:7-alpine                       healthy, restart=0
yuapi-caddy     caddy:2-alpine                       running, restart=0
```

The application remains bound to `127.0.0.1:3001`; only Caddy exposes ports 80
and 443. MySQL and Redis are not exposed publicly.

## Data Verification

The final database was created after stopping the old application write path.
The old snapshot and restored database matched exactly at cutover:

```text
tables: 39
users: 12
channels: 28
tokens: 59
maximum log id: 7272
```

Core endpoint checks on the new server:

```text
/: 200
/api/status: 200
/api/pricing: 200
/v1/models without authentication: 401
```

A real ordinary-user browser check passed through the retained old domain:

```text
login: 200
/api/user/self: 200
/api/user/models: 200, 32 models
Studio: loaded with the main sidebar
Canvas: React Flow loaded with four initial nodes
```

A temporary user API token was created, used, and deleted. A real
`gpt-5.4-mini` request sent to `https://api.yuaiapi.com/v1/chat/completions`
returned HTTP 200 with a completion choice. This confirms the new unrestricted
model API hostname reaches the migrated channel pool.

## Domain And Region Policy

The new Caddy origin implements the intended hostname boundary:

```text
yuaiapi.com + CF-IPCountry: CN: 403
global.yuaiapi.com + CF-IPCountry: CN: 200
api.yuaiapi.com + CF-IPCountry: CN: 200
```

Cloudflare should use the same root-only custom rule:

```text
(http.host in {"yuaiapi.com" "www.yuaiapi.com"} and ip.src.country eq "CN")
```

The model API hostname must not receive an interactive challenge or a country
block.

## TLS And Edge

Caddy obtained a Let's Encrypt certificate for the new hostnames. Cloudflare
`Full (strict)` works and all three public `/api/status` endpoints return 200.

The old domain remains available. The old Caddy now proxies its NewAPI traffic
to `https://api.yuaiapi.com`, so existing clients continue to use the migrated
database while the previous NewAPI application container remains stopped.

Old Caddy rollback backup:

```text
/opt/sub2api/Caddyfile.before-yuaiapi-cutover-20260711190250
```

## Backups

Initial and final migration material is retained on both servers during the
observation window.

New server cold backups:

```text
/opt/cold-backups/20260711
```

This includes:

- final NewAPI MySQL dump and verification metadata;
- retained Sub2API PostgreSQL logical dump;
- retained Sub2API files and configuration;
- UAG/media configuration, MySQL dump, Redis data, logs, and image-site data.

Automated production backups run daily around 03:20 Asia/Shanghai:

```text
yuapi-backup.timer
/opt/newapi/backups/daily
```

The timer keeps seven days of MySQL and runtime configuration backups. The
first backup and SHA-256 verification completed successfully.

## Security Baseline

- Fail2ban is enabled for the non-default SSH port.
- The temporary server-to-server migration key was removed from both servers.
- Production database and Redis ports are not published.
- Temporary API test tokens were deleted.
- The user should now rotate both temporary server passwords.

## Turnstile Verification

The Cloudflare Turnstile site key and secret were replaced after the new
hostnames were added to the widget allowlist. Both the live and rollback
databases received the same configuration, and each server retained a root-only
backup of the previous option rows.

Real browser verification on `global.yuaiapi.com` passed:

```text
/api/status exposed the expected new site key: yes
Turnstile token received: yes
login: 200
/api/user/self: 200
/api/user/models: 200, 32 models
Studio: loaded with a 208px main sidebar
Canvas: React Flow loaded with four initial nodes
```

The only browser-side 401 was Cloudflare's optional Private Access Token probe;
it did not affect Turnstile verification or application authentication.

## Old Server Observation Window

Do not release or erase the old server yet.

Current rollback posture:

- old NewAPI application container: stopped;
- old NewAPI MySQL and Redis: still healthy and unchanged;
- old Caddy and auxiliary Sub2API/UAG services: still running;
- final migration dump: retained on the old server;
- old domain: proxies to the new production server.

Keep this state for at least 3 to 7 days. If rollback is required, first stop
new writes, restore the old Caddy backup, synchronize the newest database back
to the old server, and only then restart the old NewAPI application.
