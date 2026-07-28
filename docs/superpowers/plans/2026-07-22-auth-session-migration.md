# Dashboard Authentication Session Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the production fork's dashboard cookie-session authentication with the complete upstream access-token, rotating-refresh-token, and server-side session-control architecture while preserving all current production customizations and excluding the experimental UI.

**Architecture:** Port authentication behavior from `origin/main` into the existing production tree instead of merging the upstream repository-wide refactor. Backend session models and services remain database-authoritative with optional Redis acceleration; frontend access tokens remain in memory and use a HttpOnly refresh cookie, one coordinated refresh path, session-aware request deduplication, and explicit query-cache isolation.

**Tech Stack:** Go 1.22+, Gin, GORM v2, Redis, testify, React 19, TypeScript, Zustand, Axios, TanStack Query/Router, Bun test, Rsbuild.

---

## File Map

Backend persistence and cache:

- Create `model/user_session.go`: session persistence, refresh rotation, cache snapshots, revocation, limits, and cleanup queries.
- Create `model/auth_flow.go`: one-time temporary authentication flows and external assertion claims.
- Create `model/external_identity_claim.go`: atomic external identity ownership.
- Create `model/user_auth_cache.go`: authentication-version cache fences.
- Modify `model/user.go`, `model/user_cache.go`, `model/main.go`, `model/errors.go`: authentication version, migrations, and security-change invalidation.

Backend authentication services and HTTP boundaries:

- Create `service/auth_token.go`: access-token and security-proof issue/verify behavior.
- Create `service/auth_session.go`: login session creation, refresh, validation, revocation, and limits.
- Create `service/auth_cleanup.go`: bounded master-node cleanup.
- Create `controller/auth_session.go`: refresh, logout, list, revoke, and revoke-others endpoints.
- Create `middleware/auth_origin.go`: secure refresh/logout origin validation.
- Create `trusted_proxies.go`: explicit Gin trusted-proxy configuration.
- Modify `middleware/auth.go`, `router/api-router.go`, `controller/user.go`, `main.go`, and the existing login-provider controllers to use the new contract.

Frontend authentication core under the existing production path:

- Create `web/default/src/lib/http-client.ts`: Axios transport, session-aware GET deduplication, and one `401` recovery path.
- Create `web/default/src/lib/auth-session.ts`: refresh coordination, authentication epochs, mismatch/race recovery, and cache cleanup.
- Create `web/default/src/lib/auth-session-sync.ts`: cross-tab login/logout/session signaling.
- Replace `web/default/src/stores/auth-store.ts`: one in-memory AuthBundle and bootstrap status.
- Modify `web/default/src/lib/api.ts`, `web/default/src/main.tsx`, and `web/default/src/routes/_authenticated/route.tsx`: use the new authentication core and remove legacy `uid`, duplicate `401`, and `sessionVerified` behavior.

Frontend login and session management:

- Modify files under `web/default/src/features/auth/`, OAuth routes, passkey and secure-verification clients to consume the unified AuthBundle.
- Add login-session management components under `web/default/src/features/profile/components/`.
- Update all six locale files through the repository i18n workflow.

## Task 1: Establish an Isolated Baseline

**Files:**
- Verify only: repository and test commands; no production files modified.

- [ ] **Step 1: Create the implementation worktree**

Run the `superpowers:using-git-worktrees` skill and create branch
`codex/auth-session-migration-impl-20260722` from commit `2c384d8d6` in a path
outside `D:/yucore-api-export`.

- [ ] **Step 2: Verify the worktree starts from the intended production state**

Run:

```powershell
git rev-parse HEAD
git status --short --branch
git diff --name-only 739cb27751e5a89932597567356326d3a73a980f..HEAD
```

Expected: HEAD contains the approved design/plan commits; the only diff from
production is under `docs/superpowers/`; the worktree is otherwise clean.

- [ ] **Step 3: Run backend authentication and fork-specific regression baselines**

Run:

```powershell
go test ./middleware ./controller ./service ./model
go test ./... -run "PrivateGroup|ProviderAccount|ChannelAffinity|Failover|ApiAddress"
```

Expected: PASS before implementation. Record any pre-existing failure with its
full command and output before changing code.

- [ ] **Step 4: Run the current production frontend baseline**

Run from `web/default`:

```powershell
bun install --frozen-lockfile
bun run typecheck
bun run build
```

Expected: dependency install, typecheck, and production build pass.

