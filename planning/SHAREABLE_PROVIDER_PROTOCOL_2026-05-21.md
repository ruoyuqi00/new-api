# Kiro / Windsurf 接入协议简版 - 2026-05-21

这份是可分享版，只描述协议路线和关键结论，不包含任何账号、token、密码、cookie、API key 或服务器信息。请只用于自己的账号和授权环境，不要绕过账号权益、地区限制或额度限制。

## 一句话结论

旧的 Kiro IDE / CodeWhisperer / Amazon Q 风格接口目前经常只能拿到少量 open models，Claude / Opus 会被上游拒绝；我们实测能跑 Claude / Opus 的 Kiro 路线是 Kiro Web Portal RPC。

Windsurf 短期继续用 `dwgx/WindsurfAPI v2.0.96` 做内网适配器，它本质上通过 Windsurf Language Server 和 Windsurf 云端对话，账号导入后再由 Sub2API 统一对外发 key。

## 对外拓扑

```text
外部客户端
  -> Sub2API 公网 HTTPS
  -> Sub2API group / channel / upstream account
  -> 内网 provider adapter
  -> Kiro Web Portal 或 Windsurf Cloud
```

建议只公开 Sub2API，不公开 Kiro/Windsurf adapter 端口。

## Kiro 协议路线

### 不推荐作为 Claude / Opus 主路线

很多开源项目使用的是旧 Kiro IDE / CodeWhisperer / Amazon Q 路线：

```text
refresh token
  -> prod.<region>.auth.desktop.kiro.dev/refreshToken
  -> q.<region>.amazonaws.com/generateAssistantResponse
```

这个路线可以刷新 token，也能请求部分 open models，例如 `qwen3-coder-next`、`deepseek-3.2`、`glm-5`、`minimax-m2.x`。但在我们的服务器实测中，Claude / Opus 会返回 `INVALID_MODEL_ID`，即使账号本身在官方 IDE 里能用。

### 当前可用路线：Kiro Web Portal RPC

Web Portal 路线大致是：

```text
官方 Web/Portal session
  -> 获取 access token / IdP / CSRF / UserId / VisitorId 等会话材料
  -> GetUserInfo
  -> GetUserUsageAndLimits
  -> CreateSpace
  -> StreamSendMessage
  -> 解析 eventstream + CBOR 响应
```

关键点：

- RPC base 类似 `https://app.kiro.dev/service/KiroWebPortalService/operation/<OperationName>`。
- 请求协议是 Smithy RPC v2 CBOR：
  - `content-type: application/cbor`
  - `accept: application/cbor`
  - `smithy-protocol: rpc-v2-cbor`
- 响应流一般是 Amazon eventstream，payload 里再包 CBOR。
- `CreateSpace` 返回 `spaceId`。
- `StreamSendMessage` 里必须带：
  - `spaceId`
  - `sessionId`
  - `sessionId = spaceId` 是关键坑位；不带或不一致容易 401/失败。
- Web Portal session 需要保留 IdP、CSRF token、UserId cookie、visitor id 等上下文，不只是一个裸 access token。

我们目前验证过的 Kiro Web 模型包括：

- `claude-sonnet-4.6`
- `claude-opus-4.7`
- streaming `claude-sonnet-4.6`

Opus 在高峰期可能返回上游繁忙或容量限制，这属于 Kiro 上游状态，不一定是 adapter 鉴权失败。

### 建议封装方式

把 Kiro Web Portal 封成一个内网 OpenAI-compatible adapter：

```text
GET  /v1/models
POST /v1/chat/completions
POST /v1/messages
```

然后在 Sub2API 里创建一个普通 OpenAI-compatible upstream account：

```text
base_url = http://kiro-web-adapter:<port>
type     = apikey / internal key
models   = claude-sonnet-4.6, claude-opus-4.7, ...
```

Sub2API 对外发自己的 API key，外部用户不直接接触 Kiro session。

## Windsurf 协议路线

短期推荐参考：

- `https://github.com/dwgx/WindsurfAPI`
- 当前观测版本：`v2.0.96`

WindsurfAPI 的思路：

```text
Windsurf account token / api key / email login
  -> WindsurfAPI /auth/login
  -> 内部账号池
  -> Language Server gRPC
  -> Windsurf Cloud
  -> OpenAI / Anthropic compatible response
```

常见导入形式：

```http
POST /auth/login
Authorization: Bearer <adapter-admin-key>
Content-Type: application/json

{"token":"<windsurf-auth-token>"}
```

或批量：

```json
{
  "accounts": [
    {"token": "<token-1>"},
    {"api_key": "<codeium-api-key>"},
    {"email": "<email>", "password": "<password>"}
  ]
}
```

我们在 Sub2API 里做的短期接法是：

```text
Sub2API 管理后台导入
  -> Sub2API 后端只转发到内网 WindsurfAPI
  -> Sub2API 不保存原始 Windsurf token/password
  -> 返回结果递归脱敏
```

Windsurf 的模型能力主要取决于：

- 账号权益：Free / Trial / Pro；
- WindsurfAPI 内部账号池里每个账号的可用模型；
- Language Server 二进制是否够新；
- Windsurf 云端动态模型目录。

如果看不到新模型，优先检查 Windsurf Language Server，而不是只改 Sub2API。

## Sub2API 分组建议

推荐把外部能力都收口到 Sub2API：

- 只公开 `https://your-domain/v1/chat/completions` 或 `/v1/messages`；
- Kiro / Windsurf adapter 都在 Docker 内网；
- Sub2API group 按模型或用途区分：
  - `claude-sonnet`
  - `claude-opus`
  - `open-models`
  - `windsurf-pro`
  - `kiro-web`
- API key 再绑定可访问分组。

这样外部用户只拿 Sub2API key，不需要知道底层是 Windsurf 还是 Kiro。

## 避坑清单

- 不要把裸 token、refresh token、cookie 写进 Git、日志或聊天记录。
- 不要公开 provider adapter 端口。
- Kiro 旧 IDE 路线能刷新 token，不等于能请求 Claude / Opus。
- Kiro Web Portal 路线不是只换 header，关键是 Web session 上下文 + `CreateSpace` / `StreamSendMessage`。
- Windsurf 新模型缺失时，重点看 Language Server 版本和云端 model catalog。
- Sub2API merge 官方更新时，要保留私有 provider adapter 文件，不要被上游“没有这些文件”误删。

## 参考项目

- Sub2API: https://github.com/Wei-Shaw/sub2api
- WindsurfAPI: https://github.com/dwgx/WindsurfAPI
- WindsurfPoolAPI: https://github.com/guanxiaol/WindsurfPoolAPI
- kiro.rs: https://github.com/hank9999/kiro.rs
- kiro-gateway: https://github.com/Jwadow/kiro-gateway
- opencode-kiro-auth: https://github.com/tickernelz/opencode-kiro-auth
- pi-kiro: https://github.com/hongyilyu/pi-kiro
