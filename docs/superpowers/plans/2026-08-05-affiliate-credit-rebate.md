# Affiliate Credit Rebate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an administrator-controlled, idempotent referral rebate that awards a percentage of eligible balance credits to the inviter's affiliate balance.

**Architecture:** A model-layer credit operation will update the invitee, create a unique reward ledger row, and update the inviter within one database transaction. Only online top-ups, redemption codes, and administrator `add` operations call it; all other quota changes remain outside the rebate path. Global settings use integer basis points, while the frontend displays percentages.

**Tech Stack:** Go 1.22+, GORM v2, SQLite/MySQL/PostgreSQL, Gin, React 19, TypeScript, React Hook Form, Zod, Bun test, i18next.

---

### Task 1: Core Reward Ledger And Calculation

**Files:**
- Create: `model/affiliate_reward.go`
- Create: `model/affiliate_reward_test.go`
- Modify: `common/constants.go`
- Modify: `model/main.go`
- Modify: `model/option.go`

- [ ] **Step 1: Write failing reward behavior tests**

Create table tests that explicitly initialize the global settings and users:

```go
func TestCreditUserQuotaWithAffiliateRewardTxAwardsConfiguredPercentage(t *testing.T) {
    invitee, inviter := createAffiliateUsers(t)
    common.AffiliateCreditRebateEnabled = true
    common.AffiliateCreditRebateBasisPoints = 525

    var reward *AffiliateReward
    require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
        var err error
        reward, err = CreditUserQuotaWithAffiliateRewardTx(
            tx, invitee.Id, 10_000, AffiliateRewardSourceTopUp, "order-1",
        )
        return err
    }))

    require.NotNil(t, reward)
    assert.Equal(t, 525, reward.RewardQuota)
    assertAffiliateBalances(t, inviter.Id, 525, 525)
    assertUserQuota(t, invitee.Id, 10_000)
}
```

