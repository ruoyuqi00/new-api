# YuCore 新服务器迁移续接文档

## 这份文档的用途

这是一份跨窗口续接文档。新窗口必须先完整阅读本文件，再阅读正式
runbook 和准备审计，然后才能进行任何服务器操作。

当前允许开始的是：购买新服务器后的只读验机和新服务器预配置。

当前不允许开始的是：停旧服务、写生产数据、修改 Cloudflare、修改 DNS、
切换流量、推送当前分支或部署生产镜像。

拿到新服务器地址和登录方式，不等于获得生产迁移授权。真正进入维护窗口前，
必须由用户再次明确授权。

## 来龙去脉

本项目是 new-api 二开，并融合了 YuCore 品牌 UI、sub2api 账号池能力、私密分组、
渠道调度、模型映射、计费、任务和图片/视频多模态能力。前期为了升级 UI 和父项目
策略，曾经出现过多条本地分支、候选 UI 和实验性实现。

用户已经明确要求：迁移到新服务器的必须是当前经过验收的“线上生产复刻版”，
任何实验 UI、候选 UI、错误分支、临时构建产物都不能进入生产。

为此，迁移准备固定在独立工作树：

```text
D:\yucore-local-production
```

对应分支：

```text
codex/local-production-brand-performance-20260725
```

这棵工作树包含用户已经确认的生产候选 UI 和后端行为。迁移期间不得从其他目录、
其他分支或旧实验构建中复制 `web/`、`dist/`、静态资源、Docker 镜像或后端文件。

## 第一硬门：实验 UI 零容忍

以下规则优先级最高：

1. 唯一允许作为迁移源的工作树是 `D:\yucore-local-production`。
2. 不得从 `D:\newapi-710-yuapi` 当前工作树或任何其他本地目录复制 UI/构建产物，
   不设逐文件批准例外。
3. 不得 merge、rebase 或 cherry-pick 任何实验 UI 分支到迁移源。
4. 迁移期间不得“顺便升级 UI”、替换主题、重新生成海报、替换首页资源或引入
   新的父项目 UI。
5. 最终镜像必须从经过确认的 `ACCEPTED_COMMIT` 执行 `git archive` 后构建，
   不能直接用脏工作区作为 Docker build context。
6. 在确定 `ACCEPTED_COMMIT` 前，必须确认下列命令无输出：

```powershell
Set-Location D:\yucore-local-production
git diff --name-only 95fc952e8..HEAD -- web
```

`95fc952e8` 已包含迁移准备开始时认可的生产候选 UI。该基线之后如果出现任何
`web/` 差异，都视为迁移污染，必须先停下审查，不能继续构建。

如果用户在迁移过程中提出任何 UI 变更，本次迁移流程必须暂停。UI 变更只能进入
新的生产候选，并重新完成 UI、后端、构建、迁移演练和用户验收；不能直接把文件
复制进当前迁移源。

## 当前本地状态

准备审计提交：

```text
1dd1e2f2f3f8da4d98f5d85d47f17bf7a180fe12
```

本续接文档所在提交可用以下命令读取：

```powershell
git log -1 --format=%H -- docs/superpowers/handoffs/2026-07-28-new-server-migration-handoff.md
```

最后一次检查时：

- 分支为 `codex/local-production-brand-performance-20260725`；
- 已跟踪文件是干净的；
- 只有既有未跟踪目录 `.superpowers/`，不得删除、提交或用于构建；
- 本地生产候选预览运行在 `http://127.0.0.1:13000`；
- 本地状态接口返回 HTTP 200；
- 没有遗留迁移演练容器、网络或卷；
- 没有 push；
- 没有生产部署；
- 没有 Cloudflare/DNS 修改；
- 没有生产容器、数据库、Redis 或 Caddy 修改。

本地预览账号属于本地 SQLite 数据库，不得把本地数据库、账号或密码迁移到生产。

## 已完成的验证

迁移工具：

- Python 迁移守卫测试 40/40 通过；
- 完整 MySQL/Redis/Caddy 本地迁移演练通过；
- 演练同时验证正向恢复和带切换后新增数据的反向恢复；
- MySQL Unicode、结构哈希和内容哈希通过；
- Redis 持久化和过期键规则通过；
- 维护页返回 503 和 `Retry-After`；
- Caddy 完成 OLD -> maintenance -> NEW -> OLD；
- 演练后容器、网络、命名卷、匿名卷和临时目录增量均为 0。

应用候选：

- 计划内 Go 测试全部通过；
- `go build ./...` 通过；
- `web/default` 131/131 测试通过；
- `web/default` typecheck 和 production build 通过；
- `web/classic` production build 通过；
- `git diff --check` 通过。

