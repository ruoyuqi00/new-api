# Kiro Claude Code Compatibility - 2026-06-16

## Summary

Kiro Web is exposed through Sub2API as an Anthropic Messages-compatible group named `kiro`.

The current production route is:

```text
Claude Code
  -> https://api.vyywcw.cn/v1/messages
  -> Sub2API api key bound to group `kiro`
  -> account 210 `kiro-web-internal-anthropic`
  -> http://kiro-web-adapter:8991
```

## Production State

- Sub2API image: `sub2api-provider-adapters:kiro-counttokens-20260616b`
- Group `kiro`:
  - `platform = anthropic`
  - account: `210 / kiro-web-internal-anthropic`
- Kiro Web adapter:
  - container: `sub2api-kiro-web-adapter`
  - internal base URL: `http://kiro-web-adapter:8991`

## Fix Applied

Kiro Web adapter supports `/v1/messages`, but currently returns a generic 404 for `/v1/messages/count_tokens`.

Claude Code may call `POST /v1/messages/count_tokens` before real generation. Sub2API now detects this Kiro-specific generic 404 and returns a local Anthropic-compatible estimate:

```json
{"input_tokens": 25}
```

This fallback is limited to accounts that are clearly Kiro adapter accounts by name, base URL, or `extra.provider_adapter`.

## Verified

Production smoke tests through `https://api.vyywcw.cn`:

- `POST /v1/messages/count_tokens` -> HTTP 200 with `input_tokens`.
- `POST /v1/messages` non-streaming -> HTTP 200, routed to account 210.
- `POST /v1/messages` streaming -> HTTP 200 SSE, routed to account 210.

The current Kiro upstream response for the tested account was an overage text response. That proves protocol routing works, but does not mean the selected Kiro account has usable quota at that moment.

## Claude Code Configuration

On another computer, use the Anthropic-style endpoint variables:

```powershell
$env:ANTHROPIC_BASE_URL="https://api.vyywcw.cn"
$env:ANTHROPIC_AUTH_TOKEN="<Sub2API key bound to the kiro group>"
$env:ANTHROPIC_MODEL="claude-sonnet-4.6"
claude
```

Notes:

- Do not point Claude Code directly at a raw Kiro credential.
- Do not use a NewAPI key for this Kiro route unless NewAPI has been explicitly bridged to the Sub2API Kiro group.
- If Claude Code only supports `ANTHROPIC_API_KEY` in that installation, set it to the same Sub2API key value, but prefer `ANTHROPIC_AUTH_TOKEN`.

## Limitations

The Kiro Web adapter currently converts incoming `tool_use` and `tool_result` content blocks into text before sending them to Kiro. It does not yet implement a full Anthropic tool-use round trip where Kiro returns structured `tool_use` blocks and Claude Code executes tools from them.

Practical implication:

- Basic Claude Messages connectivity is working.
- Claude Code may connect and stream responses.
- Full Claude Code agent behavior can still be limited until Kiro Web tool-use parity is implemented.

