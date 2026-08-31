# Stream And Image Reliability Hot Cutover

Date: 2026-08-31 (Asia/Shanghai)

## Release identity

- Source commit: `c723c5482`
- Branch: `codex/first-token-image-capability-20260831`
- Production image: `yuapi:production-20260831-stream-image-c723c5482-ui78330-v3`
- Production image ID: `sha256:300a24914591a5ce2f8c66bc91088038066566f599e9c47f2b92726b6b57309f`
- Active container: `newapi-stream-image-c723c5482`
- Rollback container: `newapi-image-selection-6e56d3d1c`
- Rollback image: `yuapi:production-20260829-6e56d3d1c`

The active container is a slave node and uses the existing production MySQL and
Redis environment. No database snapshot was restored and no balance, channel,
group, price, or user data was changed.

## Changes

- Bounded response-header and first-content waits for GPT/Claude text streams.
- Client-gone accepted streams continue a bounded terminal-usage drain when
  `STREAM_USAGE_DRAIN_ENABLED=true`.
- Non-square image requests fail closed for unknown or incompatible channel
  capabilities; square requests retain existing routing behavior.
- Channel advanced settings expose persisted image dimension capability with
  legacy `ratio` normalization.
- Local candidates are able to bind to an explicit loopback address through
  `LISTEN_ADDRESS`.

Billing formulas, task billing, channel affinity, violation fee handling,
actual-response-model recording, retry policy, and media stream handling were
not redesigned by this release.

## UI baseline

The production entry HTML and its referenced default-theme JS/CSS assets were
recovered from the running production response before the final image build.
The public and candidate homepage SHA-256 is:

`78330de2c18839c29f34305c3ff66d708911162ed331d4d5a9476f2cb8e5bb10`

The recovered assets were used only to reproduce the already-running UI and are
not a source of credentials or database data.

## Cutover evidence

- Preflight checks passed for the current application, Caddy, MySQL, Redis, and
  production network.
- MySQL consistency backup and Caddy runtime backup are retained under:
  `/opt/newapi/backups/20260831T-hotcutover-stream-image-c723c5482/`
- Caddy Admin JSON was loaded atomically; the persisted Caddyfile was updated
  only after runtime and public checks passed.
- `api.yuaiapi.com`, `global.yuaiapi.com`, and `vip.yuaiapi.com` returned HTTP
  200 in all six observation samples.
- Candidate health remained `running/healthy`, restart count `0`; the rollback
  container remained `running`, restart count `0`.
- Runtime Caddy target count was candidate `2`, old target `0`.
- Candidate and Caddy fatal/upstream connection error counts were `0` during
  the observation window.

## Rollback

Do not stop either application container first. Restore the saved
`Caddy.runtime-before.json` through the Caddy Admin API endpoint `/load`, then
restore `/opt/edge/Caddyfile` from `Caddyfile.container-before`. Verify the
three public status endpoints and homepage fingerprint before any cleanup.

Keep both application images, containers, and the MySQL backup until the
production observation window is explicitly closed.