- [ ] **Step 5: Record the protected path guard**

Use this command after every implementation commit:

```powershell
git diff --name-only 739cb27751e5a89932597567356326d3a73a980f..HEAD | rg "^(output/local-experiments/|output/imagegen/|web/experimental/|local-ui/)"
```

Expected: no output and exit code 1. Do not stage, delete, or modify any matching
path.

## Task 2: Add Session Persistence and Authentication-Version Fences

**Files:**
- Create: `model/user_session.go`
- Create: `model/auth_flow.go`
- Create: `model/external_identity_claim.go`
- Create: `model/user_auth_cache.go`
- Modify: `model/user.go`
- Modify: `model/user_cache.go`
- Modify: `model/main.go`
- Modify: `model/errors.go`
- Test: `model/user_session_test.go`
- Test: `model/user_session_migration_test.go`
- Test: `model/auth_flow_test.go`
- Test: `model/external_identity_claim_test.go`
- Test: `model/user_cache_auth_version_test.go`

- [ ] **Step 1: Port the upstream persistence tests first**

Port the listed test files from `origin/main` without production implementation.
Preserve testify `require`/`assert` usage and adapt only fixtures that differ in
the fork. The required contracts include these exact structures:

```go
type UserSession struct {
	SID                 string
	UserID              int
	Version             int64
	UserAuthVersion     int64
	RefreshHash         string
	PreviousRefreshHash string
	Status              string
	ExpiresAt           int64
}

type AuthFlow struct {
	Id        int64
	TokenHash string
	Purpose   string
	UserId    int
	SessionId string
	ExpiresAt time.Time
	ConsumedAt *time.Time
}
```

- [ ] **Step 2: Run the model tests and confirm the red state**

Run:

```powershell
go test ./model -run "UserSession|AuthFlow|ExternalIdentity|AuthVersion" -count=1
```

Expected: FAIL to compile because session, flow, identity-claim, and cache-fence
types/functions do not exist yet.

- [ ] **Step 3: Port the model implementations from the current parent**

Use `origin/main` versions of the four new model files as the behavioral source.
Apply them with `apply_patch`, then reconcile fork fields and migration order in
the existing model files. The resulting user authorization contract must expose:

```go
type UserBase struct {
	Id          int    `json:"id"`
	Group       string `json:"group"`
	Email       string `json:"email"`
	Quota       int    `json:"quota"`
	Status      int    `json:"status"`
	Role        int    `json:"role"`
	Username    string `json:"username"`
	Setting     string `json:"setting"`
	AuthVersion int64  `json:"-"`
	CacheSchema int    `json:"-"`
}

func BumpUserAuthVersion(userId int) (int64, error)
func GetUserCache(id int) (*UserBase, error)
func CreateUserSession(session *UserSession) error
func RotateUserSessionRefresh(userId int, sid, currentHash, nextHash string, now int64, replayWindow time.Duration) (*UserSession, error)
func RevokeUserSession(userId int, sid, reason string) (bool, error)
```

All new JSON encoding/decoding calls must use `common.Marshal`,
`common.Unmarshal`, `common.UnmarshalJsonStr`, or `common.DecodeJson`.

- [ ] **Step 4: Run the focused model suite**

Run:

```powershell
go test ./model -run "UserSession|AuthFlow|ExternalIdentity|AuthVersion" -count=1
```

Expected: PASS, including SQLite migration idempotency, refresh race/reuse,
revocation tombstones, cache fences, and ambiguous identity rejection.

- [ ] **Step 5: Commit the persistence layer**

```powershell
git add model/user_session.go model/user_session_test.go model/user_session_migration_test.go model/auth_flow.go model/auth_flow_test.go model/external_identity_claim.go model/external_identity_claim_test.go model/user_auth_cache.go model/user_cache_auth_version_test.go model/user.go model/user_cache.go model/main.go model/errors.go
git commit -m "feat(auth): add dashboard session persistence"
```

## Task 3: Add Token, Session, and Cleanup Services

**Files:**
- Create: `service/auth_token.go`
- Create: `service/auth_session.go`
- Create: `service/auth_cleanup.go`
- Modify: `common/constants.go`
- Modify: `common/init.go`
- Test: `service/auth_token_test.go`
- Test: `service/auth_session_test.go`

- [ ] **Step 1: Port service tests before implementations**

