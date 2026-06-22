# Channel, Group, And User-Key Mapping Runbook - 2026-06-17

This note records how a new upstream/provider channel is created and how it is mapped back to the groups that users can select. It covers both the native Sub2API admin flow and the NewAPI-fronted flow.

Do not record real API keys, refresh tokens, account payloads, or passwords in this file.

## Short Answer

In Sub2API, create the upstream route in:

```text
Admin UI -> Channels -> Channel Pricing -> Create Channel
```

The backing API is:

```text
POST /api/v1/admin/channels
PUT  /api/v1/admin/channels/:id
```

The channel maps to user-callable traffic through this chain:

```text
channel -> channel_groups -> group_id
user api key -> api_keys.group_id
gateway request -> apiKey.GroupID -> channel cache -> model mapping/pricing -> account scheduler
```

The user-visible "Available Channels" page is only a display surface. Real routing is decided by the API key's `group_id`.

## Native Sub2API Configuration Order

Use this order when Sub2API itself is the control plane:

1. Create or confirm the target groups.
   - Keep platform boundaries clean. OpenAI/Codex accounts should be in OpenAI groups, Kiro/Windsurf/Anthropic-compatible accounts in their own matching platform groups, and Gemini in Gemini groups.
   - A group is the scheduler and account-pool boundary.

2. Import upstream accounts and bind them to the target groups.
   - Channel binding alone does not create upstream capacity.
   - If a group has no active/schedulable accounts, requests can still fail even when the channel is correctly configured.

3. Create or edit the channel in `Admin -> Channels -> Channel Pricing`.
   - Enable the platform section.
   - Select the groups that should use this channel.
   - Configure model pricing.
   - Configure model mapping, for example `client-model -> upstream-model`.
   - Choose `billing_model_source` carefully:
     - `requested`: bill by the client-requested model.
     - `upstream`: bill by the actual upstream model.
     - `channel_mapped`: bill by the channel-mapped model.
   - Enable `restrict_models` only when requests outside the channel pricing/mapping list should be blocked.

4. Grant users access to those groups.
   - Standard public non-exclusive groups are bindable by users.
   - Exclusive groups require explicit user `allowed_groups`.
   - Subscription groups require an active subscription for that group.

5. Users create API keys bound to the group.
   - This is the step that makes requests route to the intended group and channel.
   - An API key without the intended `group_id` will not use that group's channel/account pool.

6. Verify with a real request.
   - Check that the API key has the expected group.
   - Check that the group has active/schedulable accounts.
   - Check usage logs for `group_id`, `channel_id`, and `model_mapping_chain`.
   - If risk control fires, use the Risk Control log `api_key_id` filter.

## What The Code Does

Important Sub2API code paths:

- `backend/internal/handler/admin/channel_handler.go`
  - `createChannelRequest` accepts `group_ids`, `model_pricing`, `model_mapping`, `billing_model_source`, `restrict_models`, and `features_config`.
  - `POST /api/v1/admin/channels` creates the channel.
  - `PUT /api/v1/admin/channels/:id` updates the channel.

- `backend/internal/repository/channel_repo.go`
  - `Create` and `Update` write the channel record.
  - `setGroupIDsTx` writes the `channel_groups` mapping.
  - `GetChannelIDByGroupID` and `GetGroupsInOtherChannels` enforce/inspect group-channel ownership.

- `backend/internal/service/channel_service.go`
  - One group can belong to only one channel. `checkGroupConflicts` rejects duplicate channel ownership.
  - Channel cache is keyed by group and model:
    - `channelByGroupID[groupID]`
    - `pricingByGroupModel[(groupID, platform, model)]`
    - `mappingByGroupModel[(groupID, platform, model)]`
  - Cache TTL is 10 minutes, but create/update/delete invalidates and rebuilds it immediately.
  - Channel updates also invalidate API-key auth cache for affected groups.

