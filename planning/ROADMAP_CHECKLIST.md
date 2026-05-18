# Roadmap Checklist

记录日期：2026-05-16

这个清单按可落地顺序组织。每一阶段都有“输入、动作、产出、完成标准、风险点”。执行时可以直接在复选框里打勾。

## 阶段 0：仓库和基线整理

目标：保证后续所有工作都在私人 Git 仓库里可追踪、可回滚。

输入：

- 本地工作副本：`D:\wflogin\sub2api-private`
- 私人 GitHub 仓库：`https://github.com/ruoyuqi00/sub2api-provider-adapters.git`
- 官方 Sub2API 远端：`https://github.com/Wei-Shaw/sub2api.git`

任务：

- [x] 创建本地私人工作目录 `sub2api-private`。
- [x] 配置 `origin` 为私人仓库。
- [x] 配置 `upstream` 为官方 Sub2API。
- [x] 禁用 `upstream` push。
- [x] 推送当前 `main` 到私人仓库。
- [x] 写入 provider adapter 参考文档。
- [x] 写入私人 Git 工作流。
- [ ] 给本仓库创建第一个 GitHub release/tag，例如 `planning-2026-05-16`。
- [ ] 在 GitHub 仓库设置分支保护：至少防止误删 `main`。
- [ ] 在 GitHub 仓库 Issues 或 Projects 创建里程碑：`M1 internal upstream`, `M2 native import`, `M3 production hardening`。

完成标准：

- `git status --short --branch` 显示干净。
- `git remote -v` 显示 `origin` 和 `upstream` 正确。
- GitHub 页面能看到完整项目和规划文档。

风险点：

- 官方历史里存在过大文件 blob，GitHub 会警告，但当前工作树无该文件。
- 如果重写官方历史，会影响后续合并 upstream，所以暂时不清历史。

## 阶段 1：接口调研和参考源跟踪

目标：确认 Windsurf/Kiro/Sub2API 之间的最小兼容边界，避免盲目写原生代码。

输入：

- `D:\wflogin\_github_research\WindsurfAPI`
- `D:\wflogin\_github_research\WindsurfPoolAPI`
- `D:\wflogin\kiro.rs-master`
- `D:\wflogin\sub2api-private`

任务：

- [ ] 按 `INTERFACE_RESEARCH_PLAYBOOK.md` 完成 WindsurfAPI 端点矩阵。
- [ ] 按 `INTERFACE_RESEARCH_PLAYBOOK.md` 完成 Kiro proxy 端点矩阵。
- [ ] 按 `INTERFACE_RESEARCH_PLAYBOOK.md` 完成 Sub2API 账号创建/测试/模型接口矩阵。
- [ ] 对 WindsurfAPI 做本地最小启动验证。
- [ ] 对 Kiro proxy 做本地最小启动验证。
- [ ] 记录每个代理的请求头、鉴权方式、错误格式、SSE 格式。
- [ ] 记录可用模型名、不可用模型错误、限流错误。
- [ ] 确认 Sub2API 对内网 `base_url` 的 URL allowlist 要求。
- [ ] 确认 Sub2API UI 是否能直接创建 `base_url=http://windsurf-api:3003` 这种 Docker service name。

产出：

- `planning/INTERFACE_RESEARCH_PLAYBOOK.md` 里的“调研记录”填充真实结果。
- 每个服务至少保留一组可脱敏 curl 样例。
- 每个服务至少保留一组成功响应结构和失败响应结构。

完成标准：

- 不经过 Sub2API 时，WindsurfAPI 和 Kiro proxy 各自能直接响应一条测试请求。
- 经过 Sub2API 时，至少一个 Windsurf 上游账号和一个 Kiro 上游账号可以创建并测试通过。

风险点：

- WindsurfAPI 需要 language server binary，服务器首次启动可能需要下载或手动安装。
- Kiro `refreshToken` 可能因为 region/auth method 不匹配而失败。
- Sub2API URL allowlist 可能默认不允许 Docker 内网 host，需要配置。

