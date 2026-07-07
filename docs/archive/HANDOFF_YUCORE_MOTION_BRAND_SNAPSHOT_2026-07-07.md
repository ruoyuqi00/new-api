# Archived: YuCore Motion / Brand Snapshot Handoff - 2026-07-07

Archive notice: this is historical YuCore brand/motion context only. It is not
the current production baseline. Start from
`BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md` for current remote,
production, and project-role decisions.

# YuCore Motion / Brand Snapshot Handoff - 2026-07-07

This document records the current YuCore motion/brand state before switching to the next feature-development window.

Important: this is a non-production snapshot. It should stay separate from production/release work until it is deliberately reviewed and merged.

## Branch

- Snapshot branch: `snapshot/yucore-motion-brand-20260707`
- Base branch at time of snapshot: `yuapi-upstream-rc15-merge-20260625`
- Purpose: preserve the current YuCore motion/brand implementation and related integration state for later follow-up.

## Current Decision

The motion/brand polish is paused here. The current state is good enough to preserve and move on to other feature work, but future brand polish can still return to:

- finer first-screen particle choreography
- center globe detail and brand signature effects
- additional frame-by-frame visual tuning against `https://ai.soulecho.cc/`
- Studio/Canvas workflow polish after feature work stabilizes

## Reference Site Provenance

Primary visual reference:

- URL: `https://ai.soulecho.cc/`
- Site title observed during QA: `SoulEcho AI 发电站⚡️`
- Reason for reference: the user explicitly pointed to it as the motion benchmark for continuous WebGL/particle choreography.
- How it was used: as a motion-language reference only, not as a product/content copy target. YuCore should preserve its own brand, palette, model-routing language, Studio/Canvas product shape, and Earth/network identity.

Reference frames captured locally:

- Directory: `output/motion-final-audit-20260707`
- `ref-0450.png`: dark CRT/noise field with a small rotating square
- `ref-1200.png`: centered `Soulecho AI Core` text emerging
- `ref-2200.png`: dense triangle/shard vortex around the title
- `ref-3600.png`: particles aggregating into a sphere above the title
- `ref-5200.png`: stable sphere with lower wave/flow lines and landing copy

Important interpretation:

- The useful lesson is continuity: the opening effect does not cut to an unrelated background. It evolves from the same particle field into a persistent scene.
- The useful layout pattern is staged density: sparse/dark start, dense middle burst, then calmer stable state.
- The useful motion pattern is mixed order: some particles are intentionally chaotic, while the sphere, lanes, packets, and lower flows are ordered.
- The YuCore implementation should not fully mimic SoulEcho. YuCore's equivalent should read as a model power-grid / route-fabric / media-workflow brand system.

Current YuCore comparison evidence:

- `output/motion-final-audit-20260707/local-home-0600.png`
- `output/motion-final-audit-20260707/local-home-1800.png`
- `output/motion-final-audit-20260707/local-home-3600.png`
- `output/motion-final-audit-20260707/local-home-6200.png`
- `output/motion-final-audit-20260707/local-home-stable.png`

Observed YuCore differences retained intentionally:

- YuCore reveals the Earth-like globe earlier than SoulEcho's final sphere.
- YuCore keeps cyan/amber route lines and grid overlays instead of SoulEcho's monochrome/pink sphere style.
- YuCore's stable state includes product navigation and operational copy, so particle density must leave text readable.
- YuCore's lower flow and route lanes are branded as model routing / power-grid continuity rather than just decorative waves.

## What Is Completed In This Motion Phase

### Reference And Local Motion Alignment

The reference site was re-captured in consecutive frames:

- dark/noisy field with a small rotating square
- shard/triangle vortex
- sphere formation
- stable sphere with lower flowing lines

Local YuCore now follows the same broad choreography while keeping YuCore-specific visual identity:

- early boot field: dark, sparse, ordered square/core
- mid boot field: shard vortex and particle aggregation
- globe build: dynamic Earth-like WebGL core with route/grid lines
- stable state: persistent globe, particles, mesh, energy lanes, route lines

### Global Route Coverage

The following local routes were verified with dynamic background layers:

- `/`
- `/sign-up`
- `/pricing`
- `/wallet`
- `/playground`
- `/playground/studio`
- `/playground/canvas`
- `/dashboard/overview`

Expected active layers in verified pages:

- `YucoreMotionCanvas`
- `YucorePersistentCore`
- `YucoreWebglEarth`
- `yucore-background-particle-mesh`
- Canvas route additionally has `yucore-canvas-motion-stage`, `yucore-canvas-particle-field`, and `yucore-canvas-agent-panel`

### Performance / Smoothness Fixes

- WebGL Earth and motion canvas frame pacing now target smooth 60Hz behavior instead of awkward 48/50/54fps caps that could visually stutter on 60Hz displays.
- Loader DOM particle/shard count was reduced while Canvas particle density was increased and structured, so visual density is preserved without the same main-thread load.
- High-refresh displays are still bounded to avoid wasteful render churn.
- Canvas Agent panel blur and darkness were reduced to avoid slab-like visual blocking.
- Workbench/auth pages now have content readability masks so dynamic particles do not overpower forms, cards, and input areas.

