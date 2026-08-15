# Cangyuan Video Catalog and Pricing Refresh Design

> Status: proposed for review
>
> Audited: 2026-08-15 (Asia/Shanghai)
>
> Source baseline: `f042e4aad379ec65d0b6f2c689e1e21ecf90967a`

## 1. Goal

Refresh only the Cangyuan VIDEO-group integration from the provider's current
authenticated inventory and pricing evidence. Remove production exposure for
models that the provider no longer offers, add newly supported video models,
set each verified YuAPI base price to 20% above the provider request price, and
apply the existing YuAPI group ratio as a separate final multiplier.

The work also refreshes the capability catalog and video developer
documentation, proves every production-enabled model with a smallest valid real
generation, and releases through a blue-green hot switch. The currently running
production container must stay running throughout preparation, cutover,
observation, and rollback readiness.

No credential, account identity, provider hostname, server address, session,
cookie, token, private task identifier, or real user identifier may be written
to Git, logs, screenshots, fixtures, or handoff evidence.

## 2. Scope and boundaries

### In scope

- Cangyuan VIDEO-group model inventory and exact public-to-upstream mapping.
- Video capability validation for duration, resolution, aspect ratio, generated
  audio, reference images, reference video/audio, and frame inputs.
- Per-generation task prices for verified video models.
- The Cangyuan video channel model lists, mappings, and enabled state.
- `TASK_PRICE_PATCH` membership for fixed per-request video models.
- YuCore Studio, Infinite Canvas, media catalog, task lifecycle, result recovery,
  download behavior, and matching developer documentation.
- Automated contract tests, real paid generations, local visual checks, and a
  no-stop production release.

### Out of scope

- The upstream `gemini-music` entry, because it is audio rather than video.
- All image providers, image models, and image prices.
- The separate non-Cangyuan Grok preview channel.
- GPT text relay, cache affinity, token usage estimation, stream settlement,
  tiered billing, the 300K threshold, and GPT prices.
- Group ratio changes, authentication changes, database migrations, Caddy/DNS/TLS
  redesign, brand redesign, or protected project identity.

Video task billing remains strictly isolated from GPT token usage. A video task
does not inherit GPT cache, token estimation, terminal-usage, disconnect, or
stream-settlement rules.

## 3. Audited provider state

The latest authenticated VIDEO token returned 21 model identifiers: 20 video
models and the out-of-scope `gemini-music` entry. Public VIDEO-group pricing
contained 14 rows: 13 video models plus `gemini-music`. The intersection yields
13 currently priced and routable video models.

Seven video identifiers are visible but have no public price. They are cataloged
as `probe` only and must not be placed on an enabled production channel until an
auditable price and real debit are established:

- `sd4-seedance-2.0`
- `sd4-seedance-2.0-fast`
- `sd8-seedance-2.0-fast`
- `seedance-2.0-fast`
- `seedance-2.0-mini`
- `seedance-2.0-mini-8s`
- `veo-clean`

An inventory entry is not availability proof. A model becomes production
enabled only when all of these agree: authenticated inventory, pricing unit and
cost, create contract, accepted task ID, same-ID polling, completed asset,
observed provider debit, YuAPI charge, and refresh/download recovery.

### 3.1 Production delta

The current Cangyuan production video routes expose 15 identifiers. The target
starts with 13 verified-price identifiers.

Retain and revalidate:

- `omni-fast`
- `omni-fast-no-water`
- `omni-v2v`
- `omni-v2v-no-water`

Remove from enabled Cangyuan channels because they are no longer in the
authenticated VIDEO inventory:

- `sora-2`
- `sora-2-pro`
- `veo-3-1`
- `veo-3-1-fast`
- `veo-3-1-ref`
- `sd5-seedance-2.0`
- `sd5-seedance-2.0-fast`

Remove from enabled channels and retain only as `probe` because no current price
is published:

- `sd4-seedance-2.0`
- `sd4-seedance-2.0-fast`
- `sd8-seedance-2.0-fast`
- `seedance-2.0-fast`
- `seedance-2.0-mini`
- `seedance-2.0-mini-8s`
- `veo-clean`

Add after real validation:

- `grok-video`
- `grok-video-1.5`
- `happyhouse-1.0`
- `happyhouse-1.1`
- `minimax-h3-2k`
- `sd7-seedance-2.0-1080p`
- `sd7-seedance-2.0-720p`
- `sd8-seedance-2.0`
- `seedance-2.0`

## 4. Pricing design