Port the two upstream test files and retain tests for purpose isolation,
tampering, opaque dotted PAT classification, security-proof scope binding,
session growth limits, refresh/revoke, Redis convergence, and authentication-
version invalidation.

- [ ] **Step 2: Verify the service tests fail**

Run:

```powershell
go test ./service -run "AccessToken|SecurityProof|LoginSession|AuthArtifacts|AuthVersion" -count=1
```

Expected: FAIL to compile because token and login-session services are absent.

- [ ] **Step 3: Port the current upstream service behavior**

Implement the current `origin/main` contracts with fork-compatible imports:

```go
type AuthBundle struct {
	AccessToken     string           `json:"access_token"`
	TokenType       string           `json:"token_type"`
	AccessExpiresAt int64            `json:"access_expires_at"`
	Session         LoginSessionView `json:"session"`
	RefreshToken    string           `json:"-"`
}

func CreateLoginSession(userID int, loginMethod, ip, userAgent string) (*AuthBundle, error)
func RefreshLoginSession(rawRefreshToken, expectedSID, ip, userAgent string) (*AuthBundle, *model.User, error)
func ValidateLoginSession(identity AuthIdentity) (*model.UserSession, *model.UserBase, error)
func ParseDashboardAccessToken(raw string) (AuthIdentity, bool, error)
```

Derive purpose-specific keys from `SESSION_SECRET`; never log token values.
Retain the latest upstream behavior where refresh `429`, `5xx`, and transport
failures remain retryable and do not imply an invalid session.

- [ ] **Step 4: Run service and persistence tests**

Run:

```powershell
go test ./service ./model -run "AccessToken|SecurityProof|LoginSession|AuthArtifacts|AuthVersion|UserSession" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit authentication services**

```powershell
git add service/auth_token.go service/auth_token_test.go service/auth_session.go service/auth_session_test.go service/auth_cleanup.go common/constants.go common/init.go
git commit -m "feat(auth): add token and session services"
```

## Task 4: Switch Middleware, Routes, Origins, and Trusted Proxies

**Files:**
- Create: `middleware/auth_origin.go`
- Create: `trusted_proxies.go`
- Modify: `middleware/auth.go`
- Modify: `router/api-router.go`
- Modify: `main.go`
- Modify: `common/session_cookie.go`
- Test: `middleware/auth_test.go`
- Test: `middleware/auth_origin_test.go`
- Test: `trusted_proxies_test.go`

- [ ] **Step 1: Add failing boundary tests**

Port the current upstream middleware and trusted-proxy tests. Preserve the fork's
existing `middleware/auth_optional_test.go` as an additional PAT regression.

- [ ] **Step 2: Verify the boundary tests fail**

Run:

```powershell
go test ./middleware . -run "UserAuth|TryUserAuth|SessionCookieOriginGuard|TrustedProxies" -count=1
```

Expected: FAIL because Bearer session authentication and strict cookie-origin
guarding are not connected.

- [ ] **Step 3: Implement the authentication classifier and guards**

The middleware classification order must be exact:

```go
identity, internal, err := service.ParseDashboardAccessToken(raw)
if internal {
	if err != nil {
		return nil, service.AuthIdentity{}, dashboardCredentialInternal, err
	}
	_, user, err := service.ValidateLoginSession(identity)
	return user, identity, dashboardCredentialInternal, err
}
patUser, err := model.ValidateAccessToken(raw)
```

Add refresh/logout OriginGuard routes and configure Gin trusted proxies from
`TRUSTED_PROXIES`. Preserve relay API-key middleware and the fork's optional PAT
recognition behavior.

- [ ] **Step 4: Run middleware and proxy tests**

Run:

```powershell
go test ./middleware . -run "UserAuth|TryUserAuth|SessionCookieOriginGuard|TrustedProxies" -count=1
```

Expected: PASS for valid access tokens, opaque/dotted PATs, invalid internal JWT
rejection, secure-origin validation, local development compatibility, and proxy
header spoofing protection.

- [ ] **Step 5: Commit HTTP authentication boundaries**

```powershell
git add middleware/auth.go middleware/auth_test.go middleware/auth_optional_test.go middleware/auth_origin.go middleware/auth_origin_test.go router/api-router.go main.go trusted_proxies.go trusted_proxies_test.go common/session_cookie.go
git commit -m "feat(auth): enforce stateless dashboard authentication"
```

## Task 5: Add Session Endpoints and Unified Password Login

**Files:**
- Create: `controller/auth_session.go`
- Modify: `controller/user.go`
- Modify: `router/api-router.go`
- Test: `controller/auth_session_test.go`
- Test: `controller/user_manage_test.go`

- [ ] **Step 1: Port failing controller contract tests**

Port the current upstream session-controller tests and the user-management
security-change cases. Keep the fork's login audit log assertions.

- [ ] **Step 2: Verify controller tests fail**

Run:

```powershell
go test ./controller -run "AuthLogout|AuthSession|SessionLimit|UserManage|Login" -count=1
```

Expected: FAIL because legacy login still writes Gin session values and does not
return an AuthBundle.

- [ ] **Step 3: Implement unified issue, refresh, logout, and revocation responses**

Successful login response shape must be:

```go
c.JSON(http.StatusOK, gin.H{
	"success": true,
	"message": "",
	"data": gin.H{
		"access_token":      bundle.AccessToken,
		"token_type":        bundle.TokenType,
		"access_expires_at": bundle.AccessExpiresAt,
		"session":           bundle.Session,
		"user":              buildSelfUserData(currentUser),
	},
})
```

Register exactly these endpoints:

```go
userRoute.POST("/auth/refresh", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.RefreshAuth)
userRoute.POST("/auth/logout", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AuthLogout)
selfRoute.GET("/sessions", middleware.DisableCache(), controller.GetLoginSessions)
selfRoute.DELETE("/sessions/:sid", middleware.DisableCache(), controller.DeleteLoginSession)
selfRoute.POST("/sessions/revoke-others", middleware.DisableCache(), controller.RevokeOtherLoginSessions)
```

Password login and 2FA completion call the single session-issuance service and
record a successful audit only after issuance succeeds.

- [ ] **Step 4: Run controller, middleware, and service tests**

Run:

```powershell
go test ./controller ./middleware ./service -run "Auth|Login|Session|UserManage" -count=1
```

Expected: PASS, including refresh-cookie SID mismatch returning the stable `409`
contract without revoking either session.

- [ ] **Step 5: Commit unified login and session endpoints**

```powershell
git add controller/auth_session.go controller/auth_session_test.go controller/user.go controller/user_manage_test.go router/api-router.go
git commit -m "feat(auth): issue and manage dashboard login sessions"
```

## Task 6: Port OAuth, Passkey, 2FA, Telegram, WeChat, and Security Proofs

**Files:**
- Modify: `controller/oauth.go`
- Modify: `controller/passkey.go`
- Modify: `controller/secure_verification.go`
- Modify: `controller/telegram.go`
- Modify: `controller/twofa.go`
- Modify: `controller/wechat.go`
- Modify: `middleware/secure_verification.go`
- Modify: `model/passkey.go`
- Modify: `model/twofa.go`
- Modify: `service/passkey/session.go`
- Test: `controller/auth_flow_test.go`
- Test: `controller/passkey_test.go`
- Test: `controller/telegram_test.go`

- [ ] **Step 1: Port the authentication-flow tests first**

Port the upstream flow, passkey proof, and Telegram atomicity tests. Retain fork-
specific provider configuration and callback URL behavior.

- [ ] **Step 2: Verify provider-flow tests fail**

Run:

```powershell
go test ./controller ./service/passkey -run "OAuth|AuthFlow|Passkey|Telegram|TwoFA|WeChat|SecurityProof" -count=1
```

Expected: FAIL where legacy Gin session state is still used or flows are not
bound to the current login session.

- [ ] **Step 3: Port each provider to the common session-issuance exit**

Every successful login provider must finish through the controller's shared
session-issuance exit:

```go
setupLogin(user, c)
```

Replace temporary Gin-session values with one-time `auth_flows`. Bind passkey
registration/deletion and channel-key reveal proofs to user ID, session SID,
authentication version, session version, and scope. Preserve the current fork's
OAuth providers and login audit content.

- [ ] **Step 4: Run provider and full authentication tests**

Run:

```powershell
go test ./controller ./middleware ./service ./service/passkey ./model -run "Auth|OAuth|Passkey|Telegram|TwoFA|WeChat|Session|SecurityProof" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit provider-flow migration**

```powershell
git add controller/oauth.go controller/passkey.go controller/passkey_test.go controller/secure_verification.go controller/telegram.go controller/telegram_test.go controller/twofa.go controller/wechat.go controller/auth_flow_test.go middleware/secure_verification.go model/passkey.go model/twofa.go service/passkey/session.go
git commit -m "feat(auth): migrate login providers to session control"
```

