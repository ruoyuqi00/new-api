# Dashboard authentication and session migration

The dashboard uses short-lived bearer access tokens, an `HttpOnly` refresh cookie, and server-side records in `user_sessions`. Dashboard authentication no longer depends on the legacy Gin cookie session or the `New-Api-User` header.

## Production runtime configuration

Configure every application node with the following non-secret values:

```env
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://yuaiapi.com,https://global.yuaiapi.com,https://api.yuaiapi.com
TRUSTED_PROXIES=127.0.0.0/8,::1/128,172.16.0.0/12
```

Generate `SESSION_SECRET` from at least 32 cryptographically random bytes and provide the same value to every application node through the production secret store. Never put the value in git, an image, this document, or a Compose file. Startup checks the byte length and rejects known placeholders such as `random_string` plus obvious low-diversity values with fewer than eight distinct bytes. This validation catches common deployment mistakes but cannot measure or prove the secret's entropy.

`SESSION_SECRET` protects dashboard access tokens, refresh-token hashes, security proofs, and temporary authentication flows. This migration does not change relay API keys, and relay clients continue using their existing keys. Rotating `SESSION_SECRET` invalidates dashboard sessions and in-progress authentication flows.

For local HTTP development, `SESSION_COOKIE_SECURE=false` may be used with `SESSION_COOKIE_TRUSTED_URL` unset. In that mode an omitted `SESSION_SECRET` retains the existing process-random development fallback. Do not expose this mode to the public internet.

## Cookie origin policy

When `SESSION_COOKIE_SECURE=true`, refresh cookies are sent only over HTTPS and refresh/logout requests enforce a strict browser origin check. `SESSION_COOKIE_TRUSTED_URL` is a comma-separated list of exact HTTPS origins, not a CORS allowlist. Wildcards, HTTP URLs, paths, queries, user information, empty entries, and suffix matching are rejected during startup.

The configured YuAI origins are:

- `https://yuaiapi.com`
- `https://global.yuaiapi.com`

TLS termination at a reverse proxy does not make a client-supplied `X-Forwarded-Proto` authoritative for this check. Both public HTTPS origins must remain explicitly configured.

## Trusted proxies

The production value trusts loopback and the Compose/private proxy range used by the application topology:

```env
TRUSTED_PROXIES=127.0.0.0/8,::1/128,172.16.0.0/12
```

The list contains proxy addresses or CIDRs, not client networks. An explicit list replaces the compatibility defaults. `TRUSTED_PROXIES=none` disables forwarded-header trust; invalid or empty explicit lists stop startup.

## Session controls

The default server-side controls are:

```env
USER_SESSION_ACTIVE_LIMIT=50
USER_SESSION_ISSUANCE_LIMIT=100
USER_SESSION_ISSUANCE_WINDOW_SECONDS=86400
USER_SESSION_REVOKED_RETENTION_DAYS=7
USER_SESSION_HOURLY_ALERT_THRESHOLD=5000
```

The active limit rejects new logins after a user reaches the configured number of unexpired active sessions. The issuance limit counts all sessions created in its window, including revoked sessions. Non-positive or invalid values fall back to defaults. The issuance window is clamped to the revoked-session retention period so cleanup cannot silently weaken issuance accounting.

The dashboard exposes session listing, single-session revocation, and revoke-other-sessions controls. Password changes, account security changes, and authentication-version changes invalidate affected server-side sessions.

## Cutover and migration

The database migration adds the server-side session and authentication-flow records used by the new dashboard flow. It is designed for SQLite, MySQL 5.7, and PostgreSQL 9.6 and must remain idempotent when startup migration runs more than once.

The `user_sessions.previous_refresh_hash` migration changes the legacy fixed-width representation to nullable `varchar(64)` while preserving existing digests. Repeated migration runs must not repeat type-changing DDL.

Legacy dashboard cookie sessions are not converted. After cutover, users with an old dashboard cookie session must log in once to establish the new refresh cookie and server-side session. Relay API keys are unaffected and do not need to be regenerated.
