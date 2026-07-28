# YuAPI Phased Fix Plan - 2026-07-09

This document tracks staged production hardening after the YuAPI/Sub2API
plus/pro consolidation.

Rule for every phase:

1. Write the phase goal and acceptance checks first.
2. Keep code changes small enough to test and roll back independently.
3. Update this document with implementation, verification, and the next phase
   objective.
4. Push the phase commit before moving to the next phase.

UI work boundary:

- YuCore UI / Studio / Canvas work is paused for backend production phases.
- Resume UI work from `docs/YUCORE_UI_NEXT_WINDOW_HANDOFF_2026-07-10.md` in a
  separate UI-focused window.
- Do not mix UI polishing with backend protocol, billing, scheduler, account
  pool, channel-pool, or production deployment phases.

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

Status: completed.

Objective:

Improve async task billing log observability by recording the originating
`node_name` in admin-only log metadata for task refunds and task quota
recalculations. Keep this as a metadata-only change; do not add request-id
persistence, schema changes, billing amount changes, scheduling changes, or
account/channel pool changes in this phase.

Acceptance checks:

```bash
go test ./service
go test ./model ./service ./middleware ./controller
```

Manual review checks:

- `node_name` is admin-only and stripped from user-visible log responses.
- Existing log `Other` fields are preserved.
- Funding/token quota refund and settlement order remains unchanged.

Implementation:

- `service/task_billing.go`
  - Added `ensureTaskAdminInfo`, shared by task billing admin-only metadata
    helpers.
  - Added `attachTaskNodeAdminInfo`, which writes `admin_info.node_name` from
    `task.PrivateData.NodeName`, falling back to `common.NodeName` only when the
    task snapshot does not carry a node.
  - `RefundTaskQuota` now records async refund `node_name` in admin-only log
    metadata and passes `NodeName` into `RecordTaskBillingLog` for data export
    consistency.
  - `RecalculateTaskQuota` now records async difference-settlement `node_name`
    alongside any existing `quota_saturation` metadata.
- `service/task_billing_test.go`
  - Added `TestRefundTaskQuota_NodeAdminInfo`.
  - Extended quota saturation metadata coverage to verify `node_name` and
    `quota_saturation` coexist under `admin_info`.

Deferred scope:

- Did not add request-id persistence to task private data.
- Did not add schema changes.
- Did not change billing amount calculations, refund order, settlement order,
  scheduling, routing, provider priority, account pools, or channel pools.

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
  go test ./relay/common ./relay/helper ./relay/channel ./relay
```

Result:

```text
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/relay/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
```

## Phase 14 - Log Stat Type Semantics Review

Status: completed.

Objective:

Review log statistic `type` filtering semantics before changing behavior. The
current stats query intentionally or historically counts consume logs for quota,
RPM, and TPM even when a non-consume `type` filter is supplied. Determine whether
the UI expects usage stats to remain consume-only or to follow the selected log
type. If a fix is accepted, keep it limited to log-stat query semantics and
tests.

Acceptance checks:

```bash
go test ./model ./controller
go test ./model ./service ./middleware ./controller
```

Manual review checks:

- No task billing, scheduling, pricing, provider priority, account-pool, or
  channel-pool changes.
- Existing default stats (`type=0`) remain unchanged.
- Any non-consume `type` behavior change is explicitly documented with tests.

Review findings:

- `origin/main` has the same consume-only `SumUsedQuota` behavior: the stats
  query accepts `logType` but always filters quota/RPM/TPM to `LogTypeConsume`.
- Local list endpoints do honor `type` filters, but the common-log stats badges
  are named as usage metrics (`Usage`, `RPM`, `TPM`), not as generic log-count
  or wallet-ledger metrics.
- Non-consume log types such as top-up/refund/error can carry quota or timing
  fields for audit context, but including them in RPM/TPM would blur the
  operational meaning of "current usage".

Decision:

- Preserve current behavior: log stats remain consume-usage stats regardless of
  non-consume `type` filters.
- Treat the `type` argument as endpoint/API compatibility until a dedicated
  generic log-stat endpoint exists.

Implementation:

- `model/log.go`
  - Added a short `SumUsedQuota` comment documenting the consume-usage semantics
    and compatibility-only `logType` argument.
- `model/log_stat_test.go`
  - Added `TestSumUsedQuotaAlwaysReportsConsumeStats` to lock behavior when
    callers pass refund/top-up filters to the stats endpoint.

Deferred scope:

- Did not change UI labels, list filtering, stats API shape, or response fields.
- Did not add a generic log-type stats endpoint.
- Did not change task billing, scheduling, pricing, provider priority,
  account-pool, or channel-pool behavior.

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
  go test ./model ./service ./middleware ./controller
```

Result:

```text
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
```

## Phase 15 - Usage Log Request Metadata Visibility

Status: completed.

Objective:

Review common usage-log detail/table visibility for request correlation fields
added in earlier phases (`request_id`, `upstream_request_id`, and async task
admin metadata). If a fix is needed, keep it display/serialization-only and do
not alter log writes, billing, task settlement, scheduling, pricing, provider
priority, account pools, or channel pools.

Acceptance checks:

```bash
go test ./model ./service ./middleware ./controller
```

Manual review checks:

- Existing request-id filters continue to work.
- User-visible log responses still strip admin-only metadata.
- No schema, billing, routing, account-pool, or channel-pool changes.

Review findings:

