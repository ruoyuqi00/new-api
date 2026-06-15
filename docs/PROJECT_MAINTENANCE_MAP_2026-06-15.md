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
- `docs/SUB2API_AND_COCKPIT_TOOLS_AUDIT_2026-05-27.md`
- `docs/UPSTREAM_PROTOCOL_AUDIT_2026-05-25.md`
- `docs/KIRO_UPSTREAM_AUDIT_2026-05-23.md`

## Workspace-wide reference folders

At `D:\wflogin`, these directories are present but are not all part of the main deployed stack:

- `sub2api-private/` is the current maintenance repo.
- `sub2api/` and `sub2api-fork/` are older checkout variants.
- `new-api/` is the sidecar/newapi reference project.
- `Kiro-Go/`, `kiro.rs-master/`, and `windsurf-relay-sanitized/` are reference implementations or extracted notes.
- `unified-ai-gateway/` is a separate sidecar/reference workspace.
- `*_src.tar`, `*.tar.gz`, `*.zip`, and `.tmp-*` files are build or investigation artifacts.

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
