# Unconfirmed GPT Text Usage Settlement Design

## Context

YuAPI reserves quota before a paid request so concurrent requests cannot spend
the same balance. A normal response is settled against authoritative upstream
usage and the reservation is refunded or supplemented by the difference.

For an upstream request that may have been accepted but ends without terminal
usage, the current implementation estimates prompt and observed completion
tokens and then floors the final charge at the full reservation. That
reservation may include a configured or default maximum output allowance.
Consequently, an interrupted request can be charged for output that was never
observed, and a request whose local token estimate is zero can still produce a
positive consumption record.

Read-only production aggregation over one recent hour found 3,305 positively
charged estimated-usage records. Of those, 3,254 were settled from the full
reservation and 42 had zero prompt and completion tokens. Most affected
requests used `/v1/responses`.

## Goals

- Keep authoritative upstream usage as the primary billing source.
- Continue bounded reading of the original accepted stream to recover terminal
  usage after the downstream disconnects.
- When terminal usage remains unavailable, charge only locally estimated input
  and output already observed from that same upstream request.
- Never create a positive finalized text-consumption record with zero estimated
  prompt and completion tokens.
- Never resubmit an accepted or ambiguous request merely to recover usage.
- Keep public errors and logs free of upstream infrastructure and credentials.

## Scope

This change applies only to GPT-compatible text processing through
`/v1/responses` and `/v1/chat/completions`, including the existing internal
Chat-to-Responses conversion path.

It does not change Claude Messages, images, video, audio, async tasks, local
sensitive-input fees, moderation fees, pricing configuration, database schema,
channel affinity, Caddy, Cloudflare, or frontend branding.

## Considered Approaches

### A. Exact usage, then token estimate (selected)

Recover authoritative terminal usage from the original stream when possible.
If recovery fails, settle from the request-side prompt estimate plus output
tokens observed before termination. The full pre-consumed quota is not a lower
bound for this fallback.

This removes maximum-output overcharging while retaining a deterministic charge
for work that demonstrably reached the upstream.

### B. Keep the full reservation

This best protects the gateway against an upstream that bills unseen work, but
it can charge users for an ungenerated maximum output allowance. Production
evidence confirms that this is not an acceptable user-facing contract.

### C. Refund every request without terminal usage

This is simple for users but can make YuAPI absorb charges for requests already
accepted and executed upstream. It is rejected.

## Settlement Rules

### Authoritative usage

When valid terminal usage is received, existing normal settlement remains
unchanged. Cache-read, cache-creation, tiered pricing, group ratio, and output
pricing use the authoritative fields.

### Estimated usage

When terminal usage cannot be recovered:

1. Prompt tokens come from the request-side estimate captured before relay.
2. Completion tokens come only from output observed on the original response.
3. Total tokens equal prompt plus completion tokens.
4. Cache token categories remain unknown and are never reported as confirmed
   hits or misses.
5. Normal ratio pricing uses those estimated prompt and completion tokens.
6. Tiered pricing evaluates the frozen expression and request snapshot with the
   estimated prompt and observed completion values. Cache categories remain
   zero because they are not authoritative.
7. The resulting estimated quota is the final quota. The full reservation is
   neither a floor nor a substitute for missing output.
8. The billing session refunds or supplements the difference between the
   temporary reservation and the estimated final quota.

If prompt and completion estimates are both zero, final quota is zero and no
positive consumption record may be written. The internal outcome remains
auditable as unconfirmed without fabricating token usage.

### Ambiguous submission

If the POST may have been written but no response headers arrived, the gateway
must not retry or switch channels automatically. It settles from the available
prompt estimate only; no output is assumed. A confirmed pre-write failure keeps
the existing refund behavior.

No gateway can guarantee exact cost when an upstream accepts work but never
returns per-request usage. This design deliberately limits the residual risk to
that unknowable interval instead of transferring a maximum-output estimate to
the user.

## Stream Recovery

The existing bounded recovery remains enabled: after an accepted stream loses
the downstream, YuAPI may continue reading that same upstream response within
the configured time, byte, global-concurrency, and per-channel limits. Recovery
never issues another POST and never writes to a disconnected client.

Recovered valid terminal usage switches settlement back to the authoritative
path. Timeout, EOF, capacity, malformed terminal data, or upstream failure uses
the estimated rules above.

## Audit and Privacy

Estimated records keep `usage_source=estimated` and
`usage_unconfirmed=true`. Cache statistics remain unknown. The record stores
the estimated token counts and final estimated quota, and does not claim that
the frozen reservation was consumed.

Public responses and user-visible logs must not expose upstream URLs, domains,
IPs, channel or account identifiers, keys, headers, raw bodies, or transport
errors. Billing settlement does not introduce new downstream error codes.

## Test Strategy

Deterministic regression tests must prove:

1. Valid terminal usage still settles normally.
2. Accepted EOF and downstream cancellation first attempt bounded recovery.
3. Missing terminal usage settles from estimated prompt plus observed output,
   below a larger reservation when appropriate.
4. Estimated tiered pricing uses the frozen expression without treating cache
   categories as authoritative.
5. Zero estimated tokens cannot produce positive finalized quota.
6. Ambiguous POST submission is not retried and uses prompt-only estimation.
7. Confirmed pre-write failure still refunds.
8. GPT text changes do not affect Claude, image, video, audio, async-task, or
   violation-fee billing tests.
9. Public errors contain none of the seeded upstream secrets or endpoints.

Verification includes focused service/relay/controller tests, full affected Go
package tests, `go vet`, `git diff --check`, a production-equivalent candidate
image, local brand/UI comparison, and private-port API checks before any hot
cutover.

## Deployment and Rollback

The billing change and the separately approved usage-log performance change are
implemented and tested as separate commits, then built into one candidate from
the production-aligned branch. The candidate binds only a loopback port until
accepted. Production cutover retains the current healthy container and image.
Any billing, API, database, cache-affinity, health, or UI regression triggers an
immediate Caddy rollback without deleting the preserved container.
