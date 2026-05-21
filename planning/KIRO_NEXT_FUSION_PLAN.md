# Kiro Next Fusion Plan

本文是 Kiro 后续融合升级清单。目标是把现在“能跑”的临时组合，逐步收敛成
可维护、可回滚、可批量导入、可从多个上游项目吸收变化的 Sub2API 内部能力。

不要在本文档写入任何原始 token、refresh token、密码、cookie、API key 或
服务器登录信息。

## 当前基线

代码基线：

- 私有 fork 已持续合并官方 `Wei-Shaw/sub2api` 上游。
- 已有 Kiro 导入桥：`POST /api/v1/admin/accounts/import/kiro`。
- 导入完整 Kiro JSON 时已尽量保留 `accessToken`、`refreshToken`、
  `profileArn`、`expiresAt`、`loginHint` 和嵌套原始元数据，但响应必须脱敏。

服务器基线：

- 公网入口：`https://api.vyywcw.cn/`
- Sub2API 是唯一公网入口。
- Kiro/Windsurf 相关 adapter 只应作为内网服务运行。
- 当前 Kiro 已验证公开模型仍应以 smoke 结果为准，不因为隐藏模型列表存在就公开。

Kiro 关键判断：

- 本地历史验证说明：`kiro.rs` 在特定本地代理环境下曾经可以访问 Claude 系列，
  不能简单说“kiro.rs 不支持 Claude”。
- 服务器上是否可用，取决于账号凭据、region、profileArn、machineId、出口网络、
  engine 路线和官方当前策略。
- 后续问题要拆成：
  - 凭据导入是否保留完整 metadata；
  - 服务器出口是否等价于本地可用环境；
  - Web Portal route 和 IDE/AmazonQ route 各自支持哪些模型；
  - 模型是否经过 direct smoke 和 public smoke。

## 参考项目策略

从 2026-05-21 开始，Kiro 新功能不再默认以 `kiro.rs` 为主参考。

新的优先级：

1. 本 fork 的 Sub2API private adapter 和 `adapters/kiro-web`：
   决定公网行为和当前 Web Portal Claude/Opus 路线。
2. `Quorinex/Kiro-Go`：
   优先参考服务端 Go 结构、账号池、Docker、模型感知路由。
3. `chaogei/Kiro-account-manager`：
   优先参考 KAM 的账号策略、断路器、`round-robin/sticky`、
   token refresh single-flight、模型发现和 endpoint fallback。
4. `Jwadow/kiro-gateway`：
   继续作为已部署 open-model fallback/reference。
5. `hank9999/kiro.rs`：
   降级为 legacy 参考，只用于旧 IDE/API key 行为、历史回归和回滚判断。

完整说明见：

- `planning/KIRO_KAM_ROUTE_RESEARCH_2026-05-21.md`
- `planning/KIRO_RUNTIME_ADMIN_CONSOLE_PLAN_2026-05-21.md`

## 阶段 1：Kiro Runtime 管理后台

目标：

- 不再只有一个“导入 Kiro”弹窗。
- 在 Sub2API 后台增加 `Kiro Runtime` 管理页。
- 页面能看账号、额度、模型、路由、smoke 和同步状态。

任务：

- [ ] 新增 Kiro Runtime 页面入口。
- [ ] 显示 runtime online/degraded/offline。
- [ ] 显示账号池摘要：total/available/cooldown/quota_exhausted。
- [ ] 显示模型摘要：public/smoke_passed/disabled/investigation。
- [ ] 复用现有 Kiro import handler。
- [ ] 所有响应递归脱敏。

验收：

- 登录 Sub2API 后台可以看到 Kiro Runtime 页面。
- 内部 Kiro 服务离线时页面不崩溃。
- 页面不显示原始 token、refresh token 或 Authorization header。

## 阶段 2：账号池和导入升级

目标：

- 账号导入支持 KAM export JSON、Kiro-Go JSON 和现有完整 JSON。
- 同一账号重复导入做 upsert。
- 账号状态可以刷新和测试。

任务：

- [ ] 定义 Kiro account runtime state。
- [ ] 保存或读取脱敏账号状态。
- [ ] 支持 `profileArn`、`region`、`machineId`、`provider`、`authMethod`
      等 metadata 的存在状态。
- [ ] 增加 per-item import result。
- [ ] 增加账号 refresh/test 操作。

验收：

