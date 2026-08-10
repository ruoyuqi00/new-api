# Responses Chain Affinity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep each OpenAI Responses conversation on its successful channel by using scoped conversation identifiers and the `response.id`/`previous_response_id` chain without changing frontend assets, requests, billing, or production state.

**Architecture:** Extend the existing configurable affinity source resolver with two scoped source types, then store only full SHA-256 fingerprints under token/group/model boundaries in the existing hybrid cache. Responses handlers capture a candidate upstream response ID in `RelayInfo`; the controller exposes it to the existing affinity writer only after the authoritative success gate accepts the final attempt.

**Tech Stack:** Go 1.22+, Gin, `gjson`, existing `cachex.HybridCache`, `testify/require`, `testify/assert`, Docker/Playwright for the local candidate.

---

## File Map

- Modify `setting/operation_setting/channel_affinity_setting.go`: register scoped `conversation` and `response_chain` key sources after all existing explicit sources.
- Modify `service/channel_affinity.go`: resolve scoped identifiers, build tenant-bounded SHA-256 keys, retain matched rule context, and record a successful response ID.
- Modify `service/channel_affinity_template_test.go`: protect key priority, body preservation, shared-token isolation, and response-chain behavior.
- Modify `relay/common/relay_info.go`: carry and reset the per-attempt candidate response ID.
- Modify `relay/common/relay_info_test.go`: prove retries cannot reuse a failed attempt's response ID.
- Modify `relay/channel/openai/relay_responses.go`: capture buffered and terminal-success streaming response IDs.
- Modify `relay/channel/openai/relay_responses_test.go`: prove complete replies capture IDs while incomplete or failed streams do not.
- Modify `controller/relay.go`: transfer the final candidate ID to affinity storage only inside the existing authoritative success branch.
- Modify `controller/relay_retry_test.go`: protect the controller success/failure commit boundary.
- Do not modify any file below `web/`, any database model/migration, billing calculation, Caddy, or deployment configuration.

### Task 1: Scoped Conversation Source

**Files:**
- Modify: `setting/operation_setting/channel_affinity_setting.go`
- Modify: `service/channel_affinity.go`
- Test: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Write failing conversation source tests**

Add this test helper before the new tests:

```go
func newChannelAffinityRequestContext(t *testing.T, body string, tokenID int) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyTokenId, tokenID)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "gptpro")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5")
	return ctx
}
```

Add deterministic tests that set `token_id` on Gin contexts and use unique conversation values:

```go
func TestDefaultCodexAffinityUsesScopedConversationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conversationID := fmt.Sprintf("conv-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"model":"gpt-5","conversation":{"id":"%s"},"input":"hello"}`, conversationID)

	first := newChannelAffinityRequestContext(t, body, 8101)
	channelID, found := GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
	require.False(t, found)
	require.Zero(t, channelID)
	MarkChannelAffinityRequestSucceeded(first)
	RecordChannelAffinity(first, 9101)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(first) })

	same := newChannelAffinityRequestContext(t, body, 8101)
	channelID, found = GetPreferredChannelByAffinity(same, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9101, channelID)

	otherToken := newChannelAffinityRequestContext(t, body, 8102)
	channelID, found = GetPreferredChannelByAffinity(otherToken, "gpt-5", "gptpro")
	require.False(t, found)
	assert.Zero(t, channelID)
}
```

Add this table test for both supported JSON forms:

```go
func TestDefaultCodexAffinityParsesConversationForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conversationID := fmt.Sprintf("conv-forms-%d", time.Now().UnixNano())
	tests := []struct {
		name string
		body string
	}{
		{"scalar", fmt.Sprintf(`{"model":"gpt-5","conversation":"%s","input":"hello"}`, conversationID)},
		{"object", fmt.Sprintf(`{"model":"gpt-5","conversation":{"id":"%s"},"input":"hello"}`, conversationID)},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newChannelAffinityRequestContext(t, tt.body, 8110+index)
			_, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "gptpro")
			require.False(t, found)
			meta, ok := getChannelAffinityMeta(ctx)
			require.True(t, ok)
			assert.Equal(t, "conversation", meta.KeySourceType)
			assert.Empty(t, meta.KeyHint)
			assert.NotEmpty(t, meta.KeyFingerprint)
			storage, err := common.GetBodyStorage(ctx)
			require.NoError(t, err)
			bodyAfter, err := storage.Bytes()
			require.NoError(t, err)
			assert.Equal(t, []byte(tt.body), bodyAfter)
		})
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./service -run 'TestDefaultCodexAffinityUsesScopedConversationID|TestDefaultCodexAffinityParsesConversationForms' -count=1
```

Expected: FAIL because the default rule has no `conversation` source and scoped conversation keys are not implemented.

- [ ] **Step 3: Implement the minimal scoped source resolver**

Extend the setting comment and default source order:

```go
type ChannelAffinityKeySource struct {
	Type string `json:"type"` // context_int, context_string, request_header, gjson, conversation, response_chain
	Key  string `json:"key,omitempty"`
	Path string `json:"path,omitempty"`
}
```

```go
{Type: "request_header", Key: "X-Codex-Turn-Metadata", Path: "thread_id"},
{Type: "conversation"},
{Type: "response_chain", Path: "previous_response_id"},
```

In `service/channel_affinity.go`, add a scoped-source predicate and parse only scalar conversation values or `conversation.id`:

```go
func isScopedChannelAffinitySource(sourceType string) bool {
	switch strings.TrimSpace(sourceType) {
	case "conversation", "response_chain":
		return true
	default:
		return false
	}
}
```

```go
case "conversation":
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return ""
	}
	body, err := storage.Bytes()
	if err != nil || len(body) == 0 {
		return ""
	}
	conversation := gjson.GetBytes(body, "conversation")
	if !conversation.Exists() {
		return ""
	}
	if conversation.IsObject() {
		return strings.TrimSpace(conversation.Get("id").String())
	}
	if conversation.Type == gjson.String || conversation.Type == gjson.Number {
		return strings.TrimSpace(conversation.String())
	}
	return ""
