# Unconfirmed GPT Text Usage Settlement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Settle GPT text requests without terminal usage from estimated prompt and observed completion tokens instead of retaining a maximum-output reservation.

**Architecture:** Preserve existing bounded same-stream recovery and normal authoritative settlement. Add an estimated GPT-text settlement branch that recomputes frozen tiered pricing from estimated tokens, refunds the reservation difference, and forbids positive quota when both token estimates are zero.

**Tech Stack:** Go 1.22+, Gin, existing BillingSession, billingexpr, testify.

---

### Task 1: Reproduce reservation overcharge

**Files:**
- Modify: `service/text_quota_test.go`

- [ ] **Step 1: Write failing ratio and zero-token tests**

Add tests for `/v1/responses` with a 1,250-unit reservation:

```go
usage := &dto.Usage{PromptTokens: 400, CompletionTokens: 20, UsageSource: "estimated"}
```

Assert settlement uses the calculated ratio quota below 1,250 and refunds the
difference. Add a zero-token case asserting final quota is zero and no positive
consume record is created.

- [ ] **Step 2: Verify RED**

```powershell
go test ./service -run 'EstimatedGPTText|ZeroEstimatedGPTText' -count=1
```

Expected: the current floor settles 1,250 and the zero-token path can retain a
positive reservation.

### Task 2: Estimated tiered settlement

**Files:**
- Modify: `service/tiered_settle.go`
- Modify: `service/tiered_settle_test.go`
- Modify: `service/text_quota.go`

- [ ] **Step 1: Write the failing tiered regression**

Freeze an expression and reservation that include 8,192 estimated completion
tokens, then supply a smaller observed completion count. Assert the estimated
settlement evaluates the frozen expression with the smaller token vector and
does not fall back to `EstimatedQuotaAfterGroup`.

- [ ] **Step 2: Verify RED**

```powershell
go test ./service -run 'TieredEstimated|EstimatedGPTText' -count=1
```

- [ ] **Step 3: Implement estimated tiered evaluation**

Add a service-level tiered evaluator that uses the frozen expression, hash,
request snapshot, group ratio, and explicit token parameters. Unlike normal
`TryTieredSettle`, an evaluation error must be returned rather than replaced by
the full reservation.

In `postTextConsumeQuota`, use tiered evaluation for authoritative usage and
for unconfirmed GPT text usage. For the estimated branch, cached/image/audio
categories remain zero and the result is marked estimated.

- [ ] **Step 4: Remove the GPT reservation floor**

Retain the existing floor for out-of-scope relay formats. For GPT text paths,
settle the calculated estimated quota directly. If prompt and completion tokens
are both zero, force token-based quota to zero.

### Task 3: Recover a missing request-side estimate

**Files:**
- Modify: `service/text_quota.go`
- Modify: `service/text_quota_test.go`

- [ ] **Step 1: Write a failing fallback-count test**

Construct a GPT Responses request with text input, an empty stored estimate,
and no terminal usage. Assert the settlement performs billing-only token
estimation and records non-zero prompt tokens.

- [ ] **Step 2: Implement billing-only fallback estimation**

Only on unconfirmed GPT text settlement, rebuild `TokenCountMeta` from the
already parsed request and call `EstimateRequestTokenForBilling`. Do not fetch
media and do not apply this to image, audio, video, Claude, or async tasks.

- [ ] **Step 3: Verify focused billing and relay behavior**

```powershell
go test ./service ./relay ./controller -run 'Estimated|Ambiguous|Accepted|StreamRecovery|Billing|Retry|Violation' -count=1
```

Expected: all pass; accepted/ambiguous requests remain non-retryable and
pre-write failures still refund.

### Task 4: Full verification and commit

- [ ] Run:

```powershell
go test ./service ./relay ./controller ./relay/channel/openai ./relay/common ./relay/helper -count=1
go vet ./service ./relay ./controller ./relay/channel/openai ./relay/common ./relay/helper
git diff --check
```

- [ ] Commit:

```powershell
git add service/text_quota.go service/text_quota_test.go service/tiered_settle.go service/tiered_settle_test.go
git commit -m "fix: settle missing text usage from estimates"
```

### Task 5: Release verification

- [ ] Build one production-equivalent candidate containing both independent commits.
- [ ] Bind the candidate to a loopback-only port and confirm health, brand UI, login, GPT Responses, Chat Completions, usage logs, and billing logs.
- [ ] Verify no image/video/task/violation billing files changed.
- [ ] Hot-switch only after local verification; preserve the old healthy container and image.
- [ ] Observe estimated-usage quota/token aggregates after cutover and roll back on zero-token positive charges, billing errors, UI drift, unhealthy state, or increased relay failures.
