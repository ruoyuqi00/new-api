# Interface Research Playbook

记录日期：2026-05-16

这个文档用于“协议更新时怎么调研”。每次 Windsurf/Kiro/Sub2API 任何一侧变化，都按这里重新跑一遍关键接口，记录成功样例和失败样例。

## 调研原则

- 先直接调内部代理，再通过 Sub2API 调同一请求。
- 所有 token、API key、refresh token 在记录时必须脱敏。
- 每个接口至少记录：
  - method + path
  - auth header
  - required headers
  - request body shape
  - streaming/non-streaming 行为
  - success response shape
  - error response shape
  - rate limit / invalid token / no account 行为
- 如果参考项目更新，先看 diff，再跑最小请求。

## 参考项目和关键文件

Windsurf 主参考：

- GitHub: `https://github.com/dwgx/WindsurfAPI`
- Local: `D:\wflogin\_github_research\WindsurfAPI`
- Key files:
  - `README.md`
  - `docker-compose.yml`
  - `src/server.js`
  - `src/handlers/messages.js`
  - `src/handlers/chat.js`
  - `src/handlers/responses.js`
  - `src/auth.js`
  - `src/dashboard/windsurf-login.js`
  - `src/models.js`
  - `src/langserver.js`
  - `src/config.js`

Windsurf 备选参考：

- GitHub: `https://github.com/guanxiaol/WindsurfPoolAPI`
- Local: `D:\wflogin\_github_research\WindsurfPoolAPI`

Kiro 主参考：

- GitHub: `https://github.com/hank9999/kiro.rs`
- Local extracted copy: `D:\wflogin\kiro.rs-master`
- Key files:
  - `README.md`
  - `src/main.rs`
  - `src/anthropic/handlers.rs`
  - `src/kiro/token_manager.rs`
  - `src/kiro/endpoint/`
  - `src/kiro/machine_id.rs`
  - `src/admin/`
  - `src/token.rs`

Sub2API key files:

- `backend/internal/domain/constants.go`
- `backend/internal/service/account.go`
- `backend/internal/service/account_service.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/config/config.go`
- `frontend/src/components/account/credentialsBuilder.ts`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/composables/useModelWhitelist.ts`

## WindsurfAPI observed interface matrix

From `src/server.js` and README:

| Method | Path | Purpose | Auth | Research status |
| --- | --- | --- | --- | --- |
| GET | `/health` | health check | none or API key depending deployment | pending server verification |
| GET | `/auth/status` | auth/service status | API key/dashboard session | pending |
| GET | `/auth/accounts` | list accounts | API key/dashboard session | pending |
| POST | `/auth/login` | add one or batch account | API key/dashboard session | source confirmed |
| DELETE | `/auth/accounts/:id` | remove account | API key/dashboard session | pending |
| GET | `/v1/models` | OpenAI-compatible models | API key | pending |
| POST | `/v1/chat/completions` | OpenAI chat completions | API key | source confirmed |
| POST | `/v1/responses` | OpenAI responses | API key | source confirmed |
| POST | `/v1/response` | compatibility alias | API key | source confirmed |
| POST | `/v1/messages` | Anthropic messages | API key | source confirmed |

WindsurfAPI auth headers to test:

```text
Authorization: Bearer <windsurf-api-key>
x-api-key: <windsurf-api-key>
```

WindsurfAPI account import body variants:

```json
{"token":"<windsurf-token>"}
```

```json
{"api_key":"<codeium-api-key>"}
```

```json
{"accounts":[{"token":"<token-1>"},{"token":"<token-2>"}]}
```

WindsurfAPI OpenAI request probe:

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

WindsurfAPI Anthropic request probe:

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

Need to record:

- [ ] Does `/v1/messages` require `anthropic-version`?
- [ ] Does it preserve tool_use/tool_result shape?
- [ ] Does it support `system` as string and array?
- [ ] Does it support image inputs?
- [ ] Does it support streaming with valid SSE events?
- [ ] What exact error appears when no active accounts exist?
- [ ] What exact error appears when all accounts are rate-limited?
- [ ] What exact error appears when a token is invalid?
- [ ] Which models work for free/trial/pro accounts?
- [ ] Does `/v1/models` report only currently available models or static models?

## Kiro observed interface matrix

From `README.md`, `src/main.rs`, and `src/anthropic/handlers.rs`:

| Method | Path | Purpose | Auth | Research status |
| --- | --- | --- | --- | --- |
| GET | `/v1/models` | model list | proxy API key if configured | pending |
| POST | `/v1/messages` | Anthropic messages streaming | proxy API key if configured | source confirmed |
| POST | `/v1/messages/count_tokens` | token count | proxy API key if configured | source confirmed |
| POST | `/cc/v1/messages` | Claude Code compatible buffered messages | proxy API key if configured | source confirmed |
| POST | `/cc/v1/messages/count_tokens` | Claude Code count tokens | proxy API key if configured | source confirmed |
| Admin | varies | credential management | admin auth if configured | pending |

Kiro credential fields to test:

```json
{
  "accessToken": "<optional-access-token>",
  "refreshToken": "<refresh-token>",
  "expiresAt": "2026-12-31T00:00:00Z",
  "authMethod": "social",
  "profileArn": "<optional-profile-arn>",
  "region": "us-east-1",
  "authRegion": "us-east-1",
  "apiRegion": "us-east-1"
}
```

```json
{
  "refreshToken": "<refresh-token>",
  "authMethod": "idc",
  "clientId": "<client-id>",
  "clientSecret": "<client-secret>",
  "region": "us-east-1"
}
```

```json
{
  "authMethod": "api_key",
  "kiroApiKey": "<kiro-api-key>",
  "region": "us-east-1"
}
```

Kiro Anthropic request probe:

```bash
curl -sS http://127.0.0.1:8990/v1/messages \
  -H 'content-type: application/json' \
  -H 'x-api-key: <kiro-proxy-api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hi"}],
    "stream": true
  }'
