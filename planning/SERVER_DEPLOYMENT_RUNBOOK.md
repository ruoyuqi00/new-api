# Server Deployment Runbook

## 2026-05-19 Kiro/Windsurf external verification

Current internal adapter images on the US server:

- Sub2API fork: `sub2api-provider-adapters:a147caa0`
- WindsurfAPI: `ghcr.io/dwgx/windsurf-api:latest`
- Kiro Gateway: `ghcr.io/jwadow/kiro-gateway:latest`
- patched kiro.rs: `kiro-rs-admin-metadata:20260519-1425`

External smoke through `https://api.vyywcw.cn/v1/messages` passed:

- `claude-sonnet-4.6` through Windsurf: HTTP 200, text `ok`
- `qwen3-coder-next` through Kiro Gateway: HTTP 200, text `ok`
- `deepseek-3.2` through Kiro Gateway: HTTP 200, text `ok`

The current adapter credential status checks report:

- Windsurf: 1 account, 1 active, tier `pro`.
- kiro.rs: 1 credential, 1 available, 1 credential with `profileArn`.

The server `.env` `ADMIN_PASSWORD` is stale and returns 401 for CLI login.
Browser admin operations should use the current remembered password. Do not
store the current admin password in this repo.

See `planning/PROVIDER_ACCOUNT_OPERATIONS_GUIDE.md` for the admin UI account
import workflow.

## 2026-05-19 Current Kiro Gateway runtime

The server now has two Kiro-related internal services:

- `kiro-rs`: kept as the credential/admin import reference and Rust proxy experiment.
- `kiro-gateway`: active runtime adapter for the Kiro models that passed smoke.

Only Sub2API is public. Do not publish `kiro-gateway:8000` or `kiro-rs:8990`
to the host, and do not add Caddy routes for either service.

Current active Kiro runtime account in Sub2API:

```text
name: kiro-gateway-internal-anthropic
platform: anthropic
type: apikey
base_url: http://kiro-gateway:8000
group: windsurf-smoke
```

Currently enabled Kiro model mapping:

```text
deepseek-3.2
glm-5
minimax-m2.5
qwen3-coder-next
```

Safe internal Kiro Gateway checks:

```bash
cd /opt/sub2api
docker compose ps kiro-gateway
docker compose logs --tail=100 kiro-gateway

set -a
. ./.env
set +a

docker run --rm --network sub2api_sub2api-network curlimages/curl:8.16.0 \
  -sS -m 20 \
  -H "Authorization: Bearer $KIRO_API_KEY" \
  http://kiro-gateway:8000/v1/models
```

Safe public Sub2API Kiro smoke:

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H "authorization: Bearer <sub2api-test-key>" \
  -H "content-type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "deepseek-3.2",
    "max_tokens": 32,
    "messages": [{"role": "user", "content": "reply with ok"}],
    "stream": false
  }'
```

Do not add Claude-family Kiro models to Sub2API mapping until direct and public
smoke both pass. As of 2026-05-19, Claude-family Kiro requests still fail with
model/subscription errors even though the credential plan reports `KIRO PRO`.

Current deployed Sub2API fork image:

```text
sub2api-provider-adapters:a147caa0
```

Rollback image:

```text
sub2api-provider-adapters:c4cefc76
```

Latest recorded rollback files:

```text
/opt/sub2api-backups/sub2api-20260519-123646.tar.gz
/opt/sub2api-backups/docker-compose-20260519-044206-pre-a147caa0.yml
```

记录日期：2026-05-16

## 2026-05-20 Provider adapter admin entry

Current deployed Sub2API fork image:

```text
sub2api-provider-adapters:70c970c7
```

The Sub2API admin UI now has a provider adapter page:

```text
https://api.vyywcw.cn/admin/provider-adapters
```

It shows the sanitized Sub2API admin proxy status for Windsurf/Kiro and links to
the native adapter admin UIs:

```text
https://api.vyywcw.cn/windsurf-dashboard
https://api.vyywcw.cn/kiro-admin
```

The native UIs are same-domain Caddy path proxies. Adapter container ports are
still not host-published:

```text
/sub2api-windsurf-api {"3003/tcp":null}
/sub2api-kiro-rs {"8990/tcp":null}
/sub2api-kiro-gateway {"8000/tcp":null}
```

Unauthenticated public checks should remain protected:

```text
https://api.vyywcw.cn/api/v1/admin/provider-adapters/windsurf/accounts -> 401
https://api.vyywcw.cn/dashboard/api/accounts -> 401
https://api.vyywcw.cn/api/admin/credentials -> 401
```

Server-side admin proxy smoke with the Sub2API admin API key returned:

```text
/api/v1/admin/provider-adapters/windsurf/accounts=200
/api/v1/admin/provider-adapters/kiro/credentials=200
```

## Server metadata

Current production target:

- Server IP: `154.219.122.197`
- User: `root`
- OS assumption: Ubuntu 22.x
- Domain: `vyywcw.cn`
- Public Sub2API URLs:
  - `https://api.vyywcw.cn/`
  - `https://www.vyywcw.cn/`
