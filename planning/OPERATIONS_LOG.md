# Operations Log

## 2026-05-19 Sub2API image a147caa0 deployment

Deployment:

- Built local fork image on the US server from git archive:
  `sub2api-provider-adapters:a147caa0`.
- Previous production image:
  `sub2api-provider-adapters:c4cefc76`.
- Server data backup before image build:
  `/opt/sub2api-backups/sub2api-20260519-123646.tar.gz`.
- Compose backup before switching the image tag:
  `/opt/sub2api-backups/docker-compose-20260519-044206-pre-a147caa0.yml`.

Post-deploy validation:

- `sub2api` healthy on `127.0.0.1:8080->8080`.
- Public `https://api.vyywcw.cn/` returned HTTP 200.
- Public `https://www.vyywcw.cn/` returned HTTP 200.
- Internal adapter ports remained unpublished:
  - `windsurf-api` `3003/tcp`: `null`
  - `kiro-rs` `8990/tcp`: `null`
  - `kiro-gateway` `8000/tcp`: `null`
- Internal `kiro-gateway /v1/models` returned 13 model IDs.
- Public Sub2API smoke with `qwen3-coder-next` returned HTTP 200 and text
  `ok`.

## 2026-05-19 Kiro.rs Claude correction

Correction:

- Local `D:\wflogin\kiro.rs-master` can access Claude through its local runtime
  configuration.
- Local smoke against `claude-sonnet-4-6` returned HTTP 200 and text `ok`.
- The local config includes a proxy URL, while the server `kiro-rs` service
  uses direct egress.

Server recheck:

- Copied the local full `credentials.json` to
  `/opt/sub2api/kiro-rs/config/credentials.json` after backing up the previous
  server credential file.
- Restarted `kiro-rs`.
- Forced token refresh through the server `kiro-rs` admin API.
- Server direct `kiro-rs` smoke with `claude-sonnet-4-6` still returned
  `INVALID_MODEL_ID`.

Conclusion:

- Do not describe this as “kiro.rs cannot access Claude.”
- The more accurate statement is that local `kiro.rs` with the local proxy
  egress can access Claude, while the current US server direct egress cannot.
- A separate `kiro.rs` fix is still needed because its admin add-credential
  path drops `accessToken`, `profileArn`, and `expiresAt`; direct file import
  preserves those fields.

## 2026-05-19 Kiro Gateway runtime validation

Reference refresh:

- Official Sub2API upstream advanced to `14f54be0`; merged into the private
  fork after committing the Kiro credential metadata import support.
- `hank9999/kiro.rs` remained at observed HEAD `f1bbe9f`, latest observed tag
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remained at observed HEAD `a5292ca`, latest observed
  tag `v2.3`.
- `dwgx/WindsurfAPI` remained at observed HEAD `c028576`, tag `v2.0.96`.

Server changes:

- Added `kiro-gateway` to `/opt/sub2api/docker-compose.yml` as an
  internal-only service.
- Compose backup before the change:
  `/opt/sub2api-backups/docker-compose-20260519-042547-pre-kiro-gateway.yml`.
- Removed the previous ad-hoc `sub2api-kiro-gateway` container and recreated it
  through Docker Compose.
- Kept `kiro-gateway` unexposed publicly: no host port and no Caddy route.
- Persisted runtime paths:
  - `/opt/sub2api/kiro-gateway/creds`
  - `/opt/sub2api/kiro-gateway/debug_logs`
  - `/opt/sub2api/kiro-gateway/state`

Kiro account handling:

- The user-provided Kiro credential was treated as a secret and stored only on
  the server runtime path, not in Git.
- The credential initialized successfully in `kiro-gateway`.
- Claude-family Kiro models remain disabled in Sub2API mapping because direct
  runtime smoke still returns model/subscription errors.

Sub2API upstream account:

- Added database account `kiro-gateway-internal-anthropic`.
- Platform/type: `anthropic` / `apikey`.
- Base URL: `http://kiro-gateway:8000`.
- Added to group `windsurf-smoke`.
- Enabled only these smoke-passed Kiro models:
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.5`
  - `qwen3-coder-next`

Validation:

- `kiro-gateway` health became healthy after compose start.
- Internal `GET http://kiro-gateway:8000/v1/models` returned 13 model IDs.
- Internal Sub2API smoke with `qwen3-coder-next` returned HTTP 200 and text
  `ok`.
