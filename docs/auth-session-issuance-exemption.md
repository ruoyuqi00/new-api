# User Session Issuance Exemption

## Goal

Allow a small, explicitly configured set of users to continue creating login sessions after the rolling session-issuance window is exhausted. The exemption is intended for user 79 and is disabled by default.

## Scope and invariants

- Configuration uses `USER_SESSION_ISSUANCE_EXEMPT_USER_IDS`, a comma-separated list of positive numeric user IDs.
- The default is an empty list, so existing behavior is unchanged for all users unless an operator opts in.
- Exempt users skip only the rolling issuance-count check.
- Exempt users still use the active-session limit; old active sessions can still be evicted according to the existing policy.
- Password/2FA checks, JWT and refresh-token rotation, IP/interface limits, and all other authentication controls remain unchanged.
- No database schema or historical session data is changed.
- Invalid or non-positive entries are ignored (fail closed); duplicate IDs are harmless.

## Request flow

1. Startup parses the environment variable into an in-memory positive-ID set.
2. Login session creation checks the authenticated user ID against that set.
3. The model uses the existing transaction and active-session enforcement. For an exempt user it omits only the issuance-count query and limit failure.
4. Any database error still aborts session creation and is returned through the existing authentication error path.

The exemption is evaluated server-side from the user ID. It is never accepted from a request parameter, cookie, or client-provided header.

## Compatibility and rollout

The implementation keeps the existing limited-session model API as the default path and adds an explicit active-limit-only path for exempt users. This preserves existing callers and keeps SQLite, MySQL, and PostgreSQL behavior in the same transaction model.

The candidate is first tested locally with the allowlist containing `79`. Production remains untouched until the candidate is reviewed. Deployment must preserve the approved production UI assets and retain the previous container as the rollback target.

## Required regression coverage

- User 79 can create a session after the issuance window limit is reached.
- A non-exempt user is still rejected at the same issuance limit.
- User 79 remains subject to the active-session limit and eviction behavior.
- Parsing accepts whitespace and duplicates, ignores invalid IDs, and leaves the default set empty.
