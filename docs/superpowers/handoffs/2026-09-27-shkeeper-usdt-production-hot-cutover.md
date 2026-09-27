# SHKeeper USDT Production Hot Cutover

Date: 2026-09-27 (Asia/Shanghai)

## Release identity

- Source commit: `45851467000e11cf0e19a8aa54e49bcffea74634`
- Branch: `codex/image-api-resolution-routing-20260907-local`
- GitHub archive: `fork/codex/image-api-resolution-routing-20260907-local`
- Image: `yuapi:production-shkeeper-usdt-20260927-458514670`
- Image ID: `sha256:345624e6a26c9ae5497f0ab4dc6809e58bc386f5715dc4d7bae9d7978a73e107`
- Binary SHA-256: `34f890e9a9b32cd2f8f95fb4a8f38dd515f09c2bc5b299efa9e35b1b53915e38`
- Runtime version: `458514670-shkeeper-usdt-20260927`
- Active container: `yuapi-production-shkeeper-458514670`
- Rollback container: `yuapi-production-responses-budget-dc6f0bd01` (stopped and retained)
- Cutover time: `2026-09-27T01:06:21Z`

The release changed the YuAPI application image, added the three additive
SHKeeper tables, updated YuAPI's Caddy target, and retired the two task-owned
candidate/previous instance records. It did not restart or rewrite Sub2API,
YuAPI MySQL, YuAPI Redis, Caddy, other projects, users, balances, prices,
channels, or existing usage logs.

## Delivered behavior

- Administrators can configure SHKeeper with write-only API/backend keys,
  enabled networks, invoice expiry, reconciliation interval, and fixed USDT to
  site-balance packages.
- A package freezes its USDT amount, site balance, and quota at order creation.
  Partial payment credits zero; full, late-full, and overpayment credit the
  fixed entitlement once.
- Lookup transaction labels are observational only. New settlement requires a
  valid signed callback whose matching transaction has `trigger=true`.
- Transaction-hash recovery asks SHKeeper to rescan; it cannot directly mark an
  order paid.
- Credits, top-up state, quota cap enforcement, and affiliate rewards remain in
  one atomic settlement transaction.
- The default remains disabled. Production has zero `shkeeper_payment.*`
  options, so no live SHKeeper payment is offered until an administrator saves
  real provider settings.

## Merge and repository verification

The reviewed feature branch was fast-forwarded into the current production
line and pushed to GitHub before deployment. Fresh merged-result checks passed:

- `go test ./... -count=1` (exit 0);
- frontend unit scope `bun test tests scripts src` (263 pass, 0 fail);
- `bun run typecheck`;
- default and classic production builds;
- `git diff --check 27023295d..HEAD`;
- immutable YuCore renderer/style guards against `27023295d`.

Plain `bun test` still discovers two Playwright specifications as Bun unit
tests and reports the pre-existing runner mismatch. The explicit unit scope is
green; browser behavior was previously approved on the isolated local
candidate and is documented in the production-line verification handoff.

## Backup and rollback

Recovery artifacts are stored at:

`/opt/newapi/backups/20260927T005416Z-shkeeper-usdt-458514670/`

The directory includes the 2,545,286,766-byte single-transaction MySQL dump,
container/image metadata, old and new Caddy configurations, environment files
with mode `0600`, core-container identity snapshots, database verification,
logs, checksums, and an executable rollback script. The compressed database
backup passed `gzip -t`; its SHA-256 is
`d2342138e826ca80172275c1da45377c1156ade50fa41514933496ec96b14866`.

Normal application rollback runs `rollback.sh`. It starts and health-checks
the retained previous container, reloads the saved Caddy configuration, restores
the persisted Caddyfile, and only then stops the new container. It does not
restore the database dump because the added empty tables are backward-compatible.
The dump is retained for disaster recovery and must not be restored merely to
roll back application traffic.

The first backup attempt used a MySQL account without the `PROCESS` privilege
and was rejected while reading tablespace metadata. It occurred before any
candidate or routing change. That incomplete task-owned archive was stopped and
deleted, then the successful backup used `--no-tablespaces` while preserving
schema, data, routines, triggers, and transaction consistency.

## Cutover and verification

- The private candidate ran as a master for migrations but with the system task
  runner disabled. It passed status, page, network-origin, binary, and auth
  boundary checks before cutover.
- MySQL contains exactly `sh_keeper_top_up_orders`,
  `sh_keeper_credited_transactions`, and
  `sh_keeper_transaction_authorizations`; all three contain zero rows.
- The final container is `running/healthy/0`, has the expected image, binary,
  runtime version, master node identity, and uniquely designated system task
  runner.
- Caddy runtime and the persisted host Caddyfile each reference the new target
  twice and the old target zero times.
- Five repeated public samples for `api`, main, and `vip` returned success with
  the expected version. `/`, `/sign-in`, `/wallet`, `/docs`, `/pricing`, and
  `/system-settings/billing/payment` returned HTTP 200.
- Unauthenticated SHKeeper user and administrator routes returned HTTP 401,
  confirming the production router and auth boundary.
- The production frontend main asset contains the SHKeeper UI.
- Application fatal/migration errors, Caddy upstream errors, and YuAPI
  502/503/504 responses after cutover were all zero.
- The old container's established connections drained from 83 to zero before
  it was stopped. No fixed forced-stop deadline interrupted a stream.
- The local candidate was removed. `system_instances` contains the final YuAPI
  node and no candidate or retired YuAPI node from this cutover.
- Sub2API app/PostgreSQL/Redis, YuAPI MySQL/Redis, and Caddy retained the exact
  pre-cutover container IDs, start times, and restart counts.

No live SHKeeper API, blockchain payment, wallet transfer, or production user
credit was exercised. Real-provider/real-chain acceptance remains required
after the administrator supplies the provider URL and write-only credentials.
