# Estimated Usage and Affinity Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recover authoritative usage after downstream disconnects when possible, otherwise record explicit non-zero estimated token usage without charging below the frozen reservation, while preserving provider cache locality through cascading channel-affinity lookup.

**Architecture:** Extend the existing bounded stream-recovery lifecycle already proven on the production-baseline lineage, integrating it with the current atomic request-attempt and conservative settlement code. Refactor affinity selection to evaluate every applicable stable source in priority order and record provisional mappings for both response-chain and the request's stable primary identity. Keep changes backend-only and schema-free.

**Tech Stack:** Go 1.22+, Gin, GORM, existing `cachex.HybridCache`, `testify/require`, `testify/assert`, Docker, Caddy.

---

### Task 1: Cascading channel-affinity lookup

**Files:**
- Modify: `setting/operation_setting/channel_affinity_setting.go`
- Modify: `service/channel_affinity.go`
- Modify: `service/channel_affinity_template_test.go`
- Modify: `controller/relay.go`
- Modify: `controller/relay_retry_test.go`

- [ ] **Step 1: Write failing lookup-order tests**

Add deterministic tests proving that a request containing a missing `prompt_cache_key` and a known `previous_response_id` selects the response-chain channel, that conversation wins over a cache-key miss, and that all mappings remain scoped by token, group, and model.

- [ ] **Step 2: Run tests and capture RED**

Run `go test ./service ./controller -run 'Affinity.*Cascade|Affinity.*Priority|Provisional.*Primary' -count=1`.

Expected: FAIL because the current rule stops after the first present source and provisional completion records only response-chain affinity.

- [ ] **Step 3: Implement cascading lookup**

Order Responses sources as response chain, conversation, prompt cache key, then explicit session/thread identifiers. Continue after a cache miss, preserve redacted per-source metadata, and stop only on a hit or after exhausting applicable sources. Record provisional response-chain and existing primary identity mappings with a 15-minute maximum TTL.

- [ ] **Step 4: Run focused and adjacent tests**

Run `go test ./service ./controller -run 'Affinity|Retry' -count=1`.

Expected: PASS.

- [ ] **Step 5: Commit**

Stage the five listed files and commit with `fix: cascade response channel affinity`.

### Task 2: Bounded continuation of accepted streams

**Files:**
- Modify: `common/init.go`
- Modify: `constant/env.go`
- Modify: `relay/common/relay_info.go`
- Create: `relay/common/stream_recovery.go`
- Create: `relay/common/stream_recovery_test.go`
- Modify: `relay/channel/api_request.go`
- Modify: `relay/helper/stream_scanner.go`
- Modify: `relay/helper/stream_scanner_test.go`
- Modify: `relay/helper/common.go`
- Modify: `relay/channel/openai/relay_responses.go`
- Modify: `relay/channel/openai/relay_responses_test.go`
- Modify: `relay/channel/claude/relay-claude.go`
- Modify: `relay/channel/claude/relay_claude_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

Cover accepted client cancellation followed by terminal usage, ineligible pre-accept cancellation, timeout, size cap, global/per-channel admission cap, ping/write races, exact usage at the byte boundary, and both Responses and Claude terminal usage recovery.

- [ ] **Step 2: Run tests and capture RED**

Run `go test ./relay/common ./relay/helper ./relay/channel/openai ./relay/channel/claude -run 'StreamRecovery|Drain|ClientGone.*Usage|TerminalUsage' -count=1`.

Expected: FAIL because the current stream scanner cancels and closes the upstream body as soon as the downstream context ends.

- [ ] **Step 3: Implement bounded recovery**

Use `context.WithoutCancel` only for accepted responses, add explicit concurrency/time/byte limits, keep downstream writes disabled after detach, parse the original stream to terminal usage, and ensure cleanup releases body, timer, watcher, and limiter exactly once. Do not change normal live-stream timeout behavior.

- [ ] **Step 4: Run focused and package tests**

Run `go test ./relay/common ./relay/helper ./relay/channel/openai ./relay/channel/claude -run 'Stream|Responses|Claude' -count=1` and then the same packages without `-run`.

Expected: PASS.

- [ ] **Step 5: Commit**

Stage the listed stream files and commit with `fix: recover usage from interrupted streams`.

### Task 3: Explicit estimated usage settlement

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/openai/relay_responses.go`
- Modify: `relay/channel/openai/relay_responses_test.go`
- Modify: `relay/channel/claude/relay-claude.go`
- Modify: `relay/channel/claude/relay_claude_test.go`
- Modify: `service/text_quota.go`
- Modify: `service/text_quota_test.go`
- Modify: `service/log_info_generate.go`

