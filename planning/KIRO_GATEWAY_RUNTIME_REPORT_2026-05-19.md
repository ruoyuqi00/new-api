# Kiro Gateway Runtime Report - 2026-05-19

本文记录本轮 Kiro 接入的实际运行结论。不要在本文或仓库内写入任何 access token、refresh token、API key、SSH 密码或 `.env` 原文。

## 参考项目

- `hank9999/kiro.rs`
  - GitHub: https://github.com/hank9999/kiro.rs
  - 本地 mirror: `D:\wflogin\_github_research\kiro.rs-latest`
  - 2026-05-19 观察到的 HEAD: `f1bbe9f`
  - 2026-05-19 观察到的最新 tag: `v2026.3.1`
- `Jwadow/kiro-gateway`
  - GitHub: https://github.com/Jwadow/kiro-gateway
  - 本地 mirror: `D:\wflogin\_github_research\kiro-gateway`
  - 2026-05-19 观察到的 HEAD: `a5292ca`
  - 2026-05-19 观察到的最新 tag: `v2.3`

## 关键判断

当前短期运行路径选择 `kiro-gateway`，不是直接用 `kiro.rs` 承担模型请求。

原因：

- `kiro.rs` 当前主请求路径仍指向 `https://q.<region>.amazonaws.com/generateAssistantResponse`，实测 Claude/Kiro 模型会返回 `INVALID_MODEL_ID`。
- `kiro-gateway` 当前配置使用 `https://runtime.<region>.kiro.dev/generateAssistantResponse`，更贴近当前 Kiro 运行面。
- `kiro-gateway` 的模型解析策略更适合短期接入：先规范化模型名，再尽量透传给 Kiro 上游，让上游最终判定是否可用。
- 本轮 Kiro Pro 凭据可被 `kiro-gateway` 使用，且 open-model 类模型已通过 Sub2API 公网入口验证。

## 服务器部署状态

服务器：`154.219.122.197`

部署目录：`/opt/sub2api`

当前内部拓扑：

```text
Internet
  -> Caddy
  -> Sub2API
  -> internal Docker network
      -> windsurf-api
      -> kiro-rs
      -> kiro-gateway
      -> postgres
      -> redis
```

公网只暴露 Sub2API。`windsurf-api`、`kiro-rs`、`kiro-gateway` 均不发布宿主机端口，也不写入 Caddy 路由。

本轮服务器变更：

- 将临时运行的 `sub2api-kiro-gateway` 固化到 `/opt/sub2api/docker-compose.yml`。
- 部署 Sub2API 新镜像：
  - `sub2api-provider-adapters:a147caa0`
- 上一个 Sub2API 镜像：
  - `sub2api-provider-adapters:c4cefc76`
- 镜像切换前服务器数据备份：
  - `/opt/sub2api-backups/sub2api-20260519-123646.tar.gz`
- compose 变更前备份：
  - `/opt/sub2api-backups/docker-compose-20260519-042547-pre-kiro-gateway.yml`
- 镜像切换前 compose 备份：
  - `/opt/sub2api-backups/docker-compose-20260519-044206-pre-a147caa0.yml`
- `kiro-gateway` 镜像：
  - `ghcr.io/jwadow/kiro-gateway:latest`
- `kiro-gateway` 内部端口：
  - `8000/tcp`
- Kiro 凭据文件位置：
  - `/opt/sub2api/kiro-gateway/creds/kiro-auth-token.json`
- 持久化目录：
  - `/opt/sub2api/kiro-gateway/creds`
  - `/opt/sub2api/kiro-gateway/debug_logs`
  - `/opt/sub2api/kiro-gateway/state`

## 已导入凭据状态

用户提供的 Kiro 凭据已按敏感材料处理，只放在服务器运行目录，不进入 Git。

已观察到的非敏感账号状态：

- 计划：`KIRO PRO`
- 用量：服务器侧显示该账号可被 `kiro-gateway` 初始化。
- profile ARN：已存在。
- refresh token：已存在，未写入仓库。

## 模型验证结果

`kiro-gateway` 内部 `/v1/models` 当前返回 13 个模型：

```text
auto-kiro
claude-haiku-4.5
claude-opus-4.5
claude-opus-4.6
claude-opus-4.7
claude-sonnet-4
claude-sonnet-4.5
claude-sonnet-4.6
deepseek-3.2
glm-5
minimax-m2.1
minimax-m2.5
qwen3-coder-next
```

本轮实测可用并已接入 Sub2API 的模型：

```text
deepseek-3.2
glm-5
minimax-m2.5
qwen3-coder-next
```

验证记录：

- 内部直连 `kiro-gateway /v1/models`：HTTP 200。
- 内部经 Sub2API 调用 `qwen3-coder-next`：HTTP 200，返回 `ok`。
- 公网经 `https://api.vyywcw.cn/v1/messages` 调用 `deepseek-3.2`：HTTP 200，返回 `ok`。

暂不映射到 Sub2API 的模型：

```text
auto-kiro
claude-haiku-4.5
claude-opus-4.5
claude-opus-4.6
claude-opus-4.7
claude-sonnet-4
claude-sonnet-4.5
claude-sonnet-4.6
minimax-m2.1
```

原因：

- Claude 系列在本轮请求中返回 `INVALID_MODEL_ID` 或“模型 ID/订阅级别不可用”类错误。
- `minimax-m2.1` 未完成正向 smoke，不应提前公开映射。
- 为避免 Sub2API 将失败模型调度给用户，当前只映射已通过 smoke 的模型。

## Sub2API 数据库接入

新增内部 upstream account：

```text
name: kiro-gateway-internal-anthropic
platform: anthropic
type: apikey
base_url: http://kiro-gateway:8000
group: windsurf-smoke
status: active
schedulable: true
```

当前映射：

```json
{
  "deepseek-3.2": "deepseek-3.2",
  "glm-5": "glm-5",
  "minimax-m2.5": "minimax-m2.5",
  "qwen3-coder-next": "qwen3-coder-next"
}
```

注意：该账号的 `api_key` 来自服务器 `.env` 中的 `KIRO_API_KEY`，不得写入文档或 Git。

## 为什么还保留 kiro.rs

`kiro.rs` 仍有价值：

- 它有较完整的 Rust 版 Anthropic `/v1/messages` 和 `/cc/v1/messages` 结构。
- 它的 admin credential 管理、refresh token 分类、region 字段、machine id 等逻辑可以继续参考。
- 本 fork 已经有 Sub2API -> `kiro.rs` admin import bridge，可以作为后续“凭据导入控制面”的参考。

但短期模型请求不走 `kiro.rs`：

- 当前 `kiro.rs` 运行面的模型请求路径落后于 Kiro 当前线上接口。
- 直接改 `kiro.rs` 到 `runtime.<region>.kiro.dev` 需要补齐 profile ARN、模型透传、错误分类、payload 限制和流式解析回归测试。

## 下一步

1. 把 `kiro-gateway` 的 runtime endpoint、模型 resolver、credential file 写回能力整理成最小 patch 计划。
2. 在本 fork 中增加一个“Sub2API Kiro runtime upstream 创建/更新”管理入口，避免以后手工 SQL 插入账号。
3. 继续观察 `hank9999/kiro.rs` 是否切换到 `runtime.<region>.kiro.dev`；若上游完成，可考虑回到 `kiro.rs`。
4. 继续观察 `Jwadow/kiro-gateway` 新 tag；每次更新后必须重新 smoke 四个已公开模型。
5. 单独排查 Kiro Pro 账号为什么 Claude 系列仍返回不可用，确认是订阅、区域、模型名、请求 payload 还是官方策略导致。
