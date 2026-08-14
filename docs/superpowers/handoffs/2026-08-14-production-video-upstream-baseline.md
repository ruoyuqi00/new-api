# YuAPI Production Baseline and Video Upstream Handoff

> Status: active handoff baseline
>
> Recorded: 2026-08-14 (Asia/Shanghai)
>
> Scope: integrating additional video-generation upstreams without changing the
> production brand, existing billing rules, GPT text relay, cache-affinity work,
> database contents, or production routing before explicit approval.

## 1. Purpose and authority

This document is the starting point for the next YuAPI task that integrates one
or more video-generation upstreams. It is both a production fact record and an
execution boundary.

If an older design, branch name, image tag, provider inventory, or remembered
configuration conflicts with this document, stop and verify the running image
and the source history before changing code. Do not infer a production source
baseline from an image tag alone.

The following documents remain useful as historical design input, but they are
not current production baselines:

- `docs/superpowers/specs/2026-08-11-cangyuan-media-catalog-refresh-design.md`
- `docs/superpowers/specs/2026-08-03-canvas-media-model-integration-design.md`
- `docs/YUCORE_MULTIMODAL_COMPATIBILITY_DESIGN_2026-07-12.md`

Provider model lists, prices, endpoints, and capabilities in historical
documents must be re-audited. Never copy an old provider catalog into production
without a current model-list audit and real generation tests.

## 2. Immutable production baseline

The facts below were rechecked read-only on 2026-08-14. No production state was
changed while recording them.

| Item | Current baseline |
| --- | --- |
| Production source commit | `290db8f250618f1c8f690f0dfb3cbeecc58aacb2` |
| Production source branch | `codex/scanner-cache-key-safety-20260813` |
| Running image | `yuapi:production-20260814-290db8f25` |
| Image ID | `sha256:d91bd9d8d95160aa6a72fa92b163958a0337d105d469adf12047ead86d4111f2` |
| Image creation time | `2026-08-13T17:06:14.457304917Z` |
| Running container | `newapi-production-20260814-290db8f25` |
| Private binding | `127.0.0.1:13009 -> 3000/tcp` |
| Runtime state | healthy, restart count `0` |
| Caddy target count | exactly two references to `newapi-production-20260814-290db8f25:3000` |
| Caddy rollback copy | `/opt/edge/Caddyfile.pre-20260813T173802Z-b66e99504` |
| Retained rollback image | `yuapi:production-20260812-b66e99504` |
| Retained rollback container | `newapi-production-20260812-b66e99504` |
| Rollback private binding | `127.0.0.1:13005 -> 3000/tcp` |
| Rollback runtime state | healthy, restart count `0` |

The older candidate containers ending in `5f1a89e32`, `8ec9b5ffd`, and
`2ff69650d` are stopped diagnostic artifacts, not valid production baselines.
Do not select them as source or deployment candidates.

The much older `production-20260804-309717aea` image remains important incident
history: it proved that an image tag and Git commit can describe different
effective frontend build inputs. It is not the current production baseline.

The commit that adds only this handoff document may be used as the parent of a
new video-integration branch. Its application code is identical to
`290db8f250618f1c8f690f0dfb3cbeecc58aacb2`.

## 3. Brand and UI baseline

The user approved the current production-aligned local UI before the latest
switch. The public production homepage currently references these assets:

- `/static/css/6189.315a58962e.css`
- `/static/css/index.f6021f0c1d.css`
- `/static/js/6189.a8e25ef015.js`
- `/static/js/index.9c6f815e5e.js`
- `/static/js/lib-react.a6dd11adaa.js`
- `/static/js/vendor-tanstack.7425bb6434.js`
- `/static/js/vendor-ui-primitives.f8cdb75d06.js`

These fingerprints are evidence, not a substitute for visual comparison. A
frontend rebuild may legitimately change a fingerprint while accidentally
losing the brand. Every candidate must still be compared visually.

