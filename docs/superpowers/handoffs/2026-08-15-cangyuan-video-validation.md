# Cangyuan Video Refresh Validation Handoff

Date: 2026-08-15

Branch: `codex/cangyuan-video-refresh-20260815`

Implementation base before the final authenticated re-audit: `78ca6e12e`

This handoff is intentionally redacted. It contains no credentials, API keys,
cookies, real user identifiers, provider task identifiers, account balances,
private asset URLs, server addresses, container names, or image digests.

## Scope and release state

- The embedded video catalog contains exactly 13 enabled, priced models and
  seven hidden probe models.
- Video task billing remains isolated from GPT token usage, cache accounting,
  and stream interruption behavior.
- No production configuration, channel, container, image, Caddy route, or user
  traffic was changed during the validation and candidate-preparation phase.
- The current production application must remain running throughout candidate
  preparation, traffic switch, observation, and any rollback.
- The user subsequently approved the production traffic switch and explicitly
  accepted exposing all 13 models while the three recorded upstream capacity
  failures remain unresolved.
- The production cutover completed on 2026-08-16 without stopping or restarting
  the retained production application.

## Audited inventory

Enabled and priced:

```text
grok-video
grok-video-1.5
happyhouse-1.0
happyhouse-1.1
minimax-h3-2k
omni-fast
omni-fast-no-water
omni-v2v
omni-v2v-no-water
sd7-seedance-2.0-1080p
sd7-seedance-2.0-720p
sd8-seedance-2.0
seedance-2.0
```

Hidden probes without an approved price or transport contract:

```text
sd4-seedance-2.0
sd4-seedance-2.0-fast
sd8-seedance-2.0-fast
seedance-2.0-fast
seedance-2.0-mini
seedance-2.0-mini-8s
veo-clean
```

The enabled-model upstream subtotal for one minimum accepted task per model is
`31.973`. The seven probe models must not receive paid requests.

## Authenticated upstream re-audit

Immediately before paid probing, the authenticated VIDEO token and public
pricing surface were read again without writing or displaying credentials. The
upstream had changed after the initial implementation:

- The VIDEO token listed 21 model identifiers: 20 video models plus one
  out-of-scope audio model.
- Public VIDEO-group pricing listed 14 rows: 13 video models plus the same
  out-of-scope audio model.
- `sd4-seedance-2.0` and `sd4-seedance-2.0-fast` remained token-visible but no
  longer had a published price, so they moved from enabled to hidden probe.
- `seedance-2.0` became token-visible and priced at `3.9` per generation, with
  fixed 720p output, 4-15 second duration, generated audio, and limits of five
  images, three videos, three audios, and 11 references total.
- The retained 12 priced video rows had no price drift.
- The upstream VIDEO group ratio remained `1.0`.

The paid batch was stopped before any creation request or debit, as required by
the inventory-drift gate. Tests, catalog, pricing, docs, runbook, and local
candidate were then refreshed to the 13/7 baseline before paid work resumed.

## Pricing evidence

Base prices use `ceil(upstream_cost * 1.20 * 10000) / 10000`, ensuring every
base price is at least 20% above the observed upstream price.

| Model                    | Upstream | Base price | `下游多模态` 1.0 | `多模态创作` 1.2 |
| ------------------------ | -------: | ---------: | ---------------: | ---------------: |
| `grok-video`             |     0.69 |      0.828 |            0.828 |           0.9936 |
| `grok-video-1.5`         |     1.39 |      1.668 |            1.668 |           2.0016 |
| `happyhouse-1.0`         |      4.5 |        5.4 |              5.4 |             6.48 |
| `happyhouse-1.1`         |      2.9 |       3.48 |             3.48 |            4.176 |
| `minimax-h3-2k`          |      3.5 |        4.2 |              4.2 |             5.04 |
| `omni-fast`              |   0.6624 |     0.7949 |           0.7949 |          0.95388 |
| `omni-fast-no-water`     |     0.81 |      0.972 |            0.972 |           1.1664 |
| `omni-v2v`               |   0.8856 |     1.0628 |           1.0628 |          1.27536 |
| `omni-v2v-no-water`      |    1.035 |      1.242 |            1.242 |           1.4904 |
| `sd7-seedance-2.0-1080p` |      4.9 |       5.88 |             5.88 |            7.056 |
| `sd7-seedance-2.0-720p`  |      3.9 |       4.68 |             4.68 |            5.616 |
| `sd8-seedance-2.0`       |      2.9 |       3.48 |             3.48 |            4.176 |
| `seedance-2.0`           |      3.9 |       4.68 |             4.68 |            5.616 |

