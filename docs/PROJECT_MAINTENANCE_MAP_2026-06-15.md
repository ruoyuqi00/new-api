# Project maintenance map - 2026-06-15

## Mainline

These are the files and directories that should be treated as actively maintained for the deployed Sub2API stack:

- `backend/`
- `frontend/`
- `deploy/`
- `Dockerfile`
- `.dockerignore`
- `.gitignore`
- `README.md`
- `README_JA.md`
- `DEV_GUIDE.md`
- `CLA.md`

## Active reference / support code

These are kept because they support the current stack or act as operational references:

- `adapters/kiro-web/`
- `provider-patches/`
- `tools/`
- `assets/`
- `skills/`

## Historical research and planning

These directories are useful for context, but they are not the primary code path:

- `docs/`
- `planning/`

The current docs that matter most for maintenance are:

- `docs/UPSTREAM_MERGE_2026-06-10.md`
- `docs/UPSTREAM_PROTOCOL_CHECK_2026-06-15.md`
- `docs/SCHEDULER_CONCURRENCY_CACHE_2026-06-17.md`
- `docs/CHANNEL_GROUP_USER_MAPPING_RUNBOOK_2026-06-17.md` - how Sub2API channels map to groups, API keys, and NewAPI-fronted upstream channels.
- `docs/GPT_ONLY_NEWAPI_BRIDGE_2026-06-17.md` - current GPT-only NewAPI bridge layout and naming plan.
- `docs/RISK_CONTROL_DOWNSTREAM_GUARD_2026-06-17.md` - downstream request guard and key-level auto-disable behavior for large-scale expansion.
- `docs/SCALABILITY_RISK_AUTOMATION_2026-06-17.md` - current gap list and automation rules for scaling users safely.
- `docs/SUB2API_AND_COCKPIT_TOOLS_AUDIT_2026-05-27.md`
- `docs/UPSTREAM_PROTOCOL_AUDIT_2026-05-25.md`
- `docs/KIRO_UPSTREAM_AUDIT_2026-05-23.md`

## Workspace-wide reference folders

At `D:\wflogin`, these directories are present but are not all part of the main deployed stack:

- `sub2api-private/` is the current maintenance repo.
- `new-api/` is the sidecar/newapi reference project.
- `unified-ai-gateway/` is a separate sidecar/reference workspace.
- `openclaw/` and `tools/` are currently kept because Windows service `OpenClaw` is running through `D:\wflogin\tools\nssm\nssm-2.24\win64\nssm.exe`.
- `_archive_old_unused_20260615/` contains old checkouts, build artifacts, temp files, and notes moved out of the root workspace.
- `D:\wflogin\<registration-project>` is intentionally independent and was not inspected or moved. This refers to the Chinese-named folder requested by the user.

Older checkout variants and reference implementations such as `sub2api/`, `sub2api-fork/`, `Kiro-Go/`, `kiro.rs-master/`, and `windsurf-relay-sanitized/` were moved into the archive folder during the cleanup pass.

## Workspace cleanup performed

Cleanup date: 2026-06-15.

Archive root:

```text
D:\wflogin\_archive_old_unused_20260615
```

Nothing was deleted. Old or unused material was moved into subfolders:

- `artifacts/` - old tar, tar.gz, zip, and image bundles.
- `legacy-checkouts/` - older repository checkouts and reference workspaces.
- `uploads/` - old upload chunk directories.
- `temp/` - temporary response/debug folders.
- `sensitive/` - temporary files that may contain keys or local SQL snippets.
- `hidden-configs/` - old root-level `.claude` and `.cursor` folders.
- `runtime-scripts/` - old root-level `.runtime` scripts.
- `caches/`, `media/`, and `notes/` - cache, screenshot, and loose note files.

After cleanup, the root `D:\wflogin` intentionally contains only:

- `sub2api-private/`
- `new-api/`
- `unified-ai-gateway/`
- `openclaw/`
- `tools/`
- `_archive_old_unused_20260615/`
- `D:\wflogin\<registration-project>`

Skipped / retained:

- `openclaw/` was not moved because it appeared in use.
- `tools/` was not moved because `OpenClaw` service depends on `tools\nssm\nssm-2.24\win64\nssm.exe`.
- `D:\wflogin\<registration-project>` was not inspected and remains independent.

## Cleanup policy

Recommended handling:

1. Keep `sub2api-private/` as the only codebase treated as authoritative for the current server deployment.
2. Keep reference checkouts read-only unless a new protocol comparison is needed.
3. Move archive artifacts and `.tmp-*` files into an archive folder only after confirming they are no longer needed.
4. Do not delete research notes until a new condensed maintenance note is in place.
5. When adding new docs, prefer short dated records over growing one huge log.

## What is still hard to tell by filename alone

- Whether a root-level archive is still needed for rollback.
- Which of the old research docs are still valuable versus duplicated.
- Which reference folders should stay checked out locally and which can be compressed or moved away.

For those, a second pass should compare against recent commit history and active server deployment notes before any deletion.

## Suggested next pass

The safest cleanup order is:

1. Document the inventory.
2. Rename or tag the current authoritative folders.
3. Archive old tar/zip/temp files outside the active workspace.
4. Remove duplicated planning docs only after the useful facts are copied into one current note.
