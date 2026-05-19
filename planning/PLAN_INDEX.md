# Sub2API Provider Adapter Planning Index

## 2026-05-19 Provider Account Operations

- `PROVIDER_ACCOUNT_OPERATIONS_GUIDE.md`
  - Current production workflow for importing Windsurf/Kiro credentials,
    verifying public Sub2API access, checking adapter privacy, and updating the
    server later.

## 2026-05-19 Kiro Gateway Runtime Docs

- `KIRO_GATEWAY_RUNTIME_REPORT_2026-05-19.md`
  - 记录本轮 Kiro gateway 服务器部署、参考项目、已验证模型、暂不启用的模型和短期技术判断。
- `KIRO_NEXT_FUSION_PLAN.md`
  - 记录 Kiro 后续融合升级计划，包括管理接口、模型 smoke、kiro.rs patch 方向、统一 adapter 抽象和回滚要求。

记录日期：2026-05-16

这个目录是 `sub2api-provider-adapters` 私人 fork 的工程计划区。它的作用是把后续 Windsurf、Kiro、服务器部署、接口调研、测试验收、回滚策略都固化成可执行文档，避免只靠聊天记录推进。

## 当前目标

短期目标：

- 继续保留 Sub2API 作为唯一公网入口。
- 在服务器内网运行 Windsurf/Kiro 兼容代理。
- 通过 Sub2API 的 `base_url + api_key` 上游账号能力接入这些代理。
- 外部用户只访问 Sub2API，不直接访问 Windsurf/Kiro 代理。

长期目标：

- 在这个私人 fork 里逐步增加 `windsurf`、`kiro` 原生账号导入和 token 生命周期管理。
- 保持可以从官方 `Wei-Shaw/sub2api` 持续合并更新。
- 当 Windsurf/Kiro 协议更新时，从参考项目吸收变化，先验证内网上游，再决定是否改 Sub2API 原生代码。

## 文档清单

- `ROADMAP_CHECKLIST.md`
  - 总路线图、阶段划分、每阶段交付物、完成标准。
- `INTERFACE_RESEARCH_PLAYBOOK.md`
  - 接口调研方法、Windsurf/Kiro/Sub2API 端点矩阵、抓包和样例记录模板。
- `WINDSURF_INTEGRATION_SPEC.md`
  - WindsurfAPI 作为内网上游的部署、账号导入、Sub2API 接入、后续原生化设计。
- `KIRO_INTEGRATION_SPEC.md`
  - Kiro proxy 作为内网上游的部署、凭据结构、Sub2API 接入、后续原生化设计。
- `KIRO_STAGE_B_IMPORT_ENDPOINT.md`
  - 已落地的 Sub2API 管理员 Kiro 凭据导入接口、环境变量、调用示例、测试记录和下一步。
- `SERVER_DEPLOYMENT_RUNBOOK.md`
  - 服务器部署、备份、升级、回滚、安全检查、上线步骤。
- `TEST_ACCEPTANCE_MATRIX.md`
  - 手工、接口、集成、回归、安全、部署验收测试矩阵。
- 根目录已有文档：
  - `PROVIDER_ADAPTER_REFERENCES.md`
  - `UPSTREAM_PROXY_PLAN.md`
  - `KIRO_INTEGRATION_NOTES.md`
  - `PRIVATE_GIT_WORKFLOW.md`

## 当前代码和上游基线

私人仓库：

- GitHub: `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`
- 本地目录: `D:\wflogin\sub2api-private`
- 当前分支: `main`
- 官方远端: `upstream = https://github.com/Wei-Shaw/sub2api.git`
- 官方 push: disabled

已观察版本：

- Sub2API upstream: `8584b8f7 Merge pull request #2504 from yetone/fix-admin-settings-darkmode`
- WindsurfAPI: `dwgx/WindsurfAPI`, `c028576 release: 2.0.96`, tag `v2.0.96`
- WindsurfPoolAPI: `guanxiaol/WindsurfPoolAPI`, tag `v2.0.7`
- Kiro main reference: `hank9999/kiro.rs`, observed tag `v2026.3.1`
- Kiro backup reference: `jwadow/kiro-gateway`, observed tag `v2.3`

2026-05-19 latest adapter-fusion baseline:

- Private fork includes official upstream through `8584b8f7`.
- Windsurf import responses are normalized into per-account `items[]`.
- Kiro import bridge is deployed in code and expects an internal-only
  `kiro-rs` service.

## 执行顺序

1. 读 `ROADMAP_CHECKLIST.md`，确认当前阶段。
2. 读 `INTERFACE_RESEARCH_PLAYBOOK.md`，按表格完成接口调研。
3. Windsurf 先按 `WINDSURF_INTEGRATION_SPEC.md` 跑内网上游。
4. Kiro 再按 `KIRO_INTEGRATION_SPEC.md` 跑内网上游。
5. 服务器操作按 `SERVER_DEPLOYMENT_RUNBOOK.md`。
6. 每次上线按 `TEST_ACCEPTANCE_MATRIX.md` 验收。
7. 内网上游稳定后，再回到路线图里的原生接入阶段。

## 记录规则

- 不把 token、refresh token、API key、服务器密码写入仓库。
- 只写占位符，例如 `<windsurf-internal-api-key>`。
- 每次协议变化后，在相关文档里新增一条“调研记录”，不要直接覆盖旧结论。
- 每次服务器变更前先备份，并记录回滚点。
- 每次代码改动都提交到 Git，再部署。

## 2026-05-18 新增文档

- `WINDSURF_STAGE_A_IMPORT_ENDPOINT.md`
  - 已落地的 Sub2API 管理员 Windsurf 批量导入接口、环境变量、调用示例、测试记录和下一步。

## 2026-05-19 新增文档

- `KIRO_STAGE_B_IMPORT_ENDPOINT.md`
  - 已落地的 Sub2API 管理员 Kiro 凭据导入接口、内网 adapter 转发协议、脱敏策略和部署要求。