Add deterministic cases for disabled configuration, no inviter, self-reference, floor rounding, less-than-one-unit rewards, missing inviter, duplicate source rollback, and `10000` basis points.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./model -run 'TestCreditUserQuotaWithAffiliateRewardTx' -count=1`

Expected: build failure because `AffiliateReward` and `CreditUserQuotaWithAffiliateRewardTx` do not exist.

- [ ] **Step 3: Implement settings, model, and transactional credit operation**

Add disabled defaults:

```go
var AffiliateCreditRebateEnabled = false
var AffiliateCreditRebateBasisPoints = 0
```

Define stable source constants, a scalar GORM model with a unique composite source index, and the transaction function:

```go
type AffiliateReward struct {
    Id               int    `json:"id"`
    SourceType       string `json:"source_type" gorm:"type:varchar(32);uniqueIndex:idx_affiliate_reward_source"`
    SourceId         string `json:"source_id" gorm:"type:varchar(255);uniqueIndex:idx_affiliate_reward_source"`
    InviteeId        int    `json:"invitee_id" gorm:"index"`
    InviterId        int    `json:"inviter_id" gorm:"index"`
    CreditedQuota    int    `json:"credited_quota"`
    RatioBasisPoints int    `json:"ratio_basis_points"`
    RewardQuota      int    `json:"reward_quota"`
    CreatedTime      int64  `json:"created_time" gorm:"index"`
}
```

`CreditUserQuotaWithAffiliateRewardTx` must validate source identity, load the invitee, increment invitee quota, skip ineligible relationships, calculate with `decimal`, insert the ledger row, and atomically increment both inviter affiliate columns. Record creation and inviter updates stay inside the caller transaction.

Register `AffiliateReward` in both normal and fast migration lists. Add both options to `InitOptionMap` and `updateOptionMap` using `strconv.FormatBool`, `strconv.Itoa`, `strconv.ParseBool`, and `strconv.Atoi`.

- [ ] **Step 4: Run focused and package tests**

Run: `go test ./model -run 'TestCreditUserQuotaWithAffiliateRewardTx' -count=1`

Expected: PASS.

Run: `go test ./model -count=1`

Expected: PASS.

- [ ] **Step 5: Commit core ledger**

```bash
git add common/constants.go model/affiliate_reward.go model/affiliate_reward_test.go model/main.go model/option.go
git commit -m "feat: add transactional affiliate reward ledger"
```

### Task 2: Eligible Credit Sources And Idempotency

**Files:**
- Modify: `model/topup.go`
- Modify: `model/redemption.go`
- Modify: `controller/topup.go`
- Modify: `controller/user.go`
- Modify: `model/redemption_test.go`
- Create: `model/affiliate_reward_sources_test.go`

- [ ] **Step 1: Write failing source integration tests**

Cover an enabled referral relationship. The redemption regression test must exercise the public model operation twice and assert the durable ledger:

```go
func TestRedeemAwardsAffiliateRewardExactlyOnce(t *testing.T) {
    inviter, invitee := createAffiliateUsers(t)
    common.AffiliateCreditRebateEnabled = true
    common.AffiliateCreditRebateBasisPoints = 500
    redemption := &Redemption{
        Name: "affiliate-redeem",
        Key: "affiliate-redeem-key",
        Status: common.RedemptionCodeStatusEnabled,
        Quota: 10_000,
        CreatedTime: common.GetTimestamp(),
    }
    require.NoError(t, DB.Create(redemption).Error)

    credited, err := Redeem(redemption.Key, invitee.Id)
    require.NoError(t, err)
    assert.Equal(t, 10_000, credited)
    assertUserQuota(t, invitee.Id, 10_000)
    assertAffiliateBalances(t, inviter.Id, 500, 500)

    _, err = Redeem(redemption.Key, invitee.Id)
    require.Error(t, err)
    assertUserQuota(t, invitee.Id, 10_000)
    assertAffiliateBalances(t, inviter.Id, 500, 500)

    var rewardCount int64
    require.NoError(t, DB.Model(&AffiliateReward{}).
        Where("source_type = ? AND source_id = ?", AffiliateRewardSourceRedemption, strconv.Itoa(redemption.Id)).
        Count(&rewardCount).Error)
    assert.EqualValues(t, 1, rewardCount)
}
```

Add equivalent focused tests named `TestRechargeWaffoPancakeAwardsAffiliateRewardExactlyOnce`, `TestRechargeEpayAwardsAffiliateRewardAtomically`, and `TestAdminAddQuotaAwardsAffiliateReward`. Each test must assert exact invitee quota, exact inviter balances, and one reward row. The Epay test also asserts that an injected duplicate source error leaves the order pending and both balances unchanged.

Assert invitee quota, inviter `aff_quota`, inviter `aff_history_quota`, top-up/redemption state, and exactly one ledger row. Add a forced duplicate-source test that proves the entire second credit transaction rolls back.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./model -run 'Test(RedeemAwardsAffiliate|Recharge.*Affiliate|AdminAddQuotaAwardsAffiliate)' -count=1`

Expected: assertions fail because source entry points do not call the reward operation.

- [ ] **Step 3: Integrate all online top-up completion paths**

Replace direct quota increments in `Recharge`, `ManualCompleteTopUp`, `RechargeCreem`, `RechargeWaffo`, and `RechargeWaffoPancake` with:

```go
reward, err = CreditUserQuotaWithAffiliateRewardTx(
    tx, topUp.UserId, quotaToAdd, AffiliateRewardSourceTopUp, topUp.TradeNo,
)
```

Move Epay completion out of the controller's separate order-save/quota-update sequence into `model.RechargeEpay`, using the same row lock, provider check, pending-status check, transaction, and reward operation. The controller verifies the webhook and calls this model function.

Return or retain the generated reward only for post-commit system logging. A repeated successful callback must return without issuing or logging another reward.

- [ ] **Step 4: Integrate redemption and administrator add**

In `Redeem`, replace the direct quota update with the transactional reward operation using `strconv.Itoa(redemption.Id)` as `source_id`.

Add a model wrapper for administrator credits:

```go
func AddUserQuotaWithAffiliateReward(userId int, quota int, eventId string) (*AffiliateReward, error) {
    var reward *AffiliateReward
    err := DB.Transaction(func(tx *gorm.DB) error {
        var err error
        reward, err = CreditUserQuotaWithAffiliateRewardTx(
            tx, userId, quota, AffiliateRewardSourceAdminAdd, eventId,
        )
        return err
    })
    return reward, err
}
```

