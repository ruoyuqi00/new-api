# YuAPI Phased Fix Plan - 2026-07-09

This document tracks staged production hardening after the YuAPI/Sub2API
plus/pro consolidation.

Rule for every phase:

1. Write the phase goal and acceptance checks first.
2. Keep code changes small enough to test and roll back independently.
3. Update this document with implementation, verification, and the next phase
   objective.
4. Push the phase commit before moving to the next phase.

## Phase 1 - Control-Plane Input Safety

Status: completed.

Goal:

- Backport small upstream safety fixes that do not touch provider routing,
  billing, or channel selection.
- Reduce accidental or malicious control-plane edge cases before deeper
  streaming and billing changes.

Scope:

- Reject explicitly disabled API tokens in `TokenAuthReadOnly` while preserving
  read-only access for expired/exhausted/non-disabled tokens.
- Trim usernames in registration and admin/user update before validation and
  persistence.
- Reject usernames that become empty after trimming.

Out of scope:

- Login username normalization.
- Email/password hardening beyond the trim/empty username fix.
- Billing, stream disconnect handling, SSRF, and channel scheduler behavior.

Acceptance checks:

```bash
go test ./middleware ./controller ./model
```

Manual review checks:

- Disabled tokens cannot use read-only token-auth routes.
- Expired/exhausted tokens remain compatible with read-only usage endpoints.
- Registration does not persist leading/trailing whitespace in usernames.
- User update does not persist leading/trailing whitespace in usernames.

Implementation:

- `middleware/auth.go`
  - `TokenAuthReadOnly` now rejects tokens with
    `common.TokenStatusDisabled`.
  - Expiry and remaining quota are still ignored for non-disabled read-only
    usage, preserving the intended usage-query compatibility.
- `controller/user.go`
  - `Register` trims `user.Username` before validation and persistence.
  - `UpdateUser` trims `updatedUser.Username` before validation and
    persistence.
  - Both paths reject usernames that become empty after trimming.
- `middleware/auth_readonly_test.go`
  - Covers disabled-token rejection.
  - Covers expired/exhausted but non-disabled read-only compatibility.
- `controller/user_phase1_test.go`
  - Covers registration trim persistence.
  - Covers blank username rejection after trim.
  - Covers admin/user update trim persistence.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./middleware ./controller ./model
