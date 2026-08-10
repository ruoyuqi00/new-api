# Auth Session Limit Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Allow a correctly authenticated user at the active-session limit to log in by atomically replacing the least recently active session without weakening the daily issuance limit.

**Architecture:** Move active-limit enforcement into one model transaction that locks the user row, checks issuance before eviction, deterministically revokes enough old sessions, and inserts the replacement. The service remains responsible for credentials and token material; model cache fences ensure evicted sessions cannot survive through stale Redis state.

**Tech Stack:** Go 1.22+, GORM v2, SQLite/MySQL/PostgreSQL, go-redis/miniredis, Gin, testify.

---

### Task 1: Specify Atomic Replacement In Model Tests

**Files:**
- Modify: model/user_session_test.go
- Test: model/user_session_test.go

- [ ] **Step 1: Write the failing least-recently-active replacement test**

Add this test with the existing model fixtures:

~~~go
func TestCreateUserSessionWithinLimitsReplacesLeastRecentlyActive(t *testing.T) {
	setupUserSessionTest(t)
	const userID = 1010
	createUserSessionTestUser(t, userID, 1)
	now := time.Now().Unix()
	rows := []*UserSession{
		newTestUserSession("replacement-newer", userID, now-30),
		newTestUserSession("replacement-oldest", userID, now-40),
		newTestUserSession("replacement-newest", userID, now-20),
	}
	rows[0].LastActiveAt = now - 10
	rows[1].LastActiveAt = now - 20
	rows[2].LastActiveAt = now - 5
	require.NoError(t, DB.Create(rows).Error)

	replacement := newTestUserSession("replacement-current", userID, now)
	evicted, err := CreateUserSessionWithinLimits(replacement, 3, 100, 86400)
	require.NoError(t, err)
	require.Len(t, evicted, 1)
	assert.Equal(t, "replacement-oldest", evicted[0].SID)

	storedOldest, err := GetUserSessionBySID("replacement-oldest")
	require.NoError(t, err)
	assert.Equal(t, UserSessionStatusRevoked, storedOldest.Status)
	assert.Equal(t, "session_limit_replaced", storedOldest.RevokedReason)
	activeCount, err := CountActiveUserSessions(userID, time.Now().Unix())
	require.NoError(t, err)
	assert.Equal(t, int64(3), activeCount)
}
~~~

- [ ] **Step 2: Add issuance-order, over-limit, tie-break, and rollback cases**

Add focused tests with these exact contracts:

~~~go
func TestCreateUserSessionWithinLimitsChecksIssuanceBeforeEviction(t *testing.T) {
	setupUserSessionTest(t)
	const userID = 1011
	createUserSessionTestUser(t, userID, 1)
	now := time.Now().Unix()
	existing := newTestUserSession("issuance-preserved", userID, now-1)
	require.NoError(t, DB.Create(existing).Error)

	replacement := newTestUserSession("issuance-rejected", userID, now)
	evicted, err := CreateUserSessionWithinLimits(replacement, 1, 1, 86400)
	assert.ErrorIs(t, err, ErrUserSessionIssuanceLimit)
	assert.Empty(t, evicted)

	stored, err := GetUserSessionBySID(existing.SID)
	require.NoError(t, err)
	assert.Equal(t, UserSessionStatusActive, stored.Status)
	var replacementCount int64
	require.NoError(t, DB.Model(&UserSession{}).Where("sid = ?", replacement.SID).Count(&replacementCount).Error)
	assert.Zero(t, replacementCount)
}

func TestCreateUserSessionWithinLimitsRestoresActiveInvariant(t *testing.T) {
	setupUserSessionTest(t)
	const userID = 1012
	createUserSessionTestUser(t, userID, 1)
	now := time.Now().Unix()
	for index := range 4 {
		session := newTestUserSession(fmt.Sprintf("over-limit-%d", index), userID, now-int64(10-index))
		session.LastActiveAt = now - int64(100-index)
		require.NoError(t, DB.Create(session).Error)
	}
	replacement := newTestUserSession("over-limit-current", userID, now)
	evicted, err := CreateUserSessionWithinLimits(replacement, 3, 100, 86400)
	require.NoError(t, err)
	assert.Len(t, evicted, 2)
	activeCount, err := CountActiveUserSessions(userID, time.Now().Unix())
	require.NoError(t, err)
	assert.Equal(t, int64(3), activeCount)
}