The following UI is protected unless a separately approved video feature
requires a narrowly scoped change:

- home page branding, typography, motion, WebGL effects, navigation, and footer;
- sign-in and sign-up pages;
- console shell, dashboard, API key page, and system settings;
- default and classic theme behavior;
- Infinite Canvas layout, saved canvases, existing nodes, asset history, and
  existing image-generation workflows;
- developer documentation layout and current custom brand content.

Expected visual difference for a video-provider patch is zero outside approved
model options, video controls, video task states, result nodes, and matching
documentation content.

## 4. Hard no-touch boundaries

The video integration task must not make unrelated changes to:

- Caddy, Cloudflare, DNS, public domains, TLS, or production container routing;
- MySQL or Redis containers, production transaction data, balances, logs, or
  existing task rows;
- database snapshots or destructive migrations;
- rebate behavior, group ratios, existing model prices, the GPT 300K tier, or
  token accounting;
- GPT text relay, Responses pass-through, scanner status, prompt-cache key
  derivation, channel affinity, cooldown, retry, or disconnect settlement;
- login, OAuth, passkeys, session limits, Turnstile, or authentication policy;
- existing image providers and image prices unless the user explicitly expands
  the task;
- protected new-api or QuantumNous project identity and attribution.

Do not merge an old feature branch wholesale. Port each provider or protocol
change as a small, reviewable patch with its own tests and real-task evidence.

## 5. Existing media architecture

The current baseline already provides the common media foundation. A new
provider should extend this foundation instead of creating a second task system.

### 5.1 Public and authenticated YuCore surfaces

Authenticated media APIs include:

- `GET /api/yucore/media/catalog`
- `GET /api/yucore/media/models`
- `GET /api/yucore/media/health`
- `POST /api/yucore/media/uploads`
- `GET|POST /api/yucore/media/tasks`
- `GET|PATCH|DELETE /api/yucore/media/tasks/{task_id}`

Infinite Canvas uses the existing `/api/yucore/canvas` and agent-run routes.
The browser must never call a video provider directly or receive a provider
credential.

### 5.2 Supported adapter boundary

Current media adapters are:

- `mock`: local development only;
- `openai-compatible`: direct compatibility testing for one configured origin;
- `yuapi-channel`: the production path through normal YuAPI groups, channels,
  routing, accounting, logs, and managed tokens;
- `uag-proxy`: existing UAG-specific compatibility path.

For multiple production video upstreams, keep the media bridge on
`yuapi-channel`. Add each upstream as a normal YuAPI channel. Do not rotate the
global `yucore_media.base_url` or `yucore_media.api_key` between providers.

A provider should share a channel only when its base URL, credential boundary,
settlement account, concurrency limit, failure isolation, and operational owner
are the same. A different credential pool, billing account, concurrency policy,
or failure domain requires a separate channel. Different model names alone do
not require separate channels.

### 5.3 Canonical task and capability model

The existing capability layer can describe:

- public and upstream model IDs;
- image or video kind and model family;
- `enabled` or `probe` availability;
- `per_call` or `per_second` pricing unit;
- synchronous image or asynchronous task transport;
- create, edit, status, content, and cancel paths;
- duration policy (`duration`, `seconds`, `fixed`, or `none`);
- allowed durations, resolutions, aspect ratios, and generated audio;
- seed support and allowed upstream parameters;
- image, video, audio, first-frame, and last-frame references;
- per-type and total reference limits;
- polling interval, polling deadline, terminal states, and response format.

Canonical reference inputs are typed records containing role, URL, optional
MIME type, and optional duration. Optional scalar values preserve explicit
`0` and `false`; absent values remain absent.

The task lifecycle already supports persisted public task IDs, accepted
upstream task IDs, processing state, normalized assets, thumbnails, MIME type,
dimensions, duration, and Canvas backflow. Creation and polling must reuse it.

### 5.4 Upload and asset security

