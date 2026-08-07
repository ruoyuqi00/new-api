# Codex Channel Affinity Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use the incoming Codex `Session_id` header as an internal channel-affinity fallback when `prompt_cache_key` is absent, without changing upstream JSON or billing.

**Architecture:** Extend only the default Codex affinity rule's ordered key sources. The existing generic affinity engine continues handling key extraction, group-scoped cache keys, failover, recording, and the one-hour TTL.

**Tech Stack:** Go 1.22+, Gin, BodyStorage, hybrid Redis/in-memory cache, testify

---

### Task 1: Reproduce missing affinity for Session_id-only requests

**Files:**
- Modify: `service/channel_affinity_template_test.go`
- Test: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Add the common package import**

Add `github.com/QuantumNous/new-api/common` so the test can verify the request body through the project's BodyStorage API.

- [ ] **Step 2: Write the failing fallback behavior test**

Use the actual default `codex cli trace` rule and a unique session value:

```go
func TestDefaultCodexAffinityFallsBackToSessionIDWithoutChangingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestBody := `{"model":"gpt-5","input":"hello"}`
	sessionID := fmt.Sprintf("session-fallback-%d", time.Now().UnixNano())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Session_id", sessionID)

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.False(t, found)
	require.Zero(t, channelID)

	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "request_header", meta.KeySourceType)
	require.Equal(t, "Session_id", meta.KeySourceKey)
	require.Equal(t, 3600, meta.TTLSeconds)

	storage, err := common.GetBodyStorage(ctx)
	require.NoError(t, err)
	bodyAfter, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, []byte(requestBody), bodyAfter)

	RecordChannelAffinity(ctx, 2400)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(ctx) })

	nextRecorder := httptest.NewRecorder()
	nextCtx, _ := gin.CreateTestContext(nextRecorder)
	nextCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
	nextCtx.Request.Header.Set("Content-Type", "application/json")
	nextCtx.Request.Header.Set("Session_id", sessionID)

	channelID, found = GetPreferredChannelByAffinity(nextCtx, "gpt-5", "default")
	require.True(t, found)
	require.Equal(t, 2400, channelID)
}
```

- [ ] **Step 3: Run the fallback test and verify RED**

Run:

```powershell
go test ./service -run TestDefaultCodexAffinityFallsBackToSessionIDWithoutChangingBody -count=1
```

Expected: FAIL because no affinity metadata is created without `prompt_cache_key`.

### Task 2: Preserve prompt_cache_key priority and missing-key behavior

**Files:**
- Modify: `service/channel_affinity_template_test.go`
- Test: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Add the prompt-cache priority test**

Send both `prompt_cache_key` and `Session_id`, call `GetPreferredChannelByAffinity`, and assert metadata reports:

```go
require.Equal(t, "gjson", meta.KeySourceType)
require.Equal(t, "prompt_cache_key", meta.KeySourcePath)
require.Equal(t, affinityFingerprint(promptCacheKey), meta.KeyFingerprint)
```

Also assert the BodyStorage bytes remain exactly equal to the original request body.

- [ ] **Step 2: Add the no-key test**

Send neither field and assert `GetPreferredChannelByAffinity` returns no channel and `getChannelAffinityMeta` returns false. This prevents accidental prompt-based or user-wide fallback.

- [ ] **Step 3: Run all three behavior tests before implementation**

```powershell
go test ./service -run "TestDefaultCodexAffinity(FallsBackToSessionIDWithoutChangingBody|PrefersPromptCacheKey|RequiresExplicitKey)" -count=1
```

Expected: only the Session_id fallback test fails.

### Task 3: Add the ordered Session_id fallback

**Files:**
- Modify: `setting/operation_setting/channel_affinity_setting.go`
- Test: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Extend the default rule**

Keep the existing body source first and add only the header fallback:

```go
KeySources: []ChannelAffinityKeySource{
	{Type: "gjson", Path: "prompt_cache_key"},
	{Type: "request_header", Key: "Session_id"},
},
```

Do not change `DefaultTTLSeconds`, `SkipRetryOnFailure`, model regexes, or parameter override templates.

- [ ] **Step 2: Run the focused tests and verify GREEN**

```powershell
go test ./service -run "TestDefaultCodexAffinity|TestDefaultChannelAffinityRulesAllowFailover|TestChannelAffinityHitCodexTemplatePassHeadersEffective" -count=1
```

Expected: PASS.

- [ ] **Step 3: Run all affinity-related service tests**

```powershell
go test ./service -run "ChannelAffinity|Affinity" -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit the cache patch independently**

```powershell
git add setting/operation_setting/channel_affinity_setting.go service/channel_affinity_template_test.go
git commit -m "fix: add codex session affinity fallback"
```

### Task 4: Verify no request, billing, or database mutation

**Files:**
- Verify only

- [ ] **Step 1: Confirm scoped diff**

```powershell
git show --stat --oneline HEAD
git show --format= --name-only HEAD
```

Expected: only the default affinity setting and its service tests.

- [ ] **Step 2: Confirm no billing or database files changed**

```powershell
git diff 4d192bf88 --name-only | rg "^(service/(billing|quota|pre_consume)|model/)"
```

Expected: no output.

- [ ] **Step 3: Run the complete Go test suite after both patches**

```powershell
go test ./... -count=1
```

Expected: PASS.
