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

Authentication uses the same internal key style as the existing Kiro adapter:

- `Authorization: Bearer <key>`
- or `x-api-key: <key>`

By default the key is read from `/config/generated-kiro-api-key.txt`.

## Environment

| Variable | Default | Notes |
| --- | --- | --- |
| `KIRO_CREDENTIALS_FILE` | `/config/credentials.json` | Kiro credential JSON object or array |
| `KIRO_ADAPTER_API_KEY_FILE` | `/config/generated-kiro-api-key.txt` | Internal adapter auth key file |
| `KIRO_ADAPTER_API_KEY` | unset | Overrides key file |
| `KIRO_ADAPTER_HOST` | `0.0.0.0` | Bind host |
| `KIRO_ADAPTER_PORT` | `8991` | Bind port |
| `KIRO_TOKEN_BUFFER_RESERVE` | `50000` | Prompt trimming reserve below model context window |

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
- Model traffic errors from Kiro are currently passed through as text.
- This service is intentionally internal-only; Sub2API remains the public
  gateway for keys, groups, routing, and quotas.

## Upstream Sync Notes

The 2026-05-23 audit is recorded in
`docs/KIRO_UPSTREAM_AUDIT_2026-05-23.md`. Changes absorbed from the newer
reference projects:

- auto-disable stored credentials when refresh returns a hard auth failure;
- force-refresh once when the portal session cannot authenticate;
- trim very large prompts against a conservative model context budget before
  calling `StreamSendMessage`.
