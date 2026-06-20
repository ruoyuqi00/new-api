# GPT-Only NewAPI Bridge Layout - 2026-06-17

This note narrows the bridge plan to the current operating goal: expose only the GPT product line first, keep all other families reserved for later, and let NewAPI stay user-facing while Sub2API stays the internal supply/scheduler layer.

Do not paste real API keys, refresh tokens, upstream keys, or account JSON into this file.

## Current Decision

For now, only the GPT line should be connected to NewAPI.

Other families such as Kiro, Windsurf, Opus, Grok, Gemini, and image-only products are not abandoned. They should simply stay out of the current NewAPI bridge until they are intentionally created as separate product lines.

Update on 2026-06-18: the single `gpt` bridge described in the original
2026-06-17 notes has been superseded by three GPT tiers:

- `gpt-team`
- `gpt-plus`
- `gpt-pro`

Sub2API is still the internal scheduler/supply layer. The change is not a
retirement of Sub2API; it is a cleanup that makes each NewAPI user-visible tier
map to a dedicated Sub2API bridge key and supply group.

## Target Shape

```text
User
  -> NewAPI user key
  -> NewAPI user-visible group: gpt-team / gpt-plus / gpt-pro
  -> NewAPI admin-only bridge channel:
       sub2api-gpt-team / sub2api-gpt-plus / sub2api-gpt-pro
  -> Sub2API internal bridge key:
       newapi-bridge-gpt-team / newapi-bridge-gpt-plus / newapi-bridge-gpt-pro
  -> Sub2API GPT supply group:
       gpt-team / gpt-plus / gpt-pro
  -> Sub2API account scheduler / upstream fallback
```

The user should only see the NewAPI-side product naming, model list, quota, and price. The Sub2API bridge key, bridge group, account pool, and upstream API fallback are internal supply-layer details.

## Two-Sided Change

Yes, the bridge should be configured on both sides:

- NewAPI owns the user-facing product layer:
  - user-visible group, for example `gpt`;
  - user key issuance, quota, package, and price display;
  - admin-only upstream channel, for example `sub2api-gpt`;
  - public model list shown to users.
- Sub2API owns the internal supply layer:
  - dedicated bridge key, for example `newapi-bridge-gpt`;
  - GPT supply group/account pool;
  - model mapping, scheduler behavior, fallback accounts, risk logs, and account health;
  - optional channel-level pricing/restriction used by the adapter layer.

Do not try to make the NewAPI group automatically become a Sub2API group. The mapping happens through the Sub2API bridge key that NewAPI uses for that upstream channel.

## Current Production Snapshot

Read-only snapshot taken on 2026-06-17:

- Existing Sub2API channels are currently named for Windsurf product lines:
  - `channel-windsurf-opus4.6`
  - `channel-windsurf-opus4.7`
  - `channel-windsurf-gpt5.5`
  - `channel-windsurf-gpt5.4`
  - `channel-windsurf-grok`
- Those channels are attached to `windsurf-*` Anthropic-platform groups.
- The current GPT account pool is stored in the historical `GPT5.5` group, group id `8`, platform `openai`.
- The `GPT5.5` group name should be treated as a current pool name, not as a promise that the pool can only serve `gpt-5.5`.
- `GPT5.5` currently has active OpenAI supply capacity and several historical/deleted accounts.
- `GPT5.5` currently has multiple active API keys, but their names look historical/manual rather than dedicated NewAPI bridge keys.
- `GPT5.5` is not currently attached to any Sub2API channel in `channel_groups`.
- `kiro-gpt5.5` exists but is disabled and should not be used for the current GPT bridge.

Operational interpretation:

- Treat `GPT5.5` as the current GPT supply group, even though the eventual cleaner name should be `supply-gpt`.
- Do not mix NewAPI GPT traffic into the existing `windsurf-*` channels.
- Do not reuse historical/manual keys as the long-term NewAPI bridge key.
- Create one dedicated Sub2API key for NewAPI, bound to `GPT5.5`, and name it clearly.

## GPT Model Scope

The GPT supply group should support multiple GPT-family models, not only `gpt-5.5`.

Recommended rule:

