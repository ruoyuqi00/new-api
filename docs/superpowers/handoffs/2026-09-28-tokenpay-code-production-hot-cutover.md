# TokenPay Code Production Hot Cutover

Date: 2026-09-28 (Asia/Shanghai)

## Release identity

- Application source commit: `cf19e2526ff871408d06a155f3d57ac57ad23465`
- GitHub branch: `fork/codex/tokenpay-fixed-package-20260927`
- Image: `yuapi:production-tokenpay-code-20260928-cf19e2526`
- Image ID: `sha256:a59df85904f6bdddc8243447adb3acf39ee56a7da7ab60d1ff06bf1712fc3095`
- Binary SHA-256: `c4bdb5cc027d7472b2bd3e177da02c9c786ebeadbea69a9a642a8e37e2328700`
- Runtime version: `cf19e2526-tokenpay-code-20260928`
- Active YuAPI container: `yuapi-production-tokenpay-cf19e2526`
- Stopped rollback container: `yuapi-production-shkeeper-458514670`
- Stopped candidate retained for rollback: `yuapi-tokenpay-candidate-cf19e2526`

This switched only the YuAPI application image. The independent TokenPay service
was not deployed, and production still has zero `tokenpay_payment.*` options:
USDT collection remains disabled. The new TokenPay tables are additive. No
user balance, price, channel, or usage-log adjustment was made.

## Verification

- YuAPI: `go test -p 1 ./... -count=1` passed. The unrestricted parallel run
  first hit a Windows linker resource error while both frontend builds were
  running; it did not report a test assertion failure.
- Frontend: 271 targeted Bun unit tests, default and classic production builds,
  and default TypeScript typecheck passed.
- Candidate: exact source/image/binary identity, three new tables, disabled
  TokenPay settings, private HTTP routes, and unauthenticated 401 checks passed.
- Old YuAPI connections were allowed to reach zero before the old container
  stopped. Candidate connections also reached zero before it stopped.
- Final YuAPI is `running/healthy/0`; Caddy host and runtime configurations
  each route the two YuAPI sites to the final container. Three public domains
  returned the expected version. A post-cutover check found zero Caddy 502s
  and zero YuAPI fatal/migration errors in the most recent ten minutes.
- Sub2API, its PostgreSQL/Redis, YuAPI MySQL/Redis, Caddy, and the other
  protected project containers retained their IDs, start times, and restart
  counts across the cutover.

## Backup and rollback

Backup: `/opt/newapi/backups/20260928-tokenpay-code-cf19e2526/`

The directory contains the checked MySQL single-transaction dump, before/after
Caddy and container records, protected-container snapshots, restricted runtime
environment files, and `hot-switch.sh` with a `rollback` phase. The compressed
dump passed `gzip -t` and SHA-256 verification; its SHA-256 is
`0190ac5475144d448755610ab45c1c6048a0ed5a2dd7b6d4cb0c65db25fef362`.
The additive TokenPay tables do not require restoring the entire database for
an ordinary application rollback.

## Caddy incident and reload rule

The first candidate bridge briefly returned 502s, approximately five minutes,
despite the host Caddyfile being restored promptly. Caddy has a read-only,
single-file bind mount for `/etc/caddy/Caddyfile`; an earlier atomic host-file
replacement left the running container attached to an older inode that still
named `yuapi-production-protocol-routing-20260909`. Reloading that stale path
caused the 502s. The old version was restored using a validated copy in Caddy's
mounted `/config` directory, and all three public domains returned 200 before
the corrected bridge was retried.

The corrected switch script copies each Caddyfile revision into
`/opt/edge/caddy_config/tokenpay-active-20260928.Caddyfile` and reloads with:

```sh
docker exec yuapi-caddy caddy reload --config /config/tokenpay-active-20260928.Caddyfile --adapter caddyfile
```

Do **not** manually reload `/etc/caddy/Caddyfile` in the current Caddy
container: that bind-mounted inode is stale. The persisted host
`/opt/edge/Caddyfile` points to the final container and will be rebound on a
future Caddy container restart; a shared-proxy restart was intentionally not
performed during this YuAPI-only cutover.

## Remaining payment gate

The separate TokenPay fork is still local at `D:/tokenpay-yuapi` commit
`e7150f1`. Before enabling TRC20, BSC, or Polygon in production, each chain
requires its own small real transfer, confirmed scan, signed callback, exactly
one YuAPI credit, restart/recovery, and encrypted wallet-database backup
acceptance. No real wallet, RPC credential, or collection address was installed
as part of this cutover.
