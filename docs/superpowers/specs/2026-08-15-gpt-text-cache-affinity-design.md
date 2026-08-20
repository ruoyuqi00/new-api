# GPT Text Cache Affinity Design

**Date:** 2026-08-15

## Scope

Improve cache locality only for GPT text requests on `/v1/responses` and
`/v1/chat/completions`. Image, video, audio, task, Caddy, Cloudflare, billing,
database schema, and frontend visual/brand behavior are out of scope. The
existing default and classic affinity editors must preserve the backend-only
injection flag when a rule is edited.

## Evidence and root cause

The production-derived code already gives native Responses requests a stable,
token-scoped channel-affinity key and can inject a protected
`prompt_cache_key`. The same mechanism is not available to Chat Completions:

1. the built-in GPT affinity rule only matches `/v1/responses`;
2. `GetChannelAffinityPromptCacheKey` rejects every path except
   `/v1/responses`;
3. `TextHelper` does not inject a derived key into Chat requests; and
4. the Chat-to-Responses compatibility converter drops an existing
   `prompt_cache_key`.

This matches the observed production cohorts: native Responses requests with
affinity have materially better cache usage than Chat requests without
affinity.

## Behavior

The existing GPT rule covers both GPT text endpoints. It keeps the current key
priority: response/conversation context where applicable, explicit
`prompt_cache_key`, `Session_id`, then stable Codex turn metadata. An explicit
client `prompt_cache_key` is never replaced.

When the client omits `prompt_cache_key` but the selected rule has a stable
session source and explicitly enables injection, YuAPI derives a stable HMAC
key scoped by token, group, model, source identity, and source value. The raw
session value is not written to the upstream request, cache key, admin metadata,
or public response.

For direct Chat forwarding, the derived key is attached before provider
conversion. Raw pass-through bodies are materialized only when injection is
required, preserving unknown client fields. For Chat-to-Responses conversion,
the key and cache-retention option survive the conversion.

Requests without an explicit key or stable session source remain unchanged. We
do not hash the full mutable prompt and do not create a token-wide shared key.

## Security and failure behavior

Affinity data is routing input only. It must never expose the selected channel,
upstream account, upstream URL, authorization data, or raw session identifier.
Existing HTTP and SSE public-error projection remains mandatory and is covered
by regression tests.

Disconnect/retry behavior remains unchanged: an accepted or ambiguous upstream
request is not automatically replayed on another channel. A client retry that
preserves the stable session signal derives the same protected key.

## Configuration and rollout

Persisted custom affinity settings are not silently rewritten. The candidate is
verified locally first. Production rollout must explicitly update the existing
GPT rule to include `/v1/chat/completions`, then use a private-port candidate and
health checks before a Caddy switch. The prior production image/container stays
available for immediate rollback.