Reference uploads are streamed to temporary files and renamed only after
validation. Current limits are:

- image: 25 MiB;
- audio: 25 MiB;
- video: 100 MiB.

Supported files are accepted by validated content structure and canonical MIME,
not by filename alone. Public task JSON exposes authenticated local asset URLs;
private provider content and thumbnail URLs are not serialized.

Provider credentials may be attached only to an asset URL that matches the
configured adapter, normalized origin, and permitted base-path boundary.
Cross-origin signed assets are fetched without gateway credentials. Redirects
must not gain credentials, browser Authorization must never be forwarded, and
userinfo-bearing URLs must be rejected.

## 6. Configuration model for multiple upstreams

### 6.1 Global media bridge settings

These keys configure the shared media bridge. Record names and purpose only;
never record production values in Git:

| Key | Purpose |
| --- | --- |
| `yucore_media.adapter` | Shared adapter; production multi-provider path should be `yuapi-channel`. |
| `yucore_media.base_url` | Internal bridge base URL, not a place to rotate external providers. |
| `yucore_media.api_key` | Direct adapter secret; must not be used as a browser credential. |
| `yucore_media.timeout_seconds` | Adapter request timeout. |
| `yucore_media.require_real_assets` | Requires real provider results for workflow readiness. |
| `yucore_media.model_capabilities` | Operator capability overrides merged with the embedded catalog. |
| `yucore_media.managed_token_group` | Legacy/preferred managed group fallback. |
| `yucore_media.uag_model_map` | UAG-only model mapping. |
| `yucore_media.uag_allowed_providers` | UAG provider allowlist. |
| `yucore_media.uag_allowed_models` | UAG model allowlist. |
| `yucore_media.upstream_verified` | Global verification gate; never set from a model-list response alone. |

`yucore_media.api_key`, environment equivalents, and channel keys are secrets.
Use the write-only admin/runtime secret path. Do not place them in Markdown,
fixtures, screenshots, shell history, commits, or task messages.

### 6.2 Per-upstream intake record

Create one local, non-secret intake record for each provider before coding:

| Field | Required evidence |
| --- | --- |
| Internal alias | A non-customer-facing name such as `video-provider-a`. |
| Protocol | Exact create, status, content, and cancel semantics. |
| Authentication mode | Header/query/body mechanism, recorded without the credential. |
| Credential boundary | Which account or pool owns billing and rate limits. |
| Model inventory | Current authenticated model list plus audit timestamp. |
| Public model mapping | YuAPI model ID to exact provider model ID. |
| Group exposure | Existing approved YuAPI groups only. |
| Pricing unit | `per_call` or `per_second`, with source evidence. |
| Cost evidence | Current provider cost and smallest valid paid probe. |
| Concurrency | Provider/account limits and safe channel concurrency. |
| Create contract | Method, path, request fields, accepted-ID location. |
| Poll contract | Method, escaped ID path, statuses, interval, deadline. |
| Result contract | Content, thumbnail, MIME, duration, dimensions, and multi-result paths. |
| References | Image/video/audio/frame roles, MIME rules, counts, and duration limits. |
| Error contract | Safe status/code mapping without exposing provider details. |
| Availability | `probe` until the full real workflow passes. |

The intake record must not include a real provider hostname if the repository is
shared beyond operators. Use an internal alias and keep the actual base URL in
the protected channel configuration.

### 6.3 Capability and payload rules

For each model:

1. Define the exact public model ID and upstream model ID.
2. Allow only provider-documented request parameters.
3. Map canonical reference roles to the provider's exact field names.
4. Send one duration field only. Fixed-duration and unsupported-duration models
   must omit all duration fields.
5. Preserve explicit zero and false values when supported.
6. Reject unsupported combinations before any upstream request is written.
7. Keep unverified or unknown-price models in `probe`; do not expose them to
   ordinary users.

Do not forward arbitrary JSON, invent unsupported aliases, or hash a complete
changing request body to simulate a stable identity.