- 重复导入不会产生重复账号。
- 账号列表可以显示 plan、usage、reset_at、token status。
- 导入失败不会泄露原始凭据。

## 阶段 3：路由策略

目标：

- 吸收 KAM/Kiro-Go 的账号池策略。
- 支持 `round-robin`、`sticky`、模型感知路由和 token refresh single-flight。

任务：

- [ ] 默认策略为 `round-robin`。
- [ ] 增加 session sticky。
- [ ] 增加 `GetNextForModel(model)`。
- [ ] 增加失败分类：auth、quota、throttle、server、request。
- [ ] 增加 account cooldown 和 quota exhausted 状态。
- [ ] 增加同账号 token refresh single-flight。

验收：

- 多账号请求能负载均衡。
- 同一 Claude Code/OpenCode session 可以粘在同一账号。
- 某个账号 429 后自动切换账号。
- token 临期时不会多个并发请求同时刷新同一账号。

## 阶段 4：模型发现和 smoke 自动化

目标：

- 模型公开由 smoke 决定，而不是由静态列表或隐藏模型猜测决定。
- Kiro Claude/Opus 必须 direct smoke 和 public smoke 都通过后才能公开。

任务：

- [ ] 每账号调用模型发现。
- [ ] 维护 `account_id -> model_set` 缓存。
- [ ] 增加单账号单模型 smoke。
- [ ] 增加账号池模型 smoke。
- [ ] 增加 Sub2API public smoke。
- [ ] smoke 结果写入 runtime 状态。

验收：

- 已公开模型都有最近 smoke passed 记录。
- 失败模型记录错误类型和短摘要。
- Claude/Opus 不会因为出现在 KAM 隐藏模型或静态 mapping 中就自动公开。

## 阶段 5：同步到 Sub2API

目标：

- 从 Kiro Runtime 页面创建或更新 Sub2API upstream account/model mapping/group。
- 默认只同步 smoke passed 模型。

任务：

- [ ] 增加 dry-run 同步。
- [ ] 增加实际同步。
- [ ] 同步 upstream account。
- [ ] 同步 model mapping。
- [ ] 同步 group 可用模型。
- [ ] 记录同步版本和时间。

验收：

- 同步前能看到将修改的内容。
- 同步失败不影响现有可用 provider。
- 可以一键回滚到同步前 mapping。

## 阶段 6：engine 可替换

目标：

- 不押注某一个开源项目永久可用。
- 内部 Kiro runtime 支持多个 engine。

候选 engine：

- `web-portal`：当前 Claude/Opus Web Portal route。
- `ide-amazonq`：KAM/Kiro-Go 风格的 IDE/Amazon Q/CodeWhisperer route。
- `legacy-kiro-rs`：旧 Rust 路线，只作为 fallback。
- `kiro-gateway`：当前 open-model runtime fallback。

任务：

- [ ] 定义统一 engine 接口。
- [ ] 每个 engine 返回统一账号状态、模型状态和 smoke 结果。
- [ ] 页面可以显示每个账号当前绑定 engine。
- [ ] engine 失败时只禁用该 engine，不影响 Sub2API 主服务。

验收：

- 切换 engine 不需要改外部 API。
- 某个 engine 出错不会拖垮 Windsurf 或其他 Sub2API provider。

## 暂不做

- 不直接把 KAM Electron 应用部署成服务器常驻服务。
- 不直接复制 KAM AGPL 代码进私有 fork。
- 不公开 Kiro runtime 独立后台端口。
- 不在未 smoke 通过前公开 Kiro Claude/Opus。
- 不把 refresh token 存入普通文档或日志。

## 每次改 Kiro 前的检查

- [ ] 备份服务器 compose 配置。
- [ ] 记录当前 Sub2API image/tag。
- [ ] 记录当前 Kiro runtime image/digest。
- [ ] 确认 adapter 端口未公开。
- [ ] 确认本地 git 已提交。

## 每次部署后的检查

- [ ] `docker compose ps`
- [ ] internal runtime health
- [ ] Kiro Runtime 页面可打开
- [ ] required model direct smoke
- [ ] Sub2API public smoke
- [ ] Kiro upstream account active/schedulable

## 回滚

- Kiro 页面出错：保留旧导入入口，隐藏新页面入口。
- Kiro runtime 出错：禁用 Kiro upstream account，不影响 Windsurf。
- 模型同步出错：回滚 Sub2API model mapping/group 配置。
- engine 出错：切回上一个已验证 engine。
