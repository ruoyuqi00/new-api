# YuCore UI Next Window Handoff - 2026-07-10

This is the current entry point for the next YuCore UI / brand / Studio /
Canvas work window.

Use this only when intentionally returning to UI work. It is not a production
backend deployment plan.

## Current Decision

YuAPI production backend work is stable at the end of Phase 22. The next UI
window can focus on making YuCore's interface complete and polished without
mixing in backend strategy, account-pool scheduling, channel-pool behavior, or
production data changes.

## Do Not Mix With Production Backend

- Do not treat YuCore UI work as part of the current production backend phases.
- Do not change production account pools, channel priorities, scheduler
  behavior, billing formulas, live channel settings, or production data from the
  UI window.
- Do not deploy UI work to production until a separate UI readiness pass says
  it is complete enough.
- Keep backend protocol/strategy fixes on the staged YuAPI production flow.

## Current Production Backend Baseline

Production currently runs:

```text
newapi image: newapi:channel-pool-runtime-20260710-0809480bc
latest deployed code commit: 0809480bc fix: expose embedding endpoint metadata
latest docs/deploy record: ee44b5e26 docs: record phase 22 production deploy
branch: feature/yuapi-channel-pool-runtime-20260707
remote: ruoyu/feature/yuapi-channel-pool-runtime-20260707
```

Relevant backend status docs:

- `BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`
- `docs/YUAPI_PRODUCTION_STATUS_2026-07-09.md`
- `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`

## Current UI State

The unfinished UI work is intentionally paused, but it has been preserved in a
portable remote branch.

Use this branch for the next UI window:

```text
feature/yucore-ui-polish-20260710
remote: ruoyu/feature/yucore-ui-polish-20260710
wip commit: ace6a3ee6 wip: preserve yucore motion canvas cleanup
```

The original local stash still exists on this machine:

```text
stash@{0}: On feature/yuapi-channel-pool-runtime-20260707: wip: phase 20 yucore motion canvas lint cleanup
```

Do not depend on that stash when moving computers. It is local-only. The
portable source is the remote UI branch above.

If you are still on this original machine, you can inspect the old stash for
historical comparison:

```bash
git stash show --stat stash@{0}
git stash show --patch stash@{0}
```

The stale phased-plan doc change from the stash was intentionally not carried
into the UI branch because Phase 21/22 production docs are newer. The UI branch
preserves the actual UI WIP file:

```text
web/default/src/features/yucore-brand/components/yucore-motion-canvas.tsx
```

## Recommended UI Workspace

Preferred approach:

1. Check out `ruoyu/feature/yucore-ui-polish-20260710`.
2. Continue from the preserved `yucore-motion-canvas.tsx` WIP.
3. Keep UI commits separate from backend production fix phases.
4. Run frontend checks before considering any deployment.

Suggested branch name:

```text
feature/yucore-ui-polish-20260710
```

Historical YuCore snapshot context remains archived:

- `docs/archive/HANDOFF_NEWAPI_YUCORE_STUDIO_WORKFLOW_2026-07-06.md`
- `docs/archive/HANDOFF_YUCORE_MOTION_BRAND_SNAPSHOT_2026-07-07.md`

Treat those as historical reference, not as the main current entry point.

## UI Goal For The Next Window

Make the YuCore UI feel complete enough to review as a product surface:

- Finish the YuCore Studio / Canvas experience so it is usable, not only a
  static visual shell.
- Preserve the YuCore motion/brand direction, but prioritize working flows,
  clear navigation, responsive layout, and readable states.
- Continue the lint/format cleanup only where it directly supports stable UI
  work.
- Avoid broad visual rewrites unless they improve actual product completeness.
- Keep the first screen as the usable YuCore experience, not a marketing
  landing page.

## Suggested UI Work Order

1. Inspect the current UI route map and YuCore feature entry points.
2. Inspect the stashed `yucore-motion-canvas` work and decide whether to apply
   it.
3. Finish remaining lint/format debt in the active YuCore files only if it
   blocks typecheck or safe iteration.
4. Complete Studio / Canvas workflow basics:
   - create/open canvas;
   - add/edit nodes or media items;
   - persist enough state for a useful session;
   - connect visible UI actions to real or clearly mocked data paths;
   - handle empty, loading, error, and completed states.
5. Verify desktop and mobile layouts with screenshots.
6. Run frontend checks:
   - `bun run typecheck`
   - targeted `oxlint`
   - targeted `oxfmt --check`

## UI Acceptance Checks

Minimum before calling the UI window complete:

- YuCore route loads without TypeScript errors.
- Main Studio / Canvas workflow is interactable end to end.
- No obvious text overlap or broken responsive layout on mobile and desktop.
- UI checks pass for touched files.
- Backend production service and data are not modified as part of the UI work.

## Prompt For A New UI Conversation

Use this prompt to start the next UI-focused window:

```text
Continue YuCore UI / Studio / Canvas work from
D:\wflogin\new-api-ruoyu-push using
docs/YUCORE_UI_NEXT_WINDOW_HANDOFF_2026-07-10.md as the current entry point.

Important boundaries:
- This is UI-only work.
- Do not touch production backend strategy, account pools, channel-pool
  scheduling, billing formulas, live channel settings, production data, or
  server deployment.
- The current production backend is already deployed through Phase 22 and should
  stay stable.
- Start from remote branch ruoyu/feature/yucore-ui-polish-20260710, which
  preserves the previous yucore-motion-canvas WIP.

Goal:
Make the YuCore UI feel complete enough to review as a real product surface:
finish the Studio / Canvas workflow, preserve the motion/brand direction,
verify responsive layouts, and run targeted frontend checks.
```
