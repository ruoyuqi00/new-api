# Kiro Integration Spec

记录日期：2026-05-16

## Decision

短期使用 Kiro proxy 作为内部 Anthropic-compatible 上游。主参考为 `hank9999/kiro.rs`，本地已有 `D:\wflogin\kiro.rs-master`。

原因：

- 已经提供 Anthropic `/v1/messages` 兼容接口。
- 已经处理 Kiro refresh token、region、profileArn、machine id 等细节。
- 已经支持 `/cc/v1/messages`，适合 Claude Code 类客户端。
- Kiro 私有认证细节变化快，先放在内部 proxy 层风险更低。

## Runtime shape

```text
sub2api container
  -> http://kiro-rs:8990
  -> Kiro proxy
  -> Kiro/AWS upstream
```

公网：

- 只暴露 Sub2API。
- 不暴露 Kiro proxy admin API。

## Credential shapes

Social OAuth-like credential:

```json
{
  "accessToken": "<optional-access-token>",
  "refreshToken": "<refresh-token>",
  "expiresAt": "2026-12-31T00:00:00Z",
  "authMethod": "social",
  "profileArn": "<optional-profile-arn>",
  "region": "us-east-1",
  "authRegion": "us-east-1",
  "apiRegion": "us-east-1"
}
```

IdC credential:

```json
{
  "accessToken": "<optional-access-token>",
  "refreshToken": "<refresh-token>",
  "expiresAt": "2026-12-31T00:00:00Z",
  "authMethod": "idc",
  "clientId": "<client-id>",
  "clientSecret": "<client-secret>",
  "profileArn": "<optional-profile-arn>",
  "region": "us-east-1",
  "authRegion": "us-east-1",
  "apiRegion": "us-east-1"
}
```

API key credential:

```json
{
  "authMethod": "api_key",
  "kiroApiKey": "<kiro-api-key>",
  "region": "us-east-1",
  "apiRegion": "us-east-1"
}
```

Observed region priority:

- Auth refresh region:
  - credential `authRegion`
  - credential `region`
  - config `authRegion`
  - config `region`
- API request region:
  - credential `apiRegion`
  - config `apiRegion`
  - config `region`

## Internal proxy endpoints

Need to verify exact auth settings on deployed proxy.

Expected compatible endpoints:

- `GET /v1/models`
- `POST /v1/messages`
- `POST /v1/messages/count_tokens`
- `POST /cc/v1/messages`
- `POST /cc/v1/messages/count_tokens`

Direct test:

```bash
curl -sS http://127.0.0.1:8990/v1/messages \
  -H 'content-type: application/json' \
  -H 'x-api-key: <kiro-proxy-api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hi"}],
    "stream": true
  }'
```

Claude Code style buffered test:

```bash
curl -sS http://127.0.0.1:8990/cc/v1/messages \
  -H 'content-type: application/json' \
  -H 'x-api-key: <kiro-proxy-api-key>' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "hi"}],
    "stream": true
  }'
```

## Sub2API account shape

Preferred:

```json
{
  "platform": "anthropic",
  "type": "apikey",
  "name": "kiro-internal-anthropic",
  "credentials": {
    "api_key": "<kiro-proxy-api-key>",
    "base_url": "http://kiro-rs:8990"
  },
  "extra": {
    "anthropic_passthrough": true
  },
  "model_mapping": {
    "claude-sonnet-4-6": "claude-sonnet-4-6"
  }
}
```

Need to verify:

- [ ] Does Kiro proxy require `Authorization: Bearer` or `x-api-key`?
- [ ] Does Sub2API preserve `anthropic-version`?
- [ ] Does Sub2API preserve tool_use/tool_result?
- [ ] Does Sub2API need to route Claude Code clients to `/cc/v1/messages` instead of `/v1/messages`?
- [ ] Does Kiro proxy support `count_tokens` well enough for Sub2API usage accounting?

## Native Sub2API Kiro design

Stage A: proxy-backed account.

- Sub2API points to Kiro proxy via Anthropic API key account.
- Kiro proxy owns credential refresh and machine id behavior.
- Sub2API only manages routing, group, pricing, usage.

Stage B: native credential import with proxy execution.

- Sub2API UI accepts Kiro credential JSON.
- Sub2API backend validates shape and redacts secrets.
- Sub2API calls internal Kiro admin API to add credentials.
- Sub2API stores only metadata and proxy account linkage.

Stage C: direct native Kiro provider.

- Sub2API stores Kiro credentials.
- Sub2API implements refresh logic for social/idc/api_key.
- Sub2API implements Kiro endpoint requests directly.
- Sub2API owns invalid_grant disable logic.
- Sub2API owns model mapping and quota states.

Stage C should wait until Stage A/B are stable.

## Native credential schema draft

```json
{
  "platform": "kiro",
  "type": "oauth",
  "credentials": {
    "auth_method": "social",
    "access_token": "<optional>",
    "refresh_token": "<required>",
    "expires_at": "2026-12-31T00:00:00Z",
    "profile_arn": "<optional>",
    "region": "us-east-1",
    "auth_region": "us-east-1",
    "api_region": "us-east-1"
  },
  "extra": {
    "provider_mode": "kiro_proxy"
  }
}
```

```json
{
  "platform": "kiro",
  "type": "apikey",
  "credentials": {
    "auth_method": "api_key",
    "kiro_api_key": "<required>",
    "region": "us-east-1",
    "api_region": "us-east-1"
  }
}
```

## Token refresh behavior to replicate if native

From current `kiro.rs` observations:

- `social` refresh endpoint shape depends on `prod.<region>.auth.desktop.kiro.dev/refreshToken`.
- `idc` refresh endpoint uses `https://oidc.<region>.amazonaws.com/token`.
- `invalid_grant` should be treated as permanent refresh token failure.
- `kiroApiKey` mode bypasses OAuth refresh.
- machine id is derived from `kiroApiKey` or `refreshToken`.

Native implementation tasks:

- [ ] Implement credential validation.
- [ ] Implement duplicate detection by SHA-256 hash.
- [ ] Implement refresh provider.
- [ ] Implement region priority.
- [ ] Implement permanent failure classification.
- [ ] Implement machine id derivation.
- [ ] Implement admin import and update endpoints.
- [ ] Implement frontend import form.
- [ ] Implement account test route.
- [ ] Implement regression tests for invalid_grant, missing clientSecret, missing refreshToken, api_key mode.

## Open questions

- Does the proxy admin API already expose exactly what Sub2API needs for import?
- Should Sub2API expose `/cc/v1/messages` publicly, or keep normal `/v1/messages` only?
- Which Kiro models should appear by default?
- How should Kiro request usage be normalized into Sub2API billing?
- Should social/idc/api_key be separate account types or a credential field?

