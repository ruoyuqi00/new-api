# Kiro Web Portal Adapter

This adapter is a small internal service that exposes minimal Anthropic and
OpenAI-compatible endpoints while calling Kiro's current Web Portal RPC path.

It is meant to sit behind Sub2API on the private Docker network. Do not expose
this service directly to the public internet.

## Why This Exists

The older Kiro CLI / CodeWhisperer-style route can refresh the account token,
but for the current tested Kiro Pro credential it only exposes a small open
model list and rejects Claude/Opus with `INVALID_MODEL_ID`.

The Kiro Web Portal route is different:

- it calls `KiroWebPortalService` RPC v2 CBOR operations;
- `ListAvailableModels?origin=KIRO_CONSOLE` shows Claude models;
- `StreamSendMessage` works for Claude/Opus when the request includes
  `sessionId = spaceId`;
- the session must keep IdP, CSRF token, `UserId` cookie, and profile metadata
  consistent.

## Endpoints

- `GET /health`
- `GET /v1/models`
- `POST /v1/messages`
- `POST /v1/chat/completions`
- `GET /admin/usage`
- `GET /api/admin/credentials`
- `GET /api/admin/accounts`
- `GET /api/admin/status`

Authentication uses the same internal key style as the existing Kiro adapter:

- `Authorization: Bearer <key>`
- or `x-api-key: <key>`

By default the key is read from `/config/generated-kiro-api-key.txt`.

The `/api/admin/*` endpoints are internal, redacted management endpoints for
Sub2API's Adapter Admin page. They expose account counts, disabled/runtime
status, token expiry status, Profile ARN presence, model metadata, and routing
defaults, but never return raw access tokens or refresh tokens.

## Environment

| Variable | Default | Notes |
| --- | --- | --- |
| `KIRO_CREDENTIALS_FILE` | `/config/credentials.json` | Kiro credential JSON object or array |
| `KIRO_ADAPTER_API_KEY_FILE` | `/config/generated-kiro-api-key.txt` | Internal adapter auth key file |
| `KIRO_ADAPTER_API_KEY` | unset | Overrides key file |
| `KIRO_ADAPTER_HOST` | `0.0.0.0` | Bind host |
| `KIRO_ADAPTER_PORT` | `8991` | Bind port |
| `KIRO_RUNTIME_STATE_FILE` | `/config/kiro-runtime-state.json` | Non-secret account routing state file |
| `KIRO_ACCOUNT_SELECTION_STRATEGY` | `round-robin` | `round-robin`, `sticky`, or legacy `priority-first` |
| `KIRO_SESSION_AFFINITY_ENABLED` | `true` | Keep the same conversation/session hint on the same account |
| `KIRO_SESSION_AFFINITY_TTL_SECONDS` | `3600` | TTL for session-to-account affinity state |
| `KIRO_ENABLE_TOKEN_BUFFER_RESERVE` | unset | Set to `1` to enable conservative prompt trimming |
| `KIRO_TOKEN_BUFFER_RESERVE` | `20000` | Prompt trimming reserve below model context window when trimming is enabled |
| `KIRO_MODEL_CAPABILITIES_JSON` | unset | Optional JSON object to override per-model `context_window` and `max_output_tokens` |

## Account Routing And Cache State

The adapter stores account routing state in `KIRO_RUNTIME_STATE_FILE`. This
sidecar JSON file contains only rotation cursors, hashed session affinity keys,
and sanitized credential IDs. It does not contain access tokens, refresh tokens,
raw session IDs, or API keys.

Default routing is true round-robin across enabled credentials that support the
requested model. When a request includes a stable session hint, such as
`conversation_id`, `thread_id`, `session_id`, `prompt_cache_key`, `user`,
`metadata.user_id`, `X-Claude-Code-Session-Id`, `x-opencode-session`, or
`x-session-affinity`, the adapter keeps that hint on the same Kiro account for
the configured TTL. This preserves Kiro/Claude prompt-cache locality without
pinning unrelated new requests to one account.

Quota or overage-style upstream text is not treated as a dead account signal.
Only hard authentication failures during token refresh, such as invalid or
revoked tokens, disable a stored credential.

Context handling is model-aware but conservative:

- `claude-opus-4.8` is advertised and mapped as the newest Opus family alias
  learned from the current Kiro references. Live availability still depends on
  the Kiro account and Web Portal model rollout.
- `claude-opus-4.6` follows the current upstream model metadata with a
  `1,000,000` token context window and `128,000` max output tokens.
- Other advertised Kiro models have explicit context/output metadata instead
  of sharing one hidden default. Claude values follow the current upstream
  Sub2API model-pricing metadata where available; Kiro-only models use
  conservative adapter defaults that can be overridden without code changes.
- Prompt trimming is disabled by default. With the default deployment the
  adapter forwards the full request and lets Kiro enforce the real upstream
  context limit. Set `KIRO_ENABLE_TOKEN_BUFFER_RESERVE=1` only if you want the
  adapter to trim oversized prompts before forwarding them.

Example override:

```bash
KIRO_MODEL_CAPABILITIES_JSON='{"claude-opus-4.7":{"context_window":1000000,"max_output_tokens":128000},"glm-5":{"context_window":1048576}}'
```