- Public Sub2API smoke through `https://api.vyywcw.cn/v1/messages` with
  `deepseek-3.2` returned HTTP 200 and text `ok`.

Operational note:

- Do not widen Kiro model mapping until a model-specific smoke passes.
- If the Kiro runtime account fails, disable or remove only
  `kiro-gateway-internal-anthropic`; Windsurf uses a separate internal account.

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
- Extended parser to support Windsurf `email/password` and raw `email----password` lines.
- Added duplicate detection inside one request.
- Added forwarding to internal `WindsurfAPI /auth/login`.
- Added recursive upstream response redaction.
- Added secret-hash idempotency payload so raw Windsurf tokens/passwords/emails are not stored in Sub2API idempotency records.

Reference checked:

- `dwgx/WindsurfAPI` stayed at `c028576 release: 2.0.96`, tag `v2.0.96`.
- Confirmed `/auth/login` accepts `token`, `api_key`, and `accounts`.
- Confirmed source path also accepts account items containing `email` and `password`.
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

## 2026-05-18 Windsurf Stage A server deployment

Server status after deployment:

- Compose path: `/opt/sub2api`
- Public Sub2API domains:
  - `https://api.vyywcw.cn/`
  - `https://www.vyywcw.cn/`
- Deployed Sub2API fork image: `sub2api-provider-adapters:544f553b`
- Internal Windsurf adapter image: `ghcr.io/dwgx/windsurf-api:latest`
- WindsurfAPI upstream version observed in reference repo and container logs:
  `v2.0.96` / commit `c028576`
- Sub2API upstream check: local fork is ahead of official upstream; upstream
  had no new commits to merge at the time of the check.

Services:

- `sub2api` healthy after restart.
- `windsurf-api` healthy and reachable only inside the Docker network.
- `postgres`, `redis`, and `caddy` remained healthy.
- No Caddy public route and no host port were added for `windsurf-api`.

Server configuration added:

- `WINDSURF_API_KEY`
- `WINDSURF_DASHBOARD_PASSWORD`
- `WINDSURF_ADAPTER_INTERNAL_BASE_URL=http://windsurf-api:3003`
- `WINDSURF_ADAPTER_INTERNAL_API_KEY=<WINDSURF_API_KEY>`
- `WINDSURF_ADAPTER_TIMEOUT_SECONDS=30`
- `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true`
- `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`

Important: the security URL allowlist was loosened because Sub2API needed to
call an internal Docker service over plain HTTP. Keep `windsurf-api` internal
only; do not add a public route for it.

Import endpoint verified on server:

- `POST /api/v1/admin/accounts/import/windsurf`
- Email/password import succeeded.
- Top-level `api_key` import succeeded after commit `544f553b`.
- `accounts[].api_key` batch-shaped import succeeded after commit `544f553b`.

Model smoke after deployment:

- Internal WindsurfAPI direct `/v1/messages`:
  - `gemini-2.5-flash` returned HTTP 200.
  - `claude-sonnet-4.6` returned HTTP 200.
  - `claude-4.5-haiku` returned HTTP 200.
- Public Sub2API `/v1/messages` through the `windsurf-smoke` group:
  - `gemini-2.5-flash` returned HTTP 200.
  - `claude-sonnet-4.6` returned HTTP 200.

Incident found and fixed during smoke:

- Symptom: Sub2API public route returned 403/503 after WindsurfAPI rejected
  `gemini-2.5-flash` with `model_not_entitled`.
- Direct WindsurfAPI account state showed the account capability probe had
  `gemini-2.5-flash` as successful, but `availableModels` did not include it.
- Cause: WindsurfAPI `getAvailableModelsForAccount` only includes enum-keyed
  models when capability reason is `user_status`; this account had a canary
  `success` capability for `gemini-2.5-flash`, so preflight excluded it.
- Server-side short-term fix: set the Windsurf trial account tier to `pro` via
  the internal dashboard API, then clear the Sub2API upstream account error
  state and restart `sub2api`.
- Long-term fix candidate: in our adapter notes or future WindsurfAPI fork,
  treat `capabilities[model].ok === true` as available even when the reason is
  `success`, unless an explicit blocklist or `not_entitled` result exists.

Operational note:

- A single bad upstream 403 can mark the Sub2API upstream account `error`.
  Before retesting after a known adapter-side fix, clear only transient/error
  state for the internal upstream account and restart `sub2api` to refresh the
  scheduler snapshot.