- `request_id` and `upstream_request_id` are already present on the backend
  `Log` JSON payload and are rendered at the top of the common usage-log details
  dialog.
- User/self log responses still pass through `formatUserLogs`, which strips
  `Other.admin_info` before returning data to non-admin users.
- Phase 13 async task billing metadata (`admin_info.node_name`) and Phase 10
  saturation audit metadata (`admin_info.quota_saturation`) were persisted, but
  non-top-up common-log details did not have a generic admin-only section to
  render those fields.

Implementation:

- `web/default/src/features/usage-logs/types.ts`
  - Added `QuotaSaturationInfo` and typed `LogOtherData.admin_info.quota_saturation`.
- `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`
  - Added an admin-only runtime metadata section for non-top-up `node_name` and
    quota saturation audit fields (`op`, `kind`, `original`, `clamped`).
  - Reused existing `Runtime`, `Node Name`, `Operation`, `Type`, `Value`, and
    `Result` labels to avoid widening i18n scope.
  - Cleaned local lint issues in the touched dialog file by replacing index
    keys with data-derived keys and extracting the reasoning-effort badge
    variant helper.

Deferred scope:

- Did not change log write paths, task billing, refund/recalculate order,
  schemas, filters, scheduling, pricing, provider priority, account pools, or
  channel pools.
- Did not add a new generic log metadata API.

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
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxlint -c .oxlintrc.json \
  src/features/usage-logs/components/dialogs/details-dialog.tsx \
  src/features/usage-logs/types.ts
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxfmt --check \
  src/features/usage-logs/components/dialogs/details-dialog.tsx \
  src/features/usage-logs/types.ts
```

Result:

```text
ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller

Found 0 warnings and 0 errors.
All matched files use the correct format.
```

Known validation limitation:

`bun run typecheck` still fails on pre-existing frontend baseline issues outside
this phase:

- `src/features/pricing/index.tsx`: implicit `any` for `signal`.
- `src/features/yucore-brand/**`: missing `../data/content` / `./data/content`
  module plus downstream implicit `any` and color-key indexing errors.

## Phase 16 - Frontend Typecheck Baseline Cleanup

Status: completed.

Objective:

Restore the default frontend typecheck baseline by fixing the existing
`pricing` and `yucore-brand` TypeScript blockers discovered during Phase 15.
Keep the work limited to frontend compile/type correctness; do not change
backend billing, logging, task settlement, scheduling, pricing rules, provider
priority, account pools, or channel pools.

Implementation:

- Restored `web/default/src/features/yucore-brand/data/content.ts`, the shared
  YuCore content module imported by the YuCore brand components and pricing
  page.
- Added explicit TypeScript shapes for YuCore signals, metrics, capabilities,
  studio modules, and the studio accent union so downstream component indexing
  remains type-safe.
- Kept the root `.gitignore` unchanged. The new file lives under a `data/`
  directory that is ignored by the repository's broad root rule, so it was
  intentionally added with `git add -f` as the smallest version-control change.
- Did not change backend code, schemas, billing, logging semantics, task
  settlement, scheduling, pricing rules, provider priority, account pools, or
  channel pools.

Acceptance checks:

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bun run typecheck
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxlint -c .oxlintrc.json \
  src/features/yucore-brand/data/content.ts
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxfmt --check src/features/yucore-brand/data/content.ts
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
$ tsgo -b

Found 0 warnings and 0 errors.
All matched files use the correct format.

ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
```

Manual review checks:

- Missing YuCore content module is restored or imports are aligned with the
  current source tree.
- Type fixes do not rewrite YuCore branding behavior or dashboard pricing
  semantics beyond compile correctness.
- No backend, schema, scheduling, billing, account-pool, or channel-pool changes.

Known validation limitation:

- Full-directory `oxlint` and `oxfmt --check` for `src/features/yucore-brand`
  still expose pre-existing lint/format debt outside this phase, including
  nested ternaries, array-index keys, hook dependency warnings,
  `replaceAll` compatibility, non-null assertions, and broad formatting churn.
  Phase 16 intentionally restored the default TypeScript baseline first and
  limited lint/format acceptance to the newly restored module.

## Phase 17 - Frontend Lint/Format Debt Baseline Batch 1

Status: completed.

Objective:

Clean up the existing YuCore frontend lint/format debt in small, mechanical
batches so future frontend changes can use narrower and more reliable
acceptance checks. Keep this stage behavior-preserving and frontend-only; do
not change backend billing, logging, task settlement, scheduling, pricing rules,
provider priority, account pools, or channel pools.

Implementation:

- Cleaned the first low-risk YuCore frontend lint/format batch:
  - `components/yucore-command-center.tsx`
  - `components/yucore-home.tsx`
  - `components/yucore-terminal-card.tsx`
  - `components/yucore-persistent-core.tsx`
  - `i18n/use-yucore-translation.ts`
- Merged duplicate imports and normalized formatter output in the touched files.
- Replaced the terminal-card array-index key with a data-derived key.
- Replaced YuCore i18n interpolation regex `replace` with `replaceAll`.
- Extracted pure helper functions in `yucore-persistent-core.tsx` to remove
  nested ternaries and data-derived React key warnings without changing the dot
  generation formulas.
- Did not keep partial `yucore-studio-workspace.tsx` cleanup in this batch;
  that larger file remains scheduled for a dedicated phase.
- Did not change backend code, schemas, billing, logging semantics, task
  settlement, scheduling, pricing rules, provider priority, account pools, or
  channel pools.

Acceptance checks:

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxlint -c .oxlintrc.json \
  src/features/yucore-brand/components/yucore-command-center.tsx \
  src/features/yucore-brand/components/yucore-home.tsx \
  src/features/yucore-brand/components/yucore-terminal-card.tsx \
  src/features/yucore-brand/i18n/use-yucore-translation.ts \
  src/features/yucore-brand/components/yucore-persistent-core.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxfmt --check \
  src/features/yucore-brand/components/yucore-command-center.tsx \
  src/features/yucore-brand/components/yucore-home.tsx \
  src/features/yucore-brand/components/yucore-terminal-card.tsx \
  src/features/yucore-brand/i18n/use-yucore-translation.ts \
  src/features/yucore-brand/components/yucore-persistent-core.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bun run typecheck
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
Found 0 warnings and 0 errors.
All matched files use the correct format.
$ tsgo -b

ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
```

Manual review checks:

- Fix lint and format debt by category, not by broad UI rewrites.
- Keep YuCore public copy, navigation, routes, and studio behavior unchanged
  unless a lint fix requires a no-op extraction.
- Do not mix this frontend hygiene work with channel runtime, account-pool,
  quota, or scheduling changes.

Known validation limitation:

- Full-directory `oxlint` for `src/features/yucore-brand` still reports
  remaining pre-existing debt after this batch, mostly nested ternaries in
  `yucore-boot-canvas.tsx`, `yucore-entrance-loader.tsx`,
  `yucore-motion-canvas.tsx`, and `yucore-studio-workspace.tsx`, plus several
  Studio hook/key/no-non-null assertion issues.
- Full-directory `oxfmt --check src/features/yucore-brand` still reports
  formatting differences in untouched YuCore files. Phase 17 deliberately
  avoided broad all-directory formatting churn.

## Phase 18 - Frontend Lint/Format Debt Baseline Batch 2

Status: completed.

Objective:

Continue YuCore frontend lint/format cleanup in a dedicated batch for the
heavier animation files. Start with `yucore-boot-canvas` and keep every change
behavior-preserving. Do not change backend billing, logging, task settlement,
scheduling, pricing rules, provider priority, account pools, or channel pools.

Implementation:

- Cleaned `web/default/src/features/yucore-brand/components/yucore-boot-canvas.tsx`.
- Extracted pure helper functions for boot tone selection, shell weight,
  density stride, field coordinates, field blending, ordered lift, particle
  alpha/size scale, and glow color.
- Removed the file's `no-nested-ternary` lint errors without changing particle
  counts, animation order, duration handling, canvas resize behavior, or the
  underlying numeric formulas.
- Ran `oxfmt` only on the touched file. This normalized the full file format,
  so the textual diff is larger than the behavioral change.
- Did not change backend code, schemas, billing, logging semantics, task
  settlement, scheduling, pricing rules, provider priority, account pools, or
  channel pools.

Acceptance checks:

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxlint -c .oxlintrc.json \
  src/features/yucore-brand/components/yucore-boot-canvas.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxfmt --check \
  src/features/yucore-brand/components/yucore-boot-canvas.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bun run typecheck
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
Found 0 warnings and 0 errors.
All matched files use the correct format.
$ tsgo -b

ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller
```

Manual review checks:

- The new helpers are direct condition-to-value extractions from the previous
  expressions.
- Generated particle counts, timing formulas, routes, and canvas lifecycle
  behavior remain unchanged.
- No backend, schema, scheduling, billing, account-pool, or channel-pool
  changes.

Known validation limitation:

- Full-directory `oxlint` for `src/features/yucore-brand` still reports
  remaining pre-existing debt outside this batch, mainly in
  `yucore-entrance-loader.tsx`, `yucore-motion-canvas.tsx`, and
  `yucore-studio-workspace.tsx`.
- Full-directory `oxfmt --check src/features/yucore-brand` still reports
  formatting differences in untouched YuCore files. Phase 18 intentionally kept
  formatting scoped to `yucore-boot-canvas.tsx`.

## Phase 19 - Frontend Lint/Format Debt Baseline Batch 3

Status: completed.

Objective:

Continue YuCore frontend lint/format cleanup with `yucore-entrance-loader.tsx`
as the next isolated animation-file batch. Keep the rewrite mechanical:
deduplicate imports, replace array-index keys with generated stable ids, and
extract pure helpers for nested tone/position/style choices. Do not change
backend billing, logging, task settlement, scheduling, pricing rules, provider
priority, account pools, or channel pools.

Implementation:

- Cleaned
  `web/default/src/features/yucore-brand/components/yucore-entrance-loader.tsx`.
- Merged the duplicate React import into one inline type import.
- Added generated stable `id` fields for loader power dots, orbital dots,
  far-field dots, sphere shards, and sphere dots, then used those ids as React
  keys instead of array indexes.
- Extracted pure helpers for loader tone selection, color mapping, halo bias,
  far-field placement, power/orbital placement, power drift, sphere-shard
  placement, sphere-shard size, sphere-shard drift, and sphere-shard alpha.
- Removed the file's `no-nested-ternary` lint errors without changing particle
  counts, deterministic seeds, coordinate formulas, animation timing, boot
  duration, or displayed copy.
- Ran `oxfmt` only on the touched file. This normalized the full file format,
  so the textual diff is larger than the behavioral change.
- Did not change backend code, schemas, billing, logging semantics, task
  settlement, scheduling, pricing rules, provider priority, account pools, or
  channel pools.

Acceptance checks:

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxlint -c .oxlintrc.json \
  src/features/yucore-brand/components/yucore-entrance-loader.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bunx oxfmt --check \
  src/features/yucore-brand/components/yucore-entrance-loader.tsx
```

```bash
docker run --rm \
  -v "${PWD}:/src" \
  -v yuapi-bun-cache:/root/.bun \
  -v yuapi-web-node-modules:/src/web/node_modules \
  -w /src/web/default oven/bun:1.2.23 \
  bun run typecheck
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
git diff --check
```

Result:

```text
Found 0 warnings and 0 errors.
All matched files use the correct format.
$ tsgo -b

ok   github.com/QuantumNous/new-api/model
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/middleware
ok   github.com/QuantumNous/new-api/controller

git diff --check passed.
```

Manual review checks:

- Prefer pure helper extraction for nested ternaries rather than UI rewrites.
- Keep generated loader particle positions, timing formulas, route copy, and
  boot duration behavior unchanged unless a lint fix has a clearly equivalent
  rewrite.
- Do not mix this frontend hygiene work with channel runtime, account-pool,
  quota, or scheduling changes.

Known validation limitation:

- Full-directory `oxlint` for `src/features/yucore-brand` still reports
  remaining pre-existing debt outside this batch, mainly in
  `yucore-motion-canvas.tsx` and `yucore-studio-workspace.tsx`.
- Full-directory `oxfmt --check src/features/yucore-brand` still reports
  formatting differences in untouched YuCore files. Phase 19 intentionally kept
  formatting scoped to `yucore-entrance-loader.tsx`.

## Phase 20 - Tiered Billing Pre-Consume Estimate

Status: completed.

Objective:

Backport the small upstream `3fbad6a7` tiered billing safety fix so paid
tiered-expression models still reserve a plausible completion-token quota when
the client omits `max_tokens`. Keep the change narrow to pricing pre-consume
math and tests. Do not change protocol routing, channel selection, provider
priority, account pools, channel pools, scheduler behavior, schemas, or
production data.

Implementation:

- `relay/helper/price.go`
  - Added `defaultTieredPreConsumeMaxTokens = 8192`.
  - `modelPriceHelperTiered` now uses explicit `meta.MaxTokens` when present.
  - When `max_tokens` is omitted and group ratio is non-zero, tiered
    pre-consume estimates completion cost with the default 8192 tokens.
  - Zero-ratio/free groups keep zero completion fallback so free pre-consume
    behavior stays unchanged.
- `relay/helper/price_test.go`
  - Added coverage for paid omitted-`max_tokens` fallback.
  - Added coverage proving explicit `max_tokens` remains authoritative.
  - Added coverage proving zero-ratio/free groups still pre-consume zero.
- Did not change frontend code, protocol routing, channel selection, provider
  priority, scheduler behavior, account pools, channel pools, schemas, or
  production data.

Acceptance checks:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./service ./controller ./model
```

```bash
git diff --check
```

Result:

```text
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/model

git diff --check passed.
```

Manual review checks:

- Paid groups with omitted `max_tokens` use the fallback completion estimate for
  tiered pre-consume.
- Explicit `max_tokens` remains authoritative.
- Free groups / zero group ratio still pre-consume zero.
- No frontend, scheduler, account-pool, channel-pool, protocol-routing, schema,
  or production-data changes.

Known validation limitation:

- This phase only backports the narrow tiered pre-consume fallback from
  upstream `3fbad6a7`.
- It does not change model exposure, group ratio strategy, protocol routing,
  provider adapter behavior, or live channel settings.

## Phase 21 - Production Protocol Path Triage

Status: completed.

Objective:

Return to production protocol and strategy work by auditing the remaining
non-plus/pro routes that could be affected by future policy updates: image
generation, Responses/compact Responses, Anthropic/Claude-style traffic,
Codex/subscription-style channels, Kiro/Windsurf/provider adapters, and
admin-only media tooling.

Accepted narrow fix:

- Correct the protocol capability surface for Responses compact models.
- Models ending in the compact suffix should advertise
  `openai-response-compact`, not generic `openai`.
- Codex/subscription-style base models should advertise `openai-response`,
  because the Codex adapter only serves `/v1/responses` and
  `/v1/responses/compact`.

Boundary:

- Do not change frontend/YuCore UI.
- Do not apply the local YuCore UI stash
  `wip: phase 20 yucore motion canvas lint cleanup`.
- Do not change production data, provider priority, account pools, channel
  pools, scheduler behavior, group/model routing, or live channel settings.
- Do not backport upstream `246d62aa5` in this phase; it only deletes dead
  files and does not improve runtime behavior.
- Deployment may replace only the YuAPI service image/container after tests;
  MySQL, Redis, volumes, and live data must remain untouched.

Acceptance checks:

```bash
go test ./common
go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model
git diff --check
```

Manual review checks:

- Document which protocol paths are in scope and which are explicitly deferred.
- Prefer the small protocol capability fix over broad routing or adapter
  rewrites.
- Keep plus/pro channel strategy and account-pool settings unchanged unless a
  later phase explicitly targets them.
- Do not resume YuCore UI lint work in this production phase; the local UI WIP
  is preserved in stash `wip: phase 20 yucore motion canvas lint cleanup`.

Upstream triage:

- Fetched `origin/main` again on 2026-07-10.
- The incremental range `a79f96919..origin/main` contains only upstream
  `246d62aa5`, which removes dead files resurrected by the v1.0 launch commit.
- Did not backport it in this production deploy because it has no runtime
  protocol, routing, billing, Responses, channel capability, or scheduler
  behavior change.

Implementation:

- `common/model.go`
  - Added `OpenAIResponseCompactModelSuffix`.
  - Added `IsOpenAIResponseCompactModel` so protocol capability code can check
    compact models without importing `ratio_setting` into `common`.
- `setting/ratio_setting/compact_suffix.go`
  - Reused the shared common suffix constant to keep the existing
    `ratio_setting.CompactModelSuffix` API stable.
- `common/endpoint_type.go`
  - Compact-suffixed models now advertise
    `openai-response-compact`.
  - Codex channels now advertise `openai-response` for base models and
    `openai-response-compact` for compact-suffixed models.
  - Existing OpenAI, xAI, image, Jina, Claude, Gemini, and Sora endpoint
    defaults are otherwise unchanged.
- `common/endpoint_type_test.go`
  - Added focused coverage for compact model endpoint exposure.
  - Added Codex base/compact endpoint coverage.
  - Added regression coverage for existing OpenAI response-only and xAI
    defaults.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./common
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model
```

```bash
git diff --check
```

Result:

```text
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/model

git diff --check passed.
```

Deployment:

- Pushed code/docs commit:
  `4bb40fda0 fix: expose compact response endpoint metadata`.
- Built production image from that commit:
  `newapi:channel-pool-runtime-20260710-4bb40fda0`.
- Backed up compose before changing the image line:
  `/opt/newapi/backups/docker-compose-before-phase21-compact-20260710094446.yml`.
- Updated only the `newapi` service image in `/opt/newapi/docker-compose.yml`.
- Ran `docker compose up -d newapi`.
- `newapi-mysql`, `newapi-redis`, Sub2API retained services, volumes, and live
  data were not modified.

Production smoke:

```text
newapi image: newapi:channel-pool-runtime-20260710-4bb40fda0
newapi health: healthy
newapi-mysql: healthy, unchanged
newapi-redis: healthy, unchanged
local /: HTTP 200
local /api/pricing: HTTP 200
local /api/status: HTTP 200
local unauth /v1/models: HTTP 401
domain https://api.dtrljm.com/: HTTP 200
```

Protocol metadata smoke:

```text
/api/pricing compact items: 1
gpt-5.5-openai-compact: ["openai-response-compact"]
```

## Phase 22 - Strategy And Protocol Bug Sweep

Status: completed.

Objective:

After Phase 21 is deployed and smoked, continue backend-only strategy/protocol
hardening. Start with request-path capability checks and adapter-specific edge
cases for non-plus/pro traffic, then pick one small fix. Do not resume YuCore
UI work, do not change production data, and do not alter account-pool or
channel-pool scheduling semantics unless Phase 22 explicitly narrows to that
topic first.

Accepted narrow fix:

- Correct endpoint metadata for embedding models.
- The runtime and channel-test paths already recognize embedding requests and
  use `/v1/embeddings`.
- Pricing/model metadata should expose `embeddings` as the first supported
  endpoint for embedding-like models instead of advertising only generic
  chat/OpenAI endpoints.

Boundary:

- Do not change real request routing, provider priority, model mapping,
  account pools, channel pools, scheduler behavior, billing formulas, schemas,
  production data, or YuCore UI.
- Do not add OpenAI video channel-test behavior in this phase; `openai-video`
  endpoint metadata remains a separate candidate because video submit/fetch
  paths involve async task request bodies.
- Deployment may replace only the YuAPI `newapi` service image/container after
  tests; MySQL, Redis, volumes, retained Sub2API services, and live data must
  remain untouched.

Acceptance checks:

```bash
go test ./common
go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model
git diff --check
```

Manual review checks:

- Embedding-like model names advertise `embeddings` first.
- Existing compact, Codex, image, response-only, xAI, Claude, Gemini, and Sora
  endpoint behavior remains unchanged except for embedding metadata
  prepending.
- No scheduler/account-pool/channel-pool code is touched.

Upstream triage:

- Fetched `origin/main` again on 2026-07-10.
- `origin/main` remained at `246d62aa5`.
- The range `246d62aa5..origin/main` contained no new commits, so Phase 22 did
  not backport upstream code.

Implementation:

- `common/model.go`
  - Added `EmbeddingModels` patterns for `embedding`, `embed`, `prefix:m3e`,
    and `bge-`.
  - Added `IsEmbeddingModel`.
- `common/endpoint_type.go`
  - Prepends `embeddings` to supported endpoint metadata for embedding-like
    models.
  - Leaves the existing channel-native endpoint list in place after
    `embeddings`, so Gemini embedding models still retain their Gemini/OpenAI
    metadata after the explicit embeddings endpoint.
- `common/endpoint_type_test.go`
  - Added OpenAI, Gemini, and BGE embedding endpoint metadata coverage.

Verification:

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./common
```

```bash
docker run --rm -e GOPROXY=https://goproxy.cn,direct \
  -e GOSUMDB=sum.golang.google.cn \
  -v "${PWD}:/src" \
  -v yuapi-go-mod-cache:/go/pkg/mod \
  -v yuapi-go-build-cache:/root/.cache/go-build \
  -w /src golang:1.25.1 \
  go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model
```

```bash
git diff --check
```

Result:

```text
ok   github.com/QuantumNous/new-api/common
ok   github.com/QuantumNous/new-api/relay/helper
ok   github.com/QuantumNous/new-api/relay/channel
ok   github.com/QuantumNous/new-api/relay
ok   github.com/QuantumNous/new-api/service
ok   github.com/QuantumNous/new-api/controller
ok   github.com/QuantumNous/new-api/model

git diff --check passed.
```

Deployment:

- Pushed code/docs commit:
  `0809480bc fix: expose embedding endpoint metadata`.
- Built production image from that commit:
  `newapi:channel-pool-runtime-20260710-0809480bc`.
- Backed up compose before changing the image line:
  `/opt/newapi/backups/docker-compose-before-phase22-embedding-20260710101008.yml`.
- Updated only the `newapi` service image in `/opt/newapi/docker-compose.yml`.
- Ran `docker compose up -d newapi`.
- `newapi-mysql`, `newapi-redis`, retained Sub2API services, volumes, and live
  data were not modified.

Production smoke:

```text
newapi image: newapi:channel-pool-runtime-20260710-0809480bc
newapi health: healthy
newapi-mysql: healthy, unchanged
newapi-redis: healthy, unchanged
local /: HTTP 200
local /api/pricing: HTTP 200
local /api/status: HTTP 200
domain https://api.dtrljm.com/: HTTP 200
```

Protocol metadata smoke:

```text
/api/pricing items: 32
/api/pricing embedding items: 0
/api/pricing compact items: 1
gpt-5.5-openai-compact: ["openai-response-compact"]
```

Note:

- The live production dataset currently exposes no enabled embedding models in
  `/api/pricing`, so embedding endpoint metadata is verified by the new
  `common` unit tests and will appear when embedding-like abilities are enabled.

## Phase 23 - Video Endpoint Metadata Triage

Status: planned.

Next phase objective:

Review `openai-video` endpoint metadata and channel-test behavior separately.
The endpoint type exists and Sora channels advertise it, but video submit/fetch
paths involve async task request bodies and should not be folded into Phase 22's
embedding metadata fix. Decide whether the safe next step is only default
endpoint metadata, channel-test request construction, or a broader async-video
smoke harness. Do not change account pools, channel-pool scheduling, live
channel priorities, production data, or YuCore UI.

## Phase 24 - Parent Protocol And Billing Contract Backport

Status: completed.

Objective:

Selectively backport active protocol and billing correctness fixes from the
parent project without importing its broad advanced-custom routing changes or
any local experimental UI. The batch covers the active OpenAI-compatible,
Claude, and Gemini relay contracts, while preserving the existing private-group
and model-mapping behavior.

Upstream triage:

- The current branch already carries the functional equivalent of parent
  `48068ce92` and `92d3c9d18` through local commit `99b79af64`:
  native OpenAI `cache_write_tokens` is normalized through
  `CacheCreationTokensTotal`, overlap cannot create a negative uncached charge,
  and Responses/compact cache metadata is propagated.
- Parent `1086038f5` fixes a separate OpenAI Responses-to-Chat streaming bug.
  Its refactored file path does not exist locally, so the same narrowly scoped
  behavior was adapted to the existing compatible converter instead of
  importing the parent protocol-registry rewrite.
- No post-`c36418c86` parent change was selected for the Claude or Gemini
  conversion implementation. Their active cache and Responses conversion
  contracts were verified by the focused suites below.
- Parent advanced-custom discovery and routing work remains deferred to Phase
  25. No private-group grants, model aliases, channel groups, account pools,
  channel priorities, pricing, or production data were changed.

Implementation:

- `service/relayconvert/responses_to_chat.go`
  - Reuses a streaming tool-call object when a later Responses event identifies
    it through the same item ID or call ID under a different stream key.
  - Aliases the new key to the shared state before allocating an index, so
    tool name, argument cursor, sent state, and index remain stable.
- `service/relayconvert/chat_responses_compat_test.go`
  - Covers call-ID aliasing across output indexes.
  - Covers terminal `response.completed` replay by item ID and terminal
    `response.done` replay by call ID, including argument deltas and the final
    `tool_calls` finish reason.
- `.dockerignore`
  - Excludes the whole generated `output/` tree from production Docker build
    context. This prevents local posters, image generation output, and local
    experiments from entering an image layer.

Verification:

```text
go test ./service/relayconvert -count=1
go test ./dto ./service ./relay/channel/openai ./relay/channel/claude ./relay/channel/gemini -count=1
go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model ./middleware -count=1
git diff --check

all commands passed.
```

Candidate build:

```text
image: newapi:protocol-billing-contracts-20260724
image id: sha256:bb5235ed9682e88d87d4f6be7e7bd7c17fb27c44367c875cc23935d8b695d657
```

Deployment:

- Pushed the backend-only branch:
  `ruoyu/codex/protocol-billing-contracts-20260724`.
- Loaded the verified image on production after a gzip integrity check.
- Backed up compose before changing the service image:
  `/opt/newapi/backups/docker-compose-before-protocol-billing-20260724130243.yml`.
- Changed only the `newapi` image in `/opt/newapi/docker-compose.yml` and ran
  `docker compose up -d newapi`.
- `newapi-mysql`, `newapi-redis`, production volumes, channel settings,
  private groups, and experimental UI assets were not modified.
- The prior `newapi:vip-direct-20260723` image remains locally available for
  immediate compose-file rollback.

Production smoke:

```text
newapi image: newapi:protocol-billing-contracts-20260724
newapi health: healthy
local http://127.0.0.1:3001/api/status: 200
https://api.yuaiapi.com/: 200
https://api.yuaiapi.com/v1/models without token: 401
https://vip.yuaiapi.com/v1/models without token: 401
```

## Phase 25 - Advanced-Custom Routing Compatibility

Status: completed.

Objective:

Safely assess and selectively adopt parent advanced-custom upstream-model
discovery and path-matching behavior, beginning with the model-fetch work from
`a6cf42c0f`. This is a routing compatibility phase, not a broad protocol
converter replacement and not a UI rewrite.

Implementation:

- `dto/channel_settings.go`
  - Defines an exact optional `/v1/models` discovery route.
  - Rejects converter and `{model}` template use for that route.
- `model/channel.go`
  - Keeps legacy advanced-custom channels valid.
  - Requires the explicit discovery route only when that channel enables
    scheduled upstream-model checks.
- `relay/channel/advancedcustom/adaptor.go`
  - Builds model discovery URLs and route authentication without resolving or
    changing a customer request route.
- `controller/channel_upstream_update.go`
  - Uses the configured discovery route, selected enabled key, route auth,
    channel header overrides, `Host` override, and channel proxy.
  - Rejects malformed, missing, null, empty, or ID-less model lists before the
    existing update workflow can stage removals.
  - Removes URL wrappers and API-key forms from transport errors.

Compatibility and safety checks:

- A private group can discover and route only models backed by enabled channels
  in that same authorized group. No cross-group fallback is permitted.
- Downstream `/v1/models` and pricing exposure remain stable for each existing
  user-group grant and model alias; a model listed to a downstream consumer
  must be callable through an eligible channel in that group.
- Advanced-custom upstream model fetch is explicit, cacheable, and failure
  tolerant. A failed discovery request cannot erase existing configured models
  or advertise unverified models.
- Route matching preserves method and path semantics for Chat, Responses,
  Claude, and Gemini-compatible traffic, with regression fixtures for current
  aliases and private groups.
- The release has a channel-level rollback path and does not alter account
  credentials, group grants, production channel priorities, pricing, or the
  local experimental UI.

Verification:

```text
go test ./dto ./model -run 'AdvancedCustom.*ModelList|AdvancedCustomChannelRequiresModelList' -count=1
go test ./relay/channel/advancedcustom -run '^TestAdaptorBuildModelListRequest' -count=1
go test ./controller -run 'AdvancedCustom|ParseOpenAIModelIDs|FailedAdvancedCustomDetection' -count=1
go test ./controller -run '^(TestGetUserModelsFiltersByRequestedGroup|TestGetUserModelsExpandsAutoGroupsInConfiguredOrder|TestListModels.*)$' -count=1
go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model ./middleware -count=1
git diff --check

all commands passed.
```

Candidate build:

```text
image: newapi:advanced-custom-routing-20260724
image id: sha256:c484082fbc99a5cc8e08c3a1acf6d05fec38c102f52b5f526dc3756e4094f329
Docker build context: generated output and local experimental UI excluded.
```

Deployment:

- Pushed backend-only branch:
  `ruoyu/codex/advanced-custom-routing-20260724`.
- Immediately before deployment, production had no channels with type
  `Advanced Custom`; no live group, model, key, account-pool, or channel
  configuration was migrated or edited.
- Loaded the image after gzip archive verification.
- Backed up compose before replacing only the `newapi` image:
  `/opt/newapi/backups/docker-compose-before-advanced-custom-20260724185010.yml`.
- `newapi-mysql`, `newapi-redis`, production volumes, channel priorities,
  pricing, private group grants, and experimental UI assets were unchanged.
- The prior `newapi:protocol-billing-contracts-20260724` image remains on the
  host for immediate compose-file rollback.

Production smoke:

```text
newapi image: newapi:advanced-custom-routing-20260724
newapi health: healthy
local http://127.0.0.1:3001/api/status: 200
https://api.yuaiapi.com/: 200
https://api.yuaiapi.com/v1/models without token: 401
https://vip.yuaiapi.com/v1/models without token: 401
advanced-custom channels: none enabled or configured at deployment time
```

Channel-level rollback:

- Manual discovery is read-only. It does not overwrite channel `Models`,
  `Group`, `ModelMapping`, account credentials, or channel priority.
- Disable `upstream_model_update_check_enabled` on a channel to stop scheduled
  discovery. Existing channel models remain unchanged.
- If service rollback is needed, restore the backup compose file (or its
  `newapi` image line) and run `docker compose up -d newapi`.

## Phase 26 - OpenAI Video Endpoint Compatibility

Status: completed.

Objective:

Verify the existing Sora/OpenAI asynchronous video relay contract and align
endpoint metadata with only the two models that are proven to use that path:
`sora-2` and `sora-2-pro`. The synchronous channel-test implementation must
reject `openai-video` explicitly instead of sending a text-shaped request to a
video endpoint or creating a paid task that it cannot poll and settle.

Audited existing behavior:

- `POST /v1/videos` and `GET /v1/videos/:task_id` already enter the task relay
  through the video router and distributor.
- Video submit extracts the requested multipart or JSON model; fetch restores
  the origin model for model-limited tokens.
- Both `ChannelTypeSora` and `ChannelTypeOpenAI` resolve to the Sora task
  adaptor, including the normal async billing and task-fetch paths.

Boundary:

- Do not change async task submission, task persistence, polling, per-call
  billing, account pools, groups, model mapping, routing priority, affinity,
  channel configuration, or production data.
- Do not add unverified OpenAI-compatible video model families; metadata is
  limited to the exact supported Sora models.
- Do not modify or include local experimental UI, posters, generated output,
  or `web/experimental` in commits, builds, or deployment.

Implementation:

- `common/model.go`
  - Adds an exact, case-normalized `IsOpenAIVideoModel` predicate for
    `sora-2` and `sora-2-pro` only.
- `common/endpoint_type.go`
  - Reports `openai-video` for those verified models when they are provided
    through an OpenAI channel. Existing OpenAI text, Responses, compact,
    image, and embedding endpoint metadata is unchanged.
- `controller/channel-test.go`
  - Rejects explicit `openai-video` tests before user/cache lookup or any
    upstream request. Sora channels are also listed with the other async
    channel types that the synchronous tester must not probe.
- `middleware/distributor_video_test.go`
  - Covers multipart OpenAI-video model extraction and video submit relay-mode
    selection, plus model-limited video fetches resolving the stored origin
    model without selecting a new channel.
- `relay/relay_adaptor_test.go`
  - Covers the existing Sora task-adaptor selection for both Sora and OpenAI
    channel types.

Verification:

```text
go test ./common -run '^TestOpenAIChannelSoraModelsUseVideoEndpoint$' -count=1
go test ./controller -run '^(TestChannelRejectsSoraAsyncVideoTestBeforeUserLookup|TestChannelRejectsOpenAIVideoEndpointBeforeUserLookup)$' -count=1
go test ./middleware -run '^(TestGetModelRequestOpenAIVideoSubmitUsesMultipartModel|TestGetModelRequestOpenAIVideoFetchUsesStoredOriginModel)$' -count=1
go test ./common ./middleware ./relay ./relay/channel/task/sora ./controller -count=1
git diff --check

all passed; the Sora adaptor package has no direct test files.
```

Candidate build:

```text
image: newapi:video-endpoint-compatibility-20260724
image id: sha256:b4938b4f549d0a5cbdce3845b7cd14e25575d5458f3c8ddb7e20dc4bbba4c084
```

Deployment:

- Pushed backend-only branch commit `0cb78db25` to
  `ruoyu/codex/video-endpoint-compatibility-20260724`.
- Verified the compressed image before loading it on production.
- Backed up the compose file before changing only the `newapi` image line:
  `/opt/newapi/backups/docker-compose-before-video-endpoint-compatibility-20260724203722.yml`.
- Ran `docker compose up -d newapi`; MySQL, Redis, volumes, channel settings,
  private group grants, account pools, and experimental UI assets were not
  changed.

Production smoke:

```text
newapi image: newapi:video-endpoint-compatibility-20260724
newapi health: healthy
local http://127.0.0.1:3001/api/status: 200
https://api.yuaiapi.com/: 200
https://vip.yuaiapi.com/v1/models without token: 401
```

Rollback:

- Restore the backup compose file (or only its prior `newapi` image line) and
  run `docker compose up -d newapi`.

## Phase 27 - Mapped Model Response Privacy

Status: completed locally, pending production deployment.

Objective:

Prevent a channel's private upstream model mapping from appearing in ordinary
OpenAI-compatible API responses. Clients must receive the public model name
they requested, while upstream request construction, token estimation, billing,
and administrator routing diagnostics retain the actual upstream model name.

Implementation:

- `relay/common/relay_info.go`
  - Added `ClientResponseModelName`, which returns `OriginModelName` only for
    an active model mapping and otherwise preserves `UpstreamModelName`.
- `relay/helper/client_response_model.go`
  - Added a structured payload normalizer for the protocol fields `model` and
    `response.model`. It ignores unmapped requests, missing public names, and
    non-JSON SSE control data; it never rewrites user content such as
    `message.model`.
- `relay/channel/openai/relay-openai.go`
  - Normalizes raw and formatted Chat completion output, including streamed
    chunks and synthetic final usage chunks.
- `relay/channel/openai/relay_responses.go`
  - Normalizes Responses JSON output and streamed `response.*` events after
    image-status normalization.
- `relay/channel/openai/responses_via_chat.go` and
  `relay/channel/openai/chat_via_responses.go`
  - Chat/Responses conversions now use the public model only for outgoing DTO
    and stream state. Internal calls to `ResponseText2Usage` still use the
    upstream model for the existing tokenizer behavior.

Regression coverage:

- Direct OpenAI Chat, Responses, and Responses SSE output return the public
  model for a mapped request.
- Chat-to-Responses and Responses-to-Chat conversion output return the public
  model.
- Unmapped output, blank public names, non-JSON stream data, and unrelated
  nested fields remain unchanged.

Verification:

```text
go test ./relay/helper ./relay/channel/openai -count=1
go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model ./middleware -count=1
go build ./...
git diff --check

all passed.
```

Deployment boundary:

- No production database, channel mapping, group, price, account-pool, Redis,
  or UI data was changed.
- No `web/experimental/`, generated `output/`, poster, or local experimental
  asset is included in this backend-only phase.
- Production deployment requires a fresh image, compose backup, and endpoint
  smoke checks before replacing only the `newapi` container.
