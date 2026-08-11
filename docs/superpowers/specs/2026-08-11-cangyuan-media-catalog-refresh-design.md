# Cangyuan Media Catalog Refresh Design

## Goal

Refresh the Cangyuan image and video integration from the exact production
source baseline, expose only models that are genuinely usable through YuAPI,
extend the Infinite Canvas for the provider's current media capabilities, and
publish matching developer documentation without changing the existing brand
UI or weakening billing guarantees.

The implementation baseline is production source commit
`0918868420218c7b45ef0ee02702efa5e8dc7aee`. The running production image,
containers, Caddy routing, database, and traffic remain unchanged until a local
candidate has passed verification and the user gives a separate production
approval.

## Confirmed Current State

### Upstream inventory

The authenticated upstream account currently exposes 33 OpenAI-compatible video
models through `/v1/models`. Public pricing documents 31 of them. The other two,
`seedance-2.0-mini-8s` and `veo-clean`, are account-visible but do not have public
pricing entries.

The current video inventory is:

- Omni: `gemini-omni-flash`, `omni-fast`, `omni-fast-no-water`, `omni-v2v`,
  `omni-v2v-no-water`.
- Grok: `grok-video`, `grok-video-1.5`.
- Happyhouse: `happyhouse-1.0`, `happyhouse-1.1`.
- Kling: `kling-3.0`, `kling-3.0-omni`.
- Minimax: `minimax-h3-2k`.
- Seedance: `sd5-seedance-2.0`, `sd5-seedance-2.0-fast`,
  `sd6-seedance-2.0-1080p`, `sd6-seedance-2.0-720p`, `seedance-2.0`,
  `seedance-2.0-1080p`, `seedance-2.0-480p`, `seedance-2.0-4k`,
  `seedance-2.0-720p`, `seedance-2.0-fast`, `seedance-2.0-fast-480p`,
  `seedance-2.0-fast-720p`, `seedance-2.0-mini`,
  `seedance-2.0-mini-480p`, `seedance-2.0-mini-720p`,
  `seedance-2.0-mini-8s`, `seedance-2.5-480p`, and
  `seedance-2.5-720p`.
- Veo: `veo-3.1`, `veo-3.1-fast`, `veo-clean`.

The upstream image catalog currently documents nine models. YuAPI will retain
seven on this provider: `gpt-image-2-2k` plus the six existing Nano Banana
resolution variants. `gpt-image-2-1k` and `gpt-image-2-4k` are deliberately
removed only from the Cangyuan channel because the current YuAPI price for these
two routes is below this provider's cost and cheaper channels already serve
them. Their global model prices and other channels are not changed.

### Stale and removed identifiers

The production Cangyuan video configuration still contains `sora-2`,
`sora-2-pro`, `veo-3-1`, `veo-3-1-fast`, and `veo-3-1-ref`. None of these five
identifiers appears in the authenticated upstream model list. The two Sora
models are removed from this provider. The old Veo identifiers are replaced by
the current identifiers only after real request validation. `veo-clean` is not
assumed to be a semantic alias of `veo-3-1-ref`; its behavior must be established
from a real task result.

### Token mismatch

All five current Cangyuan production channels share one upstream credential.
That credential belongs to the upstream `VIDEO` group. It accepts video model
validation but cannot route the configured image models, so the existing image
channels are not valid despite being enabled.

No credential value, cookie, request header, account secret, or upstream token
may be committed, logged, included in screenshots, or written to the design or
developer documentation.

## Approaches Considered

### Recommended: capability-driven catalog with separate upstream groups

Treat the authenticated upstream model list as inventory evidence, public
pricing as pricing evidence, and paid generation as the final availability
proof. Split image and video credentials by upstream group. Store explicit
per-model capabilities, validate requests against them, and render only the
controls each model supports.

This approach fixes the known image credential defect, supports advanced media
inputs without provider-specific UI branches, and prevents a model-list entry
from being mistaken for a usable production route.

### Alternative: update only model names

Replacing stale IDs in the existing channels is smaller, but it leaves image
generation broken, omits reference video/audio and model-specific validation,
and cannot prove billing or canvas behavior. It is rejected.

### Alternative: expose all upstream fields as pass-through JSON

An unrestricted pass-through would be quick but would make validation,
documentation, billing, and future compatibility unreliable. It also increases
the risk of unsupported fields being silently ignored upstream. It is rejected.

## Channel and Group Design

### Credential boundary

- Create a dedicated upstream `IMAGE` token for the two Cangyuan image channels.
- Keep the existing upstream `VIDEO` token for video channels.
- Use the new image token only in the isolated local environment first.
- Do not change production channel credentials during implementation or local
  verification.

### YuAPI group placement

