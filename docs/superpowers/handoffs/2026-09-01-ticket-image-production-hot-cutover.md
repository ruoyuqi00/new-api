# Ticket Center And Image Routing Production Hot Cutover

Date: 2026-09-01 (Asia/Shanghai)

## Release identity

- Source commit: `b36876d296aea903c1e8ef9fba829b89146a72ed`
- Branch: `codex/first-token-image-capability-20260831`
- Image: `yuapi:production-20260901-ticket-image-b36876d29`
- Image ID: `sha256:b583516f097bf545d950449a31b9847b36d24d4219b60af88df7ee490fb62a7b`
- Active container: `newapi-ticket-image-b36876d29`
- Rollback container: `newapi-image-pricing-0eb319d84`
- Rollback image: `yuapi:production-20260831-image-pricing-0eb319d84-ui78330`

The active and rollback containers both remain running. The release continues to
use the existing production MySQL, Redis, and slave-node environment. No
database snapshot was restored and no balance, usage log, price, group,
channel, or customer record was reset.

## Database migration

Slave application nodes intentionally skip schema migration. A one-purpose
GORM migrator created only `tickets`, `ticket_messages`, and
`ticket_attachments`, then validated all three tables. It did not start the
application as a master node and did not run background billing, settlement,
cleanup, or reconciliation tasks.

The refund ticket category is communication only. It does not credit balances,
create refund transactions, or mutate billing records.

## Recovery artifacts

Fresh recovery artifacts are retained under:

`/opt/newapi/backups/20260901T-ticket-image-b36876d29/`

They include the immediate Caddy runtime JSON, container and image metadata,
the container and host Caddyfiles, pricing and homepage snapshots, and a
validated single-transaction MySQL backup. The compressed database backup hash
was checked after creation.

Normal rollback must not restore the database backup because production
transactions continue after cutover. The three additive ticket tables may
remain unused if application traffic is rolled back.

## Verification

- The candidate reached `running/healthy/0` before receiving traffic.
- Candidate Playwright checks passed for home, sign-in, sign-up, documentation,
  and pricing without page errors, internal request failures, HTTP errors, or
  JavaScript responses served as HTML.
- The production YuCore globe login page was visually checked after cutover.
- The production Playwright suite passed the same five public pages after
  cutover.
- Price, ratio, endpoint, vendor, and group business fields for all 120 models
  matched the pre-cutover snapshot. The existing nondeterministic placement of
  a duplicate per-record `pricing_version` marker was excluded; the top-level
  version and all billing fields remained unchanged.
- User and admin ticket routes reached their authenticated `401` boundary when
  accessed anonymously.
- Six final observation samples returned HTTP 200 for `api`, `global`, and
  `vip`; the candidate remained `running/healthy/0` throughout.
- Candidate fatal, panic, and database-migration error count was zero.
- MySQL and Redis remained `running/healthy/0`; Caddy remained `running/0`.
- Caddy runtime and the persisted Caddyfile both contain the new target twice
  and no active old target.

## Rollback

Do not stop the active or rollback application container first. Load
`Caddy.runtime-before.json` from the backup directory through the Caddy Admin
`/load` endpoint, then restore `Caddyfile.host-before` to
`/opt/edge/Caddyfile`. Verify the three public status endpoints, the YuCore
login page, and the runtime target before any cleanup.

Keep both application containers, images, the source archive, and the backup
directory until the production observation window is explicitly closed.