func TestCreateUserSessionWithinLimitsUsesStableTieBreakers(t *testing.T) {
	setupUserSessionTest(t)
	const userID = 1013
	createUserSessionTestUser(t, userID, 1)
	now := time.Now().Unix()
	for _, sid := range []string{"tie-b", "tie-a"} {
		session := newTestUserSession(sid, userID, now-10)
		session.LastActiveAt = now - 10
		require.NoError(t, DB.Create(session).Error)
	}
	replacement := newTestUserSession("tie-current", userID, now)
	evicted, err := CreateUserSessionWithinLimits(replacement, 2, 100, 86400)
	require.NoError(t, err)
	require.Len(t, evicted, 1)
	assert.Equal(t, "tie-a", evicted[0].SID)
}

func TestCreateUserSessionWithinLimitsRollsBackEvictionWhenInsertFails(t *testing.T) {
	setupUserSessionTest(t)
	const userID = 1014
	createUserSessionTestUser(t, userID, 1)
	now := time.Now().Unix()
	existing := newTestUserSession("duplicate-session", userID, now-10)
	require.NoError(t, DB.Create(existing).Error)

	replacement := newTestUserSession("duplicate-session", userID, now)
	_, err := CreateUserSessionWithinLimits(replacement, 1, 100, 86400)
	require.Error(t, err)
	stored, err := GetUserSessionBySID(existing.SID)
	require.NoError(t, err)
	assert.Equal(t, UserSessionStatusActive, stored.Status)
	assert.Empty(t, stored.RevokedReason)
}
~~~

- [ ] **Step 3: Verify RED**

Run:

~~~bash
go test ./model -run 'TestCreateUserSessionWithinLimits' -count=1
~~~

Expected: compilation fails because CreateUserSessionWithinLimits does not exist.

### Task 2: Implement Transactional Session Replacement

**Files:**
- Modify: model/user_session.go:133-196
- Test: model/user_session_test.go

- [ ] **Step 1: Extract shared new-session validation**

Add prepareNewUserSession and call it from existing CreateUserSession:

~~~go
func prepareNewUserSession(session *UserSession, now int64) error {
	if session == nil || session.SID == "" || session.UserID <= 0 ||
		session.UserAuthVersion <= 0 || session.RefreshHash == "" || session.ExpiresAt <= now {
		return ErrUserSessionInvalid
	}
	if session.Version <= 0 {
		session.Version = 1
	}
	if session.Status == "" {
		session.Status = UserSessionStatusActive
	}
	if session.Status != UserSessionStatusActive || session.RevokedAt != 0 {
		return ErrUserSessionInvalid
	}
	if session.LastActiveAt == 0 {
		session.LastActiveAt = now
	}
	if session.CreatedAt == 0 {
		session.CreatedAt = now
	}
	return nil
}
~~~

- [ ] **Step 2: Add the atomic model contract**

Implement:

~~~go
func CreateUserSessionWithinLimits(
	session *UserSession,
	activeLimit int,
	issuanceLimit int,
	issuanceWindowSeconds int64,
) ([]UserSession, error)
~~~

Inside one DB.Transaction:

1. Validate positive limits and call prepareNewUserSession.
2. Lock the users row with lockForUpdate(tx).Select("id").Where("id = ?", session.UserID).First(&user).
3. Count all user_sessions rows newer than now minus issuanceWindowSeconds. Return ErrUserSessionIssuanceLimit before writing cache or changing rows.
4. Count active, non-expired sessions.
5. Compute evictCount as activeCount minus activeLimit plus one.
6. Select evictCount rows using last_active_at ASC, created_at ASC, sid ASC under lockForUpdate.
7. Publish writeUserSessionDenyFence for each candidate using reason session_limit_replaced.
8. Update exactly those SIDs to revoked and insert the replacement through tx.Create.
9. If the transaction fails, restore every prewritten active cache snapshot.
10. If it commits, publish final revoked tombstones and populate the replacement cache with the same stale-observation handling as CreateUserSession.

Use ErrUserSessionLimit if the locked candidate count or update row count differs from evictCount. Do not add raw SQL or dialect-specific syntax.

- [ ] **Step 3: Verify GREEN and existing model behavior**

Run:

~~~bash
go test ./model -run 'TestCreateUserSessionWithinLimits' -count=1
go test ./model -run 'Test.*UserSession|TestSessionCache' -count=1
~~~

Expected: all new and existing session tests pass.

### Task 3: Route Login Through The Atomic Operation

**Files:**
- Modify: service/auth_session.go:56-111
- Modify: service/auth_session_test.go:98-169
- Test: service/auth_session_test.go

- [ ] **Step 1: Change the existing active-limit test to the recovery contract**

Rename TestCreateLoginSessionEnforcesActiveLimitAcrossAuthVersions to TestCreateLoginSessionReplacesOldestAtActiveLimitAcrossAuthVersions. Keep its 49-session setup. The first call must create session 50. The second call must also succeed, active count must remain 50, and active-limit-48 must be revoked with reason session_limit_replaced.

- [ ] **Step 2: Verify RED**

Run:

~~~bash
go test ./service -run 'TestCreateLoginSessionReplacesOldestAtActiveLimitAcrossAuthVersions' -count=1
~~~

Expected: the second login fails with ErrUserSessionLimit.

- [ ] **Step 3: Replace separate counts with the model transaction**

Remove the standalone active and issuance count checks from createLoginSession. After constructing the session, call:

~~~go
_, err = model.CreateUserSessionWithinLimits(
	session,
	common.UserSessionActiveLimit,
	common.UserSessionIssuanceLimit,
	common.UserSessionIssuanceWindowSeconds,
)
if err != nil {
	return nil, err
}
~~~

Keep refresh-secret generation, token issuance, and token-issue rollback unchanged.

- [ ] **Step 4: Verify active and issuance limits together**

Run:

~~~bash
go test ./service -run 'TestCreateLoginSession(ReplacesOldestAtActiveLimitAcrossAuthVersions|EnforcesIssuanceLimitAcrossAllStatuses)' -count=1
~~~

Expected: active-limit replacement succeeds; the issuance-limit test still returns ErrUserSessionIssuanceLimit without adding or revoking a session.

### Task 4: Prove Redis Denial And Login API Recovery

**Files:**
- Modify: service/auth_session_test.go
- Modify: controller/auth_session_test.go:111-156
- Test: service/auth_session_test.go
- Test: controller/auth_session_test.go

- [ ] **Step 1: Add the Redis eviction-fence test**

~~~go
func TestCreateLoginSessionAtLimitDeniesEvictedCachedSession(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	useIndependentAuthSessionRedis(t)
	common.UserSessionActiveLimit = 1
	common.UserSessionIssuanceLimit = 100

	first, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "first-agent")
	require.NoError(t, err)
	_, err = model.GetUserSessionCached(first.Session.SID)
	require.NoError(t, err)

	second, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "second-agent")
	require.NoError(t, err)
	_, err = model.GetUserSessionCached(first.Session.SID)
	assert.ErrorIs(t, err, model.ErrUserSessionInactive)
	storedSecond, err := model.GetUserSessionCached(second.Session.SID)
	require.NoError(t, err)
	assert.Equal(t, second.Session.SID, storedSecond.SID)
}
~~~

- [ ] **Step 2: Convert the controller 409 test into a success contract**

Rename TestSessionLimitDoesNotRecordRejectedLoginAsSuccessful to TestSessionLimitReplacementReturnsSuccessfulLogin. Keep active limit one, call setupLogin, and assert HTTP 200, success true, a non-empty returned session SID, the previous session revoked with reason session_limit_replaced, and exactly one active session. Set and restore a test SessionSecret in the fixture.

- [ ] **Step 3: Run focused API and cache tests**

Run:

~~~bash
go test ./service -run 'TestCreateLoginSessionAtLimitDeniesEvictedCachedSession' -count=1
go test ./controller -run 'TestSessionLimitReplacementReturnsSuccessfulLogin|TestWriteAuthSessionErrorMapsSessionGrowthLimits' -count=1
~~~

Expected: evicted cached credentials are denied and login at the active limit returns 200.

### Task 5: Full Verification And Source Preservation

**Files:**
- Modify only through formatting: model/user_session.go
- Modify only through formatting: model/user_session_test.go
- Modify only through formatting: service/auth_session.go
- Modify only through formatting: service/auth_session_test.go
- Modify only through formatting: controller/auth_session_test.go

- [ ] **Step 1: Format changed Go files**

~~~bash
gofmt -w model/user_session.go model/user_session_test.go service/auth_session.go service/auth_session_test.go controller/auth_session_test.go
~~~

- [ ] **Step 2: Run focused packages without test cache**

~~~bash
go test ./model ./service ./controller -count=1
~~~

Expected: all three packages pass.

- [ ] **Step 3: Run the complete backend suite serially**

~~~bash
go test -p 1 ./... -count=1
~~~

Expected: every backend package passes.

- [ ] **Step 4: Verify production frontend builds remain unchanged**

From web/default run bun run typecheck and bun run build. From web/classic run its configured lint command and bun run build. No frontend source or translation change is expected.

- [ ] **Step 5: Inspect and commit only the implementation scope**

~~~bash
git diff --check
git status --short
git diff --stat
git add model/user_session.go model/user_session_test.go service/auth_session.go service/auth_session_test.go controller/auth_session_test.go
git commit -m "fix: recover login at session limit"
~~~

Expected: only the model, service, controller regression test, and their session tests are committed.

- [ ] **Step 6: Push the exact source branch**

~~~bash
git push -u fork codex/auth-session-limit-recovery-20260810
~~~

Expected: the branch is preserved on the fork. Do not build or deploy a production candidate until the user reviews the local result.