- [ ] **Step 1: Write failing billing tests**

Cover timeout/size/capacity/upstream-error recovery outcomes with estimated prompt tokens, observed-output completion tokens, zero observed output, authoritative recovered usage, frozen-reservation floor, tiered-expression isolation, trusted-wallet settlement failure, and log metadata without secrets.

- [ ] **Step 2: Run tests and capture RED**

Run `go test ./relay/channel/openai ./relay/channel/claude ./service -run 'EstimatedUsage|RecoveredUsage|Reservation|UsageUnconfirmed' -count=1`.

Expected: FAIL because missing terminal usage currently logs zero prompt/completion tokens while retaining quota.

- [ ] **Step 3: Implement usage-source-aware settlement**

Build estimated usage from `RelayInfo.GetEstimatePromptTokens()` plus observed output, set `usage_source=estimated` and the bounded recovery result, retain `usage_unconfirmed=true`, keep cached subfields non-authoritative, and settle `max(estimated quota, frozen reservation)`. Preserve normal authoritative tiered settlement unchanged.

- [ ] **Step 4: Run focused, package, and vet verification**

Run `go test ./relay ./controller ./service -run 'Usage|Billing|Stream|Affinity|Retry' -count=1`, full affected package tests, `go vet ./relay/... ./controller ./service ./model/...`, and `git diff --check`.

Expected: PASS with no vet or diff-check output.

- [ ] **Step 5: Commit**

Stage the listed usage files and commit with `fix: bill interrupted streams with estimated usage`.

### Task 4: Candidate build, brand verification, and production hot switch

**Files:**
- No source edits expected.
- Preserve rollback artifacts on server.

- [ ] **Step 1: Independent code review**

Review the full production diff from `b66e99504` to branch HEAD for billing invariants, unsafe retries/refunds, cross-session affinity leakage, goroutine/resource leaks, provider protocol compatibility, secret exposure, and SQLite/MySQL/PostgreSQL compatibility. Resolve every Critical or Important finding before continuing.

- [ ] **Step 2: Run fresh full verification**

Run full relay/controller/service/model tests, vet affected packages, and `git diff --check`. Build the production image from the isolated branch with the existing production build procedure. Expected: all commands exit 0.

- [ ] **Step 3: Start private local candidate**

Bind only to a free `127.0.0.1` port. Verify health, unauthenticated API errors, database startup without destructive migrations, and that only one preview candidate is active.

- [ ] **Step 4: Verify brand and UI baseline**

Compare the candidate against the preserved production baseline for homepage, sign-in, registration, console, API keys, system settings, infinite canvas, documentation, custom brand, animations, and model configuration. Confirm the backend patch introduces no frontend source or asset diff.

- [ ] **Step 5: Verify billing and affinity behavior locally**

Use mock upstreams to prove exact usage recovery, estimated fallback with non-zero tokens, frozen-reservation floor, no retry/refund after ambiguous submission, response-chain lookup fallback, conversation isolation, and no upstream details in public errors.

- [ ] **Step 6: Build and start production candidate without traffic**

Create a new immutable image tag and container on the production server using the existing environment and database, bind only to a private port/network, wait for healthy status, and run health/API/static-fingerprint probes. Do not stop or modify the rollback container.

- [ ] **Step 7: Hot switch Caddy and monitor**

Back up the active Caddyfile, validate the candidate upstream, atomically reload Caddy to the candidate, and verify the active Caddy admin config. Run all three public domains, UI fingerprint, API error sanitization, container health/restarts, and billing safety probes continuously. Immediately reload the preserved Caddyfile to `newapi-production-20260812-b66e99504` on any anomaly.

- [ ] **Step 8: Push the archival branch**

Push `codex/estimated-usage-affinity-recovery-20260813` to the configured user fork. Keep the old container, old image, active-config backup, and candidate image until the user completes production testing.
