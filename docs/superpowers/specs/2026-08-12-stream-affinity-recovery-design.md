# Stream Affinity Recovery Design

## Context

Production relays long-running OpenAI Responses streams through the gateway. A stream can end before the provider's terminal event because the downstream client disconnects, the upstream body ends early, an idle timeout fires, or a local write fails. Those cases currently have two costly side effects:

- an upstream `response.created` ID is only committed to response-chain channel affinity after a successful terminal event, so a later request using `previous_response_id` may be routed to another channel and lose provider-side cache locality;
- incomplete streams can fall back to locally estimated usage, while the provider may already have billed the request and may not have returned authoritative cache-token details.

The fix must preserve the production UI and database, never expose upstream infrastructure to clients, never refund a request that may already have been billed upstream, and never resubmit an ambiguous request merely to obtain usage.

## Goals

1. Reduce avoidable cache loss after an interrupted Responses stream.
2. Preserve the selected channel for response chains and other configured affinity keys.
3. Give clients a stable, sanitized terminal error when the downstream connection still exists.
4. Keep billing conservative when terminal usage is absent.
5. Improve diagnostics enough to distinguish pre-response latency from mid-stream interruption without exposing secrets.

## Non-goals

- No Cloudflare, Caddy, DNS, production-container, schema, UI, or pricing-expression changes.
- No automatic refund for an incomplete or ambiguous stream.
- No automatic POST retry after the request may have reached the upstream.
- No fabricated cache-token usage.
- No universal provider result-retrieval implementation in this change. Provider-specific GET reconciliation can be added later only for providers that prove the capability.

## Considered Approaches

### A. Cost-aware affinity and conservative settlement (selected)

Record a provisional response-chain affinity as soon as a real upstream response ID is observed, preserve affinity across an incomplete stream, suppress unsafe retries, emit a sanitized terminal error when possible, and floor settlement at the frozen pre-consumed quota when authoritative terminal usage is absent.

This prioritizes billing safety and cache continuity. A client may receive a retryable error instead of being silently moved to another channel, but it will not trigger an unbounded duplicate upstream charge.

### B. Availability-first cross-channel retry

Retry another channel whenever the first stream fails before a useful answer is delivered. This can improve apparent success rate, but it can duplicate provider charges and guarantees loss of provider-local cache when the channel changes. It conflicts with the billing constraint and is rejected.

### C. Detached background drain

Keep reading the upstream after the downstream client disconnects in order to recover terminal usage. This can improve accounting completeness, but it intentionally continues a potentially expensive generation after the user has gone and complicates resource limits. It is rejected as the default. A provider-specific, bounded GET reconciliation path is safer and remains future work.

## Design

### 1. Stream outcome classification

The existing stream end reason remains the primary transport classification:

- `done`: explicit protocol completion;
- `client_gone`: downstream request context canceled;
- `handler_stop` or `ping_fail`: local downstream write failure;
- `timeout`, `scanner_error`, `eof`, or `panic`: upstream/local incomplete stream unless an explicit terminal event was already observed.

For Responses streams, terminal protocol events remain authoritative. Bare EOF is not a successful terminal result when terminal markers are required.

All incomplete outcomes after a request may have reached upstream are non-retryable inside the gateway. They do not disable or cool down a channel solely because the downstream disappeared, and they do not clear affinity.

### 2. Provisional response-chain affinity

When the upstream sends a real Responses ID in `response.created` or another valid response envelope, the relay records it immediately in `RelayInfo`, independent of terminal success.

The controller commits affinity in two modes:

- normal affinity after a successful terminal event;
- provisional response-chain affinity after an incomplete stream when a real upstream response ID was observed.

Provisional affinity binds only the authenticated token, group, model, and response ID to the selected channel. It must not invent an ID from the local request ID. It uses a shorter bounded TTL than normal affinity, capped at 15 minutes, so interrupted response chains retain cache locality without leaving stale routing indefinitely. A later successful request refreshes normal affinity using the configured TTL.

The existing primary affinity key (`prompt_cache_key`, conversation, or configured rule source) is preserved and never cleared merely because the selected channel is temporarily cooling down.

### 3. Retry and channel switching policy

The gateway does not retry or switch channels when any of these is true:

- the downstream request context is canceled;
- upstream response headers or stream events have been received;
- a Responses stream ended without its required terminal event;
- the request was selected through a rule configured to skip retry on affinity failure.

