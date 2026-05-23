# Kiro 上游协议审计记录 2026-05-23

## 结论

本轮重点不是给 `claude-opus-4-6` 强行补别名，而是确认真实可用模型名和吸收上游协议韧性修复。

- 公网 Sub2API 已确认 `claude-opus-4.6` 可调用，`/v1/messages` 返回 200。
- `claude-opus-4-6` 不是当前建议对外使用的规范模型名；外部客户端应配置 `claude-opus-4.6`。
- Kiro Web adapter 直连 `claude-opus-4.6` 的 Anthropic 兼容和 OpenAI 兼容入口均返回 200。
- 上游参考里有真实协议相关更新，需要吸收的是 token 失效处理、账号跳过、system/history 结构、tool schema 清洗、上下文裁剪。

## 本次实测

测试日期：2026-05-23

公网入口：

```text
https://api.vyywcw.cn/v1/messages
https://api.vyywcw.cn/v1/chat/completions
```

结果：

| 路径 | 模型 | 结果 |
| --- | --- | --- |
| Sub2API `/v1/messages` | `claude-opus-4.6` | 200，返回 `OPUS46_OK` |
| Sub2API `/v1/messages` | `claude-opus-4-6` | 503，调度层未选到可用账号 |
| Kiro Web adapter `/v1/messages` | `claude-opus-4.6` | 200，返回 `OPUS46_OK` |
| Kiro Web adapter `/v1/chat/completions` | `claude-opus-4-6` | 200，adapter 内部能映射到 `claude-opus-4.6` |

操作原则：

- 对外文档和客户端配置使用 `claude-opus-4.6`。
- 不把 `claude-opus-4-6` 当作必须支持的公开模型名。
- 如后续确实要兼容某些客户端的连字符写法，应在 Sub2API 的账号 `model_mapping` 和分组路由里显式维护，不能混在“官方可用模型”判断里。

## 上游参考状态

| 项目 | 本地旧版本 | 最新远端 | 是否有协议相关更新 | 结论 |
| --- | --- | --- | --- | --- |
| [chaogei/Kiro-account-manager](https://github.com/chaogei/Kiro-account-manager) | `7ad57fd` | `63ac139` | 有 | 新增按模型 context window 的 token 裁剪，避免长上下文被 Kiro 拒绝 |
| [Quorinex/Kiro-Go](https://github.com/Quorinex/Kiro-Go) | `68110f3` | `bef9418` | 有 | 新增账号失效跳过、刷新失败禁用、system prompt 放入 history、tool schema 清洗 |
| [Jwadow/kiro-gateway](https://github.com/Jwadow/kiro-gateway) | `a5292ca` | `a5292ca` | 无新增 | 继续作为参考，不需要本轮跟进 |
| [hank9999/kiro.rs](https://github.com/hank9999/kiro.rs) | `f1bbe9f` | 未发现 main 更新 | 无新增 | 继续保留历史参考，但不作为当前首选协议源 |

## 已吸收到本项目的内容

文件：

```text
adapters/kiro-web/kiro_web_adapter.py
adapters/kiro-web/README.md
```

已更新：

- 多账号凭证读取时按 `priority` 排序，并跳过 `disabled=true` 的账号。
- refresh token 遇到硬认证失败时，将该 credential 标记为 disabled，写入 `disabled_reason` 和 `disabled_at`，后续请求自动跳过。
- portal session 鉴权失败时，会强制 refresh 一次再重试，减少“token 看似未过期但 Kiro 已拒绝”的情况。
- prompt 发送到 `StreamSendMessage` 前，按 200K context、默认 50K reserve 做保守 token 估算裁剪，降低长会话触发 upstream content limit 的概率。
- README 记录 `claude-opus-4.6` 为当前规范可用模型。

## 暂未直接吸收的内容

Kiro-Go 的以下修复目前主要服务于“直接 Kiro API / IDE 协议”路径，而我们的 `kiro-web-adapter` 现在走 Web Portal `StreamSendMessage`，仍是文本 prompt 第一阶段：

- system prompt 作为 Kiro history priming pair；
- tool schema 递归清洗；
- toolUse/toolResult 成对裁剪；
- native history 结构化重放；
- `additionalModelRequestFields.thinking` 精确映射。

后续如果要把 Kiro Web adapter 从“文本 prompt 代理”升级成“结构化 Kiro conversation 代理”，以上内容要优先移植。

## 后续升级建议

1. 对外模型名统一以 `/v1/models` 返回的 canonical id 为准，例如 `claude-opus-4.6`。
2. Sub2API 后台分组不要把 alias 当成官方模型；需要 alias 时单独维护映射表。
3. 每次升级前先拉取以下参考项目：
   - `chaogei/Kiro-account-manager`
   - `Quorinex/Kiro-Go`
   - `Jwadow/kiro-gateway`
4. 重点搜索关键词：
   - `history`
   - `additionalModelRequestFields`
   - `thinking`
   - `tool schema`
   - `required`
   - `additionalProperties`
   - `refresh token`
   - `temporarily_suspended`
   - `maxInputTokens`
5. 协议更新后先测内网 adapter，再测公网 Sub2API，避免把“Sub2API 调度问题”误判为“Kiro 协议问题”。
