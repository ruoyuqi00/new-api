# Usage Log Load Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the default production usage-log query from the current-day million-row range to a rolling-hour range and reduce initial desktop rendering from 100 to 30 rows.

**Architecture:** Keep the existing list/stat APIs and filter builder. Change only the frontend defaults so explicit user-selected ranges and page sizes remain untouched.

**Tech Stack:** React 19, TypeScript, TanStack Query/Table, Bun/Node tests.

---

### Task 1: Rolling default range

**Files:**
- Modify: `web/default/src/features/usage-logs/lib/utils.ts`
- Create: `web/default/src/features/usage-logs/lib/utils.test.ts`

- [ ] **Step 1: Write the failing test**

Add a deterministic test that calls `getDefaultTimeRange(now)` with a fixed
date and expects `start = now - 1 hour` and `end = now + 1 hour`. Add a second
assertion that `buildApiParams` preserves explicit timestamps.

- [ ] **Step 2: Run the focused test and verify RED**

Run from `web/default`:

```powershell
bun test src/features/usage-logs/lib/utils.test.ts
```

Expected: failure because `getDefaultTimeRange` does not accept the injected
clock and currently starts at midnight.

- [ ] **Step 3: Implement the rolling range**

Change the function to accept a defaulted clock and calculate the bounded
window directly:

```ts
export function getDefaultTimeRange(now: Date = new Date()) {
  return {
    start: new Date(now.getTime() - 60 * 60 * 1000),
    end: new Date(now.getTime() + 60 * 60 * 1000),
  }
}
```

Do not change explicit route timestamp handling.

- [ ] **Step 4: Run focused and related frontend tests**

```powershell
bun test src/features/usage-logs/lib/utils.test.ts src/features/usage-logs/lib/query-params.test.ts src/features/usage-logs/lib/format.test.ts
```

Expected: all pass.

### Task 2: Smaller desktop page

**Files:**
- Modify: `web/default/src/features/usage-logs/components/usage-logs-table.tsx`

- [ ] **Step 1: Change the observable default**

Change only the desktop default from 100 to 30, retaining mobile 20 and the
existing user-selectable page-size options.

- [ ] **Step 2: Verify frontend contracts**

```powershell
bun run typecheck
bun run build
bun x oxlint src/features/usage-logs/lib/utils.ts src/features/usage-logs/lib/utils.test.ts src/features/usage-logs/components/usage-logs-table.tsx
```

Expected: all exit 0.

- [ ] **Step 3: Commit**

```powershell
git add web/default/src/features/usage-logs/lib/utils.ts web/default/src/features/usage-logs/lib/utils.test.ts web/default/src/features/usage-logs/components/usage-logs-table.tsx
git commit -m "perf: bound default usage log queries"
```

### Task 3: Candidate verification

- [ ] Build the production-equivalent image.
- [ ] Start it on a loopback-only port with the existing production-safe local configuration.
- [ ] Confirm the brand UI is unchanged.
- [ ] Confirm the first common-log request sends `page_size=30` and rolling-hour timestamps.
- [ ] Compare local API latency with the current production aggregate measurements.
