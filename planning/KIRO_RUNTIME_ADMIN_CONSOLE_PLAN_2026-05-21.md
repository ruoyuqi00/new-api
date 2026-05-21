# Kiro Runtime Admin Console Plan - 2026-05-21

本文定义下一轮 Kiro 管理后台和内部 runtime 的改造方向。目标是把现在
“临时导入桥 + 内部 adapter”的形态，升级成可以长期维护的 Kiro Runtime
控制台。

不要在本文档写入任何原始 token、refresh token、密码、cookie、API key、
SSH 凭据或服务器登录信息。

## 结论

Kiro 管理后台需要跟着 runtime 方案一起升级。后续不应该只保留一个
“导入 Kiro”弹窗，而应该在 Sub2API 管理后台内增加一个完整的
`Kiro Runtime` 页面。

推荐架构：

```text
外部调用方
  -> Sub2API public endpoint
    -> Sub2API channel / group / key routing
      -> internal kiro-runtime
        -> engine: Kiro Web Portal route
        -> engine: Kiro IDE / Amazon Q / CodeWhisperer route
        -> engine: legacy kiro.rs or kiro-gateway fallback
```

关键原则：

- Sub2API 仍然是唯一公网入口。
- Kiro runtime 后台不单独暴露公网端口。
- Kiro runtime 可以换底层 engine，但对 Sub2API 暴露稳定接口。
- 模型是否公开给外部用户，只由 smoke 结果和 Sub2API allowlist 决定。
- 账号凭据只在内部 runtime 或其加密存储中存在，Sub2API 页面只显示脱敏状态。

## 参考项目优先级

从本文件开始，后续 Kiro 开发按以下优先级参考，避免继续围绕已经落后的
项目做主路线设计。

| 等级 | 项目 | 用途 | 处理方式 |
| --- | --- | --- | --- |
| P0 | 本 fork 的 Sub2API private adapter | 公网入口、管理后台、key/group/channel 体系 | 主系统，所有公开行为以它为准。 |
| P0 | 本 fork 的 `adapters/kiro-web` | 当前 Web Portal Claude/Opus 路线 | 作为 Claude/Opus engine 继续维护。 |
| P1 | `Quorinex/Kiro-Go` | 服务端账号池、Go/Docker 结构、模型感知路由 | 优先借鉴并重写到我们的内部 runtime。 |
| P1 | `chaogei/Kiro-account-manager` | KAM 账号池策略、断路器、sticky/round-robin、token refresh lock、模型发现 | 学习行为和策略，不直接复制 AGPL 代码。 |
| P2 | `Jwadow/kiro-gateway` | 已验证 open-model runtime 路线 | 继续保留为线上 fallback/reference。 |
| P2 | `hank9999/kiro.rs` | 旧 IDE/API key 路线、历史行为、回滚参考 | 不再作为新架构主参考。 |
| P3 | `opencode-kiro-auth` / `pi-kiro` / `cockpit-tools` | OAuth、CLI、账号导出、环境探测的补充样本 | 只在对应子问题需要时查。 |

当前观察版本：

- `chaogei/Kiro-account-manager`: `7ad57fd`, tag `v1.6.6`
- `Quorinex/Kiro-Go`: `68110f3`, version `1.0.8`
- `hank9999/kiro.rs`: `f1bbe9f`, tag `v2026.3.1`

## 管理后台页面结构

入口建议放在 Sub2API 管理后台：

```text
管理后台
  -> Provider Adapters
    -> Kiro Runtime
```

如果短期不新增完整导航组，可以先把入口挂在现有账号管理页的
`Kiro Runtime` 按钮上，但最终应该独立成页。

### 1. 总览区

显示只读摘要：

- runtime 状态：online / degraded / offline
- 当前 engine：web-portal / ide-amazonq / kiro-go / legacy-kiro-rs
- 账号总数、可用账号数、冷却账号数、额度耗尽账号数
- 可公开模型数量、待验证模型数量、禁用模型数量
- 最近一次模型发现时间
- 最近一次 smoke 时间
- Sub2API 同步状态

### 2. 账号池

表格字段：

- 账号标识：email 或脱敏 account id
- auth method：web_portal / social / builder_id / api_key / cli_sso
- engine：web-portal / ide / amazonq / legacy
- plan：Free / Pro / Pro+ / Power / unknown
- region
- profileArn 状态：present / missing
- token 状态：valid / expiring / expired / refresh_failed
- usage：used / limit / reset_at
- runtime 状态：available / cooldown / quota_exhausted / disabled / auth_failed
- 最近使用时间
- 连续错误数
- 支持模型数

操作：

- 刷新账号信息
- 刷新 token
- 测试账号
- 查看脱敏详情
- 禁用/启用
- 删除
- 重新发现模型

所有操作返回都必须脱敏。

### 3. 导入账号

导入方式：