All values are per accepted generation. Status polling, content retrieval, and
asset download must not charge again. The exact runtime configuration is in
`docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md`.

## Automated verification

Backend formatting and tests:

```text
gofmt on all touched Go files                                      PASS
go test ./model ./service ./controller ./relay ./constant -count=1 PASS
go test -p 1 ./... -count=1                                        PASS
```

Default frontend checks:

```text
bun run typecheck                                                   PASS
bun run build                                                       PASS
bunx oxlint on the three touched TypeScript/TSX files               PASS
bunx oxfmt --check on all touched docs, TS/TSX, and locale files    PASS
bun run i18n:sync                                                   PASS
```

Classic frontend build:

```text
bun run build                                                       PASS
```

Repository-wide frontend lint baselines are not clean on the base branch. The
default full lint emits existing findings outside the touched files; its full
format check likewise reports existing unrelated files. The classic full lint
reports 122 existing source/generated-file style failures. These baseline
findings were not changed or suppressed. The touched default files pass the
targeted lint and format checks above, and both frontend production builds pass.

Documentation checks:

- The 13 base and final price values match the audited formula exactly.
- All seven fenced JSON examples and six JSON request bodies parse.
- Removed full video model identifiers are absent from the refreshed developer
  guide and production runbook.
- A generic credential/address scan found no secret-like value in the refreshed
  documentation.
- The optional operator-local private-pattern scan was not run because
  `YUAPI_PRIVATE_PATTERN_FILE` was not configured in the local environment. It
  remains a required pre-release server check.

## Local private-candidate evidence

An isolated local application was built from this branch and started at
`localhost:31845` with a disposable SQLite database, a synthetic local user,
and a fake channel exposing the 13 enabled models. It used no production or
upstream credential and sent no generation request.

Playwright exercised home, sign-in, console, pricing, Studio, Canvas, and
developer docs at desktop and mobile sizes. Results:

```text
page errors                                                        0
same-origin failed responses                                       1
expected anonymous /api/user/auth/refresh response                 401
desktop horizontal overflow                                        none
mobile horizontal overflow                                         none
Studio model buttons                                                13
Pricing model badges                                                13
720p control                                                        present
1080p control                                                       present
Native audio switch                                                 checked
Native audio switch accessibility                                  PASS
```

Screenshot files retained outside Git:

```text
home-desktop.png
sign-in-desktop.png
console-desktop.png
pricing-desktop.png
studio-video-desktop.png
studio-video-mobile.png
canvas-mobile.png
docs-desktop.png
```

Visual inspection confirmed that the video model list, per-generation prices,
resolution controls, duration controls, audio switch, action area, and results
area render without overlap at the tested desktop and mobile viewports.

## Real upstream paid-probe evidence

On 2026-08-16 (Asia/Shanghai), the authenticated VIDEO inventory, public
prices, group ratio, token-visible probe inventory, and public reference assets
were rechecked before any creation request. Each accepted task used exactly one
creation POST followed by same-ID GET polling and one content download. The
seven unpriced probe models received no paid requests.

