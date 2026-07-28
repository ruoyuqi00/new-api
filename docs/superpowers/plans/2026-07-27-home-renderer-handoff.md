# Home Renderer Handoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove WebGL shader compilation and the single large detail-tree mount from the stable-home handoff without reducing or shortening any visible YuCore animation.

**Architecture:** Keep the entrance renderers unchanged, but initialize the persistent signal field and Earth in two inactive stages behind the opaque entrance layer. Separate each persistent renderer's GPU-resource effect from its `active` state so activation reuses the existing context and program. Split below-the-fold details into primary and secondary commits separated by a browser frame.

**Tech Stack:** React 19, TypeScript, WebGL 1, Bun test runner, Rsbuild, Playwright/CDP performance traces

---

## File Map

- Create `web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.ts`: stable resource keys that exclude runtime activation.
- Create `web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.test.ts`: proves `active` changes do not change GPU-resource identity.
- Create `web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.ts`: two-stage timer ownership for persistent background preparation.
- Create `web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts`: ordering and cleanup contract.
- Modify `web/default/src/features/yucore-brand/components/yucore-render-loop.test.ts`: inactive-to-active and repeated-sync contract.
- Modify `web/default/src/features/yucore-brand/components/yucore-signal-field-webgl.tsx`: keep the context/program alive across `active` changes.
- Modify `web/default/src/features/yucore-brand/components/yucore-webgl-earth.tsx`: keep the context/program/texture alive across `active` changes.
- Modify `web/default/src/features/yucore-brand/components/yucore-background.tsx`: mount signal and Earth independently according to preparation stage.
- Modify `web/default/src/features/yucore-brand/components/yucore-home.tsx`: own preparation stage and two detail stages.
- Modify `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.ts`: schedule primary details, then expose a separate post-commit frame scheduler for secondary details.
- Modify `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts`: protect the two-commit contract.
- Modify `web/default/src/features/yucore-brand/components/yucore-home-details.tsx`: export primary and secondary detail components.
- Modify `docs/superpowers/perf/2026-07-25-local-baseline.md`: record trace evidence and acceptance result.

### Task 1: Lock Renderer Resource And Prewarm Contracts

**Files:**
- Create: `web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.ts`
- Create: `web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.test.ts`
- Create: `web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.ts`
- Create: `web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-render-loop.test.ts`

- [ ] **Step 1: Write failing resource-identity tests**

Create `yucore-renderer-resource-key.test.ts` with explicit active-only and resource-changing cases:

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getEarthResourceKey,
  getSignalFieldResourceKey,
} from './yucore-renderer-resource-key'

describe('YuCore renderer resource keys', () => {
  test('activation does not change signal field resource identity', () => {
    const base = {
      active: false,
      colorMode: 'dark' as const,
      coreMode: 'ambient' as const,
      corePlacement: 'hero' as const,
      intensity: 'hero' as const,
      renderProfile: undefined,
    }

    assert.equal(
      getSignalFieldResourceKey(base),
      getSignalFieldResourceKey({ ...base, active: true })
    )
    assert.notEqual(
      getSignalFieldResourceKey(base),
      getSignalFieldResourceKey({ ...base, colorMode: 'light' })
    )
  })

  test('activation does not change Earth resource identity', () => {
    const base = {
      active: false,
      colorMode: 'dark' as const,
      density: 'loader' as const,
      timeOffsetSeconds: 0,
    }

    assert.equal(
      getEarthResourceKey(base),
      getEarthResourceKey({ ...base, active: true })
    )
    assert.notEqual(
      getEarthResourceKey(base),
      getEarthResourceKey({ ...base, density: 'persistent' })
    )
  })
})
```

- [ ] **Step 2: Write failing two-stage prewarm tests**

Create a deterministic fake timer host in `yucore-home-renderer-prewarm.test.ts`. The contract is signal first at 80 percent, Earth second at 90 percent, and cleanup prevents both:

```ts
test('prepares signal and Earth in separate ordered stages', () => {
  const host = createFakeTimerHost()
  const stages: string[] = []

  scheduleYucoreHomeRendererPrewarm(
    host,
    1000,
    () => stages.push('signal'),
    () => stages.push('all')
  )

  host.advanceTo(799)
  assert.deepEqual(stages, [])
  host.advanceTo(800)
  assert.deepEqual(stages, ['signal'])
  host.advanceTo(900)
  assert.deepEqual(stages, ['signal', 'all'])
})