Use a server-generated UUID in the controller's `add_quota/add` branch. Leave `subtract` and `override` unchanged.

- [ ] **Step 5: Run focused and regression tests**

Run: `go test ./model -run 'Test(Redeem|Recharge|AdminAdd|CreditUserQuotaWithAffiliate)' -count=1`

Expected: PASS.

Run: `go test ./controller -run 'Test.*TopUp|Test.*Quota' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit source integrations**

```bash
git add model/topup.go model/redemption.go model/redemption_test.go model/affiliate_reward_sources_test.go controller/topup.go controller/user.go
git commit -m "feat: reward eligible affiliate balance credits"
```

### Task 3: Invitation Counting And Historical Reconciliation

**Files:**
- Modify: `model/user.go`
- Create: `model/affiliate_count_test.go`
- Modify: `model/main.go`

- [ ] **Step 1: Write failing count tests**

Add tests proving registration increments `aff_count` when `QuotaForInviter == 0`, fixed rewards remain independent, and reconciliation replaces stale counts from actual non-deleted `inviter_id` relationships.

```go
func TestInviteBindingIncrementsCountWithoutFixedReward(t *testing.T) {
    common.QuotaForInviter = 0
    // Create inviter and register invitee with inviter ID.
    // Assert count is one and affiliate quota remains zero.
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./model -run 'Test(InviteBinding|ReconcileAffiliateCounts)' -count=1`

Expected: count assertion fails because current registration calls `inviteUser` only when fixed reward is positive.

- [ ] **Step 3: Decouple count from fixed reward**

Update registration finalization so every valid inviter binding increments `aff_count` exactly once. Add fixed `aff_quota` and `aff_history_quota` only when `QuotaForInviter > 0`. Keep the invitee fixed reward behavior unchanged.

Add an idempotent reconciliation function that groups active users by `inviter_id`, resets stored counts, writes grouped counts in one transaction, and records a private migration marker so the scan runs once. Call it after `User` and `Option` migrations complete.

- [ ] **Step 4: Verify count tests and model package**

Run: `go test ./model -run 'Test(InviteBinding|ReconcileAffiliateCounts)' -count=1`

Expected: PASS.

Run: `go test ./model -count=1`

Expected: PASS.

- [ ] **Step 5: Commit invitation count correction**

```bash
git add model/user.go model/affiliate_count_test.go model/main.go
git commit -m "fix: reconcile affiliate invitation counts"
```

### Task 4: Option Validation And Wallet API

**Files:**
- Modify: `controller/option.go`
- Modify: `controller/topup.go`
- Create: `controller/affiliate_rebate_test.go`

- [ ] **Step 1: Write failing API validation tests**

Test that the generic option endpoint rejects basis points below zero and above `10000`, rejects enabling without payment compliance, and accepts valid values. Test `GetTopUpInfo` returns only the enabled flag and basis points needed by the authenticated wallet UI.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./controller -run 'TestAffiliateCreditRebate' -count=1`

Expected: FAIL because the options have no validation and top-up info omits them.

- [ ] **Step 3: Implement backend validation and response fields**

Validate `AffiliateCreditRebateEnabled` as a boolean and `AffiliateCreditRebateBasisPoints` as an integer in `0..10000`. Enabling requires payment compliance and a stored ratio in `1..10000`. A positive ratio also requires compliance.

Add to the wallet response:

```go
"affiliate_credit_rebate_enabled":      common.AffiliateCreditRebateEnabled,
"affiliate_credit_rebate_basis_points": common.AffiliateCreditRebateBasisPoints,
```

- [ ] **Step 4: Run controller tests**

Run: `go test ./controller -run 'TestAffiliateCreditRebate' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit API settings**

```bash
git add controller/option.go controller/topup.go controller/affiliate_rebate_test.go
git commit -m "feat: expose affiliate rebate settings"
```

### Task 5: Administration And Wallet UI

**Files:**
- Create: `web/default/src/features/wallet/lib/affiliate-rebate.ts`
- Create: `web/default/tests/affiliate-rebate.test.ts`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/billing/index.tsx`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/general/quota-settings-section.tsx`
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/index.tsx`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Create: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Write failing percentage utility tests**

```ts
test('formats basis points without floating point noise', () => {
  expect(formatAffiliateRebatePercent(1)).toBe('0.01%')
  expect(formatAffiliateRebatePercent(525)).toBe('5.25%')
  expect(formatAffiliateRebatePercent(10000)).toBe('100%')
})
```

Also test percentage-to-basis-point conversion rejects more than two decimals and values outside `0.01..100`.

- [ ] **Step 2: Run tests and verify RED**

Run from `web/default`: `bun test tests/affiliate-rebate.test.ts`

Expected: module-not-found failure for `affiliate-rebate.ts`.

- [ ] **Step 3: Implement utility, settings form, and wallet display**

Add typed settings and defaults. The form stores basis points but renders `basisPoints / 100` in a numeric input with `step="0.01"`. Disable the percentage field while the switch is off, and keep backend validation authoritative.

Extend `TopupInfo`, pass enabled state and basis points into `AffiliateRewardsCard`, and render a translated sentence containing `formatAffiliateRebatePercent(...)` only when enabled.

- [ ] **Step 4: Add all locale values through the required script**

Create `scripts/add-missing-keys.mjs` with a `newKeys` object containing exact translations for all six locales. Include keys for the switch label, percentage label, setting descriptions, and wallet rebate sentence. Run:

```bash
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Do not edit locale JSON files manually.

- [ ] **Step 5: Run frontend tests and static checks**

Run from `web/default`:

```bash
bun test tests/affiliate-rebate.test.ts
bun run typecheck
bun run lint
bun run format:check
bun run i18n:sync
bun run build
```

Expected: all commands exit zero; i18n report has no new missing keys.

- [ ] **Step 6: Commit frontend integration**

```bash
git add web/default/src/features/system-settings web/default/src/features/wallet web/default/src/i18n/locales web/default/scripts/add-missing-keys.mjs web/default/tests/affiliate-rebate.test.ts
git commit -m "feat: configure and display affiliate rebates"
```

### Task 6: Full Verification And Production Readiness

**Files:**
- Modify if required by verified behavior: `docs/superpowers/specs/2026-08-05-affiliate-credit-rebate-design.md`
- Modify: `docs/superpowers/plans/2026-08-05-affiliate-credit-rebate.md`

- [ ] **Step 1: Run full backend verification**

```bash
go test ./...
go vet ./...
```

Expected: exit zero.

- [ ] **Step 2: Run full frontend verification**

From `web/default`:

```bash
bun test
bun run typecheck
bun run lint
bun run format:check
bun run copyright:check
bun run build
```

Expected: exit zero.

- [ ] **Step 3: Inspect the final diff and migration safety**

Run:

```bash
git diff --check
git status --short
git log --oneline --decorate -8
```

Verify only intended source, tests, locale files, and design/plan documents changed. Confirm no environment files, credentials, generated build output, or request data are present.

- [ ] **Step 4: Build and deploy with the feature disabled**

Build a production image tagged with the final commit. Deploy using the existing zero-downtime candidate-container process, run database migrations, verify health and restart count, then switch Caddy only after candidate health succeeds.

- [ ] **Step 5: Production acceptance**

Verify the five configured domains return status successfully and unauthenticated model access remains rejected. Confirm the new options default to disabled, the reward table exists, existing invitation counts reconcile, and no reward is issued until the administrator explicitly enables the feature.

- [ ] **Step 6: Review parent project updates**

Fetch the configured upstream repository, compare the production branch with the current parent default branch, and classify newer commits into security fixes, correctness fixes, provider compatibility changes, frontend changes, and high-risk feature work. Merge only fixes that are demonstrably applicable and pass this plan's full verification; report deferred items with reasons.

- [ ] **Step 7: Push the verified branch**

```bash
git push -u origin codex/affiliate-rebate-20260805
```

Expected: remote branch updated successfully.
