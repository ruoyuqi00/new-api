# End-to-End Bridge And Capacity Map - 2026-06-17

This note summarizes the full operating chain in one place:

- the user-facing site;
- the Sub2API supply layer;
- the upstream bridge layer;
- and the controls that keep future account growth and concurrency manageable.

Do not record real API keys, refresh tokens, passwords, or full account payloads here.

## 1. Current Live Chain

```text
User
  -> NewAPI user key
  -> NewAPI user-visible group: gpt-team / gpt-plus / gpt-pro
  -> NewAPI dedicated bridge channel:
       sub2api-gpt-team / sub2api-gpt-plus / sub2api-gpt-pro
  -> Sub2API internal bridge key:
       newapi-bridge-gpt-team / newapi-bridge-gpt-plus / newapi-bridge-gpt-pro
  -> Sub2API GPT supply group:
       gpt-team / gpt-plus / gpt-pro
  -> Sub2API channel / scheduler / account pool
  -> upstream account or upstream OpenAI-compatible API account
```

This is the important rule:

- NewAPI is the public product/control plane.
- Sub2API is the internal supply/scheduler plane.
- The bridge keys are internal only.
- Users should only see the NewAPI-facing product groups and model lists.
- Upstream provider API keys should be imported into Sub2API accounts, not exposed as direct NewAPI channels.

## 2. Upstream Bridge Shape

There are two distinct bridge directions:

1. NewAPI -> Sub2API
   - used for the public GPT product line;
   - maps user traffic into the internal Sub2API supply group.

2. Sub2API -> upstream provider
   - used when Sub2API itself needs to fan out to Kiro, Windsurf, OpenAI-compatible upstreams, or other provider pools;
   - keeps account pools and provider quirks hidden from NewAPI users.

Do not collapse these into one shared channel. Keep the public bridge and the upstream bridge separate so failures stay easier to isolate.

## 3. Stability Controls

The shortest path to safer growth is:

- one customer, app, or reseller per API key;
- one intended group per channel;
- one family per supply pool;
- separate bridge keys per family;
- separate fallback channels per family.

Operational control points:

- per-key quotas;
- per-group RPM limits;
- Redis-backed or persisted cooldown state;
- scheduler snapshots and account health state;
- key-level auto-disable or quarantine for abusive downstream keys;
- hard-dead upstream credential deletion for terminal failures only;
- soft cooldown for quota, rate-limit, or overage text.

## 4. What Is Already In Place

- Kiro hard-dead credential deletion is live.
- The Sub2API and NewAPI containers are healthy on the server.
- Docker data root has already been moved off the tight root disk.
- GPT bridge wiring has been split into three user-facing NewAPI tiers:
  `gpt-team`, `gpt-plus`, and `gpt-pro`.
- NewAPI now only has those three GPT bridge channels enabled for this rollout.
- The previous single `sub2api-gpt` channel, direct external GPT upstream channel,
  and image bridge channel are disabled in NewAPI.
- Sub2API has matching internal supply groups, channels, and bridge keys for
  `gpt-team`, `gpt-plus`, and `gpt-pro`.
- The historical `GPT5.5` pool remains available as the source pool; its accounts
  were temporarily linked into the three new Sub2API groups so traffic does not
  hit an empty pool while dedicated upstreams are attached.
- Sub2API now has a read-only admin route preview endpoint for checking
  `group_id + platform + model` before exposing a model through NewAPI or
  adding a new upstream fallback.

## 5. What Still Needs Load-Scale Hardening

- repeatable load tests for auth, scheduler, and hot gateway paths;
- p50/p95/p99 latency and error-rate tracking;
- stronger Redis-backed account-health and cooldown behavior;
- aggregate reporting for risky keys, groups, models, and endpoints;
- clearer quarantine state for risky downstream keys;
- more explicit per-family fallback policy for future growth.

## 6. Expansion Rule

When a new product family is added later, use the same pattern:

```text
NewAPI user group
  -> NewAPI channel
  -> dedicated Sub2API bridge key
  -> dedicated Sub2API supply group
  -> dedicated upstream fallback or account pool
```

