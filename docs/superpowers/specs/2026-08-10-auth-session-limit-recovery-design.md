# Auth Session Limit Recovery Design

## Problem

Dashboard authentication creates a server-side session for every successful
password, 2FA, passkey, or OAuth login. A user may have at most 50 active
sessions, and each session remains active for 30 days unless it is explicitly
revoked. The refresh cookie is host-only, so `yuaiapi.com` and
`global.yuaiapi.com` maintain different browser cookies while consuming the
same per-user session allowance.

Once a user reaches the active-session limit, valid credentials cannot create
another session. The login endpoint returns `409 AUTH_SESSION_LIMIT`, but the
user cannot open the authenticated session-management page to revoke stale
sessions. Clearing browser cookies makes the deadlock worse because it removes
the credential needed to revoke the corresponding server-side session.

Production evidence on 2026-08-10 showed five login 409 responses in 30 minutes,
zero refresh 409 responses, and two users at exactly 50 active sessions.

## Goals

- Let a correctly authenticated user recover from the active-session limit
  without an administrator editing the database.
- Keep no more than the configured number of active sessions after a new login.
- Preserve the independent daily issuance limit and return 429 without revoking
  an existing session when that limit is reached.
- Revoke the least valuable session deterministically: oldest activity first,
  then oldest creation time, then SID as a stable tie-breaker.
- Keep database and Redis session state consistent enough that an evicted token
  cannot remain usable through a stale positive cache entry.
- Support SQLite, MySQL, and PostgreSQL.
- Leave billing, API tokens, channel routing, Caddy, and user balances unchanged.

## Non-Goals

- Sharing cookies across the two domains.
- Removing the active-session or daily issuance limits.
- Adding a new unauthenticated session-recovery endpoint.
- Changing access-token or login-session lifetimes.
- Automatically deleting historical session rows needed for issuance limits and
  audit retention.

## Considered Approaches

### 1. Replace the oldest active session during authenticated login

After credentials are verified, check the daily issuance limit, make room under
the active-session limit, and create the new session in one database operation.
This resolves the deadlock without weakening password, 2FA, passkey, or OAuth
verification. It also works when the browser has lost every old cookie.

This is the selected approach.

### 2. Raise or disable the active-session limit

This postpones the failure but keeps stale sessions valid and allows unbounded
session growth. It does not address the underlying recovery deadlock and weakens
account security.

### 3. Add an unauthenticated recovery flow

A dedicated page could re-verify credentials and let the user choose sessions to
revoke. This gives more control, but duplicates authentication logic, adds a new
security-sensitive endpoint, and requires substantially more frontend and abuse
protection work. It is unnecessary for the immediate production failure.

## Selected Behavior

All login methods continue to authenticate exactly as they do today. Session
creation then follows this order:

1. Lock the user's database row so concurrent logins for the same user are
   serialized on MySQL and PostgreSQL. SQLite continues to rely on its
   single-writer transaction behavior.
2. Count every session row issued within the configured issuance window. If the
   daily issuance limit is reached, return the existing 429 error and do not
   revoke anything.
3. Count active, non-expired sessions across all auth versions.
4. If creating one more session would exceed the active limit, select enough
   sessions to restore the invariant. For the normal 50-session boundary this
   evicts one session; historical over-limit data may require more.
5. Order eviction candidates by `last_active_at ASC`, `created_at ASC`, and
   `sid ASC`.
6. Publish Redis deny fences for the selected sessions, mark them revoked with
   reason `session_limit_replaced`, and insert the new session in the same
   database transaction.
7. After commit, finalize revoked cache tombstones and populate the new session
   cache. If the database transaction rolls back after a deny fence was written,
   restore the candidate's active cache snapshot.

The issuance check deliberately precedes eviction. A user who has reached the
100-per-day issuance limit must receive 429 without unexpectedly losing an
existing working session.

## Code Boundaries

- `model/user_session.go` owns the transactional limit enforcement, deterministic
  candidate selection, session-row updates, and Redis cache fencing.
- `service/auth_session.go` continues to validate the user, generate refresh
  material, and issue the access token. It calls the new atomic model operation
  instead of separately counting and inserting sessions.
- `controller/user.go` and the frontend login forms require no behavioral change:
  a valid login at the active limit now succeeds and returns the normal auth
  bundle.
- Existing `AUTH_SESSION_LIMIT` mapping remains as a fail-closed compatibility
  response for invalid configuration or an unrecoverable limit condition.

## Failure Handling

- Daily issuance limit: preserve `429 AUTH_SESSION_ISSUANCE_LIMIT`; no eviction.
- Database error before commit: no row changes; restore any prewritten cache
  fence and return the existing generic authentication error.
- Redis disabled: database state remains authoritative, matching existing
  behavior. If Redis is enabled but the pre-commit deny fence cannot be written,
  abort the database transaction rather than risk accepting an evicted token
  through stale positive cache state. Post-commit tombstone or new-session cache
  failures follow the existing logging and database-fallback behavior.
- Concurrent same-user login: user-row locking serializes the count, eviction,
  and insert sequence so the active limit is not exceeded.
- Existing token for an evicted session: the deny fence prevents a stale Redis
  hit; database validation observes the revoked row.

## Test Strategy

Backend tests will use deterministic SQLite fixtures and project-standard
`require`/`assert` assertions:

- At 49 active sessions, create the 50th without revoking anything.
- At 50 active sessions, revoke exactly the least recently active session and
  create a replacement while keeping 50 active sessions.
- Above the configured limit, revoke enough oldest sessions to restore the
  invariant after insertion.
- Use creation time and SID as deterministic tie-breakers.
- At the daily issuance limit, return the existing issuance error and leave all
  sessions active.
- Roll back eviction when inserting the replacement fails.
- Confirm the evicted session is denied by the Redis cache while the replacement
  is readable.
- Confirm password login does not return 409 solely because 50 active sessions
  already exist.
- Run the complete Go test suite and both frontend production builds even though
  no frontend source change is expected.

## Deployment And Rollback

Build a new image from this branch and bind the candidate only to a private
localhost port. Verify login, refresh, logout, session listing, and both public
domains before requesting production cutover approval.

The deployment does not require a schema migration or direct production data
cleanup. The first valid login for an affected user performs the bounded
replacement automatically. Existing production images and containers remain
available for rollback, and Caddy is not changed without explicit approval.
