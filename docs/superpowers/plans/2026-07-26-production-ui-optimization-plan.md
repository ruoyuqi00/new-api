# Production UI Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the production-lineage YuCore home and authenticated console feel faster while preserving the current brand, themes, homepage, Studio, and visible motion.

**Architecture:** Keep the production shell and progressively split work below the first viewport into route-local chunks. Reuse the existing render-loop ownership primitive, make console CSS motion visibility-aware, and represent query loading, stale-error, and terminal-error states without redirecting ordinary data failures to a `500` page.

**Tech Stack:** React 19, TypeScript, Rsbuild, TanStack Router/Query, Base UI/shadcn components, Tailwind CSS, node:test, i18next, Bun

---

## File Map

- Preserve and complete `web/default/src/features/yucore-brand/components/yucore-home.tsx` and the untracked `yucore-home-details.tsx` split.
- Create `yucore-home-details-scheduler.ts` and `.test.ts`: one-owner idle/user-intent activation.
- Create `yucore-console-motion.ts` and `.test.ts`; modify `yucore-console-background.tsx`: bounded authenticated motion lifecycle.
- Create `dashboard/components/overview/overview-panel-plan.ts` and `.test.ts` plus `overview-secondary-panels.tsx`; modify `overview-dashboard.tsx`: lazy secondary dashboard chunk.
- Create `web/default/src/lib/query-display-state.ts` and `.test.ts`: shared loading/stale-error state derivation.
- Modify `dashboard/components/overview/summary-cards.tsx`, `usage-logs/components/usage-logs-table.tsx`, and `usage-logs/components/common-logs-stats.tsx`: preserve data and show actionable errors.
- Update all six locale JSON files only through `web/default/scripts/add-missing-keys.mjs`, then run `bun run i18n:sync` and delete the temporary script.

### Task 1: Finish the below-the-fold home split with deterministic scheduling

**Files:**
- Modify: `web/default/src/features/yucore-brand/components/yucore-home.tsx`
- Create: `web/default/src/features/yucore-brand/components/yucore-home-details.tsx`
- Create: `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.ts`
- Create: `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts`

- [ ] **Step 1: Add failing scheduler ownership tests**

Define tests with a fake host that records one idle callback, one animation frame, and user-intent listeners. Assert that scroll/pointer/keyboard intent reveals details once, cancels pending idle work, and cleanup prevents late state updates. Assert the no-`requestIdleCallback` path uses exactly one animation frame.

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { scheduleYucoreHomeDetails } from './yucore-home-details-scheduler'

describe('YuCore home details scheduler', () => {
  test('user intent wins over idle work and reveals details once', () => {
    const host = createFakeHomeScheduler({ idle: true })
    let reveals = 0
    const dispose = scheduleYucoreHomeDetails(host, () => reveals++)

    host.emit('scroll')
    host.flushIdle()

    assert.equal(reveals, 1)
    assert.equal(host.pendingIdleCount(), 0)
    dispose()
  })

  test('falls back to one animation frame when idle callbacks are unavailable', () => {
    const host = createFakeHomeScheduler({ idle: false })
    let reveals = 0
    scheduleYucoreHomeDetails(host, () => reveals++)

    assert.equal(host.pendingFrameCount(), 1)
    host.flushFrame()
    assert.equal(reveals, 1)
    assert.equal(host.pendingFrameCount(), 0)
  })
})
```

Add this complete fake above the tests:

```ts
type FakeSchedulerOptions = { idle: boolean }

