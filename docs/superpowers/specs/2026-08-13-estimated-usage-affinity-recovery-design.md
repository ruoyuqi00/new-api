# Estimated Usage and Affinity Recovery Design

## Context

The production candidate introduced conservative billing for requests that may have reached the upstream but ended before authoritative terminal usage arrived. This protected the gateway from refunding requests that the upstream may already have billed, but it produced consumption records with zero prompt/completion tokens and a non-zero quota.

Production evidence from the candidate window showed 734 zero-token charged records. Of those, 729 were explicitly marked `usage_unconfirmed`. This is not an acceptable user-facing billing contract even when the retained charge is economically justified.

The same interrupted Responses flows can lose provider prompt-cache locality. The current affinity rule chooses the first present source, normally `prompt_cache_key`, and stops after that source misses. It therefore may not consult an existing `previous_response_id` mapping recorded after an interrupted stream.

The candidate has been removed from production traffic by reloading Caddy back to the healthy rollback container. Both containers and images remain available; no database or historical billing data was changed.

## Goals

1. Never present a non-zero consumption charge as a normal zero-token request.
2. Prefer authoritative terminal usage by continuing to read an already accepted upstream stream for a bounded period after the downstream disconnects.
3. When authoritative usage remains unavailable, settle from an explicit token estimate while never charging less than the frozen reservation.
4. Preserve provider prompt-cache locality by trying all stable affinity identities in priority order instead of stopping after the first cache miss.
5. Never resubmit an ambiguous request merely to recover usage or cache locality.
6. Preserve the production brand UI, database contents, pricing expressions, and cross-database compatibility.

## Non-goals

- No response-content cache and no fabricated provider prompt-cache hit.
- No tokenizer migration from `cl100k_base` to `o200k_base` in this patch.
- No automatic refund after the upstream request may have been accepted.
- No retry or channel switch after ambiguous POST submission or mid-stream interruption.
- No schema migration, Caddy change, Cloudflare change, frontend/theme change, or production deployment during implementation.
- No retroactive mutation of historical logs or balances in this patch. Historical reconciliation requires a separate audited procedure.

## Considered Approaches

### A. Exact usage recovery with estimated fallback (selected)

Continue reading the same accepted upstream response after downstream cancellation, subject to time, byte, global-concurrency, and per-channel limits. Use authoritative terminal usage when recovered. Otherwise construct an estimated usage record from the request-side input estimate and output content already observed, settle no lower than the frozen reservation, and label the record as estimated.

This avoids zero-token charges, minimizes billing uncertainty, and does not create a second upstream charge.

### B. Estimate immediately on disconnect

Stop upstream reading and immediately settle from local estimates. This is simpler but throws away terminal usage that may arrive seconds later, including authoritative cached-token details. It weakens both billing accuracy and cache diagnostics and is rejected.

### C. Refund whenever terminal usage is absent

This is easy to explain to users but can make the gateway pay upstream charges that it refunded downstream. It also incentivizes disconnects and is rejected.

## Design

### 1. Bounded continuation of the original stream

The relay enables stream recovery only for an upstream response that has been accepted. Before acceptance, downstream cancellation continues to cancel the upstream request normally.

After acceptance and downstream cancellation:

- detach only the upstream read lifecycle from the downstream request context;
- never write further bytes to the disconnected client;
- continue parsing the same upstream SSE stream;
- stop at terminal usage, terminal protocol failure, timeout, size limit, upstream error, or admission-capacity rejection;
- cap recovery globally and per channel so disconnected clients cannot exhaust gateway resources;
- preserve exact terminal usage if it arrives at the configured byte boundary;
- never create another POST and never select another channel.

Recovery defaults remain bounded and configurable. Tests use deterministic short limits; production values are selected only after local verification.

### 2. Usage source and estimation

Settlement recognizes two usage sources:

- `upstream`: terminal provider usage was received and is authoritative;
- `estimated`: terminal usage was not recovered and local token estimates were used.

Estimated usage is constructed as follows:

- prompt tokens use the request-side estimate already computed before relay;
- completion tokens use text/output observed from the original response and the existing model-aware tokenizer;
- total tokens equal prompt plus completion;
- cached, cache-creation, image, and audio token subfields remain zero unless they were authoritatively observed before termination;
- no estimated field is described as a confirmed provider cache hit or miss.

If no output was observed, the log still records the estimated prompt tokens rather than zero total tokens. The record carries `usage_source=estimated`, `usage_unconfirmed=true`, and a bounded recovery-result category such as `timeout`, `size_limit`, `capacity`, or `upstream_error`.

### 3. Settlement invariant