All 13 verified rows are billed by the provider per generation. They must use
YuAPI fixed task pricing and be included in `TASK_PRICE_PATCH`; duration,
resolution, generated audio, references, polling, content reads, and downloads
must not multiply the charge.

The exact rule is:

```text
base_price = ceil(provider_price * 1.20 * 10000) / 10000
final_price(group) = base_price * existing_group_ratio(group)
```

Rounding upward at four decimal places guarantees that decimal truncation never
reduces the intended 20% base markup. Existing ratios remain unchanged:

- `多模态创作`: `1.2`
- `下游多模态`: `1.0`

| Public model ID          | Provider price | YuAPI base | `多模态创作` final | `下游多模态` final |
| ------------------------ | -------------: | ---------: | -----------------: | -----------------: |
| `grok-video`             |           0.69 |      0.828 |             0.9936 |              0.828 |
| `grok-video-1.5`         |           1.39 |      1.668 |             2.0016 |              1.668 |
| `happyhouse-1.0`         |            4.5 |        5.4 |               6.48 |                5.4 |
| `happyhouse-1.1`         |            2.9 |       3.48 |              4.176 |               3.48 |
| `minimax-h3-2k`          |            3.5 |        4.2 |               5.04 |                4.2 |
| `omni-fast`              |         0.6624 |     0.7949 |            0.95388 |             0.7949 |
| `omni-fast-no-water`     |           0.81 |      0.972 |             1.1664 |              0.972 |
| `omni-v2v`               |         0.8856 |     1.0628 |            1.27536 |             1.0628 |
| `omni-v2v-no-water`      |          1.035 |      1.242 |             1.4904 |              1.242 |
| `sd7-seedance-2.0-1080p` |            4.9 |       5.88 |              7.056 |               5.88 |
| `sd7-seedance-2.0-720p`  |            3.9 |       4.68 |              5.616 |               4.68 |
| `sd8-seedance-2.0`       |            2.9 |       3.48 |              4.176 |               3.48 |
| `seedance-2.0`           |            3.9 |       4.68 |              5.616 |               4.68 |

The published developer table shows the `多模态创作` final price because that
is the main user-facing creative group. The API pricing surface remains the
authority for group-specific totals.

No price is invented for a `probe` model. Promotion requires a current published
price or a separately approved, bounded debit-observation procedure; after cost
is established, the same formula and full validation matrix apply.

## 5. Capability and mapping design

The capability catalog remains the single source for backend validation, Studio
and Canvas controls, and developer documentation. Only provider-documented
fields are forwarded. Optional scalar values must preserve the distinction
between absent and explicit `0` or `false`.

### 5.1 Family summary

| Family         | Audited capability boundary                                                                                                                                                          |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Grok           | 4/6/8/10/12/15 seconds; 480p or 720p; `1:1`, `16:9`, `9:16`, `4:3`, `3:4`, `3:2`, `2:3`; image references supported.                                                                 |
| Happyhouse 1.0 | 3-15 seconds; 720p or 1080p; generated audio; up to 9 images, or up to 5 images with one reference video.                                                                            |
| Happyhouse 1.1 | 3-15 seconds; 720p or 1080p; generated audio; up to 9 images; no reference video/audio or frame mode.                                                                                |
| Minimax H3     | 5-15 seconds; fixed 2K family; generated audio; frame mode; up to 5 images and 3 audio references; no reference video.                                                               |
| Omni Fast      | Fixed about 10 seconds and 720p; `16:9` or `9:16`; up to 5 images; frame inputs supported; duration/resolution omitted upstream.                                                     |
| Omni V2V       | Fixed about 10 seconds and 720p; `16:9` or `9:16`; exactly one reference video; duration/resolution omitted upstream.                                                                |
| SD7            | 4-15 seconds; resolution fixed by the public model ID; six documented aspect ratios; generated audio; up to 5 images, 3 videos, and 3 audios.                                        |
| SD8            | 5, 10, or 15 seconds; five documented aspect ratios; no generated-audio or frame control; up to 9 images, 3 videos, and 3 audios; enforce the documented person-image eye-mask rule. |
| Seedance 2.0   | 4-15 seconds; fixed 720p; six documented aspect ratios; generated audio; up to 5 images, 3 videos, and 3 audios.                                                                     |

Exact allowed values and payload field names are copied from the current
authenticated pricing metadata and API documentation, then frozen in fixtures
that match observed requests. The implementation must not preserve stale
capabilities merely because they exist in the previous 33-model catalog.

### 5.2 Public and upstream identifiers