function createFakeHomeScheduler(options: FakeSchedulerOptions) {
  let nextHandle = 1
  const listeners = new Map<string, Set<() => void>>()
  const idleCallbacks = new Map<number, () => void>()
  const frameCallbacks = new Map<number, FrameRequestCallback>()

  return {
    addEventListener(type: string, listener: () => void) {
      const registered = listeners.get(type) ?? new Set<() => void>()
      registered.add(listener)
      listeners.set(type, registered)
    },
    cancelAnimationFrame(handle: number) {
      frameCallbacks.delete(handle)
    },
    cancelIdleCallback(handle: number) {
      idleCallbacks.delete(handle)
    },
    emit(type: string) {
      for (const listener of [...(listeners.get(type) ?? [])]) listener()
    },
    flushFrame() {
      const callbacks = [...frameCallbacks.values()]
      frameCallbacks.clear()
      for (const callback of callbacks) callback(16)
    },
    flushIdle() {
      const callbacks = [...idleCallbacks.values()]
      idleCallbacks.clear()
      for (const callback of callbacks) callback()
    },
    pendingFrameCount() {
      return frameCallbacks.size
    },
    pendingIdleCount() {
      return idleCallbacks.size
    },
    removeEventListener(type: string, listener: () => void) {
      listeners.get(type)?.delete(listener)
    },
    requestAnimationFrame(callback: FrameRequestCallback) {
      const handle = nextHandle++
      frameCallbacks.set(handle, callback)
      return handle
    },
    requestIdleCallback: options.idle
      ? (callback: () => void) => {
          const handle = nextHandle++
          idleCallbacks.set(handle, callback)
          return handle
        }
      : undefined,
  }
}
```

- [ ] **Step 2: Run the scheduler test and verify it is red**

Run from `web/default`: `node --test src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts`

Expected: FAIL because the scheduler module does not exist.

- [ ] **Step 3: Implement one-owner idle and intent scheduling**

Create the scheduler with this public contract:

```ts
export type YucoreHomeSchedulerHost = {
  addEventListener: (type: string, listener: () => void, options?: AddEventListenerOptions) => void
  removeEventListener: (type: string, listener: () => void) => void
  requestAnimationFrame: (callback: FrameRequestCallback) => number
  cancelAnimationFrame: (handle: number) => void
  requestIdleCallback?: (callback: () => void, options?: { timeout?: number }) => number
  cancelIdleCallback?: (handle: number) => void
}

export function scheduleYucoreHomeDetails(
  host: YucoreHomeSchedulerHost,
  onReady: () => void
): () => void {
  let disposed = false
  let completed = false
  let frame = 0
  let idle = 0
  const events = ['pointerdown', 'keydown', 'scroll'] as const

  const cleanup = () => {
    if (frame) host.cancelAnimationFrame(frame)
    if (idle) host.cancelIdleCallback?.(idle)
    frame = 0
    idle = 0
    for (const event of events) host.removeEventListener(event, reveal)
  }
  const reveal = () => {
    if (disposed || completed) return
    completed = true
    cleanup()
    onReady()
  }

  host.addEventListener('keydown', reveal, { once: true })
  host.addEventListener('pointerdown', reveal, { once: true, passive: true })
  host.addEventListener('scroll', reveal, { once: true, passive: true })
  if (host.requestIdleCallback && host.cancelIdleCallback) {
    idle = host.requestIdleCallback(reveal, { timeout: 1200 })
  } else {
    frame = host.requestAnimationFrame(reveal)
  }

  return () => {
    disposed = true
    cleanup()
  }
}
```

Use `{ once: true }` for `keydown` and `{ once: true, passive: true }` for pointer/scroll; keep the same `reveal` callback and cleanup semantics for all three listeners.

- [ ] **Step 4: Wire the existing lazy details chunk to the scheduler**

Keep the already prepared `lazy(() => import('./yucore-home-details'))` and `Suspense` boundary. Replace the inline idle effect with:

```ts
useEffect(() => {
  if (!revealHero) return
  return scheduleYucoreHomeDetails(
    window as unknown as YucoreHomeSchedulerHost,
    () => setShowHomeDetails(true)
  )
}, [revealHero])
```

Keep the fallback `null` because the hero and the original page background remain visible while the below-fold chunk loads; no blank viewport is introduced.

- [ ] **Step 5: Verify scheduler behavior, types, and the production build**

Run from `web/default`:

```bash
node --test src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-home.tsx src/features/yucore-brand/components/yucore-home-details.tsx src/features/yucore-brand/components/yucore-home-details-scheduler.ts src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
bun run build:check
```

Expected: all exit 0 and the build emits `yucore-home-details` as a separate async chunk.

- [ ] **Step 6: Commit the complete home split, including the previously untracked file**

```bash
git add web/default/src/features/yucore-brand/components/yucore-home.tsx web/default/src/features/yucore-brand/components/yucore-home-details.tsx web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.ts web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
git commit -m "perf: defer production home detail sections"
```

### Task 2: Pause authenticated background motion when it cannot be seen

**Files:**
- Create: `web/default/src/features/yucore-brand/components/yucore-console-motion.ts`
- Create: `web/default/src/features/yucore-brand/components/yucore-console-motion.test.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-console-background.tsx`

- [ ] **Step 1: Write the motion-profile matrix first**

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getYucoreConsoleMotionMode } from './yucore-console-motion'

describe('YuCore console motion profile', () => {
  test('keeps ordinary console routes ambient only while visible and active', () => {
    assert.equal(getYucoreConsoleMotionMode('/dashboard', true, true), 'ambient')
    assert.equal(getYucoreConsoleMotionMode('/dashboard', false, true), 'static')
    assert.equal(getYucoreConsoleMotionMode('/dashboard', true, false), 'static')
  })

  test('keeps canvas and studio routes static to reserve their rendering budget', () => {
    assert.equal(getYucoreConsoleMotionMode('/playground/canvas', true, true), 'static')
    assert.equal(getYucoreConsoleMotionMode('/playground/studio', true, true), 'static')
  })
})
```