```text
NewAPI gpt product line
  -> one Sub2API GPT bridge key
  -> one Sub2API GPT supply group
  -> many GPT-family model names
```

Examples of models that may belong to the same GPT product line if the account pool and mapping support them:

- `gpt-5.5`
- `gpt-5.5-pro`
- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.4-pro`
- `gpt-5.3-codex`
- future GPT models that are intentionally tested and enabled.

Model availability is controlled by three things:

1. NewAPI channel `models`: what users are allowed to request.
2. Sub2API channel pricing/mapping/restriction: how requested model names are mapped and billed.
3. Sub2API account pool capabilities: whether selected accounts can actually serve the mapped model.

If a model is visible in NewAPI but no account in the Sub2API GPT pool can serve it, users will still see runtime failures. Add each GPT model only after one live call has been tested through the bridge.

## Recommended Names

Use stable, boring names so future expansion stays readable:

| Layer | Name | Visibility | Purpose |
| --- | --- | --- | --- |
| NewAPI group | `gpt-team` / `gpt-plus` / `gpt-pro` | User-visible | Product tiers users select/use. |
| NewAPI channel | `sub2api-gpt-team` / `sub2api-gpt-plus` / `sub2api-gpt-pro` | Admin-only | Routes each NewAPI tier to Sub2API. |
| Sub2API API key | `newapi-bridge-gpt-team` / `newapi-bridge-gpt-plus` / `newapi-bridge-gpt-pro` | Internal only | Bridge credentials used only by NewAPI. |
| Sub2API group | `gpt-team` / `gpt-plus` / `gpt-pro` | Internal/supply | GPT account/upstream pool boundary for each tier. |
| Sub2API channel | `channel-newapi-gpt-team` / `channel-newapi-gpt-plus` / `channel-newapi-gpt-pro` | Internal/admin | Channel-level pricing/mapping/restriction per tier. |

The historical `GPT5.5` group remains as a legacy source pool. Its accounts
were linked into the three new groups during the transition so the new tiers do
not start empty. Future upstream capacity should be imported directly into the
intended tier group.

## Setup Steps

1. In Sub2API, create or confirm `gpt-team`, `gpt-plus`, and `gpt-pro`.
2. In Sub2API, create one bridge key per tier and bind it to the matching group.
3. In Sub2API, create one channel per tier only if channel-level pricing, visible supported models, model restrictions, or model mapping are needed now.
   - Attach each channel only to its matching group.
   - Add every GPT model that NewAPI will expose, not just `gpt-5.5`.
   - Keep `restrict_models` enabled only if the intended GPT model list is complete and tested.
   - Keep model mapping stable so NewAPI can expose your chosen model names.
4. In NewAPI, create one channel per tier.
   - `base_url`: Sub2API OpenAI-compatible base URL, preferably internal network URL if NewAPI and Sub2API share a Docker network; otherwise use the public HTTPS Sub2API endpoint.
   - `key`: the matching `newapi-bridge-gpt-*` Sub2API key.
   - `models`: all GPT names users should see/use now, and only those already tested through the bridge.
   - `group`: only the matching NewAPI tier group.
5. In NewAPI, give users NewAPI keys assigned to the intended tier.

## What Not To Do Yet

- Do not expose Sub2API keys directly to users.
- Do not bind NewAPI `gpt` traffic to `windsurf-*` groups.
- Do not mix future Opus/Grok/Gemini traffic into the GPT NewAPI channel.
- Do not delete existing non-GPT channels during this cleanup; leave them parked until each family is intentionally rebuilt.
- Do not restart NewAPI just to document or inspect this mapping.

## Future Expansion Pattern

When adding another product family later, copy the same pattern:

```text
NewAPI user-visible group
  -> NewAPI admin-only channel
  -> dedicated Sub2API bridge key
  -> dedicated Sub2API supply group
  -> dedicated Sub2API channel/mapping/pricing
