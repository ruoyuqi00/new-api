# YuAPI 服务器与域名迁移手册 - 2026-07-11

本文用于把当前 YuAPI / YuCore 生产环境迁移到新服务器和新域名。文档不保存
密码、API Key、Cloudflare Token、数据库 DSN 或其他凭据。

## 推荐结论

不要先把旧服务器整机搬到新厂商后直接切域名。更稳妥的顺序是：

1. 先把新域名加入 Cloudflare，但暂不切生产流量。
2. 在新厂商创建一台干净的新服务器，单独配置 Docker、防火墙、时区和磁盘。
3. 从旧服务器生成一致性备份，在新服务器恢复 MySQL 和必要文件。
4. 在新服务器用临时域名或本机 `hosts` 完成全链路验收。
5. 安排短维护窗口，停止写入，做最后一次增量或最终全量备份。
6. 修改 Cloudflare DNS，把新域名切到新服务器。
7. 旧服务器保留 3 至 7 天，只作为回滚源，不立即释放或清空。

这种并行迁移方式比整机转移更容易验证，也能保留明确的回滚入口。只有当服务器
厂商提供同平台、同磁盘格式、同网络环境的可靠快照迁移，并且已经演练过恢复时，
才考虑厂商提供的整机转移。

## 当前需要保留的生产边界

- 当前公开服务入口是 `https://api.dtrljm.com`。
- 当前生产应用使用 Docker Compose，核心容器是 NewAPI、MySQL 和 Redis。
- 当前仍保留旧 Sub2API 的 PostgreSQL、Redis、配置和卷作为冷备与回滚材料。
- 当前域名反向代理仍依赖旧服务器上的 `sub2api-caddy`，在新入口验证前不能停止。
- 迁移期间不能顺手修改渠道、模型池、用户分组、倍率、计费公式或账号数据。
- 新服务器必须使用固定 `SESSION_SECRET`；否则应用重启会让已有登录会话失效。

## Cloudflare 新域名配置

### 1. 添加站点

1. 在 Cloudflare 控制台选择 **Add a site**。
2. 输入新域名，例如 `example.com`，选择合适的套餐。
3. Cloudflare 会给出两条权威 Nameserver。
4. 到域名注册商把原 Nameserver 替换成 Cloudflare 提供的两条。
5. 等待 Cloudflare 显示站点为 Active。

Nameserver 生效期间不要删除旧域名记录，也不要提前停旧服务器。

### 2. 预建 DNS

建议至少准备：

| 类型 | 名称 | 目标 | 代理状态 | 用途 |
| --- | --- | --- | --- | --- |
| A | `api` | 新服务器公网 IP | Proxied | 浏览器和普通 API 主入口 |
| A | `origin-api` | 新服务器公网 IP | DNS only | 故障排查，验收后可限制来源或删除 |
| CNAME/A | 根域名 | 按产品需要 | Proxied | 官网入口，可选 |

正式切换前，可以先使用 `staging-api.example.com` 指向新服务器做验收。不要让测试
域名被搜索引擎收录，也不要在测试域名公开真实管理入口。

### 3. SSL/TLS

- Cloudflare SSL/TLS 模式使用 **Full (strict)**，不要使用 Flexible。
- 源站可使用 Cloudflare Origin Certificate，或由 Caddy/Let's Encrypt 签发公信证书。
- 新证书应覆盖正式 API 域名和验收域名。
- 开启 Always Use HTTPS。
- HSTS 在新域名稳定运行后再开启；未验证回滚前不要启用长时间 HSTS。

### 双站点入口与独立 API 入口

`yuaiapi.com` 建议采用三个独立 Hostname，而不是让官网和模型 API 共用同一条
Cloudflare 地区限制规则：

| Hostname | 用途 | 中国大陆限制 |
| --- | --- | --- |
| `yuaiapi.com` | 受限官网与用户端 | 可按需阻止中国大陆 IP |
| `global.yuaiapi.com` | 全球备用官网 | 不限制 |
| `api.yuaiapi.com` | SDK、模型调用与后台 API | 不使用地区限制 |

三个 Hostname 可以指向同一台源站、同一个应用容器。Caddy 按 Hostname 接收请求后
反代到同一个本地应用端口。两个官网入口不要互相强制跳转，否则受限入口会失去独立
测试和切换价值。

Cloudflare 自定义 WAF 规则只匹配受限官网：

```text
(http.host in {"yuaiapi.com" "www.yuaiapi.com"} and ip.src.country eq "CN")
```

动作为 `Block`。需要开放时禁用该规则，需要限制时启用该规则。由于规则只匹配
根域名和 `www`，不会影响 `global.yuaiapi.com`，也不会影响 `api.yuaiapi.com` 的
`/v1/*` 模型请求。

