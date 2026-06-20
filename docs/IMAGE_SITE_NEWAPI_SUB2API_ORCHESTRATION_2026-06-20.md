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