```

Result:

```text
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/model
```

## Phase 2 - Streaming Disconnect Safety

Status: completed.

Goal:

Backport the upstream stream-disconnect safety work in a YuAPI-compatible way,
so client disconnects close upstream response bodies promptly, stream write
loops cannot hang indefinitely on slow clients, and users are not billed for
tokens generated after they disconnected.

Boundary:

- Keep channel-pool lease release behavior explicit and tested.
- Do not change provider selection, billing formulae, or plus/pro channel
  priorities in this phase.
- Prefer a small backport over a broad relay refactor.

Candidate upstream reference:

```text
153d7f01 fix: avoid stale stream writes after client disconnect (#5710)
```

Planned acceptance checks:

```bash
go test ./relay/helper ./relay/channel ./relay
go test ./service ./middleware ./controller
```

Additional smoke:

- Non-stream chat completion still succeeds.
- Stream chat completion still completes and records quota.
- Simulated disconnect stops the upstream body and releases channel-pool lease.

Implementation:

- `relay/helper/stream_scanner.go`
  - Added bounded per-write deadline via `ExtendWriteDeadline`.
  - Reworked stream cleanup with `cleanupOnce` and `stopOnce`.
  - Client disconnect now cancels stream goroutines, closes the upstream
    response body immediately, stops timers, and waits for stream goroutines
    before returning the Gin context.
- `relay/helper/common.go`
  - Centralized request-context cancellation checks.
  - `StringData`, `PingData`, and `FlushWriter` preserve disconnect-aware
    error returns.
  - `ResponseChunkData` now returns write errors to callers that can stop.
  - Claude stream helpers skip writes after request cancellation.
- `relay/channel/api_request.go`
  - Stream ping keepalive now exposes a done channel and is waited before
    `doRequest` returns.
  - Ping writes use `helper.ExtendWriteDeadline` instead of spawning nested
    write goroutines.
- `relay/channel/openai/relay_image.go`
  - Image stream SSE writes use shared stream helper functions, so request
    cancellation and write failures are visible.
- `relay/channel/openai/responses_via_chat.go`
  - Chat-to-Responses stream conversion now stops on `ResponseChunkData`
    write errors.
- `relay/channel/gemini/relay_responses.go`
  - Gemini Responses stream conversion now stops on `ResponseChunkData`
    write errors.
- `relay/channel/openai/helper.go`
  - Adapted the generic Responses stream send helper to the new
    `ResponseChunkData` signature.
- `relay/helper/stream_scanner_test.go`
  - Added a client-cancel regression test proving the handler returns promptly,
    closes upstream, and ignores chunks sent after disconnect.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./service ./middleware ./controller
```

Result:

```text
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
```

## Phase 3 - Channel-Pool Scheduler Observability

Status: completed.

Goal:

Harden YuAPI channel-pool scheduling visibility without changing the current
plus/pro routing policy. Add enough counters/log context/tests to explain why a
candidate was skipped, full, cooled down, selected, or released, so future
account-pool tuning can be done from evidence instead of guesswork.

Boundary:

- Do not change provider priority, model mapping, group routing, or billing
  formulae in this phase.
- Do not delete or rewrite existing plus/pro channels.
- Keep lease acquire/release behavior explicit and covered by focused tests.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Channel-pool full and cooldown paths are distinguishable in logs.
- Lease release remains idempotent across success, error, retry, and client
  disconnect paths.
- Added observability does not expose API keys, OAuth tokens, or account
  credentials.

Implementation:

- `model/channel_pool_runtime.go`
  - Added `ChannelPoolCandidateStatusFor`, which preserves the existing
    availability decision while exposing a non-secret reason:
    `available`, `full`, `cooldown`, or `no_channel`.
  - Added `ChannelPoolSelectionSnapshotFor`, a read-only selection summary for
    empty channel-selection results. It reports candidate, available, full,
    cooldown, missing, skipped, and path-skipped counts.
  - Kept channel-cache locking narrow: the snapshot collects candidate channel
    pointers under the cache read lock, then checks cooldown/inflight state
    after releasing it.
- `service/channel_pool.go`
  - Added request-scoped lease logs for reuse, replacement, full acquire,
    acquire, and release paths.
  - Added context-aware affinity skip logging via
    `IsChannelPoolTemporarilyUnavailableWithContext`.
  - Log payloads only include channel id, group, model, reason, limit,
    inflight, cooldown, and hard-limit state. They do not include API keys,
    OAuth tokens, token keys, channel keys, or account credentials.
- `service/channel_select.go`
  - Logs a channel-pool selection snapshot when random selection returns no
    channel.
  - Uses warn level only when the snapshot shows `full` or `cooldown`; ordinary
    no-candidate snapshots remain debug-only.
- `middleware/distributor.go`
  - Affinity channel-pool skip checks now use the context-aware service wrapper
    so production logs keep the request id.
- `model/channel_pool_runtime_test.go`
  - Added coverage for candidate status reasons and selection snapshot counts.
- `service/channel_pool_test.go`
  - Added focused service tests proving same-channel lease reuse does not
    double-count, replacing a selected channel releases the old lease, release
    is idempotent, and full state remains visible through the context-aware
    wrapper.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./service ./middleware ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 4 - Upstream Fix Triage And Low-Risk Bug Sweep

Status: completed.

Goal:

Compare YuAPI against the current upstream `QuantumNous/new-api` fix stream and
select only low-risk bug fixes that do not disturb YuAPI provider adapters,
plus/pro routing, group/model mapping, account-pool scheduling, or billing.
If no upstream fix is worth merging immediately, run a local bug sweep focused
on relay error handling, quota settlement edge cases, and channel selection
fallbacks.

Boundary:

- Do not rebase YuAPI onto upstream or bulk-merge upstream feature work.
- Do not change production channel/account data, provider priority, pricing, or
  plus/pro channel routing.
- Treat upstream patches as candidates; each accepted patch needs a small diff,
  focused tests, and a documented reason.
- Keep deployment separate from this phase unless explicitly requested after
  code review and tests.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Every imported upstream fix has a referenced commit or PR and a YuAPI-specific
  compatibility note.
- Rejected upstream changes are listed with a short reason.
- Any locally found bug has a reproduction note or focused regression test.

Upstream triage:

- Fetched and inspected `origin/main` without rebasing or merging. This YuAPI
  branch and `origin/main` have no clean merge-base in this worktree, so accepted
  changes were manually ported as narrow patches.
- Accepted upstream candidate:
  - `043720f9` / PR `#5923`: task quota persistence after delta settlement and
    Ali video non-positive duration fallback.
  - YuAPI compatibility note: the accepted patch touches only persisted task
    quota after already-computed settlement and Ali request normalization. It
    does not change channel/account selection, provider priority, plus/pro
    routing, group/model mapping, pricing, or account-pool scheduling.
- Deferred upstream candidates:
  - `3fbad6a7`: tiered pre-consume fallback changes pre-consumption and pricing
    behavior; defer to a dedicated billing phase.
  - `48b7f491`, `d0bd8aac`, `c9943d37`, `bae799cc`: quota/billing saturation
    chain is broader than this phase; inspect together in Phase 5.
  - `70ea899e`: transaction and row-locking changes are high blast radius for
    production billing; defer until lock semantics are reviewed against YuAPI.
  - `5fc35e28`: user/email/password hardening is useful but broad; defer to a
    user/auth hardening phase.
  - Web, i18n, and build-only changes: not relevant to the production
    provider/account-pool consolidation path in this phase.

Implementation:

- `model/task.go`
  - Added `Task.UpdateQuota()` for a single-column quota writeback.
- `service/task_billing.go`
  - After `RecalculateTaskQuota` computes and applies the delta, it now writes
    the final `task.Quota` back to `tasks.quota` and logs a non-fatal error if
    the persistence step fails.
- `relay/channel/task/ali/adaptor.go`
  - Restores Ali video default duration to 5 seconds when request seconds or
    metadata normalize to a non-positive duration.
- `service/task_billing_test.go`
  - Added a persisted-task regression proving `tasks.quota` equals the actual
    settled quota after recalculation.
- `relay/channel/task/ali/adaptor_test.go`
  - Added regressions for `seconds: "0"` and metadata `parameters.duration: 0`.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./service ./relay/channel/task/ali
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./service ./middleware ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/relay/channel/task/ali
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 5 - Task/Billing Saturation Follow-Up

Status: completed.

Goal:

Review the deferred upstream quota/billing saturation chain and YuAPI's local
task-billing paths as one small phase. The goal is to decide whether YuAPI needs
safe clamping or transaction hardening around quota settlement without changing
pricing, provider routing, channel/account-pool scheduling, or production data.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro routing,
  or group/model mapping.
- Do not deploy or migrate production data inside the phase.
- If an upstream patch changes billing formulas or pre-consumption behavior,
  document it as a candidate and keep the code change out until explicitly
  accepted.

Accepted work for this phase:

- Manually port only the low-risk saturation guard from upstream billing fixes:
  - `48b7f491`: prevent overflow in `composeTieredTextQuota` by saturating the
    final tiered quota plus tool-call surcharge total.
- Keep `d0bd8aac`, `c9943d37`, and `bae799cc` deferred because they are a
  coordinated validation/saturation/audit batch across task, image, text, audio,
  and UI surfaces.
- Do not port the broader audit/UI saturation markers from `bae799cc` in this
  phase.
- Do not port tiered pre-consume behavior changes from `3fbad6a7` in this
  phase.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Every accepted billing change has a focused test covering the exact edge case.
- Any clamp or transaction change records how it affects wallet, token, and
  subscription billing.
- Channel/account-pool lease acquire/release behavior remains unchanged.

Implementation:

- `service/text_quota.go`
  - Added `quotaFromDecimalSaturating` for the local tiered text quota path.
  - `composeTieredTextQuota` now saturates the final sum of tiered quota plus
    tool-call surcharge instead of converting only the surcharge and then doing
    unchecked integer addition.
- `service/text_quota_test.go`
  - Added fallback-path saturation coverage for `tieredQuota + surcharge`.
  - Added `TieredResult` path saturation coverage for
    `actualQuotaBeforeGroup * groupRatio + surcharge`.
  - Existing normal surcharge tests still cover ordinary pricing behavior.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./service
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./service ./middleware ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 6 - Request Quantity Bounds Triage

Status: completed.

Goal:

Inspect the remaining upstream quantity-validation and saturation candidates
from `d0bd8aac`/`c9943d37` and choose one narrow request-boundary fix that
prevents abusive quantity inputs before they enter billing math. Prefer
validation that rejects impossible user input over deeper billing rewrites.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro routing,
  or group/model mapping.
- Do not port the full saturation audit/UI stack from `bae799cc`.
- Do not change tiered pre-consume defaults from `3fbad6a7`.
- Keep deployment separate unless explicitly requested after tests.

Accepted work for this phase:

- Manually port the narrow max-token bounds from upstream `c9943d37`:
  - OpenAI chat/completions: `max_tokens` and `max_completion_tokens`.
  - Claude: `max_tokens` and `max_tokens_to_sample`.
  - Gemini: `generationConfig.maxOutputTokens`.
  - OpenAI Responses: `max_output_tokens`.
- Keep image count and task duration quantity bounds deferred because they touch
  additional image/task request surfaces and deserve their own focused tests.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- The accepted validation change rejects only invalid or abusive quantities.
- Existing normal image/task/text request quantities remain compatible.
- Any deferred upstream patch is recorded with a reason.

Implementation:

- `relay/helper/valid_request.go`
  - Added a shared `maxTokensLimit` / `exceedsMaxTokensLimit` guard.
  - Extended the existing OpenAI `max_tokens` bound to also cover
    `max_completion_tokens`.
  - Added equivalent bounds for Claude `max_tokens` and
    `max_tokens_to_sample`.
  - Added equivalent bounds for Gemini `generationConfig.maxOutputTokens`.
  - Added equivalent bounds for OpenAI Responses `max_output_tokens`.
- `relay/helper/max_tokens_bounds_test.go`
  - Added focused regressions proving pathological large max-token values are
    rejected across OpenAI, Claude, Gemini, and Responses request validators.
  - Added normal `8192` acceptance checks for OpenAI, Claude, Gemini, and
    Responses paths.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./service ./middleware ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 7 - Image/Task Quantity Bounds Follow-Up

Status: completed.

Goal:

Inspect the remaining request quantity bounds from upstream `d0bd8aac` and
choose one narrow validation patch for image count (`n`) or task video duration
(`seconds` / `duration`). Prefer a single request family per phase so each
change has precise tests and rollback scope.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro routing,
  or group/model mapping.
- Do not port broad quota math conversion or admin audit UI changes in this
  phase.
- Keep deployment separate unless explicitly requested after tests.

Accepted work for this phase:

- Manually port only the image count bound from upstream `d0bd8aac`:
  - Add `dto.MaxImageN`.
  - Reject OpenAI image JSON `n` values above the bound.
  - Reject multipart image edit `n` values that are negative, non-integer, or
    above the bound.
- Keep task video duration (`seconds` / `duration`) deferred to the next phase.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Normal image/task quantities remain compatible.
- The accepted validator rejects only invalid or abusive quantities.
- Deferred image/task quantity candidates are recorded with a reason.

Implementation:

- `dto/openai_image.go`
  - Added `MaxImageN = 128` as the image-generation count bound.
- `relay/helper/valid_request.go`
  - JSON image requests now reject `n > MaxImageN`.
  - Multipart image edit requests now parse `n` explicitly and reject negative,
    non-integer, or above-bound values before converting to `uint`.
  - Missing or zero `n` still defaults to 1.
- `relay/helper/openai_image_request_test.go`
  - Added JSON coverage for overflow-sized `n`, above-bound `n`, bound value,
    and absent default.
  - Added multipart coverage for negative `n` rejection and bound value.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./service ./middleware ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 8 - Task Duration Bounds Follow-Up

Status: planned.

Next phase objective:

Inspect the remaining upstream task duration bound from `d0bd8aac` and decide
whether YuAPI should reject abusive video task `seconds` / `duration` values at
the task request validator. Keep this phase scoped to task duration only.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro routing,
  or group/model mapping.
- Do not port broad quota math conversion or admin audit UI changes.
- Keep deployment separate unless explicitly requested after tests.

Planned acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Normal supported video durations remain compatible.
- Invalid or abusive duration values are rejected before they reach
  `OtherRatios`.
- Provider-specific default duration behavior remains unchanged.
