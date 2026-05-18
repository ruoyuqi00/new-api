# Operations Log

This log records local upstream checks, server maintenance, and deployment notes
for the private fork. Do not store secrets here.

## 2026-05-18 Upstream and Server Maintenance

Operator context:

- Local workspace: `D:\wflogin\sub2api-private`
- Production server path: `/opt/sub2api`
- Public URL checked: `https://api.vyywcw.cn/`

### Upstream checks

Sub2API official upstream:

- Remote: `https://github.com/Wei-Shaw/sub2api.git`
- Previous observed base in private fork: `6e66edb chore: update sponsors`
- New upstream head: `f5bd25be Merge pull request #2530 from lyen1688/fix/openai-responses-sse-terminal`
- Included fix commit: `cc5328c4 修复 OpenAI Responses SSE 终止事件识别`
- Action: merged `upstream/main` into private `main`
- Private fork merge commit: `b41778ef Merge remote-tracking branch 'upstream/main'`
- Push status: pushed to `origin/main`

WindsurfAPI:

- Remote: `https://github.com/dwgx/WindsurfAPI`
- Local path: `D:\wflogin\_github_research\WindsurfAPI`
- Current observed head: `c028576 release: 2.0.96`
- Current observed latest tag: `v2.0.96`
- Action: fetched successfully after one transient GitHub connection timeout
- Result: no upstream changes to apply

WindsurfPoolAPI:

- Remote: `https://github.com/guanxiaol/WindsurfPoolAPI`
- Local path: `D:\wflogin\_github_research\WindsurfPoolAPI`
- Current observed latest tag: `v2.0.7`
- Result: no upstream changes observed

Kiro:

- Remote: `https://github.com/hank9999/kiro.rs`
- New local git checkout: `D:\wflogin\_github_research\kiro.rs`
- Current observed head: `f1bbe9f chore(build): 更新 Node.js 至 22，pnpm 至 11`
- Current observed latest tag: `v2026.3.1`
- Action: cloned as a real git checkout for future tracking

kiro-gateway:

- Remote: `https://github.com/jwadow/kiro-gateway`
- Observed branch head before timeout: `b370ec5de14d3cc48cd575f9b71cab1ff9e3832f refs/heads/main`
- Action: tag fetch hit a transient GitHub connection timeout
- Follow-up: retry before using as a source of truth

### Local validation

Command:

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/service 42.458s
```

Note:

- Initial attempt with the default Go proxy failed while downloading Go
  toolchain `go1.26.3`.
- Retrying with `GOPROXY=https://goproxy.cn,direct` succeeded.

### Server check

Current server deployment:

- Compose path: `/opt/sub2api`
- Running image: `weishaw/sub2api:latest`
- Current image ID after update: `abd6d88929c4`
- Current companion images after update:
  - `caddy:2-alpine` image ID `86deaf5e3d34`
  - `postgres:18-alpine` image ID `96d56f7f57c6`
  - `redis:8-alpine` image ID `d146f83b1e0f`

Backup created before update:

```text
/opt/sub2api-backups/sub2api-20260518-142850.tar.gz
```

Update command:

```bash
cd /opt/sub2api
./update.sh
```

Result:

- `docker compose pull` completed.
- `docker compose up -d --remove-orphans` completed.
- Internal health check passed: `GET http://127.0.0.1:8080/health`.
- Public HEAD request to `https://api.vyywcw.cn/` returned HTTP 200.
- Containers remained healthy.
- No Windsurf/Kiro internal adapter services are deployed yet.

Admin account note:

- `.env` contains initialization values only.
- Actual admin user in database is:
  - email: `adminyu@vyywcw.cn`
  - username: `yu`
  - role: `admin`
  - 2FA: disabled
- The admin password is stored as a bcrypt hash and cannot be read back in
  plaintext.
- No password reset was performed because the password was remembered.

### Follow-ups

- [ ] Retry `kiro-gateway` tag fetch when GitHub connectivity is stable.
- [ ] Decide whether to update server `.env` comments/values to avoid confusion
      between initial admin email and actual admin user. Do not change secrets
      without a maintenance window.
- [ ] Add WindsurfAPI as an internal service only after collecting account
      tokens and confirming language server binary handling.
- [ ] Add Kiro proxy as an internal service only after deciding whether to use
      `kiro.rs` image/build or build from source.
- [ ] Add explicit backup paths for future Windsurf/Kiro data directories once
      those services exist.

## 2026-05-18 Windsurf Stage A implementation

Code changes:

- Added `POST /api/v1/admin/accounts/import/windsurf`.
- Added safe parser for `token`, `tokens`, `raw`, and `accounts`.
- Added duplicate detection inside one request.
- Added forwarding to internal `WindsurfAPI /auth/login`.
- Added recursive upstream response redaction.
- Added token-hash idempotency payload so raw Windsurf tokens are not stored in Sub2API idempotency records.

Reference checked:

- `dwgx/WindsurfAPI` stayed at `c028576 release: 2.0.96`, tag `v2.0.96`.
- Confirmed `/auth/login` accepts `token`, `api_key`, and `accounts`.
- Confirmed accepted auth headers include `Authorization: Bearer <key>` and `x-api-key`.

Validation:

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run Windsurf
go test ./internal/handler/admin
go test ./internal/server
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin
?  github.com/Wei-Shaw/sub2api/internal/server [no test files]
```

Not deployed yet:

- Server still needs a custom fork image.
- Server still needs internal `windsurf-api` compose service.
- Server still needs Sub2API env `WINDSURF_ADAPTER_INTERNAL_BASE_URL` and `WINDSURF_ADAPTER_INTERNAL_API_KEY`.