- `backend/internal/service/api_key_service.go`
  - `GetAvailableGroups` returns groups a user can bind.
  - `Create` and `Update` validate that the user may bind the requested `group_id`.

- Gateway handlers and services
  - OpenAI, Anthropic-compatible, Gemini, and image paths resolve the channel with `apiKey.GroupID`.
  - Model mapping and restriction are resolved before forwarding.
  - Account selection uses the same group boundary.

- `backend/internal/handler/available_channel_handler.go`
  - `GET /api/v1/channels/available` is opt-in behind `available_channels_enabled`.
  - It filters channels to groups the user can access.
  - It is a visibility feature, not the routing authority.

## NewAPI In Front Of Sub2API

When NewAPI is the public user/admin panel and Sub2API is the adapter/scheduler upstream, there are two separate group systems:

```text
NewAPI group -> NewAPI channel -> Sub2API base_url + Sub2API key -> Sub2API key group_id -> Sub2API channel/account pool
```

The NewAPI channel's `group` field only controls which NewAPI groups may use that NewAPI channel. It does not automatically pass a NewAPI group into Sub2API as `group_id`.

Recommended mapping patterns:

### Pattern A - One Shared Pool

Use this only when several NewAPI groups should share the same Sub2API group/account pool.

1. In Sub2API, create one group, bind accounts, attach that group to a channel, and create one Sub2API API key bound to that group.
2. In NewAPI, create one upstream channel:
   - `base_url`: Sub2API base URL.
   - `key`: the Sub2API API key from step 1.
   - `models`: the model names users may request.
   - `group`: one or more NewAPI groups allowed to use this channel.
3. All selected NewAPI groups will route into the same Sub2API group.

### Pattern B - Separate Pools Per User Tier Or Reseller

Use this when NewAPI groups must map to different Sub2API groups, quotas, account pools, pricing, or risk policies.

For every mapping target:

1. In Sub2API, create a group.
2. Bind the intended accounts to that Sub2API group.
3. Attach that Sub2API group to the intended Sub2API channel.
4. Create a Sub2API API key bound to that Sub2API group.
5. In NewAPI, create a separate channel for the matching NewAPI group:
   - `base_url`: Sub2API base URL.
   - `key`: the group-bound Sub2API key from step 4.
   - `models`: the model names exposed to that NewAPI group.
   - `group`: only the NewAPI group(s) that should use this Sub2API pool.

Do not put several NewAPI groups with different intended Sub2API pools into one NewAPI channel. One NewAPI channel has one upstream key, so it can only hit the one Sub2API group bound to that key.

### Current GPT Tier Mapping

Production was reorganized on 2026-06-18 so NewAPI exposes only three GPT
product groups during the first rollout:

```text
NewAPI gpt-team -> NewAPI sub2api-gpt-team -> Sub2API key newapi-bridge-gpt-team -> Sub2API group gpt-team
NewAPI gpt-plus -> NewAPI sub2api-gpt-plus -> Sub2API key newapi-bridge-gpt-plus -> Sub2API group gpt-plus
NewAPI gpt-pro  -> NewAPI sub2api-gpt-pro  -> Sub2API key newapi-bridge-gpt-pro  -> Sub2API group gpt-pro
```

For new GPT upstream capacity, create or import OpenAI-compatible API-key
accounts in Sub2API and bind them to the intended group:

- team upstreams -> `gpt-team`
- plus upstreams -> `gpt-plus`
- pro upstreams -> `gpt-pro`

Do not create direct NewAPI upstream channels for these providers. NewAPI should
remain the user-facing product layer; Sub2API should own provider supply,
account health, scheduler behavior, and later fallback policy.

The old NewAPI `gpt` group and `sub2api-gpt` channel are disabled for this
rollout. Historical tokens were moved to `gpt-team` to preserve service.

## Verification Checklist

After creating a channel/upstream, verify:

- Admin channel list shows the expected group count.
- Channel detail contains the expected `group_ids`.
- No selected group is already attached to another channel.
- The user can bind an API key to the intended group.
- The API key list shows the expected group/group ID.
- The bound group has active/schedulable accounts for the platform.
- The requested model is either directly priced/supported or mapped by channel mapping.
- If `restrict_models` is enabled, the requested model appears in the channel pricing list or matches a configured wildcard.
- Run the read-only Sub2API route preview before exposing the model:
  `GET /api/v1/admin/channels/route-preview?group_id=<id>&platform=<platform>&model=<model>`.
- Usage logs show the expected `group_id`, `channel_id`, and `model_mapping_chain`.
- In a NewAPI-fronted setup, the NewAPI channel uses the correct Sub2API key for the intended Sub2API group.

## Common Failure Modes

- User can see a channel but calls route incorrectly:
  - The API key is not bound to the expected group.
  - The user-visible available-channel page does not control routing.

- Channel exists but requests fail:
  - No usable accounts are bound to the group.
  - Accounts are disabled, rate-limited, invalidated, or unschedulable.
  - Requested model is not supported by the account pool.

- Create/update channel returns group conflict:
  - The group already belongs to another channel. Remove it from the old channel first, or use a new group.

- NewAPI group separation does not work:
  - Multiple NewAPI groups are sharing one NewAPI upstream channel and therefore one Sub2API key.
  - Create separate NewAPI channels, each with a Sub2API key bound to the intended Sub2API group.

- Calls are billed against an unexpected model:
  - Check `billing_model_source`.
  - Check channel-level mapping.
  - Check account-level model mapping; account mapping can still change the final upstream model.

## Admin Route Preview

Sub2API now includes an admin route preview tool:

```text
input: group_id + platform + requested model
output: channel_id, mapped model, restriction model, restrict decision, matching pricing, warnings
```

Use it before:

- adding a new model to a NewAPI user-facing group;
- creating a dedicated bridge key for a product family;
- attaching a Sub2API group to a new upstream fallback;
- changing channel mapping or `billing_model_source`.

The preview is intentionally read-only and does not call real upstreams. If it
returns `requires_account_level_restriction_check`, the channel is using
`billing_model_source=upstream`, so the final allow/deny decision depends on
the account-level upstream model selected by the scheduler.

## Capacity Notes

For the current GPT-first rollout, keep these practical rules:

- keep the public user-facing groups as `gpt-team`, `gpt-plus`, and `gpt-pro`;
- keep all Sub2API bridge keys internal only;
- do not reuse a bridge key across tiers or unrelated families;
- keep account growth separated from user-key growth so scheduler pressure stays visible;
- use group-level RPM / concurrency controls before expanding the pool size;
- treat rate-limit text as a cooldown signal, not a dead-account signal;
- treat terminal lock/suspension/verification text as a dead-account signal.

## External Proxy Upstream Migration - 2026-06-21

User question:

- If an upstream already exposes an OpenAI-compatible URL and API key, should
  the old Sub2API configuration be converted into NewAPI?
- Why did a direct upstream proxy cause wrong billing when a request should have
  been charged as one fixed-price image/video call?

Short answer:

- Do not bulk-convert all Sub2API configuration into NewAPI.
- Convert only plain OpenAI-compatible text/chat proxy upstreams into direct
  NewAPI channels.
- Keep real account pools, special protocol adapters, Kiro/Windsurf/Codex-like
  account scheduling, and fallback logic in Sub2API.
- Keep image and video on a separate per-call/task billing path. Do not expose
  image/video models through an ordinary text/chat passthrough channel unless a
  real image/video smoke test proves both output and billing behavior.

Current production observation:

| Area | Current state | Decision |
| --- | --- | --- |
| GPT tier pools | NewAPI groups `gpt-team`, `gpt-plus`, `gpt-pro` bridge to Sub2API keys/groups. | Keep this structure. Sub2API owns GPT supply and scheduler behavior. |
| Direct Gemini upstream | NewAPI has an OpenAI-compatible Gemini channel. Native Gemini placeholder is disabled. | Treat as text/chat only until image/video endpoints are proven. |
| Direct Grok upstream | NewAPI has a Grok/xAI channel. | Treat as text/chat only; retest before public expansion because the last smoke test returned upstream 500. |
| Sub2API Grok/Gemini groups | Sub2API has `grok` and `gemini` channels/groups, but they do not currently provide useful healthy account-pool capacity. | Do not route public NewAPI traffic through these just to wrap an external proxy. Direct NewAPI is shorter and easier to price. |
| Image site / UAG | Public image site currently exposes only `gpt-image-2`. | Keep only proven image models visible. Hide placeholders until real upstream accounts exist. |

Why direct proxy billing can go wrong:

- `/v1/chat/completions` and ordinary `/v1/responses` text calls are token
  billed. NewAPI reads upstream `usage` or estimates text tokens.
- `/v1/images/generations` in the OpenAI channel still expects image-compatible
  response/usage behavior. Some upstream proxy sites return non-standard usage
  or text-like output, so NewAPI may bill by token or parse usage incorrectly.
- Video is usually task based (`create -> poll -> fetch`) and should use
  NewAPI's task/per-call billing path, not a text/chat passthrough.
- `gpt-image-2` through Responses image generation was patched separately so
  image generation is billed as image-only/per-call instead of double-counting
  text usage. Do not assume other models have that guarantee.

Migration rule:

1. Classify the upstream by capability, not by provider name.
   - Text/chat: `/v1/chat/completions` returns OpenAI-compatible choices and
     sane usage.
   - Image: `/v1/images/generations` or a documented image task endpoint returns
     a real image URL/base64 and predictable per-call cost.
   - Image edit: `/v1/images/edits` or a documented edit endpoint accepts image
     input and returns a real result.
   - Video: documented create/poll/fetch contract and pricing unit are known.

2. For text/chat proxy upstreams:
   - Create or keep a direct NewAPI OpenAI-compatible channel.
   - Put only proven chat model IDs in `models`.
   - Configure NewAPI `ModelRatio`/`CompletionRatio` or `ModelPrice` according
     to the intended user-facing text pricing.
   - Run a non-stream and stream smoke test before adding the model to a public
     group.

3. For image/video:
   - Do not add the model to a public NewAPI text channel just because the
     upstream model name contains `image`, `grok-imagine`, `imagen`, `veo`, or
     similar words.
   - First run the exact endpoint that users or the image site will call.
   - Confirm the returned artifact is usable.
   - Confirm NewAPI usage logs show fixed image/task billing, not unexpected
     text token billing.
   - Only then expose it in NewAPI and/or UAG.

4. For Sub2API:
   - Keep it for internal supply pools, group-bound bridge keys, scheduler,
     cooldown/failover, and special protocol adapters.
   - Do not create a Sub2API layer for an external proxy if NewAPI can call the
     proxy directly and no extra scheduler behavior is needed.

Safe testing order for each new upstream:

1. Read `/v1/models` if supported and record only model IDs, not keys.
2. Test `/v1/chat/completions` non-stream with one cheap prompt.
3. Test `/v1/chat/completions` stream if the model will be offered to stream
   clients.
4. If it claims image support, test `/v1/images/generations` with the exact
   model ID.
5. If it claims video support, test the documented task endpoint and polling.
6. Inspect NewAPI usage logs for billing mode and quota delta.
7. Only then add the model to public user groups.

Operational decision:

- NewAPI is still the public URL/key/user/quota/pricing layer.
- Sub2API is still the internal account-pool/scheduler/protocol layer.
- External OpenAI-compatible proxy upstreams can be direct NewAPI channels when
  they are text/chat only.
- Image/video should be treated as a separate product capability, with its own
  per-call/task billing validation before exposure.

## NewAPI Upstream Routing/Billing Probe - 2026-06-21

User requirement:

- Treat the upstream as already configured correctly unless evidence shows
  otherwise.
- Diagnose our local channel/protocol/pricing configuration carefully.
- Do not copy upstream pricing; NewAPI must keep site-owned prices.

What was verified:

| Model/path | Result | Billing/config conclusion |
| --- | --- | --- |
| `gemini-2.5-flash` via `/v1/chat/completions` non-stream | HTTP 200. | Routed through NewAPI channel `2300`; billed with local `ModelPrice=0.066667` and group ratio `1`. |
| `gemini-2.5-flash` stream | HTTP 200. | Same local fixed-price billing path; no upstream price sync involved. |
| `gemini-3.1-flash-image` via `/v1/images/generations` | HTTP 500 from upstream adapter: only Imagen models are supported. | Do not expose this as image generation. It is only proven as a chat/text model through this upstream. |
| `grok-3` non-stream | Upstream returned service unavailable. | Not a local price issue. The upstream Grok path is currently unhealthy. |
| `grok-3` stream | Upstream sent only ping/empty stream data and no usable choices/content/usage. | NewAPI xAI handler previously treated this as success and billed a fixed price. This needed a local guard. |

Immediate production mitigation:

- Disabled NewAPI channel `2297` (`xai-grok-upstream-placeholder`) so public
  users do not hit the unhealthy Grok upstream and get charged for empty stream
  responses.
- Backup before disabling the channel:
  `/opt/newapi/backups/grok-channel-disable-20260622-000919.sql`.
- Verified after disabling:
  - `gemini-2.5-flash` still returns HTTP 200 through channel `2300`.
  - `grok-3` now returns HTTP 503 `model_not_found/no available channel`
    instead of empty HTTP 200 stream billing.

Code fix prepared:

- Patch stored at `patches/newapi/xai-empty-stream-guard-20260621.patch`.
- The patch makes NewAPI's xAI handler reject empty non-stream responses and
  empty stream responses instead of treating them as successful billable output.
- It also falls back to local usage estimation when a non-stream xAI response
  contains valid assistant content but omits usage.
- Local container test passed:
  `go test ./relay/channel/xai`.

Deployment note:

- The code guard is not deployed yet in this entry because the local NewAPI
  workspace contains several unrelated pending edits. Build/deploy should be
  done from a clean NewAPI source tree or by applying only
  `patches/newapi/xai-empty-stream-guard-20260621.patch` to the exact release
  source, then rebuilding a NewAPI image.
- Until that guard is deployed and the upstream Grok path returns real choices
  or usage, keep channel `2297` disabled.

### Follow-up: OpenAI-Compatible Empty Stream Guard - 2026-06-22

Additional finding:

- A temporary OpenAI-compatible clone of the filled Grok channel was created
  for probing only, using the existing stored upstream URL/key without printing
  the key.
- Both non-stream and stream `grok-3` probes timed out or returned upstream
  service unavailable. This means the current Grok upstream path is unhealthy
  even when treated as an OpenAI-compatible proxy.
- The stream probe also exposed a local NewAPI billing risk: an upstream stream
  that emitted no usable SSE data could still fall back to local usage
  estimation and fixed model-price billing.

Fix prepared:

- Patch stored at
  `patches/newapi/openai-xai-empty-response-guard-20260622.patch`.
- It combines the earlier xAI empty-response guard with a generic
  OpenAI-compatible guard:
  - empty non-stream HTTP 200 responses are rejected as `empty_response`;
  - empty stream responses are rejected before local usage estimation;
  - valid content, tool calls, finish reasons, or usage still count as real
    upstream signal.
- Local container tests passed:
  `go test ./relay/channel/openai ./relay/channel/xai`.

Deployment:

- Built NewAPI image locally:
  `newapi:empty-response-guard-20260622`.
- Loaded it on the production server and updated only the NewAPI service.
- Compose backup:
  `/opt/newapi/docker-compose.yml.bak-empty-response-guard-20260622`.