```

Build the storage suffix from a full SHA-256 hash of a length-delimited scope. The raw opaque value must not appear in the result:

```go
func buildScopedChannelAffinityCacheKeySuffix(rule operation_setting.ChannelAffinityRule, sourceType string, tokenID int, modelName, usingGroup, value string) (string, bool) {
	if tokenID <= 0 || strings.TrimSpace(value) == "" {
		return "", false
	}
	payload := fmt.Sprintf("%d\x00%d:%s\x00%d:%s\x00%d:%s", tokenID, len(usingGroup), usingGroup, len(modelName), modelName, len(value), value)
	fingerprint := fmt.Sprintf("%x", common.Sha256Raw([]byte(payload)))
	parts := make([]string, 0, 4)
	if rule.IncludeRuleName && rule.Name != "" {
		parts = append(parts, rule.Name)
	}
	parts = append(parts, "scoped-v1", sourceType, fingerprint)
	return strings.Join(parts, ":"), true
}
```

Use this suffix only for scoped source types. Keep the existing suffix builder byte-for-byte for all prior sources. Set `KeyHint` to `""` for scoped sources.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run the Step 2 command again. Expected: PASS.

- [ ] **Step 5: Run existing affinity tests**

Run:

```powershell
go test ./service -run 'ChannelAffinity|DefaultCodexAffinity' -count=1
```

Expected: PASS, including explicit `prompt_cache_key` and header priority tests.

- [ ] **Step 6: Commit Task 1**

```powershell
git add setting/operation_setting/channel_affinity_setting.go service/channel_affinity.go service/channel_affinity_template_test.go
git commit -m "feat: add scoped conversation affinity"
```

### Task 2: Response-Chain Lookup And Storage

**Files:**
- Modify: `service/channel_affinity.go`
- Test: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Write failing response-chain tests**

Add `TestResponsesChainAffinityContinuesSuccessfulChannel`:

```go
first := newChannelAffinityRequestContext(t, `{"model":"gpt-5","input":"first"}`, 8201)
_, found := GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
require.False(t, found)
SetChannelAffinityResponseID(first, "resp_chain_first")
MarkChannelAffinityRequestSucceeded(first)
RecordChannelAffinity(first, 9201)

