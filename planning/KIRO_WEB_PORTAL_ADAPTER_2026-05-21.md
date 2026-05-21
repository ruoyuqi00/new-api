# Kiro Web Portal Adapter Report - 2026-05-21

Do not put raw access tokens, refresh tokens, passwords, cookies, SSH
credentials, or API keys in this file.

## Summary

The Kiro Claude/Opus issue is not an account entitlement failure. The tested
Kiro Pro account can call Claude-family models through Kiro's Web Portal RPC
path. The failing path is the older Kiro CLI / CodeWhisperer-style route used
by many public adapters.

The working route is:

```text
https://app.kiro.dev/service/KiroWebPortalService/operation/CreateSpace
https://app.kiro.dev/service/KiroWebPortalService/operation/StreamSendMessage
```

`StreamSendMessage` must include:

```text
spaceId: <created space id>
sessionId: <same value as spaceId>
contentBlocks: [{"text": {"text": "...prompt..."}}]
modelId: claude-opus-4.7 / claude-sonnet-4.6 / ...
```

The session must also keep these pieces consistent:

- IdP, currently `Google` for the tested account;
- CSRF token from `https://app.kiro.dev/`;
- `UserId` cookie from the same page load;
- Kiro profile metadata;
- bearer access token refreshed from the existing refresh token.

## Latest Reference Check

Checked again on 2026-05-21:

| Reference | Latest observed state | Result |
| --- | --- | --- |
| `tickernelz/opencode-kiro-auth` | HEAD/tag `v1.10.1` at `d0d9b18c8031abe29fe27c91d505a4f2ff24e9a0` | Still CLI/CodeWhisperer oriented; no Web Portal adapter fix found |
| `hongyilyu/pi-kiro` | HEAD/tag `v0.1.3` at `438327370eb86a35ba7ac89aa1249e3c2a18ec85` | Still CLI-style route; no Web Portal adapter fix found |
| Kiro Web assets | `assets.app.kiro.dev/releases/b482385f7dc7b681/` | Frontend sends `sessionId` initialized from `spaceId` |
| Kiro official CLI models docs | Web docs list Claude Opus/Sonnet models including Opus 4.7 | Confirms these are official Kiro model names |
| Kiro model changelog | 2026-era model changelog mentions new Claude models and plan availability | Confirms Pro-family availability context |

Useful source URLs:

- `https://github.com/tickernelz/opencode-kiro-auth`
- `https://github.com/hongyilyu/pi-kiro`
- `https://kiro.dev/docs/cli/models/`
- `https://kiro.dev/changelog/models/`
- `https://kiro.dev/docs/kiro-for-web/get-started/`

## Server Validation

Server: `154.219.122.197`

The probe confirmed the current server credential can call:

| Route | Model | Result |
| --- | --- | --- |
| Web Portal `StreamSendMessage` | `claude-opus-4.7` | HTTP 200, event stream returned `OK` |
| Web Portal `StreamSendMessage` | `claude-opus-4.6` | HTTP 200, event stream returned `OK` |
| Web Portal `StreamSendMessage` | `claude-sonnet-4.6` | HTTP 200, event stream returned `OK` |
| Web Portal `StreamSendMessage` | `qwen3-coder-next` | HTTP 200, event stream returned `OK` |

The old Kiro API-key / CLI route remains limited for this account:

```text
deepseek-3.2
minimax-m2.5
minimax-m2.1
glm-5
qwen3-coder-next
```

Claude/Opus on that route still returns `INVALID_MODEL_ID`.

## Implementation Added

Added:

- `adapters/kiro-web/kiro_web_adapter.py`
- `adapters/kiro-web/Dockerfile`
- `adapters/kiro-web/README.md`
- `tools/kiro_web_adapter_smoke.py`
- `tools/kiro_web_adapter_debug.py`

Server deployment added an internal Docker service:

```text
sub2api-kiro-web-adapter
image: sub2api-kiro-web-adapter:20260521
internal URL: http://kiro-web-adapter:8991
```

It is not exposed through Caddy. Public clients should still enter through
Sub2API at `https://api.vyywcw.cn/`.

## Bugs Found During Adapter Work

