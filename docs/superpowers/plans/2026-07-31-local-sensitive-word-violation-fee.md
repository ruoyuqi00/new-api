# Local Sensitive-Word Violation Fee Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Charge exactly one configured fixed violation fee when YuAPI rejects a request locally for configured sensitive words, without selecting a channel or contacting an upstream.

**Architecture:** Preserve the existing sensitive-word scan but defer the response until token estimation and `ModelPriceHelper` have resolved the effective group ratio. A dedicated local-violation billing entry point uses the existing billing-session preference selection, records a sanitized consume log after a successful charge, and returns the original non-retryable `sensitive_words_detected` error regardless of charge outcome. Existing upstream Grok CSAM charging remains intact.

**Tech Stack:** Go 1.22+, Gin, GORM, Testify, existing YuAPI billing and relay services

---

### Task 1: Specify Violation Classification And Fee Metadata

**Files:**
- Create: `service/violation_fee_test.go`
- Modify: `service/violation_fee.go`

- [ ] **Step 1: Write failing table tests for chargeable violation reasons**

Add tests using `require`/`assert` which prove that `sensitive_words_detected` maps to a local reason, normalized Grok CSAM maps to the existing Grok reason, unrelated errors are not chargeable, and the quota calculation applies `ViolationDeductionAmount * QuotaPerUnit * groupRatio` with decimal rounding.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./service -run 'TestViolationFeeReason|TestCalcViolationFeeQuota' -count=1`

Expected: FAIL because local sensitive-word violations are not recognized yet.

- [ ] **Step 3: Implement minimal reason classification**

Add a small internal violation-reason type and classifier that recognizes both `types.ErrorCodeSensitiveWordsDetected` and the existing Grok CSAM path. Keep `NormalizeViolationFeeError` behavior unchanged for upstream errors.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `go test ./service -run 'TestViolationFeeReason|TestCalcViolationFeeQuota' -count=1`

Expected: PASS.

### Task 2: Charge A Local Violation Through The Existing Billing Preference

**Files:**
- Modify: `service/violation_fee_test.go`
- Modify: `service/violation_fee.go`

- [ ] **Step 1: Write failing behavior tests for the local charge entry point**

Use a temporary SQLite fixture and real model records to prove that a successful local fee charge reduces the selected funding source/token quota by the fixed fee, increments user used quota/request count once, writes one sanitized consume log, and does not increment channel used quota. Add an insufficient-quota case proving no successful consume record is written.

- [ ] **Step 2: Run the local-charge tests and verify RED**

Run: `go test ./service -run 'TestChargeLocalViolationFee' -count=1`

Expected: FAIL because the local charge entry point does not exist.

- [ ] **Step 3: Implement local charging and shared sanitized logging**

Add `ChargeLocalViolationFee(c, relayInfo, apiErr) bool`. It must validate the local reason, read the existing Grok violation settings, calculate the fee from the effective group ratio, call `PreConsumeBilling` to honor wallet/subscription preference, update user accounting only after success, avoid channel accounting, and record a consume log without matched words or request content. Refactor the existing upstream charge path only enough to share sanitized log construction while preserving its behavior.

- [ ] **Step 4: Run the local-charge and existing violation tests and verify GREEN**

Run: `go test ./service -run 'TestChargeLocalViolationFee|TestViolationFee|TestCalcViolationFeeQuota' -count=1`

Expected: PASS.

### Task 3: Integrate Local Charging Before Channel Selection

**Files:**
- Modify: `controller/relay.go`
- Test: `controller/relay_test.go` or the nearest existing relay test file

- [ ] **Step 1: Write a failing relay regression test**

Add a test around the relay handler proving that a configured sensitive-word match returns HTTP 400 with code `sensitive_words_detected`, resolves pricing before charging, and exits before channel selection/upstream invocation. The test must also prove a non-match continues through the existing path.

- [ ] **Step 2: Run the focused relay test and verify RED**

Run: `go test ./controller -run 'TestRelaySensitiveWordViolation' -count=1`

Expected: FAIL because the current handler returns before pricing and creates a 500-style error from a nil underlying error.

- [ ] **Step 3: Move only the sensitive-match exit point**

Retain the scan result as a boolean, run token estimation and `ModelPriceHelper`, then before normal pre-consumption build a real HTTP 400 `sensitive_words_detected` error with `ErrOptionWithSkipRetry`, invoke `ChargeLocalViolationFee`, and return. Do not select a channel or install the normal failure-refund defer for this path.

- [ ] **Step 4: Run controller and service tests**

Run: `go test ./controller ./service -count=1`

Expected: PASS.

### Task 4: Verify, Commit, Build, And Deploy Without Dropping Active Requests

**Files:**
- Verify: `service/violation_fee.go`
- Verify: `controller/relay.go`
- Verify: `web/default/src/components/header/ApiKeyDropdown.tsx`

- [ ] **Step 1: Format and run repository-focused verification**

Run: `gofmt -w service/violation_fee.go service/violation_fee_test.go controller/relay.go`

Run: `go test ./service ./controller ./relay/... -count=1`

Run: `go test ./... -count=1`

Expected: all tests PASS.

- [ ] **Step 2: Confirm scope and commit**

Run: `git diff --check && git status --short && git diff --stat`

Commit only the plan, tests, local violation fee logic, relay integration, and any directly required fixture changes. Do not include account-pool, retry, weight, Caddy, or unrelated changes.

- [ ] **Step 3: Build a versioned production image on the new server**

Transfer the committed source without secrets, build `yuapi:production-20260731-<commit>`, and start a healthy `newapi-candidate` attached to the existing production network. Do not restart MySQL, Redis, Caddy, or unrelated containers.

- [ ] **Step 4: Perform the established blue/green handoff**

Reload the current host `/opt/edge/Caddyfile` through stdin with only the YuAPI upstream changed from `newapi:3000` to `newapi-candidate:3000`; wait for the old container's active connections to drain; recreate the official `newapi` service with the new image; reload the current host Caddyfile back to `newapi:3000`; wait for candidate connections to drain; then remove only the candidate.

- [ ] **Step 5: Verify production and the URL notice**

Confirm the official container is healthy with zero restarts. For `yuaiapi.com`, `www.yuaiapi.com`, `api.yuaiapi.com`, `global.yuaiapi.com`, and `vip.yuaiapi.com`, verify `/api/status` returns 200 and unauthenticated `/v1/models` returns 401. Confirm the API-key page build still contains the supported `api` and `vip` base URLs and the `global` warning. Do not send a real sensitive prompt to production or expose the configured sensitive-word list.

- [ ] **Step 6: Retain rollback assets**

Keep the previous production image and Compose backup through the observation period. Do not delete production data, credentials, certificates, or unrelated services.