## 2026-05-19 Upstream sync and Windsurf UI import

Reference check:

- Official Sub2API upstream advanced by two commits:
  - `164e2f61 fix: add keepalive for Anthropic passthrough streams`
  - `1d78dde8 Merge pull request #2552 from lyen1688/fix/anthropic-passthrough-keepalive`
- Merged `upstream/main` into the private fork without conflicts.
- `dwgx/WindsurfAPI` remained at `c028576 release: 2.0.96`, tag `v2.0.96`.

Code changes:

- Added the admin UI entry `Import Windsurf` under account management more actions.
- Added a Windsurf import modal that calls `POST /api/v1/admin/accounts/import/windsurf`.
- UI import modes:
  - `token`: one or more Windsurf tokens.
  - `api_key`: one or more Codeium/Windsurf API keys, submitted as `api_key`.
  - `email_password`: one `email----password` pair per line.
  - `json`: full backend-supported request object or account array.
- Added frontend request/response types and an API client wrapper with an
  idempotency key header.
- Added pure parser tests for the UI payload builder.

Validation:

```powershell
$env:GOCACHE='C:\Users\Administrator\.codex\memories\gocache'
go test ./internal/handler/admin -run Windsurf
go test ./internal/service -run 'AnthropicAPIKeyPassthrough'

$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm exec vitest run src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin
ok github.com/Wei-Shaw/sub2api/internal/service
1 frontend test file passed, 6 tests passed
vue-tsc passed
```

Operational note:

- `pnpm` v11 may require build-script approval for `esbuild` and `vue-demi`
  on a fresh local checkout. For local verification, using
  `npm_config_dangerously_allow_all_builds=true` avoids committing a generated
  `pnpm-workspace.yaml` approval file.

## 2026-05-19 Kiro adapter import bridge

Reference check:

- `hank9999/kiro.rs` latest observed HEAD: `f1bbe9f`.
- `Jwadow/kiro-gateway` latest observed HEAD: `a5292ca`; latest observed tag
  line includes `v2.3`.
- `dwgx/WindsurfAPI` remained at `c028576`, tag `v2.0.96`.
- Official Sub2API upstream had advanced again; merged `upstream/main` and
  resolved the only conflict in `Dockerfile` by keeping the fork's pinned
  `PNPM_VERSION=9.15.9` while absorbing upstream's pnpm-v9 build fix.

Code changes:

- Added `POST /api/v1/admin/accounts/import/kiro`.
- Added admin UI entry `Import Kiro`.
- Added Kiro import modal with modes:
  - refresh token lines.
  - Kiro API key lines.
  - full JSON object or account array.
- Backend forwards each credential to internal `kiro.rs`
  `POST /api/admin/credentials`.
- Backend accepts snake_case and kiro.rs camelCase fields.
- Backend returns per-item import results and redacts nested secrets.
- Added local mirrors:
  - `D:\wflogin\_github_research\kiro.rs-latest`
  - `D:\wflogin\_github_research\kiro-gateway`
- Added `planning/KIRO_STAGE_B_IMPORT_ENDPOINT.md`.

Validation:

```powershell
go test ./internal/handler/admin -run Kiro
go test ./internal/handler/admin
go test ./internal/server

$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm exec vitest run src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
corepack pnpm exec vite build
```

Result:

```text
Kiro backend tests passed.
Admin handler tests passed.
Server route package passed.
Frontend parser tests passed: 12 tests across Kiro and Windsurf.
vue-tsc passed.
vite build passed with existing chunk/dynamic import warnings.
```

Deployment note:

- This code path requires a private internal Kiro adapter service and these
  Sub2API env vars:
  - `KIRO_ADAPTER_INTERNAL_BASE_URL=http://kiro-rs:8990`
  - `KIRO_ADAPTER_ADMIN_API_KEY=<kiro-rs adminApiKey>`
  - `KIRO_ADAPTER_TIMEOUT_SECONDS=30`
- Do not expose `kiro-rs` through Caddy or Docker host ports.

## 2026-05-19 provider adapter fusion round 2

Reference refresh:

- Official Sub2API upstream advanced from `11870cf8` to `8584b8f7`.
- Merged upstream updates into the private fork. The upstream changes cover
  OpenAI-compatible usage parsing, Codex tool-call ID test alignment, and admin
  settings dark-mode readability.
