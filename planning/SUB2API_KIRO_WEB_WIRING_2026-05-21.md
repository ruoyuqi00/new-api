# Sub2API Kiro Web Wiring - 2026-05-21

Do not put raw API keys, account passwords, access tokens, refresh tokens,
cookies, or SSH credentials in this file.

## What Changed

The server now has a new Sub2API upstream account:

```text
name: kiro-web-internal-openai
platform: openai
type: apikey
base_url: http://kiro-web-adapter:8991
group: provider-mixed
```

This account routes public Sub2API OpenAI-compatible requests to the internal
Kiro Web Portal adapter. The adapter itself is still private on the Docker
network and is not exposed by Caddy.

The existing public entrypoint remains:

```text
https://api.vyywcw.cn/
```

## Public Usage

Use a Sub2API key from the `provider-mixed` group.

### Kiro Web Claude Models

Use OpenAI Chat Completions:

```bash
curl https://api.vyywcw.cn/v1/chat/completions \
  -H 'Authorization: Bearer <your-sub2api-key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "claude-opus-4.7",
    "messages": [{"role": "user", "content": "hi"}],
    "stream": false
  }'
```

Validated through the public domain:

| Endpoint | Model | Result |
| --- | --- | --- |
| `/v1/chat/completions` | `claude-sonnet-4.6` | HTTP 200, expected text returned |
| `/v1/chat/completions` | `claude-opus-4.7` | HTTP 200, expected text returned |
| `/v1/chat/completions` streaming | `claude-sonnet-4.6` | HTTP 200, SSE text returned |

### Windsurf Models

Use Anthropic Messages:

```bash
curl https://api.vyywcw.cn/v1/messages \
  -H 'x-api-key: <your-sub2api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [{"role": "user", "content": "hi"}],
    "max_tokens": 64
  }'
```

Validated through the public domain:

| Endpoint | Model | Result |
| --- | --- | --- |
| `/v1/messages` | `claude-sonnet-4.6` | HTTP 200, expected text returned |

The current Windsurf account mapping is broad and includes Claude, GPT,
Gemini, GLM, Grok, Kimi, MiniMax, SWE, and other model aliases. Use the
Sub2API admin account model view as the source of truth before giving a model
to users.

## Server Topology

```text
Public client
  -> https://api.vyywcw.cn/
  -> Caddy
  -> sub2api
  -> internal adapters:
       windsurf-api:3003
       kiro-gateway:8000
       kiro-web-adapter:8991
```

Current provider accounts in Sub2API:

| Sub2API account | Platform | Purpose |
| --- | --- | --- |
| `windsurf-internal-anthropic` | `anthropic` | Windsurf-backed Anthropic-compatible route |
| `kiro-gateway-internal-anthropic` | `anthropic` | Older Kiro gateway route for the 5 open models |
| `kiro-web-internal-openai` | `openai` | Kiro Web Portal Claude/Opus/Sonnet route |

## How To Add Accounts Later

### Sub2API

Sub2API is the public key, group, routing, quota, and billing layer.

Use the Sub2API admin UI to:

- create user-facing API keys;
- bind keys to `provider-mixed` or a future provider-specific group;
- adjust account concurrency, priority, status, and model mapping;
- add or remove model routing rules.

### Windsurf

Add Windsurf accounts in the Windsurf adapter backend/admin first. After the
adapter sees the account and token state, keep the Sub2API account pointing to:

```text
http://windsurf-api:3003
```

The public Sub2API side usually does not need a new account for every Windsurf
login. The Windsurf adapter should own the provider account pool; Sub2API owns
the public API key and routing layer.

### Kiro

Add or refresh Kiro credentials in the Kiro adapter storage used by:

```text
/opt/sub2api/kiro-rs/config/credentials.json
```

The Kiro Web adapter reuses that credential file. Sub2API should keep a single
internal account pointing to:

```text
http://kiro-web-adapter:8991
```

If Kiro credentials are rotated, restart or wait for the adapter to refresh its
cached session, then smoke-test through Sub2API.

## Smoke Test Commands

Run these on the server. They intentionally do not print API keys.

```bash
API_KEY="$(docker exec -i sub2api-postgres psql -U sub2api -d sub2api -At <<'SQL'
select key from api_keys where name='server-provider-mixed' and deleted_at is null order by id limit 1;
SQL
)"

curl -sS https://api.vyywcw.cn/v1/chat/completions \
  -H "Authorization: Bearer ${API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"Reply exactly: ok"}],"stream":false}'

curl -sS https://api.vyywcw.cn/v1/messages \
  -H "x-api-key: ${API_KEY}" \
  -H 'anthropic-version: 2023-06-01' \
  -H 'Content-Type: application/json' \
  -d '{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":"Reply exactly: ok"}],"max_tokens":32}'
```

## Update Notes

Reference status checked on 2026-05-21:

| Project | Latest observed state | Note |
| --- | --- | --- |
| `Wei-Shaw/sub2api` | `bd3d4d9a` on upstream `main` | Local fork is ahead and behind; merge should be a separate tested batch |
| `Jwadow/kiro-gateway` | HEAD `a5292ca0`, latest tag family through `v2.3` | Useful Kiro gateway reference, but current server fix uses our Web Portal adapter |
| `tickernelz/opencode-kiro-auth` | `v1.10.1` / `d0d9b18c` | Still CLI/CodeWhisperer oriented for this issue |
| `hongyilyu/pi-kiro` | `v0.1.3` / `43832737` | Still CLI-style route for this issue |

Do not replace the working Kiro Web adapter only because another project
updates. First compare whether the reference now uses the same Web Portal RPC
route with `CreateSpace` and `StreamSendMessage` plus `sessionId = spaceId`.