That keeps GPT, Opus, Grok, Kiro, Gemini, and image traffic separable instead of turning into one mixed pool.

For the current GPT rollout, use the same pattern per tier:

```text
gpt-team -> sub2api-gpt-team -> newapi-bridge-gpt-team -> gpt-team
gpt-plus -> sub2api-gpt-plus -> newapi-bridge-gpt-plus -> gpt-plus
gpt-pro  -> sub2api-gpt-pro  -> newapi-bridge-gpt-pro  -> gpt-pro
```

Attach new upstream API accounts in Sub2API by binding them to the intended
Sub2API group. Do not add direct user-facing upstream channels in NewAPI unless
the product strategy intentionally changes.

## 7. Multi-Upstream Concurrency Rule

Multiple upstream accounts or upstream OpenAI-compatible API accounts can share
traffic, but only inside the same matching supply pool.

For GPT text tiers, the intended shape is:

```text
NewAPI visible group: gpt-team / gpt-plus / gpt-pro
  -> one hidden NewAPI bridge channel for that tier
  -> one Sub2API bridge key for that tier
  -> one Sub2API supply group for that tier
  -> many active, schedulable upstream accounts in that same group
```

Sub2API does not send one user request to every upstream. It selects one
eligible account per request. Under concurrency, the scheduler spreads requests
across all eligible accounts in the supply group, using the account status,
`schedulable`, cooldown fields, requested-model support, `priority`,
`concurrency`, `load_factor`, current Redis load, and recent-use state.

Operational rules:

- Put same-tier upstreams in the same Sub2API group if they should share user
  load.
- Keep model families separated. Do not mix normal GPT text, GPT image, Grok,
  Gemini, Opus, Kiro, and video capacity into one pool.
- Set `concurrency` to the real simultaneous request capacity for that upstream.
  This controls the hard account slot limit.
- Set `load_factor` only when the account should carry more or less scheduling
  weight than its raw `concurrency`.
- Keep `priority` aligned among peers that should share load. A lower priority
  number is preferred, so a much better priority can starve fallback accounts.
- Make model mappings consistent across all upstreams in the same pool. If one
  account cannot support a model, it will be skipped for that model.
- Do not expose upstream provider API keys as user-facing NewAPI channels. They
  belong in Sub2API supply accounts; NewAPI should expose only the product
  groups users buy or receive.

For image/video, UAG has its own account group scheduler. Multiple UAG provider
accounts in the same UAG account group can share image-site traffic when they
are enabled, have the right model whitelist, and are not cooled down. When UAG
bridges through NewAPI/Sub2API, that bridge account is only one UAG upstream
unless more UAG accounts are added to the same UAG group.

## 8. Production Reorg Snapshot - 2026-06-18

Backups created before the production database edits:

- NewAPI: `/opt/newapi/backups/channel-reorg-20260618-101100.sql`
- Sub2API: `/opt/sub2api/backups/channel-reorg-20260618-101138.sql`

NewAPI enabled channels after the reorg:

| Channel | Group | Base URL | Models |
| --- | --- | --- | --- |
| `sub2api-gpt-team` | `gpt-team` | `http://sub2api:8080` | GPT six-model list |
| `sub2api-gpt-plus` | `gpt-plus` | `http://sub2api:8080` | GPT six-model list |
| `sub2api-gpt-pro` | `gpt-pro` | `http://sub2api:8080` | GPT six-model list |

Disabled NewAPI channels:

- `sub2api-gpt`
- `external-gpt-upstream-s2cf`
- `Sub2API GPT5.5 image2 upstream`

NewAPI options were reduced to:

- `GroupRatio`: `gpt-team`, `gpt-plus`, `gpt-pro`
- `UserUsableGroups`: `GPT Team`, `GPT Plus`, `GPT Pro`
- `AutoGroups`: empty
- `TopupGroupRatio`: `gpt-team`, `gpt-plus`, `gpt-pro`