- Running production NewAPI image after deployment:
  `newapi:empty-response-guard-20260622`.
- NewAPI MySQL, NewAPI Redis, Sub2API, and UAG were not recreated.

Verification after deployment:

- NewAPI health returned healthy.
- `gemini-2.5-flash` through channel `2300` returned HTTP 200 with a normal
  assistant reply and local fixed-price billing.
- `grok-3` remained blocked because channel `2297` is disabled, returning
  HTTP 503 `model_not_found` for both non-stream and stream. No Grok consume
  log was created.
- A temporary local fake upstream was used to return:
  - non-stream HTTP 200 `{}`;
  - stream HTTP 200 `data: [DONE]` without content or usage.
- Both fake upstream calls returned HTTP 502 `empty_response` through NewAPI,
  and the consume-log count for the temporary token was `0`.

Operational conclusion:

- NewAPI pricing remains site-owned. The fix does not import or mirror upstream
  prices; it only prevents false-success responses from being settled.
- Keep Grok channel `2297` disabled until the upstream returns real
  `choices/content/usage` in both non-stream and stream probes.
- Gemini text through channel `2300` remains usable. Image/video exposure still
  requires endpoint-specific model IDs and per-call/task billing validation.

### Follow-up: Image Empty Response Guard - 2026-06-22

Additional finding:

- The generic OpenAI-compatible image path (`/v1/images/generations` and
  `/v1/images/edits`) used `OpenaiHandlerWithUsage`, forwarded the upstream
  JSON response, and then billed using the local image usage path.
- That is correct when the upstream returns a real OpenAI image payload, but it
  was too trusting for proxy upstreams that return HTTP 200 with `{}`,
  `{"data":[]}`, or `{"data":[{}]}`.
- In that false-success case NewAPI could write a useless response and still
  continue into local quota settlement.

Fix prepared:

- Patch stored at
  `patches/newapi/openai-xai-empty-response-image-guard-20260622.patch`.
- It extends the deployed OpenAI/xAI empty-response guard with an image payload
  check:
  - OpenAI-compatible text: still requires content, tool calls, finish reason,
    or usage.
  - OpenAI-compatible stream: still requires real SSE signal or usage.
  - xAI text/stream: still rejects empty upstream success.
  - OpenAI-compatible image: now requires at least one `data[].url` or
    `data[].b64_json` before NewAPI writes the response or settles quota.
- Upstream `error` objects are now preserved for image responses instead of
  being treated as successful image payloads.

Local tests:

- `go test ./relay/channel/openai ./relay/channel/xai`

Deployment:

- Built NewAPI image locally:
  `newapi:image-empty-response-guard-20260622`.
- Loaded it on the production server and updated only the NewAPI service.
- Compose backup:
  `/opt/newapi/docker-compose.yml.bak-image-empty-response-guard-20260622`.
- Running production NewAPI image after deployment:
  `newapi:image-empty-response-guard-20260622`.
- NewAPI MySQL, NewAPI Redis, Sub2API, and UAG were not recreated.

Verification after deployment:

- NewAPI container became healthy.
- A temporary fake OpenAI-compatible image upstream returned HTTP 200 with
  `{"data":[]}`. NewAPI returned HTTP 502 with `error.code=empty_response`,
  and the temporary user's consume-log count stayed `0 -> 0`.
- A temporary `gemini-2.5-flash` chat smoke test returned HTTP 200 with a
  non-empty assistant message, and its temporary user's consume-log count
  changed `0 -> 1`.

Operational conclusion:

- Pricing remains local/site-owned. This fix does not read or copy upstream
  prices.
- The correct way to add image/video upstreams is still endpoint-specific:
  first prove the exact image/video endpoint returns real image/video output,
  then expose it with local per-call/task pricing.
- Text-only upstream proxy models such as the current Gemini OpenAI-compatible
  route should not be renamed into public image/video models unless the image
  endpoint itself passes this smoke test.
