# YuCore 多模态兼容接入设计（2026-07-12）

## 1. 审计范围与结论

本阶段对 `image.cjyyswq.com` 的公开文档、已登录账号可见模型、令牌权限边界，以及 YuAPI、YuCore Studio 和无限画布现有实现进行了对照审计。

结论：不应逐页、逐文案或逐模型照搬目标站点，也不需要为每个模型创建一个渠道。应复用它清晰的协议边界，将“上游凭据与结算边界”和“模型能力描述”分开管理，在 YuAPI 内形成自己的通用多模态兼容层。

推荐粒度：

- 渠道负责上游地址、凭据、协议类型、结算边界、并发、优先级和权重。
- 模型配置负责模型映射、输入类型、同步/异步模式、尺寸、比例、质量、时长策略和结果解析规则。
- Studio 只消费规范化后的模型能力，不感知当前请求最终落到传统渠道还是账号池。
- 同一凭据可以承载多个模型；只有凭据、权限、计费或限流边界不同时才拆分渠道。

## 2. 目标站点协议审计

公开文档包含快速开始、请求格式、中转说明、定价、错误码、图片和视频等独立章节。其主要价值不是章节数量，而是把同步图片、异步任务、媒体输入和时长策略分开说明。

### 2.1 端点

| 能力 | 方法与路径 | 返回模式 |
| --- | --- | --- |
| 文生图 | `POST /v1/images/generations` | 同步图片结果 |
| 图生图 | `POST /v1/images/edits` | 同步图片结果 |
| 视频或异步图片 | `POST /v1/videos` | 先返回任务 ID |
| 异步任务查询 | `GET /v1/videos/{task_id}` | 轮询状态与结果 |

同步图片结果位于 `data[0].url` 或 `data[0].b64_json`。异步创建响应的任务 ID 可能位于：

- `id`
- `task_id`
- `data.id`
- `data.task_id`

异步任务的媒体结果可能位于：

- `video_url`
- `url`
- `data.video_url`
- `data.url`
- `output[0].url`
- `data.output[0].url`
- `metadata.result_urls[0]`

状态需要归一化为 YuCore 的五态：

| 上游状态 | YuCore 状态 |
| --- | --- |
| `pending`, `queued` | `pending` |
| `processing`, `running` | `processing` |
| `completed`, `succeeded`, `success` | `completed` |
| `failed`, `error` | `failed` |
| `cancelled`, `canceled` | `canceled` |

客户端建议每 5 秒查询一次已有任务，查询过程不得重复创建任务。

### 2.2 媒体与时长约束

- 上游参考图和参考视频要求公网 HTTPS 直链。
- `duration` 和 `seconds` 是不同模型族的字段，不能同时无差别发送。
- 固定时长模型需要完全省略 `duration` 与 `seconds`。
- 不支持时长参数的模型也应完全省略这两个字段。
- 图片、视频和异步图片可能共用任务端点，但在 YuCore 内仍需保留真实 `kind`，以便预览、下载、计费和画布节点正确工作。

### 2.3 账号配置观察

已登录账号内可见的令牌按文字、图片、视频、任意比例和固定时长等用途拆分，但部分令牌实际暴露了相同的一组 Grok 文字、图片和视频模型。这说明目标站点的细分主要服务于权限、计费和使用说明，并不能推导出“一个模型必须对应一个渠道”。

本项目不得持久化目标站点登录密码或完整令牌。真实调用时只临时读取指定令牌，生产配置使用写入后不可回显的密钥路径。

## 3. YuAPI 与 Studio 现状

### 3.1 已具备能力

YuAPI 已具备以下基础能力：

- `/v1/images/generations` 与 `/v1/images/edits` 图片转发。
- `/v1/videos` 与 `/v1/videos/{task_id}` 异步任务路由。
- OpenAI-compatible 图片响应中的 URL 和 Base64 结果处理。
- YuCore 媒体任务持久化、任务状态、素材代理和画布结果回流。
- Studio 模型能力字段，包括模式、尺寸、比例、质量、格式、数量、时长、参考图上限和定价元数据。
- 无限画布发起任务、轮询任务、将成功素材写回结果节点并持久化画布版本。
- 管理端通过 `yucore_media.*` 选项控制适配器、模型白名单、模型映射和真实素材要求。

### 3.2 当前差距

现有 `openai-compatible` Studio 适配器只支持同步图片，并且总是调用 `/v1/images/generations`；图生图虽然在 UI 模型能力中可选，但没有按输入素材切换到 multipart `/v1/images/edits`。