## Docker Compose Snippet

```yaml
kiro-web-adapter:
  build:
    context: ./kiro-web-adapter
  image: sub2api-kiro-web-adapter:20260521
  container_name: sub2api-kiro-web-adapter
  restart: unless-stopped
  expose:
    - "8991"
  volumes:
    - ./kiro-rs/config:/config
  environment:
    - KIRO_CREDENTIALS_FILE=/config/credentials.json
    - KIRO_ADAPTER_API_KEY_FILE=/config/generated-kiro-api-key.txt
    - KIRO_RUNTIME_STATE_FILE=/config/kiro-runtime-state.json
    - KIRO_ACCOUNT_SELECTION_STRATEGY=round-robin
    - KIRO_SESSION_AFFINITY_ENABLED=true
    - KIRO_ADAPTER_HOST=0.0.0.0
    - KIRO_ADAPTER_PORT=8991
    - TZ=${TZ:-Asia/Shanghai}
  networks:
    - sub2api-network
```

## Smoke Tests

Run from inside the adapter container after copying the smoke script:

```bash
python3 /tmp/kiro_web_adapter_smoke.py --model claude-opus-4.7
python3 /tmp/kiro_web_adapter_smoke.py --model claude-opus-4.6
python3 /tmp/kiro_web_adapter_smoke.py --model claude-sonnet-4.6
python3 /tmp/kiro_web_adapter_smoke.py --mode openai --model claude-sonnet-4.6
python3 /tmp/kiro_web_adapter_smoke.py --stream --mode openai --model claude-sonnet-4.6
python3 /tmp/kiro_web_adapter_smoke.py --stream --mode anthropic --model claude-sonnet-4.6
```

Expected successful shape:

```json
{"status":200,"model":"claude-sonnet-4.6","text":"..."}
```

## Tested Server Results

On 2026-05-21, the deployed server confirmed:

- `claude-opus-4.7` non-stream Anthropic-compatible request returned 200.
- `claude-sonnet-4.6` non-stream Anthropic-compatible request returned 200.
- `claude-sonnet-4.6` non-stream OpenAI-compatible request returned 200.
- `claude-sonnet-4.6` OpenAI-compatible streaming returned 200.
- `claude-sonnet-4.6` Anthropic-compatible streaming returned 200.
- `claude-opus-4.7` can sometimes return a Kiro upstream "high volume of
  traffic" message; that is upstream model load, not adapter authentication.

On 2026-05-23, public Sub2API testing confirmed:

- canonical model `claude-opus-4.6` returned 200 through
  `https://api.vyywcw.cn/v1/messages`.
- direct adapter calls to `claude-opus-4.6` also returned 200 through both
  Anthropic-compatible and OpenAI-compatible endpoints.
- `claude-opus-4-6` should not be treated as the canonical public model name;
  use `claude-opus-4.6` unless a client explicitly owns an alias mapping.

## Sub2API Wiring

Short-term wiring should use an internal upstream account/channel:

```text
base_url: http://kiro-web-adapter:8991
api_key: same internal Kiro adapter key
models: claude-opus-4.7, claude-opus-4.6, claude-sonnet-4.6, ...
```

Use the OpenAI-compatible path if the Sub2API account/channel is configured as
OpenAI upstream. Use the Anthropic-compatible path if configured as Anthropic
upstream.

Current server wiring uses the OpenAI-compatible path:

```text
Sub2API account: kiro-web-internal-openai
Sub2API group: provider-mixed
public endpoint: https://api.vyywcw.cn/v1/chat/completions
internal base_url: http://kiro-web-adapter:8991
```

Validated through the public Sub2API domain on 2026-05-21:

- `claude-sonnet-4.6`
- `claude-opus-4.7`
- streaming `claude-sonnet-4.6`

Validated through the public Sub2API domain on 2026-05-23:

- `claude-opus-4.6`

## Current Limitations

- Text-only prompt conversion for the first production cut.
- Tool calls, image input, and full Anthropic feature parity are not yet
  implemented.
- Each request creates a fresh Kiro Web space/session.
- Auth/session material is cached in memory briefly, while account routing and
  conversation affinity are persisted in the runtime state sidecar.
- Model traffic errors from Kiro are currently passed through as text.
- This service is intentionally internal-only; Sub2API remains the public
  gateway for keys, groups, routing, and quotas.

## Upstream Sync Notes

The 2026-05-23 audit is recorded in
`docs/KIRO_UPSTREAM_AUDIT_2026-05-23.md`. Changes absorbed from the newer
reference projects:

- auto-disable stored credentials when refresh returns a hard auth failure;
- force-refresh once when the portal session cannot authenticate;
- optionally trim very large prompts against a conservative model context
  budget before calling `StreamSendMessage`.

The 2026-06-04 admin-console update kept this adapter as the Kiro runtime used
by Sub2API and learned only UI/protocol shape from Kiro-Go-style local gateway
projects:

- expose `/api/admin/credentials` so Sub2API's Kiro Runtime panel can populate
  accounts instead of showing sparse/offline data;
- include sanitized fields for auth method, region, Profile ARN presence,
  token status, runtime status, disabled reason, and supported model count;
- keep the public model/runtime endpoints unchanged.