- KAM export JSON
- Kiro-Go account JSON
- 本 fork 当前完整 JSON
- refresh token line
- API key line
- email + credential line

导入行为：

- 保留 `profileArn`、`region`、`machineId`、`provider`、`authMethod`、
  `clientId/clientSecret` 的存在状态，但不在页面显示原文。
- 同一账号重复导入时做 upsert，不重复创建。
- per-item 返回导入结果：created / updated / skipped / failed。
- 错误中不能带原始凭据。

### 4. 路由策略

配置项：

- 默认账号选择：`round-robin` / `sticky`
- session sticky：on / off
- 模型感知路由：on / off
- 失败自动切账号：on / off
- 额度耗尽自动切账号：on / off
- 是否允许 overage：global off by default
- 每账号权重：默认 1
- 冷却时间策略：base cooldown + max cooldown

推荐默认值：

```text
default_strategy = round-robin
session_sticky = true
model_aware_routing = true
auto_switch_on_quota = true
allow_overage = false
```

### 5. 模型管理

模型表字段：

- model id
- display name
- 来源：official discovery / hidden / static fallback / smoke only
- 支持账号数
- last smoke status
- last smoke error type
- 是否公开到 Sub2API
- 是否允许进入 group mapping
- 禁用原因

关键规则：

- 只把 smoke 通过的模型同步到 Sub2API。
- Claude/Opus 系列即使出现在隐藏模型或静态表里，也必须 direct smoke 和
  public smoke 都通过后才能公开。
- 如果官方发现模型少于预期，页面要显示“当前账号/出口/region 下未发现”，
  不要自动猜测为可用。

### 6. Smoke 测试

支持：

- 单账号单模型测试
- 单模型跨账号池测试
- 全量 required model 测试
- Sub2API public smoke 测试

记录字段：

- model id
- account id 脱敏值
- engine
- HTTP status
- error type
- short summary
- latency
- timestamp

禁止记录：

- prompt 原文
- access token
- refresh token
- Authorization header
- 完整上游响应体中的敏感字段

### 7. 同步到 Sub2API

页面需要一个“同步到 Sub2API”区域：

- 创建或更新 Kiro upstream account
- 更新 model mapping
- 更新 group 可用模型
- 标记当前同步版本
- 提供 dry-run

默认只同步：

- runtime 可用
- 至少一个账号支持
- smoke 通过
- 未被管理员禁用

## 后端接口草案

兼容保留现有接口：

```text
POST /api/v1/admin/accounts/import/kiro
```

新增 runtime 管理接口建议：

```text
GET  /api/v1/admin/provider-adapters/kiro/runtime/status
GET  /api/v1/admin/provider-adapters/kiro/accounts
POST /api/v1/admin/provider-adapters/kiro/accounts/import
GET  /api/v1/admin/provider-adapters/kiro/accounts/{id}
POST /api/v1/admin/provider-adapters/kiro/accounts/{id}/refresh
POST /api/v1/admin/provider-adapters/kiro/accounts/{id}/discover-models
POST /api/v1/admin/provider-adapters/kiro/accounts/{id}/smoke
PATCH /api/v1/admin/provider-adapters/kiro/accounts/{id}
DELETE /api/v1/admin/provider-adapters/kiro/accounts/{id}

GET  /api/v1/admin/provider-adapters/kiro/models
POST /api/v1/admin/provider-adapters/kiro/models/discover
POST /api/v1/admin/provider-adapters/kiro/models/smoke
PATCH /api/v1/admin/provider-adapters/kiro/models/{model_id}

GET  /api/v1/admin/provider-adapters/kiro/routing
PUT  /api/v1/admin/provider-adapters/kiro/routing

POST /api/v1/admin/provider-adapters/kiro/sync-sub2api
```

## 数据结构草案

```json
{
  "account": {
    "id": "kiro_xxx",
    "label": "user***@example.com",
    "auth_method": "web_portal",
    "engine": "web-portal",
    "region": "us-east-1",
    "plan_name": "KIRO PRO",
    "profile_arn_present": true,
    "token_status": "valid",
    "runtime_status": "available",
    "usage_current": 44.6,
    "usage_limit": 1000,
    "usage_reset_at": 1780272000,
    "error_count": 0,
    "cooldown_until": null,
    "last_used_at": 1778752272,
    "supported_model_count": 5
  }
}
```

```json
{
  "model": {
    "id": "qwen3-coder-next",
    "display_name": "Qwen3 Coder Next",
    "source": "official_discovery",
    "supported_account_count": 1,
    "last_smoke_status": "passed",
    "public_enabled": true,
    "sync_to_sub2api": true,
    "disabled_reason": null
  }
}
```

```json
{
  "routing": {
    "default_strategy": "round-robin",
    "session_sticky": true,
    "model_aware_routing": true,
    "auto_switch_on_quota": true,
    "allow_overage": false,
    "base_cooldown_seconds": 60,
    "max_cooldown_seconds": 86400
  }
}
```