- `dwgx/WindsurfAPI` remains at `c028576`, tag `v2.0.96`.
- `hank9999/kiro.rs` remains at `f1bbe9f`; local tags were refreshed through
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remains at `a5292ca`, tag line `v2.3`.

Code changes:

- Normalized Windsurf import responses into per-item `items[]`, matching the
  Kiro import shape more closely.
- `POST /api/v1/admin/accounts/import/windsurf` now returns:
  - `succeeded`
  - `failed`
  - `items[].index`
  - `items[].kind`
  - `items[].success`
  - `items[].error`
  - redacted `items[].upstream`
- The admin Windsurf import modal now shows success/failure counts and a
  per-item preview instead of relying only on the raw upstream body.

Operational impact:

- Existing clients that only read `total`, `forwarded`, `duplicate_count`,
  `upstream_status`, or `upstream` remain compatible.
- Operators can now identify the exact failed row in a batch import without
  printing tokens, API keys, or passwords.

Deployment:

- Built and deployed server image `sub2api-provider-adapters:c4cefc76`.
- Previous server image was `sub2api-provider-adapters:f95c2073`.
- Server backup before switch:
  `/opt/sub2api-backups/sub2api-20260519-102757-pre-c4cefc76.tar.gz`
  (about 40 MB).
- `docker compose ps` showed:
  - `sub2api` healthy on `127.0.0.1:8080->8080`.
  - `windsurf-api` internal only on `3003/tcp`.
  - `kiro-rs` internal only on `8990/tcp`.
  - Postgres, Redis, and Caddy running.

Smoke results:

- Local Sub2API health: `GET http://127.0.0.1:8080/health` returned
  `{"status":"ok"}`.
- Public `https://api.vyywcw.cn/` returned HTTP 200.
- Public `https://www.vyywcw.cn/` returned HTTP 200.
- Internal Windsurf health from Sub2API container returned status ok and
  reported one active account.
- Internal Kiro models endpoint from Sub2API container returned a model list.
- Internal Kiro admin credentials endpoint returned `total: 0`, which is
  expected before a real Kiro credential is imported.
- Docker inspect confirmed adapter ports are not published:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`

Note:

- The deploy helper's first health curl ran too early while the container was
  still starting and returned a transient connection reset. A follow-up retry
  verification passed after the container became healthy.

## 2026-05-19 kiro.rs admin metadata patch and public smoke

Reference refresh:

- `hank9999/kiro.rs` remains at `f1bbe9f`, latest observed tag
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remains at `a5292ca`, latest observed tag `v2.3`.
- Private Sub2API fork remains synced with `origin/main` at `1709e676` before
  this documentation update.

kiro.rs patch:

- Patched local `D:\wflogin\kiro.rs-master` so admin
  `POST /api/admin/credentials` accepts full exported metadata:
  - `accessToken` / `access_token`
  - `profileArn` / `profile_arn`
  - `expiresAt` / `expires_at`
- Added snake_case aliases for common admin import fields so direct API import
  and Sub2API bridge import use the same shape.
- Changed `AdminService::add_credential` to pass `access_token`,
  `profile_arn`, and `expires_at` into `KiroCredentials` instead of setting
  them to `None`.
- Built server image `kiro-rs-admin-metadata:20260519-1425`.
- Switched only the internal `kiro-rs` service to that image.

Verification:

- Targeted Rust test passed:
  `cargo test admin::types::tests::add_credential_request_accepts_full_export_metadata`.
- Full local `cargo test` still has 8 pre-existing Anthropic converter failures
  around old `claude-sonnet-4` / `claude-opus` model assertions. Those failures
  are unrelated to the admin import metadata patch.
- Server `kiro-rs` restarted successfully and loaded 1 credential.
- Internal Kiro admin credential status returned:
  - `total: 1`
  - `available: 1`
  - `with_profile_arn: 1`
  - `auth_methods: ["social"]`
- Internal Windsurf account status returned:
  - `total: 1`
  - `active: 1`
  - tier: `pro`
- Public Sub2API smoke through `https://api.vyywcw.cn/v1/messages` passed:
  - Windsurf `claude-sonnet-4.6`: HTTP 200, text `ok`
  - Kiro `qwen3-coder-next`: HTTP 200, text `ok`
  - Kiro `deepseek-3.2`: HTTP 200, text `ok`

Exposure check:

- Adapter ports remain internal only:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`
  - `sub2api-kiro-gateway`: `{"8000/tcp":null}`
- Caddy has no `windsurf-api`, `kiro-rs`, `kiro-gateway`, `3003`, `8990`, or
  `8000` routes.

## 2026-05-19 post-push verification

- Pushed commit `76633539` to
  `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`.
- Confirmed local fork is clean and aligned with `origin/main`.
- Server compose status:
  - `sub2api` healthy on `127.0.0.1:8080->8080`.
  - `windsurf-api`, `kiro-rs`, and `kiro-gateway` are running without public
    port mappings.
- Internal adapter status:
  - Windsurf: `total: 1`, `active: 1`, tier `pro`.
  - Kiro: `total: 1`, `available: 1`, `with_profile_arn: 1`,
    `auth_methods: ["social"]`.
- External public-domain smoke from local machine through
  `https://api.vyywcw.cn/v1/messages` returned HTTP 200 for:
  - `claude-sonnet-4.6`
  - `qwen3-coder-next`
  - `deepseek-3.2`

Operational note:

- Browser admin login should use the current remembered password.

## 2026-05-20 upstream main merge and server rollout

Reference refresh:

- `Wei-Shaw/sub2api` upstream was merged through `3d22dd34`.
- `dwgx/WindsurfAPI` remained at `c028576`, tag `v2.0.96`.
- `guanxiaol/WindsurfPoolAPI` remained at `a8d2f4c`, tag `v2.0.7`.
- `hank9999/kiro.rs` remained at `f1bbe9f`, tag `v2026.3.1`.
- `Jwadow/kiro-gateway` remained at `a5292ca`, tag `v2.3`.

Validation before deploy:

- `go test ./internal/handler/admin -run "Windsurf|Kiro|AvailableModels|SyncUpstream"` passed.
- `go test ./internal/server ./internal/pkg/apicompat ./internal/pkg/openai_compat` passed.
- `go test ./internal/handler/admin` passed.
- `go test ./internal/service -run "Upstream|OpenAI|Gateway|AccountCredentials|AdminService|Windsurf|Kiro|Pricing|Channel|Gemini"` passed.
- `go test ./...` passed after using a temporary `GOPROXY=https://goproxy.cn,direct`
  because local access to `proxy.golang.org` timed out over IPv6.
- `corepack pnpm exec vitest run src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` passed.
- `corepack pnpm build` passed with only Vite chunk/dynamic-import warnings.

Server rollout:

- Built server image `sub2api-provider-adapters:66940db0`.
- Backed up compose and database before switching:
  - `/opt/sub2api-backups/docker-compose-20260520-094723-pre-66940db0.yml`
  - `/opt/sub2api-backups/sub2api-db-20260520-094723-pre-66940db0.dump`
- Updated `/opt/sub2api/docker-compose.yml` to use
  `sub2api-provider-adapters:66940db0`.
- Restarted only the `sub2api` service. Adapter services were not rebuilt.

Post-deploy verification:

- `sub2api` is healthy on `127.0.0.1:8080->8080`.
- Internal adapter ports remain un published:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`
  - `sub2api-kiro-gateway`: `{"8000/tcp":null}`
- Internal adapter status:
  - Windsurf: `total: 1`, `active: 1`, tier `pro`.
  - Kiro: `total: 1`, `available: 1`, `with_profile_arn: 1`,
    `auth_methods: ["social"]`.
- Public smoke through `https://api.vyywcw.cn/v1/messages` returned HTTP 200
  for:
  - `claude-sonnet-4.6`
  - `claude-opus-4.6`
  - `qwen3-coder-next`
  - `deepseek-3.2`

Opus 4.7 alias note:

- Windsurf already exposed effort-specific Opus 4.7 model keys such as
  `claude-opus-4-7-low` and `claude-opus-4-7-medium`.
- Added migration `140_windsurf_opus47_aliases.sql` so user-facing dotted
  aliases map to the Windsurf keys.
- Manually applied the same aliases on the current server before the migration
  image was rebuilt.
- Public smoke returned HTTP 200 for:
  - `claude-opus-4.7`
  - `claude-opus-4-7`
  - `claude-opus-4.7-medium`
- High/max Opus 4.7 effort keys can still hit upstream rate limits depending
  on current account quota and Windsurf upstream state.
- The server `.env` `ADMIN_PASSWORD` no longer matches the live admin password,
  so CLI admin import smoke through `POST /api/v1/admin/accounts/import/kiro`
  was skipped after a 401 login response. Do not store the current admin
  password in Git or docs.