## Task 7: Add the Frontend Authentication Core

**Files:**
- Create: `web/default/src/lib/http-client.ts`
- Create: `web/default/src/lib/auth-session.ts`
- Create: `web/default/src/lib/auth-session-sync.ts`
- Replace: `web/default/src/stores/auth-store.ts`
- Modify: `web/default/src/lib/api.ts`
- Modify: `web/default/src/main.tsx`
- Modify: `web/default/src/routes/_authenticated/route.tsx`
- Modify: `web/default/src/components/sign-out-dialog.tsx`
- Test: `web/default/src/lib/auth-session.test.ts`

- [ ] **Step 1: Add the frontend session regression tests**

Port `origin/main:web/src/lib/auth-session.test.ts` to the existing default path
and adapt imports only. The tests must retain explicit cases for `401`, `409`,
`429`, `503`, network failure, exhausted races, stale refresh epochs, and
query/mutation/auth cleanup.

- [ ] **Step 2: Verify the frontend tests fail**

Run from `web/default`:

```powershell
bun test src/lib/auth-session.test.ts
```

Expected: FAIL because the new session coordinator and atomic AuthBundle store
do not exist.

- [ ] **Step 3: Port the current upstream core under `web/default`**

Use the latest `origin/main` versions, including commit `172114422`, and expose
the atomic state shape:

```ts
export interface AuthBundle {
  access_token: string
  token_type: 'Bearer' | string
  access_expires_at: number
  user: AuthUser
  session: LoginSession
}

export type AuthBootstrapState = 'idle' | 'checking' | 'complete'

interface AuthState {
  auth: {
    user: AuthUser | null
    accessToken: string | null
    accessExpiresAt: number | null
    session: LoginSession | null
    bootstrapState: AuthBootstrapState
    setBundle: (bundle: AuthBundle) => void
    reset: (bootstrapState?: AuthBootstrapState) => void
  }
}
```

The GET deduplication key must include
`useAuthStore.getState().auth?.session.sid ?? 'anonymous'`. The response
interceptor is the only global `401` owner. Remove legacy `uid`, persisted user,
React Query `401` reset, and global `sessionVerified` logic.

- [ ] **Step 4: Run unit tests, type checking, and build**

Run from `web/default`:

```powershell
bun test src/lib/auth-session.test.ts
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 5: Commit the frontend authentication core**

```powershell
git add web/default/src/lib/http-client.ts web/default/src/lib/auth-session.ts web/default/src/lib/auth-session-sync.ts web/default/src/lib/auth-session.test.ts web/default/src/stores/auth-store.ts web/default/src/lib/api.ts web/default/src/main.tsx web/default/src/routes/_authenticated/route.tsx web/default/src/components/sign-out-dialog.tsx
git commit -m "feat(auth): coordinate dashboard authentication in the client"
```

## Task 8: Migrate Frontend Login Methods and Session Management UI

**Files:**
- Modify: `web/default/src/features/auth/api.ts`
- Modify: `web/default/src/features/auth/hooks/use-auth-redirect.ts`
- Modify: `web/default/src/features/auth/hooks/use-oauth-login.ts`
- Modify: `web/default/src/features/auth/sign-in/components/user-auth-form.tsx`
- Modify: `web/default/src/features/auth/sign-up/components/sign-up-form.tsx`
- Modify: `web/default/src/features/auth/otp/components/otp-form.tsx`
- Modify: `web/default/src/features/auth/passkey/api.ts`
- Modify: `web/default/src/features/auth/secure-verification/api.ts`
- Modify: `web/default/src/routes/(auth)/oauth.tsx`
- Modify: `web/default/src/routes/(auth)/sign-in.tsx`
- Modify: `web/default/src/routes/oauth/$provider.tsx`
- Create: `web/default/src/features/profile/components/login-session-utils.ts`
- Create: `web/default/src/features/profile/components/login-session-item.tsx`
- Create: `web/default/src/features/profile/components/login-session-dialogs.tsx`
- Create: `web/default/src/features/profile/components/login-sessions-card.tsx`
- Modify: `web/default/src/features/profile/index.tsx`
- Test: `web/default/src/features/auth/api.test.ts`
- Test: `web/default/src/features/profile/components/login-session-utils.test.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Load the project i18n skill and add failing login-flow tests**

Use the repository `i18n-translate` skill before editing user-facing strings.
Port the current upstream logout-coordination and login-session presentation
tests to `web/default`.