## 7. Asynchronous task invariants

Every video creation must follow this sequence:

1. Validate user group, enabled model, capability, references, and billing.
2. Freeze the pricing inputs and pre-consume once.
3. Send exactly one creation POST.
4. Persist the first accepted public/upstream task ID and provider diagnostic ID.
5. Poll only the persisted accepted ID with GET or the provider's documented
   idempotent status method.
6. Normalize status and results without replacing the accepted ID from later
   poll payloads.
7. Publish only local authenticated content and thumbnail URLs.
8. Restore the same task and assets in Studio and Infinite Canvas after refresh.

Never submit a replacement task because polling failed, the browser closed, the
client disconnected, or the process restarted.

A proven pre-write rejection may follow the existing safe refund path. Once a
creation request was accepted or may have reached the upstream, treat it as
accepted/ambiguous: do not automatically repeat it and do not automatically
refund a charge the upstream may already have taken. Persist enough state for
operator review.

## 8. Billing rules

Video billing is task billing, not GPT token billing. A video request may have
zero text-token usage and still have a valid per-call or per-second charge. User
and operator logs must identify the model, billing unit, duration where
applicable, task status, and quota; they must not imply that zero tokens means a
free video task.

Required invariants:

- Existing model base prices and group ratios remain unchanged.
- A new model is not production-enabled until its billing unit is explicit.
- If YuAPI has no existing base price, set one only after verifying provider
  cost and keep the customer base price above cost; group ratio remains a
  separate business multiplier.
- `per_call` models charge once and duration/resolution metadata cannot multiply
  that charge.
- `per_second` models use one validated billable duration consistently across
  validation, payload, reservation, persistence, and final settlement.
- Polling, content reads, thumbnails, downloads, Canvas restoration, and status
  refreshes never charge again.
- A failed real probe must be classified by whether creation was definitely not
  sent, rejected, accepted, or ambiguous before any refund decision.
- Any change touching tiered/dynamic billing must first follow
  `pkg/billingexpr/expr.md`.

Do not use production balances as an ad hoc test budget. Bound each real probe
to the smallest valid duration/resolution and record only non-secret audit facts.

## 9. Validation matrix

### 9.1 Automated contract tests

Each provider patch must cover applicable cases:

- model mapping and capability validation;
- omitted versus explicit `0`/`false` parameters;
- text-to-video;
- image-to-video;
- first-frame and last-frame inputs;
- reference video and reference audio where supported;
- generated audio enabled and explicitly disabled;
- exact duration and resolution mapping;
- accepted ID extraction from the provider's real response envelope;
- escaped status path and same-ID polling without a second POST;
- queued, processing, completed, failed, and canceled normalization;
- nested, multi-result, relative, and cross-origin signed asset URLs;
- authenticated content and thumbnail read-through, Range, MIME, and size limits;
- missing credentials, malformed envelopes, retryable poll failures, and
  sanitized customer-facing errors;
- no double billing, no poll billing, and no refund after accepted/ambiguous
  creation;
- SQLite, MySQL, and PostgreSQL-compatible persistence behavior.

Do not force provider behavior into generic production code merely to satisfy a
fixture. Fixtures must reflect an observed or documented contract.

### 9.2 Real provider probes

For each model intended for production, run the smallest valid paid task through
an isolated local candidate. At minimum record:

- internal test case ID;
- provider alias and model ID;
- media mode and non-secret parameters;
- one creation attempt count;
- accepted task ID presence, without publishing the full provider identifier;
- normalized status progression;
- output kind, MIME, duration, dimensions, and thumbnail availability;
- observed provider debit and YuAPI quota;
- Studio result, Infinite Canvas result, refresh recovery, and download result;
- pass/fail and operator notes.

Do not record response headers, credentials, cookies, raw account data, private
database rows, or credential-bearing asset URLs.

### 9.3 Local UI acceptance

