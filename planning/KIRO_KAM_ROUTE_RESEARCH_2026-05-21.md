# Kiro KAM Route Research - 2026-05-21

本记录用于回答“是否应该参考 KAM 来升级我们的 Kiro 账号路由、并发和模型调度”。
不要在本文档写入任何原始 token、refresh token、密码、cookie、API key 或服务器登录信息。

## 结论

`chaogei/Kiro-account-manager` 确实是目前比旧 `kiro.rs` 更值得参考的
Kiro 账号池实现。它的价值不只是“账号导入”，而是已经把 Kiro IDE /
CodeWhisperer / Amazon Q 风格请求的运行策略补得比较完整：

- 账号选择支持 `round-robin` 和 `sticky` 两种策略。
- 失败账号有断路器、指数退避和小概率半开重试。
- 账号配额耗尽会被单独标记，并在返回错误时说明 exhausted/cooldown 数量。
- token 刷新有单账号刷新锁，避免并发请求同时刷新同一个账号。
- 非流式请求支持 401/403 刷新 token、402/429 切 endpoint 或切账号、5xx 重试。
- 模型列表会动态调用 `ListAvailableModels`，并合并隐藏模型和静态兜底模型。

但是它不是一个可以直接替换我们当前 Kiro Web Portal adapter 的项目。
KAM 仍主要是 Kiro IDE / CodeWhisperer / Amazon Q 路径；它没有加入我们现在
用于 Claude/Opus 路线的 Web Portal 序列：

```text
GetUserInfo -> GetUserUsageAndLimits -> CreateSpace -> StreamSendMessage
```

所以推荐路线是：借鉴 KAM/Kiro-Go 的账号池和调度策略，重写到我们的
内部 Kiro service 或 Sub2API provider adapter 中，而不是把 KAM Electron
程序直接塞进服务器部署。

## Reference Policy

从 2026-05-21 开始，后续 Kiro 开发不要再默认把 `kiro.rs` 当主参考。
新的参考顺序如下：

1. 本 fork 的 Sub2API adapter 和 `adapters/kiro-web`：决定公网行为和
   Claude/Opus Web Portal 路线。
2. `Quorinex/Kiro-Go`：优先参考服务端结构、Go 账号池、Docker 部署和
   模型感知路由。
3. `chaogei/Kiro-account-manager`：优先参考账号池策略、失败分级、
   token refresh single-flight、session sticky 和模型发现行为。
4. `Jwadow/kiro-gateway`：仅保留为线上 open-model fallback/reference。
5. `hank9999/kiro.rs`：降级为 legacy 参考，只在旧 IDE/API key 行为、
   历史回归或回滚场景中使用。

如果未来有人要基于 `kiro.rs` 做新功能，必须先说明为什么 KAM/Kiro-Go
不能覆盖该场景。

## 本次检查到的参考项目

| 项目 | 当前观察版本 | 许可证 | 本地镜像 | 作用 |
| --- | --- | --- | --- | --- |
| `chaogei/Kiro-account-manager` | `7ad57fd`, tag `v1.6.6` | AGPL-3.0 | `D:\wflogin\_github_research\Kiro-account-manager` | 最强 Kiro 桌面账号池/代理参考，适合学习策略，不建议直接拷贝代码。 |
| `Quorinex/Kiro-Go` | `68110f3`, version `1.0.8` | MIT | `D:\wflogin\_github_research\Kiro-Go` | Go 服务化实现，适合迁移账号池、按模型路由、Docker 部署结构。 |
| `hank9999/kiro.rs` | `f1bbe9f`, tag `v2026.3.1` | 已有本地参考 | `D:\wflogin\_github_research\kiro.rs-latest` | 旧 Rust 路由参考，当前不如 KAM/Kiro-Go 适合继续扩展账号池。 |

验证：

```text
Kiro-Go: go test ./... passed
KAM: package version = 1.6.6;未安装依赖，未跑 Electron/Node 测试
```

## KAM 里最值得借鉴的点

### 1. 账号池策略

KAM 的 `accountPool.ts` 中有显式 `AccountSelectionStrategy`：

```text
round-robin: 成功后 currentIndex 指向下一个账号，偏负载均衡
sticky: 成功后 currentIndex 固定在当前账号，偏 prompt cache 命中
```

这比旧 `kiro.rs` 更适合我们的场景，因为我们会同时遇到两种流量：

- 普通 API 调用：更适合 `round-robin`，避免一个账号先被打爆。
- Claude Code/OpenCode 长会话：更适合 `sticky`，减少上下文和缓存漂移。

建议落地：

- 在内部 Kiro service 增加全局默认策略。
- 后续如果 Sub2API 的 key/group 已经能区分用途，可以让模型或 group 覆盖策略。
- 默认用 `round-robin`，对 Claude Code 风格请求通过 session hint 进入 sticky。

### 2. 失败分级和断路器

KAM 对失败分级比较细：

- `401/403/Auth`：优先刷新 token。
- `402/429/quota/throttle/limit`：标记配额或限流，切 endpoint 或切账号。
- `5xx`：指数等待后重试。
- 请求本身错误不应该把账号长期打入冷却。

建议落地：

- 我们的 Kiro adapter 应记录每个账号的 `errorCount`、`cooldownUntil`、
  `quotaExhaustedAt`、`lastUsed`。
- `quota` 类错误单独统计，不和网络错误混在一起。
- 对外错误只返回脱敏摘要，例如：
  `All accounts quota exhausted (1/3 exhausted, 1 in cooldown)`。

### 3. token 刷新并发锁

KAM 用 `refreshingTokens: Set<string>` 防止同一账号同时被多个请求刷新。
这是服务器上必须补的能力，因为公网并发请求很容易同时撞到 token 临期。

