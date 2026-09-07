# YuAPI 独立 Sub2API 号池部署记录

## 结果

- 部署时间：2026-09-07（Asia/Shanghai）
- 服务器：`199.231.85.194`
- 公网后台：`https://sub2.yuaiapi.com`
- 官方上游：`Wei-Shaw/sub2api:main`
- 用户镜像：`ruoyuqi00/sub2api-provider-adapters:sub2api`
- 两端提交：`ab99d56e9626e6cd731592dae8553c9758a0efa2`（Sub2API `0.2.1`）
- 本地镜像：`yuapi-sub2api:ab99d56e9626e6cd731592dae8553c9758a0efa2`
- 镜像 ID：`sha256:f3dd8e935011c1a9f0452107b5874319eee4a7f5767863344ed243b19cd5a8f7`

用户仓库为私有仓库，生产服务器未保存 GitHub 凭据。构建时从公开官方仓库检出与用户镜像分支完全相同的固定提交；部署不会跟随 `latest` 自动漂移。

## 独立资源

- 目录：`/opt/yuapi-sub2api-v2`
- Compose 项目：`yuapi-sub2api-v2`
- 容器：`yuapi-sub2api-v2-app`、`yuapi-sub2api-v2-postgres`、`yuapi-sub2api-v2-redis`
- 数据库私网：`yuapi-sub2api-v2-internal`
- 既有数据平面：`sub2api_sub2api-network`
- 内网别名：`sub2api-internal`
- 宿主监听：`127.0.0.1:18473 -> 8080`
- 卷：`yuapi-sub2api-v2_sub2api_data`、`yuapi-sub2api-v2_postgres_data`、`yuapi-sub2api-v2_redis_data`

PostgreSQL 和 Redis 仅连接 `yuapi-sub2api-v2-internal`，没有宿主端口映射。应用同时连接数据库私网和既有 YuAPI 数据平面。未创建或复用新的 `yuapi-backplane`，也未修改 YuAPI 容器网络。

服务器密钥文件 `/opt/yuapi-sub2api-v2/yuapi-internal.key` 权限为 `0600`。`.env` 权限为 `0600`，已移除一次性管理员引导密码变量。初始凭据文件已在用户改密并通过重启登录验证后删除。

## Sub2API 配置

- 管理员：`admin@yuapi.internal`
- 注册、密码找回、支付、推广和第三方 OAuth：关闭
- 后台模式：仅管理员
- 内部组：`yuapi-internal`，ID `2`，平台 `openai`，状态 active
- 内部 API Key：ID `1`，值仅保存在服务器 root-only 文件中
- 密钥校验指纹：`f6d69c210831a2a3`（SHA-256 前 16 位）
- 内部虚拟余额：`1000000`，仅用于避免内部号池被余额门槛阻断，不对应外部支付

新号池目前没有上游账号。模型目录可返回 15 个 OpenAI 兼容模型，但不会接收 YuAPI 流量。

## YuAPI 接入

使用现有标准 OpenAI 渠道适配器，不新增 Sub2API 专用代码，也没有构建、替换或重启 YuAPI/NewAPI 镜像和容器。

- 渠道 ID：`2556`
- 名称：`Sub2API Internal Pool`
- 类型：OpenAI（`type=1`）
- Base URL：`http://sub2api-internal:8080`
- 状态：手动禁用（`status=2`）
- 优先级：`-10`
- 模型能力：15 条，启用数 `0`

既有内置账号池代码和表保留。生产数据库中的旧池 `gpt` 没有账号且没有渠道绑定；`Grok OAuth Import Pool` 已禁用且同样为空，因此旧池当前不会参与路由。

## 公网边界

Caddy 配置：`/opt/edge/Caddyfile`

部署前备份：`/opt/newapi/backups/caddy-before-sub2api-20260906163332`

公网仅允许后台页面、静态资源、登录/会话、管理员和个人资料控制面接口，以及 `/health`、`/setup/status`。注册、密码找回、OAuth 注册入口和所有模型网关路径由最终 `404` 处理器拒绝。

边缘容器曾因单文件 bind mount 指向旧 inode 而读取旧配置。最终已只重建 `yuapi-caddy`，重新连接其原有三个网络，并验证容器内挂载文件包含新域名。YuAPI 应用容器未重启。

## 验收

- `https://sub2.yuaiapi.com/login`：`200`
- `https://sub2.yuaiapi.com/api/v1/settings/public`：`200`
- 注册、找回密码和 OAuth 注册路径：`404`
- `/v1/models`、`/responses`、`/chat/completions`、`/backend-api/codex/responses`、`/v1beta/models`、图片和视频路径：`404`
- `127.0.0.1:18473/health`：`200`
- YuAPI 容器解析 `sub2api-internal`：成功
- YuAPI 容器经内网请求 Sub2API `/health`：`200`
- YuAPI 容器经内部密钥请求 `/v1/models`：`200`，15 个模型
- 三个新容器完整重启后：全部 healthy，管理员、组和密钥持久化
- YuAPI 容器：ID 和启动时间未变化，healthy
- `https://api.yuaiapi.com/api/status`：`200`
- `https://api.yuaet.top/health` 与 `https://yuaet.top/`：`200`

## 官方同步

1. 在可信工作站获取官方 `main`，记录新 SHA。
2. 仅在用户仓库 `sub2api` 分支可快进时推送；禁止 force-push，禁止合并到 YuAPI `main`。
3. 在服务器检出记录的固定 SHA，构建新的不可变镜像标签。
4. 升级前备份 PostgreSQL，保留当前镜像标签；验证迁移和空闲环境后再更新 `.env` 中的 `OFFICIAL_SHA`。
5. 只更新 `yuapi-sub2api-v2` 项目，验证后台、公网拒绝规则、内网模型请求及现有 YuAPI 健康状态。

## 启用条件

只有在 Sub2API 后台加入账号，并完成实际模型的非流式、流式和 Responses 灰度测试后，才可启用 YuAPI 渠道 `2556`。新增 Anthropic 或 Gemini 号池时，应建立独立 Sub2API 组、独立密钥和对应协议的 YuAPI 渠道。

## 回滚

成功部署时不要执行以下命令；它们仅用于故障回滚。

1. 在 YuAPI 后台保持或恢复渠道 `2556` 为禁用。
2. 恢复 Caddy 备份到 `/opt/edge/Caddyfile`，验证后只重建 `yuapi-caddy`，并按 `/opt/yuapi-sub2api-v2/caddy-networks-before.txt` 恢复网络连接。
3. 停止新栈但保留数据：

   ```bash
   docker compose -p yuapi-sub2api-v2 \
     -f /opt/yuapi-sub2api-v2/docker-compose.yml \
     --env-file /opt/yuapi-sub2api-v2/.env stop
   ```

禁止执行 `docker compose down -v`，禁止删除三个新卷，禁止修改现有 YuAPI/NewAPI 数据库、Redis、镜像或容器。既有 `sub2api_sub2api-network` 是生产共享网络，不得删除。