The public YuAPI IDs are the authenticated `/v1/models` IDs in this design.
Some provider examples use prefixed or historical aliases instead of those
public IDs. In particular, Grok, Omni, and Seedance examples are not consistent with
the inventory response.

Real probes resolve this discrepancy. A prefixed alias may be placed in channel
`model_mapping` only after an explicit upstream validation rejection proves the
public ID was not accepted and the alternate ID succeeds. An accepted or
ambiguous creation is never retried with another alias. Provider-only aliases
are never exposed as YuAPI public model IDs.

## 6. Channel and configuration design

The VIDEO credential, upstream origin, channel type, priorities, weights, and
approved groups remain unchanged. No secret is rotated or exposed by this work.
Target channels are staged as disabled records and separated by family so a
single family can be disabled without taking down all video routes:

- Omni: four retained models.
- Grok: two new models.
- Happyhouse: two new models.
- Minimax: one new model.
- Seedance: four Seedance/SD7/SD8 models.

The legacy Cangyuan video channel rows are retained disabled during the
observation period as an exact configuration rollback source. They are not
deleted during cutover. No target model may be enabled in both a legacy and a
replacement channel at the same priority, which would make routing and debit
evidence ambiguous.

The scoped production configuration set consists only of:

- replacement video channel rows and their model mappings;
- enabled/disabled states for the legacy and replacement video channels;
- the 13 video entries in `ModelPrice`;
- the exact 13-model `TASK_PRICE_PATCH` value;
- the refreshed `yucore_media.model_capabilities` override only if an operator
  override is still needed after the embedded catalog is updated.

Before any write, export the old values for only these keys and rows to a
root-readable, timestamped rollback artifact outside Git. Apply the target
values in one database transaction where supported by the existing settings
path, refresh caches explicitly, and verify the readback. Do not restore a
database snapshot for either release or rollback.

## 7. Task and billing invariants

Every video generation follows the existing asynchronous task state machine:

1. Resolve one enabled model and group, validate capability and references, and
   freeze the fixed base price plus group ratio.
2. Pre-consume exactly once.
3. Send exactly one creation `POST /v1/videos`.
4. Persist the first accepted public/upstream task ID.
5. Poll only that escaped ID with `GET /v1/videos/{task_id}`.
6. Read content only from `GET /v1/videos/{task_id}/content` or the normalized
   authenticated local asset route.
7. Persist completion and restore the same asset after refresh in Studio and
   Infinite Canvas.

Polling, page refresh, content reads, thumbnails, downloads, Canvas restoration,
and client reconnects never charge again and never create a replacement task.
A proven pre-write rejection may use the existing safe rollback. An accepted or
ambiguous creation is neither automatically retried nor automatically refunded.

## 8. Documentation design

Refresh the existing YuCore developer document rather than adding a second
provider-specific guide. It must contain only production-enabled public model
IDs and observed contracts:

- the 14-model final-price table for `多模态创作`;
- the shared create, same-ID poll, content, status, and error contract;
- concise examples for Grok, Happyhouse, Minimax, Omni image-to-video, Omni
  video-to-video, SD4 frame/multimodal mode, SD7 resolution-specific models, and
  SD8 reference constraints;
- exact supported duration, resolution, aspect ratio, audio, and reference
  limits;
- an explicit statement that all listed models are fixed per generation and
  that polling/downloads are free;
- removal of Sora, old Veo, SD5, old Seedance, and unpriced Mini examples.

Examples use environment placeholders and non-sensitive example asset URLs.
They never contain the provider origin, channel IDs, credentials, real task IDs,
or account data.

## 9. Verification

### 9.1 Automated checks

Implementation starts test-first and covers observable contracts:

- exact enabled/probe inventory and public-to-upstream mappings;
- family-specific allowed and rejected parameters;
- absent versus explicit `false` where generated audio is supported;
- fixed-duration Omni omission of duration/resolution;
- required Omni V2V video input;
- frame/multimodal mutual exclusion and reference count limits;
- one creation POST, accepted-ID persistence, escaped same-ID polling, and no
  second POST after retryable polling failures;
- completed, failed, canceled, malformed, and ambiguous responses;
- no double charge and no charge for poll/content/download;
- exact fixed base prices and group multiplication;
- catalog, Studio, Canvas, refresh recovery, and documentation consistency;
- existing image and GPT text regression suites.

Required verification includes focused Go tests, `go test ./...`, Default and
Classic frontend builds, affected frontend lint/type checks, and production-like
container health checks. No database-specific implementation may break SQLite,
MySQL, or PostgreSQL.