Existing NewAPI users and active GPT tokens were migrated to `gpt-team` so
existing keys keep using the base GPT tier instead of orphaned historical
groups.

Sub2API internal objects:

| Tier | Group | Channel | Bridge key name |
| --- | --- | --- | --- |
| Team | `gpt-team` | `channel-newapi-gpt-team` | `newapi-bridge-gpt-team` |
| Plus | `gpt-plus` | `channel-newapi-gpt-plus` | `newapi-bridge-gpt-plus` |
| Pro | `gpt-pro` | `channel-newapi-gpt-pro` | `newapi-bridge-gpt-pro` |

The three Sub2API groups are OpenAI-platform, active, and exclusive. They
currently share the historical GPT account pool as a transitional capacity
source. When adding a real upstream for a tier, import it as a Sub2API OpenAI
API-key account and bind it to only that tier's group.

## 9. Runtime Storage Snapshot - 2026-06-20

Runtime storage was adjusted so future image pulls/builds have room:

- Docker data-root: `/www/docker`
- containerd root: `/www/containerd`
- root disk after migration: about 57% used, about 8.6G free
- `/www` after migration: about 26% used, about 23G free
- verification after migration: Docker and containerd active; Sub2API, NewAPI,
  UAG, Redis, MySQL, and Postgres containers running/healthy where health checks
  exist.

Future Sub2API builds should still prefer local build plus image upload when
possible, because the server root disk remains modest. If building on the
server, check `/`, `/www`, DockerRootDir, and containerd root first.

## 10. Live Supply Snapshot - 2026-06-20

Read-only production check after the storage migration:

| Sub2API group | Active schedulable supply | Notes |
| --- | ---: | --- |
| `gpt-team` | 0 accounts | Empty pool. Do not sell or route base-tier traffic here until a team upstream is attached, or explicitly make a temporary cross-tier capacity decision. |
| `gpt-plus` | 2 OpenAI-compatible API accounts | Both active and schedulable; configured concurrency sum was 20. |
| `gpt-pro` | 2 OpenAI-compatible API accounts | Both active and schedulable; configured concurrency sum was 10010 because one upstream was configured with a very high hard concurrency. |
| `image-gpt` | 0 accounts | Empty pool. GPT image traffic should not depend on this group until an image-capable upstream is attached and smoke-tested. |
| `grok` | 0 active schedulable accounts | One linked account existed but was `error` and not schedulable. Replace or fix the upstream key/base URL before exposing Grok. |
| `gemini` | 0 accounts | Empty pool. Add a Gemini upstream before exposing Gemini text or image capabilities. |

This means the multi-upstream scheduling mechanism is ready, but current live
capacity is only present for `gpt-plus` and `gpt-pro`. `gpt-team`, `image-gpt`,
`grok`, and `gemini` require upstream attachment or repair before they can carry
normal user traffic.

If the intent is for every configured pro upstream to carry some traffic instead
of letting the high-concurrency upstream dominate, set `load_factor` separately
from `concurrency`. Keep `concurrency` as the hard slot limit, and use
`load_factor` as the scheduling weight.

## 11. Admin Route Preview

Use the Sub2API admin route preview before changing the user-visible NewAPI
model list, creating a new bridge key, or attaching a new upstream fallback.

```text
GET /api/v1/admin/channels/route-preview?group_id=<id>&platform=<platform>&model=<model>
```

The endpoint is read-only and does not call any upstream provider. It reports:

- the group platform loaded from the channel cache;
- the active channel attached to the group;
- channel-level model mapping;
- the model used for channel restriction/pricing checks;
- whether the channel would directly restrict the request;
- the matching channel pricing entry, if any;
- warnings such as:
  - `no_active_channel_for_group`;
  - `requested_platform_differs_from_group_platform`;
  - `no_channel_pricing_for_restriction_model`;
  - `model_restricted_by_channel`;
  - `requires_account_level_restriction_check`.

For `billing_model_source=upstream`, the preview intentionally marks
`requires_account_level_restriction_check`, because the final restriction model
depends on the account-level upstream mapping selected by the scheduler.