第一次 Go 测试曾因本地机器只剩约 1.96 GiB 内存，在测试运行时启动前发生内存
分配失败，没有断言失败。Go 进程退出、可用内存恢复后，完全相同的命令复跑通过。
这属于本地资源事件，不是代码失败。

完整证据见：

- `docs/superpowers/acceptance/2026-07-28-cross-server-migration-preparation-audit.md`
- `docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md`
- `docs/superpowers/specs/2026-07-28-production-cross-server-migration-design.md`
- `docs/superpowers/plans/2026-07-28-production-cross-server-migration.md`

## 第二硬门：当前分支禁止直接 push

当前跟踪树已经不包含已知旧 SSH 凭据，通用 API/Cloudflare 密钥扫描也为 0。

但是，本地分支较早的历史提交曾把一个旧 SSH 凭据写进自检命令。该值已经从当前
树删除，Git 历史仍保留旧对象。因此：

1. 当前分支禁止直接 push。
2. 迁移前必须轮换 OLD 服务器的旧 SSH 凭据。
3. 必须从审查后的当前树生成干净 squash 分支，或完整清理敏感历史。
4. 必须重新检查当前树和分支历史，确认旧凭据不存在。
5. `ACCEPTED_COMMIT` 只能指向这个干净且复审通过的新提交。
6. 不得在聊天、文档、commit、命令参数或日志中再次写出旧值。

不要在未确认生产基线前擅自改写当前分支历史。先识别生产基线，再创建干净提交，
保留当前本地分支作为只读证据。

## 新服务器购买后的必需输入

用户提供以下信息后，只能先做只读验机：

- 新服务器公网 IPv4；
- 可选公网 IPv6；
- SSH 端口；
- 临时 SSH 用户和临时公钥访问方式；
- 购买配置或订单截图；
- CPU 型号和核心线程数；
- 内存容量；
- NVMe 型号、数量、容量；
- RAID 方案；
- 操作系统和版本；
- 端口速率、实际带宽、流量限制和机房位置；
- 服务商防火墙当前规则截图。

优先使用临时 SSH 公钥，不要要求用户再次发送长期 root 密码。不要要求用户发送
Cloudflare 主账号密码或服务商面板密码。

建议初始系统为 Ubuntu 24.04 LTS minimal、x86_64、无控制面板。双 NVMe 优先
RAID1。实际磁盘、RAID、文件系统和网络必须以新服务器只读证据为准，不能只信订单。

## 新窗口收到服务器信息后的第一阶段

第一阶段只允许对 NEW 服务器操作，不连接 OLD，不修改生产：

1. 验证 SSH 主机指纹并记录。
2. 读取 CPU、内存、NUMA、NVMe SMART、RAID、文件系统和挂载。
3. 验证 OS、内核、时钟、时区和 NTP。
4. 验证公网 IPv4/IPv6、实际上下行和端口速率。
5. 检查服务商防火墙和主机防火墙现状。
6. 检查 80、443、3000、3001、3306、6379 等监听。
7. 检查 Docker/Compose 版本；如果未安装，先给出安装计划再执行。
8. 把只读结果与订单配置比较；不一致时停止，不继续迁移。

连接 NEW 做只读验机，不代表可以连接或修改 OLD。连接 OLD 前也必须由用户在新窗口
中明确授权，并使用轮换后的临时凭据。

## 最终构建源和镜像边界

正式构建前必须完成：

1. 当前树无实验 UI 污染。
2. 旧凭据已轮换。
3. 干净 squash 分支已经创建。
4. 当前树和历史密钥扫描通过。
5. 用户确认干净提交作为 `ACCEPTED_COMMIT`。
6. `HEAD == ACCEPTED_COMMIT`。
7. 从 `ACCEPTED_COMMIT` 使用 `git archive` 生成构建源。
8. 构建 `linux/amd64` 不可变镜像。
9. 记录 commit、image ID、image archive SHA-256 和 helper SHA-256。
10. 新旧两端验证所有哈希一致。

不得把本地 `one-api.db`、`.local-preview/`、`.env.local`、日志、缓存、
`node_modules` 或 `.superpowers/` 放进构建或传输包。

## Cloudflare 当前状态和切换边界

现在不要修改 Cloudflare。

已确认的四条记录：

| 主机名 | 当前/目标代理状态 | TTL |
|---|---|---|
| `yuaiapi.com` | Proxied，保持橙云 | 保持原值/自动 |
| `api.yuaiapi.com` | Proxied，保持橙云 | 保持原值/自动 |
| `global.yuaiapi.com` | Proxied，保持橙云 | 保持原值/自动 |
| `vip.yuaiapi.com` | DNS-only，保持灰云 | 保持 300 秒 |

