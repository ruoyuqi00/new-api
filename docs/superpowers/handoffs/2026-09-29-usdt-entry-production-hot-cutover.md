# USDT Entry Production Hot Cutover

Date: 2026-09-29 (Asia/Shanghai)

## Release identity

- Source commit: `c1874d09d` on `codex/tokenpay-fixed-package-20260927`, pushed to the `fork` GitHub remote.
- Source archive SHA-256: `a0481ff41c3bda3e8471acdd4c4a3b0102ec07999970b8b5bfb02a5d50a334aa`.
- Image: `yuapi:production-usdt-ui-20260929-c1874d09d`.
- Image ID: `sha256:bc4d58b7c93475119922911d0f8832b05ce8ff03d3c0f1f1afadd23ce4f21247`.
- Binary SHA-256: `c2908f8e9dbcdef8516cd595c83e858af69fc83f3a4afd22eb1bda35b955361e`.
- Runtime version: `c1874d09d-usdt-ui-20260929`.
- Active YuAPI container: `yuapi-production-usdt-ui-c1874d09d`.
- Stopped rollback containers: `yuapi-production-tokenpay-cf19e2526` and `yuapi-usdt-ui-candidate-c1874d09d`.

The application change only enlarges and relabels the TokenPay wallet entry as
"USDT top-up". Redemption-code input and redemption-code purchase link logic
were not changed. Payment configuration, balances, channels, prices, Sub2API,
TokenPay, and other project containers were not deliberately changed.

## Verification and rollback

The targeted Bun unit test, desktop/mobile Playwright test, TypeScript typecheck,
changed-file lint and format checks, and default frontend production build
passed before the release. The source archive was transferred with a matching
SHA-256, then the image was built on the production host. The private candidate
was healthy, served `/wallet` with HTTP 200, and preserved HTTP 401 for an
unauthenticated top-up information request.

The final container is `running/healthy/0`. Both YuAPI reverse-proxy entries
in the persisted Caddyfile point to it. `api.yuaiapi.com`, `yuaiapi.com`, and
`vip.yuaiapi.com` returned the expected runtime version. The protected
containers retained their pre-cutover IDs, start times, and restart counts.
The old and candidate YuAPI containers were stopped only after their connection
counters reached zero for three consecutive checks. No unrelated container was
restarted or removed.

Backup and rollback script:

`/opt/newapi/backups/20260929-usdt-ui-c1874d09d/`

The compressed MySQL dump passed `gzip -t` and SHA-256 verification. Its hash is
`142bbb80c90618a68419371f3334429f61dfe96c3a689cee2a321327386cd354`.
The task-specific `hot-switch.sh rollback` phase is retained there. This UI
release did not add database migrations; an ordinary application rollback does
not require restoring the database dump.

## Cutover incident

During candidate drain, three zero-queue TCP connections were assumed idle and
closed individually. Two were actually pending, long-running `/v1/responses`
requests. Caddy returned HTTP 502 to one client at 2026-09-29 22:27:05 and
22:27:57 Asia/Shanghai. Zero TCP queue and a long interval since the last
packet did not prove that a request had finished. Do not use those signals to
force-close future candidate connections.

Both requests belonged to user ID 79. Their consume-log IDs are `43369834` and
`43370384`; each recorded 755 quota from local estimated usage, totaling 1510
quota (0.00302 site currency units at 500000 quota per unit). Their request IDs
are `202609291407013600279528268d9d6Q8bOxSSf` and
`202609291404557481245708268d9d6tHQ1j9fg`. No refund or balance adjustment
was made. Any correction should be linked to these logs and separately approved.

After 2026-09-29 22:29 Asia/Shanghai, the Caddy log check found no further
502/503/504 responses. The final application log check found no panic, fatal,
or migration error in the preceding five minutes.