- [ ] **Step 2: Run the test and observe the missing module failure**

Run: `node --test src/features/yucore-brand/components/yucore-console-motion.test.ts`

Expected: FAIL because the motion-profile module does not exist.

- [ ] **Step 3: Implement the pure profile and document-visibility state**

Create:

```ts
export type YucoreConsoleMotionMode = 'ambient' | 'static'

export function getYucoreConsoleMotionMode(
  pathname: string,
  active: boolean,
  documentVisible: boolean
): YucoreConsoleMotionMode {
  if (!active || !documentVisible) return 'static'
  if (pathname === '/playground/canvas' || pathname === '/playground/studio') {
    return 'static'
  }
  return 'ambient'
}
```

In `yucore-console-background.tsx`, track `!document.hidden` with one `visibilitychange` listener, compute the profile through this function, and keep the existing SVG/CSS DOM unchanged. Do not remove or shorten any animation.

- [ ] **Step 4: Verify no hidden route owns ambient CSS motion**

Run:

```bash
node --test src/features/yucore-brand/components/yucore-console-motion.test.ts src/features/yucore-brand/components/yucore-render-loop.test.ts
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-console-background.tsx src/features/yucore-brand/components/yucore-console-motion.ts src/features/yucore-brand/components/yucore-console-motion.test.ts
bun run typecheck
```

Expected: PASS; public WebGL render-loop tests remain unchanged and ordinary console motion resumes when visibility returns.

- [ ] **Step 5: Commit the bounded console profile**

```bash
git add web/default/src/features/yucore-brand/components/yucore-console-background.tsx web/default/src/features/yucore-brand/components/yucore-console-motion.ts web/default/src/features/yucore-brand/components/yucore-console-motion.test.ts
git commit -m "perf: bound authenticated console motion"
```

### Task 3: Split secondary overview panels from the route-ready dashboard

**Files:**
- Create: `web/default/src/features/dashboard/components/overview/overview-panel-plan.ts`
- Create: `web/default/src/features/dashboard/components/overview/overview-panel-plan.test.ts`
- Create: `web/default/src/features/dashboard/components/overview/overview-secondary-panels.tsx`
- Modify: `web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`

- [ ] **Step 1: Write the panel-visibility contract**

Create `overview-panel-plan.test.ts`:

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getOverviewPanelPlan } from './overview-panel-plan'