Retries remain possible only for failures that are already proven safe by the existing request/retry policy, such as a local failure before the request is written or an authoritative rejection. This design does not broaden retry eligibility.

If an affinity channel is temporarily unavailable, its mapping remains intact. Current routing may use an existing fallback only when the affinity rule explicitly permits retry; strict response-chain rules return the existing sanitized availability error rather than silently changing providers.

### 4. Client-visible error contract

When the client connection is still writable and a Responses stream ends without a terminal event, the relay sends exactly one protocol-valid `response.failed` event containing only:

- a stable public code such as `upstream_stream_incomplete`;
- a generic Chinese or English message selected through the existing API convention;
- the gateway request ID or the already-public upstream response ID when it was previously emitted to that client.

The public response must never contain upstream URLs, domains, IP addresses, provider/channel names or IDs, API keys, authorization values, request headers, proxy destinations, redirects, raw upstream bodies, Go transport error text, SQL errors, or private upstream request IDs.

The internal log may retain an end-reason category and bounded, redacted diagnostic text. Secret-bearing strings and full URLs are never stored in public log fields.

If the downstream client is already gone, no additional write is attempted. The upstream request remains canceled through the existing request context.

### 5. Billing and usage

Authoritative terminal usage remains the only source for confirmed prompt, completion, and cached-token fields.

For an incomplete or client-gone Responses stream:

- set `PreservePreConsumedQuota` whenever the upstream may have accepted the request, including `client_gone` and local handler-stop outcomes;
- settle no lower than the frozen pre-consumed quota;
- do not refund;
- do not resubmit;
- do not report locally estimated cache tokens as confirmed usage;
- mark channel-affinity cache statistics as unknown rather than a cache miss.

Observed output tokens may remain an internal estimate for diagnostics, but they cannot reduce the frozen quota or create authoritative cache fields. Existing tiered pricing uses authoritative usage only; absent terminal usage therefore cannot undercut the frozen reservation.

### 6. Keepalive and first-byte behavior

SSE ping remains enabled where configured because it prevents idle intermediaries from closing a valid long-running connection. This change does not disable ping or increase first-byte timeout.

Because a pre-response ping can commit the downstream HTTP status, later upstream failures must use the sanitized SSE terminal error instead of attempting to replace the response with an HTTP error. Before any response bytes are committed, existing sanitized HTTP errors remain available.

### 7. Diagnostics

Add structured internal fields, without raw URLs or headers, for:

- whether upstream headers were received;
- whether any stream event was received;
- whether a real response ID was observed;
- terminal marker presence and success;
- stream end reason;
- whether pre-consumed quota was preserved;
- whether provisional affinity was recorded.

Existing first-response timing remains the primary latency signal. This scope does not add a database migration or store request/response bodies.

## Test Strategy

Deterministic Go regression tests will cover:

1. `response.created` followed by EOF records provisional `previous_response_id` affinity to the same channel.
2. Client cancellation after a real response ID preserves the pre-consumed quota and provisional affinity but attempts no terminal write.
3. Incomplete streams without a real upstream response ID never create a response-chain mapping.
4. A later successful terminal event upgrades/refreshes normal affinity.
5. Client-gone, handler-stop, EOF, timeout, and scanner errors never trigger an unsafe retry after upstream activity.
6. Missing terminal usage is classified unknown for cache statistics and cannot settle below frozen pre-consumption.
7. Public HTTP and SSE errors do not contain seeded upstream domains, IPs, channel identifiers, keys, authorization values, raw bodies, or transport error text.
8. Keepalive behavior remains compatible with terminal success and incomplete-stream failure events.

Focused package tests run first, followed by full tests for affected backend packages and `git diff --check`. No production or Caddy operation is part of verification.

## Acceptance Criteria

- Interrupted Responses chains with an observed upstream response ID select the same channel on the next `previous_response_id` request.
- Incomplete/ambiguous requests are never automatically refunded or resubmitted.
- Confirmed cache statistics use only authoritative terminal usage.
- Public errors contain no upstream infrastructure or credentials.
- Existing completed-stream billing, successful affinity, heartbeat, UI, and async media behavior remain unchanged.
- SQLite, MySQL, and PostgreSQL compatibility is preserved; this design requires no migration.
