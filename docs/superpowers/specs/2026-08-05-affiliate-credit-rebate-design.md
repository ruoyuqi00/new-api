# Affiliate Credit Rebate Design

## Summary

Add an optional referral rebate that awards an inviter a configurable percentage of each eligible balance credit received by an invited user. The existing registration-time fixed invitation rewards remain independent.

The rebate is based on the quota actually credited to the invitee, not the payment provider's charged amount. Rewards accumulate in the inviter's existing affiliate balance (`aff_quota`) and can be transferred to the main balance through the existing flow.

## Goals

- Award the inviter on every eligible credit received by an invited user.
- Let administrators enable or disable percentage rebates globally.
- Support a percentage from `0.01%` through `100%`, with at most two decimal places.
- Keep the fixed registration rewards controlled by `QuotaForInviter` and `QuotaForInvitee` unchanged and independent.
- Make reward issuance atomic with the originating balance credit.
- Prevent duplicate rewards from repeated payment callbacks or redemption attempts.
- Preserve support for SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+.

## Non-Goals

- Multi-level referrals or rewards for an inviter's inviter.
- Cash withdrawal or direct payment-provider payouts.
- Per-user, per-group, or per-invitation-code percentages.
- Retroactive rewards for credits completed before this feature is enabled.
- Rewards for subscriptions or non-wallet quota.

## Eligibility

An affiliate rebate is issued only for these explicit balance-credit sources:

1. A successful online wallet top-up, including supported payment providers and manual completion of an existing pending top-up order.
2. A successful redemption-code redemption.
3. An administrator's explicit `add` balance operation.

The following sources never issue an affiliate rebate:

- Refunds and task-failure quota returns.
- Check-in awards.
- New-user quota and registration-time invitation rewards.
- Transfers from `aff_quota` to the main balance.
- Subscription quota.
- Quota synchronization and billing reconciliation.
- Administrator `subtract` and `override` operations.

The invitee must have a non-zero `inviter_id` that points to an existing, non-deleted user. Existing invitation bindings become eligible for future credits as soon as the feature is enabled. An inviter cannot be rewarded for their own credit.

## Configuration

Add two global options:

- `AffiliateCreditRebateEnabled`: boolean, default `false`.
- `AffiliateCreditRebateBasisPoints`: integer, default `0`, valid range `1` through `10000` when enabled.

The administrator-facing field is expressed as a percentage with at most two decimal places. The API converts the percentage to basis points, where `5.25%` is stored as `525`. Integer basis points avoid floating-point drift across databases and payment providers.

The reward formula is:

```text
reward_quota = floor(credited_quota * basis_points / 10000)
```

If the result is less than one quota unit, no reward record is created and no reward is issued.

Changing the percentage affects only later credits. Every reward record stores the applied basis points so historical calculations remain auditable.

## Data Model

Add an `AffiliateReward` model and table with these fields:

- `id`: database-generated primary key.
- `source_type`: stable source identifier (`topup`, `redemption`, or `admin_add`).
- `source_id`: top-up trade number, redemption record ID, or a server-generated administrator event ID.
- `invitee_id`: user receiving the original balance credit.
- `inviter_id`: user receiving the affiliate reward.
- `credited_quota`: actual quota added to the invitee.
- `ratio_basis_points`: percentage snapshot used for this reward.
- `reward_quota`: quota added to affiliate balances.
- `created_time`: Unix timestamp.

Create a unique composite index on `source_type` and `source_id`. This is the durable idempotency boundary for reward issuance. Add indexes on `inviter_id`, `invitee_id`, and `created_time` for administrative inspection and future reporting.

Use GORM migration patterns that work on SQLite, MySQL, and PostgreSQL. The table uses scalar columns only and does not require database-specific JSON or generated columns.

## Transaction Flow

Introduce a reusable model-layer operation representing the stable business concept of crediting an eligible balance source and applying its affiliate reward. It accepts an existing GORM transaction, invitee ID, credited quota, source type, and source ID.

Within the caller's transaction it:

1. Validates the credited quota and source identity.
2. Credits the invitee's main balance.
3. Stops if the feature is disabled or the invitee has no valid inviter.
4. Calculates the reward using the current basis-point setting.
5. Inserts the unique reward record.
6. Atomically increments the inviter's `aff_quota` and `aff_history_quota`.

Every eligible top-up completion path, redemption, and administrator `add` operation must use this operation. Ineligible quota changes continue to use their existing functions and cannot accidentally trigger rebates.

