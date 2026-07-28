# Local Production Brand Performance Design

## Goal

Validate backend fixes and improve the production-brand UI locally without
changing the deployed UI or using production data.

## Baseline

- Source baseline: `codex/video-endpoint-compatibility-20260724` at
  `1c83672ce`, which contains the production brand UI.
- Backend-only additions: preserve mapped client model names and retain MySQL
  `LONGTEXT` media assets during migration.
- Local environment: isolated worktree, isolated database and Redis volumes,
  and local-only accounts for user and administrator flows.

## Constraints

- Do not replace the production brand shell, theme modes, or visual language.
- Keep the boot, background, and earth motion experiences. Improve their
  scheduling, visibility handling, rendering cost, and data flow instead.
- Do not connect the local application to production databases, Redis,
  channels, account pools, or API keys.
- Remove the obsolete experimental UI only after confirming it has no imports,
  routes, or build references.

## Approach

1. Establish a local Docker environment from the production-brand source and
   add deterministic user and super-admin fixtures.
2. Record browser performance evidence for public, user, and administrator
   pages before changes. Capture request waterfalls, long tasks, animation
   frame behavior, and bundle boundaries.
3. Fix measured bottlenecks in the brand animation runtime. Prefer pausing
   offscreen and hidden motion, using worker/offscreen rendering where
   supported, avoiding redundant paint layers, and preventing repeated React
   work in dashboard data views.
4. Compare the current upstream NewAPI UI selectively. Port statistics, usage,
   and list-display improvements only when they can use the brand layout and
   existing local i18n patterns.
5. Validate anonymous, user, and super-admin routes locally. The production
   UI must remain visually intact while interaction and page readiness improve.

## Deployment Gate

No production build or container change is allowed until the local preview has
been reviewed in all three roles and the performance trace shows no regression.
