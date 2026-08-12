# Stream Affinity Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve response-chain channel affinity and conservative billing across interrupted OpenAI Responses streams without exposing upstream infrastructure or changing the production brand UI.

**Architecture:** Extend `RelayInfo` with explicit evidence that a real upstream response ID was observed, then let the controller commit either normal successful affinity or a short-lived response-chain-only provisional mapping. Treat all missing-terminal outcomes as usage-unknown and preserve frozen pre-consumption whenever upstream activity makes billing ambiguous. Keep public stream failures generated from fixed local constants, and verify the backend-only candidate against the production UI baseline before any deployment decision.

**Tech Stack:** Go 1.22+, Gin, testify, existing hybrid Redis/in-memory channel-affinity cache, Docker/Playwright only for final local candidate verification.

---

### Task 1: Capture real upstream response IDs before terminal completion

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/openai/relay_responses.go`
- Test: `relay/channel/openai/relay_responses_test.go`
- Test: `relay/common/relay_info_test.go`

- [ ] **Step 1: Write failing tests for observed response IDs and incomplete billing evidence**

Add deterministic table cases showing that `response.created` followed by EOF, client cancellation, handler stop, timeout, and scanner failure retain the real response ID and set `PreservePreConsumedQuota`; a stream without a real ID must leave it empty. Assert a completed stream still records terminal success and authoritative usage.

- [ ] **Step 2: Run the focused tests and capture RED**

Run:

```powershell
go test ./relay/channel/openai ./relay/common -run 'ResponsesStream|RelayInfo' -count=1
```

Expected: FAIL because incomplete streams currently publish `ChannelAffinityResponseID` only after terminal success and exclude client-gone/handler-stop from the pre-consumption floor.

- [ ] **Step 3: Implement the minimal relay state**

Add an explicit boolean such as `ChannelAffinityResponseIDObserved` to `RelayInfo`, reset it in `InitChannelMeta`, and set it only when an actual upstream Responses envelope contains a non-empty ID. Preserve pre-consumption for every non-terminal outcome after upstream stream activity. Never set the observed flag for a synthetic local fallback ID.

- [ ] **Step 4: Run focused tests and capture GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add relay/common/relay_info.go relay/common/relay_info_test.go relay/channel/openai/relay_responses.go relay/channel/openai/relay_responses_test.go
git commit -m "fix: retain interrupted response identity"
```

### Task 2: Record short-lived provisional response-chain affinity

**Files:**
- Modify: `service/channel_affinity.go`
- Test: `service/channel_affinity_template_test.go`
- Modify: `controller/relay.go`
- Test: `controller/relay_retry_test.go`

- [ ] **Step 1: Write failing service tests for response-chain-only provisional mapping**

Add a test API `RecordProvisionalResponseChainAffinity(c, channelID, responseID)` through the desired behavior. Assert it:

- scopes by token, group, model, and response ID;
- routes the next `previous_response_id` request to the same channel;
- does not create or refresh the request's primary affinity key;
- caps TTL at 15 minutes;
- ignores empty IDs and missing rule context.

- [ ] **Step 2: Write failing controller tests for incomplete-stream affinity**

Extend `TestCommitResponseChainAffinityOutcome` so `response.created` plus EOF/client-gone records provisional affinity, while an incomplete stream without an observed real ID does not. Completed streams must continue using normal affinity.

- [ ] **Step 3: Run the focused tests and capture RED**

```powershell
go test ./service ./controller -run 'ProvisionalResponseChainAffinity|CommitResponseChainAffinityOutcome' -count=1
```

Expected: FAIL because no provisional response-chain API or controller outcome exists.

- [ ] **Step 4: Implement the minimal affinity path**

Keep `RecordChannelAffinity` unchanged for normal success. Add a response-chain-only cache write that reuses the existing scoped-key builder and hybrid cache but caps TTL at 900 seconds. Update `commitChannelAffinityOutcome` to:

- commit normal affinity on terminal success;
- otherwise record provisional response-chain affinity only when a real observed ID exists and a channel was selected;
- never mark the entire request successful for an incomplete stream.

- [ ] **Step 5: Run focused and adjacent affinity tests**

```powershell
go test ./service ./controller -run 'ChannelAffinity|RelayRetry' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add service/channel_affinity.go service/channel_affinity_template_test.go controller/relay.go controller/relay_retry_test.go
git commit -m "fix: preserve response-chain channel affinity"
```

### Task 3: Enforce sanitized incomplete-stream errors

**Files:**
- Modify: `relay/channel/openai/relay_responses.go`
- Test: `relay/channel/openai/relay_responses_test.go`

- [ ] **Step 1: Write failing leakage tests**

Seed upstream failure inputs with a fake URL, IP, channel name/ID, API key, Authorization value, raw HTML body, redirect, and transport error. Assert the public SSE stream contains exactly one locally generated `response.failed` event with stable code `upstream_stream_incomplete`, a generic message, and no seeded secret substring. Assert client-gone performs no extra write.

- [ ] **Step 2: Run the focused tests and capture RED**