该规则根据访问者出口 IP 判断。中国大陆用户连接境外 VPN 后，出口 IP 不再是 `CN`，
因此能够访问受限官网，符合预期。但公开保留一个不受限制的官网地址并不
等同于严格的地区合规封锁；它只是一种运营冗余方案。

### 4. WAF 与 API 规则

- `/v1/*`、`/api/*` 不能出现交互式 Challenge，否则 SDK 和服务端客户端会失败。
- 登录、注册页面可以继续使用 Turnstile。
- 为 `/v1/*` 配置基于 IP、Token 或业务需求的限速，不要使用浏览器质询。
- WebSocket、SSE 和流式响应必须保持可用。
- 大文件上传需要同时检查 Cloudflare 套餐限制、Caddy/Nginx 限制和应用限制。
- 超长非流式任务可能触发 Cloudflare 超时；耗时媒体任务应采用创建任务后轮询，
  不应让一个 HTTP 请求长时间等待生成完成。

## 新服务器基础配置

推荐使用与当前环境兼容的 Linux LTS 版本，并完成：

1. 创建带 `sudo` 权限的运维用户。
2. 配置 SSH Key，确认密钥登录可用后关闭 root 密码远程登录。
3. 开启防火墙，只开放 `22`、`80`、`443`；MySQL 和 Redis 不暴露公网。
4. 安装 Docker Engine 和 Docker Compose Plugin。
5. 设置时区为 `Asia/Shanghai`，启用时间同步。
6. 为 Docker 数据、数据库和备份预留足够磁盘与 inode。
7. 设置日志轮转、磁盘告警和容器健康检查。

若必须临时使用 root 密码，可以提供一次性密码完成首次配置；迁移结束后应立即改密
码并改为 SSH Key。不要提供服务器厂商账号，也不要提供 Cloudflare Global API Key。

## Cloudflare 最小权限凭据

自动配置 DNS 时，优先创建临时 API Token：

- Zone / Zone / Read
- Zone / DNS / Edit
- 资源范围只选择新域名对应的 Zone
- 设置较短有效期，迁移完成后撤销

如果还要自动管理 Origin Certificate 或特定 WAF 规则，再按实际操作单独增加最小
权限。不要直接共享 Cloudflare 主账号密码或 Global API Key。

## 需要迁移的数据

### 必须迁移

- MySQL 主数据库：用户、令牌、渠道、模型映射、分组、计费设置、系统选项、任务和日志索引。
- 当前生产 Compose 文件及其环境变量。
- 固定的 `SESSION_SECRET` 和其他应用运行密钥。
- YuCore 上传素材目录和应用挂载的数据目录。
- 当前生产镜像对应的代码提交或可重复构建的镜像包。
- Caddy/Nginx 配置、证书策略和域名路由规则。

### 可选择迁移

- Redis RDB/AOF：如果 Redis 只承担缓存和限流，可以在新服务器重建；如果保存关键
  队列或不可重建状态，则必须在停写窗口迁移。
- 应用历史日志：可以只归档，不必全部恢复到在线磁盘。
- 旧 Sub2API PostgreSQL、Redis 和卷：建议整体冷备到对象存储或加密归档，暂不接入
  新生产流量。

### 不建议直接复制

- 不要跨服务器直接复制正在运行的 MySQL Docker volume。
- 不要在 MySQL 未停写时复制底层数据目录。
- 不要混用不同 MySQL 大版本的数据目录。
- 不要只复制容器而遗漏 Compose、挂载卷、环境变量和外部代理配置。

## 备份与恢复方法

### MySQL

使用逻辑备份，确保一致性：

```bash
mysqldump \
  --single-transaction \
  --routines \
  --triggers \
  --events \
  --hex-blob \
  --default-character-set=utf8mb4 \
  --databases NEWAPI_DATABASE \
  > newapi.sql
```

数据库密码应放在权限为 `600` 的临时客户端配置文件中，不要直接写进命令历史。
恢复后检查表数量、关键表行数、字符集、最大 ID 和最近记录时间。

### Redis

如果需要迁移 Redis：

1. 记录当前持久化模式和 Redis 版本。
2. 在维护窗口执行 `BGSAVE`，确认完成。
3. 复制 RDB/AOF 到相同或兼容版本的新 Redis。
4. 启动后检查 key 数量、过期时间和错误日志。

如果 Redis 只作为缓存，推荐让新实例从空库启动，避免带入过期限流和旧节点缓存。

### 文件与配置

建议归档：

```text
/opt/newapi/
/opt/newapi/backups/
/opt/sub2api/              # 冷备，暂不启用旧应用
反向代理配置目录
应用上传与媒体缓存挂载目录
```

归档应生成 SHA-256 校验值，并在新服务器解压后重新校验。任何包含密钥的归档都应
加密传输和加密保存。

## 分阶段迁移流程

### 阶段 A：盘点与备份演练