- Current deployment path: `/opt/sub2api`

Do not store SSH password, provider tokens, API keys, or `.env` secrets in this repo.

## Current desired topology

```text
Internet
  -> Caddy :80/:443
  -> sub2api container
  -> internal Docker network
      -> postgres
      -> redis
      -> windsurf-api
      -> kiro-rs
```

Only Caddy/Sub2API is public.

## Pre-change checklist

Run before any server change:

```bash
cd /opt/sub2api
docker compose ps
docker compose config >/tmp/sub2api-compose.rendered.yml
./backup.sh
date -u
df -h
free -h
docker system df
```

Record:

- Current compose file checksum.
- Current image tags.
- Current Sub2API container ID.
- Current DB backup path.
- Current git/image commit if using custom image.

## Secrets checklist

Generate secrets on server, not in Git:

```bash
openssl rand -base64 36
```

Required secrets:

- `WINDSURF_API_KEY`
- `WINDSURF_DASHBOARD_PASSWORD`
- `KIRO_PROXY_API_KEY`
- `KIRO_ADMIN_PASSWORD` if the proxy supports admin password

Store in server `.env` or existing secret mechanism:

```env
WINDSURF_API_KEY=<secret>
WINDSURF_DASHBOARD_PASSWORD=<secret>
KIRO_PROXY_API_KEY=<secret>
```

Never commit this file.

## Add WindsurfAPI service

Server steps:

```bash
cd /opt/sub2api
mkdir -p windsurf-api/data windsurf-api/opt/windsurf windsurf-api/tmp/windsurf-workspace
chmod 700 windsurf-api/data
```

Add compose service using `WINDSURF_INTEGRATION_SPEC.md` as source.

Validate compose:

```bash
docker compose config
```

Start only the new service first:

```bash
docker compose up -d windsurf-api
docker compose logs --tail=200 windsurf-api
docker compose ps windsurf-api
```

Health:

```bash
curl -sS http://127.0.0.1:3003/health
```

If no host port is published, run from inside Docker network:

```bash
docker compose exec sub2api sh -lc 'wget -qO- http://windsurf-api:3003/health || true'
```

## Add Kiro proxy service

This depends on final selected image/build. Draft steps:

```bash
cd /opt/sub2api
mkdir -p kiro-rs/data
chmod 700 kiro-rs/data
```

Add service:

```yaml
  kiro-rs:
    image: <selected-kiro-image-or-local-build>
    restart: unless-stopped
    environment:
      API_KEY: ${KIRO_PROXY_API_KEY}
    volumes:
      - ./kiro-rs/data:/data
    networks:
      - default
```

Validate and start:

```bash
docker compose config
docker compose up -d kiro-rs
docker compose logs --tail=200 kiro-rs
```

## Caddy exposure checklist

Caddy must keep raw adapter ports private. The current production exception is
path-level same-domain admin UI proxying for Windsurf and Kiro, with each native
admin API still protected by its own password/API key.

Check:

```bash
cd /opt/sub2api
grep -R "windsurf\\|kiro\\|3003\\|8990" -n Caddyfile . || true
docker compose port windsurf-api 3003 || true
docker compose port kiro-rs 8990 || true
```

Expected:

- Caddy may show `/windsurf-dashboard*`, `/dashboard/api/*`, `/dashboard/i18n/*`,
  `/dashboard/data/*`, `/kiro-admin*`, `/admin/assets/*`, `/admin/vite.svg`, and
  `/api/admin/*`.
- No public host port mapping for internal adapters, unless bound to `127.0.0.1`
  for short-lived debugging.
- Public unauthenticated `https://api.vyywcw.cn/dashboard/api/accounts` returns
  `401`.
