# 上游协议巡检 2026-05-25

## 结论

本轮有需要跟进的小改动，但不是模型协议大改。

- Kiro 方向：`Kiro-account-manager` 和 `Kiro-Go` 都有更新，主要集中在上下文裁剪、token 计量、账号失效 failover、stream tool 收尾。
- Windsurf 方向：`WindsurfAPI` 和 `windsurf-tools` 有更新，主要集中在 Cascade 复用、批量导入解析、API server URL 保存、auth1/session pool、本地登录/MITM 匹配。
- 我们当前自研 `kiro-web-adapter` 是文本代理，不做结构化 tool/history 协议转译，所以 Kiro-Go 的 stream tool 细节暂不直接移植。
- 需要跟进的是 KAM 的 `tokenBufferReserve` 行为：默认关闭静默裁剪，显式启用后再裁剪。已同步到本项目。

## 本轮上游状态

| 项目 | 上次审计基线 | 当前远端 | 变化判断 |
| --- | --- | --- | --- |
| [chaogei/Kiro-account-manager](https://github.com/chaogei/Kiro-account-manager) | `63ac139` | `dea402c` / `v1.6.9` | 有协议相关变更 |
| [Quorinex/Kiro-Go](https://github.com/Quorinex/Kiro-Go) | `bef9418` | `8c1f751` | 有协议相关变更 |
| [Jwadow/kiro-gateway](https://github.com/Jwadow/kiro-gateway) | `a5292ca` | `a5292ca` | 无新增 |
| [hank9999/kiro.rs](https://github.com/hank9999/kiro.rs) | `f1bbe9f` | `f1bbe9f` | 无新增 |
| [dwgx/WindsurfAPI](https://github.com/dwgx/WindsurfAPI) | `c028576` | `41a36b9` / `v2.0.97` | 有可参考更新 |
| [tranhailong012/windsurf-tools](https://github.com/tranhailong012/windsurf-tools) | `dd96d45` | `a96437a` | 有可参考更新，且远端 main 强推过 |
| [jlcodes99/cockpit-tools](https://github.com/jlcodes99/cockpit-tools) | `2b14843` | `a62cd33` / `v0.24.8` | 有工具层更新，暂不直接影响服务端协议 |

## Kiro 方向

### Kiro-account-manager

新增/变更：

- `v1.6.8` 新增 `tokenCounter.ts`，优先使用 Kiro `tokenUsage`，其次 `contextUsageEvent` 百分比反推，再用 tokenizer/字符估算兜底。
- `tokenBufferReserve` 从强制启用 50K 改成“开关默认关闭”，启用后默认 20K。
- 关闭时不再静默裁剪 history，超出上下文由 Kiro 后端返回 `CONTENT_LENGTH_EXCEEDS_THRESHOLD`。
- `v1.6.9` 修复多账号模式中过期 token 账号永远不刷新：有 `refreshToken` 的过期账号仍进入刷新流程。

对我们的影响：

- 我们没有完整 Kiro history 结构，不能直接移植 tokenCounter/history 成对裁剪。
- 但“默认不静默裁剪”适合我们的公网服务，已同步：新增 `KIRO_ENABLE_TOKEN_BUFFER_RESERVE`，默认关闭；开启后 `KIRO_TOKEN_BUFFER_RESERVE` 默认 20K。

### Kiro-Go

新增/变更：

- 新增账号失败分类：quota、overage、suspension、profile unavailable、auth failed。
- 请求失败时会排除当前账号并重试下一个账号，最多 3 次。
- `CallKiroAPI` 会按当前账号刷新 `profileArn`，避免 payload 里残留旧账号 ARN。
- stream tool 事件 EOF 时会补完未结束的 toolUse；兼容不同字段名和缺失 toolUseId。

对我们的影响：

- 我们 `kiro-web-adapter` 目前走 Web Portal `StreamSendMessage`，没有结构化 toolUse 转译，stream tool 修复暂不适用。
- 我们已具备 refresh auth failure 禁用凭证、portal 鉴权失败强制 refresh 一次的机制。
- 后续如果 adapter 升级为结构化 history/tool 代理，应优先移植 Kiro-Go 的 failover 和 toolUse EOF 收尾。

## Windsurf 方向

### WindsurfAPI

新增/变更：

- `v2.0.97` 增加 Cascade reuse 优化：`CASCADE_REUSE_BY_CALLER` 和 `CASCADE_POOL_MAX`。
- 批量导入解析增强：支持邮箱密码多分隔符、token、API key、JSON 上传、代理绑定。
- `addAccountByKey` 支持保存 `apiServerUrl`，避免只导入 key 时丢失服务端地址。
- 新增 Astraflow provider。

对我们的影响：

- 目前服务器上 Windsurf 部分跑的是上游容器，不在本仓库维护源码。
- 如果后续要做自己的 Windsurf adapter，批量导入解析和 `apiServerUrl` 持久化需要吸收。
- Cascade reuse 属于 WindsurfAPI 内部会话复用优化，暂不需要改 Sub2API。

### windsurf-tools

新增/变更：

- 支持 `auth1_` session pool key 获取 Windsurf SessionKey。
- 支持本地 Windsurf 登录/切号链路。
- 修复当前 MITM active account 匹配。

对我们的影响：

- 这主要影响桌面导入工具和 MITM 账号提取。
- 我们服务端导入方式暂不依赖它，所以本轮只记录，不改服务端。

## 本项目已同步

文件：

```text
adapters/kiro-web/kiro_web_adapter.py
adapters/kiro-web/README.md
docs/UPSTREAM_PROTOCOL_AUDIT_2026-05-25.md
```

变更：

- 新增 `KIRO_ENABLE_TOKEN_BUFFER_RESERVE`。
- `KIRO_TOKEN_BUFFER_RESERVE` 默认从 50K 调整为 20K。
- 未显式启用时不再静默裁剪 prompt。
- README 记录新的环境变量含义。

## 后续观察项

1. 如果 Kiro Web Portal 也开始返回结构化 tool events，再考虑移植 Kiro-Go 的 toolUse EOF 收尾逻辑。
2. 如果公网出现 `CONTENT_LENGTH_EXCEEDS_THRESHOLD`，可以在服务器 `kiro-web-adapter` 环境中显式设置：

```text
KIRO_ENABLE_TOKEN_BUFFER_RESERVE=1
KIRO_TOKEN_BUFFER_RESERVE=20000
```

3. 如果要把 Windsurf 导入能力做进自研后台，优先参考 WindsurfAPI 的 `src/dashboard/import-parser.js` 和 windsurf-tools 的 auth1/session pool 逻辑。
