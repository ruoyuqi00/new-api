# Kiro Next Fusion Plan

本文是 Kiro 后续融合升级清单，目标是把“能跑”逐步收敛成“可维护、可回滚、可批量导入”的 Sub2API 内部能力。

## 当前基线

代码基线：

- 私有 fork 已包含官方 upstream 到 `14f54be0`。
- 本 fork 新增 Kiro 导入桥：`POST /api/v1/admin/accounts/import/kiro`。
- 本轮新增增强：导入完整 Kiro 导出 JSON 时保留 `accessToken`、`refreshToken`、`profileArn`、`expiresAt`、`loginHint` 和嵌套 `kiro_auth_token_raw`。

服务器基线：

- Sub2API 对公网服务：`https://api.vyywcw.cn/`
- `kiro-gateway` 已作为内部 compose 服务运行。
- Sub2API 已有 `kiro-gateway-internal-anthropic` upstream account。
- 当前只公开调度已 smoke 通过的 Kiro 模型：
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.5`
  - `qwen3-coder-next`

Kiro correction:

- 本地 `D:\wflogin\kiro.rs-master` 已验证可以通过 `claude-sonnet-4-6` 返回 HTTP 200。
- 本地 `kiro.rs-master` 使用本机代理出口；服务器 `kiro-rs` 直连出口仍返回 `INVALID_MODEL_ID`。
- 后续不能再把问题简化为“kiro.rs 不支持 Claude”，应拆成：
  - admin import 是否保留完整 credential metadata。
  - 服务器出口是否等价于本地代理出口。
  - `q.<region>.amazonaws.com` 与 `runtime.<region>.kiro.dev` 两条路径分别支持哪些模型。

## 阶段 1：把 Kiro 接入从手工操作变成管理动作

目标：

- 不再通过手工 SQL 创建 Kiro gateway upstream account。
- 管理员可以在 Sub2API 后台完成：
  - 导入 Kiro credential JSON。
  - 查看非敏感账号状态。
  - 创建或更新 Kiro gateway upstream account。
  - 只启用已验证模型。

任务：

- [ ] 新增 Kiro runtime upstream 管理接口。
- [ ] 新增模型 allowlist 字段，默认只包含已 smoke 通过模型。
- [ ] 后台页面增加 “Kiro Gateway account status” 面板。
- [ ] 后台页面显示模型状态：`available`、`smoke_passed`、`disabled_due_to_error`。
- [ ] 所有接口响应继续递归脱敏 token、API key、Authorization header。

验收：

- 不使用 SQL 也能更新 `kiro-gateway-internal-anthropic`。
- 重新导入同一凭据不会重复创建账号。
- 后台页面不显示原始 token。

## 阶段 2：补 Kiro 模型 smoke 自动化

目标：

- 每次更新 `kiro-gateway` 或 Kiro 凭据后，能自动跑模型 smoke。
- smoke 结果决定是否允许模型进入 Sub2API mapping。

任务：

- [ ] 新增 server-side smoke 脚本。
- [ ] smoke 目标先固定四个已验证模型。
- [ ] Claude 系列单独记录失败原因，不自动启用。
- [ ] smoke 输出只写模型名、HTTP 状态、错误类型和短摘要，不写 token。
- [ ] smoke 结果写入规划文档或服务器 ops log。

建议模型矩阵：

```text
deepseek-3.2        required
glm-5               required
minimax-m2.5        required
qwen3-coder-next    required
minimax-m2.1        candidate
claude-sonnet-4.6   investigation
claude-sonnet-4.5   investigation
claude-haiku-4.5    investigation
auto-kiro           investigation
```

验收：

- 四个 required 模型全部 HTTP 200 才允许继续部署。
- 任一 required 模型失败时，不重启 Sub2API 或不扩大 mapping。

## 阶段 3：判断是否 fork/patch kiro.rs

目标：

- 决定是长期使用 `kiro-gateway`，还是把 `kiro.rs` 修到当前 Kiro runtime。

需要调研的差异点：

| 主题 | kiro.rs 当前状态 | kiro-gateway 当前状态 | 需要确认 |
| --- | --- | --- | --- |
| 主请求域名 | `q.<region>.amazonaws.com` | `runtime.<region>.kiro.dev` | Kiro 当前官方运行面是否完全迁移 |
| 模型策略 | Rust 静态映射偏 Claude | fallback + normalize + pass-through | 哪些模型应公开 |
| profile ARN | 已有相关字段 | runtime 请求要求 profile ARN | 缺失时如何恢复 |
| token refresh | social/idc/api_key 比较完整 | 支持 JSON/SQLite/env | 哪个更稳定 |
| Claude Code `/cc` | `kiro.rs` 有 `/cc/v1/messages` | 需验证 | 是否必须公开 |
| payload 限制 | 需复核 | 有 payload size guard | Kiro 当前大小限制 |

patch `kiro.rs` 的最小路线：

- [ ] 修复 admin `POST /api/admin/credentials` 丢弃 `accessToken`、`profileArn`、`expiresAt` 的问题。
- [ ] 新增 runtime endpoint 配置，默认 `runtime.<region>.kiro.dev`。
- [ ] 保留旧 endpoint 作为 fallback 或可配置项。
- [ ] 调整模型 ID 规范化，支持 dot 格式：`claude-sonnet-4.6`。
- [ ] 对未知模型采取 pass-through，而不是提前拒绝。
- [ ] 增加 open models：`deepseek-3.2`、`glm-5`、`minimax-m2.5`、`qwen3-coder-next`。
- [ ] 补 profile ARN header 逻辑和缺失错误提示。
- [ ] 增加 direct smoke 测试和 `/v1/models` 回归测试。
- [ ] 增加代理出口 smoke：直连、服务器代理、本地代理三种路径分别记录结果。

暂不做的事：

- 不把 Kiro refresh token 存入 Sub2API 主库。
- 不公开 `kiro-gateway` 或 `kiro-rs` 端口。
- 不把 Claude 系列 Kiro 模型加入 mapping，直到 smoke 通过。

## 阶段 4：Windsurf 与 Kiro 的统一 adapter 抽象

目标：

- 后台导入体验统一。
- 两类 adapter 都能返回统一的 per-item 结果。
- 后续扩展新 provider 时不用复制大量 handler/UI 代码。

抽象建议：

```text
ProviderImportAdapter
  - Parse(input)
  - Normalize(account)
  - SecretFingerprint(account)
  - Forward(ctx, account)
  - Redact(response)
  - BuildResult(item)
```

先不要急着抽象。等 Windsurf 和 Kiro 的第二轮需求稳定后再提炼，否则会把还在变化的协议过早固化。

## 阶段 5：运营和回滚

每次改 Kiro 相关服务前：

- [ ] 备份 `/opt/sub2api/docker-compose.yml`。
- [ ] 备份 `/opt/sub2api/kiro-gateway/creds`。
- [ ] 记录当前 Sub2API image tag。
- [ ] 记录当前 `kiro-gateway` image digest。
- [ ] 确认 adapter 端口未公开。

每次部署后：

- [ ] `docker compose ps`
- [ ] `GET http://kiro-gateway:8000/v1/models`
- [ ] Sub2API 内部 `qwen3-coder-next` smoke
- [ ] Sub2API 公网 `deepseek-3.2` smoke
- [ ] 查询 Sub2API account 是否 active/schedulable

回滚：

- 如果 `kiro-gateway` 失败，只禁用 `kiro-gateway-internal-anthropic` 或从 group 移除，不影响 Windsurf。
- 如果 compose 失败，恢复最近的 `docker-compose-*-pre-kiro-gateway.yml`。
- 如果新 Sub2API image 失败，切回前一个 image tag。