Authoritative usage follows the normal pricing and tiered-expression settlement path.

Estimated usage is visible for explanation and audit, but its calculated quota must not reduce the frozen reservation. Final quota is:

```text
max(estimated_usage_quota, frozen_reservation_quota)
```

Tiered billing must not reclassify unconfirmed estimated cache fields as authoritative. The frozen billing snapshot, estimated tier, final quota, and `settled_from_reservation` state remain in the private billing metadata. Trusted-wallet settlement errors must fail before recording a successful consumption log or aggregate.

Requests proven not sent retain the existing refund path. Requests that may have reached upstream never refund or retry automatically.

### 4. Affinity lookup cascade

For Responses/Codex traffic, lookup tries stable identities in this order:

1. `previous_response_id` response chain;
2. explicit `conversation` ID;
3. `prompt_cache_key`;
4. explicit session/thread headers or turn metadata.

The lookup continues to the next source when a higher-priority source is present but has no cache entry. It stops only on a cache hit or when all applicable sources have been tried.

Scoped identities are hashed with authenticated token ID, group, and model. Different users, groups, models, and concurrent conversations cannot share a mapping merely because their raw identifiers match. Raw stable identifiers are not logged.

### 5. Provisional and confirmed affinity

As soon as a real upstream response ID is observed, the selected channel is recorded provisionally for:

- the response-chain ID; and
- the stable primary request identity already used for this request, when present.

Provisional mappings are capped at 15 minutes. A successful terminal outcome promotes/refreshes mappings using the configured normal TTL. Interrupted streams never clear an existing mapping solely because terminal usage is absent.

If the preferred channel is unavailable, existing explicit failover policy still applies. Ambiguous submission, downstream cancellation after acceptance, or any received response event suppresses automatic retry and channel switching.

### 6. User-visible billing contract

Consumption logs with retained quota must contain non-zero estimated prompt tokens whenever request-side input tokens were available. The user sees a concise estimated-usage indicator, not an internal pending/frozen state and not a zero-token normal charge.

Public responses and logs never expose upstream URLs, domains, IPs, channel identifiers, API keys, authorization values, raw upstream bodies, transport errors, or database errors.

This patch changes backend log metadata only. It does not edit frontend source; the existing UI renders the numeric tokens and content metadata. A separate UI change is required only if the current renderer cannot surface the existing estimated marker without brand or theme changes.

## Failure Handling

- Before request write: retain normal safe retry/refund behavior.
- After possible request write but before headers: classify ambiguous, do not retry/refund, and settle with estimated prompt usage at or above the frozen reservation.
- After accepted stream and client disconnect: detach and drain within limits.
- Terminal usage recovered: authoritative settlement.
- Drain limit/error: estimated settlement with explicit recovery result.
- Consumption settlement failure: do not write a false successful billing log.
- Affinity cache failure: continue the request through normal channel selection and record only a redacted internal category.

## Test Strategy

Deterministic regression tests cover:

1. Client cancellation after acceptance continues reading the same stream and recovers exact terminal usage without downstream writes.
2. Recovery timeout, size, capacity, and upstream-error outcomes create estimated prompt/output usage rather than zero-token consumption.
3. Estimated settlement never falls below the frozen reservation and never runs tiered settlement as authoritative usage.
4. Requests proven not sent still refund; ambiguous requests do not retry or refund.
5. A present but missing `prompt_cache_key` mapping falls through to a matching response-chain or conversation mapping.
6. Response-chain, conversation, cache-key, session, token, group, and model scopes do not bleed into each other.
7. Provisional mappings use a maximum 15-minute TTL and successful completion refreshes normal TTL.
8. Public errors and log metadata contain no seeded upstream infrastructure or credentials.
9. Existing successful streaming, authoritative usage, cache statistics, billing, and affinity tests remain green.

Verification runs focused relay/service/controller tests, full affected Go package tests, `go vet` for affected packages, `git diff --check`, a production-equivalent local image build, backend health checks, and static UI fingerprint/Playwright comparison against the preserved production baseline. Production traffic remains untouched until the user explicitly approves the verified candidate.

## Acceptance Criteria

- No new consumption path writes a positive quota with zero prompt and completion tokens when a request-side prompt estimate exists.
- Recovered terminal usage is authoritative; fallback usage is explicitly estimated.
- Final estimated quota is never less than the frozen reservation.
- No ambiguous request is automatically retried, switched, or refunded.
- A higher-priority affinity source miss does not prevent a lower-priority stable identity from hitting its channel mapping.
- Interrupted response chains remain on the same eligible channel without cross-user/session leakage.
- No frontend, branding, schema, production database, Caddy, or Cloudflare changes are included.