| Model                    | Create/result                                                   | Status path                        | Verified media          | Billing evidence                                                                                     |
| ------------------------ | --------------------------------------------------------------- | ---------------------------------- | ----------------------- | ---------------------------------------------------------------------------------------------------- |
| `grok-video`             | Explicit capacity rejection; no task ID                         | HTTP 429                           | None                    | Three bounded client attempts; zero-amount error logs and zero balance delta                         |
| `grok-video-1.5`         | Explicit capacity rejection; no task ID                         | HTTP 429                           | None                    | Two bounded client attempts; zero-amount error logs and no charge                                    |
| `happyhouse-1.0`         | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 3.163 s  | Final consume log `4.5`; the initially observed partial balance settlement reconciled to this amount |
| `happyhouse-1.1`         | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 3.163 s  | Balance delta and consume log `2.9`                                                                  |
| `minimax-h3-2k`          | Completed                                                       | queued -> in progress -> completed | MP4, 2560x1440, 5.167 s | Balance delta and consume log `3.5`                                                                  |
| `omni-fast`              | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 10.005 s | Single-model balance delta and consume log `0.6624`                                                  |
| `omni-fast-no-water`     | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 10.005 s | Balance delta and consume log `0.81`                                                                 |
| `omni-v2v`               | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 5.013 s  | Balance delta and consume log `0.8856`                                                               |
| `omni-v2v-no-water`      | Completed                                                       | queued -> in progress -> completed | MP4, 1280x720, 5.013 s  | Balance delta `1.034998` versus price `1.035`; two-millionths quota-conversion rounding              |
| `sd7-seedance-2.0-1080p` | Completed                                                       | queued -> in progress -> completed | MP4, 1902x1092, 4.042 s | Balance delta and consume log `4.9`                                                                  |
| `sd7-seedance-2.0-720p`  | Completed                                                       | queued -> in progress -> completed | MP4, 1268x728, 4.042 s  | Balance delta and consume log `3.9`                                                                  |
| `sd8-seedance-2.0`       | Accepted, then failed because no provider account was available | pending -> queued -> failed        | None                    | Two bounded tasks; each consumed `2.9`, refunded `2.9`, and had net `0`                              |
| `seedance-2.0`           | Completed                                                       | queued -> in progress -> completed | MP4, 1268x728, 4.042 s  | Balance delta `3.9`; upstream consume log used its internal 720p route name                          |

The ten completed models consumed `26.992998` in total versus published cost
`26.993`; the difference is the same two-millionths quota-conversion rounding
on `omni-v2v-no-water`. Polling and content retrieval produced no additional
charges. The two Grok rejections and the failed `sd8-seedance-2.0` task had zero
net cost.

The observed 1080p and 720p outputs were close to, but not exactly, standard
1920x1080 and 1280x720 pixel dimensions. Public documentation therefore keeps
these values as provider model tiers and does not promise exact output pixels.
The capacity failures do not indicate a request-mapping defect, but they remain
release evidence gaps. A cooldown retry produced the same result, so those
three models require one successful bounded probe after the provider restores
capacity; no further immediate retry is authorized.

## Production candidate evidence

The fully pushed commit `923edbb9b8110dc6f75bff146ea6c0dd62cedba9` was built
server-side into a uniquely tagged image. The image revision label matched the
source commit. A final candidate was started on a unique localhost-only port and
release-network alias with task polling, batch quota updates, automatic channel
updates, and model-update tasks disabled. It remained healthy with restart count
zero while the existing production application remained healthy with restart
count zero.

Production preparation changed no public Caddy route and performed no Caddy
reload. The active Caddyfile retained exactly two references to the existing
application and zero references to the candidate. Caddy reached the candidate
over the isolated network, and both the active and staged full configurations
validated successfully.

Five replacement channels and 26 two-group ability rows were staged. All five
channels have disabled status and all 26 abilities have `enabled=0`. A first
staging transaction used enabled ability rows and was corrected immediately
after readback showed that disabled channels alone do not prevent pricing
exposure. After the correction and cache refresh, both existing and candidate
pricing surfaces returned to the exact pre-stage inventory. No replacement
channel was routable.

