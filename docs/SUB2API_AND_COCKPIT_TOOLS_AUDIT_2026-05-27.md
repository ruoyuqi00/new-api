# Sub2API 与 cockpit-tools 上游检查 2026-05-27

本记录只写项目状态和可学习点，不记录任何账号、token、API key、服务器密码或管理密码。

## 结论

- Sub2API 官方 `main` 有更新，已经合并到我们的私有 fork。
- 官方最新 tag 仍是 `v0.1.131`，但 `main` 比我们之前的 fork 多 20 个提交。
- 这批 Sub2API 更新值得合并，主要是 OpenAI WS 账号失败切换、长上下文 cache_read 计费修正、Chat/Responses usage 保留、重新授权不清空 Extra。
- cockpit-tools 远端已经到 `v0.24.9`，本轮没有发现能直接修复我们 Kiro/Windsurf 服务端协议的变更，但有不少账号管理、额度缓存、模型映射和管理 API 结构值得吸收。

## Sub2API 上游状态

- 上游仓库：https://github.com/Wei-Shaw/sub2api
- 本地 fork：`D:\wflogin\sub2api-private`
- 合并前本地 HEAD：`d9e1e36e docs: record windsurf cache routing update`
- 合并的上游 HEAD：`a3916351 更新 OpenAI 使用密钥配置`
- 合并提交：`fff48d42 Merge remote-tracking branch 'upstream/main'`
- 最新观察 tag：`v0.1.131`

本轮合并前做过 `git merge-tree --write-tree HEAD upstream/main`，结果可自动合并。正常 merge 后确认我们的私有文件仍保留，包括 Kiro/Windsurf 导入 handler、前端导入面板、`planning/` 文档和 `adapters/kiro-web/`。

## Sub2API 需要关注的更新

1. `08061717 fix: enable account failover for OpenAI WS rate limits`

   对 OpenAI WebSocket 路径增加账号失败切换能力。这个和我们外部用 Sub2API key 调 OpenAI/Codex 类账号有关，遇到上游 WS 限流时更容易切到下一个账号。

2. `b9509e82 fix(billing): apply long-context multiplier to cache_read price`

   长上下文场景下，`cache_read` 计费会套用 long-context multiplier。它不解决“上下文记忆”本身，但会让后台账单、用量统计更接近真实成本。

3. `f7ac5e59 fix(openai): preserve chat responses usage billing`

   Chat Completions 和 Responses 兼容转换时保留 usage billing。对我们从外部客户端走 Sub2API 统计消耗很重要。

4. `11fe7de9 fix(account): 重新授权不再清空 Extra 配置`

   重新授权 OAuth 或 setup-token 时新增 `apply-oauth-credentials`，只增量合并凭据和 extra，不再用全量 update 把 `Extra` 里的运行态配置清掉。这个对我们自定义 provider adapter 特别重要，因为 Kiro/Windsurf 账号常把模型别名、额度、私有路由等元数据放在 `Extra`。

5. `a9c7a3a0 fix(bedrock): strip context_management when beta is removed`

   Bedrock 路径会在 beta 被移除时剥离 `context_management`，避免 Anthropic/Bedrock 兼容参数冲突。当前不是 Kiro/Windsurf 主路径，但属于上游协议兼容修复。

6. ops/business-limit 系列提交

   官方把 API key group、Google group、local platform gate、fast-policy 等本地策略拒绝标记为业务限制，不计入 SLA 错误率。这对后台告警和错误统计更干净。

## 验证结果

已运行：

```powershell
go test ./internal/service ./internal/handler ./internal/server/middleware ./internal/pkg/apicompat
corepack pnpm --dir frontend typecheck
```

结果：

- Go 关键包测试通过。
- 前端 TypeScript 检查通过。

## cockpit-tools 上游状态

- 上游仓库：https://github.com/jlcodes99/cockpit-tools
- 本地研究目录：`D:\wflogin\_github_research\cockpit-tools`
- 本地 checkout：`2b14843`
- 远端 `origin/main`：`866f526`
- 最新观察 tag：`v0.24.9`

本地 checkout 没有直接切过去，主要通过 `origin/main` 读取远端文件，避免把研究目录工作区强行重置。

## cockpit-tools 值得学习的地方

### 1. Windsurf 额度缓存和失败容错

相关思路来自 `crates/cockpit-core/src/modules/windsurf_account.rs` 和近期提交：