## 阶段 2：Windsurf 内网上游部署

目标：在服务器上增加 `windsurf-api` 服务，只在 Docker 网络内部被 Sub2API 调用。

任务：

- [ ] 生成强随机 `WINDSURF_API_KEY`。
- [ ] 生成强随机 `WINDSURF_DASHBOARD_PASSWORD`。
- [ ] 在服务器 `/opt/sub2api` 下添加 WindsurfAPI 数据目录。
- [ ] 在现有 compose 里添加 `windsurf-api` 服务。
- [ ] 确认不在 Caddy 暴露 WindsurfAPI。
- [ ] 确认不映射公网端口；如必须调试，只绑定 `127.0.0.1`。
- [ ] 启动服务并检查 `docker compose logs windsurf-api`。
- [ ] 安装或验证 `language_server_linux_x64`。
- [ ] 通过服务器本机 curl 调用 `/health`。
- [ ] 通过服务器本机 curl 导入一个测试 token。
- [ ] 通过服务器本机 curl 调用 `/v1/messages`。
- [ ] 通过服务器本机 curl 调用 `/v1/chat/completions`。

产出：

- `windsurf-api` 容器运行。
- 数据持久化目录包含 `accounts.json` 等状态文件。
- 一个测试账号导入成功。

完成标准：

- `docker compose ps` 中 `windsurf-api` healthy/running。
- `curl http://127.0.0.1:3003/health` 或 Docker 内网等价请求成功。
- `curl /v1/messages` 成功返回。
- 外部公网无法直接访问 WindsurfAPI。

## 阶段 3：Sub2API 接入 Windsurf/Kiro 上游

目标：把内网代理作为 Sub2API 账号接入，实现公网统一入口。

任务：

- [ ] 在 Sub2API 管理台创建 Windsurf Anthropic-compatible 账号。
- [ ] `platform=anthropic`。
- [ ] `type=apikey` 或现有 upstream 风格。
- [ ] `credentials.api_key=<windsurf-internal-api-key>`。
- [ ] `credentials.base_url=http://windsurf-api:3003`。
- [ ] `extra.anthropic_passthrough=true`，如测试发现需要。
- [ ] 配置模型映射，例如把 `claude-sonnet-4-6` 映射到 WindsurfAPI 支持的模型。
- [ ] 测试账号连接。
- [ ] 配置分组和优先级，避免影响已有官方 Anthropic/OpenAI 账号。
- [ ] 对 Kiro 重复上述过程，`base_url=http://kiro-rs:8990`。

完成标准：

- 公网只调用 Sub2API URL。
- Sub2API 能把请求路由到 Windsurf。
- Sub2API 能把请求路由到 Kiro。
- Sub2API usage log 能看出目标模型和上游错误。

风险点：

- Anthropic passthrough 行为需要实测；不一定所有上游都需要。
- 如果模型映射不匹配，Sub2API 账号测试可能失败，但代理直接调用可能成功。

## 阶段 4：生产稳定性和维护脚本

目标：让服务器日常升级、备份、回滚有固定流程。

任务：

- [ ] 更新 `/opt/sub2api/backup.sh`，确认包括 compose、env、Caddyfile、Sub2API DB dump。
- [ ] 为 WindsurfAPI/Kiro 数据目录加备份。
- [ ] 更新 `/opt/sub2api/update.sh`，区分官方 Sub2API image、私人 fork image、内部代理 image。
- [ ] 写入服务健康检查命令。
- [ ] 写入一键查看日志命令。
- [ ] 设置 Docker restart policy。
- [ ] 检查防火墙，只开放 80/443/SSH。
- [ ] 检查 Caddy 只反代 Sub2API。
- [ ] 配置日志轮转，避免代理日志占满磁盘。

完成标准：

- 任意服务升级前能快速备份。
- 任意服务升级失败后能回退到上一版本。
- token 和账号数据不丢失。

## 阶段 5：Sub2API 原生 Windsurf 导入