next := newChannelAffinityRequestContext(t, `{"model":"gpt-5","previous_response_id":"resp_chain_first","input":"next"}`, 8201)
channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
require.True(t, found)
assert.Equal(t, 9201, channelID)
```

Add `TestResponsesChainAffinityScopesAndSeparatesSharedTokenChains` with subtests using response ID `resp_scope` and changing exactly one of token `8201`, group `other`, or model `gpt-5-mini`; each changed scope must miss. In the same test, record `resp_chain_a -> 9201` and `resp_chain_b -> 9202` under token `8201`, group `gptpro`, model `gpt-5`, then assert each corresponding next-turn body retrieves only its recorded channel.

Add `TestResponsesChainAffinityRequiresAuthoritativeSuccess` that sets a candidate response ID but does not call `MarkChannelAffinityRequestSucceeded`; verify the next request misses.

- [ ] **Step 2: Run response-chain tests and verify RED**

Run:

```powershell
go test ./service -run 'TestResponsesChainAffinity' -count=1
```

Expected: build FAIL because `SetChannelAffinityResponseID` does not exist.

- [ ] **Step 3: Implement matched-rule context and response-chain recording**

Add private Gin keys for the matched rule context and successful response ID. Add a small internal context type containing the matched rule, model, group, and path.

When a rule matches model/path/User-Agent and contains a `response_chain` source, retain that rule context even if the request has no current stable key. This allows the first successful turn to create the mapping without creating ordinary affinity metadata or applying parameter templates. Set `ginKeyChannelAffinityLogInfo` to an anonymous map containing `rule_name`, `using_group`, `model`, `request_path`, `key_source: "none"`, and `missing_key: true`; never include request content or identifiers.

Implement:

```go
func SetChannelAffinityResponseID(c *gin.Context, responseID string) {
	if c == nil {
		return
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return
	}
	c.Set(ginKeyChannelAffinityResponseID, responseID)
}
```

Add `response_chain` extraction using the scalar `previous_response_id` field. In `RecordChannelAffinity`, keep the existing success check, write the current request mapping when present, then independently write the scoped response-chain mapping from the retained rule context and candidate response ID. Both writes use the existing TTL calculation and `cache.SetWithTTL`.

Do not return early merely because `getChannelAffinityContext` is absent; the first turn intentionally has no current mapping but can still create a mapping for its new response ID.

- [ ] **Step 4: Run response-chain tests and verify GREEN**

Run the Step 2 command again. Expected: PASS.

- [ ] **Step 5: Run all service affinity tests**

```powershell
go test ./service -run 'ChannelAffinity|DefaultCodexAffinity|ResponsesChainAffinity' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```powershell
git add service/channel_affinity.go service/channel_affinity_template_test.go
git commit -m "feat: preserve Responses chain affinity"
```

### Task 3: Capture Only Authoritative Response IDs

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/common/relay_info_test.go`
- Modify: `relay/channel/openai/relay_responses.go`
- Modify: `relay/channel/openai/relay_responses_test.go`

- [ ] **Step 1: Write failing per-attempt reset test**

Extend `TestInitChannelMetaResetsStreamOutcomeForRetry`:

```go
info := &RelayInfo{
	ChannelAffinityResponseID:      "resp_failed_attempt",
	StreamStatus:                   NewStreamStatus(),
	StreamTerminalMarkersRequired:  true,
	StreamTerminalSuccess:          true,
	StreamTerminalUsageSeen:        true,
}
info.InitChannelMeta(c)
require.Empty(t, info.ChannelAffinityResponseID)
```

Run:

```powershell
go test ./relay/common -run TestInitChannelMetaResetsStreamOutcomeForRetry -count=1
```

Expected: build FAIL because the field is undefined.

- [ ] **Step 2: Add and reset the candidate field**

Add `ChannelAffinityResponseID string` beside the stream outcome fields in `RelayInfo`, and set it to `""` in `InitChannelMeta` before each channel attempt.

Run the Step 1 command again. Expected: PASS.

- [ ] **Step 3: Write failing buffered and streaming capture tests**

Add a buffered response test that supplies `{"id":"resp_buffered","status":"completed","usage":...}` and asserts `info.ChannelAffinityResponseID == "resp_buffered"` after `OaiResponsesHandler` succeeds.

Add a streaming table test with these exact outcomes:

```go
tests := []struct {
	name   string
	body   string
	wantID string
}{
	{"completed", "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_ok\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ok\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\ndata: [DONE]\n\n", "resp_ok"},
	{"incomplete", "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_incomplete\"}}\n\ndata: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_incomplete\"}}\n\n", ""},
	{"eof_without_terminal", "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_eof\"}}\n\n", ""},
}
```

Run:

```powershell
go test ./relay/channel/openai -run 'TestOaiResponses.*CapturesAffinityResponseID' -count=1
```

Expected: FAIL because handlers do not populate the candidate field.

- [ ] **Step 4: Implement minimal handler capture**

In `OaiResponsesHandler`, assign the parsed non-empty `responsesResponse.ID` only after JSON parsing and upstream error checks succeed.

In `OaiResponsesStreamHandler`, continue collecting `responseID` from stream events, but assign `info.ChannelAffinityResponseID = responseID` only after scanning finishes and only when `terminalReceived`, `info.StreamTerminalSuccess`, and `responseID != ""`. Do not assign it from `response.created` alone.

- [ ] **Step 5: Run handler tests and verify GREEN**

Run the Step 3 command. Expected: PASS.

Run broader Responses tests:

```powershell
go test ./relay/channel/openai -run 'Responses' -count=1
go test ./relay/common -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

```powershell
git add relay/common/relay_info.go relay/common/relay_info_test.go relay/channel/openai/relay_responses.go relay/channel/openai/relay_responses_test.go
git commit -m "feat: capture successful Responses chain ids"
```

