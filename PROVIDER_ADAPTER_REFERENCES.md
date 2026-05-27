# Provider Adapter References and Update Playbook

记录日期：2026-05-16

这个文档用于维护我们自己的 Sub2API fork。目标不是一次性把所有第三方项目的代码搬进来，而是把可靠参考源、协议变化跟踪方式、短期部署方式和长期原生接入路径固定下来。后面 Windsurf、Kiro 或 Sub2API 本身协议变了，就按这里的流程拉取上游、对照差异，再决定更新内部上游服务还是改我们自己的 Sub2API 代码。

## 总体架构

公网只暴露 Sub2API：

```text
Client
  -> Caddy / HTTPS
  -> Sub2API
  -> internal Docker network
  -> WindsurfAPI / Kiro proxy / future provider adapters
```

原则：

- Sub2API 是统一入口、鉴权、分组、计费和模型路由层。
- Windsurf、Kiro 这类协议变化快的服务先以内网上游方式接入。
- 内部上游服务不走公网域名，不在 Caddy 暴露路径。
- 账号 token、refresh token、API key 不写进仓库，不写进文档。
- 等某个适配器跑稳定以后，再考虑把账号导入、刷新、错误处理做成 Sub2API 原生账号类型。

## 参考项目清单

| 项目 | 地址 | 本地位置 | 当前观察版本 | 用途 | 结论 |
| --- | --- | --- | --- | --- | --- |
| Sub2API upstream | https://github.com/Wei-Shaw/sub2api | `D:\wflogin\sub2api-private` | `upstream/main` = `a3916351 更新 OpenAI 使用密钥配置`，latest tag `v0.1.131` | 我们 fork 的主项目 | 已合并到私有 fork；继续优先跟进官方 main |
| WindsurfAPI | https://github.com/dwgx/WindsurfAPI | `D:\wflogin\_github_research\WindsurfAPI` | `master` = `41a36b9 release: 2.0.97`, latest tag `v2.0.97` | Windsurf internal upstream | Deployed internally with Cascade caller reuse enabled |
| WindsurfPoolAPI | https://github.com/guanxiaol/WindsurfPoolAPI | `D:\wflogin\_github_research\WindsurfPoolAPI` | `main` = `a8d2f4c v2.0.7`，最高 tag `v2.0.7` | Windsurf 备选参考 | 作为协议/实现对照，不作为第一部署选择 |
| cockpit-tools | https://github.com/jlcodes99/cockpit-tools | `D:\wflogin\_github_research\cockpit-tools` | `origin/main` = `866f526`，latest tag `v0.24.9` | 桌面账号管理、额度展示、CLIProxyAPI 结构参考 | 不直接替换服务端；优先学习账号导入容错、额度缓存、模型映射和 per-client upstream key |
| kiro.rs | https://github.com/hank9999/kiro.rs | `D:\wflogin\kiro.rs-master` | 远端 `master` = `f1bbe9f`，最高观察 tag `v2026.3.1` | Kiro Anthropic 兼容代理主参考 | 短期以内网上游接入，长期可拆 token/provider 逻辑 |
| kiro-gateway | https://github.com/jwadow/kiro-gateway | 未克隆，远端可跟踪 | 远端 `main` = `6544d1f`，最高观察 tag `v2.3` | Kiro 备选参考 | 用于对照认证、路由、模型映射变化 |
| 本地 Windsurf Electron 工具 | `D:\wflogin\windsurf-register-free-2\windsurf-auto-free-main` | 本地 | 非服务端项目 | 账号登录/刷新逻辑参考 | 不直接部署到 API 网关路径 |

## 当前推荐短期方案

### Windsurf

短期使用 `dwgx/WindsurfAPI`。

它已经提供：

- `POST /v1/chat/completions`：OpenAI 兼容。
- `POST /v1/messages`：Anthropic 兼容。
- `POST /auth/login`：账号导入，支持单个 token 和批量 token。
- 多账号池、token 保存、模型访问检测、错误/限流处理。
- Docker 部署。

接入方式：

```text
Sub2API account
  platform: anthropic 或 openai
  type: apikey / upstream style
  base_url: http://windsurf-api:3003
  api_key: WindsurfAPI 内部 API_KEY
```

如果走 OpenAI 兼容接口，`base_url` 可使用：

```text
http://windsurf-api:3003/v1
```

如果走 Anthropic 兼容接口，优先测试：