## 实施阶段

### 阶段 0：文档和参考策略

- [x] 明确 KAM/Kiro-Go 是下一轮主要参考。
- [x] 将 `kiro.rs` 降级为 legacy/reference。
- [x] 写入本控制台改造计划。

### 阶段 1：只读 Kiro Runtime 页面

- [ ] 新增 Kiro Runtime 页面。
- [ ] 显示 runtime 状态、账号池摘要、模型摘要。
- [ ] 复用现有 Kiro import handler，不改变现有导入入口。
- [ ] 所有字段先只读。

验收：

- 登录 Sub2API 后台能看到 Kiro Runtime 页面。
- 页面不显示任何原始 token。
- 如果内部 runtime 不在线，页面显示 degraded/offline，而不是空白崩溃。

### 阶段 2：账号导入和账号状态

- [ ] 把现有 Kiro 导入弹窗迁移或复用到 Runtime 页面。
- [ ] 支持 KAM export JSON 和 Kiro-Go JSON。
- [ ] 增加账号状态刷新。
- [ ] 增加 per-item import 结果表。

验收：

- 同一账号重复导入只更新，不重复创建。
- 错误结果脱敏。
- 可看到账号是否有 profileArn、region、plan、usage。

### 阶段 3：路由策略和模型发现

- [ ] 增加 routing config 后端接口。
- [ ] 增加模型发现接口。
- [ ] 增加 model-aware account selection。
- [ ] 增加 refresh single-flight。

验收：

- 多账号时 round-robin 生效。
- 同一 session sticky 生效。
- 目标模型只选支持该模型的账号。

### 阶段 4：Smoke 和 Sub2API 同步

- [ ] 增加 smoke 接口和页面按钮。
- [ ] 记录最近 smoke 结果。
- [ ] 增加同步到 Sub2API 的 dry-run。
- [ ] 只同步 smoke passed 模型。

验收：

- public enabled 模型必须有 direct smoke 记录。
- 同步前能看到将要修改的 account/model mapping/group。
- Claude/Opus 不会因为出现在隐藏列表就自动公开。

### 阶段 5：底层 engine 可替换

- [ ] 定义 engine 接口。
- [ ] Web Portal route 作为 engine A。
- [ ] Kiro-Go/AmazonQ route 作为 engine B。
- [ ] Legacy kiro.rs/kiro-gateway 作为 fallback/reference。

验收：

- 切换 engine 不需要改 Sub2API 对外 API。
- 任一 engine 失败可以禁用，不影响其他 provider。

## 前端文件建议

可能涉及：

```text
frontend/src/views/admin/ProviderAdaptersView.vue
frontend/src/components/admin/account/KiroImportModal.vue
frontend/src/components/admin/account/kiroImport.ts
frontend/src/types/index.ts
frontend/src/i18n/locales/zh.ts
frontend/src/i18n/locales/en.ts
```

建议新增：

```text
frontend/src/views/admin/KiroRuntimeView.vue
frontend/src/components/admin/kiro/KiroRuntimeSummary.vue
frontend/src/components/admin/kiro/KiroAccountPoolTable.vue
frontend/src/components/admin/kiro/KiroModelMatrix.vue
frontend/src/components/admin/kiro/KiroRoutingSettings.vue
frontend/src/components/admin/kiro/KiroSmokePanel.vue
frontend/src/components/admin/kiro/KiroSyncPanel.vue
```

## 后端文件建议

可能涉及：

```text
backend/internal/handler/admin/account_kiro_import.go
backend/internal/handler/admin/account_kiro_import_test.go
backend/internal/server/routes
backend/internal/service
backend/internal/model
```

建议新增：

```text
backend/internal/handler/admin/provider_kiro_runtime.go
backend/internal/service/kiro_runtime_admin.go
backend/internal/service/kiro_runtime_admin_test.go
```

## 安全规则

- 管理页面永远不显示原始 token。
- 日志永远不打印 Authorization header。
- 导入接口必须递归脱敏嵌套字段。
- Kiro runtime 服务不开放公网端口。
- Caddy 只暴露 Sub2API。
- Smoke prompt 使用固定短文本，不使用用户真实 prompt。

## 回滚

- 如果 Runtime 页面出错，保留旧 `Import Kiro` 弹窗入口。
- 如果新 runtime 调度出错，只禁用 Kiro upstream account，不影响 Windsurf。
- 如果模型同步出错，回滚 Sub2API model mapping/group 配置。
- 如果新 engine 出错，切回当前已验证的 `kiro-gateway`/`kiro-web` 路径。

## Sources

- https://github.com/chaogei/Kiro-account-manager
- https://github.com/Quorinex/Kiro-Go
- https://github.com/hank9999/kiro.rs
