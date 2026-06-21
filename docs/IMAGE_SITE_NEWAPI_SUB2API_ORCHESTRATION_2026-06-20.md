# Image Site / NewAPI / Sub2API Orchestration Agreement - 2026-06-20

本文记录生图站 `unified-ai-gateway`、NewAPI、Sub2API 三套系统之间的职责边界、桥接方式和后续维护约定。

不要在本文档写入真实 API Key、密码、refresh token、access token、账号 JSON 或任何完整凭证。

## 1. Current Site Inventory

| System | Current role | Public/admin entry | Internal service notes |
| --- | --- | --- | --- |
| NewAPI | 面向普通 API 用户的产品控制面，负责用户、用户 Key、GPT 文本模型分组、额度、后续充值/邮件。 | `https://dtrljm.com`、`https://api.dtrljm.com/v1`、`https://admin.dtrljm.com` | 当前 GPT 三档为 `gpt-team`、`gpt-plus`、`gpt-pro`。 |
| Sub2API | 内部供给/调度/账号池层，负责上游账号、上游 API、分组、调度、风控、桥接 Key。 | `https://api.vyywcw.cn/admin/accounts`、`/admin/groups`、`/admin/channels`、`/admin/api-keys` | 不直接作为普通用户注册/售卖入口。 |
| UAG image site | 独立生图/生视频产品面，负责图片/视频 UI、任务、作品、用户侧生图 API、图片任务日志。 | `https://image.vyywcw.cn` | 对应 `uag-user-web` + `uag-api`。 |
| UAG admin | 生图站后台，负责生图站用户、积分、模型价格、任务日志、UAG 原生账号池。 | `https://image-admin.vyywcw.cn` | 对应 `uag-admin-web` + `uag-admin`。 |
| UAG OpenAI API | 生图/视频的 OpenAI-compatible API。 | `https://image-api.vyywcw.cn/v1` | 对应 `uag-openai`，支持 `/v1/images/generations` 等图片协议。 |

## 2. Ownership Decision

当前推荐不要把生图站简单并入 NewAPI，也不要把 UAG 的账号池废掉。

稳定边界如下：

| Layer | Owns | Does not own |
| --- | --- | --- |
| NewAPI | 通用 API 用户、GPT 文本产品分组、用户 Key、额度/充值入口、用户可见模型列表。 | 底层 CPA/OAuth 账号池细节；UAG 的图片任务和作品历史。 |
| Sub2API | 上游供给、账号池、桥接 Key、调度、上游健康、风控、账号导入和分组。 | 面向最终用户的产品展示和注册；UAG 图片任务生命周期。 |
| UAG | 生图/生视频站点、图片任务、作品、图片 API Key、UAG 自己的用户/积分、UAG 原生账号池。 | NewAPI 的普通 GPT 文本用户；Sub2API 的底层桥接 Key 外放。 |

这意味着：

- 普通 GPT API 用户继续走 NewAPI。
- GPT 文本供给继续由 NewAPI 桥接到 Sub2API 的 `gpt-team`、`gpt-plus`、`gpt-pro`。
- 生图/生视频用户可以继续在 UAG 注册、登录、扣点和看作品。
- UAG 可以把 NewAPI 或 Sub2API 当成一个内部上游账号，但不能丢失 UAG 自己已经做好的账号池、任务、日志、积分和模型价格能力。

## 3. Recommended Closed Loop

```text
General API user
  -> NewAPI user key
  -> NewAPI group: gpt-team / gpt-plus / gpt-pro
  -> NewAPI hidden bridge channel
  -> Sub2API bridge key
  -> Sub2API GPT supply group
  -> upstream account or upstream API

Image/video user
  -> UAG user site or UAG OpenAI-compatible image API
  -> UAG task / wallet / image logs
  -> UAG account pool
       -> native UAG provider account, for Grok / Flow / direct image providers
       -> dedicated internal GPT image bridge account, for GPT image supply
  -> NewAPI or Sub2API internal bridge
  -> upstream account or upstream API
```

The image site is therefore not only a UI skin. It is a product surface with its own task queue, wallet, image/video protocols, and operational logs.

## 4. Short-Term GPT Image Bridge

Short-term recommended path:

```text
UAG image user
  -> UAG image task
  -> UAG provider account: newapi-image-gpt
  -> NewAPI hidden image group/channel
  -> Sub2API dedicated image supply group, if the image upstream is behind Sub2API
  -> GPT image upstream
```

Why this path:

- UAG already supports image UI, task polling, OpenAI-compatible `/v1/images/generations`, model pricing, image logs, and account pool selection.
- NewAPI already owns user-facing API products and can hold an internal image channel if we need one stable API-shaped upstream for UAG.
- Sub2API remains the preferred place to add or remove large upstream account capacity, but image protocol support must be verified for the exact route before UAG depends on it directly.
- Previous image2 debugging showed direct NewAPI image probing could receive a non-empty image payload, while UAG task handling still had stream/task lifecycle timeout risk. Therefore NewAPI can be a compatibility bridge, but UAG worker behavior still needs smoke testing before public rollout.

Recommended names:

| Layer | Suggested name | Visibility |
| --- | --- | --- |
| NewAPI hidden group | `image-gpt` or `uag-image-gpt` | Admin/internal only |
| NewAPI hidden channel | `sub2api-image-gpt` or `uag-image-gpt` | Admin/internal only |
| Sub2API group | `image-gpt` | Internal supply |
| Sub2API bridge key for NewAPI | `newapi-bridge-image-gpt` | Internal only |
| Sub2API bridge key for direct UAG, only if tested | `uag-bridge-image-gpt` | Internal only |
| UAG account group | `gpt-image-default` / `gpt-image-pro` | UAG admin |
| UAG provider account | `newapi-image-gpt` or `sub2api-image-gpt` | UAG admin |

Do not add `gpt-image-2` back into normal NewAPI GPT text channels such as `gpt-team`, `gpt-plus`, or `gpt-pro`. Image supply needs its own channel boundary so text traffic and image traffic do not break each other.

## 5. When To Bridge Through NewAPI vs Sub2API

Use UAG -> NewAPI when:

- UAG needs one OpenAI-compatible API base URL and API key as an upstream provider account.
- NewAPI image channel behavior has already been smoke-tested.
- We want NewAPI admin to show the internal image upstream channel health.
- Sub2API image route is not yet directly verified from UAG.

Use UAG -> Sub2API directly when:

- Sub2API has a verified image-compatible route for the exact endpoint UAG calls.
- A successful smoke test has been run through UAG using a dedicated `uag-bridge-image-gpt` Sub2API key.
- We want to remove NewAPI from the image supply path and let Sub2API be the only provider scheduler.

Do not treat direct UAG -> Sub2API image routing as complete until a real UAG image task completes and stores a usable image result.

## 6. Native UAG Pool Must Stay

The UAG account pool should remain a first-class mechanism.

Keep using UAG native accounts for:

- Grok image/video or Grok web/API routes.
- Flow image/video providers.
- Any image/video provider where UAG already has a better adapter than NewAPI/Sub2API.
- Per-image-site provider experiments that should not affect general GPT API users.

The bridge account is just another UAG account-pool member. It should not replace:

- `account`
- `account_group`
- `account_group_member`
- health/cooldown state
- generation task logs
- UAG wallet and consume records

## 7. User And Billing Boundary

There are two valid product modes. Pick one per product line and document it before selling.

### Mode A - Recommended Now: UAG Owns Image Users

```text
Image user pays/uses UAG credits
  -> UAG deducts image points
  -> UAG uses internal bridge account
  -> NewAPI/Sub2API are supply cost layers only
```

This preserves the existing image site user center and product UX.

Rules:

- UAG users register and recharge in UAG.
- NewAPI internal bridge key used by UAG should not be a normal customer key.
- NewAPI/Sub2API quota should be managed as internal supply budget, not as the end-user ledger.
- UAG task logs are the first place to debug user image failures.

### Mode B - Later: NewAPI Owns All Users

```text
User pays/uses NewAPI credits
  -> UAG becomes mostly a frontend/task surface
  -> UAG must query or reserve NewAPI quota
  -> NewAPI becomes the single wallet
```

This is possible, but it requires deliberate SSO/session, wallet, quota reservation, refund, and failure reconciliation work. Do not assume it is active today.

## 8. Operational Workflows

| Need | Where to do it |
| --- | --- |
| Add ordinary GPT text capacity for public API users | Sub2API `gpt-team` / `gpt-plus` / `gpt-pro` groups. |
| Add GPT image capacity for image site | Prefer Sub2API `image-gpt` group behind a hidden NewAPI image channel, or UAG native `gpt-image-*` account group if the provider is UAG-native. |
| Add Grok or video capacity | UAG admin account pool first, unless a separate tested Sub2API video bridge is created later. |
| Manage normal API users and user keys | NewAPI admin. |
| Manage image site users, points, image API keys, tasks and works | UAG admin. |
| Manage internal bridge keys and upstream account health | Sub2API admin. |
| Change public GPT text model list | NewAPI channel/group first, then verify matching Sub2API group route. |
| Change image model list or image prices | UAG admin model/pricing config first, then verify upstream bridge can serve that model. |

## 9. Debugging Order

When image generation fails:

1. Check UAG task status and UAG generation logs first.
2. Check whether the UAG task selected the expected UAG account or group.
3. If the selected account is a NewAPI bridge account, check NewAPI hidden image channel usage/errors.
4. If NewAPI bridges to Sub2API, check the Sub2API image group, bridge key, channel, and upstream account health.
5. Only after UAG shows the upstream request was actually made should NewAPI/Sub2API missing logs be treated as an upstream routing problem.

Important known behavior:

- If UAG fails before the upstream call, NewAPI and Sub2API may show no request.
- If NewAPI image output arrives as stream/partial image data, UAG must parse and finalize the task correctly.
- A loading image task in the browser usually means UAG task state should be inspected before assuming the upstream is dead.

## 10. Do Not Do

- Do not expose Sub2API bridge keys to customers.
- Do not use a normal customer NewAPI key as UAG's permanent upstream key.
- Do not mix image-only models into GPT text channels unless the channel is explicitly image-capable and tested.
- Do not delete UAG account-pool functionality because an API bridge exists.
- Do not let the same end-user request be billed independently by both UAG and NewAPI unless that double ledger is intentional and visible.
- Do not configure all products into one shared upstream group; keep GPT text, GPT image, Grok/video, Flow, Opus, Gemini, and future families separated.

## 11. Rollout Checklist For GPT Image

Before exposing GPT image generation to users:

- Confirm the intended product mode: UAG wallet mode or NewAPI wallet mode.
- Create a hidden image-only group/channel in NewAPI if using NewAPI as the bridge.
- Create or confirm a dedicated Sub2API `image-gpt` supply group if Sub2API supplies the underlying capacity.
- Add UAG provider account `newapi-image-gpt` or `sub2api-image-gpt` to the intended UAG account group.
- Keep model whitelist narrow, for example `gpt-image-2` only at first.
- Run one UAG web image task and one `https://image-api.vyywcw.cn/v1/images/generations` smoke test.
- Verify the generated result is not empty, can be downloaded, and is visible in UAG history.
- Verify UAG logs, NewAPI usage, and Sub2API upstream logs show the same trace path where applicable.
- Only then enable the model or package for normal image users.

## 12. Future Maintenance Rule

Every new product family should follow this pattern:

```text
User-facing product surface
  -> one visible product group or package
  -> one internal bridge channel/key per family or tier
  -> one supply group per family or tier
  -> explicit logs and smoke tests
```

For the current stack:

- GPT text: NewAPI visible groups -> Sub2API GPT tier groups.
- GPT image: UAG image product -> hidden image bridge -> image-specific supply group.
- Grok/video: UAG native pool unless a dedicated tested bridge is created.
- Future Opus/Gemini/etc.: create separate NewAPI/Sub2API families later; do not mix them into the GPT image path.

## 13. Production Placeholder Created - 2026-06-20

On 2026-06-20, a disabled/hidden GPT Image2 supply line was created so the upstream URL and key can be filled later from the admin UI without mixing image traffic into GPT text channels.

Backups created before the database changes:

- NewAPI: `/opt/newapi/backups/image2-placeholder-20260620-022645.sql`
- UAG: `/opt/unified-ai-gateway/backups/image2-placeholder-20260620-022645.sql`
- Sub2API: `/opt/sub2api/backups/image2-placeholder-20260620-022645.sql`

No real upstream key was written to this document.

### NewAPI

Created a disabled placeholder channel:

| Field | Value |
| --- | --- |
| Channel id | `2296` |
| Name | `uag-gpt-image2-upstream` |
| Type | OpenAI-compatible (`type = 1`) |
| Status | disabled (`status = 2`) |
| Group | `image` |
| Models | `gpt-image-2` |
| Tag | `uag-image2` |
| Base URL | placeholder only; replace in admin before enabling |
| Key | placeholder only; replace in admin before enabling |

The historical disabled channel `2150 / Sub2API GPT5.5 image2 upstream` was left in place for reference. Do not use it for the new path unless it is intentionally migrated and re-tested.

NewAPI options after the original placeholder change:

- `GroupRatio` includes internal `image`.
- `TopupGroupRatio` includes internal `image`.
- `AutoGroups` remains empty.

## 14. UAG 502 Busy Message Fix - 2026-06-21

User-visible symptom:

- UAG user site showed `服务正在更新或繁忙，请稍后重试`.
- `https://image.vyywcw.cn/api/v1/models` returned HTTP 502.
- `https://image-api.vyywcw.cn/v1/models` returned HTTP 502.
- The frontend maps HTTP 502/503/504 to that generic busy/update message.

Root cause:

- `uag-api`, `uag-admin`, and `uag-openai` had restarted after MySQL was not
  ready during server boot.
- `uag-nginx` had resolved Docker service names to old container IPs and kept
  proxying to those stale IPs.
- Nginx logs showed `connect() failed (111: Connection refused)` to the old
  upstream IPs, while the backend containers were already running on new IPs.

Production fix applied:

- Updated UAG nginx configs to use Docker DNS resolver `127.0.0.11` with
  variable-based `proxy_pass`, so service names are re-resolved instead of
  pinned to stale container IPs.
- Backed up old server configs under `/opt/unified-ai-gateway/backups/`.
- Ran `docker exec uag-nginx nginx -t`.
- Reloaded only `uag-nginx`; NewAPI and Sub2API were not restarted.

Reproducible patch:

```text
patches/uag/nginx-docker-dns-resolver-20260621.patch
```

Verification:

- `https://image.vyywcw.cn/api/v1/models` returned HTTP 200.
- `https://image-admin.vyywcw.cn/` returned HTTP 200.
- `https://image-api.vyywcw.cn/v1/models` returned HTTP 401, which is expected
  without an API key.
- Restarted only `uag-api`; after health became healthy,
  `https://image.vyywcw.cn/api/v1/models` still returned HTTP 200 without
  reloading nginx. This confirms the stale-IP failure mode is fixed for the
  user API path.

Current status check on 2026-06-20:

- `UserUsableGroups` now includes `image` as `GPT-image`.
- `AutoGroups` remains empty, so new users should not receive `image` automatically unless an operator or package grants it.
- `uag-gpt-image2-upstream` is enabled in NewAPI, but its base URL was still the placeholder `https://fill-in-upstream.example/v1` during the check.

Operationally, do not treat GPT Image2 as a working public route until the placeholder URL/key are replaced with a real image-capable upstream and one UAG web/API smoke test succeeds.

### UAG

Created a dedicated UAG account group:

| Field | Value |
| --- | --- |
| Group id | `5` |
| Provider | `gpt` |
| Code | `gpt-image2-newapi` |
| Name | `GPT Image2 NewAPI Bridge` |
| Status | active |

Created a disabled UAG model entry:

| Field | Value |
| --- | --- |
| Model id | `14` |
| Code | `gpt-image-2` |
| Name | `GPT Image 2` |
| Kind | `image` |
| Provider | `gpt` |
| Group | `gpt-image2-newapi` |
| Default params | `route=api`, `resolution=1K`, `quality=high` |
| Status | disabled (`status = 0`) |

The model is intentionally disabled until the NewAPI channel is filled and a real UAG smoke test succeeds.

Current UAG still has the older active account:

- account name: `newapi-gpt-image-2`
- provider: `gpt`
- auth type: `api_key`
- group: `gpt-image-default`
- whitelist: `gpt-image-2`

Do not assume that old account is the new production path. Prefer moving or recreating the provider account into `gpt-image2-newapi` after the new NewAPI channel is configured.

### Sub2API

Created a dedicated image supply group and channel:

| Layer | Value |
| --- | --- |
| Group id | `16` |
| Group name | `image-gpt` |
| Platform | `openai` |
| Status | active |
| Image generation | enabled |
| Model list | `gpt-image-2` |
| Channel id | `10` |
| Channel name | `channel-newapi-image-gpt` |
| Channel features | `image` |

No upstream accounts were moved into this group during this pass. The group is a clean landing zone for future image-specific supply.

### How To Finish Configuration Later

If using NewAPI as the UAG bridge:

1. Open NewAPI admin.
2. Find channel `uag-gpt-image2-upstream`.
3. Replace the placeholder base URL with the real upstream base URL.
4. Replace the placeholder key with the real upstream key.
5. Keep group as `image` and model list as `gpt-image-2`.
6. Enable the channel only after the upstream provider is known to support image generation.
7. In UAG admin, add or move a GPT API-key provider account into `gpt-image2-newapi` using the NewAPI image base URL/key.
8. Enable UAG model `gpt-image-2`.
9. Run one UAG web image task and one `https://image-api.vyywcw.cn/v1/images/generations` smoke test.

If using Sub2API as the underlying supply:

1. Import or create image-capable upstream accounts into Sub2API group `image-gpt`.
2. Keep image capacity out of `gpt-team`, `gpt-plus`, `gpt-pro`, and the historical mixed `GPT5.5` group unless intentionally sharing capacity.
3. Use a dedicated bridge key such as `newapi-bridge-image-gpt` if NewAPI calls Sub2API.
4. Point the NewAPI `uag-gpt-image2-upstream` channel to Sub2API only after a direct image route smoke test succeeds.

### Verification Performed

After creating the placeholders:

- `https://dtrljm.com` returned HTTP 200.
- `https://api.dtrljm.com` returned HTTP 200.
- `https://image.vyywcw.cn` returned HTTP 200.
- `https://image.vyywcw.cn/api/v1/ping` returned HTTP 200.
- `https://api.vyywcw.cn` returned HTTP 200.
- `newapi`, `sub2api`, `uag-api`, and related containers remained up/healthy.

No service restart was performed for this configuration-only pass.

## 14. Grok And Gemini Bridge Placeholders - 2026-06-20

On 2026-06-20, two separate bridge families were created for Grok and Gemini so both the user-facing API site and the image/video site can later reuse the same supply pattern without mixing these models into the GPT text or GPT image groups.