test('cleanup cancels every pending preparation stage', () => {
  const host = createFakeTimerHost()
  const stages: string[] = []
  const dispose = scheduleYucoreHomeRendererPrewarm(
    host,
    1000,
    () => stages.push('signal'),
    () => stages.push('all')
  )

  dispose()
  host.advanceTo(1000)
  assert.deepEqual(stages, [])
})
```

- [ ] **Step 3: Extend the render-loop behavior test**

Add a case that starts inactive, activates twice, and verifies only one queued frame:

```ts
test('activates an initialized inactive renderer without duplicate frames', () => {
  const scheduler = createFrameScheduler()
  const loop = createYucoreRenderLoop({
    isActive: false,
    render: () => undefined,
    scheduler,
  })

  loop.start()
  assert.equal(scheduler.queuedCount(), 0)
  loop.setActive(true)
  loop.setActive(true)
  assert.equal(scheduler.queuedCount(), 1)
  loop.setActive(false)
  assert.equal(scheduler.queuedCount(), 0)
})
```

- [ ] **Step 4: Run tests and verify RED**

Run from `web/default`:

```powershell
bun test src/features/yucore-brand/components/yucore-renderer-resource-key.test.ts src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts src/features/yucore-brand/components/yucore-render-loop.test.ts
```

Expected: the two new modules fail to resolve; the existing render-loop case passes.

- [ ] **Step 5: Implement the pure contracts**

`yucore-renderer-resource-key.ts` must serialize only resource-owning properties:

```ts
type SignalFieldResourceProps = {
  active?: boolean
  colorMode?: 'dark' | 'light'
  coreMode?: 'full' | 'ambient'
  corePlacement?: 'auth' | 'hero' | 'intro'
  intensity?: 'calm' | 'hero' | 'workbench'
  renderProfile?: 'default' | 'console' | 'entrance'
}

type EarthResourceProps = {
  active?: boolean
  colorMode?: 'dark' | 'light'
  density?: 'loader' | 'persistent'
  timeOffsetSeconds?: number
}

export function getSignalFieldResourceKey(props: SignalFieldResourceProps) {
  return [
    props.colorMode ?? 'dark',
    props.coreMode ?? 'ambient',
    props.corePlacement ?? 'auth',
    props.intensity ?? 'calm',
    props.renderProfile ?? 'default',
  ].join(':')
}

export function getEarthResourceKey(props: EarthResourceProps) {
  return [
    props.colorMode ?? 'dark',
    props.density ?? 'persistent',
    props.timeOffsetSeconds ?? 0,
  ].join(':')
}
```

`yucore-home-renderer-prewarm.ts` must use the host for ownership and cleanup:

```ts
export type YucoreHomePrewarmHost = {
  clearTimeout(handle: number): void
  setTimeout(callback: () => void, delay: number): number
}

export function scheduleYucoreHomeRendererPrewarm(
  host: YucoreHomePrewarmHost,
  durationMs: number,
  prepareSignal: () => void,
  prepareAll: () => void
) {
  const handles = [
    host.setTimeout(prepareSignal, durationMs * 0.8),
    host.setTimeout(prepareAll, durationMs * 0.9),
  ]
  return () => handles.forEach((handle) => host.clearTimeout(handle))
}
```

- [ ] **Step 6: Verify GREEN and commit**

Run the Task 1 test command, then:

```powershell
git add web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.ts web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.test.ts web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.ts web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts web/default/src/features/yucore-brand/components/yucore-render-loop.test.ts
git commit -m "test: define production renderer handoff contracts"
```

### Task 2: Keep WebGL Resources Alive Across Activation

**Files:**
- Modify: `web/default/src/features/yucore-brand/components/yucore-signal-field-webgl.tsx`
- Modify: `web/default/src/features/yucore-brand/components/yucore-webgl-earth.tsx`

- [ ] **Step 1: Separate resource identity from active state in the signal field**

Add `activeRef`, `activationRef`, and `resourcePropsRef`. Compute
`resourceKey = getSignalFieldResourceKey(props)`. The resource effect reads a
single captured `resourceProps` object, uses `activeRef.current` inside render,
and depends only on `resourceKey`. Store this activation callback before the
effect returns:

```ts
let activeStartedAt = window.performance.now()
let wasActive = activeRef.current

