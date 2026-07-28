# Dashboard Authentication Session Migration Design

## Objective

Port the complete dashboard authentication and session-control behavior from
upstream new-api commit `31d70fca393ff2e09bbae012af2e3ccefdd389a1`, including the transient refresh
failure fix at `1721144221ec5c94dd87891a7ae1bee228e7bb63`, into the current production
fork without importing the upstream frontend-directory reorganization or
removing fork-specific production features.

The migration must eliminate the current cookie-session consistency failures:

- stale browser user and `uid` state;
- cross-session reuse of in-flight GET requests;
- duplicate global `401` handling;
- global `sessionVerified` state surviving account changes;
- logout on transient refresh `429`, `5xx`, timeout, or network failures;
- stale React Query data after logout or account switching;
- refresh races across tabs or between old and new login attempts.

Existing dashboard sessions may be invalidated during the first production
cutover. Users will be required to sign in once after the upgrade.

## Scope

### Included

- Stateless dashboard Access Tokens and rotating HttpOnly Refresh Tokens.
- Database-backed login session control and authentication-version invalidation.
- Session listing, individual revocation, and revoke-other-session APIs.
- Unified login responses for password, 2FA, passkey, OAuth, WeChat, and Telegram.
- Temporary authentication flows and security proofs required by the upstream
  authentication contract.
- Session-aware frontend request deduplication, refresh coordination, cross-tab
  synchronization, and query-cache cleanup.
- Current dashboard PAT compatibility without requiring `New-Api-User`.
- SQLite, MySQL, and PostgreSQL-compatible migrations and tests.
- Production configuration, observability, backup, cutover, and rollback steps.

### Excluded

- Moving `web/default` to `web`.
- Removing `web/classic`.
- Importing unrelated upstream frontend, branding, theme, billing, routing, or
  provider changes.
- Uploading or modifying the local experimental UI.
- Deploying before local and pre-production verification is complete and the
  user explicitly approves the production cutover.

## Chosen Approach

Perform a capability-complete, path-adapted port into the current production
fork. Backend authentication components are ported by behavior and dependency,
while frontend authentication components are placed under the existing
`web/default/src` structure. Fork-specific controller behavior, UI, routing,
billing, channel scheduling, private groups, and media integrations remain in
place and are reconciled at each touched boundary.

This is preferable to merging the upstream refactor commit wholesale because
that commit also flattens the frontend tree and deletes the classic frontend.
It is preferable to a small compatibility patch because the requested outcome
is the full session-control fix rather than temporary symptom suppression.

## Backend Architecture

### Token Model

- Access Tokens are JWTs with a 15-minute lifetime and remain only in browser
  memory.
- Refresh Tokens are opaque random values stored only in an HttpOnly cookie,
  with their HMAC digests persisted server-side.
- Refresh Tokens rotate on every successful refresh. The deterministic recovery
  behavior from upstream is retained so concurrent tabs can recover the same
  successor instead of invalidating each other.
- `SESSION_SECRET` derives purpose-separated keys for access tokens, refresh
  digests, authentication flows, and security proofs.

### Session Control

- `user_sessions` is the authority for login session status, expiry, revocation,
  device metadata, and authentication version.
- `users.auth_version` invalidates all prior login sessions after security-
  relevant account changes.
- Redis caches authentication and session snapshots when enabled, while the
  database remains authoritative. Cache misses and Redis-disabled deployments
  fall back to the database.
- Revocation tombstones and bounded TTLs prevent stale cache writes from
  reauthorizing revoked sessions.

### Authentication Flows

- `auth_flows` stores HMAC-digested, one-time temporary flows for OAuth, 2FA,
  passkey, Telegram, and other multi-step authentication operations.
- `external_identity_claims` enforces ownership of external identities during
  login and binding operations.
- Sensitive operations use short-lived, scoped security proofs bound to the
  user, session, authentication version, and intended action.

### API Contract

- All successful login methods return one AuthBundle containing `user`,
  `access_token`, `access_expires_at`, and current session metadata.
- `POST /api/user/auth/refresh` rotates the Refresh Token and issues a new
  Access Token.
- `POST /api/user/auth/logout` revokes the current login session and clears the
  Refresh Cookie.
- Session management APIs list, revoke one, or revoke all other sessions.
- Dashboard authentication accepts Bearer Access Tokens. Existing PATs continue
  to work as Bearer or legacy single-value Authorization credentials.
- Relay API-key authentication and channel routing are not changed.

## Frontend Architecture

### Auth State

- The Zustand store owns one atomic AuthBundle: user, Access Token, expiry, and
  session SID.
- Access Tokens are never persisted in localStorage or shared through cross-tab
  messages.
- Legacy `user` and `uid` localStorage entries are removed during migration and
  are no longer used for request authentication.
- The authenticated route requires a resolved authenticated bundle rather than
  a process-global `sessionVerified` flag.

### Refresh and Concurrency

- One shared refresh promise prevents duplicate refreshes in a tab.
- Web Locks serialize refreshes across tabs when available.
- BroadcastChannel, with storage-event fallback, synchronizes only login,
  logout, and session identifiers across tabs.
