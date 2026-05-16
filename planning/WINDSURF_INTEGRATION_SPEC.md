# Windsurf Integration Spec

记录日期：2026-05-16

## Decision

短期使用 `dwgx/WindsurfAPI` 作为内部代理，不直接把 Windsurf 私有协议写进 Sub2API。

原因：

- 已经支持 OpenAI compatible `/v1/chat/completions`。
- 已经支持 Anthropic compatible `/v1/messages`。
- 已经支持批量账号 token 导入。
- 已经有 Docker 部署和账号池逻辑。
- 协议变化时可以先升级 WindsurfAPI，再决定是否同步到 Sub2API fork。

备选项目 `guanxiaol/WindsurfPoolAPI` 只作为协议对照。

## Runtime shape

```text
sub2api container
  -> http://windsurf-api:3003
  -> WindsurfAPI
  -> local language_server_linux_x64
  -> Windsurf cloud
```

公网：

- `https://api.vyywcw.cn` exposes Sub2API.
- Do not expose WindsurfAPI.

Docker network：

- `sub2api`
- `windsurf-api`
- `postgres`
- `redis`
- `caddy`

## WindsurfAPI environment

Required:

```env
PORT=3003
DATA_DIR=/data
LS_BINARY_PATH=/opt/windsurf/language_server_linux_x64
API_KEY=<strong-internal-api-key>
DASHBOARD_PASSWORD=<strong-dashboard-password>
```

Optional:

```env
LS_DATA_DIR=/opt/windsurf/data
LS_PORT=42100
ALLOW_PRIVATE_PROXY_HOSTS=1
CODEIUM_API_KEY=<only-if-using-direct-key-mode>
```

Security requirements:

- `API_KEY` must not be empty.
- `DASHBOARD_PASSWORD` must not be empty.
- Do not bind `3003` to public `0.0.0.0`.
- Prefer no published port at all; use Docker service name from Sub2API.
- If debugging on host, bind `127.0.0.1:3003:3003` only.

## Persistent data

WindsurfAPI stores:

- `accounts.json`
- `proxy.json`
- `stats.json`
- `runtime-config.json`
- `model-access.json`
- `logs/`

Server path recommendation:

```text
/opt/sub2api/windsurf-api/data
/opt/sub2api/windsurf-api/opt/windsurf
/opt/sub2api/windsurf-api/tmp/windsurf-workspace
```

Backup requirements:

- Back up `accounts.json` before every upgrade.
- Back up `runtime-config.json` if runtime credentials are changed through dashboard.
- Never copy these files into Git.

## Compose service draft

This is a draft. Before applying, compare with the actual server compose file.

```yaml
  windsurf-api:
    image: ghcr.io/dwgx/windsurf-api:latest
    restart: unless-stopped
    environment:
      PORT: "3003"
      DATA_DIR: /data
      LS_BINARY_PATH: /opt/windsurf/language_server_linux_x64
      API_KEY: ${WINDSURF_API_KEY}
      DASHBOARD_PASSWORD: ${WINDSURF_DASHBOARD_PASSWORD}
    volumes:
      - ./windsurf-api/data:/data
      - ./windsurf-api/opt/windsurf:/opt/windsurf
      - ./windsurf-api/tmp/windsurf-workspace:/tmp/windsurf-workspace
    networks:
      - default
```

Do not add Caddy route for `windsurf-api`.

## Account import

Single token:

```bash
curl -sS http://127.0.0.1:3003/auth/login \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <windsurf-api-key>' \
  -d '{"token":"<windsurf-token>"}'
```

Batch:

```bash
curl -sS http://127.0.0.1:3003/auth/login \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <windsurf-api-key>' \
  -d '{"accounts":[{"token":"<token-1>"},{"token":"<token-2>"}]}'
```

Operational rule:

- Put real tokens in a temporary root-only file on the server.
- Post the file from server shell.
- Delete the temporary file immediately.
- Do not paste tokens into Git, docs, or long-lived shell history.

## Direct validation

Health:

```bash
docker compose ps windsurf-api
docker compose logs --tail=100 windsurf-api
curl -sS http://127.0.0.1:3003/health
```