No upstream provider key, user key, refresh token, or account payload is recorded here.

Backups created before the database changes:

- NewAPI: `/opt/newapi/backups/grok-gemini-bridge-20260620-201601.sql`
- UAG: `/opt/unified-ai-gateway/backups/grok-gemini-bridge-20260620-201601.sql`
- Sub2API: `/opt/sub2api/backups/grok-gemini-bridge-20260620-201601.sql`

An earlier failed SQL attempt was blocked inside a transaction before any rows were written; the successful backup timestamp is the one above.

### Public API Surface

For normal external API users, the public OpenAI-compatible endpoint remains:

```text
https://api.dtrljm.com/v1
```

For image-site users or external image API callers, the UAG OpenAI-compatible endpoint is:

```text
https://image-api.vyywcw.cn/v1
```

For image generation, callers should use:

```text
POST https://image-api.vyywcw.cn/v1/images/generations
```

The image web product remains:

```text
https://image.vyywcw.cn
```

Sub2API bridge keys are internal only and should not be handed to external users.

### Sub2API

Created supply groups:

| Group id | Name | Platform | Purpose |
| ---: | --- | --- | --- |
| `17` | `grok` | `openai` | Internal Grok supply group for NewAPI/UAG bridge traffic. |
| `18` | `gemini` | `openai` | Internal Gemini supply group for NewAPI/UAG bridge traffic. |

Created bridge channels:

| Channel id | Name | Group | Status | Model family |
| ---: | --- | --- | --- | --- |
| `11` | `channel-newapi-grok` | `grok` | active | `grok-3-beta`, `grok-3-fast-beta`, `grok-3-mini-beta`, `grok-3-mini-fast-beta`, `grok-2`, `grok-2-vision` |
| `12` | `channel-newapi-gemini` | `gemini` | active | `gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-2.0-flash`, `gemini-1.5-pro-latest`, `gemini-1.5-flash-latest` |

Created internal bridge API keys:

| Key name | Bound group | Usage |
| --- | --- | --- |
| `newapi-bridge-grok` | `grok` | Stored in the NewAPI `sub2api-grok` channel. |
| `newapi-bridge-gemini` | `gemini` | Stored in the NewAPI `sub2api-gemini` channel. |

Add future upstream API-key accounts or provider accounts into the matching Sub2API group first, then enable the matching NewAPI channel only after smoke tests pass.

### NewAPI

Created disabled placeholder channels:

| Channel id | Name | Status | Group | Base URL | Models |
| ---: | --- | --- | --- | --- | --- |
| `2297` | `sub2api-grok` | disabled | `grok` | `http://sub2api:8080` | `grok-3-beta`, `grok-3-fast-beta`, `grok-3-mini-beta`, `grok-3-mini-fast-beta`, `grok-2`, `grok-2-vision` |
| `2298` | `sub2api-gemini` | disabled | `gemini` | `http://sub2api:8080` | `gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-2.0-flash`, `gemini-1.5-pro-latest`, `gemini-1.5-flash-latest` |

NewAPI group options after this pass:

- `GroupRatio` includes `grok: 1` and `gemini: 1`.
- `TopupGroupRatio` includes `grok: 1` and `gemini: 1`.
- `UserUsableGroups` was not expanded for `grok` or `gemini`.
- `AutoGroups` remains empty.

This means Grok/Gemini are prepared as admin-controlled product groups, but are not automatically given to normal users yet.

### UAG

Created UAG account groups:

| Group id | Provider | Code | Status | Purpose |
| ---: | --- | --- | --- | --- |
| `6` | `gemini` | `gemini-newapi-bridge` | active | Landing group for Gemini through NewAPI/Sub2API. |
| `7` | `grok` | `grok-newapi-bridge` | active | Optional landing group for Grok through NewAPI/Sub2API. |

Created disabled Gemini model placeholders:

| Model id | Code | Kind | Provider | Group | Status |
| ---: | --- | --- | --- | --- | --- |
| `15` | `gemini-2.5-flash` | text | `gemini` | `gemini-newapi-bridge` | disabled |
| `16` | `gemini-2.5-pro` | text | `gemini` | `gemini-newapi-bridge` | disabled |

Existing UAG Grok groups and models were left unchanged. They are still the native Grok web/video path and should not be overwritten by the new bridge until a specific migration is tested.

### Verification Performed

After the Grok/Gemini placeholders were created:

- Sub2API groups `grok` and `gemini` existed and were active.
- Sub2API channels `channel-newapi-grok` and `channel-newapi-gemini` were active and bound to the matching group.
- Sub2API bridge keys `newapi-bridge-grok` and `newapi-bridge-gemini` existed and were bound to the matching group.
- NewAPI channels `sub2api-grok` and `sub2api-gemini` existed, pointed to `http://sub2api:8080`, and remained disabled.
- NewAPI `UserUsableGroups` was not expanded for `grok` or `gemini`.
- UAG `gemini-newapi-bridge` and `grok-newapi-bridge` account groups existed.
- `https://dtrljm.com` returned HTTP 200.
- `https://api.dtrljm.com` returned HTTP 200.
- `https://image.vyywcw.cn` returned HTTP 200.
- `https://image-api.vyywcw.cn/v1/models` returned HTTP 401 without an API key, which confirms the endpoint is protected rather than public.

No NewAPI, Sub2API, or UAG service restart was performed.

### Enablement Order

For Grok or Gemini external upstream-first rollout:

1. Add the upstream API URL and key as an account/channel in Sub2API under the matching `grok` or `gemini` group.
2. Smoke test the upstream directly from the server.
3. Smoke test through Sub2API using the matching bridge group/key.
4. Enable the matching NewAPI channel `sub2api-grok` or `sub2api-gemini`.
5. Add `grok` or `gemini` to the intended NewAPI user/package group only after the NewAPI route returns a real answer.
6. If the image/video site should use the same capability, add a UAG provider account pointing at the tested NewAPI or Sub2API route and enable the UAG model after a real task succeeds.

### 2026-06-20 OpenAI-Compatible Grok/Gemini Mapping Fix

Problem found after adding a Grok upstream in Sub2API: the upstream account was stored as `platform=openai`, but its account-level `credentials.model_mapping` still contained GPT/OpenAI self-mapping defaults. The scheduler therefore rejected `grok-*` and `gemini-*` models even though the `grok` and `gemini` groups already had the right model lists.

