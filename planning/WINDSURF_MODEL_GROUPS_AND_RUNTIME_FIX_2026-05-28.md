# Windsurf Model Groups And Runtime Fix

Date: 2026-05-28

This note records the live `api.vyywcw.cn` Windsurf model-family split, public
Sub2API calling pattern, and the `WindsurfAPI v2.0.97` runtime hotfix applied
after the split.

Do not put provider tokens, public API keys, SSH passwords, account passwords,
or `.env` values in this file.

## Live Shape

Public entry:

- Base URL: `https://api.vyywcw.cn`
- Anthropic Messages endpoint: `https://api.vyywcw.cn/v1/messages`
- Models endpoint: `https://api.vyywcw.cn/v1/models`

Internal services:

- Sub2API image: `sub2api-provider-adapters:6611e027`
- Windsurf adapter image: `sub2api-windsurf-api-acctfix:20260528`
- Windsurf upstream base: `dwgx/WindsurfAPI v2.0.97`
- Windsurf upstream revision:
  `41a36b9176633a9e67eb7ca87d725b5eb98564b8`

The Windsurf adapter is still internal-only. External clients call Sub2API
only.

## Model-Family Groups

Sub2API has one group, one channel, and one API key per Windsurf model family:

| Group | Public model aliases |
| --- | --- |
| `windsurf-opus4.6` | `opus4.6`, `opus4.6-thinking` |
| `windsurf-opus4.7` | `opus4.7`, `opus4.7-low`, `opus4.7-medium`, `opus4.7-high`, `opus4.7-xhigh`, `opus4.7-max`, `opus4.7-medium-thinking`, `opus4.7-high-thinking`, `opus4.7-xhigh-thinking`, `opus4.7-low-fast`, `opus4.7-medium-fast`, `opus4.7-high-fast`, `opus4.7-xhigh-fast`, `opus4.7-max-fast` |
| `windsurf-gpt5.5` | `gpt5.5`, `gpt5.5-none`, `gpt5.5-low`, `gpt5.5-medium`, `gpt5.5-high`, `gpt5.5-xhigh`, `gpt5.5-none-fast`, `gpt5.5-low-fast`, `gpt5.5-medium-fast`, `gpt5.5-high-fast`, `gpt5.5-xhigh-fast` |
| `windsurf-gpt5.4` | `gpt5.4`, `gpt5.4-none`, `gpt5.4-low`, `gpt5.4-medium`, `gpt5.4-high`, `gpt5.4-xhigh` |
| `windsurf-grok` | `grok`, `grok3`, `grok3-mini-thinking`, `grok-code-fast` |

The API keys are stored on the live server in a root-only file:

```text
/root/sub2api-windsurf-model-keys-20260528.txt
```

That file is intentionally not committed to Git.

## Routing Rules

The implementation uses both soft routing and hard model restrictions:

- `model_routing` chooses the preferred upstream account/channel.
- `channels.restrict_models = true` enforces the allowed model list.
- `channel_model_pricing.models` contains the allowed public aliases.
- `channels.billing_model_source = requested` makes the restriction check use
  the short public alias requested by the caller.
- `channels.model_mapping` maps short public aliases to Windsurf upstream model
  names.

This matters because `model_routing` alone is not a hard authorization boundary.
The channel model restriction is what prevents a key from calling another
model-family group.

## Public Calling Example

Use the API key for the target model-family group.

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H "Authorization: Bearer <windsurf-opus4.6-api-key>" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "opus4.6",
    "max_tokens": 64,
    "metadata": {
      "user_id": "stable-user-or-conversation-id"
    },
    "messages": [
      {"role": "user", "content": "Return exactly OK."}
    ]
  }'
```

For multi-turn clients, keep sending the full `messages` history and keep
`metadata.user_id` stable for the same conversation. The adapter cache/reuse
layer can reduce upstream work, but it does not replace client-side
conversation history.

## Verification

Public `/v1/models` was checked for each API key:

- `windsurf-opus4.6` showed 2 aliases.
- `windsurf-opus4.7` showed 14 aliases.
- `windsurf-gpt5.5` showed 11 aliases.
- `windsurf-gpt5.4` showed 6 aliases.
- `windsurf-grok` showed 4 aliases.

Allowed public calls after the runtime fix:

| Key group | Model | Result |
| --- | --- | --- |
| `windsurf-opus4.6` | `opus4.6` | HTTP 200, text `OK` |
| `windsurf-gpt5.5` | `gpt5.5-low` | HTTP 200, text `OK` |
| `windsurf-gpt5.4` | `gpt5.4-low` | HTTP 200, text `OKOK` |
| `windsurf-grok` | `grok` | HTTP 200, text `OK` |

`windsurf-opus4.7` with `opus4.7-low` returned HTTP 429 during this check:

```text
Upstream rate limit exceeded, please retry later
```

That was an upstream rate-limit response, not a Sub2API routing failure and not
the `acct is not defined` runtime bug.

Cross-group restriction was checked by using the `windsurf-opus4.6` key to call
`gpt5.5-low`; Sub2API rejected it:

```text
No available accounts supporting model: gpt5.5-low (channel pricing restriction)
```

## WindsurfAPI Runtime Hotfix

Observed issue before the fix:

- Windsurf upstream generated the model response.
- WindsurfAPI then returned HTTP 502.
- Logs showed `Chat error: acct is not defined`.
- Sub2API surfaced the adapter error to the public caller.

Root cause:

- `nonStreamResponse()` used `acct` while `acct` was only defined in the outer
  account-selection scope.
- The failing line was in the sticky session success path after Cascade returned
  text and usage.

Server fix:

- Backed up the container file as:
  `/app/src/handlers/chat.js.bak-20260528-acctfix`
- Added `accountId: acct.id` to the `poolCtx` object passed into
  `nonStreamResponse()`.
- Replaced the sticky binding write to use `poolCtx.accountId` and
  `poolCtx.apiKey`.
- Persisted the fix as a local server build context:
  `/opt/sub2api/windsurf-api-acctfix`
- Rebuilt and recreated `windsurf-api` as:
  `sub2api-windsurf-api-acctfix:20260528`

Repository copy:

- `provider-patches/windsurf-api/acct-scope-hotfix-20260528/`

## Operational Commands

Check live services:

```bash
cd /opt/sub2api
docker compose ps
```

Rebuild only the patched Windsurf adapter:

```bash
cd /opt/sub2api
docker compose build windsurf-api
docker compose up -d --no-deps --force-recreate windsurf-api
```

Confirm the patch is present:

```bash
cd /opt/sub2api
docker compose exec -T windsurf-api sh -lc \
  'grep -n "accountId: acct.id\\|poolCtx.accountId" /app/src/handlers/chat.js'
```

View recent adapter errors:

```bash
cd /opt/sub2api
docker compose logs --tail=200 windsurf-api
```

## Upstream Follow-Up

Track upstream:

- `https://github.com/dwgx/WindsurfAPI`

When upstream moves past `41a36b9`, inspect `src/handlers/chat.js` before
updating the server. If upstream has fixed the `acct` scope issue, remove the
local hotfix and switch the compose service back to the upstream image.

If upstream has not fixed it, keep rebuilding from the local hotfix context
after pulling the latest base image, then re-run the smoke tests above.
