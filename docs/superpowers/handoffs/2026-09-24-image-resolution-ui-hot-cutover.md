# Image Resolution Pricing UI Hot Cutover

Date: 2026-09-24 (Asia/Shanghai)

## Release identity

- Source commit: `f0205c45c`
- Branch: `codex/image-api-resolution-routing-20260907-local`
- Image: `yuapi:production-image-resolution-ui-20260924-f0205c45c`
- Image ID: `sha256:eebb0d5d58856b83fa09ef7330b3021925da9ed9be19f4b7487fa73fd7178c0a`
- Binary SHA-256: `7d05d1ba8fa4ea6f69ec25b7f537fd1a70d94cb9718f26b6d59e402df319a261`
- Runtime version: `f0205c45c-image-resolution-ui-20260924`
- Active container: `yuapi-production-protocol-routing-20260909`
- Previous image retained for rollback: `yuapi:production-mobile-keys-scroll-20260921-a49f49648`

The release replaces only the YuAPI application binary. It preserves the
stable container name and existing Caddy target. No channel, ability, group,
price option, user balance, usage log, MySQL, Redis, Sub2API, or unrelated
project configuration was changed.

## Changes

- Added the guided `Image resolution prices` administration tab.
- Added independent 1K, 2K, and 4K price editing with positive,
  non-decreasing validation.
- Added image-resolution starting prices and group-adjusted tier details to
  the model marketplace.
- Added the new UI strings to all six supported frontend locales.
- Kept canonical image-model billing and routing behavior unchanged.

## Recovery artifacts

Recovery artifacts are stored at:

`/opt/newapi/backups/20260923T172541Z-image-resolution-ui-f0205c45c/`

The directory contains the old and new image metadata, pre-release container
metadata, Caddy runtime and persisted configuration snapshots, hashed image
channel/ability/price-option snapshots, the post-release container metadata,
checksums, and an executable `rollback.sh`.

Normal rollback uses that `rollback.sh` to recreate the stable container from
the previous image and reload Caddy. It must not restore a database snapshot.

## Verification

- The active container is `running/healthy/0` on the expected image, binary
  hash, and runtime version.
- `api.yuaiapi.com`, `yuaiapi.com`, and `vip.yuaiapi.com` returned HTTP 200
  with the new runtime version in repeated samples.
- `/`, `/sign-in`, `/docs`, `/pricing`, and
  `/system-settings/billing/model-pricing` returned HTTP 200.
- The public `index.2ebaacc768.js` asset contains the image-resolution pricing
  UI.
- Caddy runtime and persisted configuration each reference the stable YuAPI
  container twice.
- Pre- and post-release image option, channel, and ability snapshots matched
  byte-for-byte.
- Candidate and retained temporary containers and their heartbeat rows were
  removed after verification.
- Sub2API application, PostgreSQL, Redis, YuAPI MySQL, and YuAPI Redis retained
  their existing container IDs/start times and remain `running/healthy/0`.
- After `2026-09-23T17:31:10Z`, application fatal errors, Caddy 502/503/504
  responses, and Caddy connection-resolution failures were all zero.

## Cutover observation

The prior stable container was given a 300-second graceful-stop interval.
Long-running requests still open when that interval expired were terminated at
approximately `2026-09-23T17:31:03Z`. Caddy recorded 169 one-time HTTP 502
responses in the `17:31:00Z` to `17:31:10Z` window. No further 5xx or upstream
connection failures were observed after that window, and all bounded public
checks remained healthy. The new release was retained to avoid causing a
second interruption through rollback.

Future streaming releases should drain the old container without a fixed
forced-stop deadline before removing it, while the parallel candidate serves
new connections.
