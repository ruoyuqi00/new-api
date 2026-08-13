# Violation Fee Log Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Distinguish failed violation-fee attempts from successful deductions in both usage-log views without changing billing or stored data.

**Architecture:** Add one pure formatter in the existing usage-log formatting module that maps successful, failed, and legacy audit metadata to display keys and an amount. Both the table summary and details dialog consume that formatter, while all new copy is added through the existing six-locale i18n workflow.

**Tech Stack:** React 19, TypeScript, Vitest via Bun, i18next, Rsbuild.

---

### Task 1: Define and test violation-fee display state

**Files:**
- Modify: `web/default/src/features/usage-logs/types.ts`
- Modify: `web/default/src/features/usage-logs/lib/format.ts`
- Modify: `web/default/src/features/usage-logs/lib/format.test.ts`

- [ ] **Step 1: Add a failing behavioral test**

Import a wished-for `getViolationFeeDisplay` formatter and add deterministic cases asserting:

```ts
expect(
  getViolationFeeDisplay(
    { violation_fee: true, charge_succeeded: false, requested_quota: 2500 },
    0
  )
).toEqual({
  statusKey: 'Violation blocked, charge failed',
  amountKey: 'Attempted fee',
  amount: 2500,
})
```

Also assert a successful record prefers `charged_quota`, and a legacy record preserves `fee_quota` with the existing `Violation Fee`/`Fee` keys.

- [ ] **Step 2: Run the test and capture RED**

Run from `web/default/`:

```powershell
bun test src/features/usage-logs/lib/format.test.ts
```

Expected: FAIL because `getViolationFeeDisplay` is not exported.

- [ ] **Step 3: Add the typed pure formatter**

Extend `LogOtherData` with optional `requested_quota`, `charged_quota`, and `charge_succeeded`. Implement:

```ts
export type ViolationFeeDisplay = {
  statusKey: 'Violation Fee' | 'Violation blocked, charge failed'
  amountKey: 'Fee' | 'Attempted fee'
  amount: number
}

export function getViolationFeeDisplay(
  other: LogOtherData,
  logQuota: number
): ViolationFeeDisplay {
  if (other.charge_succeeded === false) {
    return {
      statusKey: 'Violation blocked, charge failed',
      amountKey: 'Attempted fee',
      amount: other.requested_quota ?? logQuota,
    }
  }
  if (other.charge_succeeded === true) {
    return {
      statusKey: 'Violation Fee',
      amountKey: 'Fee',
      amount: other.charged_quota ?? other.fee_quota ?? logQuota,
    }
  }
  return {
    statusKey: 'Violation Fee',
    amountKey: 'Fee',
    amount: other.fee_quota ?? logQuota,
  }
}
```

- [ ] **Step 4: Run the focused test and capture GREEN**

Run:

```powershell
bun test src/features/usage-logs/lib/format.test.ts
```

Expected: PASS.

### Task 2: Use one display rule in both log views

**Files:**
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`

- [ ] **Step 1: Replace inline violation amount selection**

In both components, call `getViolationFeeDisplay(other, log.quota)` only inside the already-confirmed violation branch. Render `t(display.statusKey)`, `t(display.amountKey)`, and `formatLogQuota(display.amount)`.

- [ ] **Step 2: Run focused lint and typecheck**

Run from `web/default/`:

```powershell
bun x oxlint -c .oxlintrc.json src/features/usage-logs/types.ts src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/format.test.ts src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/dialogs/details-dialog.tsx
bun run typecheck
```

Expected: both commands exit 0.

### Task 3: Add localized failure copy through the required script

**Files:**
- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`
- Delete after use: `web/default/scripts/add-missing-keys.mjs`

- [ ] **Step 1: Create the sanctioned locale update script**

Use the exact stable writer from `.agents/skills/i18n-translate/SKILL.md` and populate these two keys for all six locales:

```js
const newKeys = {
  en: {
    'Attempted fee': 'Attempted fee',
    'Violation blocked, charge failed': 'Violation blocked, charge failed',
  },
  zh: {
    'Attempted fee': '尝试扣费金额',
    'Violation blocked, charge failed': '违规已拦截，扣费失败',
  },
  fr: {
    'Attempted fee': 'Montant tenté',
    'Violation blocked, charge failed': 'Violation bloquée, débit échoué',
  },
  ja: {
    'Attempted fee': '請求予定額',
    'Violation blocked, charge failed': '違反をブロックしましたが、請求に失敗しました',
  },
  ru: {
    'Attempted fee': 'Попытка списания',
    'Violation blocked, charge failed': 'Нарушение заблокировано, списание не выполнено',
  },
  vi: {
    'Attempted fee': 'Số tiền thử khấu trừ',
    'Violation blocked, charge failed': 'Đã chặn vi phạm, khấu trừ thất bại',
  },
}
```

- [ ] **Step 2: Apply and normalize locales**

Run:

```powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Expected: both keys exist in all six locale files and the sync report has no missing key for either string.

- [ ] **Step 3: Delete the temporary locale writer**

Remove only `web/default/scripts/add-missing-keys.mjs` after confirming locale writes succeeded.

### Task 4: Verify and prepare local review

**Files:**
- No additional production files.

- [ ] **Step 1: Run focused and full frontend verification**

Run from `web/default/`:

```powershell
bun test src/features/usage-logs/lib/format.test.ts
bun run typecheck
bun x oxlint -c .oxlintrc.json src/features/usage-logs/types.ts src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/format.test.ts src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/dialogs/details-dialog.tsx
bun run build
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 2: Review scope and commit**

Confirm the diff contains only this plan, the three usage-log modules/components, their focused test, and six locale files. Commit with:

```powershell
git add docs/superpowers/plans/2026-08-13-violation-fee-log-display.md web/default/src/features/usage-logs web/default/src/i18n/locales
git commit -m "fix: clarify failed violation fee logs"
```

- [ ] **Step 3: Start the existing branded local candidate**

Start the existing project through its established local candidate workflow on a free loopback-only port. Verify `/`, `/sign-in`, and the usage-log route load without changing production traffic. Report the local URL for user review; do not deploy until the user explicitly approves.