```

Examples:

- `opus` -> `sub2api-opus` -> `newapi-bridge-opus` -> `supply-opus`
- `grok` -> `sub2api-grok` -> `newapi-bridge-grok` -> `supply-grok`
- `gemini` -> `sub2api-gemini` -> `newapi-bridge-gemini` -> `supply-gemini`
- `image` -> `sub2api-image` -> `newapi-bridge-image` -> `supply-image`

Each product family gets its own bridge key so NewAPI group-level routing maps cleanly back to the intended Sub2API supply pool.

## Quick Verification

For the GPT-only bridge, verify:

- NewAPI user key belongs to NewAPI group `gpt`.
- NewAPI channel `sub2api-gpt` is enabled for group `gpt`.
- NewAPI channel key is the dedicated Sub2API key `newapi-bridge-gpt`.
- Sub2API key `newapi-bridge-gpt` is bound to group `GPT5.5`.
- `GPT5.5` has active OpenAI accounts that can serve every GPT model exposed by NewAPI.
- A request from a NewAPI user key creates usage on the NewAPI side and reaches Sub2API with the bridge key.
- Sub2API usage/risk logs show the bridge key and group `GPT5.5`.
- Each exposed GPT model has at least one successful bridge smoke test.

If any of these fail, do not add more product families yet. Fix the GPT bridge first.

## Production Configuration Applied

Applied on 2026-06-17 after the first GPT-only routing decision:

- Sub2API has a dedicated internal API key named `newapi-bridge-gpt`, bound to group `GPT5.5` / id `8`.
- Sub2API has `channel-newapi-gpt`, active, attached only to `GPT5.5`.
- `channel-newapi-gpt` pricing/model scope is OpenAI token billing for:
  - `gpt-5.5`
  - `gpt-5.4`
  - `gpt-5.4-mini`
  - `gpt-5.3-codex`
  - `gpt-5.3-codex-spark`
  - `gpt-5.2`
- NewAPI now has channel `sub2api-gpt`.
  - Base URL: internal Docker network URL `http://sub2api:8080`.
  - NewAPI group: `gpt`.
  - Model list: the six GPT models above.
  - Tag: `bridge-gpt`.
- Existing active NewAPI tokens in group `gpt` were updated from single-model `gpt-5.5` visibility to the six-model GPT list above.
- NewAPI channel cache refresh returned HTTP 200.
- NewAPI was not restarted.

Operational verification:

- `GET /v1/models` with an existing NewAPI `gpt` token returned all six GPT model ids.
- A tiny `gpt-5.4-mini` chat smoke no longer fails on NewAPI disk pressure after Docker build cache cleanup.
- The current chat smoke reached the bridge but returned `Upstream authentication failed, please contact administrator`, so the remaining failure is upstream GPT supply/account health, not NewAPI model visibility.
- Root filesystem was at 97% and NewAPI returned `system_disk_overloaded`; `docker builder prune -f` safely reclaimed build cache and lowered `/` usage to 89%.

## Production Reorg Applied - 2026-06-18

The 2026-06-17 single `gpt` NewAPI bridge was replaced with the tiered GPT
bridge:

| NewAPI group | NewAPI channel | Sub2API key name | Sub2API group | Sub2API channel |
| --- | --- | --- | --- | --- |
| `gpt-team` | `sub2api-gpt-team` | `newapi-bridge-gpt-team` | `gpt-team` | `channel-newapi-gpt-team` |
| `gpt-plus` | `sub2api-gpt-plus` | `newapi-bridge-gpt-plus` | `gpt-plus` | `channel-newapi-gpt-plus` |
| `gpt-pro` | `sub2api-gpt-pro` | `newapi-bridge-gpt-pro` | `gpt-pro` | `channel-newapi-gpt-pro` |

NewAPI visible groups are now only:

- `gpt-team`
- `gpt-plus`
- `gpt-pro`

Disabled NewAPI channels:

- `sub2api-gpt`
- `external-gpt-upstream-s2cf`
- `Sub2API GPT5.5 image2 upstream`

The direct external GPT upstream channel was intentionally disabled because
user traffic should reach upstream supply through Sub2API, not through direct
NewAPI channels. Add upstream API keys and base URLs as Sub2API OpenAI API-key
accounts, then bind those accounts to `gpt-team`, `gpt-plus`, or `gpt-pro`.

Existing NewAPI users and GPT tokens were moved to `gpt-team` to preserve the
base tier while hiding historical temporary groups.

## External GPT Upstream Added