describe('overview panel plan', () => {
  test('uses only enabled panels for ordinary users', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: false,
        apiInfo: true,
        announcements: false,
        faq: true,
        uptime: true,
      }),
      { left: ['api-info', 'faq'], uptime: true }
    )
  })

  test('adds performance health for admins', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: true,
        apiInfo: false,
        announcements: true,
        faq: false,
        uptime: false,
      }),
      { left: ['performance', 'announcements'], uptime: false }
    )
  })

  test('does no secondary panel work when everything is disabled', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: false,
        apiInfo: false,
        announcements: false,
        faq: false,
        uptime: false,
      }),
      { left: [], uptime: false }
    )
  })
})
```

```ts
export type OverviewPanelPlan = {
  left: Array<'performance' | 'api-info' | 'announcements' | 'faq'>
  uptime: boolean
}
```

Run: `node --test src/features/dashboard/components/overview/overview-panel-plan.test.ts`

Expected: FAIL because the production module does not exist.

- [ ] **Step 2: Implement the pure plan**

```ts
export function getOverviewPanelPlan(input: {
  isAdmin: boolean
  apiInfo: boolean
  announcements: boolean
  faq: boolean
  uptime: boolean
}): OverviewPanelPlan {
  const left: OverviewPanelPlan['left'] = []
  if (input.isAdmin) left.push('performance')
  if (input.apiInfo) left.push('api-info')
  if (input.announcements) left.push('announcements')
  if (input.faq) left.push('faq')
  return { left, uptime: input.uptime }
}
```

- [ ] **Step 3: Move only the summary/content section into an async module**

`overview-secondary-panels.tsx` imports `SummaryCards`, `PerformanceHealthPanel`, `ApiInfoPanel`, `AnnouncementsPanel`, `FAQPanel`, and `UptimePanel`. Use this component body so panel order and current classes remain unchanged:

```tsx
export function OverviewSecondaryPanels(props: {
  plan: OverviewPanelPlan
}) {
  const hasLeftPanels = props.plan.left.length > 0
  const showContentPanels = hasLeftPanels || props.plan.uptime

  return (
    <>
      <SummaryCards />
      {showContentPanels && (
        <CardStaggerContainer
          className={cn(
            'grid grid-cols-1 gap-4',
            hasLeftPanels &&
              props.plan.uptime &&
              'xl:grid-cols-[minmax(0,1fr)_22rem]'
          )}
        >
          {hasLeftPanels && (
            <div
              className={cn(
                'grid min-w-0 grid-cols-1 gap-4',
                props.plan.left.some((panel) => panel !== 'performance') &&
                  'lg:grid-cols-2'
              )}
            >
              {props.plan.left.includes('performance') && (
                <CardStaggerItem className='lg:col-span-2'>
                  <PerformanceHealthPanel />
                </CardStaggerItem>
              )}
              {props.plan.left.includes('api-info') && (
                <CardStaggerItem><ApiInfoPanel /></CardStaggerItem>
              )}
              {props.plan.left.includes('announcements') && (
                <CardStaggerItem><AnnouncementsPanel /></CardStaggerItem>
              )}
              {props.plan.left.includes('faq') && (
                <CardStaggerItem><FAQPanel /></CardStaggerItem>
              )}
            </div>
          )}
          {props.plan.uptime && (
            <CardStaggerItem><UptimePanel /></CardStaggerItem>
          )}
        </CardStaggerContainer>
      )}
    </>
  )
}
```

Use these imports above the component (after the required existing project copyright header):

```ts
import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { cn } from '@/lib/utils'

import type { OverviewPanelPlan } from './overview-panel-plan'
import { AnnouncementsPanel } from './announcements-panel'
import { ApiInfoPanel } from './api-info-panel'
import { FAQPanel } from './faq-panel'
import { PerformanceHealthPanel } from './performance-health-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'
```

Do not alter role checks, panel order, card styles, or copy.

In `overview-dashboard.tsx`, replace eager imports with:

```ts
const LazyOverviewSecondaryPanels = lazy(() =>
  import('./overview-secondary-panels').then((module) => ({
    default: module.OverviewSecondaryPanels,
  }))
)
```

Compute the plan in `OverviewDashboard`:

```ts
const panelPlan = useMemo(
  () =>
    getOverviewPanelPlan({
      isAdmin,
      apiInfo: showApiInfoPanel,
      announcements: showAnnouncementsPanel,
      faq: showFAQPanel,
      uptime: showUptimePanel,
    }),
  [
    isAdmin,
    showAnnouncementsPanel,
    showApiInfoPanel,
    showFAQPanel,
    showUptimePanel,
  ]
)
```

Then render:

```tsx
<Suspense fallback={<OverviewSecondaryPanelsFallback plan={panelPlan} />}>
  <LazyOverviewSecondaryPanels plan={panelPlan} />