- Public unauthenticated `https://api.vyywcw.cn/api/admin/credentials` returns
  `401`.

## Import Windsurf accounts

After the custom Sub2API fork is deployed, prefer importing through Sub2API so the public/admin workflow stays consistent:

```bash
curl -sS http://127.0.0.1:8080/api/v1/admin/accounts/import/windsurf \
  -H "content-type: application/json" \
  -H "idempotency-key: windsurf-import-$(date +%Y%m%d%H%M%S)" \
  -H "authorization: Bearer <sub2api-admin-token>" \
  --data-binary @/root/windsurf_import.json
```

`/root/windsurf_import.json` can contain tokens or email/password lines:

```json
{
  "raw": "token-1\ntoken-2\nuser-a@example.com----password-a\nuser-b@example.com----password-b"
}
```

Sub2API container env required for that endpoint:

```env
WINDSURF_ADAPTER_INTERNAL_BASE_URL=http://windsurf-api:3003
WINDSURF_ADAPTER_INTERNAL_API_KEY=<WINDSURF_API_KEY>
WINDSURF_ADAPTER_TIMEOUT_SECONDS=30
```

Before the custom fork is deployed, import directly into WindsurfAPI from the server shell:

Use a temporary file:

```bash
umask 077
cat >/root/windsurf_tokens.tmp <<'EOF'
token-1
token-2
EOF
```

Convert to JSON without printing secrets:

```bash
python3 - <<'PY' >/root/windsurf_import.json
import json
tokens = [line.strip() for line in open('/root/windsurf_tokens.tmp') if line.strip()]
print(json.dumps({"accounts": [{"token": t} for t in tokens]}))
PY
```

Post:

```bash
curl -sS http://127.0.0.1:3003/auth/login \
  -H "content-type: application/json" \
  -H "authorization: Bearer $WINDSURF_API_KEY" \
  --data-binary @/root/windsurf_import.json
```

Cleanup:

```bash
shred -u /root/windsurf_tokens.tmp /root/windsurf_import.json 2>/dev/null || rm -f /root/windsurf_tokens.tmp /root/windsurf_import.json
```

## Create Sub2API upstream account

Preferred manual path:

1. Open Sub2API admin UI.
2. Create Anthropic API Key account.
3. Set API key to internal WindsurfAPI key.
4. Set base URL to `http://windsurf-api:3003`.
5. Enable passthrough only if direct test requires it.
6. Add model mapping.
7. Run account test.

If API is used later, use `/api/v1/admin/accounts` after confirming admin auth and request schema.

## URL allowlist risk

Sub2API has URL allowlist/security configuration. If account test rejects Docker service hosts, add explicit internal hosts to server config:

```yaml
security:
  url_allowlist:
    upstream_hosts:
      - api.anthropic.com
      - api.openai.com
      - windsurf-api
      - kiro-rs
      - 127.0.0.1
```

Exact config path must be verified against current server `config.yaml`.

## Smoke tests after deployment

Public:

```bash
curl -I https://api.vyywcw.cn/
curl -I https://www.vyywcw.cn/
```

Internal:

```bash
cd /opt/sub2api
docker compose ps
docker compose logs --tail=100 sub2api
docker compose logs --tail=100 windsurf-api
docker compose logs --tail=100 kiro-rs
```

Sub2API request:

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H 'content-type: application/json' \
  -H 'x-api-key: <public-sub2api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hi"}],
    "stream": false
  }'