现有 UAG 适配器使用自定义 `/api/v1/gen/*` 协议，不能直接把目标站点的 `/v1/videos` 协议当作 UAG 接入。OpenAI 渠道的视频路由虽然已有 Sora 任务适配器，但 YuCore Studio 的独立适配器没有复用完整的渠道调度、账号池、计费和任务规范化能力。

需要补齐：

- 通用 OpenAI-compatible 异步任务创建与查询。
- 嵌套任务 ID、嵌套状态和多种结果 URL 位置解析。
- 异步图片任务的 `kind=image` 结果回流。
- `/v1/images/edits` multipart 请求和多参考图上传。
- 公网 HTTPS 素材 URL 校验与本地上传素材的对外可达性检查。
- 每个模型的时长字段策略，而不是统一发送 `duration`。
- 模型级参数白名单，避免把某模型不支持的字段转发给上游。
- Studio 媒体请求进入 YuAPI 标准渠道/账号池调度和标准计费路径，避免旁路使用单一全局密钥。
- 真实调用成功后再把 `upstream_verified` 标记为已验证。

## 4. 推荐兼容模型

### 4.1 渠道边界

建议按以下条件之一拆分渠道：

- 上游 Base URL 不同。
- API 凭据或账号池不同。
- 上游结算账户、余额或发票边界不同。
- 并发、速率限制或故障隔离要求不同。
- 安全策略或允许用户分组不同。

不要仅因为模型名不同就拆分渠道。以当前目标站点测试为例，第一阶段只需一个图片测试渠道；后续视频凭据若计费或并发边界不同，再单独建视频渠道。

### 4.2 模型能力结构

模型能力应作为结构化配置保存，建议语义如下：

```json
{
  "model": "grok-imagine-image-quality",
  "upstream_model": "grok-imagine-image-quality",
  "kind": "image",
  "modes": ["text-to-image", "image-to-image"],
  "transport": "sync-image",
  "create_path": "/v1/images/generations",
  "edit_path": "/v1/images/edits",
  "input_media": ["image"],
  "duration_policy": "none",
  "duration_field": "",
  "sizes": ["1k", "2k"],
  "aspect_ratios": ["1:1", "16:9", "9:16"],
  "qualities": ["standard", "high"],
  "response_formats": ["url", "b64_json"],
  "max_reference_images": 3,
  "poll_interval_seconds": 5
}
```

对于公开模型 ID 中带有最大分辨率的图片模型，`sizes` 表示能力上限而不是
固定输出形状：2K 模型开放 1K 和 2K，4K 模型开放 1K、2K 和 4K。选中的
分辨率和 `aspect_ratio` 相互独立。转发层会根据上限计算标准宽高，最长边
不会超过选中的分辨率，因此横图和竖图不会被默认强制成正方形。

`transport` 至少支持：

- `sync-image`
- `async-video-task`
- `async-image-task`

`duration_policy` 至少支持：

- `duration`：发送 `duration`。
- `seconds`：发送 `seconds`。
- `fixed`：完全省略时长字段，只展示固定时长说明。
- `none`：完全省略时长字段，UI 不展示时长控件。

固定时长本身可用 `fixed_duration_seconds` 表示。字段是否发送必须由适配器根据策略决定，不能由前端直接拼接。

### 4.3 结果规范化

兼容层统一输出：

```json
{
  "task_id": "provider-task-id-or-local-id",
  "kind": "image",
  "status": "completed",
  "progress": 100,
  "assets": [
    {
      "kind": "image",
      "source_url": "https://provider.example/result.png",
      "mime_type": "image/png"
    }
  ]
}
```

解析器应使用有序候选路径，找不到结果时保留上游响应摘要用于诊断，但不得记录 Authorization、Cookie、完整令牌或包含敏感查询参数的 URL。

## 5. 调度与计费边界

Studio 和无限画布不应直接使用全局 `YUCORE_MEDIA_API_KEY` 作为最终生产路径。最终请求应进入 YuAPI 标准路由，以便统一获得：

- 用户分组与模型权限检查。
- 渠道优先级、权重、并发与冷却。
- 账号池内账号级并发、健康状态和重试。
- 预扣费、真实结果结算、失败退款和日志审计。
- 上游请求 ID、渠道 ID、账号 ID 和模型映射记录。