### 9.2 Real paid tasks

Run one smallest valid task for each of the 13 target models through an isolated
candidate using the real VIDEO-group quota. The known provider-price subtotal
for one accepted task per model is `31.973` provider pricing units. Explicitly
rejected, non-debited model-ID checks may precede a successful task; accepted or
ambiguous requests are never repeated.

Use the smallest supported duration/resolution even though these models are
priced per generation. Reuse only non-sensitive test assets. For each model,
record a redacted case ID, public model, proven upstream mapping, input mode,
creation attempt count, accepted-ID presence, normalized status progression,
result MIME/dimensions/duration, provider balance delta, expected YuAPI base and
group charge, Studio/Canvas recovery, download outcome, and pass/fail.

The seven unpriced `probe` models are not included in the known-cost batch. They
remain unavailable unless a separately reviewed price/debit procedure first
bounds their unknown spend.

### 9.3 Local and private candidate acceptance

Before server preparation, run the candidate locally on a private port with an
isolated test database/configuration. Verify home, authentication pages, console,
API keys, settings, pricing, Studio, Infinite Canvas, and developer docs. Compare
desktop and mobile screenshots against the production visual baseline; the
expected visual difference outside video model controls and documentation is
zero.

On the server, start the candidate on a new private localhost port and a unique
network alias while the current production container remains healthy and
serving. Verify candidate health, restart count, asset fingerprints, database
compatibility, private catalog, one bounded end-to-end task, billing readback,
and logs before requesting traffic-switch approval.

## 10. Hot production release

Production cutover is a separate explicit approval gate. No current container,
rollback container, Caddy container, database, or Redis service is stopped or
restarted for the switch.

1. Re-audit the running image, containers, networks, private ports, Caddy runtime
   config, and the exact two active YuAPI upstream references. Runtime evidence
   wins over an older handoff table.
2. Build a uniquely tagged image from the reviewed commit and record its digest.
3. Start a uniquely named candidate beside production on a new localhost port.
   Attach a new alias without removing, reusing, or remapping any existing alias.
4. Prove Caddy-to-candidate connectivity and all private acceptance checks while
   the public route still points to the current app.
5. Obtain explicit user approval for the traffic switch.
6. Validate the proposed Caddy file, replace only the two intended YuAPI upstream
   references, and perform a graceful Caddy reload. Do not restart Caddy and do
   not stop the current app. Existing connections may drain against the old app,
   whose alias and container must remain reachable.
7. Apply and read back the scoped video configuration transaction immediately
   after the candidate route is healthy. Both code versions must remain safe for
   existing accepted-task polling during the overlap.
8. Verify public health, protected pages, new catalog and pricing, one cheapest
   bounded target task, same-ID polling/content, quota, aggregate errors, restart
   counts, database errors, and absence of 502s.
9. Keep the old app, all old aliases, rollback artifacts, and legacy channel rows
   intact for the full observation period. Cleanup requires separate approval.

### 10.1 Rollback

Rollback is configuration restoration plus a graceful traffic reload; it is not
a database snapshot restore.

1. Restore only the captured video channel, `ModelPrice`, `TASK_PRICE_PATCH`, and
   capability override values; refresh caches and verify readback.
2. Validate a Caddy config that restores the exact two prior upstream references
   and reload it gracefully.
3. Verify public health, old catalog/pricing, accepted-task polling, assets,
   billing, logs, and no 502s.
4. Keep the candidate running until rollback is confirmed. Do not stop the old
   or candidate container as part of the immediate rollback.

Rollback is triggered by unexpected UI changes, health/restart regression,
database errors, 502s, duplicate creation, lost accepted IDs, asset leakage,
missing/double/below-cost billing, poll billing, or image/GPT/auth regressions.

## 11. Definition of done

The work is complete only when:

- the 13 priced and visible video models pass real generation and are the only
  enabled Cangyuan video inventory;
- all removed and unpriced models are absent from enabled channels;
- every base price follows the exact 20%-markup formula and both group totals
  match the unchanged group ratios;
- fixed task charging is isolated from GPT token/cache/stream rules;
- public-to-upstream mappings and capabilities match observed requests;
- automated, build, visual, task, billing, recovery, and download checks pass;
- developer documentation contains no stale models, prices, or contracts;
- the candidate is reviewed locally and privately before production approval;
- production cutover uses a graceful two-reference Caddy reload with the current
  container continuously running and reachable;
- the scoped configuration rollback and Caddy rollback are proven and retained;
- no secret or real user identity appears in source control or test evidence.
