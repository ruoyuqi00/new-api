# Kiro Stage B Import Endpoint

记录日期：2026-05-19

## 当前实现状态

本阶段已经在 Sub2API fork 中加入 Kiro 凭据导入桥接。设计目标和
Windsurf 一致：公网仍然只暴露 Sub2API，Kiro 私有认证和 token 刷新
继续由内网 adapter 负责。

新增接口：

```text
POST /api/v1/admin/accounts/import/kiro
```

该接口走现有 admin 鉴权，并要求调用方带 `Idempotency-Key`。后端不会
保存原始 Kiro refreshToken、Kiro API key 或 clientSecret，只把它们转发
到内网 adapter 的 admin API。

## 参考项目

主参考：

- GitHub: `https://github.com/hank9999/kiro.rs`
- 最新观测提交：`f1bbe9f`
- 本地镜像：`D:\wflogin\_github_research\kiro.rs-latest`
- 关键接口：`POST /api/admin/credentials`

备选参考：

- GitHub: `https://github.com/Jwadow/kiro-gateway`
- 最新观测提交：`a5292ca`
- 最新观测 tag 线：`v2.3`
- 本地镜像：`D:\wflogin\_github_research\kiro-gateway`

官方方向参考：

- `https://kiro.dev/docs/kiro-cli/api-keys/`

## Sub2API 请求形态

Refresh token:

```json
{
  "refresh_tokens": ["<refresh-token-1>", "<refresh-token-2>"]
}
```

Kiro API key:

```json
{
  "kiro_api_keys": ["<ksk-key-1>", "<ksk-key-2>"]
}
```

原始文本：

```json
{
  "raw": "user-a@example.com----<refresh-token-a>\nuser-b@example.com----<ksk-key-b>"
}
```

完整 accounts 数组：

```json
{
  "accounts": [
    {
      "refreshToken": "<refresh-token>",
      "authMethod": "social",
      "email": "user@example.com",
      "region": "us-east-1",
      "authRegion": "us-east-1",
      "apiRegion": "us-east-1"
    },
    {
      "refreshToken": "<refresh-token>",
      "authMethod": "idc",
      "clientId": "<client-id>",
      "clientSecret": "<client-secret>"
    },
    {
      "kiroApiKey": "<ksk-key>",
      "authMethod": "api_key"
    }
  ]
}
```

后端同时兼容 snake_case 和 kiro.rs 的 camelCase 字段。

## 转发行为

Sub2API 对每条凭据独立调用：

```text
POST http://kiro-rs:8990/api/admin/credentials
Authorization: Bearer <KIRO_ADAPTER_ADMIN_API_KEY>
x-api-key: <KIRO_ADAPTER_ADMIN_API_KEY>
Content-Type: application/json
```

转发给 `kiro.rs` 的字段使用 camelCase：

- `accessToken`
- `refreshToken`
- `profileArn`
- `expiresAt`
- `authMethod`
- `clientId`
- `clientSecret`
- `priority`
- `region`
- `authRegion`
- `apiRegion`
- `machineId`
- `email`
- `proxyUrl`
- `proxyUsername`
- `proxyPassword`
- `kiroApiKey`
- `endpoint`

## 响应形态

```json
{
  "total": 2,
  "forwarded": 2,
  "succeeded": 1,
  "failed": 1,
  "duplicate_count": 0,
  "items": [
    {
      "index": 0,
      "kind": "refresh_token",
      "upstream_status": 200,
      "success": true,
      "upstream": {
        "success": true,
        "credentialId": 1
      }
    },
    {
      "index": 1,
      "kind": "api_key",
      "upstream_status": 400,
      "success": false,
      "error": "kiro adapter returned status 400",
      "upstream": {
        "error": {
          "message": "credential already exists"
        }
      }
    }
  ]
}
```

即使某些条目失败，接口也会返回 200 和逐条结果，便于管理员看清楚哪一
条失败。只有请求无法解析、adapter 未配置、或请求构造失败时返回错误。

## 脱敏规则

以下字段会在上游响应和非 JSON 错误文本中脱敏：

- `refreshToken` / `refresh_token`
- `accessToken` / `access_token`
- `kiroApiKey` / `kiro_api_key`
- `clientSecret` / `client_secret`
- `proxyPassword` / `proxy_password`
- `authorization`
- `x-api-key`
- 任意包含 `token`、`secret`、`password`、`apikey`、`api_key` 的字段

幂等 payload 只保存 SHA-256 hash，不保存原文。

## 环境变量

Sub2API:

```env
KIRO_ADAPTER_INTERNAL_BASE_URL=http://kiro-rs:8990
KIRO_ADAPTER_ADMIN_API_KEY=<kiro-rs adminApiKey>
KIRO_ADAPTER_TIMEOUT_SECONDS=30
```

兼容别名：

```env
PROVIDER_ADAPTERS_KIRO_INTERNAL_BASE_URL=http://kiro-rs:8990
PROVIDER_ADAPTERS_KIRO_ADMIN_API_KEY=<kiro-rs adminApiKey>
PROVIDER_ADAPTERS_KIRO_TIMEOUT_SECONDS=30
```

`kiro.rs` config:

```json
{
  "host": "0.0.0.0",
  "port": 8990,
  "apiKey": "<internal-model-request-key>",
  "adminApiKey": "<internal-admin-key>",
  "region": "us-east-1",
  "tlsBackend": "rustls",
  "defaultEndpoint": "ide"
}
```

## 管理后台

账号管理的更多操作菜单新增：

```text
导入 Kiro
```

UI 支持三种模式：

- Refresh Token：一行一个 refreshToken，也支持 `email----refreshToken`。
- API Key：一行一个 `ksk_` Kiro API key，也支持 `email----ksk_xxx`。
- JSON：完整对象或 accounts 数组。

UI 会给每次导入生成 `kiro-import-<timestamp>-<random>` 幂等键。

## 已验证

```powershell
go test ./internal/handler/admin -run Kiro
go test ./internal/handler/admin
go test ./internal/server

$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm exec vitest run src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
corepack pnpm exec vite build
```

## 后续

- 服务器部署内网 `kiro-rs` 服务，挂载 `/opt/sub2api/kiro-rs/config`。
- 给 Sub2API 容器注入 `KIRO_ADAPTER_*` 环境变量。
- 导入一条真实 Kiro 凭据后先直连 `kiro-rs /v1/messages` 冒烟。
- 直连通过后再在 Sub2API 中创建 `kiro-internal-anthropic` 上游账号。
- 评估是否需要把 Claude Code 客户端映射到 `/cc/v1/messages`。