建议落地：

- 按 account id 建立 refresh single-flight。
- 第一个请求刷新，后续请求等待短时间后复查 token。
- 刷新失败时只冷却该账号，不影响其他账号。

### 4. 请求取消和流式请求生命周期

KAM 对每个请求创建 `AbortController`，连接断开或服务停止时能取消上游请求。

建议落地：

- Go 侧使用 `context.Context` 贯穿 Sub2API -> Kiro service -> upstream。
- Python adapter 侧若继续保留，也要把客户端断开传到 `aiohttp/httpx` timeout/cancel。
- 流式请求失败时要能记录账号错误，但不能把用户主动断开误判成账号失败。

### 5. 模型发现和隐藏模型合并

KAM 的模型策略比旧静态表更稳：

- 先调用官方 `ListAvailableModels`。
- 请求带 `profileArn`。
- 根据账号 region 选 `q.us-east-1.amazonaws.com` 或 `q.eu-central-1.amazonaws.com`。
- 合并隐藏模型，例如 `simple-task`、CodeWhisperer internal model id。
- 对 `claude-{sonnet|haiku|opus}-...` 形式做向前兼容透传。

建议落地：

- 内部 Kiro service 维护 `account_id -> model set` 缓存。
- Sub2API 外部 `/v1/models` 只暴露 smoke 通过的模型。
- 内部账号调度时优先选择支持目标模型的账号。
- 未见过但格式合理的 Claude 模型可以进入 investigation，不应直接公开。

## Kiro-Go 的补充价值

`Quorinex/Kiro-Go` 值得作为服务化实现的直接参考：

- Go 代码，结构和 Sub2API 后端更近。
- `pool.AccountPool` 使用 `sync.RWMutex` 和 `atomic.AddUint64` 做并发安全轮询。
- 有 `GetNextForModel(model)`，可以根据账号缓存模型列表选择账号。
- 支持账号权重、超额使用开关、过期 token 跳过、冷却 fallback。
- 有 Dockerfile 和 docker-compose，更适合服务器部署。
- 本次 `go test ./...` 通过。

短期可以优先借鉴 Kiro-Go 的 Go 账号池形态，再吸收 KAM 更细的错误分类和
endpoint fallback。

## 不建议直接做的事

- 不建议把 KAM Electron 作为服务器上的常驻服务。它是桌面应用，部署面太大。
- 不建议直接复制 KAM 代码到私有闭源 fork。KAM 是 AGPL-3.0，直接复制会带来
  许可证义务；我们应该重写相同设计思路，或明确 fork/开源边界。
- 不建议因为 KAM 有隐藏模型列表就直接在 Sub2API 暴露 Claude/Opus。是否可用
  仍以服务器 direct smoke 和 public smoke 为准。
- 不建议立即删除 `kiro-gateway` 或 `kiro-web`。当前线上路由需要保守演进。

## 推荐落地顺序

### 阶段 A：先做只读调度层，不改公开模型

- 增加内部 Kiro account runtime state：
  - `last_used`
  - `error_count`
  - `cooldown_until`
  - `quota_exhausted_at`
  - `usage_current`
  - `usage_limit`
  - `model_set`
- 增加 `round-robin` 默认调度。
- 增加 `GetNextForModel`。
- 增加 token refresh single-flight。
- 增加错误分类和脱敏日志。

验收：

- 现有已公开 Kiro 模型 smoke 仍通过。
- 多账号时请求分布不再长期粘在同一账号。
- 一个账号 429 后会切到另一个账号。

### 阶段 B：加 session sticky

- 从请求头和 body 提取 session hint：
  - `X-Claude-Code-Session-Id`
  - `x-opencode-session`
  - `conversation_id`
  - `thread_id`
  - `session_id`
- 建立 `session -> account` 短 TTL 映射。
- 对 Claude Code/OpenCode 类型请求启用 sticky。

验收：

- 同一 session 多轮请求命中同一账号。
- 账号失败后 session 可以迁移到可用账号。

### 阶段 C：模型发现和模型感知路由

- 每个账号刷新时调用 Kiro `ListAvailableModels`。
- 缓存账号支持的模型集合。
- 对外模型列表仍由 Sub2API allowlist 控制。
- 内部调度按目标模型过滤账号。

验收：

- 请求 `qwen3-coder-next` 只会选支持该模型的账号。
- 如果账号模型缓存为空，冷启动时可乐观放行，但要异步刷新。
- Claude/Opus 仍必须 direct smoke 通过后才公开。

### 阶段 D：重新评估是否替换 `kiro.rs`

候选方向：

1. 基于 Kiro-Go 做一个轻量内部 Kiro runtime service。
2. 继续 patch `kiro.rs`，但把 KAM/Kiro-Go 的账号池策略移植进去。
3. 保持 `kiro-web` 作为 Claude/Opus 路线，只把账号池能力补到这个 adapter。

当前倾向：

- 如果目标是服务器稳定运行：优先 Kiro-Go 风格。
- 如果目标是继续跟 Kiro IDE/桌面登录：继续参考 KAM。
- 如果目标是 Web Portal Claude/Opus：继续维护我们自己的 `kiro-web`。

## 后续重新检查命令

```powershell
git ls-remote https://github.com/chaogei/Kiro-account-manager.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/Quorinex/Kiro-Go.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/hank9999/kiro.rs.git HEAD refs/heads/* refs/tags/*
```

## Sources

- https://github.com/chaogei/Kiro-account-manager
- https://github.com/Quorinex/Kiro-Go
- https://github.com/hank9999/kiro.rs
