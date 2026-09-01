# Ticket Center Locale Follow-up Hot Cutover

Date: 2026-09-01 (Asia/Shanghai)

## Release identity

- Source commit used for the image: `4ec5943a778c8eefa5edd3168084f052abd81590`
- Branch: `codex/first-token-image-capability-20260831`
- Active image: `yuapi:production-20260901-ticket-image-4ec5943a7`
- Active container: `newapi-ticket-image-4ec5943a7`
- Rollback image: `yuapi:production-20260901-ticket-image-b36876d29`
- Rollback container: `newapi-ticket-image-b36876d29`

The image was built by replacing the binary in the previously accepted
production image. The binary was cross-compiled with the same `greenteagc`
experiment used by the production Docker build. The release contains the
ticket-center locale follow-up; no billing, channel, affinity, violation-fee,
database, or customer-data logic was changed.

## Cutover

- Candidate was started privately on `127.0.0.1:3114` with the existing
  production environment and slave-node settings.
- Candidate reached `running/healthy` with restart count `0`.
- Caddy runtime was loaded atomically, replacing exactly two old targets with
  two new targets. The persisted host Caddyfile was updated after the public
  checks passed.
- The previous container remains running without traffic as the rollback
  target; no production container or image was deleted.
- Recovery artifacts are retained under
  `/opt/newapi/backups/20260901T-hotcutover-ticket-image-4ec5943a7/`.

## Verification

- Candidate `/api/status`, `/`, `/tickets`, and `/admin-tickets` returned HTTP
  200 on the private port.
- Public `api`, `global`, and `vip` status endpoints, homepage, and pricing
  endpoint returned HTTP 200 in three consecutive samples.
- Anonymous `/api/tickets` returned the expected HTTP 401 boundary.
- Caddy runtime contains the new target twice and the old target zero times;
  the persisted Caddyfile has the same counts.
- Active candidate remained healthy with restart count `0`; the rollback
  container remained running with restart count `0`.
- Candidate logs showed zero `fatal`, `panic`, or database-migration-start
  messages during post-cutover observation.
- No database migration, snapshot restore, balance update, usage-log cleanup,
  pricing change, or channel configuration change was performed.

## Rollback

Restore `Caddy.runtime-before.json` through the Caddy Admin `/load` endpoint,
restore `Caddyfile.before` to `/opt/edge/Caddyfile`, and verify the three public
status endpoints before stopping either application container. Do not restore a
database snapshot for this release.