A protected server-local rollback artifact records the four legacy channel
rows, their abilities, all 24 affected price values or absence markers, the
complete relevant option rows, the existing task-price patch, and the active
Caddyfile. Directories are mode `0700`, files are mode `0600`, and checksums
verify. The cutover database transaction, the staged Caddyfile, and the complete
database rollback were each exercised against production structure without
commit; all dry runs ended in `ROLLBACK` and post-run readback matched the
preparation baseline.

Anonymous Playwright validation through a private SSH tunnel covered home,
sign-in, pricing, and developer docs on desktop and mobile. All pages rendered
without horizontal overflow or page errors. The only failed same-origin calls
were the expected anonymous auth-refresh responses. The candidate served the
new video documentation and new static asset fingerprints without changing the
production assets.

Authenticated candidate image and GPT smoke requests were not run because no
synthetic production credential was available and active users' keys or balances
must not be borrowed for release testing.

## Production cutover evidence

The user explicitly accepted the known upstream capacity risk and approved all
13 models for production. The final cutover began at 2026-08-16 09:05:59
(Asia/Shanghai). The deployed image was built from
`923edbb9b8110dc6f75bff146ea6c0dd62cedba9`; the final candidate used the exact
13-model `TASK_PRICE_PATCH` and remained a slave with task polling, batch quota
updates, automatic channel updates, and upstream model-update tasks disabled.

The last preflight rejected an earlier candidate because its environment still
contained the retired 15-model task-price patch. A corrected candidate was
started and verified before any production traffic changed. Caddy's runtime
readback also exposed an older read-only bind-mount inode that differed from the
current host pathname. Guarded cutover attempts automatically restored both the
database and route when this mismatch and a later verification-command quoting
error were detected. Neither attempt produced a 502 or stopped an application.

The durable switch moved the existing stable frontend alias to the verified
candidate and restored the same stable-alias Caddyfile across runtime, container
mount, and host persistence. Caddy was gracefully reloaded during validation
but never restarted; its start time and restart count were unchanged. The
retained production application remained running and healthy with restart count
zero and is still available as the immediate rollback target.

Committed database and public readback:

```text
replacement channels active                                      5/5
replacement abilities active                                   26/26
legacy video channels disabled                                    4/4
legacy video abilities active                                       0
target ModelPrice entries                                          13
canceled ModelPrice entries                                         0
exact target base prices                                             PASS
obsolete video capability key                                        0
group ratios 1.2 / 1.0                                              PASS
public pricing rows / target / canceled                       95 / 13 / 0
```

The retained application, candidate, and production Caddy path returned the
same 13 prices and both enabled groups. Production served the refreshed
developer guide and the new static fingerprints
`index.8580691911.css` / `index.52ddaa4d5e.js`.

Six post-cutover samples over more than five minutes, followed by three samples
after the stable-alias handoff, all reported HTTP 200, healthy applications,
restart count zero, zero Caddy 502 responses, zero database/Redis errors, and
zero fatal candidate errors. Server-local checksummed rollback artifacts now
cover both the scoped database restore and a no-stop stable-alias handback.

## Administrator sample asset local verification

On 2026-08-16, the private sample import workflow was exercised against an
isolated application, disposable database, disposable accounts, and ten
one-second synthetic MP4 fixtures. These fixtures verify product behavior only;
they are not provider outputs and must never be imported into production.

The first sequential import created 10/10 completed, zero-cost tasks. A second
import returned the same ten task identities with `created=false`, proving
idempotency. Quota remained unchanged. Full-file and Range responses matched
all ten source checksums.

Permission checks covered an administrator, an ordinary user, and a temporary
administrator demotion. The administrator could list, inspect, stream, and
download all ten samples. The ordinary user and demoted owner received no
sample list entries and could not read sample detail or content. Restoring the
administrator role restored access without rewriting any managed file.