```text
http://windsurf-api:3003
```

2026-05-27 live Windsurf update:

- Current upstream reference: `dwgx/WindsurfAPI v2.0.97`
  (`41a36b9 release: 2.0.97`).
- Live server enabled `CASCADE_REUSE_BY_CALLER=1`, `CASCADE_POOL_MAX=5`, and
  `CASCADE_REUSE_HASH_SYSTEM=0`.
- Force Windsurf through Sub2API with `ws-` aliases:
  `ws-claude-sonnet-4.6`, `ws-claude-sonnet-4.6-thinking`,
  `ws-claude-opus-4.6`, `ws-claude-opus-4.6-thinking`,
  `ws-gemini-2.5-flash`, `ws-gpt-5.1`, and `ws-gpt-5.2`.
- Full cache/calling record:
  `planning/WINDSURF_CACHE_AND_SUB2API_CALLING_2026-05-27.md`.

需要重点配置：

- `API_KEY`：内部调用密钥，必须设置。
- `DASHBOARD_PASSWORD`：后台密码，必须设置。
- `DATA_DIR=/data`：账号和运行状态持久化。
- `LS_BINARY_PATH=/opt/windsurf/language_server_linux_x64`：Windsurf language server 路径。

账号导入只在服务器本机或 Docker 内网执行：

```bash
curl -sS http://127.0.0.1:3003/auth/login \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <internal-api-key>' \
  -d '{"accounts":[{"token":"token-1"},{"token":"token-2"}]}'
```

### Kiro

短期使用 `kiro.rs` 或同类 Kiro proxy 作为内部 Anthropic 上游。

它支持的重点能力：

- `/v1/messages`
- `/v1/messages/count_tokens`
- `/cc/v1/messages`
- `/cc/v1/messages/count_tokens`
- OAuth refresh token 刷新。
- `kiroApiKey` 静态 bearer 凭据。
- 多凭据、代理、admin API。

接入方式：

```text
Sub2API account
  platform: anthropic
  type: apikey / upstream style
  base_url: http://kiro-rs:8990
  api_key: Kiro proxy 内部 API key
```

后续如果 Kiro 官方协议变动，优先对照 `hank9999/kiro.rs`，再看 `jwadow/kiro-gateway` 是否已经适配。

## 协议更新时看哪里

### WindsurfAPI 重点文件

在 `D:\wflogin\_github_research\WindsurfAPI` 里优先看：

- `README.md`：部署、接口和环境变量。
- `src/server.js`：路由入口，特别是 `/auth/login`、`/v1/messages`、`/v1/chat/completions`。
- `src/handlers/messages.js`：Anthropic Messages 兼容层。
- `src/handlers/chat.js`：OpenAI Chat Completions 兼容层。
- `src/auth.js`：账号池、持久化、限流、账号状态。
- `src/dashboard/windsurf-login.js`：Windsurf 登录、Firebase token 刷新相关逻辑。
- `src/models.js`：模型名、模型权限和模型映射。
- `src/langserver.js`：language server 启动和 gRPC 通信。

重点观察这些变化：

- 请求路径是否变化。
- 请求头是否变化，尤其是 `Authorization`、`x-api-key`、Windsurf/Codeium 相关头。
- token 刷新接口是否变化。
- language server binary 或启动参数是否变化。
- streaming SSE event shape 是否变化。
- tool call / tool result 格式是否变化。
- 模型名、模型权限、限流错误格式是否变化。

### Kiro 重点文件

在 `D:\wflogin\kiro.rs-master` 或上游 `hank9999/kiro.rs` 里优先看：

- `README.md`：凭据结构、接口列表、部署方式。
- `src/anthropic/handlers.rs`：Anthropic `/v1/messages` 和 `/cc/v1/messages` 兼容层。
- `src/kiro/token_manager.rs`：`refreshToken`、`accessToken`、`clientId`、`clientSecret` 刷新逻辑。
- `src/kiro/endpoint/`：Kiro 上游端点和请求体加工。
- `src/kiro/machine_id.rs`：machine id 派生规则。
- `src/admin/`：账号导入、验证、脱敏和管理 API。
- `src/token.rs`：count token 行为和外部 count_tokens 接口。

重点观察这些变化：

