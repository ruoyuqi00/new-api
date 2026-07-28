# Production Backend Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the three proven backend reliability gaps without changing routing architecture, billing, private-group behavior, or production state.

**Architecture:** Add small contract tests at the HTTP middleware and relay-error boundaries, then remove proxy-cache resets from operations that do not change proxy configuration. Existing targeted proxy invalidation remains authoritative for proxy edits and single-channel deletion; routing and settlement suites run as a compatibility gate.

**Tech Stack:** Go 1.22+, Gin, GORM, go-redis v8, miniredis, testify, SQLite test fixtures

---

## File Map

- Create `middleware/rate_limit_test.go`: memory and Redis `429` response contracts.
- Modify `middleware/rate-limit.go`: shared `Retry-After` response writer used by all limiter paths.
- Modify `service/error_test.go`: structured-empty-message logging regression.
- Modify `service/error.go`: bounded preview logging for the empty-message branch.
- Modify `controller/channel_test_internal_test.go`: channel-status cache-stability regression.
- Modify `controller/channel_upstream_update_test.go`: model-refresh cache-stability regression.
- Create `service/codex_credential_refresh_test.go`: credential-refresh cache-stability regression.
- Modify `controller/channel.go`, `controller/channel_upstream_update.go`, `controller/codex_usage.go`, `service/codex_credential_refresh.go`, `service/codex_credential_refresh_task.go`, and `service/grok_provider_account_refresh_task.go`: remove broad resets from non-proxy mutations.
- Modify `service/provider_account_test.go`: explicit retryable transport-failure compatibility gate.
- Keep `service/http_client.go` and its targeted `InvalidateProxyClient` behavior unchanged unless a failing test exposes a defect.

### Task 1: Expose HTTP rate-limit retry timing

**Files:**
- Create: `middleware/rate_limit_test.go`
- Modify: `middleware/rate-limit.go`

- [ ] **Step 1: Write failing memory and Redis limiter tests**

Create `middleware/rate_limit_test.go` with two public-behavior cases sharing a request helper:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exerciseRateLimit(t *testing.T, limiter gin.HandlerFunc) {
	t.Helper()
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/limited", limiter, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.10:12345"
		router.ServeHTTP(recorder, req)
		return recorder
	}

	assert.Equal(t, http.StatusNoContent, request().Code)
	limited := request()
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "37", limited.Header().Get("Retry-After"))
}

func TestMemoryRateLimiterAddsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	previousLimiter := inMemoryRateLimiter
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		inMemoryRateLimiter = previousLimiter
	})

	exerciseRateLimit(t, rateLimitFactory(1, 37, "TEST_MEMORY"))
}

func TestRedisRateLimiterAddsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	exerciseRateLimit(t, rateLimitFactory(1, 37, "TEST_REDIS"))
}
```

- [ ] **Step 2: Run the tests and verify the contract is red**

Run: `go test ./middleware -run 'Test(Memory|Redis)RateLimiterAddsRetryAfter' -count=1`

Expected: both cases fail because `Retry-After` is empty.

- [ ] **Step 3: Add one response writer and use it in every limiter rejection**

Add `strconv` to `middleware/rate-limit.go`, then add:

```go
func writeRateLimited(c *gin.Context, retryAfterSeconds int64) {
	if retryAfterSeconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
	c.Status(http.StatusTooManyRequests)
	c.Abort()
}
```

Replace the repeated `StatusTooManyRequests` plus `Abort` pairs in `redisRateLimiter`, `memoryRateLimiter`, the in-memory branch of `userRateLimitFactory`, and `userRedisRateLimiter` with:

```go
writeRateLimited(c, duration)
return
```

- [ ] **Step 4: Format and verify both limiter implementations**

Run: `gofmt -w middleware/rate-limit.go middleware/rate_limit_test.go`

Run: `go test ./middleware -run 'Test(Memory|Redis)RateLimiterAddsRetryAfter' -count=1`

Expected: PASS for memory and Redis.

- [ ] **Step 5: Commit only the rate-limit contract**

```bash
git add middleware/rate-limit.go middleware/rate_limit_test.go
git commit -m "fix: expose rate limit retry windows"
```

### Task 2: Log bounded structured error bodies with empty messages

**Files:**
- Modify: `service/error_test.go`
- Modify: `service/error.go`

- [ ] **Step 1: Add the failing structured-empty-message regression**

Insert this test after `TestRelayErrorHandlerKeepsOpenAIErrorMessage`:

```go
func TestRelayErrorHandlerLogsStructuredEmptyErrorBody(t *testing.T) {
	withDebugEnabled(t, false)

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	body := `{"error":{"message":"","type":"upstream_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Contains(t, logBuffer.String(), "empty error message")
	require.Contains(t, logBuffer.String(), body)
}
```

- [ ] **Step 2: Run the single test and confirm the missing log**

Run: `go test ./service -run TestRelayErrorHandlerLogsStructuredEmptyErrorBody -count=1`

Expected: FAIL because the log buffer does not contain the empty-message diagnostic.

- [ ] **Step 3: Log only the existing bounded preview**

Replace the final `errResponse.ToMessage()` construction in `RelayErrorHandler` with:

```go
message := errResponse.ToMessage()
if message == "" {
	logger.LogError(ctx, fmt.Sprintf(
		"bad response status code %d with empty error message, body: %s",
		resp.StatusCode,
		responseBodyPreview,
	))
}
newApiErr = types.NewOpenAIError(
	errors.New(message),
	types.ErrorCodeBadResponseStatusCode,
	resp.StatusCode,
)
```

Do not log `responseBodyText`; `responseBodyPreview` preserves the existing masking and truncation boundary.

- [ ] **Step 4: Verify empty, invalid, long, and valid structured bodies together**

Run: `gofmt -w service/error.go service/error_test.go`

Run: `go test ./service -run 'TestRelayErrorHandler' -count=1`

Expected: PASS, including the existing truncation tests.

- [ ] **Step 5: Commit the relay-error diagnostic**

```bash
git add service/error.go service/error_test.go
git commit -m "fix: log empty structured upstream errors"
```

### Task 3: Keep proxy connections stable during non-proxy refreshes

**Files:**
- Modify: `controller/channel_test_internal_test.go`
- Modify: `controller/channel_upstream_update_test.go`
- Create: `service/codex_credential_refresh_test.go`
- Modify: `controller/channel.go`
- Modify: `controller/channel_upstream_update.go`
- Modify: `controller/codex_usage.go`
- Modify: `service/codex_credential_refresh.go`
- Modify: `service/codex_credential_refresh_task.go`
- Modify: `service/grok_provider_account_refresh_task.go`

- [ ] **Step 1: Add a failing channel-status connection-stability test**

Add a test that creates an enabled OpenAI channel with `http://proxy.example:8080`, obtains the cached client, calls `UpdateChannelStatus`, and expects the same client afterward:

```go
func TestUpdateChannelStatusPreservesUnchangedProxyClient(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	service.ResetProxyClientCache()
	t.Cleanup(service.ResetProxyClientCache)

	settingBytes, err := common.Marshal(dto.ChannelSettings{
		Proxy: "http://proxy.example:8080",
	})
	require.NoError(t, err)
	setting := string(settingBytes)
	channel := model.Channel{
		Type: constant.ChannelTypeOpenAI, Name: "status test", Key: "test-key",
		Models: "gpt-test", Group: "default", Status: common.ChannelStatusEnabled,
		Setting: &setting,
	}
	require.NoError(t, db.Create(&channel).Error)

	before, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	require.NoError(t, err)
	body := fmt.Sprintf(`{"status":%d}`, common.ChannelStatusManuallyDisabled)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", channel.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/channel/status", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateChannelStatus(ctx)

	after, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	require.NoError(t, err)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	assert.Same(t, before, after)
}
```

Add `strings` to the test imports.

- [ ] **Step 2: Add a failing upstream-model refresh stability test**

Append to `controller/channel_upstream_update_test.go`:

```go
func TestRefreshChannelRuntimeCachePreservesProxyClients(t *testing.T) {
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCacheEnabled })
	service.ResetProxyClientCache()
	t.Cleanup(service.ResetProxyClientCache)

	proxyURL := "http://model-refresh-proxy.example:8080"
	before, err := service.GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)

	refreshChannelRuntimeCache()

	after, err := service.GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	assert.Same(t, before, after)
}
```

- [ ] **Step 3: Add a failing Codex credential-refresh stability test**

Create `service/codex_credential_refresh_test.go`. Use an in-memory SQLite `model.Channel`, cache a proxy client, replace that cached client's `Transport` with a test `RoundTripper` returning a valid OAuth refresh response, and assert an unrelated proxy client remains identical after `RefreshCodexChannelCredential(..., ResetCaches: true)`:

```go
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRefreshCodexChannelCredentialPreservesProxyClients(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	ResetProxyClientCache()
	t.Cleanup(func() {
		ResetProxyClientCache()
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	proxyURL := "http://codex-refresh-proxy.example:8080"
	settingBytes, err := common.Marshal(dto.ChannelSettings{Proxy: proxyURL})
	require.NoError(t, err)
	setting := string(settingBytes)
	keyBytes, err := common.Marshal(CodexOAuthKey{
		AccessToken: "old-access", RefreshToken: "old-refresh", Type: "codex",
	})
	require.NoError(t, err)
	channel := model.Channel{
		Type: constant.ChannelTypeCodex, Name: "codex refresh", Key: string(keyBytes),
		Models: "gpt-test", Group: "default", Status: common.ChannelStatusEnabled,
		Setting: &setting,
	}
	require.NoError(t, db.Create(&channel).Error)

	oauthClient, err := GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	oauthClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`,
			)),
			Header: make(http.Header),
		}, nil
	})
	unrelatedURL := "http://unrelated-proxy.example:8080"
	before, err := GetHttpClientWithProxy(unrelatedURL)
	require.NoError(t, err)

	_, _, err = RefreshCodexChannelCredential(
		context.Background(), channel.Id, CodexCredentialRefreshOptions{ResetCaches: true},
	)
	require.NoError(t, err)
	after, err := GetHttpClientWithProxy(unrelatedURL)
	require.NoError(t, err)
	assert.Same(t, before, after)
}
```

Imports are `context`, `io`, `net/http`, `strings`, `testing`, project `common`, `constant`, `dto`, `model`, testify, `gorm.io/driver/sqlite`, and `gorm.io/gorm`.

- [ ] **Step 4: Run the three regressions and verify broad resets break them**

Run: `go test ./controller -run 'Test(UpdateChannelStatusPreservesUnchangedProxyClient|RefreshChannelRuntimeCachePreservesProxyClients)' -count=1`

Run: `go test ./service -run TestRefreshCodexChannelCredentialPreservesProxyClients -count=1`

Expected: the identity assertions fail while broad resets remain.

- [ ] **Step 5: Remove only non-proxy reset calls**

Delete `ResetProxyClientCache` calls from:

```text
controller/channel.go                    UpdateChannelStatus, BatchUpdateChannelStatus
controller/channel_upstream_update.go    refreshChannelRuntimeCache
controller/codex_usage.go                refreshed credential persistence
service/codex_credential_refresh.go      ResetCaches branch
service/codex_credential_refresh_task.go successful refresh batch
service/grok_provider_account_refresh_task.go successful account refresh batch
```

Keep `model.InitChannelCache()` and `model.InitAccountPoolCache()` calls. Keep broad reset fallback in `DeleteChannel` when the deleted channel cannot be pre-read, and keep deletion-path resets; deleting a channel changes the set of valid proxy owners. Keep `InvalidateProxyClient(originProxy)` in `UpdateChannel` when the normalized proxy value changes.

- [ ] **Step 6: Verify only proxy-change/deletion paths invalidate connections**

Run: `gofmt -w controller/channel.go controller/channel_test_internal_test.go controller/channel_upstream_update.go controller/channel_upstream_update_test.go controller/codex_usage.go service/codex_credential_refresh.go service/codex_credential_refresh_test.go service/codex_credential_refresh_task.go service/grok_provider_account_refresh_task.go`

Run: `go test ./controller -run 'Test(UpdateChannelStatusPreservesUnchangedProxyClient|RefreshChannelRuntimeCachePreservesProxyClients|DeleteChannelResetsProxyCacheWhenPreReadFails)' -count=1`

Run: `go test ./service -run 'Test(RefreshCodexChannelCredentialPreservesProxyClients|ProxyClientCacheCanonicalizationAndTargetedInvalidation|HTTPClientsEnableHTTP2KeepAlive)' -count=1`

Run: `rg -n 'ResetProxyClientCache|InvalidateProxyClient' controller service --glob '*.go'`

Expected: production call sites are limited to proxy edit/deletion invalidation and the cache API itself; test setup may still call full reset.

- [ ] **Step 7: Commit the proxy lifecycle fix**

```bash
git add controller/channel.go controller/channel_test_internal_test.go controller/channel_upstream_update.go controller/channel_upstream_update_test.go controller/codex_usage.go service/codex_credential_refresh.go service/codex_credential_refresh_test.go service/codex_credential_refresh_task.go service/grok_provider_account_refresh_task.go
git commit -m "fix: preserve proxy clients across channel refreshes"
```

### Task 4: Run the routing, group, mapping, and settlement compatibility gate

**Files:**
- Modify: `service/provider_account_test.go`
- Run existing model, controller, relay, and settlement tests; no production source edits expected.

- [ ] **Step 1: Add the missing transport-failure characterization test**

Append next to the existing `401`/`429`/`5xx` provider-account tests:

```go
func TestProviderAccountTransportFailureExcludesAccountWithinRequest(t *testing.T) {
	context := providerAccountTestContext()
	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 99112)
	transportErr := types.NewError(
		errors.New("connection reset by peer"),
		types.ErrorCodeDoRequestFailed,
	)

	MaybeCooldownSelectedProviderAccount(context, transportErr)

	failedAccountIDs, exists := context.Get(ginKeyFailedProviderAccountIDs)
	require.True(t, exists)
	assert.Contains(t, failedAccountIDs.(map[int]struct{}), 99112)
}
```

Run: `gofmt -w service/provider_account_test.go`

Run: `go test ./service -run TestProviderAccountTransportFailureExcludesAccountWithinRequest -count=1`

Expected: PASS because this characterizes an existing contract and does not require production code.

- [ ] **Step 2: Verify provider-account and channel retry behavior**

Run:

```bash
go test ./model -run 'Test(AcquireProviderAccountSkipsAccountsFailedEarlierInRequest|AcquireProviderAccountFallsBackToLowerPoolPriority|GetChannelWithOptionsSelectsHighestPriorityAfterExcludingFailedChannel)' -count=1
go test ./service -run 'Test(ProviderAccount429FailsOverWithinRequestAndAppliesDefaultCooldown|ProviderAccount401FailsOverWithoutDisablingAccount|ProviderAccount5xxFailsOverAndReturns503WhenPoolIsExhausted|ProviderAccountTransportFailureExcludesAccountWithinRequest|RetryParamExcludesFailedChannelWithoutSkippingNextPriority|RetryParamClearsProviderChannelWhenChannelMustFailOver|ClearCurrentChannelAffinityCache)' -count=1
```

Expected: PASS; no failed account is selected twice, exhausted pools allow channel fallback, and stale affinity does not pin retries.

- [ ] **Step 3: Verify private-group discovery and mapped-model privacy**

Run:

```bash
go test ./controller -run 'Test(GetPricingHidesPrivateGroupFromAnonymousRequest|GetPricingIncludesPrivateGroupForAuthorizedUser|GetUserModelsFiltersByRequestedGroup|GetGroupCatalogCombinesRatiosVisibilityAndCoverage)' -count=1
go test ./relay/helper ./relay/channel/openai -run 'Test(ClientResponseModelNameUsesOriginOnlyForMappedRequests|NormalizeClientResponseModelJSONRewritesOnlySupportedModelFields|.*ReturnsPublicModelForMapped.*)' -count=1
```

Expected: PASS; unauthorized users cannot discover private groups and mapped upstream names do not leak.

- [ ] **Step 4: Verify asynchronous settlement and failed-refund reconciliation**

Run:

```bash
go test ./model -run 'Test(GetUnrefundedFailedTasksFiltersLegacyAndLimits|RestoreQuotaAfterFailedRefundOnlyRestoresClaimedMarker|HasTaskPollingWorkIncludesOnlyRefundableFailedTasks)' -count=1
go test ./service -run 'Test(SweepUnrefundedFailedTasksRefundsModernTaskAndSkipsLegacy|SweepUnrefundedFailedTasksRestoresMarkerAfterFundingFailure)' -count=1
```

Expected: PASS; refund claims remain at-most-once and recoverable.

- [ ] **Step 5: Run affected packages and the repository build**

Run:

```bash
go test ./middleware ./service ./model ./controller ./relay/... -count=1
go build ./...
```

Expected: all commands exit 0. No database-specific SQL or migration changes were introduced, so SQLite/MySQL/PostgreSQL compatibility remains unchanged.

- [ ] **Step 6: Commit the compatibility test by itself**

```bash
git add service/provider_account_test.go
git commit -m "test: protect provider transport failover"
```

- [ ] **Step 7: Confirm production remains untouched**

Run: `git status --short --branch`

Expected: the three local fix commits and one compatibility-test commit are present; no push, image build, deployment, migration, SSH command, or production restart has occurred.