```

Need to record:

- [ ] Which auth header does the proxy require?
- [ ] Does `/v1/messages` stream event order match Anthropic?
- [ ] Does `/cc/v1/messages` change `usage.input_tokens`?
- [ ] What happens when `refreshToken` expires with `invalid_grant`?
- [ ] Does region priority work as documented?
- [ ] Does `kiroApiKey` bypass refresh fully?
- [ ] Does `profileArn` need to be present for some accounts?
- [ ] Does count_tokens need external API?

## Sub2API observed admin interface matrix

From `backend/internal/handler/admin/account_handler.go`:

| Method | Path | Purpose | Notes |
| --- | --- | --- | --- |
| GET | `/api/v1/admin/accounts` | list accounts | supports filters |
| GET | `/api/v1/admin/accounts/:id` | get account | account detail |
| POST | `/api/v1/admin/accounts` | create account | primary path for upstream accounts |
| PUT | `/api/v1/admin/accounts/:id` | update account | edit base_url/model mapping |
| DELETE | `/api/v1/admin/accounts/:id` | delete account | destructive |
| POST | `/api/v1/admin/accounts/:id/test` | test account | SSE streaming test |
| POST | `/api/v1/admin/accounts/batch` | batch create | useful for later native import |
| POST | `/api/v1/admin/accounts/data` | import account data | existing export/import path |
| GET | `/api/v1/admin/accounts/:id/models` | available models | model mapping path |
| POST | `/api/v1/admin/accounts/:id/refresh` | refresh token | only some account types |

Current platform constants:

```text
anthropic
openai
gemini
antigravity
```

Current account type constants:

```text
oauth
setup-token
apikey
upstream
bedrock
service_account
```

Sub2API fields relevant to internal adapters:

- `credentials.api_key`
- `credentials.base_url`
- `extra.anthropic_passthrough`
- `extra.openai_passthrough`
- `model_mapping`
- account groups and priorities

Need to record:

- [ ] Can UI create Anthropic API key account with `base_url=http://windsurf-api:3003`?
- [ ] Does URL allowlist allow Docker service names?
- [ ] Does account test path append `/v1/messages` correctly for Anthropic API key accounts?
- [ ] Does OpenAI API key account test use `/v1/responses` by default?
- [ ] If WindsurfAPI only supports `/v1/chat/completions`, should OpenAI route disable responses and use chat?
- [ ] Which account type gives the cleanest pass-through behavior?

## Research log template

Copy this block for each protocol change.

```markdown
## YYYY-MM-DD Provider Change Research

Provider:
Reference project:
Reference commit/tag:
Files inspected:

Change summary:

Direct proxy tests:
- endpoint:
- request:
- response:
- error cases:

Sub2API-through tests:
- account config:
- model mapping:
- request:
- response:
- error cases:

Decision:
- update internal proxy only:
- update Sub2API fork:
- no action:

Follow-ups:
- [ ] item
```