```

## Rollback

If internal adapter fails:

```bash
cd /opt/sub2api
docker compose stop windsurf-api
docker compose stop kiro-rs
```

Then disable the corresponding Sub2API account in admin UI.

If compose change breaks stack:

```bash
cd /opt/sub2api
cp <backup-compose-file> docker-compose.yml
docker compose config
docker compose up -d
docker compose ps
```

If custom Sub2API image breaks:

```bash
cd /opt/sub2api
# edit compose back to previous image tag
docker compose up -d sub2api
docker compose logs --tail=200 sub2api
```

## Update policy

WindsurfAPI:

```bash
cd /opt/sub2api
docker compose pull windsurf-api
docker compose up -d windsurf-api
docker compose logs --tail=200 windsurf-api
```

Sub2API official sync in local fork:

```powershell
cd D:\wflogin\sub2api-private
git fetch upstream main
git log --oneline HEAD..upstream/main
git merge upstream/main
git push
```

Custom Sub2API image deployment:

1. Build image from private repo.
2. Push image or build on server.
3. Backup server.
4. Replace image tag.
5. Start one service.
6. Smoke test.
7. Keep previous tag for rollback.

## Incident checklist

If public API fails:

- [ ] Check Caddy logs.
- [ ] Check Sub2API logs.
- [ ] Check DB/Redis health.
- [ ] Disable newly added upstream accounts.
- [ ] Stop internal adapter services.
- [ ] Re-test public API with known-good account.

If Windsurf/Kiro route fails only:

- [ ] Direct curl internal proxy.
- [ ] Check account count.
- [ ] Check token invalid/expired.
- [ ] Check model name mapping.
- [ ] Check rate-limit/cooldown.
- [ ] Check Sub2API account test output.

## Current Windsurf deployment notes

As of 2026-05-18, the server is using the private fork image
`sub2api-provider-adapters:544f553b` and an internal `windsurf-api` service from
`ghcr.io/dwgx/windsurf-api:latest`.

Only Sub2API is publicly exposed. `windsurf-api` must stay internal:

```bash
cd /opt/sub2api
docker compose port windsurf-api 3003
grep -R "windsurf-api\|3003" -n Caddyfile . || true
```

The first command should not show a public host port. The second command should
not show a Caddy route that exposes WindsurfAPI.

### Safe internal Windsurf smoke

Run from the server. Do not print real API keys.

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

docker compose exec -T -e WKEY="$WINDSURF_API_KEY" windsurf-api node - <<'JS'
const payload = {
  model: 'claude-sonnet-4.6',
  max_tokens: 32,
  messages: [{ role: 'user', content: 'reply with ok' }],
};
const res = await fetch('http://127.0.0.1:3003/v1/messages', {
  method: 'POST',
  headers: { 'content-type': 'application/json', 'x-api-key': process.env.WKEY },
  body: JSON.stringify(payload),
});
const data = await res.json().catch(() => ({}));
console.log(res.status, data.error?.type || (data.content ? 'content' : 'unknown'));
JS
```

Expected: HTTP 200 with `content`.

### Safe public Sub2API smoke

Use a test Sub2API key from the database or admin UI. Do not paste it into docs
or logs.

```bash
curl -sS http://127.0.0.1:8080/v1/messages \
  -H "authorization: Bearer <sub2api-test-key>" \
  -H 'content-type: application/json' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4.6",
    "max_tokens": 32,
    "messages": [{"role": "user", "content": "reply with ok"}]
  }'
```

Also smoke `gemini-2.5-flash` after account probe or manual tier correction.

### If Windsurf says `model_not_entitled`

Check whether this is a real entitlement failure or a stale/over-strict
`availableModels` preflight:

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

docker compose exec -T -e WKEY="$WINDSURF_API_KEY" windsurf-api node - <<'JS'
const res = await fetch('http://127.0.0.1:3003/auth/accounts', {
  headers: { 'x-api-key': process.env.WKEY },
});
const data = await res.json();
for (const a of data.accounts || []) {
  const available = a.availableModels || [];
  const cap = a.capabilities?.['gemini-2.5-flash'];
  console.log({
    id: a.id,
    status: a.status,
    tier: a.tier,
    hasGemini: available.includes('gemini-2.5-flash'),
    geminiCapOk: cap?.ok,
    geminiCapReason: cap?.reason,
  });
}
JS
```

If the account is a valid Pro/Trial account and the capability is successful
but `availableModels` excludes the model, apply the short-term internal
dashboard correction:

```bash
docker compose exec -T \
  -e DPASS="$WINDSURF_DASHBOARD_PASSWORD" \
  -e ACCOUNT_ID="<windsurf-account-id>" \
  windsurf-api node - <<'JS'
const res = await fetch(`http://127.0.0.1:3003/dashboard/api/accounts/${process.env.ACCOUNT_ID}`, {
  method: 'PATCH',
  headers: {
    'content-type': 'application/json',
    'x-dashboard-password': process.env.DPASS,
  },
  body: JSON.stringify({ tier: 'pro', resetErrors: true }),
});
const data = await res.json().catch(() => ({}));
console.log(res.status, data.success === true ? 'success' : data);
JS
```

Then clear only the internal Sub2API upstream account transient/error state:

```bash
docker compose exec -T postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
UPDATE accounts
SET temp_unschedulable_until = NULL,
    temp_unschedulable_reason = NULL,
    rate_limit_reset_at = NULL,
    overload_until = NULL,
    error_message = NULL,
    schedulable = true,
    status = '\''active'\''
