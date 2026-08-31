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

## Post-cutover rollback

The release was rolled back after users reported frontend 500 pages while
navigating to lazy-loaded routes such as sign-in, wallet, and usage logs.
Runtime evidence showed that the candidate embedded the recovered production
entry HTML and root JS/CSS but did not contain two matching lazy-loaded JS
chunks. Requests for those chunks returned the SPA HTML fallback with HTTP 200
and `text/html`, which bypassed HTTP 5xx monitoring but failed in the browser.

Caddy runtime and the persisted host Caddyfile were restored to:

- Active container: `newapi-image-selection-6e56d3d1c`
- Active image: `yuapi:production-20260829-6e56d3d1c`

After rollback, the three public status endpoints passed three consecutive
samples, the affected chunk URLs returned `200 text/javascript`, and both the
active rollback container and retained candidate had restart count `0`. The
candidate remains private and must not receive production traffic until its
complete frontend asset graph, rather than only entry assets, matches the
production baseline.

## Official image resolution pricing candidate

Source and scope:

- Source commit: `ce4ecdc0d`
- Branch: `codex/first-token-image-capability-20260831`
- Local binary SHA-256: `fec2aedc8f87d6ccd161ec45ee564d3d16d829fcf1a71a789eb691abe920f390`
- Loopback candidate: `http://127.0.0.1:13015`
- Database: independent disposable SQLite; no production database or Redis
  connection is used.
- Production traffic, Caddy, production containers, production options, user
  balances, channels, and logs were not modified.

The candidate implements price selection only:

- official-model 1K/2K/4K base price policies;
- smallest-square-boundary dimension classification;
- legacy alias minimum tiers;
- pre-consume and settlement price freezing;
- atomic option validation and replacement;
- optional `/api/pricing` policy metadata;
- non-sensitive consume-log audit metadata.

Channel selection, upstream model mapping, upstream image parameters, task
billing, billing expressions, affinity, violation fees, actual response model,
retry, and media stream behavior remain on their existing paths.

Verification evidence:

- `go test ./... -count=1` passed.
- Explicit billing-expression, tiered billing, task billing, affinity,
  violation-fee, actual-response-model, image, and pricing regressions passed.
- Go race tests could not start because the local Windows Go toolchain has CGO
  disabled; this is a toolchain limitation, not a test failure.
- The production runtime chunk map was parsed from the recovered production
  entry bundle. All 119 entry and lazy-loaded assets were fetched read-only
  from the public production domain, validated as JavaScript/CSS where
  applicable, and embedded in the local candidate.
- All 119 candidate assets matched their production SHA-256 values. The two
  previously missing chunks, `3395.7bb002d7bb.js` and `531.c823517b31.js`,
  now return JavaScript with their production hashes.
- Playwright verified homepage, sign-in, sign-up, docs, dashboard, API keys,
  usage logs, wallet, system settings, and infinite canvas with no page error,
  request failure, HTTP error, or chunk-load failure.
- Screenshots confirmed the default YuCore brand UI, blue-marble globe login,
  custom navigation, API-key layout, system settings, and infinite canvas.

No paid image request was sent. The local pricing simulation produced:

| Model and request | Tier | Base price | Count | Group | Quota |
| --- | --- | ---: | ---: | ---: | ---: |
| `gpt-image-2`, `650x1024` | 1K | 0.01 | 1 | 1.0 | 5,000 |
| `gpt-image-2`, `1024x1536` | 2K | 0.04 | 1 | 0.3 | 6,000 |
| `gpt-image-2-4k`, `1024x1024` | 4K floor | 0.045 | 2 | 0.3 | 13,500 |
| `nano-banana-pro`, `2048x3072` | 4K | 0.161416666667 | 1 | 1.0 | 80,708 |
| `nano-banana2`, `auto` | default 1K | 0.063916666667 | 1 | 1.0 | 31,958 |

`4097x512` was rejected before billing, and an unmanaged image model retained
the legacy pricing path. The candidate is ready for user UI review only; no
production rollout is authorized by this record.

## Official image resolution pricing production rollout

The user approved the loopback UI candidate before production changes.

An initial cutover of commit `da138b00f` passed availability, UI asset, and
billing-field checks, but the before/after pricing audit found that
`gpt-5.3-codex-spark` changed its display vendor from OpenAI to 讯飞. Traffic
was immediately restored to `newapi-image-selection-6e56d3d1c` through the
saved Caddy runtime JSON. The rollback passed all three public status checks,
the homepage fingerprint check, and retained both containers with restart
count zero.

The root cause was an existing unordered Go map used for default vendor
matching: the model name contains both `gpt` and `spark`, so process startup
could select either rule. Commit `0eb319d84` replaces the unordered matcher
with explicit deterministic precedence and adds a regression test. The full
Go suite passed before rebuilding.

Final active release:

- Source commit: `0eb319d8438a9807c561b2009ae59783d90897a6`
- Image: `yuapi:production-20260831-image-pricing-0eb319d84-ui78330`
- Image ID: `sha256:067a9467ba9caf21b20fe708d678658ccf42f8231dabf7f7d45b3a7a01efa85a`
- Active container: `newapi-image-pricing-0eb319d84`
- Active state: `running/healthy/0`
- Rollback container: `newapi-image-selection-6e56d3d1c`
- Rollback state: `running/0`

Fresh pre-cutover recovery artifacts are retained under:

`/opt/newapi/backups/20260831T-image-pricing-0eb319d84/`

They include the immediate Caddy runtime JSON, persisted Caddyfile, old
container/image metadata, current pricing snapshot, and a validated MySQL
single-transaction backup. No database snapshot was restored, no balances or
logs were reset, and no production price option was rewritten.

Final verification:

- The production candidate matched all 119 production UI asset SHA-256 values.
- Homepage SHA-256 remains
  `78330de2c18839c29f34305c3ff66d708911162ed331d4d5a9476f2cb8e5bb10`.
- The previously missing `3395.7bb002d7bb.js` and
  `531.c823517b31.js` chunks return JavaScript.
- Current old/new snapshots have identical model sets, existing billing
  fields, group data, endpoints, and vendor assignments. Nine models expose
  the new optional image-resolution pricing metadata.
- Six observation samples returned HTTP 200 for `api`, `global`, and `vip`.
- Public read-only Playwright checks passed for sign-in, sign-up, and docs.
- MySQL and Redis remain `running/healthy/0`; Caddy remains `running/0`.
- Caddy runtime and persisted Caddyfile both contain the new target twice and
  the old target zero times.
- Candidate fatal/panic/database-migration error count is zero.

The old production image/container, both rejected/private candidates, both new
release images, and both backup directories remain retained. No cleanup was
performed so rollback remains available while production is being tested.
