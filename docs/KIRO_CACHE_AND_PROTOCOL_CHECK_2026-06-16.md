# Kiro cache and protocol check - 2026-06-16

## Summary

Kiro accounts that return quota/overage/rate-limit style text are still treated
as callable accounts. The Kiro Web adapter should not disable those accounts or
remove them from routing. Only hard authentication failures during token refresh
are treated as dead credentials.

This pass added server-side account routing state for the Kiro Web adapter:

- default true round-robin across enabled credentials;
- model-aware credential filtering when a credential declares model metadata;
- session affinity for stable conversation hints;
- a non-secret runtime state file at `/config/kiro-runtime-state.json`;
- `claude-opus-4.8` model mapping and metadata.

The runtime state file stores only rotation cursors, hashed affinity keys, and
sanitized credential IDs. It does not store access tokens, refresh tokens, API
keys, or raw session IDs.

## Upstream/reference check

Sources checked:

- `https://github.com/Wei-Shaw/sub2api`
- `https://github.com/chaogei/Kiro-account-manager`
- `https://github.com/Quorinex/Kiro-Go`
- `https://github.com/Jwadow/kiro-gateway`
- `https://github.com/hank9999/kiro.rs`

Findings:

- Sub2API upstream advanced from `e34ad2b1` to `f069c9ae` after the previous
  2026-06-15 check. The new commits are useful gateway/core fixes, but they are
  not required for this Kiro cache patch and were not merged into this deploy.
  A normal tree diff against upstream also shows that upstream does not contain
  our private adapter/admin/docs files, so a broad merge should stay a separate
  reviewed operation.
- `Kiro-account-manager` latest main stayed at `447adcdb`. Its relevant ideas
  are true `round-robin`, optional `sticky` routing, `conversation_id` session
  affinity, prompt cache accounting, thinking support, and tool XML leak fixes.
  This patch absorbed only the routing/cache-safe pieces that match our current
  Kiro Web Portal adapter.
- `Kiro-Go` main stayed at `a2e3971`; no newer remote head was found.
- `kiro-gateway` main stayed at `a5292ca`; no newer remote head was found.
- `kiro.rs` latest main is `b9e757e`, notably adding Claude Opus 4.8 support.
  The adapter now advertises and maps `claude-opus-4.8`.

Protocol note: our Kiro Web adapter is still a text-first Web Portal bridge.
It does not yet claim full Anthropic `tool_use`, image input, or thinking block
round-trip parity. The reference projects have richer tool/thinking handling,
but that should be a dedicated protocol implementation and test pass.

## Code changes

Changed files:

- `adapters/kiro-web/kiro_web_adapter.py`
- `adapters/kiro-web/README.md`

New environment knobs:

- `KIRO_RUNTIME_STATE_FILE`, default `/config/kiro-runtime-state.json`
- `KIRO_ACCOUNT_SELECTION_STRATEGY`, default `round-robin`
- `KIRO_SESSION_AFFINITY_ENABLED`, default `true`
- `KIRO_SESSION_AFFINITY_TTL_SECONDS`, default `3600`

Supported affinity hints:

- headers: `x-session-affinity`, `x-claude-code-session-id`,
  `x-opencode-session`, `x-conversation-id`, `x-thread-id`
- body: `conversation_id`, `conversationId`, `thread_id`, `threadId`,
  `session_id`, `sessionId`, `prompt_cache_key`, `promptCacheKey`, `user`
- metadata: the same body keys plus `user_id` and `userId`

## Deployment

Server:

- host: `154.219.122.197`
- stack path: `/opt/sub2api`
- service restarted: `kiro-web-adapter` only
- services not restarted: `sub2api`, NewAPI, database, Redis, Caddy

Backup created before replacing the adapter:

```text
/opt/sub2api-backups/kiro_web_adapter.py.before-cache-20260616-1533
```

Deploy command shape:

```bash
cd /opt/sub2api
docker compose build kiro-web-adapter
docker compose up -d kiro-web-adapter
```

## Validation

Local:

```text
python -m py_compile adapters/kiro-web/kiro_web_adapter.py -> OK
git diff --check -> OK
selector test -> round_robin ["a","b","c","a"], sticky ["b","b","b"]
```

Server:

```text
kiro-web-adapter /health inside container -> {"status":"ok"}
public https://api.vyywcw.cn/health -> {"status":"ok"}
docker compose ps -> sub2api stayed healthy, kiro-web-adapter recreated only
/config/kiro-runtime-state.json created
```

Smoke results:

```text
claude-sonnet-4.6 Anthropic-compatible direct adapter call -> HTTP 200 with upstream "Too many requests" text
claude-sonnet-4.6 OpenAI-compatible direct adapter call -> HTTP 200, "adapter-ok"
claude-opus-4.8 Anthropic-compatible direct adapter call -> HTTP 200, "adapter-ok"
```

The "Too many requests" response is not treated as dead account evidence. It is
an upstream runtime/rate-limit message returned through a live authenticated
account.

## Sub2API bridge follow-up

On the same day, the Kiro adapter was checked from the Sub2API side.

Findings:

- Sub2API can reach `http://kiro-web-adapter:8991/health` on the shared Docker
  network.
- Kiro upstream accounts existed in Sub2API, but they were attached to the
  mixed `GPT5.5` group together with many normal OpenAI/CPA accounts.
- A direct `GPT5.5` group request for `claude-opus-4.8` initially failed with
  HTTP 502 because Sub2API selected ordinary OpenAI accounts first and those
  accounts returned upstream auth failures for the Claude model.

Operational correction:

- Added `claude-opus-4.8` and `claude-opus-4-8` to the Kiro Web account model
  mappings for account IDs `3` and `210`.
- Enabled a `claude-*` model routing rule on `GPT5.5` as a compatibility guard.
- Created a dedicated Sub2API group named `kiro`.
- Attached only account ID `3` (`kiro-web-internal-openai`) to that group.
- Created a dedicated Sub2API API key named `kiro-dedicated-20260616`.

Validation:

```text
Sub2API /v1/chat/completions
model: claude-opus-4.8
group/key: kiro-dedicated-20260616
result: HTTP 200, assistant text "sub2api-ok"
```

Recommendation:

- Use the dedicated `kiro` Sub2API key/group for Kiro Claude models.
- Do not rely on the mixed `GPT5.5` key for Claude/Kiro calls unless the group
  routing behavior has been revalidated after a Sub2API restart/cache refresh.
