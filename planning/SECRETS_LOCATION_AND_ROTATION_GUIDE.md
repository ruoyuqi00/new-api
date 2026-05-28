# Secrets Location and Rotation Guide

Date: 2026-05-28

This document intentionally does **not** store plaintext passwords, API keys, or
provider tokens. Even private Git repositories keep secrets forever in commit
history, so real values must stay in runtime secret stores only.

## Where real secrets live

Production server:

```text
/opt/sub2api/.env
```

Important values currently stored there:

```text
MAIL_RELAY_PROVIDER=resend
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
RESEND_API_KEY=<stored on server, starts with re_>
POSTGRES_PASSWORD=<stored on server>
REDIS_PASSWORD=<stored on server, may be empty>
WINDSURF_API_KEY=<stored on server>
WINDSURF_DASHBOARD_PASSWORD=<stored on server>
KIRO_API_KEY / KIRO_* secrets=<stored on server if present>
```

Sub2API database:

```text
settings.admin_api_key
settings.smtp_*
```

Current saved SMTP settings may still show the legacy QQ SMTP configuration
until production email is switched to the internal relay. The relay runtime
itself already has the Resend key in `/opt/sub2api/.env`.

## Safe inspection commands

Check whether a secret exists without printing it:

```bash
cd /opt/sub2api
grep -E '^(MAIL_RELAY_PROVIDER|MAIL_RELAY_FROM|MAIL_RELAY_FROM_NAME)=' .env
grep -E '^RESEND_API_KEY=' .env | sed 's/=.*/=<redacted>/'
```

Check Sub2API admin API key status without printing the key:

```bash
cd /opt/sub2api
docker compose exec -T postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT key, length(value), left(value, 10) FROM settings WHERE key = '\''admin_api_key'\'';"'
```

Check SMTP settings without exposing password:

```bash
cd /opt/sub2api
docker compose exec -T postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT key, CASE WHEN key = '\''smtp_password'\'' THEN '\''<redacted>'\'' ELSE value END FROM settings WHERE key LIKE '\''smtp_%'\'' ORDER BY key;"'
```

## Resend key

Provider: Resend

Dashboard location:

```text
Resend -> API Keys
```

If the key is lost, create a new API key in Resend, then update the server:

```bash
cd /opt/sub2api
cp .env .env.bak-resend-$(date +%Y%m%d-%H%M%S)
nano .env
docker compose up -d mail-relay
docker compose logs --tail=80 mail-relay
```

Expected env values:

```text
MAIL_RELAY_PROVIDER=resend
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
RESEND_API_KEY=<new re_... key>
```

## SSH credential

Do not commit the SSH password to Git.

Recommended storage:

- A password manager.
- A local-only file ignored by Git.
- A server access vault.

Rotation path:

```bash
passwd root
```

After rotation, update the password manager or local-only note.

## Local-only notes

If a plaintext note is absolutely needed on the development machine, put it in a
file ignored by Git, for example:

```text
D:\wflogin\sub2api-private\.codex\local-secrets-note.txt
```

The repository `.gitignore` already ignores `.codex/`. Do not `git add -f` this
file.

## Current production checks

As of 2026-05-28:

```text
Sub2API image: sub2api-provider-adapters:6611e027
mail-relay image: sub2api-mail-relay:5e5d524a
mail-relay provider: resend
public health: https://api.vyywcw.cn/health -> 200
```

The Resend API key is installed on the server, but real sending still depends
on Resend domain verification for `vyywcw.cn`.

Relay health:

```bash
cd /opt/sub2api
docker compose ps mail-relay
docker compose exec -T mail-relay python3 -c 'import urllib.request; print(urllib.request.urlopen("http://127.0.0.1:8080/health", timeout=5).read().decode())'
```

No public relay port should be exposed:

```bash
docker compose port mail-relay 1025 || true
docker compose port mail-relay 8080 || true
```