- `authMethod` 是否新增类型。
- `refreshToken` 刷新 URL、请求体或错误格式是否变化。
- `profileArn`、region、machine id 是否变成必填。
- `/cc/v1/messages` 和 `/v1/messages` 的事件差异是否变化。
- count_tokens 是否需要外部服务。
- API key 凭据和 OAuth 凭据是否仍然可共存。

### Sub2API fork 重点文件

在 `D:\wflogin\sub2api-fork` 里，做原生接入时优先看：

- `backend/internal/service/account_service.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/domain/openai_messages_dispatch.go`
- `frontend/src/components/account/credentialsBuilder.ts`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/composables/useModelWhitelist.ts`
- `frontend/src/constants/account.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- 相关数据库 migration 和 ent schema。

原生接入通常需要改：

- 平台枚举：增加 `windsurf` / `kiro`。
- 账号类型：增加 provider-specific credential shape。
- 凭据存储：保存 token、refresh token、expires_at、region 等。
- token 刷新：后台刷新并处理永久失效。
- 请求转发：复用 Anthropic/OpenAI translator，必要时加 provider adapter。
- 模型列表和模型映射。
- 账号测试和错误提示。
- 管理 UI 和批量导入 UI。

## 拉取上游和对照流程

### 更新 Sub2API fork

```powershell
cd D:\wflogin\sub2api-fork
git fetch upstream main
git status --short
git log --oneline HEAD..upstream/main
git merge upstream/main
```

如果我们自己的改动比较多，也可以用 rebase，但服务器部署分支优先保持可读、可回滚。

### 更新 WindsurfAPI 参考源

```powershell
cd D:\wflogin\_github_research\WindsurfAPI
git fetch --tags origin master
git log --oneline HEAD..origin/master
git tag --sort=-v:refname | Select-Object -First 5
```

如果有新提交：

```powershell
git pull --ff-only origin master
```

然后重点 diff：

```powershell
git diff HEAD~1..HEAD -- src/server.js src/handlers/messages.js src/handlers/chat.js src/auth.js src/dashboard/windsurf-login.js src/models.js src/langserver.js README.md
```

### 更新 WindsurfPoolAPI 备选参考

```powershell
cd D:\wflogin\_github_research\WindsurfPoolAPI
git fetch --tags origin main
git log --oneline HEAD..origin/main
git tag --sort=-v:refname | Select-Object -First 5
```

它不作为主部署源，但如果 WindsurfAPI 出现协议滞后，可以看这个项目是否更早修复。

### 更新 Kiro 参考源

如果把 `hank9999/kiro.rs` 克隆到研究目录：

```powershell
cd D:\wflogin\_github_research\kiro.rs
git fetch --tags origin master
git log --oneline HEAD..origin/master
git tag --sort=-v:refname | Select-Object -First 5
```

如果只想临时看远端状态：

```powershell
git ls-remote --heads https://github.com/hank9999/kiro.rs.git
git ls-remote --tags https://github.com/hank9999/kiro.rs.git "refs/tags/*"
```

`kiro-gateway` 作为备选参考：

```powershell
git ls-remote --heads https://github.com/jwadow/kiro-gateway.git
git ls-remote --tags https://github.com/jwadow/kiro-gateway.git "refs/tags/*"
```

## 什么时候只更新内部上游，什么时候改 Sub2API

只更新内部上游服务：

- 第三方只改了 token 刷新、上游 header、language server 或 provider 私有协议。
- Sub2API 的 OpenAI/Anthropic 兼容请求不需要变化。
- `base_url + api_key` 仍然能稳定工作。

改 Sub2API fork：

- 需要在 Sub2API 管理台直接批量导入 Windsurf/Kiro 账号。
- 需要 Sub2API 自己管理 refresh token 生命周期。
- 需要按 Windsurf/Kiro 的账号池、quota、模型权限做调度。
- 需要在 Sub2API 里展示 provider-specific 状态和错误。
- 需要绕过中间 proxy，直接从 Sub2API 请求 provider。

## 每次协议更新后的验证清单

Windsurf：

- 内部服务 `/auth/login` 能导入 1 个测试 token。
- `/v1/messages` 非流式和流式都能返回。
- `/v1/chat/completions` 非流式和流式都能返回。
- tool calls 能正常出现和回传。
- 账号耗尽、限流、token 失效时错误能被 Sub2API 看懂。
- Docker 重启后 `accounts.json` 还在。
- Caddy 没有暴露 WindsurfAPI。

Kiro：

