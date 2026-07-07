# Project / Remote / Production Baseline - 2026-07-07

This is the current canonical baseline for the next work window.

Use this document first. Older YuCore handoff files are now archived under
`docs/archive/` and should be treated only as historical evidence.

## Executive Baseline

- The production feature line is `ruoyu/main` in
  `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`.
- The YuCore WebGL / brand UI work is preserved as a non-production snapshot.
- The local `main` branch is not the production feature line. It tracks
  `origin/main`, where `origin` is `QuantumNous/new-api`.
- Do not merge the YuCore snapshot into production until production features
  are stable and a deliberate UI/brand replacement plan exists.
- The current deploy compose on the Sub2API production line still points at the
  upstream image `weishaw/sub2api:latest`. Custom production deployment needs a
  ruoyu-owned image such as `ghcr.io/ruoyuqi00/sub2api:<version>`.

## Remote Map

### `origin`

- URL: `https://github.com/QuantumNous/new-api.git`
- Role: upstream / new-api reference line.
- Default branch locally: `origin/main`.
- Current observed `origin/main`: `12603a77 fix(redemption): add status filtering and cleanup action`
- Local `main`: `00d23abf`, tracks `origin/main`, and is behind upstream.
- Production status: not the ruoyu production branch. Do not use local `main`
  for ruoyu production work.

### `ruoyu`

- URL: `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`
- Role: user-owned project remote and current production-function baseline.
- Default branch: `main`
- Current observed branches:

```text
main                                        4cf70f0d docs: record nova media frontend poc
docs/uag-newapi-image2-handoff-20260609     6c6c5557
media-frontend-nova-replacement-20260622    03ac14a7
snapshot/yucore-motion-brand-20260707-ruoyu 1c77b9b0
```

## Project Roles

### Reference Motion Site

- URL: `https://ai.soulecho.cc/`
- Observed title: `SoulEcho AI`
- Role: visual/motion reference only.
- Important lesson: continuous particle/WebGL choreography from boot state to
  persistent scene.
- Not a copy target, not a codebase to merge.

### Reference Infinite Canvas Project

- GitHub: `https://github.com/basketikun/infinite-canvas.git`
- Role: behavior/reference source for future infinite-canvas product work.
- Use it for canvas interaction expectations, not as an automatic full
  replacement unless a later implementation plan explicitly says so.

### Main Production Project

- Remote: `ruoyu`
- Branch: `ruoyu/main`
- Commit: `4cf70f0d`
- Product: Sub2API production feature line.
- Stack observed from repo:
  - backend: Go / Gin / Ent
  - frontend: Vue 3 / Vite
  - data: PostgreSQL + Redis
  - deployment: Docker Compose, Caddy, systemd helpers under `deploy/`
- This is the line to use for near-term production feature adjustments.

### Brand UI Upgrade Project

- Local branch: `snapshot/yucore-motion-brand-20260707`
- Local commit: `834c9eba chore: snapshot yucore motion brand state`
- Remote snapshot branch:
  `ruoyu/snapshot/yucore-motion-brand-20260707-ruoyu`
- Remote snapshot commit:
  `1c77b9b0 chore: snapshot yucore motion brand state for ruoyu remote`
- Role: preserve YuCore WebGL / brand / Studio / Canvas direction for later
  UI replacement.
- Production status: not production-ready and not merged into `ruoyu/main`.

### Sub2API Production Version

- Baseline branch: `ruoyu/main`
- Current deploy compose:
  `deploy/docker-compose.local.yml`
- Default runtime image in compose:
  `weishaw/sub2api:latest`
- Local services in compose:
  - `sub2api`
  - `postgres`
  - `redis`
- Default port mapping: `${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080`
- Release behavior:
  - CI runs on push / pull request.
  - Release runs on `v*` tags or manual workflow dispatch.
  - Simple release image target:
    `ghcr.io/ruoyuqi00/sub2api:<version>` and
    `ghcr.io/ruoyuqi00/sub2api:latest`.

Important production implication:

Changing `ruoyu/main` is not enough to update a server that is still pulling
`weishaw/sub2api:latest`. Production deployment must either build a local image
or switch compose/systemd deployment to a ruoyu-owned image.

### NewAPI Production / Upstream Version

- Remote: `origin`
- Branch: `origin/main`
- Observed commit: `12603a77`
- Product: QuantumNous new-api upstream line.
- Local status: local `main` tracks this line and is behind it.
- Role in this workspace: upstream/new-api reference and source of the YuCore
  brand snapshot lineage.
- Production status for the user's current plan: not the ruoyu Sub2API
  production baseline unless the user explicitly decides to replace production
  with a new-api-based product later.

## Current Local Worktrees

```text
D:\wflogin\new-api
  branch: snapshot/yucore-motion-brand-20260707
  purpose: YuCore / new-api snapshot workspace

D:\wflogin\new-api-ruoyu-push
  branch: snapshot/yucore-motion-brand-20260707-ruoyu
  purpose: pushed ruoyu snapshot branch workspace
```

Do not use either as the next production feature workspace.

## Recommended Next Production Workspace

Create a clean worktree from `ruoyu/main`:

```powershell
git worktree add -B prod/function-adjust-20260707 D:\wflogin\sub2api-prod ruoyu/main
```

Then work from:

```text
D:\wflogin\sub2api-prod
```

This keeps production feature work separate from the YuCore brand snapshot.

## Required Production Confirmation Before Deploy

This document confirms the repository and deployment configuration. It does
not confirm the live server's currently running image, because no production
server shell/container state was inspected in this thread.

Before deploying any production change, verify on the server:

```bash
docker ps
docker inspect sub2api --format '{{.Config.Image}}'
docker compose -f docker-compose.local.yml config | grep image
```

If the server image is still `weishaw/sub2api:latest`, switching Git branches
alone will not deploy custom changes.

## Do Not Do

- Do not push local `main` to `ruoyu/main`; local `main` is the `origin/main`
  new-api line.
- Do not treat the YuCore snapshot as production.
- Do not merge `snapshot/yucore-motion-brand-20260707-ruoyu` into `ruoyu/main`
  until production feature work is stable and UI replacement is explicitly
  planned.
- Do not use archived handoff documents as the current entry point.
- Do not migrate the deprecated `D:\wflogin\image-site-v2` spike into the
  production line.

## Archived Handoff Files

These files are historical context only:

- `docs/archive/HANDOFF_NEWAPI_YUCORE_STUDIO_WORKFLOW_2026-07-06.md`
- `docs/archive/HANDOFF_YUCORE_MOTION_BRAND_SNAPSHOT_2026-07-07.md`

Use them only when returning to YuCore Studio/Canvas or brand-motion polish.

