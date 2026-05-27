# Windsurf Cache And Sub2API Calling Notes

Date: 2026-05-27

This note records the live `api.vyywcw.cn` Windsurf update, cache diagnosis,
and the public calling pattern through Sub2API.

Do not put provider tokens, public API keys, SSH passwords, or `.env` values in
this file.

## Current Server State

Live deployment:

- Public entry: `https://api.vyywcw.cn/`
- Sub2API container image: `sub2api-provider-adapters:c555fba9`
- Windsurf adapter: `ghcr.io/dwgx/windsurf-api:latest`
- WindsurfAPI version after update: `2.0.97`
- WindsurfAPI revision after update:
  `41a36b9176633a9e67eb7ca87d725b5eb98564b8`
- Compose backup before this change:
  `/opt/sub2api/docker-compose.yml.bak.20260527-154627`

The adapter remains internal-only. It is not published as a raw host port.
External clients should call Sub2API only.

## Why Cache Was Missing

Before the update, the running WindsurfAPI container was `2.0.96`.

Observed live health before update:

- `conversationPool.hitRate = 0.0%`
- `conversationPool.misses = 156`
- `cache.hitRate = 0.0%`
- recent logs showed large histories, `reuse MISS`, history trimming, and a
  provider timeout for `claude-opus-4.6`.

The relevant upstream update is `dwgx/WindsurfAPI v2.0.97`, especially commit
`f8db932 feat: Cascade reuse optimization + HTTPS proxy + configurable pool size`.
It adds caller-level fallback reuse and makes single-user Claude Code style
sessions much less sensitive to fingerprint drift.

## Server Cache Settings

The live `windsurf-api` compose service now has:

```yaml
- CASCADE_REUSE_BY_CALLER=1
- CASCADE_POOL_MAX=5
- CASCADE_REUSE_HASH_SYSTEM=0
```

Reasoning:

- `CASCADE_REUSE_BY_CALLER=1` lets WindsurfAPI reuse the most recent Cascade
  for the same caller and model when the strict fingerprint misses.
- `CASCADE_POOL_MAX=5` is enough for a single-admin deployment and avoids a
  huge stale pool.
- `CASCADE_REUSE_HASH_SYSTEM=0` reduces drift from dynamic Claude Code system
  prompts. This is suitable for this private/single-admin usage. Do not use
  this blindly for a public shared relay unless caller isolation is carefully
  designed.

Post-update health confirmed:

- `version = 2.0.97`
- `conversationPool.maxSize = 5`
- `conversationPool.reuseByCaller = true`

## Sub2API Routing

The existing `provider-mixed` group keeps normal Claude model names routed to
Kiro adapters first, so existing working Kiro behavior is not disturbed.

To force Windsurf through Sub2API, use the new `ws-` aliases:

- `ws-claude-sonnet-4.6` -> Windsurf `claude-sonnet-4.6`
- `ws-claude-sonnet-4.6-thinking` -> Windsurf `claude-sonnet-4.6-thinking`
- `ws-claude-opus-4.6` -> Windsurf `claude-opus-4.6`
- `ws-claude-opus-4.6-thinking` -> Windsurf `claude-opus-4.6-thinking`
- `ws-gemini-2.5-flash` -> Windsurf `gemini-2.5-flash`
- `ws-gpt-5.1` -> Windsurf `gpt-5.1`
- `ws-gpt-5.2` -> Windsurf `gpt-5.2`

These aliases are configured in:

- account `windsurf-internal-anthropic`
- group `provider-mixed` model routing

After direct DB edits, Sub2API was restarted to refresh the scheduler snapshot.
Future changes should preferably go through the admin API/UI or explicitly
refresh/restart Sub2API so scheduler caches see the update.

## Public Calling Example

Use the existing Sub2API public API key from the `provider-mixed` group. Do not
call `windsurf-api` directly from outside.

Anthropic Messages:

```bash
curl -sS https://api.vyywcw.cn/v1/messages \
  -H "authorization: Bearer <your-sub2api-api-key>" \
  -H "content-type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "ws-claude-sonnet-4.6",
    "max_tokens": 256,
    "metadata": {
      "user_id": "my-stable-session-id"
    },
    "messages": [
      {"role": "user", "content": "hello"}
    ],
    "stream": false
  }'
```

For multi-turn context, keep sending the full `messages` history and keep
`metadata.user_id` stable for the same conversation:

```json
{
  "model": "ws-claude-sonnet-4.6",
  "metadata": {"user_id": "project-a-session-001"},
  "messages": [
    {"role": "user", "content": "first question"},
    {"role": "assistant", "content": "first answer"},
    {"role": "user", "content": "follow-up question"}
  ]
}
```

The model does not remember context by API key alone. The client must keep
sending the conversation history. The cache/reuse layer reduces upload and
provider work; it is not a replacement for client-side conversation state.

## Verification Results

Direct internal WindsurfAPI smoke:

- `claude-sonnet-4.6` returned HTTP 200 and text `ws-ok`.

Public Sub2API smoke:

- `ws-claude-sonnet-4.6` first turn returned HTTP 200 and text
  `ws-public-one`.
- `ws-claude-sonnet-4.6` second turn returned HTTP 200 and text
  `ws-public-two`.
- Second turn usage included `cache_read_input_tokens = 1935`, proving the
  Windsurf-side cache/reuse path was hit.
- Windsurf health after public probe showed `conversationPool.hits = 1` and
  `hitRate = 33.3%`.
- `ws-claude-opus-4.6` returned HTTP 200 and text `ws-opus-ok`.

## Practical Client Guidance

For Claude Code or any client that uses Anthropic-compatible `/v1/messages`:

- Base URL: `https://api.vyywcw.cn`
- API key: Sub2API key bound to `provider-mixed`
- Model for Windsurf Sonnet: `ws-claude-sonnet-4.6`
- Model for Windsurf Opus: `ws-claude-opus-4.6`
- Keep one stable conversation/session id when the client supports metadata.
- If the client cannot set metadata, WindsurfAPI `2.0.97` still has caller
  fallback reuse, but isolation is weaker because Sub2API calls the adapter
  from the same internal service.

## Update Watch

Reference upstream:

- `https://github.com/dwgx/WindsurfAPI`

When checking for protocol/cache updates, inspect:

- `README.md`
- `.env.example`
- `src/conversation-pool.js`
- `src/caller-key.js`
- `src/handlers/messages.js`
- `src/handlers/chat.js`
- `src/models.js`
- `src/dashboard/windsurf-login.js`

Follow-up actions after future upstream updates:

1. Fetch local research checkout:
   `git -C D:\wflogin\_github_research\WindsurfAPI fetch --all --tags --prune`
2. Compare latest tag/revision against the deployed container label.
3. Read changes in the files above.
4. Pull/recreate `windsurf-api` on the server only if protocol, auth, model, or
   cache changes are relevant.
5. Re-run direct internal smoke and public Sub2API smoke.