迁移只允许修改这四条 A 记录的 `content`，即 OLD IPv4 -> NEW IPv4。
记录 ID、名称、类型、`proxied` 和 TTL 必须保持不变。

以下内容不得修改：

- MX、SPF、DKIM、DMARC；
- SSL/TLS 模式；
- WAF、规则集、缓存、重定向；
- Turnstile；
- 无关 DNS 记录。

如果新快照证明 Origin Rules 或 Load Balancer Pool 写死 OLD IP，才允许加入变更计划。

正式 runbook 只支持 Cloudflare API 受控变更路径，需要：

- `CF_ZONE_ID`；
- 临时最小权限 Zone DNS Read/Edit Token；
- 两人检查四记录 dry-run plan；
- 七天回滚保留期结束并通过独立延迟清理授权后撤销 Token。

Token 不得出现在进程参数、日志、文档或 commit 中。runbook 使用 mode-0600 临时
curl config，并在每次成功或失败时清理。回滚保留期内，Token 只能保存在操作员
控制的外部安全存储中，不能留在 OLD、NEW 或 CONTROL 的普通文件、环境变量、
shell 历史和进程中。

## 迁移阶段顺序

### 阶段 A：NEW 只读验机

验证实际硬件、系统、网络和防火墙。失败即停止。

### 阶段 B：干净提交和不可变制品

轮换 OLD 凭据，建立无敏感历史的 squash 分支，确认 `ACCEPTED_COMMIT`，构建并
校验不可变镜像和迁移 helper。

### 阶段 C：NEW 预配置

安装并锁定 Docker/Compose、目录、权限、备份单元、MySQL、Redis、Caddy 和应用
候选。公网入口保持阻断。

### 阶段 D：预迁移

从 OLD 生成初始逻辑 MySQL dump、Redis 持久化快照和文件同步，恢复到 NEW，使用
独立 MySQL/Redis verifier 验证相同不可变快照。OLD 继续提供生产服务。

### 阶段 E：私下验收

使用 `curl --resolve` 和临时凭据检查：

- 匿名首页、主题、登录、注册、定价和模型广场；
- 普通用户资料、钱包、日志、任务和私密组权限；
- 管理员用户、渠道、账号池、映射、计费、备份和系统设置；
- 普通/流式聊天；
- 映射模型不泄露真实上游；
- 图片和视频按次计费、预扣、结算和退款；
- 最小成本图片/视频；
- SMTP、Turnstile 和 OAuth 回调；
- MySQL、Redis 和文件清单。

所需临时输入包括：

- 下游 API key；
- 用户 session/PAT；
- 管理员 PAT；
- 私密组标识；
- 对外映射模型标识；
- 最小成本普通聊天、图片和视频模型标识。

### 阶段 F：维护窗口

只有用户再次明确授权后才能进入：

1. OLD 切维护页。
2. 强制停止 OLD 应用，目标五秒内完成。
3. 生成最终 MySQL 逻辑 dump。
4. Redis `SAVE`、记录清单、停止并归档。
5. 最终文件增量和哈希传输。
6. NEW 恢复并启动唯一写主。
7. 完成健康、镜像、重启次数、数据清单和最小付费探针。

MySQL 不能复制物理数据目录。密码必须留在容器环境的 `MYSQL_PWD` 中，不能出现在
命令参数或输出。

### 阶段 G：写权桥接、入口和 DNS

顺序不能改变：

1. 在 OLD/CONTROL 记录并哈希 NEW 写权边界 marker。
2. OLD Caddy 桥接到 NEW。
3. 服务商防火墙从 OLD-only 转为审核后的公网 80/443。
4. NEW 主机防火墙开放审核后的 80/443。
5. 从 CONTROL 和 OLD 分别验证 NEW/桥接路径。
6. 获取 CF DNS 快照并生成只含四条 A 记录的 plan。
7. 人工检查后应用四条记录。
8. 至少观察两个 `vip` TTL 周期。

`vip` 是直连入口，因此服务商防火墙不能只允许 Cloudflare IP；它必须按 runbook
显式支持公网 80/443，同时继续阻断 3000、3001、3306、6379。

### 阶段 H：观察和延迟清理

在 1、5、15、30、60 分钟采样：

- HTTP 状态、流式和长请求；
- OOM、内核、CPU、内存、磁盘、网络；
- 路由、账号池、429/5xx 回退；
- 私密组、模型映射和计费；
- 图片、视频、SMTP、注册；
- 备份状态。

OLD 至少保留七天。没有用户明确批准，不能删除 OLD 资源或撤销回滚能力。

## 回滚分界

### Pre-traffic rollback