Build the candidate from the production-derived branch and bind it only to a
private localhost port. Use Playwright screenshots and functional checks for:

- home page;
- sign-in and sign-up;
- console layout;
- API key page;
- system settings;
- Studio image and video views;
- Infinite Canvas;
- developer documentation;
- custom brand, animation, model selector, and task/result states.

The user must inspect the local candidate and explicitly approve it before any
server-side preparation or production routing change.

## 10. Production release and rollback

Production release is a separate approval gate, even after local acceptance.

1. Keep the current production container and rollback container untouched.
2. Build a uniquely tagged candidate from the reviewed branch.
3. Start it beside production on a new `127.0.0.1` private port.
4. Verify health, restart count, homepage assets, authenticated media catalog,
   one bounded task, task recovery, asset proxy, billing evidence, and database
   compatibility without changing Caddy.
5. Obtain explicit user approval for the traffic switch.
6. Update only the intended Caddy upstream references and use a graceful config
   reload. Do not stop the old production container first.
7. Recheck public health, UI, task creation/poll/result, logs by aggregate status,
   billing totals, and database error counters.
8. Keep the old image, container, Caddy backup, and rollback metadata until the
   observation period ends and the user explicitly approves cleanup.

Rollback immediately if any of the following appears:

- brand UI or protected pages differ unexpectedly;
- health fails or restart count increases;
- database migration/write errors occur;
- a creation POST is duplicated;
- task polling loses the accepted ID or cannot recover after refresh;
- assets or thumbnails expose private upstream URLs or fail authentication;
- billing is missing, duplicated, below provider cost, or applied to poll/read;
- existing image, GPT text, login, or console behavior regresses.

Rollback means restoring Caddy to the retained healthy application container.
It does not mean restoring an old database snapshot, deleting new transaction
data, resetting balances, or replacing MySQL/Redis.

## 11. Separate known GPT cache issue

The current GPT cache/account-pool investigation is not part of video-provider
integration and must not be mixed into that branch.

Current evidence shows that YuAPI can preserve explicit `prompt_cache_key` and
improve channel affinity, but it cannot pin an account inside an upstream-owned
account pool that does not expose account identity. The remaining design work is
to coordinate stable session/model affinity with the upstream pool while using
capacity-aware deterministic failover. It must not expose upstream account IDs
or keys, and it must not route every session to one account.

The `hyojoo` traffic analysis is a GPT text validation workload. Do not use it
as acceptance evidence for video integration and do not modify its cache path
from a video branch.

## 12. Next-window start instruction

The next task should begin with this instruction:

> Read `docs/superpowers/handoffs/2026-08-14-production-video-upstream-baseline.md`
> completely and treat it as the operational baseline. Start a new `codex/`
> branch from the commit containing that document. Do not touch production,
> Caddy, the database, billing, GPT relay/cache logic, or brand UI. First perform
> a read-only audit of each proposed video provider's current models, protocol,
> price unit, credentials boundary, concurrency, async status/result contract,
> and reference-media rules. Do not print or commit secrets. Present the audited
> provider matrix and patch order for approval before implementing one provider
> at a time. Every provider must pass automated contract tests, a bounded real
> generation, Studio/Infinite Canvas recovery, billing verification, local UI
> review, and a separate production approval gate.

## 13. Definition of done

A multi-provider video integration is complete only when:

- every enabled model has a current audited contract and explicit billing unit;
- every provider uses a clear channel/credential/failure boundary;
- no provider secret or private URL is exposed to the browser, logs, docs, or
  repository;
- create happens once, poll reuses the accepted ID, and refresh recovers results;
- real outputs play and download in Studio and Infinite Canvas;
- charges are exact, auditable, and never repeated by polling or asset reads;
- all relevant Go, frontend, i18n, build, browser, and cross-database tests pass;
- protected production UI and unrelated GPT/image/auth behavior remain intact;
- the user approves the local UI and then separately approves production;
- rollback remains immediately available after the switch.