</Suspense>
```

Add this fallback in `overview-dashboard.tsx` and import `Skeleton` plus `type OverviewPanelPlan`:

```tsx
function OverviewSecondaryPanelsFallback(props: {
  plan: OverviewPanelPlan
}) {
  const showContentPanels = props.plan.left.length > 0 || props.plan.uptime

  return (
    <div className='space-y-4' aria-hidden='true'>
      <Skeleton className='h-64 w-full rounded-lg' />
      {showContentPanels && (
        <Skeleton className='h-72 w-full rounded-lg' />
      )}
    </div>
  )
}
```

Stable height prevents the overview from jumping while the async module loads.

- [ ] **Step 4: Verify the async boundary and unchanged UI contract**

Run:

```bash
node --test src/features/dashboard/components/overview/overview-panel-plan.test.ts
bunx oxlint -c .oxlintrc.json src/features/dashboard/components/overview/overview-dashboard.tsx src/features/dashboard/components/overview/overview-secondary-panels.tsx src/features/dashboard/components/overview/overview-panel-plan.ts src/features/dashboard/components/overview/overview-panel-plan.test.ts
bun run build:check
```

Expected: PASS and an overview-secondary async chunk appears in `dist/static/js`; the dashboard hero and setup guide remain in the route-ready chunk.

- [ ] **Step 5: Commit the overview split**

```bash
git add web/default/src/features/dashboard/components/overview/overview-dashboard.tsx web/default/src/features/dashboard/components/overview/overview-secondary-panels.tsx web/default/src/features/dashboard/components/overview/overview-panel-plan.ts web/default/src/features/dashboard/components/overview/overview-panel-plan.test.ts
git commit -m "perf: split secondary dashboard panels"
```

### Task 4: Preserve data on refresh errors and show actionable states

**Files:**
- Create: `web/default/src/lib/query-display-state.ts`
- Create: `web/default/src/lib/query-display-state.test.ts`
- Modify: `web/default/src/features/dashboard/components/overview/summary-cards.tsx`
- Modify: `web/default/src/features/usage-logs/components/usage-logs-table.tsx`
- Modify: `web/default/src/features/usage-logs/components/common-logs-stats.tsx`
- Temporary create/delete: `web/default/scripts/add-missing-keys.mjs`
- Generated modify: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Write a shared query-state decision table**

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getQueryDisplayState } from './query-display-state'

describe('query display state', () => {
  test('distinguishes first load, refresh, stale error, and terminal error', () => {
    assert.equal(getQueryDisplayState({ hasData: false, isPending: true, isFetching: true, isError: false }), 'initial-loading')
    assert.equal(getQueryDisplayState({ hasData: true, isPending: false, isFetching: true, isError: false }), 'refreshing')
    assert.equal(getQueryDisplayState({ hasData: true, isPending: false, isFetching: false, isError: true }), 'stale-error')
    assert.equal(getQueryDisplayState({ hasData: false, isPending: false, isFetching: false, isError: true }), 'terminal-error')
    assert.equal(getQueryDisplayState({ hasData: true, isPending: false, isFetching: false, isError: false }), 'ready')
  })
})
```

Run: `node --test src/lib/query-display-state.test.ts`

Expected: FAIL because the module is absent.

- [ ] **Step 2: Implement the pure state resolver**

```ts
export type QueryDisplayState =
  | 'initial-loading'
  | 'refreshing'
  | 'stale-error'
  | 'terminal-error'
  | 'ready'

export function getQueryDisplayState(input: {
  hasData: boolean
  isPending: boolean
  isFetching: boolean
  isError: boolean
}): QueryDisplayState {
  if (input.isError) return input.hasData ? 'stale-error' : 'terminal-error'
  if (!input.hasData && input.isPending) return 'initial-loading'
  if (input.hasData && input.isFetching) return 'refreshing'
  return 'ready'
}
```

- [ ] **Step 3: Apply the state contract to overview and usage logs**

For `SummaryCards`, add `placeholderData: previousData => previousData` and `meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR`. Derive state with:

```ts
const usageDisplayState = getQueryDisplayState({
  hasData: usageTrendQuery.data !== undefined,
  isPending: usageTrendQuery.isPending,
  isFetching: usageTrendQuery.isFetching,
  isError: usageTrendQuery.isError,
})
const summaryLoading =
  loading || usageDisplayState === 'initial-loading'
const showUsageError =
  usageDisplayState === 'stale-error' ||
  usageDisplayState === 'terminal-error'
```

Pass `summaryLoading` to every `StatCard`. Immediately below the summary heading render:

```tsx
{showUsageError && (
  <Alert variant='destructive' className='mt-2'>
    <TriangleAlert aria-hidden='true' />
    <AlertTitle>{t('Unable to load usage summary')}</AlertTitle>
    {usageDisplayState === 'stale-error' && (
      <AlertDescription>
        {t('Showing the last available data.')}
      </AlertDescription>
    )}
    <AlertAction>
      <Button
        variant='outline'
        size='sm'
        onClick={() => void usageTrendQuery.refetch()}
      >
        <RefreshCw data-icon='inline-start' />
        {t('Retry')}
      </Button>
    </AlertAction>
  </Alert>
)}
```