### Task 4: Controller Success-Gate Integration

**Files:**
- Modify: `controller/relay.go`
- Modify: `controller/relay_retry_test.go`

- [ ] **Step 1: Write the failing controller boundary test**

Add a stable domain function immediately below `shouldCommitChannelAffinity` and call it from the existing successful relay branch:

```go
func commitChannelAffinityOutcome(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	if !shouldCommitChannelAffinity(c, relayInfo) {
		return
	}
	service.SetChannelAffinityResponseID(c, relayInfo.ChannelAffinityResponseID)
	service.MarkChannelAffinityRequestSucceeded(c)
}
```

Write `TestCommitResponseChainAffinityOutcome` as a table with completed, `client_gone`, context-canceled, and missing-terminal-success cases. Each case creates a first request context, calls `GetPreferredChannelByAffinity` to retain its rule, invokes `commitChannelAffinityOutcome`, then `RecordChannelAffinity`. A new request containing the candidate as `previous_response_id` must find the selected channel only in the completed case.

Run:

```powershell
go test ./controller -run 'Test.*ResponseChainAffinityCommit' -count=1
```

Expected: FAIL because the controller does not transfer the candidate ID to the service context.

- [ ] **Step 2: Wire the candidate inside the existing success gate**

Replace only the authoritative branch:

```go
commitChannelAffinityOutcome(c, relayInfo)
```

Do not add any call in error, retry preparation, incomplete-stream, or HTTP-status-only paths.

- [ ] **Step 3: Run controller tests and verify GREEN**

```powershell
go test ./controller -run 'Test.*ResponseChainAffinityCommit|TestShouldCommitChannelAffinity|TestRetry' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run cross-layer focused regression tests**

```powershell
go test ./service ./relay/common ./relay/channel/openai ./controller -run 'ChannelAffinity|ResponsesChain|OaiResponses|ShouldCommit|Tiered|Quota|Billing' -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 5: Commit Task 4**

```powershell
git add controller/relay.go controller/relay_retry_test.go
git commit -m "fix: commit Responses chain affinity after success"
```

### Task 5: Verification, Local Candidate, And Brand Gate

**Files:**
- Verify only; no expected source edits.

- [ ] **Step 1: Format and inspect the complete diff**

```powershell
gofmt -w setting/operation_setting/channel_affinity_setting.go service/channel_affinity.go service/channel_affinity_template_test.go relay/common/relay_info.go relay/common/relay_info_test.go relay/channel/openai/relay_responses.go relay/channel/openai/relay_responses_test.go controller/relay.go controller/relay_retry_test.go
git diff --check 9da9d049b..HEAD
git diff --name-only 9da9d049b..HEAD
```

Expected: no whitespace errors and no path under `web/`, `model/`, migration files, billing implementation, Caddy, or deployment configuration.

- [ ] **Step 2: Run focused behavior and billing tests fresh**

```powershell
go test ./service ./relay/common ./relay/channel/openai ./controller -run 'ChannelAffinity|ResponsesChain|OaiResponses|ShouldCommit|TextQuota|Tiered|Billing' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run the backend suite**

```powershell
go test ./... -count=1
```

Expected: exit code 0 with no package failures.

- [ ] **Step 4: Build the production-form candidate without changing frontend sources**

Build from this worktree using the repository's existing production Dockerfile and the same recorded production build inputs. Assign a new candidate-only tag. Do not overwrite either retained production image.

Inspect the candidate image history and verify the built frontend files are present. Bind the candidate only to an unused `127.0.0.1` port, using a separate container name and no Caddy labels or route changes.

- [ ] **Step 5: Verify brand and frontend parity**

Compare production and candidate homepage HTML plus static asset references. Use Playwright at desktop and mobile widths against the localhost candidate for:

- homepage;
- sign-in and sign-up;
- dashboard layout;
- API token page;
- system settings;
- infinite canvas;
- documentation;
- custom brand, motion, theme, and model configuration.

Expected: branded UI and user-visible behavior match the accepted production baseline. Any missing brand content, setup screen, default parent-project UI, broken asset, overlap, console error, or unexpected redirect rejects the candidate.

- [ ] **Step 6: Verify candidate API behavior without production traffic**

Exercise shared-token concurrent response chains against the private candidate. Verify each chain retains its own channel, complete streams record affinity, abnormal streams do not, request bodies remain byte-identical through selection, and returned usage produces the same quota as the production-baseline billing functions.

- [ ] **Step 7: Hand the localhost candidate to the user**

Report the localhost URL, image/container identifiers, test evidence, UI screenshots, exact diff scope, and known remaining limits. Do not modify Caddy or production. Wait for explicit user confirmation before any production action.
