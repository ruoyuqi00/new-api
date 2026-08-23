# User Session Issuance Exemption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let configured users such as user 79 bypass only the rolling login-session issuance limit while retaining every other authentication control.

**Architecture:** Parse a comma-separated environment allowlist into a read-only positive-ID set during startup. The service selects an explicit model operation that enforces the active-session transaction without the issuance-count query; the existing limited operation remains unchanged for all non-exempt users.

**Tech Stack:** Go 1.22+, GORM v2, SQLite test fixtures, testify `require`/`assert`.

---

### Task 1: Configuration parsing

**Files:**
- Modify: `common/constants.go`
- Modify: `common/init.go`
- Test: `common/user_session_test.go`

- [ ] **Step 1: Write the failing configuration tests**

Add assertions that `USER_SESSION_ISSUANCE_EXEMPT_USER_IDS="79, 81,79,invalid,0,-2"` contains only 79 and 81 after `initUserSessionSettings`, and that an empty value produces an empty set.

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./common -run 'UserSessionSettings|IssuanceExempt' -count=1`

Expected: FAIL because `UserSessionIssuanceExemptUserIDs` and its membership helper do not exist.

- [ ] **Step 3: Implement the minimal parser**

Add a startup-populated `map[int]struct{}` and:

```go
func IsUserSessionIssuanceExempt(userID int) bool {
    _, ok := UserSessionIssuanceExemptUserIDs[userID]
    return ok
}
```

Parse trimmed comma-separated values with `strconv.Atoi`, retaining only positive IDs. The map is replaced during startup and only read afterward.

- [ ] **Step 4: Run the focused common tests**

Run: `go test ./common -run 'UserSessionSettings|IssuanceExempt' -count=1`

Expected: PASS.

### Task 2: Active-limit-only model operation

**Files:**
- Modify: `model/user_session.go`
- Test: `model/user_session_test.go`

- [ ] **Step 1: Write the failing model regression**

Create one existing active session, call the wished-for `CreateUserSessionWithinActiveLimit` with active limit 1, and assert the old session is evicted while the replacement is created. The setup deliberately represents an issuance count that would fail at limit 1.

- [ ] **Step 2: Run the test to verify RED**

Run: `go test ./model -run 'CreateUserSessionWithinActiveLimit' -count=1`

Expected: FAIL because the explicit model operation does not exist.

- [ ] **Step 3: Implement the shared transaction option**

Keep `CreateUserSessionWithinLimits` as the issuance-enforcing public API. Add `CreateUserSessionWithinActiveLimit` and route both through the same transaction implementation with an explicit `enforceIssuanceLimit` boolean. Only the issuance query/check is conditional; locking, SID uniqueness, eviction, insertion, rollback fencing, and cache updates stay shared.

- [ ] **Step 4: Run the focused model tests**

Run: `go test ./model -run 'CreateUserSessionWithin(Limits|ActiveLimit)' -count=1`

Expected: PASS.

### Task 3: Service selection and user 79 regression

**Files:**
- Modify: `service/auth_session.go`
- Test: `service/auth_session_test.go`

- [ ] **Step 1: Write failing service regressions**

Configure the allowlist with user 79, active limit 1, and issuance limit 1. Assert user 79 can log in twice and has exactly one active session after eviction. Create a separate non-exempt user and assert its second login returns `model.ErrUserSessionIssuanceLimit`.

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./service -run 'LoginSession.*IssuanceExempt|LoginSession.*IssuanceLimit' -count=1`

Expected: FAIL because `createLoginSession` always invokes issuance enforcement.

- [ ] **Step 3: Implement service-side selection**

Use the authenticated database user ID only:

```go
if common.IsUserSessionIssuanceExempt(userID) {
    _, err = model.CreateUserSessionWithinActiveLimit(session, common.UserSessionActiveLimit)
} else {
    _, err = model.CreateUserSessionWithinLimits(session, common.UserSessionActiveLimit, common.UserSessionIssuanceLimit, common.UserSessionIssuanceWindowSeconds)
}
```

- [ ] **Step 4: Run focused service tests**

Run: `go test ./service -run 'LoginSession.*IssuanceExempt|LoginSession.*IssuanceLimit' -count=1`

Expected: PASS.

### Task 4: Operator documentation and verification

**Files:**
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `docs/authentication.md`

- [ ] **Step 1: Document the opt-in variable**

Add commented examples using `USER_SESSION_ISSUANCE_EXEMPT_USER_IDS=79` and explicitly state that it does not bypass active-session or authentication security controls.

- [ ] **Step 2: Format and run complete affected-package verification**

Run:

```text
gofmt -w common/constants.go common/init.go common/user_session_test.go model/user_session.go model/user_session_test.go service/auth_session.go service/auth_session_test.go
go test ./common ./model ./service ./controller -count=1
go vet ./common ./model ./service ./controller
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 3: Commit the scoped implementation**

Stage only the listed files and commit with `fix: allow scoped login issuance exemptions`. Do not change production, Caddy, databases, or UI assets in this task.