只有 NEW 写权 marker 不存在、OLD bridge 未激活、生产写请求尚未到达 NEW 时才允许。
必须确认 NEW 应用已停止，才能启动 OLD，避免双写主。

### Post-traffic rollback

一旦 NEW 写权 marker 存在或 OLD bridge 已激活，禁止直接启动冻结的 OLD 数据。
必须重新进入维护：

1. 停止 NEW 写入。
2. 从 NEW 反向导出 MySQL、Redis 和文件。
3. 恢复到 OLD。
4. 比较清单。
5. 启动 OLD 的不可变镜像。
6. NEW 桥接回 OLD。
7. 回退四条 DNS 记录。
8. 等待两个 TTL 周期后再收口防火墙和 marker。

## 明确禁止事项

- 禁止混入实验 UI 或其他分支的 `web/`/`dist/`。
- 禁止直接 push 当前凭据污染历史分支。
- 禁止仅因收到服务器凭据就启动生产迁移。
- 禁止白天停止 OLD 生产服务。
- 禁止在私下验收完成前改 CF/DNS。
- 禁止使用 `docker prune` 或任何全局清理。
- 禁止按“主机上第一个 MySQL 镜像”选择容器；必须使用精确容器名。
- 禁止物理复制 live MySQL 数据目录。
- 禁止公开 3306、6379、3000、3001。
- 禁止在命令参数、文档、commit、日志中写密码或 Token。
- 禁止忽略失败继续切换。
- 禁止在 NEW 和 OLD 同时保持可写应用。
- 禁止没有反向数据迁移就进行 post-traffic rollback。
- 禁止迁移时顺便做 UI、路由、计费或模型接入功能开发。

## 新窗口的第一条操作

新窗口必须先执行本地只读检查：

```powershell
Set-Location D:\yucore-local-production
git status --short --branch
git log -5 --oneline
git diff --check
git diff --name-only 95fc952e8..HEAD -- web
```

预期：

- 分支名称正确；
- 只有 `.superpowers/` 未跟踪；
- `git diff --check` 无输出；
- `95fc952e8..HEAD -- web` 无输出。

如果不符合，先停下，不要 SSH、不要构建、不要迁移。

随后阅读：

```text
docs/superpowers/acceptance/2026-07-28-cross-server-migration-preparation-audit.md
docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md
docs/superpowers/specs/2026-07-28-production-cross-server-migration-design.md
```

如果用户已提供新服务器信息，先只读验 NEW。如果信息不完整，只询问缺失项，不要
猜测。不要使用历史窗口中的旧 SSH 密码。

## 可直接粘贴到新窗口的续接提示词

```text
请先完整阅读：
D:\yucore-local-production\docs\superpowers\handoffs\2026-07-28-new-server-migration-handoff.md
D:\yucore-local-production\docs\superpowers\acceptance\2026-07-28-cross-server-migration-preparation-audit.md
D:\yucore-local-production\docs\superpowers\runbooks\2026-07-28-production-cross-server-migration.md

唯一允许使用的迁移工作树是 D:\yucore-local-production。
不要合并、复制或构建任何实验 UI、其他分支 UI、其他目录的 web/dist 或临时资源。
先做本地只读完整性检查；如果我提供了新服务器信息，只允许先对 NEW 做只读验机。
不要连接或修改 OLD 生产端，不要改 Cloudflare/DNS，不要 push，不要部署，不要停服务。
收到服务器凭据不等于获得生产迁移授权。

当前分支历史含已从当前树删除的旧凭据，禁止直接 push。迁移前必须先轮换旧凭据，
再从已审查当前树建立干净 squash 分支并重新做历史密钥扫描。

请先报告：
1. 当前本地分支/状态是否符合续接文档；
2. 新服务器信息是否齐全；
3. 你准备执行的第一批只读验机命令；
4. 如何证明实验 UI 不会进入 ACCEPTED_COMMIT 和最终镜像。

未得到我新的明确授权前，不进入维护窗口和生产切换。
```

## 完成定义

“准备完成”不等于“迁移完成”。只有以下条件全部成立，才能宣布迁移完成：

- 干净 `ACCEPTED_COMMIT` 已确认；
- 不可变镜像和所有制品哈希一致；
- NEW 硬件、系统、网络和防火墙验收通过；
- 私下匿名/用户/管理员/下游/图片/视频验收通过；
- 最终 MySQL、Redis、文件清单一致；
- OLD bridge、服务商防火墙、主机防火墙和四条 CF 记录按顺序切换；
- 观察窗口无不可接受的 OOM、5xx、路由、计费或长请求问题；
- 回滚能力完整保留；
- 用户明确确认迁移完成。