Backend fix deployed to Sub2API:

- On admin account create/update, if `platform=openai` is bound to a group that has a custom model list, Sub2API now auto-populates account `model_mapping` from that group model list.
- Existing custom mappings such as `custom-model -> upstream-model` are preserved.
- Empty mappings, default GPT/OpenAI mappings, and pure GPT self-mappings are replaced when the target group model list is non-OpenAI-compatible family models such as `grok-*` or `gemini-*`.
- Focused test run: `go test -tags unit ./internal/service -run "OpenAICompatible|DefaultOpenAIModelMapping"`.

Production actions:

- Database backup: `/opt/sub2api/backups/grok-gemini-model-mapping-fix-20260620-213631.sql`.
- Updated existing Grok account `7925` to use only the `grok` group models in `model_mapping`.
- Deployed Sub2API image `sub2api-provider-adapters:grok-gemini-model-map-20260620-214218`.
- Only the `sub2api` service was recreated. NewAPI was not restarted.

Verification:

- `https://api.vyywcw.cn/health` returned HTTP 200.
- `newapi-bridge-grok` `/v1/models` returned only:
  `grok-2`, `grok-2-vision`, `grok-3-beta`, `grok-3-fast-beta`, `grok-3-mini-beta`, `grok-3-mini-fast-beta`.
- A direct Grok chat test no longer failed with model selection/mapping errors, but returned upstream authentication failure. This means the model-mapping bug is fixed; the current Grok upstream API key or URL must be replaced with a valid upstream before exposing it in NewAPI.
- No Gemini upstream account was bound at the time of this check, so Gemini will use the new auto-mapping path when the first upstream account is added to the `gemini` group.

### 2026-06-21 UAG GPT Image2 Invalid Token / Main Model Fix

User-visible symptom:

- The image site test user `33376394541` could log in, but every GPT image generation failed with:
  `provider call: gpt image2 401 ... Invalid token`.

Findings:

- UAG image tasks for user id `2` reached UAG and NewAPI.
- NewAPI logs showed `/v1/responses` rejected the UAG upstream credential as `Invalid token`.
- UAG account `1` (`newapi-gpt-image-2`) still held an old encrypted NewAPI token and was marked broken.
- NewAPI already had an enabled internal token named `uag-image-2` in group `image`.
- After replacing the encrypted UAG credential with the enabled NewAPI internal token, `/v1/models` returned HTTP 200 and the 401 disappeared.
- The next failure was `model_not_found`: UAG's GPT image2 `/v1/responses` body used top-level `model: gpt-5.5`, while the NewAPI image group/channel is intentionally image-only and only routes `gpt-image-2`.

Production configuration fix:

- Updated UAG account `1` encrypted credential from the enabled NewAPI internal token `uag-image-2`.
- Kept the token value out of docs and logs; only compared SHA-256 internally.
- Restored UAG account `1` to enabled status.
- Moved UAG account `1` to the dedicated `gpt-image2-newapi` account group.
- Enabled UAG model `gpt-image-2`, set its group to `gpt-image2-newapi`, and set default params to `route=api`, `quality=high`, `resolution=1K`.

Code fix deployed to UAG:

- Patch stored at `patches/uag/image2-newapi-token-and-main-model-20260621.patch`.
- `backend/internal/provider/gpt/gpt.go` now uses top-level `gpt-image-2` for GPT image2 `/v1/responses` API-route calls instead of `gpt-5.5`.
- Added a focused provider test for that default.
- `frontend/apps/user/src/pages/create/CreateImagePage.tsx` now sends `params.route=api` and `params.main_model=gpt-image-2` when the user selects `gpt-image-2`.

Verification:

- `go test ./internal/provider/gpt` passed inside a `golang:1.24-alpine` container on the production server.
- `docker compose --env-file ./env/.env.local build user-web` completed successfully.
- `docker compose --env-file ./env/.env.local build api` completed successfully.
- Recreated only UAG `api`, `openai`, `worker`, and `user-web`; NewAPI and Sub2API were not restarted.
- `https://image.vyywcw.cn/api/v1/models` returned HTTP 200.
- Direct NewAPI image2 smoke through `https://newapi.vyywcw.cn/v1/responses` returned HTTP 200 and produced an image stream.
- UAG OpenAI-compatible smoke through `https://image-api.vyywcw.cn/v1/images/generations` with a temporary test key returned HTTP 200 and a non-empty `b64_json` image payload.
- The temporary UAG test key was soft-deleted immediately after verification.
- The successful UAG task was `76dc77edf56245dd8a07199731`, status `2`, progress `100`.
- UAG public model pricing/config was cleaned so old GPT image aliases `img-v3`, `img-real`, `img-anime`, and `img-3d` are no longer exposed; `https://image.vyywcw.cn/api/v1/models` now exposes `gpt-image-2` as the GPT image model.

Operational note:

- Do not add `gpt-5.5` to the NewAPI `image` group just to satisfy UAG image2. The image route is image-only by design; UAG should send top-level `gpt-image-2` for this bridge.

### 2026-06-21 Image2 Non-Stream And Per-Image Billing Fix

User-visible symptom:

- GPT image2 could generate through the image site, but NewAPI usage logs still showed the upstream bridge as a `/v1/responses` stream-like request.
- This made the image route look like text/Responses billing and could confuse upstream or user-side accounting. Image generation should be billed per image call, not as a streamed text request.

UAG code fix:

- Patch files:
  - `patches/uag/image2-newapi-nonstream-responses-20260621-gpt.patch`
  - `patches/uag/image2-newapi-nonstream-responses-20260621-test.patch`
- `backend/internal/provider/gpt/gpt.go` now treats NewAPI/OpenAI-compatible `/v1/responses` as non-stream JSON for GPT image2:
  - no `stream: true` in the request body;
  - `Accept: application/json`;
  - JSON Responses output is parsed directly.
- ChatGPT Codex backend remains stream-compatible, because that route still requires SSE behavior.

NewAPI code fix:

- Patch file: `patches/newapi/image2-responses-image-only-billing-20260621.patch`.
- `service/text_quota.go` now detects `gpt-image-2` Responses results that include `image_generation_call` and bills them as image-only:
  - `is_stream=false`;
  - `billing_mode=image`;
  - `image_generation_only=true`;
  - `model_price=0`;
  - quota is only the image generation call fee, without the extra text/Responses base model fee.