### Visual Identity Preserved

The implementation intentionally does not copy SoulEcho one-to-one. YuCore keeps:

- cyan/amber/rose energy palette
- real Earth-like globe shader feel
- model routing / power-grid language
- persistent orbital routes and packet lanes
- Studio/Canvas glass-energy workspace style

## Key Files

Core motion/brand components:

- `web/default/src/features/yucore-brand/components/yucore-boot-canvas.tsx`
- `web/default/src/features/yucore-brand/components/yucore-entrance-loader.tsx`
- `web/default/src/features/yucore-brand/components/yucore-webgl-earth.tsx`
- `web/default/src/features/yucore-brand/components/yucore-motion-canvas.tsx`
- `web/default/src/features/yucore-brand/components/yucore-persistent-core.tsx`
- `web/default/src/features/yucore-brand/components/yucore-background.tsx`
- `web/default/src/features/yucore-brand/components/yucore-home.tsx`
- `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`

Global styling:

- `web/default/src/styles/index.css`

Route/layout integration:

- `web/default/src/components/layout/components/authenticated-layout.tsx`
- `web/default/src/components/layout/components/public-layout.tsx`
- `web/default/src/features/auth/auth-layout.tsx`
- `web/default/src/features/pricing/index.tsx`
- `web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`
- `web/default/src/features/home/index.tsx`

Related existing handoff:

- `HANDOFF_NEWAPI_YUCORE_STUDIO_WORKFLOW_2026-07-06.md`

## Verification Evidence

Evidence directories generated locally during QA:

- `output/motion-final-audit-20260707`
- `output/motion-final-route-20260707`
- `output/motion-route-after-clear-20260707b`
- `output/motion-after-turn-20260707b`

These output directories were used as local QA evidence and are not intended to be committed.

Important screenshots from the final audit:

- `output/motion-final-audit-20260707/ref-0450.png`
- `output/motion-final-audit-20260707/ref-2200.png`
- `output/motion-final-audit-20260707/ref-3600.png`
- `output/motion-final-audit-20260707/ref-5200.png`
- `output/motion-final-audit-20260707/local-home-0600.png`
- `output/motion-final-audit-20260707/local-home-1800.png`
- `output/motion-final-audit-20260707/local-home-3600.png`
- `output/motion-final-audit-20260707/local-home-6200.png`
- `output/motion-final-audit-20260707/local-home-stable.png`

Final route audit facts:

- `output/motion-final-route-20260707/facts.json`

## Validation Commands Already Passed

Run from `D:\wflogin\new-api\web\default`:

```powershell
npm exec -- tsc -b --pretty false
npm exec -- rsbuild build
```

Run from `D:\wflogin\new-api`:

```powershell
git diff --check -- web/default/src/features/yucore-brand/components/yucore-background.tsx web/default/src/features/yucore-brand/components/yucore-webgl-earth.tsx web/default/src/features/yucore-brand/components/yucore-motion-canvas.tsx web/default/src/features/yucore-brand/components/yucore-boot-canvas.tsx web/default/src/features/yucore-brand/components/yucore-entrance-loader.tsx web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx web/default/src/styles/index.css
```

Residual QA process check returned:

```text
NO_QA_RESIDUALS
```

## Known Runtime Notes

- Some screenshots show `Request failed with status code 504` toasts. These are backend/API availability issues during local QA, not motion-layer failures.
- The old `D:\wflogin\image-site-v2` remains intentionally out of scope and should not be migrated into this line.
- QA output folders, browser profiles, logs, and SQLite snapshots should remain uncommitted unless a later task explicitly asks to archive test artifacts.
- The current implementation is intentionally a branch snapshot, not a production-ready release claim.

## Archived Suggested Next Window

The suggestion below was correct for the earlier YuCore feature window, but it
is not the current production-function baseline. For current production work,
use `ruoyu/main` as described in
`BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`.

For feature work, continue from this snapshot branch or create a new feature branch from it.

Recommended next feature target:

- Infinite Canvas / Studio functional alignment
- Review `basketikun/infinite-canvas.git` as the reference for infinite-canvas behavior
- Connect user-side Canvas to YuCore APIs and current Agent backflow
- Keep the current YuCore brand background as the visual shell, but prioritize working feature completeness

Suggested resume prompt:

```text
Continue from branch snapshot/yucore-motion-brand-20260707. Motion/brand polish is paused and documented in HANDOFF_YUCORE_MOTION_BRAND_SNAPSHOT_2026-07-07.md. Do not mix this with production. Start the next functional phase: finish YuCore Studio/Canvas infinite-canvas alignment, using basketikun/infinite-canvas.git as behavioral reference, and connect it to YuCore APIs/Agent backflow while preserving the current YuCore visual shell.
```
