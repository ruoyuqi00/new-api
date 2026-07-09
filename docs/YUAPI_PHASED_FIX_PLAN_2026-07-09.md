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

Status: completed.

Goal:

Inspect the remaining upstream task duration bound from `d0bd8aac` and decide
whether YuAPI should reject abusive video task `seconds` / `duration` values at
the task request validator. Keep this phase scoped to task duration only.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro routing,
  or group/model mapping.
- Do not port broad quota math conversion or admin audit UI changes.
- Keep deployment separate unless explicitly requested after tests.

Accepted work for this phase:

- Add a shared task-duration request validator for YuAPI task submit paths.
- Reject negative, non-numeric, overflowed, or above-bound `seconds` values.
- Reject negative or above-bound `duration` values.
- Reject task metadata duration values that can override provider request or
  task `OtherRatios`, including `metadata.durationSeconds` and
  `metadata.parameters.duration`.
- Preserve provider-specific default duration behavior when duration is absent
  or zero.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Normal supported video durations remain compatible.
- Invalid or abusive duration values are rejected before they reach
  `OtherRatios`.
- Provider-specific default duration behavior remains unchanged.

Review notes:

- A read-only subagent audit confirmed that `ValidateMultipartDirect` and
  `ValidateBasicTaskRequest` were the right shared request-boundary hooks.
- The audit also confirmed metadata duration bypasses for Gemini/Vertex
  `durationSeconds` and Ali `parameters.duration`; those are included in this
  phase because they feed task duration multipliers.
- No duration field participates in account-pool or channel-pool selection.
  Local request rejection happens before bad values can reach upstream
  provider calls or cooldown handling.

Implementation:

- `relay/common/relay_utils.go`
  - Added `MaxTaskDurationSeconds = 3600`.
  - Added shared task duration validation for standard `duration`, standard
    `seconds`, `metadata.duration`, `metadata.durationSeconds`,
    `metadata.parameters.duration`, and
    `metadata.parameters.durationSeconds`.
  - `ValidateMultipartDirect` and `ValidateBasicTaskRequest` now reject bad
    duration fields before storing `task_request`.
  - Multipart task parsing now preserves explicit `seconds` strings so
    non-numeric values cannot be silently dropped.
- `relay/common/relay_info.go`
  - `TaskSubmitReq.UnmarshalJSON` now returns a validation error for explicit
    non-integer or overflowed JSON `duration` values instead of silently
    normalizing them to zero.
- `relay/common/relay_utils_test.go`
  - Added JSON coverage for above-bound, negative, non-numeric, fractional, and
    metadata duration inputs.
  - Added multipart coverage for non-numeric and above-bound `seconds`.
  - Added acceptance coverage for normal values, max-bound metadata duration,
    and zero duration preserving provider defaults.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/common
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/common ./relay/helper ./relay/channel ./relay
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

Result:

```text
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
```

## Phase 9 - Task Billing Ratio Saturation Triage

Status: completed.

Goal:

Inspect task quota calculation where provider `EstimateBilling` values are
merged into `PriceData.OtherRatios` and multiplied into pre-consumed quota.
Decide whether YuAPI should add saturating arithmetic or narrower validation
for task billing multipliers, using upstream `d0bd8aac` / `bae799cc` as
references without importing broad audit UI or pricing behavior changes.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro
  routing, or group/model mapping.
- Do not change provider-supported normal duration/resolution values.
- Do not port broad admin audit UI changes unless a task-billing bug requires a
  tiny display fix.
- Keep deployment separate unless explicitly requested after tests.

Accepted work for this phase:

- Manually port only the low-risk quota saturation primitive from upstream
  `d0bd8aac`.
- Apply saturating float-to-int conversion to task submit `OtherRatios` quota
  multiplication.
- Apply the same saturation to submit-time adjusted-ratio recomputation and
  asynchronous token-based task quota recalculation.
- Defer upstream `bae799cc` admin audit/UI surfacing because it is broader than
  this phase and crosses log formatting plus frontend surfaces.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Normal task prices and plus/pro routing remain unchanged.
- Any accepted saturation behavior has a focused overflow or mismatch
  regression test.
- Deferred upstream billing/audit changes are listed with a reason.

Implementation:

- `common/quota_math.go`
  - Added `QuotaFromFloat`, a shared saturating conversion for computed quota
    products.
  - Clamps overflow to `math.MaxInt32`, underflow to `math.MinInt32`, and `NaN`
    to `0`.
