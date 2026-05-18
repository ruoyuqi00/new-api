# Windsurf Stage A Import Endpoint

记录日期：2026-05-18

## 当前实现状态

本阶段已经在 Sub2API 后端增加管理员接口，用来把 Windsurf 账号凭据转发到内网 `dwgx/WindsurfAPI`，实现“Sub2API 是唯一公网入口，WindsurfAPI 只在内网运行”的最短期接入。

已实现代码：

- `backend/internal/handler/admin/account_windsurf_import.go`
- `backend/internal/handler/admin/account_windsurf_import_test.go`
- `backend/internal/server/routes/admin.go`

新增接口：

```text
POST /api/v1/admin/accounts/import/windsurf
```

该接口走现有 admin 鉴权路由组，生产环境调用时仍需要 Sub2API 管理员登录态/管理员 API 鉴权，并建议带 `Idempotency-Key`。

## 参考项目

主参考：

- GitHub: `https://github.com/dwgx/WindsurfAPI`
- 本地路径：`D:\wflogin\_github_research\WindsurfAPI`
- 当前观测提交：`c028576 release: 2.0.96`
- 当前观测 tag：`v2.0.96`
- 关键文件：`src/server.js`

确认到的 WindsurfAPI 导入接口：

```text
POST /auth/login
Authorization: Bearer <WINDSURF_API_KEY>
x-api-key: <WINDSURF_API_KEY>
Content-Type: application/json
```

支持请求形态：

```json
{"token":"<windsurf-token>"}
```

```json
{"api_key":"<codeium-api-key>"}
```

```json
{"accounts":[{"token":"<token-1>"},{"token":"<token-2>"}]}
```

Sub2API 新接口采用批量 `accounts` 形态转发，便于一次导入多个账号，也方便后续加逐条结果解析。

## Sub2API 新接口请求

支持单 token：

```json
{
  "token": "<windsurf-token>"
}
```

支持 token 数组：

```json
{
  "tokens": ["<token-1>", "<token-2>"]
}
```

支持一行一个 token 的原始文本，也会兼容逗号和分号分隔：

```json
{
  "raw": "<token-1>\n<token-2>"
}
```

支持完整 accounts 结构：

```json
{
  "accounts": [
    {
      "token": "<token-1>",
      "label": "account-a",
      "proxy": "http://127.0.0.1:9000"
    },
    {
      "api_key": "<codeium-api-key>",
      "label": "direct-key-account"
    }
  ]
}
```

规则：

- 单个 account 只能提供 `token` 或 `api_key` 其中一种。
- 请求内重复凭据会去重，返回 `duplicate_count`。
- 空输入会返回 400。
- 不会在 Sub2API 数据库保存原始 Windsurf token。
- 幂等请求指纹使用 token/api_key 的 SHA-256 哈希，不使用原文。
- 上游响应会递归脱敏 `token`、`api_key`、`apiKey`、`authorization`、`password`、`secret` 等字段后再返回。

## Sub2API 新接口响应

成功示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 2,
    "forwarded": 2,
    "duplicate_count": 0,
    "upstream_status": 200,
    "upstream": {
      "results": [
        {
          "id": "1",
          "email": "a@example.com",
          "status": "active"
        }
      ],
      "active": 1
    }
  }
}
```

配置缺失：

```json
{
  "code": 503,
  "message": "windsurf adapter internal base URL is not configured",
  "reason": "WINDSURF_ADAPTER_NOT_CONFIGURED"
}
```

上游失败：

```json
{
  "code": 502,
  "message": "windsurf adapter returned status 401",
  "reason": "WINDSURF_IMPORT_UPSTREAM_FAILED"
}
```

## 运行配置

Sub2API 容器需要配置下面两个环境变量：

```env
WINDSURF_ADAPTER_INTERNAL_BASE_URL=http://windsurf-api:3003
WINDSURF_ADAPTER_INTERNAL_API_KEY=<WINDSURF_API_KEY>
WINDSURF_ADAPTER_TIMEOUT_SECONDS=30
```

兼容别名：

```env
PROVIDER_ADAPTERS_WINDSURF_INTERNAL_BASE_URL=http://windsurf-api:3003
PROVIDER_ADAPTERS_WINDSURF_INTERNAL_API_KEY=<WINDSURF_API_KEY>
PROVIDER_ADAPTERS_WINDSURF_TIMEOUT_SECONDS=30
```

WindsurfAPI 自己仍需要配置：

```env
API_KEY=<WINDSURF_API_KEY>
DASHBOARD_PASSWORD=<strong-dashboard-password>
PORT=3003
DATA_DIR=/data
```

安全要求：

- WindsurfAPI 不对公网暴露。
- Caddy 只反代 Sub2API。
- `WINDSURF_API_KEY` 不提交到 Git。
- 批量 token 临时文件用 `umask 077` 创建，用完删除。

## 管理员调用示例

登录 Sub2API 管理后台后，可由前端接入该接口。临时 curl 调试时需要使用现有 admin 鉴权方式，并带幂等键：

```bash
curl -sS https://api.vyywcw.cn/api/v1/admin/accounts/import/windsurf \
  -H 'content-type: application/json' \
  -H 'idempotency-key: windsurf-import-20260518-001' \
  -H 'authorization: Bearer <sub2api-admin-token>' \
  -d '{"tokens":["<token-1>","<token-2>"]}'
```

如果从服务器本机导入，推荐先写临时 JSON，然后调用 Sub2API 内网地址，避免 shell history 泄漏：

```bash
umask 077
cat >/root/windsurf_import.json <<'EOF'
{"raw":"<token-1>\n<token-2>"}
EOF

curl -sS http://127.0.0.1:8080/api/v1/admin/accounts/import/windsurf \
  -H 'content-type: application/json' \
  -H 'idempotency-key: windsurf-import-20260518-001' \
  -H 'authorization: Bearer <sub2api-admin-token>' \
  --data-binary @/root/windsurf_import.json

shred -u /root/windsurf_import.json 2>/dev/null || rm -f /root/windsurf_import.json
```

## 已跑测试

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run Windsurf
go test ./internal/handler/admin
go test ./internal/server
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin
?  github.com/Wei-Shaw/sub2api/internal/server [no test files]
```

覆盖点：

- `token`、`tokens`、`raw`、`accounts` 解析。
- 同请求去重。
- 拒绝同一账号同时传 `token` 和 `api_key`。
- 幂等 payload 不包含 raw token/api_key。
- 转发路径为 `/auth/login`。
- 同时发送 `Authorization: Bearer` 和 `x-api-key`。
- 上游响应递归脱敏。
- 上游非 2xx 以安全错误返回。
- 适配器配置缺失返回 503，且不回显 token。

## 下一步

1. 给 Sub2API 管理后台补一个 Windsurf 批量导入 UI。
2. 在服务器 compose 里加 `windsurf-api` 内网服务。
3. 把 Sub2API 从官方镜像切到本 fork 构建镜像。
4. 配置 `WINDSURF_ADAPTER_INTERNAL_BASE_URL` 和 `WINDSURF_ADAPTER_INTERNAL_API_KEY`。
5. 用一个测试 token 跑完整链路：导入到 WindsurfAPI，Sub2API 创建/测试内网上游账号，公网只访问 Sub2API。
6. 开始 Kiro Stage A：先复用同样的“内网代理 + Sub2API 管理导入入口”模式，不急着把 Kiro token 生命周期全部写进 Sub2API。