目标：在 Sub2API 管理台直接批量导入 Windsurf token，Sub2API 自己管理上游账号元数据；是否直接管理 provider token 取决于前四阶段稳定性。

任务：

- [ ] 增加平台常量 `PlatformWindsurf = "windsurf"`。
- [ ] 增加账号类型或复用 `apikey/upstream`，先做兼容上游模式。
- [ ] 前端新增 Windsurf 创建账号表单。
- [ ] 前端新增批量导入 textarea，支持一行一个 token 或 JSON。
- [ ] 后端新增 Windsurf 导入 handler。
- [ ] 后端导入时调用内部 WindsurfAPI `/auth/login`。
- [ ] Sub2API 保存内部上游账号，不保存原始 Windsurf token，除非明确进入原生 token 管理阶段。
- [ ] 增加重复 token 检测策略，优先由 WindsurfAPI 返回结果决定。
- [ ] 账号测试复用 Anthropic/OpenAI compatible test。
- [ ] 增加单元测试和 handler 测试。

完成标准：

- 管理台可以批量导入 Windsurf 账号。
- 导入失败能展示每个账号的错误原因。
- 不在 Sub2API 日志里打印 token。
- 仍可通过上游项目升级协议。

## 阶段 6：Sub2API 原生 Kiro 导入

目标：在 Sub2API 管理台直接导入 Kiro OAuth/API key 凭据，逐步把 token 生命周期管理迁移到 Sub2API 或内部 Kiro service。

任务：

- [ ] 增加平台常量 `PlatformKiro = "kiro"`。
- [ ] 定义 Kiro credential schema。
- [ ] 支持 `authMethod=social`。
- [ ] 支持 `authMethod=idc`。
- [ ] 支持 `authMethod=api_key` 或 `kiroApiKey`。
- [ ] 支持 `region/authRegion/apiRegion`。
- [ ] 支持 `profileArn`。
- [ ] 支持 `clientId/clientSecret`。
- [ ] 设计 refreshToken 永久失效的账号禁用策略。
- [ ] 设计 machine id 派生策略是否由 Sub2API 实现。
- [ ] 增加 token refresh provider。
- [ ] 增加 `/cc/v1/messages` 使用场景评估。

完成标准：

- Kiro 凭据可以导入、测试、失效提示。
- 至少 social 和 api_key 两类凭据跑通。
- invalid_grant 能标记为永久失效。

## 阶段 7：自定义 Sub2API 镜像和服务器替换

目标：当 fork 里有实际代码改动后，把服务器从官方镜像切换到私人 fork 构建的镜像。

任务：

- [ ] 本地构建前端和后端。
- [ ] 本地跑 Go test。
- [ ] 本地跑 frontend lint/test/build。
- [ ] 构建 Docker image。
- [ ] 给 image 打 tag，例如 `sub2api-provider-adapters:YYYYMMDD-HHMM-commit`。
- [ ] 推送到私有 registry 或直接在服务器构建。
- [ ] 服务器备份。
- [ ] 修改 compose image。
- [ ] 启动新容器。
- [ ] 验证公网和内网上游。
- [ ] 保留上一版 image 作为回滚点。

完成标准：

- 服务器运行私人 fork 镜像。
- 官方 upstream 仍可合并。
- 回滚命令已验证。

## 2026-05-18 进度更新

- [x] Windsurf Stage A 后端导入接口已实现：`POST /api/v1/admin/accounts/import/windsurf`。
- [x] 已参考 `dwgx/WindsurfAPI` 的 `/auth/login` 批量导入协议，Sub2API 只做管理员入口和安全转发，不保存原始 Windsurf token。
- [x] 已增加单元测试和 handler 覆盖：解析、去重、配置缺失、上游转发、响应脱敏、上游错误。
- [ ] 管理后台 UI 还未接入该接口。
- [ ] 服务器还未部署 `windsurf-api` 内网服务，也还未把线上 Sub2API 切到本 fork 镜像。
