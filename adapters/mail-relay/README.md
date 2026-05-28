# Sub2API Mail Relay

这个适配器把 Sub2API 的 SMTP 发信请求转成 HTTPS API 请求，用来绕开服务器机房屏蔽 25/465/587/2525 等 SMTP 出站端口的问题。

推荐拓扑：

```text
Sub2API SMTP client
  -> mail-relay:1025
  -> HTTPS 443
  -> Cloudflare Email Service 或 Resend
```

`mail-relay` 只应该放在 Docker 内网，不要映射到公网。

## 环境变量

通用：

```env
MAIL_RELAY_PROVIDER=resend
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
MAIL_RELAY_PORT=1025
MAIL_RELAY_HEALTH_PORT=8080
MAIL_RELAY_TIMEOUT_SECONDS=20
```

Resend：

```env
RESEND_API_KEY=...
```

Cloudflare Email Service：

```env
MAIL_RELAY_PROVIDER=cloudflare
CLOUDFLARE_ACCOUNT_ID=...
CLOUDFLARE_API_TOKEN=...
```

Cloudflare 官方 REST API 使用：

```text
POST https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send
Authorization: Bearer <API_TOKEN>
```

Resend 官方 REST API 使用：

```text
POST https://api.resend.com/emails
Authorization: Bearer <API_KEY>
```

## Sub2API 后台 SMTP 配置

切换到 relay 后在后台邮件配置里填：

```text
SMTP Host: mail-relay
SMTP Port: 1025
SMTP Username: 留空
SMTP Password: 留空
From Email: no-reply@vyywcw.cn
From Name: vyywcw
Use TLS: false
```

如果暂时没有第三方 API Key，不要把 Sub2API 正式切到 relay；服务可以先部署，发信会在 provider 缺少 key 时返回临时失败。