第一阶段真实验证可以临时使用写入后不可回显的测试渠道凭据，但不得把临时密钥提交到 Git。生产启用前必须验证图片数量、尺寸/质量倍率和异步任务最终结算不会重复扣费。

## 6. Studio 与无限画布接入

Studio 只依赖 `/api/yucore/media/models` 返回的能力元数据：

- 仅展示当前用户分组可用模型。
- 根据 `modes` 决定文生图、图生图、文生视频等入口。
- 根据模型能力动态显示尺寸、比例、质量、格式、数量、时长和参考素材控件。
- `duration_policy=fixed` 时显示固定值但不让请求体发送时长字段。
- 不支持某项能力时不展示控件，也不向后端发送该字段。

无限画布继续以 YuCore 媒体任务作为稳定边界。画布节点只保存 YuCore 任务 ID、模型 ID、参数快照和规范化素材，不保存上游凭据。任务完成后通过现有 backflow 机制更新提示节点、任务节点和结果节点。

## 7. 分阶段实施清单

### 阶段二：通用协议兼容层

- 为 OpenAI-compatible Studio 适配器增加同步图片、图生图和异步任务三种传输模式。
- 增加任务 ID、状态和结果 URL 的多路径规范化。
- 增加模型级时长策略和参数过滤。
- 使用 `common.Marshal`、`common.Unmarshal` 和 `common.DecodeJson`，不新增直接 JSON 编解码调用。
- 添加确定性的 Go 单元测试，覆盖协议契约和失败分支。

### 阶段三：目标站点真实生图与画布验证

- 安全读取现有图片令牌，创建测试渠道并检查 `/v1/models`。
- 使用成本明确的最低规格执行一次文生图。
- 验证同步结果 URL 或 Base64 结果进入 YuCore 媒体任务。
- 使用普通用户从 Studio 发起一次真实任务。
- 使用无限画布发起一次任务并验证结果节点、保存、刷新恢复和素材下载。
- 记录调用模型、渠道、请求 ID、扣费前后差额和结果状态，不记录完整密钥。

### 阶段四：生产化与部署

- 将测试凭据改为生产写入后不可回显配置。
- 配置模型白名单、分组、价格、并发和失败冷却。
- 完成 Go 测试、TypeScript、lint、格式、生产构建和浏览器 E2E。
- 提交并推送当前功能分支。
- 只重建并替换 `newapi` 应用容器，不修改 MySQL 与 Redis 数据容器。
- 部署后验证健康接口、普通用户生图、无限画布回流、计费日志和失败重试。

## 8. 真实调用测试矩阵

| 场景 | 请求 | 必须验证 |
| --- | --- | --- |
| 文生图 URL | `/v1/images/generations` | `data[0].url`、素材代理、预览、下载、一次扣费 |
| 文生图 Base64 | `/v1/images/generations` | `b64_json` 转 data URL、MIME、画布保存恢复 |
| 图生图 | `/v1/images/edits` | multipart、多参考图、输入限制、结果回流 |
| 异步图片 | `/v1/videos` + 查询 | 图片 `kind`、状态归一化、只创建一次任务 |
| 异步视频 | `/v1/videos` + 查询 | 任务 ID 变体、结果 URL 变体、5 秒轮询 |
| `duration` 模型 | 创建任务 | 只发送 `duration` |
| `seconds` 模型 | 创建任务 | 只发送 `seconds` |
| 固定时长模型 | 创建任务 | 两个字段都不发送 |
| 上游失败 | 任一端点 | 错误归一化、失败退款、渠道冷却、无重复扣费 |
| 刷新恢复 | Studio/画布 | 任务与节点仍对应同一上游任务，不重复创建 |

## 9. 阶段一验收结果

- 已明确目标站点协议与账号配置的有效粒度。
- 已确认 YuAPI 基础路由、Studio 和无限画布不需要重做。
- 已确认不能直接照搬目标站点，也不应按模型一对一创建渠道。
- 已形成通用兼容层、真实测试和生产部署的顺序与验收标准。
- 本文未包含登录密码、完整 API Key 或可复用会话信息。

## 10. 阶段三、阶段四生产验收（2026-07-13）

本节记录最终生产事实；如与第 3.2 节的实施前差距描述冲突，以本节为准。

### 10.1 已部署能力

