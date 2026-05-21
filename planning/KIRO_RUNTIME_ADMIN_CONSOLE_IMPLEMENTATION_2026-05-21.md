# Kiro Runtime Admin Console Implementation - 2026-05-21

本文记录本轮已经落地到 `sub2api-private` 的 Kiro Runtime 管理入口改造。不要在本文档写入任何原始 token、refresh token、密码、cookie、API key、SSH 凭据或服务器登录信息。

## 本轮结论

本轮先做“Sub2API 后台统一只读查看”，不直接开放 Kiro/Windsurf 容器端口，也不在 Sub2API 页面显示原始凭据。

实现后的入口：

```text
Sub2API 管理后台
  -> 适配器后台
    -> /admin/provider-adapters/kiro
```

该页面会通过 Sub2API 管理员会话读取内网 Kiro adapter 的脱敏管理接口，并展示：

- Kiro Runtime 总览状态
- Kiro account pool 摘要
- Kiro 账号表
- Kiro 模型表
- 当前默认路由策略
- 脱敏 raw JSON

## 已新增后端接口

```text
GET /api/v1/admin/provider-adapters/kiro/runtime/status
GET /api/v1/admin/provider-adapters/kiro/runtime/accounts
GET /api/v1/admin/provider-adapters/kiro/runtime/models
GET /api/v1/admin/provider-adapters/kiro/runtime/routing
```

兼容保留：

```text
GET /api/v1/admin/provider-adapters/kiro/credentials
```

这些接口当前是只读接口，主要用于把 Kiro Runtime 的状态拉进 Sub2API 后台。后续如果替换底层实现为 KAM/Kiro-Go 风格服务，Sub2API 前端可以保持同一组接口不变。

## 已新增前端能力

涉及文件：

```text
frontend/src/types/index.ts
frontend/src/api/admin/accounts.ts
frontend/src/components/admin/account/ProviderAdaptersPanel.vue
frontend/src/views/admin/ProviderAdaptersView.vue
```

页面行为：

- `/admin/provider-adapters/kiro` 默认打开 Kiro Runtime 状态。
- 顶部保留 Windsurf/Kiro 原生后台入口，但主要状态已经在 Sub2API 页面内聚合。
- Kiro Runtime 区块展示账号、模型、路由摘要。
- 账号表只显示账号标识、plan、runtime 状态、token 状态、usage，不显示原始 token。
- 模型表显示 model id、来源、smoke 状态、公网公开状态。
- Raw JSON 区域展示后端已经脱敏后的返回，方便排查字段兼容问题。

## 当前数据归一化策略

为了兼容旧的 `kiro.rs`、当前 `kiro-web`、未来 KAM/Kiro-Go 风格 runtime，本轮后端会对多个可能字段名做归一化：

- 账号列表：`credentials` / `accounts` / `items` / `data`
- 模型列表：`models` / `data` / `availableModels` / `available_models` / `items`
- 账号状态：`status` / `runtime_status` / `runtimeStatus`
- token 状态：优先读取 `token_status`，否则按 `expires_at` 推断
- usage：兼容 `credits_used`、`credits_total`、`usage_current`、`usage_limit`、`currentUsageWithPrecision`
- profile ARN：只暴露是否存在，不暴露原文

这样做的目的，是后续底层 adapter 返回格式有变化时，Sub2API 页面不会立刻失效。

## 当前默认路由声明

当前页面展示的 routing 是只读声明，不代表已经完整替换底层调度器：

```text
default_strategy = round-robin
session_sticky = true
model_aware_routing = true
auto_switch_on_quota = true
allow_overage = false
```

后续真正落地时，应该按 Kiro-Go/KAM 的经验补齐：

- account pool 并发安全轮询
- 按模型选择账号
- token refresh single-flight
- 401/403 触发刷新
- 402/429 触发额度或冷却切换
- 5xx 短退避重试
- session sticky

## 参考项目优先级

从本轮开始，后续 Kiro runtime 迭代优先参考：

1. 本 fork 的 Sub2API provider adapter：决定公网 API、后台入口、key/group/channel 行为。
2. 本 fork 的 `adapters/kiro-web`：继续维护当前 Claude/Opus Web Portal 路线。
3. `Quorinex/Kiro-Go`：优先参考服务端化、Go 账号池、Docker、按模型调度。
4. `chaogei/Kiro-account-manager`：优先参考账号池策略、错误分级、token refresh lock、session sticky、模型发现。
5. `Jwadow/kiro-gateway`：保留为 open-model fallback/reference。
6. `hank9999/kiro.rs`：降级为 legacy/reference，不再作为新架构主参考。

注意：KAM 是 AGPL-3.0，当前策略是学习行为和策略，不直接复制其源码进私有 fork。

## 验证结果

本轮本地验证：

```text
backend: go test ./internal/handler/admin -run Kiro
backend: go test ./...
frontend: npm run build
```

结果均通过。

## 下一轮建议

1. 在服务器上拉取本次提交并重启 Sub2API，让公网后台看到新的 Kiro Runtime 页面。
2. 用真实 Kiro adapter 返回值检查归一化字段是否完整。
3. 如果 Kiro Runtime 页面可以看到账号但模型为空，下一步优先补模型发现接口兼容。
4. 如果模型能看到但外部调用失败，下一步做 direct smoke 和 public smoke 双层验证。
5. 再往后才做写操作：导入、刷新 token、禁用账号、同步模型到 Sub2API。