Tests:

- UAG: `go test ./internal/provider/gpt` passed locally in `golang:1.26.4` and on the production server in `golang:1.24-alpine`.
- NewAPI: `go test ./service` passed locally in `golang:1.26.4`.

Production deployment:

- UAG:
  - Backups:
    - `/opt/unified-ai-gateway/backups/image2-nonstream-20260621-182333/`
  - Rebuilt `unified-ai-gateway/backend:latest`.
  - Recreated only UAG `api`, `openai`, `worker`, and `admin`.
  - NewAPI and Sub2API were not restarted during the UAG patch.
- NewAPI:
  - Built preloaded image `newapi:image2-per-call-20260621-55a25843`.
  - Loaded it on the server with `docker load`.
  - Backed up compose file:
    - `/opt/newapi/docker-compose.yml.bak-image2-per-call-20260621-55a25843`
  - Recreated only the `newapi` container. NewAPI MySQL/Redis, Sub2API, and UAG were left running.

Verification:

- `https://image.vyywcw.cn/api/v1/models` returned HTTP 200.
- `https://image-api.vyywcw.cn/v1/images/generations` with a temporary image-only UAG key returned HTTP 200 and a non-empty image payload.
- Temporary UAG smoke keys were soft-deleted immediately after verification.
- NewAPI log for the final smoke request showed:
  - `request_path=/v1/responses`
  - `is_stream=false`
  - `billing_mode=image`
  - `image_generation_only=true`
  - `model_price=0`
  - `quota=41750`
  - `content="Image Generation Call ..."`

Operational note:

- It is acceptable that the hidden bridge still uses `/v1/responses` internally because the GPT image2 tool currently returns image outputs there. The important production invariants are: no SSE stream for this NewAPI bridge, and NewAPI accounting is image-only/per-call.

### 2026-06-21 Grok / Gemini / Image-Video Channel Setup

Goal:

- NewAPI users should be able to call the public API after the admin fills real upstream URL/key.
- The image/video site should keep its own UAG task, wallet, model, account-pool, and work-history layer.
- Sub2API remains the private supply/scheduler layer; do not expose Sub2API keys to ordinary users.
- Do not store real upstream keys, passwords, access tokens, refresh tokens, or full account payloads in this repo.

NewAPI production placeholders:

| Channel id | Name | Type | Status | Base URL | Groups |
| --- | --- | --- | --- | --- | --- |
| `2297` | `xai-grok-upstream-placeholder` | `48` xAI | disabled until key is filled | `https://api.x.ai` | `grok,gpt-team,gpt-plus,gpt-pro` |
| `2298` | `google-gemini-upstream-placeholder` | `24` Gemini | disabled until key is filled | `https://generativelanguage.googleapis.com` | `gemini,gpt-team,gpt-plus,gpt-pro` |

NewAPI xAI placeholder model list:

- `grok-4.3`
- `grok-build-0.1`
- `grok-4-1-fast-reasoning`
- `grok-4-1-fast-non-reasoning`
- `grok-code-fast-1`
- `grok-4-fast-reasoning`
- `grok-4-fast-non-reasoning`
- `grok-4-0709`
- `grok-3-mini`
- `grok-3`
- `grok-2-vision-1212`
- `grok-imagine-image-quality`
- `grok-imagine-image`
- `grok-imagine-image-pro`
- `grok-2-image-1212`
- `grok-imagine-video`
- `grok-imagine-video-1.5`
- `grok-imagine-video-1.5-preview`

NewAPI Gemini placeholder model list:

- `gemini-3.1-flash-image`
- `gemini-3-pro-image`
- `gemini-3.1-flash-image-preview`
- `gemini-3-pro-image-preview`
- `nano-banana-pro-preview`
- `gemini-2.5-pro`
- `gemini-2.5-flash`
- `gemini-2.5-flash-lite`
- `imagen-4.0-generate-001`
- `imagen-4.0-ultra-generate-001`
- `imagen-4.0-fast-generate-001`
- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`
- `veo-3.1-lite-generate-preview`

Initial NewAPI model-ratio placeholders were inserted for:

- `grok-4.3`
- `grok-build-0.1`
- `grok-imagine-image-quality`
- `grok-imagine-image`
- `grok-imagine-image-pro`
- `grok-imagine-video`
- `grok-imagine-video-1.5`
- `grok-imagine-video-1.5-preview`
- `gemini-3.1-flash-image`
- `gemini-3.1-flash-image-preview`
- `gemini-3-pro-image`
- `gemini-3-pro-image-preview`
- `nano-banana-pro-preview`
- `veo-3.1-lite-generate-preview`

These values are only runnable defaults so requests do not fail with "model price not configured". The production selling price should still be adjusted in the NewAPI admin pricing/model settings UI before public rollout.

Where to fill credentials:

1. NewAPI admin, for public API users:
   - Open the channel list.
   - Edit `xai-grok-upstream-placeholder`, fill the real xAI-compatible URL/key, then enable the channel.
   - Edit `google-gemini-upstream-placeholder`, fill the real Gemini API key, then enable the channel.
   - Keep these channels bound to `gpt-team`, `gpt-plus`, and `gpt-pro` only if these GPT groups are meant to include Grok/Gemini fallback/capacity. Otherwise remove the group binding before enabling.
2. UAG admin, for the image/video product site:
   - GPT Image2: use provider `gpt`, account group `gpt-image2-newapi`, model whitelist `["gpt-image-2"]`, and point the account at the NewAPI internal image bridge.
   - Grok Imagine video: use provider `grok`, account group `grok-imagine-native`, model whitelist `["grok-imagine-video"]` or the exact video model supported by the current provider.
   - Gemini/Imagen/Veo product models: use provider `flow`, account group `flow-gemini-image-video`, and model whitelist such as `["gemini-3.1-flash-image-*","imagen-4.0-*","veo_3_1_*"]`.
3. Sub2API admin:
   - Use Sub2API for private supply, scheduler, account pools, and bridge keys.
   - Do not give Sub2API bridge keys directly to users.
   - If NewAPI should consume Sub2API capacity, create a dedicated Sub2API API key per supply group and put that key into a hidden NewAPI channel.

UAG product model codes:

| Product surface | UAG provider | UAG model codes |
| --- | --- | --- |
| GPT image | `gpt` | `gpt-image-2` |
| Grok video | `grok` | `grok-imagine-video` |
| Gemini/Imagen image | `flow` | `gemini-3.1-flash-image-square`, `gemini-3.1-flash-image-landscape`, `gemini-3.1-flash-image-portrait`, `imagen-4.0-generate-preview-landscape`, `imagen-4.0-generate-preview-portrait` |
| Veo video | `flow` | `veo_3_1_t2v_fast_landscape`, `veo_3_1_t2v_fast_portrait`, `veo_3_1_t2v_landscape_6s`, `veo_3_1_t2v_portrait_6s`, `veo_3_1_i2v_s_fast_fl_landscape`, `veo_3_1_i2v_s_fast_fl_portrait` |

Important distinction:

- NewAPI model names are public API/upstream model ids, for example `gemini-3-pro-image` or the legacy-compatible `gemini-3-pro-image-preview`.
- UAG model codes are product-side task codes, for example `gemini-3.1-flash-image-square` or `veo_3_1_t2v_fast_landscape`.
- Do not force these two lists to be identical. UAG maps product codes to its provider protocol internally.

UAG provider mode and deployment status:

- Production UAG env has real provider modes enabled:
  - `KLEIN_PROVIDER_GPT=real`
  - `KLEIN_PROVIDER_GROK=real`
  - `KLEIN_PROVIDER_FLOW=real`
- Current deployed UAG backend image:
  - `unified-ai-gateway/backend:model-whitelist-20260621`
- Recreated UAG services only:
  - `uag-api`
  - `uag-admin`
  - `uag-openai`
  - `uag-worker`
- NewAPI was not restarted for the Grok/Gemini placeholder setup. NewAPI channel/options cache syncs from DB periodically, and saving/enabling channels in the admin UI also refreshes the practical runtime state.

UAG scheduling fix:

- Patch stored at `patches/uag/model-whitelist-scheduler-20260621.patch`.
- `GenerationService.pickAccountForTask` now respects account `model_whitelist` for all provider/model selections.
- GPT Image2 API-key and OAuth/Codex route predicates also include the whitelist check.
- Whitelist supports exact match and prefix wildcard, for example:
  - `["gpt-image-2"]`
  - `["veo_3_1_*"]`
  - `["gemini-3.1-flash-image-*"]`
- Malformed legacy whitelist JSON fails open to avoid unexpectedly breaking old accounts.

Verification:

- UAG focused test passed locally:
  - `docker run --rm -v "D:\wflogin\unified-ai-gateway\backend:/app" -w /app golang:1.24-alpine go test ./internal/service`
- Production container state after deploy:
  - `uag-api` healthy on `unified-ai-gateway/backend:model-whitelist-20260621`.
  - `newapi` healthy and was not restarted during this placeholder setup.

Backups:

- NewAPI DB snapshot:
  - `/opt/newapi/backups/grok-gemini-config-20260621-210240.sql`
- UAG DB snapshot:
  - `/opt/unified-ai-gateway/backups/grok-gemini-config-20260621-210240.sql`
- UAG backend source backup before whitelist deploy:
  - `/opt/unified-ai-gateway/backups/backend-before-whitelist-20260621-212419.tar.gz`

Current caveat:

- Live Grok/Gemini generation was not smoke-tested in this pass because no real upstream Grok/Gemini URL/key was configured in the placeholders.
- After filling keys, first smoke test with a cheap text model:
  - xAI: `grok-3` or the cheapest configured text model.
  - Gemini: `gemini-2.5-flash`.
- Then test image/video:
  - NewAPI image: `grok-imagine-image-quality` and `gemini-3.1-flash-image`.
  - UAG site: `gpt-image-2`, one Flow image model, and one Flow/Grok video model.

Gemini image model note:

- Prefer GA ids `gemini-3.1-flash-image` and `gemini-3-pro-image` when the upstream accepts them.
- Keep preview ids `gemini-3.1-flash-image-preview`, `gemini-3-pro-image-preview`, and `nano-banana-pro-preview` only for compatibility with older upstream gateways.
- If an upstream gateway reports a model mismatch, use that upstream's exact `/models` response as the source of truth before exposing the model publicly.

xAI model note:

- Prefer the current GA ids when accepted by the upstream, such as `grok-4.3`, `grok-build-0.1`, `grok-imagine-image-quality`, and `grok-imagine-video-1.5`.
- Keep legacy or preview ids such as `grok-imagine-image`, `grok-imagine-image-pro`, and `grok-imagine-video-1.5-preview` only for compatibility with older upstream gateways.
- UAG native Grok video currently uses the existing `grok-imagine-video` product/provider path. Do not expose `grok-imagine-video-1.5` through UAG until the upstream video create/poll protocol is verified.

Before wiring any new upstream:

Ask what protocol the upstream actually exposes before deciding where to configure it.

| Upstream interface | Preferred place | Why |
| --- | --- | --- |
| OpenAI-compatible `/v1/chat/completions` or `/v1/responses` | NewAPI channel | Best fit for public API users, keys, groups, quotas, and logs. |
| OpenAI-compatible `/v1/images/generations` returning URL or `b64_json` | NewAPI for public API; UAG `gpt` provider for image site if product tasks/history are needed | NewAPI can expose the API; UAG should own the image product workflow. |
| Gemini native API / Google Generative Language API | NewAPI Gemini channel for public API; UAG Flow provider only when using Flow/Labs-style image/video task adapters | Gemini text/image API and Flow/Labs product tasks are not the same integration. |
| xAI-compatible text/image API | NewAPI xAI channel for public API | xAI is close to OpenAI-compatible for chat/image paths. |
| Async video/image task API with create/poll/fetch | UAG provider or NewAPI task channel only after a real task smoke test | Video often needs task IDs, polling, callback, duration billing, and result normalization. |
| Account-pool/OAuth/CPA supply requiring custom refresh and scheduler | Sub2API or UAG native account pool | This is supply management, not a direct public user API channel. |

Minimum upstream questions:

1. What base URL and path are used for chat, image, and video?
2. Is the request OpenAI-compatible, Gemini-native, xAI-native, or a custom task API?
3. Does image return immediately as `url`/`b64_json`, or does it return a task id that must be polled?
4. Does video bill by task, second, duration, resolution, or token-like usage?
5. What model ids are accepted exactly, and are image/video model ids separate from chat model ids?
6. What error format and rate-limit headers are returned?

Do not enable a public NewAPI channel for an upstream until the cheapest text smoke test and one protocol-specific smoke test have passed. For image/video, "models list succeeds" is not enough; a real task must complete and produce a usable result.

### 2026-06-21 Filled Upstream Smoke Result

The user filled both NewAPI placeholders with the same upstream host and enabled them. Secrets were not printed or recorded.

Observed NewAPI channel state:

| Channel | Status | Has key | Upstream model list |
| --- | --- | --- | --- |
| `xai-grok-upstream-placeholder` | enabled | yes | `grok-3`, `grok-420-fast`, `grok-420-fast-deepsearch` |
| `google-gemini-upstream-placeholder` | enabled | yes | `gemini-2.5-flash`, `gemini-2.5-pro`, `gemini-2.5-flash-lite`, `gemini-3-flash`, `gemini-3-flash-agent`, `gemini-3.1-flash-image`, `gemini-3.1-pro`, `gemini-3.1-pro-high`, `gemini-3.1-pro-low`, `gemini-3.5-flash` |

Actual upstream checks:

- xAI/OpenAI-compatible chat:
  - `POST /v1/chat/completions` with `model=grok-3` returned HTTP 200 and the expected text reply.
  - Therefore this upstream is usable for Grok text through NewAPI.
- xAI image:
  - skipped because upstream `/v1/models` did not expose any `grok-imagine-*` or image model.
  - Therefore this Grok channel should currently be treated as text-only.
- Gemini model list:
  - `/v1/models` returned HTTP 200 through the OpenAI-compatible model-list shape.
  - This is not enough to prove Gemini-native generation works.
- Gemini native text/image:
  - `POST /v1beta/models/gemini-2.5-flash:generateContent` returned Cloudflare 1010.
  - `POST /v1beta/models/gemini-3.1-flash-image:generateContent` also returned Cloudflare 1010.
  - Therefore the current Gemini channel cannot be considered usable yet in NewAPI's Gemini-native adapter path.

Current capability conclusion:

- Public NewAPI Grok text: works.
- Public NewAPI Grok image/video: not available from this upstream model list.
- Public NewAPI Gemini text/image: blocked by upstream Cloudflare 1010 on the Gemini-native path; not ready.
- Public NewAPI video: not available; no `veo-*`, Grok video, or other video task model is exposed by the filled upstream.
- UAG image/video site:
  - GPT Image2 bridge account exists and is separate.
  - No active UAG `flow` or native `grok` account was observed for Gemini/Veo/Grok video; only `gpt` provider accounts were listed.
  - Therefore the filled NewAPI Grok/Gemini upstream does not automatically enable UAG video.

Next fix direction:

1. Ask the upstream whether Gemini calls should use OpenAI-compatible `/v1/chat/completions` instead of native `/v1beta/models/...:generateContent`.
2. If the upstream is OpenAI-compatible only, create it as an OpenAI-compatible channel or use model mapping instead of NewAPI's Gemini channel type.
3. If the upstream claims Gemini-native support, the upstream owner must relax Cloudflare/WAF rules for server-to-server API calls or provide a non-CF API origin.
4. For video, request exact video model ids and the create/poll/fetch API contract before adding it to NewAPI or UAG.

### 2026-06-21 Filled Upstream Route Fix

NewAPI channel changes:

- Created `openai-compatible-gemini-upstream` as channel `2300`, type `1`
  OpenAI-compatible.
- Copied the filled upstream base URL, key, group list, and model list from
  `google-gemini-upstream-placeholder`.
- Disabled `google-gemini-upstream-placeholder` because this upstream is not
  usable through NewAPI's native Gemini adapter path.
- Rebuilt abilities for channel `2300` across `gemini`, `gpt-team`,
  `gpt-plus`, and `gpt-pro`.
- Backup before the change:
  `/opt/newapi/backups/gemini-openai-compat-20260621-223209.sql`.

Smoke test result through the public NewAPI API endpoint, using a temporary
test token that was deleted after the test:

| Path | Model | Result |
| --- | --- | --- |
| `/v1/chat/completions` | `gemini-2.5-flash` | HTTP 200, returned text. |
| `/v1/chat/completions` | `gemini-3.1-flash-image` | HTTP 200, returned text. |
| `/v1/chat/completions` | `grok-3` | HTTP 500 from upstream: service temporarily unavailable. |
| `/v1/images/generations` | `gemini-3.1-flash-image` | HTTP 500 from upstream: not supported for image generation; upstream says only Imagen models are supported. |

Important testing note:

- A Python `urllib` smoke client was blocked by Cloudflare with `error code:
  1010` before the request reached NewAPI. Re-testing with `curl` reached
  NewAPI and produced normal relay logs. Treat 1010 from ad-hoc scripts as a
  client/WAF artifact unless NewAPI relay logs show the same request.

Current capability after the fix:

- NewAPI Gemini text is usable through the OpenAI-compatible route.
- `gemini-3.1-flash-image` is only proven usable as a chat/text model through
  this upstream. It is not proven as image generation.
- This upstream rejected `/v1/images/generations` for
  `gemini-3.1-flash-image`; if image generation is needed from this upstream,
  ask the upstream for exact `imagen-*` model IDs or a task-style image API.
- Grok channel config is present, but the filled upstream returned HTTP 500
  during the final public smoke test. Retest later or replace the upstream.
- No NewAPI video model/channel is currently exposed; there are no enabled
  `veo`, `video`, `sora`, `kling`, `hailuo`, `vidu`, `jimeng`, or
  `grok-imagine` abilities in NewAPI.

UAG image/video site status:

- Public image site is reachable.
- `https://image-api.vyywcw.cn/v1/models` correctly returns HTTP 401 without a
  key, so the image API is protected.
- Active UAG accounts currently observed:
  - `newapi-gpt-image-2`, provider `gpt`, group `gpt-image2-newapi`,
    whitelist `["gpt-image-2"]`, base URL `https://newapi.vyywcw.cn`.
  - `uag`, provider `gpt`, base URL `https://dtrljm.com`.
- UAG has model entries for Flow/Gemini/Veo image/video and
  `grok-imagine-video`, but no active Flow or native Grok account was observed.
  Therefore those model entries are product placeholders until real provider
  accounts are added.
- Practical state: GPT Image2 text-to-image is the only proven image path in
  UAG today. Image-to-image still needs separate validation because recent UAG
  tasks showed image-token counting errors. Video is not usable until a real
  Flow/Veo/Grok video provider account and create/poll/fetch protocol are
  configured and smoke tested.