1. Missing `profileArn` in Web Portal CBOR body caused `CreateSpace` 401.

   Fix: include `profileArn` in portal operation bodies when available.

2. The adapter initially accepted `BuilderId` because `GetUserInfo` returned a
   stale response, then used a mismatched CSRF session.

   Fix: use the IdP that returns an active user session, and do not override it
   with the index page meta value.

3. The first CBOR map decoder accidentally reversed key/value order because
   Python evaluates the right-hand side of assignment before the subscript
   target.

   Fix: read `key` and `value` into variables before assigning.

4. SSE smoke tests waited for EOF because the adapter sent `Connection:
   keep-alive`.

   Fix: use `Connection: close` for adapter SSE responses.

## Current Smoke Test Results

On the deployed adapter:

| Endpoint | Model | Stream | Result |
| --- | --- | --- | --- |
| `/v1/messages` | `claude-opus-4.7` | no | HTTP 200, returned `adapter-ok` |
| `/v1/messages` | `claude-sonnet-4.6` | no | HTTP 200, returned `sonnet-adapter-ok` |
| `/v1/chat/completions` | `claude-sonnet-4.6` | no | HTTP 200, returned `openai-sonnet-ok` |
| `/v1/chat/completions` | `claude-sonnet-4.6` | yes | HTTP 200, returned `stream-openai-ok` |
| `/v1/messages` | `claude-sonnet-4.6` | yes | HTTP 200, returned `stream-anthropic-ok` |

`claude-opus-4.7` OpenAI-compatible non-stream returned a Kiro upstream
"high volume of traffic" message once. That is model load from Kiro, not an
adapter auth/protocol failure.

## Public Sub2API Wiring Completed

Completed on 2026-05-21:

- Added Sub2API account `kiro-web-internal-openai`.
- Account type is `platform=openai`, `type=apikey`.
- Internal base URL is `http://kiro-web-adapter:8991`.
- Account is bound to the existing `provider-mixed` group.
- Account extra forces Chat Completions passthrough with
  `openai_responses_mode=force_chat_completions`.
- `provider-mixed` model routing now includes the Kiro Claude model names.

Public validation through `https://api.vyywcw.cn/`:

| Endpoint | Model | Result |
| --- | --- | --- |
| `/v1/chat/completions` | `claude-sonnet-4.6` | HTTP 200, expected text returned |
| `/v1/chat/completions` | `claude-opus-4.7` | HTTP 200, expected text returned |
| `/v1/chat/completions` streaming | `claude-sonnet-4.6` | HTTP 200, SSE text returned |

Windsurf public validation remains on the Anthropic-compatible path:

| Endpoint | Model | Result |
| --- | --- | --- |
| `/v1/messages` | `claude-sonnet-4.6` | HTTP 200, expected text returned |

Operational details are in:

- `planning/SUB2API_KIRO_WEB_WIRING_2026-05-21.md`

## How To Wire Into Sub2API

Recommended short-term wiring:

1. Add or edit a Sub2API upstream account/channel.
2. Use internal base URL:

   ```text
   http://kiro-web-adapter:8991
   ```

3. Use the same internal Kiro adapter key stored on the server.
4. Add model names such as:

   ```text
   claude-opus-4.7
   claude-opus-4.6
   claude-sonnet-4.6
   claude-opus-4.5
   claude-sonnet-4.5
   claude-haiku-4.5
   ```

5. Put those models into a Kiro-specific group or model route in Sub2API.

Keep this adapter internal. Do not add a Caddy public route for it unless there
is a separate authentication and rate-limit plan.

## Next Improvements

- Add a real Sub2API channel/account seed script so Kiro Web models can be
  created from the server without manual UI work.
- Add upstream error normalization for Kiro "high volume of traffic" messages.
- Support richer Anthropic requests: tool use, tool results, images, and
  request metadata.
- Reuse a session for follow-up turns when the client provides a stable
  conversation/session id.
- Periodically re-check the reference projects. If `opencode-kiro-auth`,
  `pi-kiro`, `kiro.rs`, or `kiro-gateway` adds a first-party Web Portal adapter,
  compare their request shape against this implementation.
