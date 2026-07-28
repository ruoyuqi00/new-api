# Parent Routing Reliability Design

## Goal

Port the parent project's reliability fixes that improve channel selection,
private-group model discovery, streaming cleanup, task consistency, and proxy
transport behavior without importing its unrelated UI, billing, authentication,
or protocol-conversion rewrites.

## Scope

This work selectively ports and adapts these parent changes:

- `4aa08f917`: derive auto-group model listings from available groups.
- `e13d4033e`: normalize channel proxy URLs and invalidate cached HTTP clients
  when channel connection settings change.
- `153d7f01a`: prevent stale writes after a downstream stream disconnects.
- `4a188deea`: expose and enforce the behavior for clearing channel affinity
  after a preferred channel is disabled.
- `933ea0cdd`: make relay idle connection timeout configurable.
- `d2dcbc313`: let channels configure asynchronous task polling delay.
- `e0d515611`: make async task refund transitions compare-and-set so duplicate
  pollers cannot refund a task twice.

The existing local fixes for upstream 401 handling, account-pool cooldown,
channel retry, and stale affinity failover remain authoritative. In particular,
we will not port parent commit `5fe8e98ee`, because its default
`SkipRetryOnFailure=true` for Codex/Claude affinity conflicts with the required
behavior: an unhealthy preferred channel must permit selection of another
eligible channel.

## Non-goals

- Do not merge or rebase `origin/main`.
- Do not port `c36418c86` or its large protocol conversion rewrite.
- Do not port the parent routing-reliability UI wholesale. The existing default
  UI receives only the controls required by the imported runtime settings.
- Do not alter account-pool bindings, provider-account credentials, channel
  groups, pricing, private-group grants, or production data.
- Do not introduce cross-group fallback. A private group may only route to its
  own eligible, enabled channels.

## Design

### Channel Selection And Affinity

The retry loop continues to clear the current affinity cache entry before it
selects a retry channel. The affinity setting gets an explicit
`KeepOnChannelDisabled` switch. Its default remains `false`, so disabling or
failing a preferred channel clears its affinity entry and allows the request to
choose another eligible channel in the same group.

The implementation must preserve the current `SkipRetryOnFailure=false` rules
for Codex and Claude. The setting is an opt-in administrative override, not a
new default. Tests cover both disabled-channel behavior and a 5xx retry that
leaves the private group unchanged.

### Group Model Discovery

Auto-group model listing is derived through the same eligible-group path used
by routing. Disabled channels and private groups not granted to the caller do
not make models visible. The API must not advertise a model merely because a
disabled channel still lists it in its raw `models` field.

### Proxy And Idle Connections

Channel proxy URLs are normalized once and HTTP-client cache keys include the
effective proxy configuration. Updating a channel's proxy setting evicts its
old client, so later traffic cannot use a stale transport. Relay idle timeout
is read from an environment/config setting with the current behavior retained
when it is unset.

### Streaming And Tasks

Stream scanners stop writing after the downstream context is canceled and
preserve the existing `client_gone` accounting path. This must not turn a
client cancellation into a channel failure or automatic channel disable.

Async task polling accepts a channel-level delay toggle. State transitions that
apply a refund use a conditional update, so only the first successful poller
can refund a task. Repeated polling observes the completed transition without
changing quota again.

## Delivery Order

1. Add regression tests and port the small group/affinity/stream behavior.
2. Port proxy cache invalidation and idle timeout with focused transport tests.
3. Port task polling delay and compare-and-set refunds with deterministic task
   tests.
4. Add the minimum default-UI controls and all six locale translations.
5. Run backend, frontend, and targeted routing/task tests; build a candidate
   image before any production deployment.

## Acceptance Criteria

- A 5xx or 401 provider-account failure retries an eligible account/channel and
  does not pin a request to the failed affinity channel.
- Private-group routing never chooses a channel outside the selected group.
- Auto groups list only models available through enabled, eligible groups.
- Changing a channel proxy cannot reuse a stale HTTP client.
- A downstream client disconnect produces no stale stream write and no channel
  auto-disable.
- Concurrent task pollers produce at most one refund.
- New settings are controllable from the existing default UI and translated in
  every supported locale.
- No local experimental UI files appear in commits, Docker context, or images.