For top-ups and redemption codes, the existing order or redemption state change remains in the same transaction. A repeated callback finds an already completed source or conflicts with the unique reward source and cannot issue a second reward. The administrator `add` endpoint generates one source ID per accepted operation and performs the invitee credit, reward record, and inviter update in a single transaction.

If any database operation fails, the entire source transaction rolls back. Payment-provider retries can then complete normally without leaving either party partially credited.

## Invitation Binding And Counts

Invitation binding remains registration-only. The registration flow resolves the affiliate code and persists `inviter_id`; users cannot change it later.

Fix invitation counting so `aff_count` increments whenever a valid invitation binding is created, regardless of whether `QuotaForInviter` is zero. Fixed registration rewards are applied only when their existing amount settings are positive.

Run a one-time, cross-database-safe reconciliation that recalculates each user's `aff_count` from non-deleted users grouped by `inviter_id`. The reconciliation changes counts only; it does not create historical rebate rewards.

## Logs And Auditability

After a transaction commits, write an inviter-facing system log containing only:

- Source type.
- Invitee user ID.
- Credited quota.
- Reward quota.

Do not log payment credentials, provider payloads, request bodies, tokens, or invitation codes.

The reward table is the accounting record and idempotency source. Existing management audit logging remains in place for administrator balance additions.

## Administration UI

Extend the quota settings section with:

- A switch for `Affiliate Credit Rebate`.
- A percentage input enabled when the switch is on, with `0.01` step and validation from `0.01` to `100`.

The backend independently validates both settings. Enabling the feature requires payment compliance confirmation, matching the existing invitation-reward restrictions.

All new user-facing text must use the existing i18n system and be present in `en`, `zh`, `fr`, `ja`, `ru`, and `vi` locale files.

## User UI

When the feature is enabled, the existing affiliate rewards card displays the current percentage and explains that eligible credits from invited users earn that percentage as affiliate rewards. It continues to display:

- Pending affiliate balance.
- Total affiliate earnings.
- Invitation count.
- Referral link.
- Transfer-to-balance action.

When the feature is disabled, the percentage-rebate description is hidden. The referral link, fixed registration rewards, existing affiliate balances, and transfer action remain available.

Expose the enabled state and percentage through the authenticated wallet/top-up information response. Do not expose internal reward records or other users' financial data.

## Error Handling

- Invalid percentages are rejected without changing either setting.
- A missing or deleted inviter skips the reward but does not block the invitee's credit.
- A self-referential `inviter_id` skips the reward and records a server-side diagnostic without user data beyond IDs.
- Duplicate source records do not award again.
- A database failure while creating a legitimate first reward rolls back the originating credit transaction.
- Post-commit user-facing log failures are reported to system diagnostics but do not repeat or roll back a completed financial transaction.

## Test Plan

Backend tests must cover these behavioral contracts:

- Disabled feature credits only the invitee.
- Enabled feature awards the configured percentage to `aff_quota` and `aff_history_quota`.
- `0.01%`, fractional percentages, `100%`, and floor rounding.
- Rewards below one quota unit are skipped.
- Existing invitation bindings participate in future eligible credits.
- Online top-up, redemption, and administrator `add` sources issue rewards.
- Refunds, check-ins, registration grants, affiliate transfers, subscriptions, synchronization, `subtract`, and `override` do not issue rewards.
- Repeated top-up callbacks and redemption attempts do not duplicate rewards.
- Transaction failure leaves the order or redemption, invitee balance, inviter balances, and reward table unchanged.
- Fixed registration rewards remain independent.
- Invitation counts increment with zero fixed inviter reward and historical reconciliation is correct.
- Model migration and unique-index behavior work under project SQLite fixtures, with implementation restricted to cross-database GORM patterns.

Frontend tests must cover:

- Settings switch and percentage validation.
- Percentage conversion to and from basis points.
- Disabled-state field behavior.
- Affiliate card percentage visibility when enabled and absence when disabled.
- Existing transfer and referral-link behavior remains intact.

## Deployment

1. Deploy the schema and code with the feature disabled by default.
2. Verify migration completion and invitation-count reconciliation.
3. Verify an administrator test credit and a redemption test in a non-production fixture.
4. Enable the feature and set the desired percentage through system settings.
5. Observe reward records, inviter balances, and duplicate-callback handling before broader promotion.

No historical credits are replayed. Disabling the feature is an immediate operational rollback for future rewards and does not remove already earned affiliate balances.