```powershell
go test ./relay/channel/openai -run 'ResponsesStream.*Sanitized|ResponsesStream.*ClientGone' -count=1
```

Expected: FAIL because the current generated failure uses generic `server_error` and lacks the complete leakage contract.

- [ ] **Step 3: Implement fixed public error construction**

Construct the public failure only from fixed local constants plus an already-public real response ID or local gateway request ID. Do not interpolate `StreamStatus.Err`, upstream body text, URL, channel metadata, or request headers. Keep detailed causes confined to existing internal logging with bounded local previews.

- [ ] **Step 4: Run focused tests and capture GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add relay/channel/openai/relay_responses.go relay/channel/openai/relay_responses_test.go
git commit -m "fix: sanitize incomplete stream failures"
```

### Task 4: Lock retry and billing invariants for incomplete streams

**Files:**
- Modify: `controller/relay.go` only if tests expose a missing guard
- Test: `controller/relay_retry_test.go`
- Modify: `service/text_quota.go` only if tests expose a missing floor/unknown path
- Test: `service/text_quota_test.go`

- [ ] **Step 1: Write failing/characterization tests for retry safety**

Cover downstream cancellation, upstream headers/events received, missing terminal marker, handler-stop, timeout, EOF, and scanner error. Assert no retry or channel switch occurs after ambiguous upstream activity. Preserve existing safe pre-write retry behavior.

- [ ] **Step 2: Write billing/cache-stat tests**

For every incomplete end reason, assert final quota is not lower than `FinalPreConsumedQuota`, no refund path is selected, and affinity cache statistics increment `Unknown` rather than `Hit`, `Miss`, or confirmed token totals. Assert neither `cl100k_base` nor `o200k_base` estimates create provider-confirmed cached tokens.

- [ ] **Step 3: Run focused tests and capture RED or verified characterization**

```powershell
go test ./controller ./service -run 'Retry|Incomplete|PreConsumedQuota|ChannelAffinityUsage' -count=1
```

Expected: any newly exposed gap fails for the missing invariant; already-correct safety behavior passes as characterization and remains unchanged.

- [ ] **Step 4: Implement only missing guards**

Use existing `StreamTerminalMarkersRequired`, stream status, request context, and pre-consumption floor. Do not add a tokenizer migration, pricing expression change, refund, or POST reconciliation.

- [ ] **Step 5: Run focused tests and capture GREEN**

Run the command from Step 3. Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add controller/relay.go controller/relay_retry_test.go service/text_quota.go service/text_quota_test.go
git commit -m "fix: keep incomplete stream billing conservative"
```

### Task 5: Backend regression and scope verification

**Files:**
- Verify only; no planned production-file additions

- [ ] **Step 1: Run affected package tests**

```powershell
go test ./relay/common ./relay/helper ./relay/channel/openai ./controller ./service -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full backend tests**

```powershell
go test ./... -count=1
```

Expected: PASS. If an unrelated pre-existing package failure appears, record it separately and do not hide it.

- [ ] **Step 3: Verify backend-only diff and formatting**

```powershell
gofmt -d relay/common/relay_info.go relay/channel/openai/relay_responses.go service/channel_affinity.go controller/relay.go service/text_quota.go
git diff --check
git diff --name-only b66e99504...HEAD
```

Expected: no formatting/diff errors and no files under `web/`, theme, branding, docs UI, canvas, or frontend locale paths except the approved design/plan documents.

- [ ] **Step 4: Request independent code review and address only confirmed findings**

Review must focus on retry/refund safety, response-chain cache continuity, leakage, cross-request scoping, Redis/in-memory parity, and database-neutral behavior. Any fix starts with a failing regression test.

### Task 6: Build local candidate and verify production brand baseline

**Files:**
- Create only ephemeral build artifacts outside Git
- Do not modify Caddy or the production container

- [ ] **Step 1: Record production UI fingerprints read-only**

Capture current production homepage HTML hash, referenced static asset names/hashes, and screenshots for sign-in, registration, console layout, API-key page, system settings, infinite canvas, documentation, animations, and custom model configuration. Do not capture credentials, tokens, headers, database bodies, or access logs.

- [ ] **Step 2: Build and start a loopback-only candidate**

Build from the current branch and bind the candidate only to an unused `127.0.0.1` port. Reuse a safe local test database/configuration; never point the candidate at production traffic.

- [ ] **Step 3: Compare candidate with production baseline**

Use Playwright screenshots and static asset fingerprints. Expected: no branch-attributable brand/UI/layout/function differences. Any mismatch blocks deployment and is investigated before further action.

- [ ] **Step 4: Exercise stream regressions against local fixtures**

Verify terminal success, `response.created` then EOF, client disconnect, timeout, sanitized SSE error, provisional `previous_response_id` affinity, no retry, and conservative quota behavior without real upstream spend.

- [ ] **Step 5: Report candidate result and wait for explicit deployment approval**

Provide the local URL, commit range, test output summary, UI comparison result, and rollback reference. Do not modify production, Caddy, or remove old images/containers without a separate explicit confirmation.