- An authentication epoch prevents an older refresh from restoring credentials
  after a newer login or logout.
- A `401` from an authenticated request attempts refresh once and retries the
  original request with the new Access Token.
- Refresh `429`, `5xx`, timeout, and network failures are transient and do not
  clear authentication state. Only explicit invalid credentials, confirmed
  session mismatch, or exhausted race recovery transitions to anonymous state.

### Request and Cache Isolation

- The concurrent GET deduplication key includes the current session SID, so
  anonymous, previous-user, and current-user requests cannot share responses.
- Only the HTTP client owns global authentication recovery. React Query does not
  independently reset authentication on `401`.
- Logout and account switching cancel in-flight user work and clear user-bound
  queries and mutations before navigation.

## Database Migration

- Add `users.auth_version` and initialize existing users to the expected active
  version.
- Add `user_sessions`, `auth_flows`, and `external_identity_claims` with indexes
  required for lookup, issuance limits, cleanup, and uniqueness.
- Keep migrations idempotent and compatible with SQLite, MySQL 5.7.8+, and
  PostgreSQL 9.6+.
- Detect ambiguous historical external-identity bindings before cutover. A
  conflict aborts migration with an actionable report rather than silently
  choosing an owner.
- Validate migration against a sanitized production schema snapshot before any
  production image is started.

The old binary is expected to ignore additive tables and columns, which keeps
application rollback possible. A database backup remains mandatory because
authentication data created after cutover cannot be represented by the old
cookie-session model.

## Production Configuration

- Configure the same high-entropy `SESSION_SECRET` on every application node.
- Set `SESSION_COOKIE_SECURE=true` in production.
- Set `SESSION_COOKIE_TRUSTED_URL` to the exact HTTPS panel origins used by the
  primary and global domains.
- Configure `TRUSTED_PROXIES` for the actual Nginx/Cloudflare ingress topology so
  client IP and authentication rate limits cannot trust arbitrary public
  forwarding headers.
- Confirm all nodes share the same primary database and, when applicable, the
  same Redis and `CRYPTO_SECRET`.

## Error Handling and Observability

- Authentication API failures use stable machine-readable codes for invalid
  refresh credentials, session mismatch, session limits, issuance limits, and
  temporary service failures.
- Security-sensitive logs include session identifiers only in safe, non-secret
  form and never log Access or Refresh Tokens.
- Cutover monitoring tracks login success by method, refresh status codes,
  session mismatch/race recovery, database migration duration, and authenticated
  endpoint `401`, `409`, `429`, and `5xx` rates.
- A temporary spike in login traffic is expected because all old dashboard
  cookies become invalid after cutover.

## Test Strategy

Backend regression coverage must include:

- login, refresh rotation, logout, revocation, expiry, and auth-version changes;
- concurrent refresh recovery and replay rejection;
- session mismatch behavior;
- `429`, `5xx`, timeout, and network failures remaining non-destructive;
- password, 2FA, passkey, OAuth, WeChat, and Telegram login contracts;
- PAT compatibility and relay API-key non-regression;
- migration idempotency and supported-database behavior;
- Redis-enabled, Redis-disabled, cache-miss, and revocation-tombstone paths.

Frontend regression coverage must include:

- cold-start refresh and anonymous resolution;
- retry after an authenticated request receives `401`;
- no logout on refresh `429`, `503`, timeout, or network failure;
- stale refresh cannot overwrite a newer login or logout;
- session-aware GET deduplication;
- cross-tab login/logout synchronization;
- logout and account switching clear user-bound query and mutation state;
- authenticated route behavior without global verification state.

Verification also includes Go tests, frontend unit tests, frontend type checking,
the production frontend build, and browser checks of password login, logout,
profile, wallet, usage logs, registration, and both public domains.

## Cutover and Rollback

1. Build and test an isolated candidate image without changing production.
2. Back up the production database and record current image/configuration.
3. Run migration validation against a restored database copy.
4. Start the candidate against the copied database and complete browser smoke
   tests through the real proxy and domain configuration.
5. During the approved maintenance window, deploy the candidate, run additive
   migrations, and verify health before directing normal traffic to it.
6. Expect existing dashboard users to sign in once. Relay API keys continue to
   serve traffic throughout the dashboard authentication cutover.
7. If health, login, refresh, or protected-page checks fail, restore the previous
   application image immediately. Restore the database backup only if the
   additive schema itself is shown to prevent rollback or data integrity is at
   risk.

## Acceptance Criteria

- All dashboard login methods produce the new AuthBundle and can refresh,
  logout, and revoke sessions correctly.
- Temporary refresh failures do not log users out.
- Account switching cannot display or reuse prior-user requests or cached data.
- Current production channel routing, billing, private groups, media models,
  documentation, brand theme, and API URL exports remain unchanged except where
  authentication integration explicitly requires adaptation.
- Relay traffic remains available during candidate development and cutover.
- The local experimental UI is absent from the commit, build context, image,
  push, and deployment.
- Production deployment occurs only after explicit user approval of the tested
  candidate and rollback evidence.
