# SHKeeper Fixed-Package Port onto the Production UI Line

**Date:** 2026-09-26

**Status:** Approved for design documentation

**Target branch:** `codex/shkeeper-usdt-production-line`

**Target baseline:** `27023295d` (`dc6f0bd01` application source)

**Reference implementation:** `codex/shkeeper-usdt` at `79ac694b3`

## Goal

Add the reviewed SHKeeper fixed-package USDT recharge feature to the current
YuAPI production lineage without importing, replacing, or restyling the
production YuCore interface and motion system.

The target production line already contains the accepted renderer-aware UI from
`a2099648d`, including GPU detection, software-rendering fallback, entrance
motion, background renderers, and resource-lifecycle optimizations. Those files
are the visual authority. The earlier SHKeeper branch was created from an
unrelated Git history and is only a business-logic reference; it must not be
merged or cherry-picked wholesale.

## Confirmed Product Rules

- Administrators configure explicit packages such as `10 USDT -> 66 site
  balance`.
- Package USDT amounts are positive whole numbers. Package balance values must
  produce at least one internal quota unit and remain within the database's
  signed `INT` quota ceiling.
- Users select a configured package and one enabled network: BSC/BEP20,
  TRON/TRC20, or Polygon.
- The selected package and network are resolved again on the server. Client
  balance values are never trusted.
- Every order snapshots requested USDT, balance entitlement, and quota
  entitlement. Later configuration or `QuotaPerUnit` changes cannot alter it.
- Partial confirmed payment credits zero.
- Reaching or exceeding the requested USDT credits the fixed entitlement once.
- Overpayment is recorded but does not grant additional balance.
- Callbacks, scheduled reconciliation, and transaction-hash recovery use one
  provider-verified settlement path and cannot duplicate credit.
- Generic administrator manual completion rejects SHKeeper orders so it cannot
  bypass the SHKeeper ledger.
- Existing legacy empty-mode SHKeeper orders retain their frozen rate behavior.
- API keys and backend keys are write-only and never returned to browsers or
  general settings endpoints.
- Users remain inside YuAPI. YuAPI never redirects them to the SHKeeper admin
  interface.

## Production Line Authority

The release record at `27023295d` identifies:

- application source `dc6f0bd01`;
- production image `yuapi:production-responses-budget-20260924-dc6f0bd01`;
- active container `yuapi-production-responses-budget-dc6f0bd01`;
- renderer-aware UI ancestry `a2099648d`.

The new worktree is created directly from this release head. No commit from the
old SHKeeper branch is merged automatically. Each behavior is reintroduced
semantically against the target code and its current APIs.

## Immutable Visual Surface

The following production areas are immutable for this port:

- `web/default/src/features/yucore-brand/**`
- `web/default/src/styles/**`
- global theme presets and renderer assets
- entrance loader, WebGL earth, signal field, motion canvas, console
  background, and render-loop/resource scheduling
- authenticated page shell and global navigation composition
- unrelated routes, Studio, Canvas, docs, sign-in, sign-up, home, and model
  marketplace pages

The implementation gate requires the final branch diff against `27023295d` to
be empty for those areas. If a payment UI seems to require editing them, the
payment UI must instead adapt to existing production components and tokens.

## Port Boundaries

### Backend

Port the reviewed behavior into the production line's current equivalents of:

- `setting/operation_setting/shkeeper_payment.go`
- a focused quota validation module in the same package
- `model/shkeeper_topup.go` and additive GORM migration registration
- `service/shkeeper.go` and `service/shkeeper_reconcile.go`
- `controller/topup_shkeeper.go`
- `controller/topup_shkeeper_admin.go`
- public top-up information and system-task reconciliation integration
- API routes for settings, invoice creation/status, webhook, and transaction
  recovery

All JSON operations use `common.*` wrappers. Database logic uses GORM-compatible
operations for SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6.

### Frontend

Integrate only inside the target production line's current wallet and payment
settings features:

- SHKeeper request/response types and API calls
- settings form model and package editor
- wallet package selection, invoice status, and transaction recovery
- literal i18next callsites and six locale files through the project translation
  script

Do not copy the old branch's standalone black marketing panel, branded payment
composition, layout shell, or global CSS. The package picker follows the
existing production recharge dialog structure: compact header, package choices,
network warning, network choices, and the existing footer action hierarchy. The
invoice state uses the same dialog family and existing production controls.

The administrator editor follows the current production payment-settings
surface. It may add package rows and a compact inline mapping preview, but it
must not introduce a second visual system or page-level effects.

## Provider Contract

The integration targets the pinned SHKeeper v2.5.32 API:

- quote responses use `amount_crypto`;
- creation responses include the crypto amount and wallet address;
- invoice lookup returns `external_id`, `fiat`, `amount_fiat`, `balance_fiat`,
  `status`, and transaction rows;
