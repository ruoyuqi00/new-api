# Backend Affinity And Billing Hot Cutover Handoff

Date: 2026-08-21 (Asia/Shanghai)

## Release identity

- Source commit: `81c6b91d8820fb3cee72715e43ed8c2876981267`
- Source branch: `codex/backend-integration-20260821`
- Image: `yuapi:production-20260822-ui-baseline-81c6b91d8`
- Image ID: `sha256:93b150f5b0840c654b0fdbe586bbb50a9cf6700909e489a02abd3064209cabaa`
- Candidate container (active): `newapi-ui-baseline-20260822-rc1`
- Candidate private binding: `127.0.0.1:13045 -> 3000/tcp`
- Previous backend candidate (retained, not active): `newapi-backend-affinity-billing-20260821-rc2`
- Current production target: `newapi-ui-baseline-20260822-rc1`

The release source contains the accepted production UI lineage from
`d6605a79a` plus the committed stream recovery, GPT cache affinity, missing
usage settlement, log-load, sensitive-block billing, Codex prelude, and XAI
media adapter changes. The full Go suite passed on the exact release commit.

## Baseline recovery finding

Before this cutover, the host Caddyfile and the running Caddy container did not
represent the same file inode. The host path had already been changed to a
backend candidate, while the running container still exposed and used the old
`d6605a79a` target. This explains why rebuilding or reloading by looking only at
the host file could produce inconsistent UI and backend results.

The actual running Caddy configuration was captured from inside the container
before this cutover. Do not infer the live target from the host path alone.

## Candidate correction

The first 2026-08-21 backend container lacked the production `/data` mount and
had no Docker healthcheck. It was not sent production traffic. The `rc2`
container was created from the same verified image with:

- the candidate's existing server environment, without printing or storing its
  values in Git;
- `/opt/newapi/data:/data`;
- restart policy `unless-stopped`;
- a loopback-only private port;
- an HTTP `/api/status` healthcheck; and
- the existing application and Caddy release networks.

The temporary environment export was deleted immediately after container
creation.

## Caddy state and backups

The pre-cutover runtime Caddyfile and the validated candidate are retained in:

```text
/opt/newapi/backups/20260821T010903Z-backend-affinity-billing-3a7c7519d/
```

Relevant files:

- `Caddyfile.runtime-before`
- `Caddyfile.to-backend-rc2`

The candidate was briefly loaded and then rolled back after the public UI
fingerprint did not match the known production UI. The running Caddy process
was subsequently gracefully reloaded from:

```text
/config/Caddyfile.runtime-before
```

The host path and the in-container Caddy path previously diverged by inode.
Do not infer the live target from `/opt/edge/Caddyfile`; inspect the running
Caddy configuration before any future reload. The candidate configuration is
retained as `/config/Caddyfile.to-backend-rc2` and the rollback configuration as
`/config/Caddyfile.runtime-before`.

The old baseline is currently receiving production traffic. The candidate is
healthy but receives no traffic until its embedded UI assets are rebuilt from
the exact production artifact set and revalidated locally.

## Verification

After the graceful reload:

- Caddy runtime contained exactly two new targets and zero old targets.
- The active container was healthy with restart count zero.
- `api.yuaiapi.com`, `global.yuaiapi.com`, and `vip.yuaiapi.com` each returned
  20/20 successful server-side status checks.
- The same three domains each returned 10/10 successful independent external
  status checks.
- The public homepage SHA-256 exactly matched the private candidate homepage.
- Candidate panic/fatal count was zero.
- Candidate database/migration error count was zero.
- Caddy dial/name-resolution/connection-refused error count was zero.
- The previous UI baseline and all older rollback containers remained running.

No database snapshot was restored, no balance was rewritten, and no previous
container or image was removed.

## Current production state (2026-08-22)

- Caddy runtime target count: UI baseline candidate `2`, previous target `0`.
- UI baseline candidate remains healthy with restart count zero.
- Public `api.yuaiapi.com` and `global.yuaiapi.com` home responses match the
  retained old baseline fingerprint.
- No production database, balance, or user data was changed during this UI-only
  cutover.
- The candidate UI mismatch was treated as a build-artifact recovery issue. The
  active image was rebuilt with the recovered production `web/default/dist`
  artifact set; no frontend source or backend strategy was changed for this
  cutover.

## UI-baseline cutover (2026-08-22)

The recovered production UI archive was transferred separately from the source
tree and verified with SHA-256:

- UI archive: `1b0f21708da87215db99d3a088f5ce732c1ec088824b7c636e13cc99309b7b83`
- Source archive: `81c6b91d8820fb3cee72715e43ed8c2876981267`
- Pre-cutover runtime Caddy backup: `/opt/newapi/backups/20260822T-ui-baseline/Caddyfile.runtime-before-ui`
- Cutover Caddy file: `/opt/newapi/backups/20260822T-ui-baseline/Caddyfile.to-ui-baseline`

The candidate was first verified privately on `127.0.0.1:13045`. Its homepage
SHA-256 exactly matched the retained baseline (`3f7a57592f3be48734bad3b19bec98ded5b1980e699491da7ddb08424e7cdf48`), and the
old baseline asset fingerprints `index.134936448f.js` and
`index.16fc389747.css` were served successfully. Caddy was then gracefully
reloaded. The old containers and images remain available for rollback.

Post-cutover checks recorded 20 consecutive successful `/api/status` requests
for each of `api.yuaiapi.com`, `global.yuaiapi.com`, and `vip.yuaiapi.com`.
The active candidate remained healthy with restart count zero, and the public
homepage continued to return the baseline SHA-256.

## Rollback

Rollback changes only Caddy runtime routing and the persisted host Caddyfile.
Do not stop the new container first.

1. Confirm the retained pre-cutover target is reachable through
   `newapi-backend-affinity-billing-20260821-rc2:3000`.
2. Copy `/opt/newapi/backups/20260822T-ui-baseline/Caddyfile.runtime-before-ui`
   over `/opt/edge/Caddyfile`.
3. Validate `/config/Caddyfile.rollback-ui-baseline` inside `yuapi-caddy`.
4. Gracefully reload Caddy from `/config/Caddyfile.rollback-ui-baseline`.
5. Verify the three public status endpoints, homepage fingerprint, retained
   container state, and error counters.
6. Keep both release containers and both images until a later observation gate
   explicitly approves cleanup.