activationRef.current = (nextActive) => {
  if (nextActive && !wasActive) {
    activeStartedAt = window.performance.now()
    lastAnimationTime = activeStartedAt
    lastRenderTime = Number.NEGATIVE_INFINITY
  }
  wasActive = nextActive
  activeRef.current = nextActive
  renderLoop.setActive(nextActive)
}
```

Replace constant `animate` reads with `activeRef.current`, including adaptive
quality, frame throttling, time, and reveal calculations. Use `activeStartedAt`
for reveal progress. Attach pointer listeners for every non-reduced,
non-console resource and let inactive canvases ignore animation through the
render loop. Clear `activationRef.current` during cleanup.

Add a separate effect after the resource effect:

```ts
useEffect(() => {
  activationRef.current?.(props.active !== false)
}, [props.active])
```

- [ ] **Step 2: Apply the same lifetime boundary to Earth**

Use `getEarthResourceKey(props)`, capture resource properties through a ref,
and make active changes call `renderLoop.setActive` without deleting the
program, buffer, texture, or image. Reset `sceneStartedAt` and
`lastRenderTime` only on inactive-to-active transition. In the image and
resize callbacks, replace `!animate` with `!activeRef.current`.

- [ ] **Step 3: Run focused and static checks**

Run from `web/default`:

```powershell
bun test src/features/yucore-brand/components/yucore-renderer-resource-key.test.ts src/features/yucore-brand/components/yucore-render-loop.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-signal-field-webgl.tsx src/features/yucore-brand/components/yucore-webgl-earth.tsx src/features/yucore-brand/components/yucore-renderer-resource-key.ts
```

Expected: all commands exit 0 and neither renderer's resource effect includes
`props.active` in its dependency boundary.

- [ ] **Step 4: Commit renderer lifetime changes**

```powershell
git add web/default/src/features/yucore-brand/components/yucore-signal-field-webgl.tsx web/default/src/features/yucore-brand/components/yucore-webgl-earth.tsx web/default/src/features/yucore-brand/components/yucore-renderer-resource-key.ts
git commit -m "perf: reuse production webgl resources on activation"
```

### Task 3: Prewarm Persistent Home Renderers In Separate Stages

**Files:**
- Modify: `web/default/src/features/yucore-brand/components/yucore-background.tsx`
- Modify: `web/default/src/features/yucore-brand/components/yucore-home.tsx`

- [ ] **Step 1: Add an explicit preparation stage to the background**

Add:

```ts
export type YucoreBackgroundPreparation = 'none' | 'signal' | 'all'
```

and an optional `preparation` prop. Derive the backwards-compatible stage:

```ts
const active = props.active !== false
const preparation = props.preparation ?? (active ? 'all' : 'none')
const signalPrepared = preparation !== 'none'
const earthPrepared = preparation === 'all'
```

Mount `LazyYucoreSignalFieldWebgl` when `signalPrepared` and mount the Earth
container when `earthPrepared && props.showEarthCore !== false`. Pass the
unchanged `props.active` to both renderers. Keep the CSS/static mesh fallback
visible when `signalPrepared` is false.

- [ ] **Step 2: Make the home own staged preparation**

In `YucoreHome`, initialize:

```ts
const [backgroundPreparation, setBackgroundPreparation] =
  useState<YucoreBackgroundPreparation>('none')