- lookup does not return invoice-level `crypto`, crypto `amount`, or `wallet`;
- confirmed transaction rows provide `amount`, `crypto`, `addr`, `txid`, and
  `status`.

Creation and lookup therefore use separate DTOs and validators. Lookup validates
the external ID and fixed 1:1 fiat amount, then validates each confirmed
transaction against the stored network and address. A found but addressless
ambiguous creation never triggers a second create; it remains pending for
provider/operator recovery.

## Data and Settlement

New fixed-package orders store additive fields for settlement mode, requested
USDT, package balance, and package quota. Existing rate-era columns remain for
compatibility and rollback inspection.

Settlement occurs in a main-database transaction with row locking:

1. Lock the SHKeeper order, generic top-up, and target user state needed for the
   update.
2. Validate order/provider/network/address identity.
3. Normalize and deduplicate transaction IDs; conflicting amounts for one ID
   fail closed.
4. Store new confirmed transaction evidence idempotently.
5. Keep fixed entitlement at zero below the requested USDT amount.
6. Once funded, conditionally increment user quota only if the signed `INT`
   ceiling is not exceeded.
7. Persist the credited balance/quota and complete the generic top-up atomically.
8. Preserve the first completion timestamp and funded status on replay.

The scheduled reconciler records attempt time before provider work so failed
orders cannot monopolize a bounded batch.

## Failure Handling

- Invalid or stale package: reject before creating a local/provider invoice.
- Provider amount mismatch: do not expose the address; retain diagnostic state.
- Ambiguous provider create: query once, never create again automatically.
- Underpayment: keep partial/recoverable and credit zero.
- Duplicate callback/reconciliation: no additional credit.
- Polling business failure: retain the last valid invoice, show a localized
  retry action, and retry the same order without creating a new invoice.
- Unknown transaction hash: request provider rescan only; never credit directly.
- Addressless or pre-snapshot reference orders: fail closed for operator review.
- Disabling new invoices does not stop reconciliation of existing orders.

## Verification Strategy

### Automated

- Package normalization, precision, quota floor/ceiling, duplicates, and empty
  disabled settings.
- Exact v2.5.32 quote/create/lookup fixtures.
- Package tampering, amount mismatch, ambiguous create, and one-create-only
  behavior.
- Partial, exact, overpaid, late, callback replay, concurrent settlement,
  mutable conversion, capacity, and completion-time invariants.
- Generic manual completion rejection and secret redaction.
- Reconciliation failure rotation and recent-final aging.
- Frontend first-use null arrays, package validation, ordinary integer recharge,
  polling error retention/retry, and close/reopen reset.
- Full Go suite, frontend tests, typecheck, scoped lint/format, i18n sync, and
  production build.

### Visual and Motion

Build two candidates from the same production baseline and equivalent synthetic
settings:

1. untouched baseline `27023295d`;
2. production-line SHKeeper port.

At desktop and mobile sizes in light and dark themes, compare sign-in, docs,
wallet shell, navigation, home, and settings before opening any payment UI.
Verify:

- identical renderer choice and fallback behavior;
- identical Canvas/WebGL element count and dimensions;
- nonblank canvas pixel samples;
- motion continues across sampled frames when hardware rendering is available;
- equivalent static fallback when software rendering is detected;
- no new console/page/request errors;
- no layout shift, clipping, or changed global effects.

Then verify the new payment dialog and settings editor separately. Payment UI
screenshots must look native to the production dialog/settings system rather
than like a new branded microsite.

The final Git guard is:

```text
git diff --exit-code 27023295d..HEAD -- \
  web/default/src/features/yucore-brand \
  web/default/src/styles
```

Any output blocks approval.

## Local Rollout Gate

1. Run the untouched production baseline locally on one port.
2. Run the ported candidate on a separate free port with an isolated database.
3. Show both to the user for visual comparison.
4. Keep all provider traffic local and synthetic; do not move funds.
5. Stop before production deployment.

Production hot-switch requires a separate explicit approval after:

- SHKeeper 1:1, zero-fee, no-recalculation configuration is verified;
- one small real payment succeeds on BSC, TRON, and Polygon;
- callback and scheduled reconciliation replay remain idempotent;
- rollback retains a binary capable of reconciling already-created fixed-package
  orders.

## Rollback

The current production container and Caddy target remain unchanged during local
work. A future production rollout retains the previous container and recovery
artifacts. Application rollback disables new invoices but must keep a compatible
SHKeeper reconciliation path available for funds already sent.

## Explicit Non-Goals

- Redesigning YuCore or changing global motion/performance behavior.
- Merging or cherry-picking the old SHKeeper branch wholesale.
- Cleaning unrelated account-pool, texture, lint, formatting, or copyright
  issues.
- Modifying Sub2API, production channels, prices, balances, usage logs, Caddy,
  databases, or containers during local implementation.