- `relay/relay_task.go`
  - Added `applyTaskOtherRatiosQuota`.
  - Task submit `OtherRatios` multiplication now computes in float64 and uses
    `common.QuotaFromFloat` once at the final conversion.
  - `recalcQuotaFromRatios` now preserves the base quota in float64 while
    reversing old ratios and clamps the final adjusted quota.
- `service/task_billing.go`
  - Added `taskTokenRecalculatedQuota`.
  - `RecalculateTaskQuotaByTokens` now saturates the final
    `tokens * modelRatio * groupRatio * otherMultiplier` conversion.
- Tests:
  - `common/quota_math_test.go` covers normal values, overflow, underflow,
    infinities, and `NaN`.
  - `relay/relay_task_test.go` covers normal task ratio multiplication,
    overflow saturation, and adjusted-ratio recomputation saturation.
  - `service/task_billing_test.go` covers normal token recalculation and
    overflow saturation.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./common ./relay ./service
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
  go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
```

## Phase 10 - Billing Saturation Audit Follow-Up

Status: completed.

Objective:

Add a small admin-only audit marker for task quota saturation events, using
upstream `bae799cc` as a reference, while keeping Phase 9 quota results
unchanged. Keep the scope to server-side log metadata only and defer broader
frontend usage-log UI.

Boundary:

- Do not change account-pool or channel-pool scheduling semantics.
- Do not change model prices, group ratios, provider priority, plus/pro
  routing, or group/model mapping.
- Do not change quota calculation results from Phase 9.
- Prefer server-side log metadata over frontend table changes.
- Keep deployment separate unless explicitly requested after tests.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- Saturation markers, if added, are admin-only and do not leak sensitive
  channel/key/account data.
- Normal non-saturated billing logs remain unchanged.
- Any frontend/admin UI change is explicitly justified or deferred.

Implementation:

- `common/quota_math.go`
  - Added `QuotaFromFloatChecked`, which preserves the existing saturated quota
    result and also returns a `QuotaClamp` audit record when a value is clamped.
  - `QuotaFromFloat` remains the compatibility wrapper, so existing quota
    calculation call sites keep the same return values.
  - `QuotaClamp` stores `kind`, stringified `original`, `clamped`, and optional
    `op`. Stringifying the original value avoids invalid JSON for `Inf` or
    `NaN`.
- `relay/common/relay_info.go`
  - Added `TaskQuotaClamp` for async task billing log metadata only.
- `relay/relay_task.go`
  - Task submit `OtherRatios` and submit-time adjusted-ratio recalculation now
    use the checked conversion path.
  - The first saturation event is attached to `RelayInfo.TaskQuotaClamp` with
    an operation name (`task_submit_other_ratios` or
    `task_submit_adjusted_ratios`).
- `service/task_billing.go`
  - Added `attachQuotaClampAdminInfo`, which writes only
    `Other.admin_info.quota_saturation`.
  - `LogTaskConsumption` attaches submit-time task quota saturation metadata.
  - `RecalculateTaskQuota` accepts an optional clamp marker and preserves old
    call sites through a variadic parameter.
  - `RecalculateTaskQuotaByTokens` records
    `task_token_recalculation` saturation metadata when token-based async
    recalculation clamps.
- Tests:
  - `common/quota_math_test.go` covers checked conversion and audit maps.
  - `relay/relay_task_test.go` covers checked task ratio conversions and
    first-clamp retention on `RelayInfo`.
  - `service/task_billing_test.go` covers admin-only quota saturation metadata,
    no `admin_info` on normal non-saturated recalculation, checked token
    recalculation, and preservation of existing `admin_info` fields.

Deferred upstream scope:

- Did not port the broader upstream `bae799cc` frontend usage-log display.
- Did not port unrelated text/audio/tiered billing audit changes in this phase.
- Did not change pricing, group ratios, provider priority, plus/pro routing,
  group/model mapping, channel scheduling, account-pool behavior, or production
  data.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./common ./relay ./service
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
  go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
```

## Phase 11 - Upstream Low-Risk Bug Triage

Status: completed.

Objective:

Review recent upstream `origin/main` fixes again and pick one low-risk server-side
bug fix that benefits YuAPI production without touching account-pool/channel-pool
scheduling, provider priority, plus/pro routing, pricing, or group/model mapping.
If no suitable upstream item is narrow enough, inspect YuAPI's local task/logging
surface for one focused bug and document why it is safe to fix.

Acceptance checks:

```bash
go test ./common ./relay ./service
go test ./model ./service ./middleware ./controller
go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Manual review checks:

- The selected Phase 11 fix is independently revertible.
- Scheduling and account/channel selection semantics remain unchanged.
- Any deferred upstream item is recorded with a reason.

Upstream triage result:

- Reviewed recent `origin/main` server-side fixes through sub-agent and local
  git inspection.
- The lowest-risk upstream candidates were already present in this branch:
  - `fae39cd90` / `dfcb74b52`: subscription migration tag fixes.
  - `0d5995eb6`: read-only token access for non-disabled tokens.
  - `bfddc5fea`: omit `access_token` from normal user queries.
  - `cf6ae6fde`: preserve SMTP PLAIN auth TLS guard.
  - `d2f7f9ee3`: anonymous request body limit.
  - `3aa113b5a`: Dify remote-image nil pointer fix.
  - `87cc22d7e`: video task GET model lookup for token model limits.
  - `df44a75d5`: ClickHouse log LIKE escaping.
  - `0977965d9`: Ollama non-stream tool calls.
  - `502858d35`: Claude empty tool-call arguments preservation.
  - `933ea0cdd`: relay idle connection timeout.
- Because the safe upstream fixes were already absorbed, Phase 11 used the
  fallback path and fixed a local task/logging bug.

Implementation:

- `service/task_billing.go`
  - `RefundTaskQuota` now sets `task.Quota = 0` after wallet/subscription and
    token quota refunds succeed.
  - Persisted tasks call `task.UpdateQuota()` so task list/detail views no
    longer show the original pre-consumed quota after a successful refund.
  - The refund log still records the original refunded quota amount.
- `service/task_billing_test.go`
  - Existing wallet and subscription refund tests now assert the in-memory task
    quota is cleared.
  - Added `TestRefundTaskQuota_PersistsZeroQuota`, which creates a real task row
    and verifies the database `quota` is persisted to `0` after refund.

Deferred scope:

- Did not change account-pool/channel-pool scheduling, provider priority,
  plus/pro routing, model pricing, group ratios, or group/model mapping.
- Did not alter refund funding order; quota is cleared only after funding and
  token refund steps complete.
- Deferred log-stat request-id filtering as Phase 12 because it touches
  controller/model log query contracts rather than task refund state.

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
  go test ./common ./relay ./service
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
  go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
```

## Phase 12 - Log Stat Filter Alignment

Status: completed.

Objective:

Align log-stat filtering with log-list filtering for `request_id` and
`upstream_request_id`, so admin/user log statistic cards narrow with the same
request filters as the table. Keep the scope to log query/controller contracts
only; do not change task billing, scheduling, pricing, routing, or account pool
behavior.

Acceptance checks:

```bash
go test ./model ./controller
go test ./model ./service ./middleware ./controller
```

Manual review checks:

- Existing log list filters remain unchanged.
- Stats without request filters keep their current result.
- Request-id filtered stats use exact-match semantics and do not introduce LIKE
  wildcard behavior.

Implementation:

- `controller/log.go`
  - `GetLogsStat` now reads `request_id` and `upstream_request_id`.
  - `GetLogsSelfStat` now reads the same request-id filters.
- `model/log.go`
  - `SumUsedQuota` now accepts `requestId` and `upstreamRequestId`.
  - Both the quota sum query and the recent RPM/TPM query apply exact
    `request_id = ?` and `upstream_request_id = ?` filters when provided.
- `model/log_stat_test.go`
  - Added request-id and upstream-request-id coverage for quota, RPM, and TPM.
  - Confirms missing request ids return zero stats.

Deferred scope:

- Did not change log list filtering behavior.
- Did not change `type` statistic semantics; existing stats continue to count
  consume logs for quota/RPM/TPM.
- Did not change task billing, scheduling, pricing, routing, account-pool, or
  channel-pool behavior.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./model ./controller
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./common ./relay ./service
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
  go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
```

## Phase 13 - Async Task Billing Node Metadata

Status: planned.

Next phase objective:

Improve async task billing log observability by recording the originating
`node_name` in admin-only log metadata for task refunds and task quota
recalculations. Keep this as a metadata-only change; do not add request-id
persistence, schema changes, billing amount changes, scheduling changes, or
account/channel pool changes in this phase.

Planned acceptance checks:

```bash
go test ./service
go test ./model ./service ./middleware ./controller
```

Manual review checks:

- `node_name` is admin-only and stripped from user-visible log responses.
- Existing log `Other` fields are preserved.
- Funding/token quota refund and settlement order remains unchanged.
