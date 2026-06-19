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