Browser verification covered desktop and mobile viewports. Both rendered ten
playable videos with no media errors, page errors, or horizontal overflow. The
desktop flow downloaded an MP4 with the expected public filename, placed the
sample on Canvas, played it there, removed only the Canvas node, and then still
showed all ten gallery samples.

That browser run exposed and then verified a native-media authentication fix in
commit `a262a341f`. Native `<video>` and download requests cannot attach the
dashboard Authorization header. Media reads now use a distinct 15-minute,
HttpOnly, Secure-in-production, SameSite=Strict cookie scoped to task assets.
The cookie is accepted only for `GET` and `HEAD` asset reads, remains bound to a
live dashboard session, and cannot authenticate as a dashboard bearer token.
Existing ownership, current-role, and sample mutation guards remain unchanged.

The manifest rollback removed 10/10 sample tasks and 10/10 managed copies. The
ten source hashes remained unchanged and an unrelated ordinary upload remained
present. After rollback, the isolated task and gallery counts were both zero.

Fresh verification after the authentication fix:

```text
focused service/middleware/router/controller tests                 PASS
go test -p 1 ./... -count=1                                        PASS
frontend docs/gallery/locale tests                         13/13 PASS
operator importer tests                                      5/5 PASS
operator importer assertions                               66/66 PASS
three-edition documentation check                                PASS
TypeScript typecheck                                               PASS
default frontend production build                                 PASS
git diff --check                                                   PASS
```

The original retained provider videos could not be recovered from local
operator storage or the production host. Stored historical object references
no longer returned media. No paid task was repeated, no production sample was
imported, and no production route or runtime was changed during this local
verification. Production sample import remains blocked until the original ten
files are supplied or a separate approval explicitly authorizes regenerating
the ten already completed paid tasks.

After the local commits were pushed, a production read-only audit was repeated
at 2026-08-16 17:26 (Asia/Shanghai). The current application and retained
rollback application were both healthy with restart count zero. Caddy retained
its original start time and restart count zero; its persisted configuration
validated successfully. The primary and global status endpoints both returned
HTTP 200, and the public fingerprints remained
`index.8580691911.css` / `index.52ddaa4d5e.js`. The prior 30 minutes contained
zero Caddy 502 entries and zero application fatal, database, or Redis error
entries. Production contained zero managed sample tasks and had 24% root-disk
use. This audit made no server change. Commit `a262a341f` has been pushed but
had not yet been built or staged at the time of that read-only audit.

## Media-asset candidate after sample scope reduction

The user subsequently narrowed the release scope to a working video chain and
explicitly declined regenerating or importing the ten provider samples. No paid
generation request was sent, no sample file was imported, and the production
managed-sample count remained zero.

Commit `d6605a79a0ccb34c1d89e982f82b8b10058e8c53` combines the native-media
authorization fix with a documentation correction that makes all six tutorial
screenshots use explicit site paths. It was pushed before the server build. A
uniquely tagged image was built from that commit, and its revision label matched
the full source commit. The exact image and container identifiers remain only
in the protected server-local release artifact.

The final private candidate uses a loopback-only port and unique network aliases
with no stable or public alias. It remained healthy with restart count zero.
The current live application, retained rollback application, and Caddy also
remained healthy with restart count zero. Caddy reached the candidate directly
through its unique alias, but no Caddy route, reload, stable alias, or public
traffic was changed.

Private verification results:

```text
candidate API status                                               200
desktop/mobile documentation browser tests                         2/2 PASS
tutorial screenshot failures                                            0
three-edition documentation contract                               PASS
public pricing rows / target / canceled                       95 / 13 / 0
exact target prices and both enabled groups                        PASS
anonymous administrator import                                      401
anonymous task-asset read                                            401
managed production sample tasks                                       0
candidate fatal/database/Redis errors                                 0
Caddy-to-candidate private request                                 PASS
```

An initial candidate was discarded before public traffic because starting it
on the isolated release network before the internal service network caused
seven database-name-resolution restarts. The corrected candidate starts on the
internal network first, then attaches the isolated network; it has restart count
zero. The discarded candidate never owned a production alias.