- Image routes remain in `生图按次`, `多模态创作`, and `下游多模态`.
- Video routes remain in `多模态创作` and `下游多模态`.
- Existing group ratio `1.2` remains unchanged.

### Channel partition

Provider models are partitioned by family so an upstream family failure can be
disabled without taking down unrelated media routes:

- `cangyuan-gpt-image-fixed`: `gpt-image-2-2k` only.
- `cangyuan-nano-banana-fixed`: the six existing Nano Banana variants.
- `cangyuan-video-omni`: the five Omni models.
- `cangyuan-video-grok`: the two Grok models.
- `cangyuan-video-happyhouse`: the two Happyhouse models.
- `cangyuan-video-kling`: the two Kling models.
- `cangyuan-video-minimax`: `minimax-h3-2k`.
- `cangyuan-video-seedance`: the 18 Seedance-family models.
- `cangyuan-video-veo`: the three current Veo identifiers.

The catalog tracks all 33 account-visible video models. The 31 publicly priced
models can be enabled after real validation. `seedance-2.0-mini-8s` and
`veo-clean` remain disabled until a paid task proves creation, completion,
download, actual mode, and an auditable upstream charge. They may then be
enabled without changing the catalog schema.

## Capability Model

Extend `YucoreMediaModelCapability` so one catalog record is the source of truth
for server validation, Canvas controls, and generated documentation data. The
record describes:

- public model ID and upstream model ID;
- media kind and sync or async transport;
- creation, edit, status, content, and optional cancel paths;
- allowed durations, resolutions, aspect ratios, and fixed-value constraints;
- support for generated audio and audio enablement;
- reference modes and limits for image, video, and audio inputs;
- first-frame and last-frame semantics;
- allowed generation fields such as negative prompt and seed;
- poll interval, maximum poll duration, terminal states, and result format;
- pricing unit (`request` or `second`) and availability state;
- capability notes that can be displayed in documentation without exposing
  internal channel data.

Existing task `inputs` and `metadata` JSON fields carry the expanded request and
result metadata. No database migration is required.

The backend, not the browser, is authoritative. It rejects unsupported fields,
too many references, invalid duration/resolution combinations, mismatched media
types, and unknown reference modes before submitting upstream. Optional scalar
fields use pointer semantics so omitted values stay omitted while explicit zero
or false values remain representable where the provider supports them.

## Request and Task Flow

The Infinite Canvas and public API use the same YuAPI request path. The browser
does not call Cangyuan directly.

1. Resolve the public model and its capability record.
2. Validate prompt, duration, resolution, aspect ratio, audio setting, and all
   references against that record.
3. Normalize the accepted request into the existing task `inputs` structure.
4. Pre-consume exactly once using a frozen billing snapshot.
5. Submit exactly one `POST /v1/videos` or image request to the selected channel.
6. Persist the returned upstream task ID and accepted state.
7. Poll only that task ID through the same channel/account context.
8. Store terminal status, content URL, thumbnail, duration, and media metadata.
9. Restore the result as a playable or downloadable Canvas node after refresh.

### Retry and ambiguity rules

A request may be retried on another channel only when failure is proven to have
occurred before an upstream task was accepted. A network error after request
bytes may have reached the upstream is ambiguous and must not trigger an
automatic second creation request. Poll failures retry the same task lookup;
they never create a replacement task.

Client disconnect, page closure, polling timeout, or process restart after the
upstream accepts a task does not refund quota and does not create a duplicate.
The persisted task remains recoverable by its public task ID.

## Billing Invariants

Existing model prices and all existing group ratios remain unchanged.

- Per-request upstream models use their verified upstream request price as the
  YuAPI base model price and are included in `TASK_PRICE_PATCH`, so duration or
  resolution metadata cannot multiply the fixed request charge.
- Per-second upstream models use the verified upstream per-second price as the
  base model price, are excluded from `TASK_PRICE_PATCH`, and multiply the frozen
  base price by the validated billable duration through task `OtherRatios`.
- The `1.2` group ratio is applied after the base cost calculation, keeping the
  customer charge above the recorded upstream cost.
- Submitted duration is normalized once and used by validation, upstream
  payload construction, pre-consumption, task persistence, and audit logging.
- A task is charged once. Polling, result download, Canvas restoration, and
  repeated status reads never charge again.
- A failure proven to occur before upstream acceptance follows existing safe
  pre-consumption rollback. Any accepted or ambiguous submission retains the
  charge because the upstream may already have billed it.
- Unknown-price models cannot be production-enabled merely because they appear
  in `/v1/models`. Their real upstream debit must be observed and converted into
  an explicit base price first.

This work does not alter GPT token pricing, tiered token expressions, cache
pricing, or the existing 300K threshold.

## Infinite Canvas Design

The production brand, navigation, page composition, animation, and existing
Canvas controls are preserved. New controls appear only within the existing
media generation workflow and only when the selected capability requires them.

