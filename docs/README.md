# YuAPI Production Documentation Index

Current production entry:

- `YUAPI_SERVER_MIGRATION_STATUS_2026-07-11.md`
  - Current server/domain cutover state, verification evidence, backups,
    rollback posture, and the remaining Cloudflare Turnstile hostname task.
- `../BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`
  - Root-level project/remote baseline with the 2026-07-09 production update.
- `YUAPI_PRODUCTION_STATUS_2026-07-09.md`
  - Short current-state document for live YuAPI service, channel state,
    rollback notes, and the next production queue.

Migration and runtime details:

- `YUAPI_SERVER_DOMAIN_MIGRATION_RUNBOOK_2026-07-11.md`
  - Production runbook for moving YuAPI to a new server and Cloudflare-managed
    domain, including backup, staged cutover, verification, and rollback.
- `YUAPI_SUB2API_MINIMAL_MIGRATION_DRY_RUN_2026-07-07.md`
  - Full migration worksheet and operation log for Sub2API plus/pro account
    import, channel audit, load smoke, observation, and conservative Sub2API app
    retirement.
- `YUAPI_CHANNEL_POOL_RUNTIME_2026-07-07.md`
  - Minimal YuAPI channel-pool runtime design and deployment record.

Upstream audit:

- `YUAPI_UPSTREAM_NEWAPI_AUDIT_2026-07-09.md`
  - Selective audit of recent `QuantumNous/new-api` fixes/features. Use this
    before backporting upstream changes.

Historical context:

- `archive/HANDOFF_NEWAPI_YUCORE_STUDIO_WORKFLOW_2026-07-06.md`
- `archive/HANDOFF_YUCORE_MOTION_BRAND_SNAPSHOT_2026-07-07.md`

The archived handoff files are not current production entry points.

Operational rule of thumb:

- Do not merge `origin/main` wholesale into the YuAPI production branch.
- Backport specific upstream fixes in small batches with focused tests.
- Do not remove Sub2API Postgres/Redis/volumes until the remaining adapter
  paths are either migrated, explicitly retired, or documented as out of scope.