- [ ] **Step 2: Verify the login-flow tests fail**

Run from `web/default`:

```powershell
bun test src/features/auth/api.test.ts src/features/profile/components/login-session-utils.test.ts
```

Expected: FAIL because AuthBundle parsing, logout mismatch recovery, and session
presentation utilities are absent.

- [ ] **Step 3: Migrate every login path to `setAuth`**

Normalize every successful response through the store's single bundle setter:

```ts
if (response.success && response.data?.access_token) {
  useAuthStore.getState().auth.setBundle(response.data)
}
```

Password, 2FA, passkey, OAuth, WeChat, and Telegram paths must all call this
setter. Logout must recover one SID mismatch and must not pretend success on a
temporary refresh/logout failure.

- [ ] **Step 4: Add the session-management card and translations**

Port the upstream session list/revoke UI into the existing profile page without
changing the current brand theme or page layout. Run:

```powershell
bun run i18n:sync
bun test src/features/auth/api.test.ts src/features/profile/components/login-session-utils.test.ts
bun run typecheck
```

Expected: translations are present for all six locales, tests pass, and
typecheck passes.

- [ ] **Step 5: Commit frontend login and session management**

```powershell
git add web/default/src/features/auth web/default/src/features/profile web/default/src/routes web/default/src/i18n/locales
git commit -m "feat(auth): migrate dashboard login flows and session controls"
```

## Task 9: Add Production Configuration and Migration Validation

**Files:**
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `docs/authentication.md`
- Test: `common/user_session_test.go`
- Test: `controller/auth_flow_test.go`

- [ ] **Step 1: Add configuration validation tests**

Port the upstream `common/user_session_test.go` cases for `SESSION_SECRET`,
secure cookies, and trusted origins. Retain the existing production environment
variable names and Docker service topology.

- [ ] **Step 2: Verify invalid configuration fails closed**

Run:

```powershell
go test ./common ./controller -run "Session|AuthFlow|Origin" -count=1
```

Expected: PASS tests explicitly prove missing/invalid production secrets and
origins are rejected; no permissive production fallback is introduced.

- [ ] **Step 3: Document exact runtime configuration**

Configure the session secret through the production secret store and add these
non-secret values to the deployment environment:

```env
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://yuaiapi.com,https://global.yuaiapi.com
TRUSTED_PROXIES=127.0.0.0/8,::1/128,172.16.0.0/12
```

Use one generated 32-byte-or-longer `SESSION_SECRET` value on every application
node; store it outside git and never print it in command output.

Document that API relay keys are unaffected and all legacy dashboard cookies
will require one new login after cutover.

- [ ] **Step 4: Run migration tests twice against SQLite**

Run:

```powershell
go test ./model -run "Migration|ExternalIdentity|UserSession" -count=2
```

Expected: PASS twice, proving startup migration idempotency.

- [ ] **Step 5: Run the configured migration tests on MySQL and PostgreSQL**

Start isolated compatibility databases:

```powershell
docker run -d --name newapi-auth-mysql -e MYSQL_ROOT_PASSWORD=auth-session-test -e MYSQL_DATABASE=auth_session -p 33306:3306 mysql:5.7.44
docker run -d --name newapi-auth-postgres -e POSTGRES_PASSWORD=auth-session-test -e POSTGRES_DB=auth_session -p 35432:5432 postgres:9.6-alpine
while (-not (docker exec newapi-auth-mysql mysqladmin ping -h 127.0.0.1 -pauth-session-test --silent 2>$null)) { Start-Sleep -Seconds 1 }
while (-not (docker exec newapi-auth-postgres pg_isready -U postgres -d auth_session 2>$null)) { Start-Sleep -Seconds 1 }
$env:TEST_MYSQL_DSN='root:auth-session-test@tcp(127.0.0.1:33306)/auth_session?charset=utf8mb4&parseTime=True&loc=Local'
$env:TEST_POSTGRES_DSN='host=127.0.0.1 port=35432 user=postgres password=auth-session-test dbname=auth_session sslmode=disable'
go test ./model -run 'TestUserSessionPreviousRefreshHashMigrationConfiguredDatabases' -count=2
Remove-Item Env:TEST_MYSQL_DSN
Remove-Item Env:TEST_POSTGRES_DSN
docker rm -f newapi-auth-mysql newapi-auth-postgres
```