```

Replace the 400 ms import-only timer with:

```ts
useEffect(
  () =>
    scheduleYucoreHomeRendererPrewarm(
      window,
      YUCORE_BOOT_LOADER_DURATION_MS,
      () => setBackgroundPreparation('signal'),
      () => setBackgroundPreparation('all')
    ),
  []
)
```

Pass `preparation={revealHero ? 'all' : backgroundPreparation}` to
`YucoreBackground`. The entrance loader and its visible signal/Earth sequence
remain unchanged.

- [ ] **Step 3: Verify the prewarm behavior**

Run:

```powershell
bun test src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts src/features/yucore-brand/components/yucore-render-loop.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-background.tsx src/features/yucore-brand/components/yucore-home.tsx src/features/yucore-brand/components/yucore-home-renderer-prewarm.ts
```

Expected: all commands exit 0.

- [ ] **Step 4: Commit staged preparation**

```powershell
git add web/default/src/features/yucore-brand/components/yucore-background.tsx web/default/src/features/yucore-brand/components/yucore-home.tsx web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.ts web/default/src/features/yucore-brand/components/yucore-home-renderer-prewarm.test.ts
git commit -m "perf: prewarm production home renderers in stages"
```

### Task 4: Split Below-The-Fold Details Across Frames

**Files:**
- Modify: `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-home-details.tsx`
- Modify: `web/default/src/features/yucore-brand/components/yucore-home.tsx`

- [ ] **Step 1: Write the failing post-commit scheduler tests**

Keep `scheduleYucoreHomeDetails` responsible for the primary idle/user-intent
reveal. Add `scheduleYucoreHomeSecondaryDetails`, which is called only after the
primary React component commits. Update the fake host tests as follows:

```ts
host.flushIdle()
assert.deepEqual(stages, ['primary'])
const disposeSecondary = scheduleYucoreHomeSecondaryDetails(
  host,
  () => stages.push('secondary')
)
assert.equal(host.pendingFrameCount(), 1)
host.flushFrame()
assert.deepEqual(stages, ['primary', 'secondary'])
disposeSecondary()
```

Keep user-intent-wins and cleanup cases, asserting each stage runs at most once.

- [ ] **Step 2: Run the scheduler test and verify RED**

```powershell
bun test src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
```

Expected: TypeScript/test failure because the post-commit secondary scheduler
does not exist.

- [ ] **Step 3: Implement staged scheduling**

Keep the existing primary scheduler semantics. Implement
`scheduleYucoreHomeSecondaryDetails` with one owned animation frame; its cleanup
cancels the frame, and its callback runs at most once.

- [ ] **Step 4: Split the detail component at a stable visual boundary**

Export `YucoreHomeDetailsPrimary` containing the metric strip, supported-model
route section, operations heading, and capability grid (current lines 128-235).
It accepts a stable `onCommitted` callback and invokes
`scheduleYucoreHomeSecondaryDetails(window, props.onCommitted)` from an effect;
therefore the secondary state cannot be set before the primary component has
committed, even when the lazy chunk is still loading.
Export `YucoreHomeDetailsSecondary` containing service advantages, access flow,
Studio entry, and enterprise section (current lines 237-388). Keep one outer
section per component with identical width and horizontal padding; give the
secondary section no extra top padding so the existing vertical rhythm is
unchanged.

In `YucoreHome`, replace `showHomeDetails` with `detailStage: 0 | 1 | 2`, lazy
load both named exports from the same module, and render primary at stage 1 and
secondary at stage 2. Pass only the primary setter to
`scheduleYucoreHomeDetails`, and pass a `useCallback`-stable secondary setter as
the primary component's `onCommitted` prop.

- [ ] **Step 5: Verify detail scheduling and frontend checks**

```powershell
bun test src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-home.tsx src/features/yucore-brand/components/yucore-home-details.tsx src/features/yucore-brand/components/yucore-home-details-scheduler.ts src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit detail staging**

```powershell
git add web/default/src/features/yucore-brand/components/yucore-home.tsx web/default/src/features/yucore-brand/components/yucore-home-details.tsx web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.ts web/default/src/features/yucore-brand/components/yucore-home-details-scheduler.test.ts
git commit -m "perf: stage production home detail commits"
```

### Task 5: Production-Build Trace And Visual Acceptance

**Files:**
- Modify: `docs/superpowers/perf/2026-07-25-local-baseline.md`

- [ ] **Step 1: Run complete frontend checks**

From `web/default`:

```powershell
bun test
bun run typecheck
bun run build
```

Run changed-file lint and formatter checks. Build `web/classic` with its
existing package script. Record whole-tree lint/format debt separately and do
not rewrite unrelated files.

- [ ] **Step 2: Start a temporary production preview**

Serve the accepted `dist` on unused loopback port `13001`, proxying `/api`,
`/mj`, and `/pg` to the existing local backend on `3000`. Do not stop ports
`3000`, `3001`, or the control on `13000`.

- [ ] **Step 3: Capture three serial control/candidate measurements**

For each run use Edge headless at 1440x900, wait for stable-home state, then
collect FCP, LCP, long tasks, frame average/p95, active canvases, and a CPU
profile. The trace must show no signal-field or Earth shader compile after
stable-home activation. Compare medians; do not claim improvement from one run.

- [ ] **Step 4: Verify visual parity**

Capture light and dark desktop screenshots and 390x844 mobile screenshots.
Assert both entrance and stable backgrounds are nonblank, the Earth and signal
field remain visible and moving, the first secondary detail block follows the
primary block without a spacing discontinuity, and no text or controls overlap.

- [ ] **Step 5: Stop only the temporary preview and record evidence**

Resolve the process listening on `13001`, verify its command line belongs to
the temporary preview, stop it, and confirm `3000`, `3001`, and `13000` remain
listening. Preserve JSON and screenshots; delete only temporary trace scripts.

- [ ] **Step 6: Update performance evidence and commit**

Append environment, commit, three-run medians, trace attribution, screenshots,
and any residual risk to `docs/superpowers/perf/2026-07-25-local-baseline.md`.
Run `git diff --check`, then:

```powershell
git add docs/superpowers/perf/2026-07-25-local-baseline.md
git commit -m "docs: verify production renderer handoff"
```
