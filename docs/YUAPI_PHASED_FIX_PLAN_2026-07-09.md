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

Status: planned.

Next phase objective:

Harden YuAPI channel-pool scheduling visibility without changing the current
plus/pro routing policy. Add enough counters/log context/tests to explain why a
candidate was skipped, full, cooled down, selected, or released, so future
account-pool tuning can be done from evidence instead of guesswork.

Boundary:

- Do not change provider priority, model mapping, group routing, or billing
  formulae in this phase.
- Do not delete or rewrite existing plus/pro channels.
- Keep lease acquire/release behavior explicit and covered by focused tests.

Planned acceptance checks:

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