- 刷新 payload 没拿到有效配额时，不立刻覆盖旧额度，而是保留旧 `quota` 快照并记录 `quota_query_last_error`。
- 批量刷新使用并发上限，当前是 `MAX_CONCURRENT = 5`。
- `upsert` 时根据 GitHub identity 和 API key 去重，避免同一个账号重复导入多条记录。
- 本地 `windsurfAuthStatus` 可以作为补充来源，刷新时把本地状态合并到账户对象。
- UI 侧增加“按推荐排序”，推荐账号按剩余额度、最近使用等信号排序。
- `remaining` 字段缺失但 reset 信息存在时，不丢弃额度行；这对官方字段偶发缺失很有用。

对我们的影响：

- 服务器当前 Windsurf 主路径走 `dwgx/WindsurfAPI`，协议请求不需要直接改 Sub2API。
- 如果后续把 Windsurf 账号管理原生做进 Sub2API，应该优先吸收“保留旧额度快照”和“导入去重”。
- 现有 Windsurf 公网调用缓存问题，仍主要靠上游 `windsurf-api` 的 Cascade reuse 和 Sub2API 模型别名固定调用方解决；cockpit-tools 的这部分更偏账号管理和 UI。

### 2. Kiro 导入兼容格式

相关思路来自 `crates/cockpit-core/src/modules/kiro_account.rs`：

- 导入时不只接受完整 `KiroAccount`，还兼容裸 JSON、数组、`accounts`、`items`。
- auth token 字段兼容 `kiro_auth_token_raw`、`authToken`、`token`、`auth`。
- access/refresh/profile/region/client 等字段同时兼容 snake_case 和 camelCase。
- usage/profile 可从 `usageData`、`usage_state`、`kiro_usage_raw`、`profileData` 等字段拾取。
- 刷新 Kiro 时跳过 banned account，并发上限也是 5。

对我们的影响：

- 这和用户之前给的 Kiro JSON 很贴近。我们的导入接口应该继续保持“宽输入、严归一化”。
- 如果以后再改 Kiro 后台导入，优先补齐 cockpit-tools 这种字段兼容矩阵，而不是只支持单一导出格式。

### 3. CLIProxyAPI sidecar 的服务端结构

cockpit-tools 新版包含 `sidecars/cockpit-cliproxy/cdk/CLIProxyAPI/`。它不是 Kiro/Windsurf 专用修复，但结构很值得参考：

- provider-specific 路由，例如 `/api/provider/{provider}/v1/messages`，用于明确协议表面。
- model alias / model mapping，可把客户端请求模型映射到实际上游模型，并把响应里的 model 改回客户端请求名。
- per-client upstream API key mapping：同一个代理可以按外部客户端 key 选择不同上游 key。
- 配置更新后显式 invalidate cache，避免旧 key 或旧模型映射继续生效。
- 对本地 provider 过滤不支持的 `Anthropic-Beta`，但对正式上游代理路径保留完整 beta header。
- 有管理 API、auth files、usage、quota、model definitions、request logging 等独立模块。

对我们的影响：

- Sub2API 已经承担公网 API key、分组、模型映射和计费；不建议把 CLIProxyAPI 整体塞进 Sub2API。
- 但可以吸收“provider-specific 内部路由”、“按客户端 key 路由到不同上游账号”、“配置变更主动清缓存”、“响应 model 重写”这些设计。
- 这比继续在一个通用 `/v1/messages` 里硬猜来源更清晰，适合未来做 Kiro/Windsurf 原生 provider adapter。

## 不建议直接照搬的部分

- cockpit-tools 本体是桌面账号管理工具，不是我们公网 API 网关的直接替代品。
- 它的 Windsurf/Kiro 逻辑偏本机 IDE 账号提取、切号和配额展示，不等同于服务器上稳定代理模型请求。
- CLIProxyAPI sidecar 是另一个完整网关，直接套进来会和 Sub2API 的鉴权、计费、用户、分组、日志体系重复。

## 后续建议

1. 短期保持当前部署形态：公网只暴露 Sub2API，Windsurf/Kiro 仍作为内网上游。
2. Windsurf：继续跟 `dwgx/WindsurfAPI`，只在协议或 Cascade 复用有更新时重启上游服务。
3. Kiro：继续用 KAM/Kiro-Go/kiro-gateway 做协议参考，`kiro.rs` 降级为 legacy 行为参考。
4. Sub2API：以后每次官方 main 更新，优先检查 WS failover、usage billing、Extra 合并、模型上下文参数这几类提交。
5. 自研原生 adapter 时，优先实现 cockpit-tools 里验证过的三件事：导入字段宽兼容、额度快照不被空刷新覆盖、按外部 key/模型做稳定路由。

## 参考链接

- https://github.com/Wei-Shaw/sub2api
- https://github.com/Wei-Shaw/sub2api/commits/main
- https://github.com/jlcodes99/cockpit-tools
- https://github.com/jlcodes99/cockpit-tools/releases/tag/v0.24.9
