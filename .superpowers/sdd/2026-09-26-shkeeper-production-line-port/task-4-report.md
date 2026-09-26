# Task 4 report — native production wallet flow

Status: implemented and verified. All work is confined to the production-line worktree and wallet feature plus this report.

Implementation commit: `b5ee7fbf9` (`feat: add native fixed-package wallet flow`). The report directory is ignored by the repository, so this specifically requested report is explicitly staged in its own documentation commit.

## TDD evidence

Commands run from `web/default`:

1. Before adding production helpers, contracts, API methods, or hook:
   `bun test src/features/wallet/lib/shkeeper-payment-model.test.ts src/features/wallet/lib/payment.test.ts`
   RED: exit 1; 0 pass, 2 fail, 2 errors. Missing `parseOrdinaryTopUpAmount` export and missing `../hooks/use-shkeeper-payment` module, exactly the absent functionality anticipated by the brief.
2. After adding minimal model/API/query implementation, the same command was GREEN: exit 0; 5 pass, 0 fail.
3. Added an ordinary payment hook regression before changing the hooks:
   `bun test src/features/wallet/lib/payment.test.ts`
   RED: exit 1; 1 pass, 1 fail. Six invalid amounts caused 30 quote/create requests where the expected request list was empty. The test exercised real hooks through a React server-render harness and intercepted only Axios transport.
4. Added integer guards and removed flooring in ordinary/Waffo/Pancake hooks. The same command was GREEN: exit 0; 2 pass, 0 fail.
5. Final focused command: exit 0; 6 pass, 0 fail. The ordinary hook test additionally verifies the preserved valid integer request routes/payloads for Stripe, Epay, Waffo, and Pancake.

Tests protect immutable package sorting, all eight server statuses and recovery eligibility, partial zero-credit behavior, once-per-order positive credit refresh claims, digit-only positive safe integers, bounded status retries, explicit HTTP-200 business failure rejection, retention of the last valid invoice, same-order recovery without another create, and the rescan endpoint/body contract. The QueryClient test uses retryDelay 0 and awaited promises, with no sleeps or timing comparisons. No tests inspect source text.

## Implementation

- `types.ts`: Task 2 public package, network, expiry, recovery, request, and invoice contracts.
- `api.ts`: existing shared API instance for create/status/rescan. Create explicitly sends only `usdt_amount` and `crypto`; rescan sends only `txid`. HTTP-200 `success:false` rejects, with localized display owned by the dialog/form. Status receives the query cancellation signal.
- `lib/shkeeper-payment-model.ts`: immutable sorting, server-status terminal/recovery rules, and once-per-trade positive-credit refresh guard.
- `hooks/use-shkeeper-payment.ts` and `hooks/index.ts`: one creation attempt per dialog session, one trade number after creation, 3-second nonterminal polling, one retry per failed fetch, retained cached invoice, same-order Retry, no create mutation retries, and wallet refresh on the first positive credited balance. Closing unmounts the hook/query observer and consumes the query abort signal.
- `components/dialogs/shkeeper-{payment-dialog,package-picker,invoice-view,transaction-recovery}.tsx`: same-dialog package/network selection and invoice, exact amounts, fixed balance credit, status, expiry, address/QR, copy controls, received/credited totals, detected transaction IDs, localized inline polling errors, and provider-only rescan recovery.
- `components/recharge-form-card.tsx` and `index.tsx`: independent USDT action outside ordinary minimum validation; conditional mounting clears package/network/hash state on close. Ordinary amount input uses numeric keyboard/step 1 and accepts only digit text, with positive safe-integer validation. Ordinary initialization runs once so clearing the input is not immediately replaced with the minimum. Local input draft is derived against the current amount rather than mirrored in an effect.
- `lib/payment.ts`, `hooks/use-payment.ts`, `hooks/use-waffo-payment.ts`, and `hooks/use-waffo-pancake-payment.ts`: reject invalid values before requests instead of flooring. Existing valid provider request/redirect logic remains intact.
- `lib/payment.test.ts` and `lib/shkeeper-payment-model.test.ts`: focused behavioral regression coverage described above.

Existing touched-file nested ternary/index-key lint errors were resolved with direct branches and stable choice/skeleton identities. Existing license headers were preserved; new source files carry the same project copyright header.

## UI and i18n discipline

Read project shadcn, React performance, and i18n skills. `bunx shadcn@latest info --json` confirmed Base UI, base-nova, Tailwind v4, and Hugeicons. Inspected current recharge form, payment confirmation, Dialog primitives, Alert/Badge/Input, copy button, and existing radio option-set composition. There is no component named OptionSet in this checkout; the picker uses the current Base UI `RadioGroup`/`RadioGroupItem` primitives and label composition.

Packages use two columns on mobile and four on desktop; networks use one column on mobile and three on desktop. Selection keeps border width and dimensions stable. Dialog content scrolls normally with a reachable footer/recovery form. No new cards, marketing panel, fixed/sticky body section, global effects, raw color values, manual dark overrides, CSS, shell, or route changes. The old shkeeper-usdt branch was not used as a visual reference.

New UI copy uses literal English `t(...)` keys. Administrator-provided package labels and technical network names remain data. Locale JSON was intentionally not edited: Task 5 owns all six locale writes and synchronization via the approved script.

## Verification

- Focused Bun tests: PASS, 6/6.
- `bun run typecheck`: PASS, exit 0 (`tsgo -b`).
- `bunx oxlint -c .oxlintrc.json` on all 17 Task 4 wallet source/test files: PASS, exit 0, zero errors. One pre-existing warning remains in the unchanged Safari detection expression in `lib/payment.ts` (`prefer-includes`).
- `bunx oxfmt --check` on the same 17 files: PASS, exit 0, all matched files formatted. Formatter writes used only those scoped files.
- `bun run build`: PASS, exit 0; Rsbuild completed in 24.0 seconds. Reported full application output was 57,117.7 kB, 16,635.2 kB gzip; no before/after bundle comparison was claimed.
- `git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles`: PASS.
- Stronger working-tree guard, `git diff --exit-code 27023295d -- web/default/src/features/yucore-brand web/default/src/styles`: PASS.
- `git diff --check`: PASS.

## Self-review and concerns

Checked backend endpoint/status contracts, no client balance in create requests, no creation during Retry, bounded query retries, credit refresh deduplication, partial/excess copy, recovery eligibility, mounted-session cleanup, and stable responsive option dimensions. Query errors use the existing local error policy; mutation errors override the global handler so the localized inline/toast explanation owns display.

No production services, payment provider, deployment, or real balances were touched. No browser screenshot/E2E pass was performed in Task 4; Task 5 should verify mobile/desktop dialog layout, live hook mount/unmount behavior, and the full six-locale UI. Creem source/flow was not changed. A lost create response deliberately disables another creation attempt in that session and directs the user to order history before starting another payment; this avoids an unsafe automatic duplicate invoice.
