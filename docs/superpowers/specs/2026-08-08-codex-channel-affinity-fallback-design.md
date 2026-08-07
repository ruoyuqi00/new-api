# Codex Channel Affinity Fallback Design

## Context

Recent production aggregates for the affected downstream `gptpro` group show that requests
with a channel-affinity key have a much higher cached-token ratio than requests
without one. The default Codex affinity rule currently accepts only the request
body's `prompt_cache_key`. When that field is absent or empty, the request is
distributed among the group's three channels and no affinity mapping is recorded.

The incoming Codex `Session_id` header is already an approved passthrough header and
is stable enough to use as an internal routing fallback. It must not be copied into
or substituted for `prompt_cache_key` in the upstream JSON body.

## Goals

- Keep requests with the same `prompt_cache_key` on the same channel exactly as
  today.
- When `prompt_cache_key` is absent, use a non-empty `Session_id` header only as the
  internal channel-affinity key.
- Preserve the upstream JSON body byte-for-byte through affinity selection.
- Preserve existing channel failover behavior and existing billing based on actual
  upstream `cached_tokens`.
- Improve cache locality for the observed missing-key request subset without
  changing model input or provider account selection logic.

## Non-Goals

- Do not generate, inject, replace, or normalize `prompt_cache_key`.
- Do not hash prompts or store request bodies as affinity keys.
- Do not change pricing, cached-token ratios, settlement, refunds, or database
  schemas.
- Do not change account-pool selection; the affected production channels currently
  use one key each.
- Do not expand the default one-hour affinity TTL without verified downstream cache
  retention data.
- Do not add unrelated model patterns such as `codex-auto-review` in this patch.
- Do not change production configuration or traffic.

## Considered Approaches

### 1. Add `Session_id` as the second key source (selected)

The existing rule engine checks key sources in order. Keeping `prompt_cache_key`
first and adding request header `Session_id` second preserves current behavior and
uses the fallback only for the affected missing-key subset. This reuses existing
cache storage, channel selection, failover, logging, and TTL behavior.

### 2. Inject `Session_id` into `prompt_cache_key`

This might influence an upstream provider's own cache directly, but it changes the
client request and could alter provider semantics or cache accounting. It is
rejected.

### 3. Pin the entire group to one channel

This could increase cache locality but would sacrifice load distribution and fault
tolerance for all users. It is rejected.

## Detailed Design

The default `codex cli trace` rule uses these key sources in order:

1. body `prompt_cache_key` via `gjson`;
2. request header `Session_id`.

The first non-empty value wins. Existing cache keys continue to include the rule
name and using group. The fallback therefore cannot cross user groups. The value is
used only to derive the existing affinity cache key and its fingerprint; the raw
value is not persisted in request logs by this change.

The default TTL remains 3600 seconds. Increasing it may help across longer sessions,
but doing so without confirmed downstream retention would extend stale channel
pinning without evidence that the provider cache survives that long.

`SkipRetryOnFailure` remains false. A stale or unhealthy preferred channel can use
the established failover path, and the successful channel can replace the affinity
mapping according to `SwitchOnSuccess`.

## Billing Invariant

Affinity selection does not alter billing inputs. YuAPI continues to charge from
the usage returned by upstream. If improved routing produces more legitimate
`cached_tokens`, the existing cached-token formula applies; if upstream reports no
cache hit, normal input-token charging applies. There is no synthetic cache credit.

## Testing

Tests use `testify/require` and `testify/assert` and cover:

- `prompt_cache_key` remains higher priority than `Session_id`;
- missing `prompt_cache_key` plus a non-empty `Session_id` records and retrieves the
  same channel affinity;
- missing both values creates no affinity context;
- the request JSON before and after affinity selection is identical;
- group separation remains part of the cache key;
- default retry and one-hour TTL behavior remain unchanged.

Focused service tests run first, followed by the repository backend test suite.

## Rollout Boundary

This design produces a separate local commit from the stream fix. It is evaluated
in a local candidate without changing production. Deployment requires explicit
user confirmation and the current production image remains the rollback baseline.