WHERE name = '\''windsurf-internal-anthropic'\'';
"'
docker compose restart sub2api
```

Retest internal WindsurfAPI first, then public Sub2API.

### Updating the server later

Sub2API fork:

```powershell
cd D:\wflogin\sub2api-private
git fetch upstream main
git log --oneline HEAD..upstream/main
git merge upstream/main
go test ./internal/handler/admin -run Windsurf
go test ./internal/handler/admin
go test ./internal/server
git push
```

If the change touches the frontend admin UI, also run from `frontend`:

```powershell
$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm install --frozen-lockfile
corepack pnpm exec vitest run src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
```

Do not commit a generated `frontend/pnpm-workspace.yaml` unless it is an
intentional repository policy change.

Build and deploy a new server image with a new immutable tag. Keep the previous
tag for rollback.

WindsurfAPI:

```bash
cd /opt/sub2api
docker compose pull windsurf-api
docker compose up -d windsurf-api
docker compose logs --tail=200 windsurf-api
```

After every WindsurfAPI update, re-run the account list check and smoke
`gemini-2.5-flash` plus `claude-sonnet-4.6`.

### Internal Kiro adapter deployment

Target shape:

```text
sub2api -> http://kiro-rs:8990 -> kiro.rs -> Kiro/AWS upstream
```

Do not publish port `8990` and do not add a Caddy route for `kiro-rs`.

Server files:

```text
/opt/sub2api/kiro-rs/config/config.json
/opt/sub2api/kiro-rs/config/credentials.json
```

`config.json` shape:

```json
{
  "host": "0.0.0.0",
  "port": 8990,
  "apiKey": "<internal-model-request-key>",
  "adminApiKey": "<internal-admin-key>",
  "tlsBackend": "rustls",
  "region": "us-east-1",
  "defaultEndpoint": "ide"
}
```

`credentials.json` can start as an empty array:

```json
[]
```

Compose service should be internal only:

```yaml
  kiro-rs:
    image: ghcr.io/hank9999/kiro-rs:latest
    restart: unless-stopped
    volumes:
      - ./kiro-rs/config:/app/config
```

Sub2API environment:

```env
KIRO_ADAPTER_INTERNAL_BASE_URL=http://kiro-rs:8990
KIRO_ADAPTER_ADMIN_API_KEY=<same-as-config-adminApiKey>
KIRO_ADAPTER_TIMEOUT_SECONDS=30
```

Safe health checks:

```bash
cd /opt/sub2api
docker compose ps kiro-rs
docker compose logs --tail=100 kiro-rs
docker compose exec -T sub2api wget -q -T 5 -O - http://kiro-rs:8990/v1/models || true
```

Import smoke after adding a real credential through Sub2API admin UI:

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

docker compose exec -T -e KIRO_KEY="$KIRO_API_KEY" kiro-rs sh -lc '
wget -q -T 30 -O - \
  --header="content-type: application/json" \
  --header="x-api-key: $KIRO_KEY" \
  --post-data="{\"model\":\"claude-sonnet-4-6\",\"max_tokens\":32,\"messages\":[{\"role\":\"user\",\"content\":\"reply with ok\"}]}" \
  http://127.0.0.1:8990/v1/messages
'
```

If direct Kiro smoke passes, create or enable the Sub2API upstream account:

```json
{
  "platform": "anthropic",
  "type": "apikey",
  "name": "kiro-internal-anthropic",
  "credentials": {
    "api_key": "<KIRO_API_KEY>",
    "base_url": "http://kiro-rs:8990"
  },
  "extra": {
    "anthropic_passthrough": true
  }
}
```

Potential follow-up: evaluate whether Claude Code clients should route through
`/cc/v1/messages`; standard Sub2API Anthropic passthrough uses `/v1/messages`.

### Current fork image after 2026-05-19 fusion round 2

- Current image on the US server: `sub2api-provider-adapters:c4cefc76`.
- Rollback image: `sub2api-provider-adapters:f95c2073`.
- Pre-deploy backup:
  `/opt/sub2api-backups/sub2api-20260519-102757-pre-c4cefc76.tar.gz`.
- Public Sub2API health passed on `https://api.vyywcw.cn/` and
  `https://www.vyywcw.cn/`.
- `windsurf-api` and `kiro-rs` remain internal Docker services with no
  published host ports.