Applied on 2026-06-17 after the user provided an OpenAI-compatible upstream API endpoint for the first external GPT supply channel.

Important: the upstream API key is intentionally not recorded in this repository. Do not add it to docs, commits, issue comments, screenshots, or shell transcripts.

NewAPI production channel:

- Channel id: `2292`.
- Channel name: `external-gpt-upstream-s2cf`.
- Channel type: OpenAI-compatible.
- Base URL root: `https://s2cf.c5mc.cn`.
- NewAPI group: `gpt`.
- Tag: `external-gpt-s2cf`.
- Priority: `120`.
- Weight: `80`.
- Auto-ban: disabled for this channel so an upstream-side incident does not silently remove the external fallback without operator review.
- NewAPI was not restarted.

Enabled models on this external GPT channel:

- `gpt-5.5`
- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.3-codex`
- `codex-auto-review`

Verification performed from the production server:

- Direct upstream `/v1/models` returned HTTP 200 from the server, while the local Windows machine was blocked by upstream Cloudflare policy.
- Direct upstream `gpt-5.4-mini` chat smoke returned HTTP 200 with `OK`.
- NewAPI `channel/fix` returned HTTP 200 with `success: 3`, `fails: 0`, refreshing channel abilities/cache without restarting NewAPI.
- Forced NewAPI request through channel `2292` with `gpt-5.4-mini` returned HTTP 200 with `OK`.
- Normal NewAPI `gpt` group request with `gpt-5.5` returned HTTP 200 with `OK`.

Operational interpretation:

- The user-facing product group remains only `gpt`; this external upstream is an admin-only supply channel behind that group.
- NewAPI can now route the `gpt` group through both the Sub2API bridge channel and the external OpenAI-compatible channel.
- Do not create a separate user-visible group for this upstream unless it becomes a distinct product/package later.
- When another upstream is added, use the same pattern: a dedicated NewAPI channel, a clear tag, only the intended user-visible group, and at least one successful smoke test for every exposed model before making it available to users.

## NewAPI Pricing Alignment - 2026-06-20

NewAPI is the user-facing billing plane. Sub2API is the supply/scheduler plane.
Changing Sub2API channel pricing alone does not change what NewAPI users see or
what NewAPI deducts from user quota.

On 2026-06-20 the NewAPI GPT model pricing was aligned with the Sub2API GPT
fallback pricing for the six exposed GPT models. The change was applied through
NewAPI's `/api/option/` root setting API so the running process picked up the
new values without a NewAPI restart.

Backup before the edit:

- NewAPI options table: `/opt/newapi/backups/pricing-align-20260620-181134.sql`

Models aligned:

| Model | Input $/MTok | Output $/MTok | Cache read $/MTok | Cache write $/MTok |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.5` | 2.5 | 15 | 0.25 | 2.5 |
| `gpt-5.4` | 2.5 | 15 | 0.25 | 2.5 |
| `gpt-5.4-mini` | 0.75 | 4.5 | 0.075 | 0.75 |
| `gpt-5.3-codex` | 1.75 | 14 | 0.175 | 1.75 |
| `gpt-5.3-codex-spark` | 1.25 | 10 | 0.125 | 1.25 |
| `gpt-5.2` | 1.75 | 14 | 0.175 | 1.75 |

NewAPI stores these as legacy ratio maps:

- `ModelRatio = input_price_per_mtok / 2`
- `CompletionRatio = output_price_per_mtok / input_price_per_mtok`
- `CacheRatio = cache_read_price_per_mtok / input_price_per_mtok`
- `CreateCacheRatio = cache_write_price_per_mtok / input_price_per_mtok`

Validation after the change:

- `GET http://127.0.0.1:3001/api/pricing` returned the six GPT models with the
  input/output/cache read/cache write prices above.
- NewAPI was not restarted.

Important: NewAPI `GroupRatio` still applies on top of the base model price.
For example, if `gpt-pro` remains `1.8`, the user-facing `gpt-pro` price will be
1.8x the base price. This is a product-tier markup, not a Sub2API supply-price
setting. If the public price should exactly equal Sub2API base pricing for every
group, set the corresponding NewAPI group ratios to `1`.
