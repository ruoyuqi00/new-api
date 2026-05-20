# Provider Account Operations Guide

Last updated: 2026-05-20

This guide covers the live `vyywcw.cn` deployment. Do not copy provider
tokens, API keys, account passwords, SSH passwords, or `.env` values into this
file.

## Current Public Entry

Sub2API is the primary public entry:

- `https://api.vyywcw.cn/`
- `https://www.vyywcw.cn/`

Provider adapter admin entry in Sub2API:

- `https://api.vyywcw.cn/admin/provider-adapters`
- Sidebar label: `适配器后台`
- Account page path: `账号管理` -> `更多操作` -> `适配器后台`

Native adapter admin UIs are exposed through same-domain Caddy paths so the
entry can live inside the Sub2API admin page:

- Windsurf console: `https://api.vyywcw.cn/windsurf-dashboard`
- Kiro admin: `https://api.vyywcw.cn/kiro-admin`

Those native UIs keep their own auth. Windsurf dashboard/API calls require the
dashboard password, and Kiro admin API calls require the Kiro admin API key.

Internal adapters stay inside the Docker network:

- `windsurf-api:3003`
- `kiro-rs:8990`
- `kiro-gateway:8000`

Docker currently publishes none of those adapter ports to the host. Caddy only
adds path-level reverse proxies for the native admin UIs and their supporting
asset/API paths; it does not publish raw container ports.

Current Caddy adapter paths:

- `/windsurf-dashboard*` -> `windsurf-api:3003` `/dashboard`
- `/dashboard/api/*`, `/dashboard/i18n/*`, `/dashboard/data/*` -> `windsurf-api:3003`
- `/kiro-admin*` -> `kiro-rs:8990` `/admin`
- `/admin/assets/*`, `/admin/vite.svg`, `/api/admin/*` -> `kiro-rs:8990`

Current external Sub2API group/key boundary:

- Group: `provider-mixed`
- Group type: exclusive (`is_exclusive=true`), not public.
- Public API key name: `server-provider-mixed`
- The group contains both `windsurf-internal-anthropic` and
  `kiro-gateway-internal-anthropic`.
- The group has model routing enabled for Kiro Gateway models so overlapping
  model names do not randomly select the Windsurf account.

## Current Provider Runtime Status

Windsurf:

- Internal service: `sub2api-windsurf-api`
- Sub2API upstream account: `windsurf-internal-anthropic`
- Public smoke passed through `https://api.vyywcw.cn/v1/messages`
- Smoke model: `claude-sonnet-4.6`
- Current account capability status: the Windsurf dashboard reports 81 `ok`
  models. Sub2API exposure is synced from that set plus usable aliases, with
  direct-smoke deprecated failures such as `gpt-4o-mini` and `grok-3-mini`
  intentionally pruned from `/v1/models`.

Kiro:

- Active runtime adapter for public Kiro models: `sub2api-kiro-gateway`
- Reference/admin adapter: `sub2api-kiro-rs`
- Current patched `kiro-rs` image: `kiro-rs-admin-metadata:20260519-1425`
- Sub2API upstream account: `kiro-gateway-internal-anthropic`
- Public smoke passed through `https://api.vyywcw.cn/v1/messages`
- Smoke models:
  - `qwen3-coder-next`
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.1`
  - `minimax-m2.5`

Do not expose Claude-family Kiro models until server-side and public smoke both
pass. Local `kiro.rs` can use Claude with the local proxy path, but the US
server direct egress still needs separate proxy/egress investigation before
Claude Kiro models should be opened publicly.

## Add Provider Credentials In The Admin UI

Open the admin UI:

```text
https://api.vyywcw.cn/
```

Then:

1. Log in as the Sub2API administrator.
2. Go to `账号`.
3. Open `更多操作`.
4. Choose `导入 Windsurf` or `导入 Kiro`.

These import buttons add credentials into the internal adapter. They are not the
same as creating a normal Sub2API upstream account. The upstream accounts that
route traffic to `windsurf-api` and `kiro-gateway` already exist on the server.

### Windsurf Import Modes

Use `导入 Windsurf`.

Supported modes:

- `Token`: one Windsurf token per line.
- `API Key`: one Codeium/Windsurf key per line.
- `邮箱密码`: one `email----password` per line.
- `JSON`: full object or `accounts` array.

Example JSON shape:

```json
{
  "accounts": [
    {
      "email": "user@example.com",
      "password": "<password>"
    },
    {
      "token": "<windsurf-token>"
    },
    {
      "api_key": "<codeium-or-windsurf-key>"
    }
  ]
}
```

After import, test through Sub2API, not by exposing WindsurfAPI:

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H "authorization: Bearer <your-sub2api-api-key>" \
  -H "content-type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4.6",
    "max_tokens": 32,
    "messages": [{"role": "user", "content": "reply with ok"}],
    "stream": false
  }'
```

### Kiro Import Modes

Use `导入 Kiro`.

Supported modes:

- `Refresh Token`: one refresh token per line, or `email----refreshToken`.
- `API Key`: one `ksk_...` Kiro API key per line, or `email----ksk_...`.
- `JSON`: full object, exported object, or `accounts` array.

For exported Kiro account JSON, prefer `JSON` mode and paste the complete object
or array. The Sub2API bridge extracts and forwards:

- `accessToken` / `access_token`
- `refreshToken` / `refresh_token`
- `profileArn` / `profile_arn`
- `expiresAt` / `expires_at`
- `email` / `loginHint`
- region, client, proxy, and endpoint fields when present

The patched `kiro-rs` image now accepts and preserves the full token metadata
instead of dropping `accessToken`, `profileArn`, and `expiresAt` during admin
import.

The re-applyable patch is stored at
`provider-patches/kiro-rs/admin-metadata-preservation-20260519.patch`.

Example JSON shape:

```json
{
  "accounts": [
    {
      "email": "user@example.com",
      "accessToken": "<access-token>",
      "refreshToken": "<refresh-token>",
      "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/ABCDEF",
      "expiresAt": "2026-05-14T10:51:10Z",
      "authMethod": "social",
      "apiRegion": "us-east-1",
      "authRegion": "us-east-1"
    }
  ]
}
```

Kiro public traffic currently routes through `kiro-gateway`, not `kiro-rs`.
After import, only enable additional public Kiro model mappings after smoke
passes. Current safe public Kiro smoke:

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H "authorization: Bearer <your-sub2api-api-key>" \
  -H "content-type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "qwen3-coder-next",
    "max_tokens": 32,
    "messages": [{"role": "user", "content": "reply with ok"}],
    "stream": false
  }'
```

## Server-Side Verification Commands

Check services:

```bash
cd /opt/sub2api
docker compose ps
```

Confirm adapter container ports are not host-published:

```bash
docker inspect sub2api-windsurf-api sub2api-kiro-rs sub2api-kiro-gateway \
  --format '{{.Name}} {{json .NetworkSettings.Ports}}'
```

Expected:

```text
/sub2api-windsurf-api {"3003/tcp":null}
/sub2api-kiro-rs {"8990/tcp":null}
/sub2api-kiro-gateway {"8000/tcp":null}
```

Confirm public admin paths and unauthenticated protection:

```bash
curl -sS -o /dev/null -w 'sub2api=%{http_code}\n' https://api.vyywcw.cn/
curl -sS -o /dev/null -w 'adapter_page=%{http_code}\n' https://api.vyywcw.cn/admin/provider-adapters
curl -sS -o /dev/null -w 'windsurf_ui=%{http_code}\n' https://api.vyywcw.cn/windsurf-dashboard
curl -sS -o /dev/null -w 'kiro_ui=%{http_code}\n' https://api.vyywcw.cn/kiro-admin
curl -sS -o /dev/null -w 'sub2api_adapter_unauth=%{http_code}\n' https://api.vyywcw.cn/api/v1/admin/provider-adapters/windsurf/accounts
curl -sS -o /dev/null -w 'windsurf_native_unauth=%{http_code}\n' https://api.vyywcw.cn/dashboard/api/accounts
curl -sS -o /dev/null -w 'kiro_native_unauth=%{http_code}\n' https://api.vyywcw.cn/api/admin/credentials
```

Expected public status:

```text
sub2api=200
adapter_page=200
windsurf_ui=200
kiro_ui=200
sub2api_adapter_unauth=401
windsurf_native_unauth=401
kiro_native_unauth=401
```

Check `kiro-rs` credential status without printing tokens:

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

docker compose exec -T -e KADM="$KIRO_ADMIN_API_KEY" kiro-rs sh -lc '
wget -q -T 20 -O - \
  --header="x-api-key: $KADM" \
  --header="authorization: Bearer $KADM" \
  http://127.0.0.1:8990/api/admin/credentials
'
```

The response is designed to show counts, hashes, email labels, and status. It
must not include raw access tokens or refresh tokens.

## Updating Later

Sub2API fork:

```powershell
cd D:\wflogin\sub2api-private
git fetch upstream main
git log --oneline HEAD..upstream/main
git merge upstream/main
go test ./internal/handler/admin -run "Windsurf|Kiro"
go test ./internal/handler/admin
go test ./internal/server
git push
```

Then build a new immutable Sub2API image, update `/opt/sub2api/docker-compose.yml`,
restart only `sub2api`, and run public smoke.

WindsurfAPI:

```bash
cd /opt/sub2api
docker compose pull windsurf-api
docker compose up -d windsurf-api
docker compose logs --tail=200 windsurf-api
```

After every WindsurfAPI update, re-run a public Sub2API smoke for
`claude-sonnet-4.6`.

kiro.rs:

1. Fetch `hank9999/kiro.rs`.
2. Re-apply the admin metadata patch if upstream has not merged an equivalent
   fix.
3. Build a new internal image tag.
4. Switch only `kiro-rs` in compose.
5. Confirm `GET /api/admin/credentials` still reports `hasProfileArn` for
   imported full credentials.

kiro-gateway:

```bash
cd /opt/sub2api
docker compose pull kiro-gateway
docker compose up -d kiro-gateway
```

After every update, re-run public smoke for the Kiro models before adding or
restoring mappings.

## Current Caveat

The server `.env` `ADMIN_PASSWORD` no longer matches the current admin login.
That is fine for browser operation because the user knows the current password,
but command-line admin import smoke cannot log in until the current password or
a valid admin session token is supplied. Do not write that password into Git or
shell history.