The protected release directory is mode `0700`; its redacted container
inspection, image identifier, active Caddy baseline, network mapping, build log,
scope declarations, and verification summary are mode `0600`. Every recorded
checksum verifies. The sample-scope record states that import is disabled by the
user and both sample counts are zero.

After candidate verification, the primary and global public status endpoints
still returned HTTP 200. Public fingerprints remained
`index.8580691911.css` / `index.52ddaa4d5e.js`, and the preceding 30 minutes
contained zero Caddy 502 responses. The candidate is ready for the final
no-stop traffic guard. At this checkpoint, traffic switching had not yet been
approved; the subsequent cutover is recorded below.

## Media-asset hot-cutover evidence

The user explicitly approved the no-stop switch. The final guard confirmed the
candidate image and revision, candidate/current/retained application health,
restart counts, Caddy start time and configuration checksum, database and Redis
health, disk capacity, candidate pricing, documentation assets, unique-alias
reachability, zero managed sample tasks, and a validated rollback configuration.

The switch began at 2026-08-16 18:33:55 and was fully verified at 18:48:08
(Asia/Shanghai). Caddy first received a graceful runtime reload that pointed
directly to the new candidate's unique alias. After public checks passed, the
stable aliases were attached to the new candidate, removed from the previous
live application, and the unchanged canonical Caddyfile was gracefully
reloaded. Caddy was never restarted. This two-phase sequence kept a reachable
upstream throughout alias ownership changes.

The previous live application was never stopped or restarted. It remains
healthy on its unique internal aliases with restart count zero. A validated
rollback configuration can route traffic directly to it before restoring the
stable aliases, so rollback does not depend on stopping the new application.
The older retained rollback application also remains healthy and preserved.

Final public checks:

```text
primary/global status                                             200 / 200
public fingerprints                 index.134936448f.js / index.16fc389747.css
home/sign-in/keys/pricing/Studio/Canvas/docs routes                 7/7 PASS
desktop/mobile documentation browser tests                         2/2 PASS
documentation editions and tutorial images                         3/3, 6/6
public pricing rows / target / canceled                       95 / 13 / 0
exact target prices and both enabled groups                        PASS
anonymous task-asset read                                            401
managed production sample tasks                                       0
paid generation requests                                               0
production sample imports                                              0
```

Eleven observation samples ran from 18:39:09 through 18:44:39, spanning more
than five minutes. Every sample reported primary and global HTTP 200, healthy
new and previous applications, application restart count zero, Caddy restart
count zero, zero Caddy 502 responses, and zero candidate fatal, database, or
Redis errors. A final desktop/mobile browser run also completed without failed
documentation assets, page errors, or horizontal overflow.

The protected server-local release artifact now contains the canonical Caddy
baseline, validated cutover and rollback configurations, redacted pre/post
container state, post-cutover network ownership, runtime Caddy state, build
evidence, the executable rollback operation, and the final verification
summary. The directory remains mode `0700`; evidence files are mode `0600`, the
rollback script is mode `0700`, and every recorded checksum verifies.

## Remaining constraints

1. `grok-video`, `grok-video-1.5`, and `sd8-seedance-2.0` remain exposed under
   the user's explicit risk acceptance even though their real upstream probes
   ended in capacity failures. Re-probe only after the provider reports restored
   capacity; do not repeat the ten completed paid probes.
2. Authenticated production media and GPT smoke requests remain unexecuted
   because no dedicated synthetic credential was available. The authenticated
   media flow is covered by isolated browser verification. Never borrow an
   active user's key, balance, session, or assets.
3. Keep the retained production application, legacy channel rows, old image,
   stable-alias rollback script, and protected rollback artifacts until a
   separate cleanup approval is given.
4. Production samples are deliberately out of scope. Do not generate, import,
   or delete provider samples unless a later request explicitly reopens that
   work. The empty sample collection does not block the media-chain cutover.