- OAuth refresh token 能刷新 access token。
- `kiroApiKey` 凭据能直接作为 bearer 使用。
- `/v1/messages` 能流式返回。
- `/cc/v1/messages` 能给 Claude Code 类客户端正确 usage。
- `/v1/messages/count_tokens` 可用，或者 fallback 行为可接受。
- region/profileArn/machineId 变化不会导致所有账号失效。
- Caddy 没有暴露 Kiro proxy。

Sub2API：

- 管理台能创建/编辑对应 upstream account。
- 账号测试能通过。
- 公网只访问 `https://api.vyywcw.cn` 或配置好的 Sub2API 域名。
- 新模型名能正确映射到目标 provider。
- 日志里看得到真实 upstream 错误，方便排查。

## 服务器部署更新原则

当前服务器上已有官方 Sub2API Docker 部署。短期加适配器时，不替换 Sub2API 镜像，只加内部服务：

```text
/opt/sub2api/docker-compose.yml
  sub2api
  postgres
  redis
  caddy
  windsurf-api   # internal only
  kiro-rs        # internal only, optional
```

上线前：

```bash
cd /opt/sub2api
./backup.sh
docker compose config
docker compose up -d
docker compose ps
```

更新内部上游：

```bash
cd /opt/sub2api
docker compose pull windsurf-api
docker compose up -d windsurf-api
docker compose logs --tail=100 windsurf-api
```

更新我们自己的 Sub2API fork 后：

1. 本地合并上游并测试。
2. 构建自定义 Docker image。
3. 推到服务器或镜像仓库。
4. 服务器备份数据库和配置。
5. 替换 compose 里的 Sub2API image。
6. `docker compose up -d sub2api`。
7. 测试公网 HTTPS 和内部 upstream。

## 当前决策

- 第一阶段：Sub2API 不直接集成 Windsurf/Kiro 私有协议，只接内部 OpenAI/Anthropic 兼容 proxy。
- Windsurf 第一选择：`dwgx/WindsurfAPI`。
- Kiro 第一选择：`hank9999/kiro.rs` / 本地 `kiro.rs-master`。
- Sub2API fork 保持跟进官方 `Wei-Shaw/sub2api`。
- 后续协议变化时，先更新参考项目，确认兼容接口是否还能工作，再决定是否把变化吸收到我们的 fork。
## 2026-05-21 Kiro Web Portal Update

The Kiro short-term choice has changed for Claude/Opus models:

- Keep `kiro.rs` for its admin UI, credential storage, and old API-key/CLI
  route coverage.
- Use the new internal `kiro-web-adapter` for Claude-family Kiro Web models.
- Public traffic still enters through Sub2API only.

Current internal URL:

```text
http://kiro-web-adapter:8991
```

Current implementation and deployment notes:

- `adapters/kiro-web/README.md`
- `planning/KIRO_WEB_PORTAL_ADAPTER_2026-05-21.md`

The checked references did not yet solve this path directly:

- `tickernelz/opencode-kiro-auth` latest observed `v1.10.1`
- `hongyilyu/pi-kiro` latest observed `v0.1.3`
- `Jwadow/kiro-gateway` latest observed HEAD `a5292ca0`, latest tag family
  through `v2.3`; useful gateway reference but not a replacement for the
  validated Web Portal adapter yet.

The decisive behavior came from the official Kiro Web frontend: new sessions
send `sessionId` equal to the newly created `spaceId`. Mirroring that behavior
lets `StreamSendMessage` call `claude-opus-4.7`, `claude-opus-4.6`, and
`claude-sonnet-4.6` successfully with the tested Kiro Pro account.

## 2026-05-21 Server Wiring Result

The server now keeps all public traffic on Sub2API:

```text
https://api.vyywcw.cn/
```

Sub2API routes internally to:

- `windsurf-api:3003` for Windsurf-backed Anthropic-compatible requests;
- `kiro-gateway:8000` for the older Kiro gateway open-model path;
- `kiro-web-adapter:8991` for Kiro Web Claude/Opus/Sonnet requests.

The Kiro Web route is exposed to users as OpenAI Chat Completions through the
existing `provider-mixed` group. The public checks passed for:

- `claude-sonnet-4.6`;
- `claude-opus-4.7`;
- streaming `claude-sonnet-4.6`.

See `planning/SUB2API_KIRO_WEB_WIRING_2026-05-21.md`.