Expected: MySQL 5.7 and PostgreSQL 9.6 subtests both PASS twice without repeated
schema-changing DDL.

- [ ] **Step 6: Commit configuration and migration documentation**

```powershell
git add .env.example docker-compose.yml docs/authentication.md common/user_session_test.go
git commit -m "docs(auth): define production session migration settings"
```

## Task 10: Run Full Regression and Fork-Compatibility Verification

**Files:**
- Modify only when a failing regression identifies an authentication integration defect.

- [ ] **Step 1: Run the full backend suite**

```powershell
go test ./... -count=1
```

Expected: PASS. Fix only failures caused by the authentication migration; record
pre-existing failures separately with evidence.

- [ ] **Step 2: Run frontend tests and quality checks**

Run from `web/default`:

```powershell
bun test
bun run typecheck
bun run lint
bun run format:check
bun run build
```

Expected: PASS.

- [ ] **Step 3: Run fork-specific regression suites**

```powershell
go test ./... -run "PrivateGroup|ProviderAccount|ChannelAffinity|Failover|ApiAddress|Media|Billing" -count=1
```

Expected: PASS for private downstream group visibility/routing, provider-account
failover, channel affinity recovery, API URL export, media pricing, and billing.

- [ ] **Step 4: Audit the final diff for excluded content**

```powershell
git diff --stat 739cb27751e5a89932597567356326d3a73a980f..HEAD
git diff --name-only 739cb27751e5a89932597567356326d3a73a980f..HEAD | rg "^(output/local-experiments/|output/imagegen/|web/experimental/|local-ui/)"
git status --short
```

Expected: authentication code, tests, configuration, and docs only; protected
path search has no output; worktree has no unstaged implementation changes.

- [ ] **Step 5: Commit any final integration-only corrections**

```powershell
git add middleware/auth.go controller/auth_session.go service/auth_session.go web/default/src/lib/auth-session.ts web/default/src/lib/http-client.ts
git commit -m "fix(auth): complete production fork integration"
```

Skip this commit when no correction was required. Never stage with `git add .`.

## Task 11: Build and Validate a Non-Production Candidate

**Files:**
- Output only: candidate image and local test artifacts; do not commit generated files.

- [ ] **Step 1: Build the production candidate image**

```powershell
docker build --pull=false -t newapi:auth-session-candidate-20260722 .
```

Expected: image builds from the production Dockerfile and does not contain
experimental UI output.

- [ ] **Step 2: Restore a sanitized production database copy**

Create a separate test database from the latest production backup. Point only a
candidate container at that copy; do not reuse the production database or Redis.

- [ ] **Step 3: Start the isolated candidate and verify migration health**

Use a separate container name, ports, database, Redis namespace, and session
secret. Expected: startup completes, additive migrations run once, and a second
restart produces no schema changes or migration errors.

- [ ] **Step 4: Run browser smoke checks through local proxy configuration**

Verify password login, 2FA when enabled, logout, refresh, profile, wallet, usage
logs, registration/email verification, session list/revoke, and account switch.
Verify a forced refresh `429` and `503` leaves the user signed in and retryable.

- [ ] **Step 5: Verify relay traffic independently**

Using a dedicated test token and minimal-cost request, verify `/v1/models` and a
small `/v1/responses` call against the candidate. Expected: relay authentication,
channel routing, and billing behavior match production.

## Task 12: Prepare the Reversible Cutover and Stop for Approval

**Files:**
- Create: `docs/superpowers/plans/2026-07-22-auth-session-cutover-checklist.md`

- [ ] **Step 1: Record immutable rollback inputs**

Include the current production image `newapi:production-console-20260722-739cb2775`,
commit `739cb27751e5a89932597567356326d3a73a980f`, container configuration, health
endpoint, and database-backup checksum.

- [ ] **Step 2: Record cutover commands without executing them**

The checklist must separate database backup, candidate start, health checks,
traffic switch, login/refresh checks, and rollback into individually reversible
commands. It must state that the old dashboard cookies are intentionally invalid.

- [ ] **Step 3: Run verification-before-completion**

Invoke `superpowers:verification-before-completion`, rerun the required focused
and full suites, and attach command results to the handoff summary.

- [ ] **Step 4: Request explicit production deployment approval**

Stop before SSH, database backup, image transfer, container replacement, traffic
switch, push, or production migration. Present candidate evidence, expected
5-15 minute cutover behavior, and rollback evidence to the user.
