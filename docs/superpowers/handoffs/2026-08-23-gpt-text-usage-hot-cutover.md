# GPT 文本 usage 安全修复热切换记录

更新时间：2026-08-23（Asia/Shanghai）

## 发布内容

- 源码分支：`codex/grok-production-baseline-20260822`
- 源码提交：`b6bfa6bbb`
- GitHub 归档：`fork/codex/grok-production-baseline-20260822`
- 生产镜像：`yuapi:production-20260823-gpt-b6bfa6bbb-ui`
- 候选容器：`newapi-gpt-b6bfa6bbb`
- 候选私有端口：`127.0.0.1:13051 -> 3000/tcp`

本次只涉及 GPT 文本 usage 校验、无效 usage 清洗和文本结算边界；图片、视频、音频、品牌 UI、数据库结构、用户余额、渠道价格和 Caddyfile 内容均未修改。

## 运行基线

切换前按 Caddy 运行态解析到的稳定别名确认旧目标为：

- `newapi-candidate-20260816-d6605a79a-media-docs-rc2`
- 旧容器和旧镜像均保留，未停止、未删除，可用于立即回滚

此前部分交接文档记录的容器名与运行态别名不一致；后续以 Caddy 容器内实时配置和 Docker 网络别名为准，不以宿主机旧文件或镜像标签单独推断流量目标。

## 切换方式

- 复用生产镜像中的品牌 UI 资产，避免源码树 UI 覆盖生产品牌
- 新容器复用现有生产环境变量和 `/opt/newapi/data:/data` 挂载
- Caddy 配置未重载，稳定别名通过发布网络完成蓝绿切换
- 旧容器保持运行，回滚只需将稳定别名接回旧容器，不恢复数据库快照
- 临时环境变量导出文件已删除

## 验证结果

- 新候选健康：`running`、`healthy`、重启次数 `0`
- 旧回滚容器健康：`running`、`healthy`、重启次数 `0`
- 候选与生产主页 SHA-256：`97102537d7dba340791fa533746b7a66f6275c763e5af647f9c28c1a8b8c5b7f`
- `api.yuaiapi.com`、`global.yuaiapi.com`、`vip.yuaiapi.com` 切换后均 `10/10` 返回 HTTP 200，随后观察轮均 `5/5`
- 候选最近观察日志：502、panic/fatal、数据库错误、Redis 错误均为 `0`

## 回滚

不要停止新容器。将稳定别名从 `newapi-gpt-b6bfa6bbb` 接回旧目标
`newapi-candidate-20260816-d6605a79a-media-docs-rc2`，确认旧目标健康后，再验证三个公共域名、主页指纹和 `/api/status`。保留新旧镜像、容器、`/data` 和 Caddy 回滚副本，未经单独批准不得清理。
