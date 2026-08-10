# Cache Affinity Stream Safety Design

## Goal

Prevent retryable failures and abnormal stream endings from destroying a working
channel-affinity mapping or recording a failed route as the new successful
mapping. Keep billing conservative and unchanged.

## Confirmed Failure Chain

1. A request starts on its affinity channel.
2. A retryable relay error deletes the existing mapping before the retry result
   is known.
3. Streaming responses can already have HTTP status 200 when the client leaves
   or the upstream stream ends abnormally.
4. The distributor records the last channel because it only checks HTTP status.
5. A later request is routed to a different channel and loses provider-side
   prompt-cache locality.
6. Abnormal streams often lack terminal usage, so zero cached tokens are also
   counted as a confirmed cache miss.

## Approaches Considered

### Recommended: commit affinity only after confirmed success

Keep the previous cache mapping during retries. Mark a request affinity-safe
only after the relay completes successfully and, for streams, the request is
still connected and the stream status has no errors. Make the affinity writer
require that marker. Record missing terminal usage from abnormal streams as
unknown instead of a miss.

This is the smallest change that fixes the proven failure chain and preserves
availability behavior.

### Alternative: disable retries for affinity requests

This preserves cache locality but turns recoverable upstream failures into
user-visible failures. It is rejected because availability would regress.

### Alternative: pin channel, provider account, and multi-key index

This can improve locality for account-pool and multi-key channels, but requires
availability-aware leases, cache-format migration, and production configuration
evidence. It is deferred from this patch because the historical GPT Pro layout
uses one direct channel per upstream account and the immediate defect is the
channel-affinity lifecycle.

## Design

### Affinity outcome

The relay controller marks the Gin context only when a request is safe to commit
as affinity success:

- non-streaming relay returned no error and the client context is active;
- streaming relay returned no error, the client context is active, and any
  available stream status is a normal end without recorded errors.

`RecordChannelAffinity` refuses to write without this marker. HTTP 200 alone is
not proof of success.

### Retry behavior

Retry preparation continues to exclude the failed channel or provider account,
but it no longer deletes the existing session mapping. If a retry completes
normally, the normal post-request write atomically replaces the old channel. If
all retries fail or the client disconnects, the old mapping remains available
for the next request.

### Cache usage statistics

Normal terminal usage continues to increment `total` and either hit or miss.
Abnormal streams and disconnected clients increment a separate `unknown`
counter and do not increment `total`. Missing terminal usage therefore does not
depress the confirmed hit rate.

### Billing boundary

No quota calculation, pre-consumption floor, settlement, refund, model price,
group ratio, or tiered-billing behavior changes. An accepted incomplete upstream
stream continues to preserve pre-consumed quota.

## Tests

- Affinity writes require a confirmed success marker.
- A retryable failure does not delete an existing mapping.
- A normal completed request can replace the old mapping.
- Client cancellation and abnormal stream status cannot replace the mapping.
- Unknown usage increments `unknown` but not the confirmed hit-rate denominator.
- Existing affinity, retry, Responses stream, quota, and controller tests remain
  green.

## Deployment Boundary

This patch is local-only. It does not modify Caddy, production containers,
production traffic, database schema, or production data. A local candidate must
pass tests and user verification before any production action is considered.