For both usage-log queries, throw `new Error(result.message || t('Failed to load logs'))` when `success` is false instead of returning zero-shaped data. In `UsageLogsTable`, retain `data`, `isLoading`, and `isFetching`, and additionally read `isError` and `refetch`. Derive:

```ts
const logsDisplayState = getQueryDisplayState({
  hasData: data !== undefined,
  isPending: isLoading,
  isFetching,
  isError,
})
```

Wrap the existing `DataTablePage` in `div.flex.h-full.min-h-0.flex-col.gap-2`; when the state is `stale-error` or `terminal-error`, render this before the table:

```tsx
<Alert variant='destructive' className='shrink-0'>
  <TriangleAlert aria-hidden='true' />
  <AlertTitle>{t('Failed to load logs')}</AlertTitle>
  {logsDisplayState === 'stale-error' && (
    <AlertDescription>{t('Showing the last available data.')}</AlertDescription>
  )}
  <AlertAction>
    <Button variant='outline' size='sm' onClick={() => void refetch()}>
      <RefreshCw data-icon='inline-start' />
      {t('Retry')}
    </Button>
  </AlertAction>
</Alert>
```

Pass `className='min-h-0 flex-1'` to `DataTablePage`; its existing `DataTablePageProps` already supports that property. In `CommonLogsStats`, use the same thrown error and state resolver. A terminal error renders a compact outline `Button` with `RefreshCw`, `t('Retry')`, and `onClick={() => void refetch()}`; a stale error keeps the badges and adds the same retry button after them. Do not change filters, pagination, columns, or API request shapes.

- [ ] **Step 4: Add the two new strings through the mandatory locale script**

Create `scripts/add-missing-keys.mjs` with the complete stable writer and this `newKeys` object:

```js
const newKeys = {
  en: {
    'Showing the last available data.': 'Showing the last available data.',
    'Unable to load usage summary': 'Unable to load usage summary',
  },
  zh: {
    'Showing the last available data.': '正在显示最近一次可用的数据。',
    'Unable to load usage summary': '无法加载用量摘要',
  },
  fr: {
    'Showing the last available data.': 'Affichage des dernières données disponibles.',
    'Unable to load usage summary': "Impossible de charger le résumé d’utilisation",
  },
  ja: {
    'Showing the last available data.': '最後に取得できたデータを表示しています。',
    'Unable to load usage summary': '使用量の概要を読み込めません',
  },
  ru: {
    'Showing the last available data.': 'Показаны последние доступные данные.',
    'Unable to load usage summary': 'Не удалось загрузить сводку использования',
  },
  vi: {
    'Showing the last available data.': 'Đang hiển thị dữ liệu khả dụng gần nhất.',
    'Unable to load usage summary': 'Không thể tải tóm tắt mức sử dụng',
  },
}

async function main() {
  let totalApplied = 0
  for (const [locale, translations] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))
    let applied = 0
    for (const [key, value] of Object.entries(translations)) {
      if (json.translation[key] !== value) {
        json.translation[key] = value
        applied++
      }
    }
    if (applied > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) =>
          a.localeCompare(b)
        )
      )
      await fs.writeFile(filePath, `${JSON.stringify(json, null, 2)}\n`, 'utf8')
    }
    console.log(`${locale}: ${applied} translations applied`)
    totalApplied += applied
  }
  console.log(`Total: ${totalApplied} translations applied`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
```

Prepend these imports and constant to that script:

```js
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')
```

Also create `scripts/find-missing-keys.mjs` for read-only verification:

```js
import fs from 'node:fs/promises'
import path from 'node:path'

const localesDir = path.resolve('src/i18n/locales')
const sourceDir = path.resolve('src')
const en = JSON.parse(
  await fs.readFile(path.join(localesDir, 'en.json'), 'utf8')
)
const knownKeys = new Set(Object.keys(en.translation))
const missing = new Map()

async function walk(directory) {
  const files = []
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === 'locales') continue
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) files.push(...(await walk(fullPath)))
    else if (/\.(tsx?|jsx?)$/.test(entry.name)) files.push(fullPath)
  }
  return files
}

const literalCall = /\bt\(\s*['"`]([^'"`\n]+?)['"`]\s*[,)]/g
for (const file of await walk(sourceDir)) {
  const source = await fs.readFile(file, 'utf8')
  literalCall.lastIndex = 0
  let match
  while ((match = literalCall.exec(source)) !== null) {
    const key = match[1]
    if (key.includes('${') || knownKeys.has(key)) continue
    const locations = missing.get(key) ?? []
    locations.push(path.relative(sourceDir, file))
    missing.set(key, locations)
  }
}

if (missing.size > 0) {
  for (const [key, files] of missing) {
    console.error(`${key}: ${[...new Set(files)].join(', ')}`)
  }
  process.exitCode = 1
} else {
  console.log('All t() keys found in en.json!')
}
```

Run from `web/default`:

```bash
node scripts/add-missing-keys.mjs
bun run i18n:sync
node scripts/find-missing-keys.mjs
```

Expected: six locales updated and the final command reports all literal `t()` keys present. Delete the two temporary scripts `add-missing-keys.mjs` and `find-missing-keys.mjs`; preserve all pre-existing project scripts.

- [ ] **Step 5: Verify query states, i18n, and the frontend build**

Run:

```bash
node --test src/lib/query-display-state.test.ts
bunx oxlint -c .oxlintrc.json src/lib/query-display-state.ts src/lib/query-display-state.test.ts src/features/dashboard/components/overview/summary-cards.tsx src/features/usage-logs/components/usage-logs-table.tsx src/features/usage-logs/components/common-logs-stats.tsx
bun run typecheck
bun run format:check
bun run build
```

Expected: all commands exit 0; a failed refresh keeps the current page and last available rows instead of rendering a false empty state or routing to a `500` page.

- [ ] **Step 6: Commit data-state and locale changes together**

```bash
git add web/default/src/lib/query-display-state.ts web/default/src/lib/query-display-state.test.ts web/default/src/features/dashboard/components/overview/summary-cards.tsx web/default/src/features/usage-logs/components/usage-logs-table.tsx web/default/src/features/usage-logs/components/common-logs-stats.tsx web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "fix: preserve dashboard data on refresh errors"
```

### Task 5: Build and inspect every local role before production approval

**Files:**
- Verification only; no production action.

- [ ] **Step 1: Run the complete frontend verification set**

From `web/default`:

```bash
node --test src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts src/features/yucore-brand/components/yucore-console-motion.test.ts src/features/yucore-brand/components/yucore-render-loop.test.ts src/features/dashboard/components/overview/overview-panel-plan.test.ts src/lib/query-display-state.test.ts
bun run typecheck
bun run lint
bun run format:check
bun run build
```

From `web/classic`: `bun run build`

Expected: every command exits 0.

- [ ] **Step 2: Start a local-only frontend against the existing local backend**

From `web/default` run: `bun run dev -- --port 3001`

Expected: Rsbuild prints `http://localhost:3001`; `/api`, `/mj`, and `/pg` proxy only to the local backend at `http://localhost:3000`.

- [ ] **Step 3: Inspect anonymous, user, and super-admin surfaces**

Use the isolated local accounts only:

```text
Anonymous: no login
User:      localuser / LocalUser!2026
Admin:     localadmin / LocalAdmin!2026
```

At desktop `1440x900` and mobile `390x844`, inspect `/`, `/sign-in`, `/dashboard`, `/keys`, `/usage-logs`, `/wallet`, `/playground/studio`, `/users`, `/channels`, `/account-pools`, private-group pricing/configuration, and system settings. Open a local usage-log row containing cache tokens and confirm cache read/write values and pricing remain visible in both the table and details dialog. Confirm light and dark themes, readable text, no blank canvas, no overlapping controls, no console exceptions, and no local API `500` responses.

- [ ] **Step 4: Record before/after performance evidence**

Repeat the baseline method from `docs/superpowers/perf/2026-07-25-local-baseline.md`: record FCP, LCP, long-task count/duration, homepage chunk requests, and active canvases while visible/offscreen. Add a dated local-only results section; claim improvement only when the same browser/viewport evidence supports it.

- [ ] **Step 5: Stop at the production gate**

Run: `git status --short --branch`

Expected: local commits and any intentional preview notes only. Do not push, build a production image, SSH to the server, migrate a database, restart a container, or deploy until the user has reviewed all three local roles and separately approves production work.