- 记录容器镜像、容器 ID、健康状态、端口、网络和卷。
- 记录数据库版本、库大小、表数量和备份耗时。
- 生成一次预迁移备份并在隔离环境试恢复。
- 验证当前域名和关键 API，保存基线结果。

### 阶段 B：搭建新服务器

- 安装 Docker 和防火墙。
- 恢复 Compose、应用镜像、MySQL 和必要文件。
- Redis 按最终决定恢复或重建。
- 使用 staging 域名或 `hosts` 访问，不切正式 DNS。

### 阶段 C：迁移前验收

至少验证：

- `/`、`/api/status`、`/api/pricing` 返回正常。
- 未认证 `/v1/models` 返回预期的 `401`。
- 普通用户登录、退出、刷新会话正常。
- 管理员后台、用户数据、API Key、渠道数量和模型列表一致。
- 文本模型非流式与流式请求正常。
- Studio、无限画布、上传、任务列表和素材读取正常。
- Cloudflare 下的 Turnstile、SSE、WebSocket 和大请求体正常。
- MySQL、Redis 和应用容器健康，重启后仍能恢复。

### 阶段 D：最终切换

1. 公告短维护窗口，暂停后台配置和用户写入。
2. 记录旧库最后写入时间和关键表最大 ID。
3. 生成最终 MySQL 备份并恢复到新服务器。
4. 同步最终上传文件和必要 Redis 状态。
5. 再次执行关键 smoke test。
6. 修改 Cloudflare A/AAAA 记录到新服务器。
7. 从多个网络验证新域名、证书、登录和 API。
8. 观察错误率、延迟、容器重启、数据库连接和磁盘。

### 阶段 E：观察与退役

- 旧服务器保持只读或停止应用但保留数据，至少观察 3 至 7 天。
- 每天确认新服务器备份可用。
- 确认 OAuth 回调、邮件链接、Webhook、Turnstile Hostname 和第三方白名单已更新。
- 观察期结束后先做最终加密归档，再释放旧服务器。

## 域名相关配置清单

换域名不只是修改 DNS，还要检查：

- Cloudflare Turnstile 允许的 Hostname。
- GitHub、Discord、OIDC 等 OAuth Callback URL。
- 邮件中的站点链接与密码重置链接。
- 支付回调、Webhook 和第三方 API IP/域名白名单。
- CORS、Cookie Domain、SameSite、Secure 和反向代理 Host Header。
- 网站标题、公开 API Base URL、文档示例和 SDK 配置。
- Cloudflare WAF、Rate Limit、缓存规则和上传限制。
- 监控、Uptime Kuma、日志告警和证书到期提醒。

旧域名建议至少保留 30 天：网页流量使用 `301` 跳转；API 流量不要盲目跳转，先给
客户端留迁移期，旧 API 域名可临时反代到新服务器。

## 回滚条件与方法

出现以下任一情况应停止切换并回滚：

- 登录或会话大面积失败。
- 关键模型请求失败率明显升高。
- 新数据库缺表、缺记录或持续出现写入错误。
- Cloudflare 阻断 `/v1/*`、SSE 或 WebSocket。
- 新服务器出现持续重启、磁盘满或数据库连接耗尽。

回滚步骤：

1. 暂停新服务器写入，保存故障现场和新产生的数据。
2. 将 Cloudflare DNS 指回旧服务器。
3. 恢复旧服务器应用写入。
4. 对比切换窗口中新旧数据库差异，制定数据回灌方案。

不能在新旧两套数据库同时开放写入，否则会产生难以合并的双写分叉。

## 迁移所需信息

实际执行前需要准备：

- 新服务器公网 IP、系统版本和配置规格。
- 一个临时 sudo/root 登录方式，优先 SSH Key。
- 新域名及 Cloudflare Zone 状态。
- 最小权限 Cloudflare API Token，或由域名所有者手动完成 DNS 操作。
- 计划维护时间和允许的最大停写时间。
- 是否保留旧域名，以及旧域名计划保留多久。
- Redis 是恢复还是重建的决定。
- 新服务器是否继续使用 Caddy，或改用其他反向代理。

不需要提供服务器厂商主账号。已有旧服务器 SSH 权限时，也不需要另外发送数据库
密码；可以在服务器内使用现有 Compose 环境完成备份，并避免把数据库凭据带出服务器。

## 迁移完成标准

- 新域名经 Cloudflare 提供有效证书并稳定访问。
- 新服务器所有核心容器健康，重启次数为 0。
- 数据、渠道、用户、API Key、配额和关键配置核对一致。
- 登录、文本 API、流式 API、Studio、Canvas 和上传链路通过。
- 定时备份、监控、日志轮转和告警已启用。
- 已完成一次可执行的 DNS 回滚演练。
- 临时 SSH 密码和 Cloudflare Token 已撤销或轮换。
