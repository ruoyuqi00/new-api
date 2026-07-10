# Portable Workspace Baseline - 2026-07-10

This document is the handoff for moving YuAPI / YuCore work to another
computer.

Use it as the first checklist after cloning the repository elsewhere.

## Canonical Remote

Use the user-owned remote as the working remote:

```text
ruoyu = https://github.com/ruoyuqi00/sub2api-provider-adapters.git
```

The upstream reference remote is:

```text
origin = https://github.com/QuantumNous/new-api.git
```

`origin/main` is upstream NewAPI reference only. Do not use local `main` as the
YuAPI production or UI continuation line.

2026-07-10 remote-entry cleanup:

- `ruoyu/main` is now the clean default YuAPI production backend entry.
- `ruoyu/main` points at the same commit as
  `ruoyu/feature/yuapi-channel-pool-runtime-20260707`.
- The named feature branch is retained as a pinned backend phase line.

## Current Fixed Backend Baseline

Production backend work is stable through Phase 22.

```text
branch: feature/yuapi-channel-pool-runtime-20260707
remote branch: ruoyu/feature/yuapi-channel-pool-runtime-20260707
default clone branch: ruoyu/main
current repo baseline commit: 52354ce1f docs: record portable workspace baseline
latest deployed code commit: 0809480bc fix: expose embedding endpoint metadata
latest deploy record: ee44b5e26 docs: record phase 22 production deploy
portable UI handoff base commit: 3b2072a94 docs: add yucore ui next window handoff
```

Production server state:

```text
newapi image: newapi:channel-pool-runtime-20260710-0809480bc
newapi: healthy
newapi-mysql: healthy
newapi-redis: healthy
```

Do not change production data, account pools, channel-pool scheduling, channel
priorities, billing formulas, or live channel settings from UI work.

## Current UI Continuation Branch

The local UI stash has been preserved as a real remote branch so it can be used
from another computer.

```text
branch: feature/yucore-ui-polish-20260710
remote branch: ruoyu/feature/yucore-ui-polish-20260710
wip commit: ace6a3ee6 wip: preserve yucore motion canvas cleanup
base: 3b2072a94 docs: add yucore ui next window handoff
```

This branch includes the previously local UI WIP from:

```text
stash@{0}: wip: phase 20 yucore motion canvas lint cleanup
```

Only the UI file was carried forward:

```text
web/default/src/features/yucore-brand/components/yucore-motion-canvas.tsx
```

The stale phased-plan doc change from the stash was intentionally not carried
into the UI branch because the Phase 21/22 production docs are newer.

## Branch Map

Use these branches deliberately:

| Branch | Remote | Purpose | Use Now |
| --- | --- | --- | --- |
| `main` | `ruoyu` | Clean default YuAPI production backend baseline for new clones | Yes, default backend entry |
| `feature/yuapi-channel-pool-runtime-20260707` | `ruoyu` | Pinned YuAPI backend phase branch, same head as `ruoyu/main` on 2026-07-10 | Yes, for backend fixes |
| `feature/yucore-ui-polish-20260710` | `ruoyu` | Next YuCore UI / Studio / Canvas continuation branch | Yes, for UI work |
| `snapshot/yucore-motion-brand-20260707-ruoyu` | `ruoyu` | Historical YuCore brand snapshot | Reference only |
| `media-frontend-nova-replacement-20260622` | `ruoyu` | Parallel historical media/protocol fixes; audit selectively only | Reference only |
| `docs/uag-newapi-image2-handoff-20260609` | `ruoyu` | Old UAG image2 handoff/archive line | Reference only |
| `main` | `origin` | Upstream QuantumNous/new-api reference | Reference only |

Local-only worktrees on this machine, not required on a new computer:

```text
D:\wflogin\new-api
D:\wflogin\new-api-ruoyu-push
D:\wflogin\yuapi-production-channel-pool
D:\wflogin\yucore-ui-polish-20260710
```

The new computer should recreate only the worktrees it needs.

## New Computer Setup

Clone the user-owned repo:

```bash
git clone -o ruoyu https://github.com/ruoyuqi00/sub2api-provider-adapters.git yuapi
cd yuapi
git remote add origin https://github.com/QuantumNous/new-api.git
git fetch --all --prune
```

Expected default checkout after clone:

```bash
git status --short --branch
# ## main...ruoyu/main
```

If the repository was cloned without `-o ruoyu`, verify and rename remotes
before adding the upstream reference:

```bash
git remote -v
git remote rename origin ruoyu
git remote add origin https://github.com/QuantumNous/new-api.git
git fetch --all --prune
```

Backend baseline checkout is already `main` after a clean clone. To work on the
named backend phase branch instead:

```bash
git checkout -B feature/yuapi-channel-pool-runtime-20260707 \
  ruoyu/feature/yuapi-channel-pool-runtime-20260707
```

UI continuation checkout:

```bash
git checkout -B feature/yucore-ui-polish-20260710 \
  ruoyu/feature/yucore-ui-polish-20260710
```

Recommended if working on both backend and UI on the new computer:

```bash
git worktree add ../yuapi-backend \
  ruoyu/main
git worktree add ../yucore-ui \
  ruoyu/feature/yucore-ui-polish-20260710
```

## UI Next Window

Use this UI entry document:

```text
docs/YUCORE_UI_NEXT_WINDOW_HANDOFF_2026-07-10.md
```

UI work should start from:

```text
feature/yucore-ui-polish-20260710
```

Goal:

- Finish the YuCore Studio / Canvas workflow into a usable product surface.
- Preserve the current motion/brand direction.
- Verify desktop/mobile layout.
- Run targeted frontend checks.

Do not deploy UI work to production until a separate UI readiness pass says it
is complete enough.

## Backend Next Phase

Backend staged work may continue from:

```text
feature/yuapi-channel-pool-runtime-20260707
```

The next documented backend candidate is Phase 23:

```text
openai-video endpoint metadata and channel-test fail-fast
```

Keep that separate from UI work.

## Important Docs

- `BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`
- `docs/YUAPI_PRODUCTION_STATUS_2026-07-09.md`
- `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`
- `docs/YUCORE_UI_NEXT_WINDOW_HANDOFF_2026-07-10.md`
- `docs/PORTABLE_WORKSPACE_BASELINE_2026-07-10.md`

## Do Not Do

- Do not push a local `main` that tracks upstream `origin/main` to
  `ruoyu/main`. On a clean clone made with `git clone -o ruoyu`, local `main`
  should track `ruoyu/main`.
- Do not merge UI WIP into the backend production branch by accident.
- Do not use the local stash as the only source of UI work; the portable source
  is now `ruoyu/feature/yucore-ui-polish-20260710`.
- Do not modify production databases or volumes while preparing the new UI
  workspace.
- Do not treat archived handoff files as current entry points.