Anthropic-compatible request:

```bash
curl -sS http://127.0.0.1:3003/v1/messages \
  -H 'content-type: application/json' \
  -H 'x-api-key: <windsurf-api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4.6",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hi"}],
    "stream": false
  }'
```

OpenAI-compatible request:

```bash
curl -sS http://127.0.0.1:3003/v1/chat/completions \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <windsurf-api-key>' \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [{"role": "user", "content": "hi"}],
    "stream": false
  }'
```

## Sub2API account shape

Preferred first route: Anthropic-compatible account.

```json
{
  "platform": "anthropic",
  "type": "apikey",
  "name": "windsurf-internal-anthropic",
  "credentials": {
    "api_key": "<windsurf-api-key>",
    "base_url": "http://windsurf-api:3003"
  },
  "extra": {
    "anthropic_passthrough": true
  },
  "model_mapping": {
    "claude-sonnet-4-6": "claude-sonnet-4.6"
  }
}
```

Alternative route: OpenAI-compatible account.

```json
{
  "platform": "openai",
  "type": "apikey",
  "name": "windsurf-internal-openai",
  "credentials": {
    "api_key": "<windsurf-api-key>",
    "base_url": "http://windsurf-api:3003/v1"
  },
  "extra": {
    "openai_passthrough": true
  },
  "model_mapping": {
    "gpt-5.5": "gpt-5.5",
    "claude-sonnet-4-6": "claude-sonnet-4.6"
  }
}
```

Need to verify:

- [ ] Whether Anthropic passthrough should be true or false.
- [ ] Whether Sub2API appends `/v1/messages` correctly.
- [ ] Whether OpenAI route uses Responses API by default and whether WindsurfAPI supports enough of `/v1/responses`.
- [ ] Whether model names with dots or hyphens need mapping.

## Native Sub2API Windsurf import design

Stage A: proxy-backed native import.

- Sub2API UI accepts batch Windsurf tokens.
- Sub2API backend calls internal WindsurfAPI `/auth/login`.
- Sub2API does not store raw Windsurf tokens.
- Sub2API stores one upstream account pointing to WindsurfAPI.
- Account list can show a synthetic count or link to internal service status later.

Stage B: partial state mirror.

- Sub2API periodically reads WindsurfAPI `/auth/accounts`.
- Sub2API stores account count, status, rate-limit state, available models.
- Sub2API does not own refresh logic yet.

Stage C: direct native provider.

- Sub2API stores Windsurf credential material.
- Sub2API implements token refresh or equivalent Codeium/Windsurf auth.
- Sub2API directly handles model access, quota, cooldown, and errors.
- This is highest risk and should wait until proxy path is stable.

## Implementation tasks for Stage A

Backend:

- [ ] Add internal adapter config:
  - `providers.windsurf.internal_base_url`
  - `providers.windsurf.internal_api_key`
- [ ] Add `WindsurfImportRequest`.
- [ ] Add `POST /api/v1/admin/providers/windsurf/import`.
- [ ] Validate input supports line mode and JSON mode.
- [ ] Redact all token values in logs.
- [ ] Call internal `/auth/login`.
- [ ] Normalize per-account success/failure.
- [ ] Return import summary.
- [ ] Add tests for parsing, redaction, success, partial failure.

Frontend:

- [ ] Add Windsurf import modal.
- [ ] Support textarea batch import.
- [ ] Show per-account result.
- [ ] Link to create Sub2API upstream account if missing.
- [ ] Add zh/en i18n strings.

Ops:

- [ ] Add env placeholders to server compose.
- [ ] Add backup path to backup script.
- [ ] Add logs command to runbook.

## Open questions

- Does WindsurfAPI dashboard need to be reachable at all, or should we only use its API from server shell?
- Which Windsurf model names should be exposed to users by default?
- Should Windsurf accounts live in a separate Sub2API group by default?
- Should public API keys be allowed to route to Windsurf by default, or only selected groups?
- How to price/account for Windsurf-backed requests in Sub2API billing tables?