- `openai-compatible` 兼容层已支持同步文生图、multipart 图生图和异步图片/视频任务。
- 新增 `yuapi-channel` Studio 适配器。Studio 和无限画布使用当前用户的受管令牌进入 YuAPI 标准分组、渠道调度和计费链路，用户只选择模型与分组。
- 图片 URL、Base64、嵌套任务 ID、状态变体和多种结果 URL 已统一归一化为 YuCore 媒体任务与素材。
- 生产图片渠道 ID 为 `2330`，受管令牌 ID 为 `96`，分组为 `grok按次`。这些标识可用于审计，但不包含任何可调用凭据。
- 已验证模型为 `grok-imagine-image-quality`，公开单价为 `0.032`，当前一次调用对应 YuAPI 配额 `16000`。
- 生产应用镜像为 `newapi:yucore-multimodal-20260712-578b908bd`；MySQL 与 Redis 数据容器未替换。

### 10.2 真实链路结果

| 链路 | 结果 | 调度与计费 | 结果回流 |
| --- | --- | --- | --- |
| 上游直连最低规格文生图 | 成功 | 用于确认凭据、模型和响应格式 | Base64 图片有效 |
| YuAPI 标准图片接口 | HTTP 200 | 渠道 `2330`，扣费 `16000` | Base64 图片有效 |
| 普通用户 YuCore Studio | 成功 | 渠道 `2330`、受管令牌 `96`、扣费 `16000` | 素材可预览和下载，任务持久化成功 |
| 普通用户无限画布 Agent | 成功 | 渠道 `2330`、受管令牌 `96`、扣费 `16000` | Task 节点、Agent Run 和 `canvas_apply_result` 均完成回流 |

最终核验状态：

- `yucore_media.upstream_verified=true`。
- `TurnstileCheckEnabled=true`，测试期间的临时关闭已恢复。
- `yucore_media_tasks.assets` 在 MySQL 中为 `LONGTEXT`，可持久化本次约 374-402 KB 的 Base64 素材描述。
- `newapi` 容器为 `healthy`，当前生产镜像与本节记录一致。
- 阶段性失败/停滞的 QA 任务和临时画布已软删除；成功任务与消费日志保留为审计证据。
- 用户 `quota` 会在请求内同步扣减，`used_quota` 由批处理稍后汇总。验收应以同步额度差额和消费日志共同判断，不应把瞬时 `used_quota` 未变化误判为未计费。

### 10.3 本轮修复的生产问题

1. Base64 图片超过 MySQL `TEXT` 的 64 KB 上限，导致 Studio 任务在 96% 后落库失败。媒体素材字段现按方言映射：MySQL 使用 `LONGTEXT`，SQLite/PostgreSQL 使用 `TEXT`。
2. Studio 创建入口已放行 `yuapi-channel`，但无限画布 Agent 创建入口仍保留旧白名单，导致任务创建后不执行。两个入口现统一使用 `isYucoreMediaRunnableAdapter`，并增加了画布经受管渠道实际派发的回归测试。
3. 生产验证只在 Studio、素材下载、无限画布节点回流、Agent Run 回流和计费日志全部成功后才将 `upstream_verified` 设为 `true`。

### 10.4 验证与版本状态

- 后端：`go test ./model ./controller ./relay/...` 通过。
- 前端：TypeScript、定向 lint、格式检查和 default 生产构建已通过。
- 容器：生产 Docker 镜像构建、加载、数据库自动迁移和健康检查通过。
- Git：阶段三与阶段四修复均提交到 `feature/yucore-ui-polish-20260710` 并推送同名远端分支。

## 11. 后续生视频接入步骤

生视频继续复用现有兼容层，不复制目标站点页面、业务代码或密钥管理方式。拿到视频 API 后按以下顺序推进：

1. 只读审计视频令牌可见模型、创建端点、查询端点、任务 ID、状态、结果 URL、并发和计费规则，不记录完整令牌。
2. 凭据、结算账户、并发或故障隔离边界与图片相同时复用现有渠道；任一边界不同时再建立独立视频渠道。
3. 为每个视频模型配置 `transport=async-video-task`、`create_path`、`status_path`、轮询间隔、参考素材上限和参数白名单。
4. 按模型真实语义选择 `duration`、`seconds`、`fixed` 或 `none`，禁止同时发送多个时长字段。
5. 先做一次最低成本直连调用，再做一次 YuAPI 标准渠道调用，确认只创建一个上游任务且只结算一次。
6. 最后分别验证 Studio 与无限画布：轮询恢复、取消、失败退款、素材下载、刷新恢复、节点回流和 Agent Run 回流。
7. 全链路成功后再加入生产模型白名单，并将对应上游验证状态改为已验证。