The model selector drives:

- duration, resolution, and aspect-ratio controls;
- generated-audio setting where supported;
- reference mode selection;
- image, video, and audio reference upload slots with count and type limits;
- first-frame and last-frame slots where supported;
- negative prompt and seed where supported.

Uploads show local validation state before submission. Unsupported combinations
are blocked with actionable errors. Task nodes show queued, processing,
succeeded, and failed states without layout shifts. Successful video nodes
provide playback and download, and retain enough metadata to restore the same
result after a page reload.

All new user-facing strings use `react-i18next` and are synchronized across
English, Chinese, French, Russian, Japanese, and Vietnamese locale files.

## Documentation Design

Update the existing YuCore developer document rather than adding a separate
provider-only manual. The document includes:

- a dated catalog change section listing removed, replaced, and added IDs;
- the current image and video model table with billing unit and availability;
- image creation and edit requests;
- video creation, status, and content endpoints;
- a capability matrix for duration, resolution, aspect ratio, audio, and
  reference input types;
- request validation and error behavior;
- polling and task recovery examples;
- billing examples for both per-request and per-second models;
- a warning that creation requests must not be automatically repeated after an
  ambiguous network failure.

Examples use placeholders only. They do not contain live domains that reveal
private routing, credentials, cookies, tokens, request headers, or account data.

## Verification

### Automated tests

- Catalog tests cover every enabled model, unique model IDs, valid paths,
  capability constraints, and explicit pricing units.
- Request tests cover omitted versus explicit scalar values, allowed fields,
  rejected combinations, reference counts and types, and upstream payloads.
- Billing tests assert exact quota for fixed-price and per-second examples,
  group-ratio application, one-time charging, pre-acceptance rollback, and no
  refund after accepted or ambiguous submission.
- Task tests cover single submission, same-ID polling, restart recovery,
  terminal metadata, and no duplicate creation after client cancellation.
- Frontend tests cover capability-driven controls, validation, task restoration,
  and playable output nodes.
- Existing Go, frontend build, i18n, relay, task, quota, and media tests remain
  green.

### Paid upstream validation

Use an isolated local configuration and real paid tasks. Validate the seven
retained image models and all 33 account-visible video models. For expensive or
multi-input modes, use the smallest valid duration and resolution that still
proves the capability. At least one real request must cover each special mode:
reference image, reference video, reference audio, first/last frame, generated
audio, and video-to-video where advertised.

For every task, record only non-secret audit facts: public test case ID, model,
validated parameters, upstream task status, output media type, duration,
observed upstream debit, YuAPI quota, and pass/fail. Do not record response
headers, credential-bearing URLs, raw account data, or private database rows.

The paid image tests also produce a requested local deliverable based on the
provided square purple quota-card reference. Generate final cards for quota
amounts `5`, `20`, `50`, `100`, `200`, and `500`. Preserve the reference's
Chinese information hierarchy and premium purple luminous style, keep each
amount clearly legible, and do not overwrite the supplied reference. Exercise
all seven retained image routes during generation; when two routes are used for
the same amount, keep the better result as the six-card final set while retaining
the other only as non-final test evidence. Save the six selected files in a
dated local output directory outside the Git commit, with stable filenames that
include the quota amount and generating model.

### UI and baseline comparison

Run the candidate on a private localhost port with an isolated database. Use
Playwright to compare production and local candidate pages for the home page,
sign-in/sign-up, console layout, API key page, system settings, Infinite Canvas,
developer documentation, custom brand elements, and animation behavior.

The expected visual diff is zero outside the approved media controls and
documentation content. The user reviews the running local candidate before any
production preparation.

## Deployment and Rollback Boundary

Implementation and paid tests remain local until the user approves the local
candidate. Production preparation then requires a separate explicit approval.

Any production candidate must:

- be built from this exact production-derived branch;
- bind only to `127.0.0.1` during private verification;
- use the existing production database without destructive migration;
- pass health, UI, API, task, billing, and database compatibility checks before
  traffic movement;
- preserve the currently running image, old containers, channel configuration,
  and rollback metadata.

Caddy and the production container are not changed without explicit user
confirmation. If post-switch checks find a UI, database, billing, task, or
provider regression, traffic and channel configuration are restored immediately
to the retained production baseline. No old database snapshot is restored, and
no production transaction data is reset.

## Non-Goals

- Deploying the old affiliate branch or merging it wholesale.
- Changing the production brand or redesigning the Infinite Canvas.
- Changing existing model prices, group ratios, GPT token billing, cache
  accounting, or the 300K pricing threshold.
- Restoring or resetting production database data.
- Assuming that an upstream model-list entry proves generation support.
- Removing retained production images or containers during the testing period.
