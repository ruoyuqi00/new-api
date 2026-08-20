package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildChannelAffinityTemplateContextForTest(meta channelAffinityMeta) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	setChannelAffinityContext(ctx, meta)
	return ctx
}

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

func TestApplyChannelAffinityOverrideTemplate_NoTemplate(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-no-template",
	})
	base := map[string]interface{}{
		"temperature": 0.7,
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.False(t, applied)
	require.Equal(t, base, merged)
}

func TestApplyChannelAffinityOverrideTemplate_MergeTemplate(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-with-template",
		ParamTemplate: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.95,
		},
		UsingGroup:     "default",
		ModelName:      "gpt-4.1",
		RequestPath:    "/v1/responses",
		KeySourceType:  "gjson",
		KeySourcePath:  "prompt_cache_key",
		KeyHint:        "abcd...wxyz",
		KeyFingerprint: "abcd1234",
	})
	base := map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.True(t, applied)
	require.Equal(t, 0.7, merged["temperature"])
	require.Equal(t, 0.95, merged["top_p"])
	require.Equal(t, 2000, merged["max_tokens"])
	require.Equal(t, 0.7, base["temperature"])

	anyInfo, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info, ok := anyInfo.(map[string]interface{})
	require.True(t, ok)
	overrideInfoAny, ok := info["override_template"]
	require.True(t, ok)
	overrideInfo, ok := overrideInfoAny.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, overrideInfo["applied"])
	require.Equal(t, "rule-with-template", overrideInfo["rule_name"])
	require.EqualValues(t, 2, overrideInfo["param_override_keys"])
}

func TestApplyChannelAffinityOverrideTemplate_MergeOperations(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-with-ops-template",
		ParamTemplate: map[string]interface{}{
			"operations": []map[string]interface{}{
				{
					"mode":  "pass_headers",
					"value": []string{"Originator"},
				},
			},
		},
	})
	base := map[string]interface{}{
		"temperature": 0.7,
		"operations": []map[string]interface{}{
			{
				"path":  "model",
				"mode":  "trim_prefix",
				"value": "openai/",
			},
		},
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.True(t, applied)
	require.Equal(t, 0.7, merged["temperature"])

	opsAny, ok := merged["operations"]
	require.True(t, ok)
	ops, ok := opsAny.([]interface{})
	require.True(t, ok)
	require.Len(t, ops, 2)

	firstOp, ok := ops[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "pass_headers", firstOp["mode"])

	secondOp, ok := ops[1].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "trim_prefix", secondOp["mode"])
}

func TestShouldSkipRetryAfterChannelAffinityFailure(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() *gin.Context
		want bool
	}{
		{
			name: "nil context",
			ctx: func() *gin.Context {
				return nil
			},
			want: false,
		},
		{
			name: "explicit skip retry flag in context",
			ctx: func() *gin.Context {
				ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-explicit-flag",
					SkipRetry:  false,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
				ctx.Set(ginKeyChannelAffinitySkipRetry, true)
				return ctx
			},
			want: true,
		},
		{
			name: "fallback to matched rule meta",
			ctx: func() *gin.Context {
				return buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-skip-retry",
					SkipRetry:  true,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
			},
			want: true,
		},
		{
			name: "no flag and no skip retry meta",
			ctx: func() *gin.Context {
				return buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-no-skip-retry",
					SkipRetry:  false,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ShouldSkipRetryAfterChannelAffinityFailure(tt.ctx()))
		})
	}
}

func TestDefaultChannelAffinityRulesAllowFailover(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	requiredRules := map[string]bool{
		"codex cli trace":  false,
		"claude cli trace": false,
	}

	for _, rule := range setting.Rules {
		if _, required := requiredRules[rule.Name]; !required {
			continue
		}
		require.False(t, rule.SkipRetryOnFailure, "default affinity rule %q must allow failover", rule.Name)
		requiredRules[rule.Name] = true
	}

	for name, found := range requiredRules {
		require.True(t, found, "default affinity rule %q not found", name)
	}
}

func TestExtractChannelAffinityValue_RequestHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("X-Affinity-Key", " tenant-123 ")

	value := extractChannelAffinityValue(ctx, operation_setting.ChannelAffinityKeySource{
		Type: "request_header",
		Key:  "X-Affinity-Key",
	})

	require.Equal(t, "tenant-123", value)
}

func TestExtractChannelAffinityValue_RequestHeaderJSONPath(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", `{"session_id":"session-123","turn_id":"turn-456"}`)

	value := extractChannelAffinityValue(ctx, operation_setting.ChannelAffinityKeySource{
		Type: "request_header",
		Key:  "X-Codex-Turn-Metadata",
		Path: "session_id",
	})

	require.Equal(t, "session-123", value)
}

func TestGetPreferredChannelByAffinity_RequestHeaderKeySource(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rule := operation_setting.ChannelAffinityRule{
		Name:       "header-affinity",
		ModelRegex: []string{"^gpt-.*$"},
		PathRegex:  []string{"/v1/responses"},
		KeySources: []operation_setting.ChannelAffinityKeySource{
			{Type: "request_header", Key: "X-Affinity-Key"},
		},
		IncludeRuleName:  true,
		IncludeModelName: true,
	}

	affinityValue := fmt.Sprintf("header-hit-%d", time.Now().UnixNano())
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(rule, "gpt-5", "default", affinityValue)

	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9528, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	setting := operation_setting.GetChannelAffinitySetting()
	originalRules := setting.Rules
	setting.Rules = append([]operation_setting.ChannelAffinityRule{rule}, originalRules...)
	t.Cleanup(func() {
		setting.Rules = originalRules
	})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("X-Affinity-Key", affinityValue)

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.True(t, found)
	require.Equal(t, 9528, channelID)

	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "request_header", meta.KeySourceType)
	require.Equal(t, "X-Affinity-Key", meta.KeySourceKey)
	require.Equal(t, buildChannelAffinityKeyHint(affinityValue), meta.KeyHint)
}

func TestChannelAffinityPromptCacheKeyIsStableAndScoped(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	originalRules := setting.Rules
	setting.Rules = []operation_setting.ChannelAffinityRule{
		{
			Name:                 "prompt-cache-session",
			ModelRegex:           []string{"^gpt-.*$"},
			PathRegex:            []string{"^/v1/responses$"},
			KeySources:           []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
			InjectPromptCacheKey: true,
			IncludeUsingGroup:    true,
			IncludeModelName:     true,
			IncludeRuleName:      true,
		},
	}
	t.Cleanup(func() { setting.Rules = originalRules })

	newContext := func(tokenID int, usingGroup string) *gin.Context {
		ctx := newChannelAffinityRequestContext(t, `{"model":"gpt-5","input":"hello"}`, tokenID)
		ctx.Request.Header.Set("Session_id", "raw-session-123")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, usingGroup)
		_, found := GetPreferredChannelByAffinity(ctx, "gpt-5", usingGroup)
		require.False(t, found)
		return ctx
	}

	firstContext := newContext(8301, "gptpro")
	meta, hasMeta := getChannelAffinityMeta(firstContext)
	require.True(t, hasMeta)
	assert.Empty(t, meta.KeyHint)
	assert.NotContains(t, meta.CacheKey, "raw-session-123")
	adminInfo := map[string]interface{}{}
	AppendChannelAffinityAdminInfo(firstContext, adminInfo)
	assert.NotContains(t, fmt.Sprint(adminInfo), "raw-session-123")

	first, ok := GetChannelAffinityPromptCacheKey(firstContext)
	require.True(t, ok)
	second, ok := GetChannelAffinityPromptCacheKey(newContext(8301, "gptpro"))
	require.True(t, ok)
	otherToken, ok := GetChannelAffinityPromptCacheKey(newContext(8302, "gptpro"))
	require.True(t, ok)
	otherGroup, ok := GetChannelAffinityPromptCacheKey(newContext(8301, "other"))
	require.True(t, ok)

	assert.Equal(t, first, second)
	assert.NotEqual(t, first, otherToken)
	assert.NotEqual(t, first, otherGroup)
	assert.NotContains(t, first, "raw-session-123")
	assert.True(t, strings.HasPrefix(first, "yuapi-pck-v1-"))
}

func TestChannelAffinityPromptCacheKeySupportsChatCompletions(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	originalRules := setting.Rules
	setting.Rules = []operation_setting.ChannelAffinityRule{
		{
			Name:                 "gpt-text-prompt-cache-session",
			ModelRegex:           []string{"^gpt-.*$"},
			PathRegex:            []string{"^/v1/responses$", "^/v1/chat/completions$"},
			KeySources:           []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
			InjectPromptCacheKey: true,
			IncludeUsingGroup:    true,
			IncludeModelName:     true,
			IncludeRuleName:      true,
		},
	}
	t.Cleanup(func() { setting.Rules = originalRules })

	newContext := func(tokenID int) *gin.Context {
		ctx := newChannelAffinityRequestContext(t, `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`, tokenID)
		ctx.Request.URL.Path = "/v1/chat/completions"
		ctx.Request.Header.Set("Session_id", "raw-chat-session-123")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "gptpro")
		return ctx
	}

	ctx := newContext(8351)

	_, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "gptpro")
	require.False(t, found)

	key, ok := GetChannelAffinityPromptCacheKey(ctx)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(key, "yuapi-pck-v1-"))
	assert.NotContains(t, key, "raw-chat-session-123")
	adminInfo := map[string]interface{}{}
	AppendChannelAffinityAdminInfo(ctx, adminInfo)
	assert.NotContains(t, fmt.Sprint(adminInfo), "raw-chat-session-123")

	MarkChannelAffinityRequestSucceeded(ctx)
	RecordChannelAffinity(ctx, 2451)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(ctx) })

	sameSession := newContext(8351)
	channelID, found := GetPreferredChannelByAffinity(sameSession, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 2451, channelID)

	otherToken := newContext(8352)
	channelID, found = GetPreferredChannelByAffinity(otherToken, "gpt-5", "gptpro")
	assert.False(t, found)
	assert.Zero(t, channelID)
}

func TestChannelAffinityPromptCacheKeyRequiresSafeSourceAndOptIn(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		body    string
		tokenID int
		enabled bool
		sources []operation_setting.ChannelAffinityKeySource
		headers map[string]string
	}{
		{
			name: "disabled rule", path: "/v1/responses", body: `{"model":"gpt-5"}`, tokenID: 8401,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
			headers: map[string]string{"Session_id": "session-disabled"},
		},
		{
			name: "explicit prompt cache key", path: "/v1/responses", body: `{"model":"gpt-5","prompt_cache_key":"client-key"}`, tokenID: 8402, enabled: true,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "gjson", Path: "prompt_cache_key"}, {Type: "request_header", Key: "Session_id"}},
			headers: map[string]string{"Session_id": "session-explicit"},
		},
		{
			name: "response chain", path: "/v1/responses", body: `{"model":"gpt-5","previous_response_id":"resp-existing"}`, tokenID: 8403, enabled: true,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "context_string", Key: operation_setting.ChannelAffinityResponseChainContextKey}},
		},
		{
			name: "unsupported path", path: "/v1/images/generations", body: `{"model":"gpt-5"}`, tokenID: 8404, enabled: true,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
			headers: map[string]string{"Session_id": "session-path"},
		},
		{
			name: "missing token", path: "/v1/responses", body: `{"model":"gpt-5"}`, enabled: true,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
			headers: map[string]string{"Session_id": "session-no-token"},
		},
		{
			name: "missing stable source", path: "/v1/responses", body: `{"model":"gpt-5"}`, tokenID: 8405, enabled: true,
			sources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := operation_setting.GetChannelAffinitySetting()
			originalRules := setting.Rules
			setting.Rules = []operation_setting.ChannelAffinityRule{{
				Name:                 "prompt-cache-negative",
				ModelRegex:           []string{"^gpt-.*$"},
				PathRegex:            []string{"^/v1/responses$"},
				KeySources:           tt.sources,
				InjectPromptCacheKey: tt.enabled,
			}}
			t.Cleanup(func() { setting.Rules = originalRules })

			ctx := newChannelAffinityRequestContext(t, tt.body, tt.tokenID)
			ctx.Request.URL.Path = tt.path
			for key, value := range tt.headers {
				ctx.Request.Header.Set(key, value)
			}
			_, _ = GetPreferredChannelByAffinity(ctx, "gpt-5", "gptpro")
			key, ok := GetChannelAffinityPromptCacheKey(ctx)
			assert.False(t, ok)
			assert.Empty(t, key)
		})
	}
}

func TestDefaultCodexAffinityEnablesPromptCacheKeyInjection(t *testing.T) {
	rules := operation_setting.GetChannelAffinitySetting().Rules
	require.NotEmpty(t, rules)
	assert.Equal(t, "codex cli trace", rules[0].Name)
	assert.True(t, rules[0].InjectPromptCacheKey)
	assert.True(t, matchAnyRegexCached(rules[0].PathRegex, "/v1/responses"))
	assert.True(t, matchAnyRegexCached(rules[0].PathRegex, "/v1/chat/completions"))
	assert.False(t, matchAnyRegexCached(rules[0].PathRegex, "/v1/images/generations"))
	assert.False(t, operation_setting.ChannelAffinityRule{}.InjectPromptCacheKey)
}

func TestClearCurrentChannelAffinityCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheKeySuffix := fmt.Sprintf("codex cli trace:default:clear-current-%d", time.Now().UnixNano())
	cacheKeyFull := channelAffinityCacheNamespace + ":" + cacheKeySuffix
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9527, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		CacheKey:   cacheKeyFull,
		TTLSeconds: 60,
		RuleName:   "codex cli trace",
		SkipRetry:  true,
	})
	require.True(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))

	deleted := ClearCurrentChannelAffinityCache(ctx)
	require.True(t, deleted)
	_, found, err := cache.Get(cacheKeySuffix)
	require.NoError(t, err)
	require.False(t, found)
	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}

func TestRecordChannelAffinityRequiresConfirmedRelaySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheKeySuffix := fmt.Sprintf("codex cli trace:default:unconfirmed-%d", time.Now().UnixNano())
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9527, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		CacheKey:   channelAffinityCacheNamespace + ":" + cacheKeySuffix,
		TTLSeconds: 60,
	})

	RecordChannelAffinity(ctx, 9528)

	channelID, found, err := cache.Get(cacheKeySuffix)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 9527, channelID)
}

func TestRecordChannelAffinityCommitsConfirmedRelaySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheKeySuffix := fmt.Sprintf("codex cli trace:default:confirmed-%d", time.Now().UnixNano())
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9527, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		CacheKey:   channelAffinityCacheNamespace + ":" + cacheKeySuffix,
		TTLSeconds: 60,
	})
	MarkChannelAffinityRequestSucceeded(ctx)

	RecordChannelAffinity(ctx, 9528)

	channelID, found, err := cache.Get(cacheKeySuffix)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 9528, channelID)
}

func TestChannelAffinityHitCodexTemplatePassHeadersEffective(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, setting)

	var codexRule *operation_setting.ChannelAffinityRule
	for i := range setting.Rules {
		rule := &setting.Rules[i]
		if strings.EqualFold(strings.TrimSpace(rule.Name), "codex cli trace") {
			codexRule = rule
			break
		}
	}
	require.NotNil(t, codexRule)

	affinityValue := fmt.Sprintf("pc-hit-%d", time.Now().UnixNano())
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(*codexRule, "gpt-5", "default", affinityValue)

	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9527, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.True(t, found)
	require.Equal(t, 9527, channelID)

	baseOverride := map[string]interface{}{
		"temperature": 0.2,
	}
	mergedOverride, applied := ApplyChannelAffinityOverrideTemplate(ctx, baseOverride)
	require.True(t, applied)
	require.Equal(t, 0.2, mergedOverride["temperature"])

	info := &relaycommon.RelayInfo{
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
			"User-Agent": "codex-cli-test",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: mergedOverride,
			HeadersOverride: map[string]interface{}{
				"X-Static": "legacy-static",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-5"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)

	require.Equal(t, "legacy-static", info.RuntimeHeadersOverride["x-static"])
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	require.Equal(t, "codex-cli-test", info.RuntimeHeadersOverride["user-agent"])

	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	_, exists = info.RuntimeHeadersOverride["x-codex-turn-metadata"]
	require.False(t, exists)
}

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

	MarkChannelAffinityRequestSucceeded(ctx)
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

func TestDefaultCodexAffinityPrefersPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	promptCacheKey := fmt.Sprintf("prompt-cache-%d", time.Now().UnixNano())
	requestBody := fmt.Sprintf(`{"model":"gpt-5","prompt_cache_key":"%s","input":"hello"}`, promptCacheKey)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Session_id", "different-session")

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.False(t, found)
	require.Zero(t, channelID)

	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "gjson", meta.KeySourceType)
	require.Equal(t, "prompt_cache_key", meta.KeySourcePath)
	require.Equal(t, affinityFingerprint(promptCacheKey), meta.KeyFingerprint)

	storage, err := common.GetBodyStorage(ctx)
	require.NoError(t, err)
	bodyAfter, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, []byte(requestBody), bodyAfter)
}

func TestDefaultCodexAffinityFallsBackToTurnMetadataSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", `{"session_id":"session-from-metadata","turn_id":"changes-each-turn"}`)

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")

	require.False(t, found)
	require.Zero(t, channelID)
	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "request_header", meta.KeySourceType)
	require.Equal(t, "X-Codex-Turn-Metadata", meta.KeySourceKey)
	require.Equal(t, "session_id", meta.KeySourcePath)
	require.Equal(t, affinityFingerprint("session-from-metadata"), meta.KeyFingerprint)
}

func TestDefaultCodexAffinityFallsBackToTurnMetadataThreadID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", `{"thread_id":"thread-from-metadata","turn_id":"changes-each-turn"}`)

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")

	require.False(t, found)
	require.Zero(t, channelID)
	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "thread_id", meta.KeySourcePath)
	require.Equal(t, affinityFingerprint("thread-from-metadata"), meta.KeyFingerprint)
}

func TestDefaultCodexAffinityRequiresExplicitKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")

	require.False(t, found)
	require.Zero(t, channelID)
	_, hasMeta := getChannelAffinityMeta(ctx)
	require.False(t, hasMeta)
}

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

func recordResponseChainAffinityForTest(t *testing.T, responseID string, tokenID int, modelName string, usingGroup string, channelID int) {
	t.Helper()
	ctx := newChannelAffinityRequestContext(t, `{"model":"gpt-5","input":"first"}`, tokenID)
	_, found := GetPreferredChannelByAffinity(ctx, modelName, usingGroup)
	require.False(t, found)
	SetChannelAffinityResponseID(ctx, responseID)
	MarkChannelAffinityRequestSucceeded(ctx)
	RecordChannelAffinity(ctx, channelID)
}

func responseChainLookupContext(t *testing.T, responseID string, tokenID int) *gin.Context {
	t.Helper()
	body := fmt.Sprintf(`{"model":"gpt-5","previous_response_id":"%s","input":"next"}`, responseID)
	return newChannelAffinityRequestContext(t, body, tokenID)
}

func TestResponsesChainAffinityContinuesSuccessfulChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseID := fmt.Sprintf("resp-chain-%d", time.Now().UnixNano())
	recordResponseChainAffinityForTest(t, responseID, 8201, "gpt-5", "gptpro", 9201)

	next := responseChainLookupContext(t, responseID, 8201)
	channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9201, channelID)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(next) })
}

func TestProvisionalResponseChainAffinityContinuesInterruptedChannelAndPrimaryIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseID := fmt.Sprintf("resp-provisional-%d", time.Now().UnixNano())
	tokenID := 8202
	first := newChannelAffinityRequestContext(t, `{"model":"gpt-5","prompt_cache_key":"primary-key","input":"first"}`, tokenID)
	_, found := GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
	require.False(t, found)

	RecordProvisionalResponseChainAffinity(first, 9202, responseID)

	next := responseChainLookupContext(t, responseID, tokenID)
	channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9202, channelID)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(next) })

	primary := newChannelAffinityRequestContext(t, `{"model":"gpt-5","prompt_cache_key":"primary-key","input":"next"}`, tokenID)
	channelID, found = GetPreferredChannelByAffinity(primary, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9202, channelID)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(primary) })
}

func TestResponsesAffinityCascadePrefersResponseChainOverPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suffix := time.Now().UnixNano()
	responseID := fmt.Sprintf("resp-cascade-%d", suffix)
	promptCacheKey := fmt.Sprintf("prompt-cascade-%d", suffix)
	tokenID := 8250
	recordResponseChainAffinityForTest(t, responseID, tokenID, "gpt-5", "gptpro", 9250)

	body := fmt.Sprintf(`{"model":"gpt-5","prompt_cache_key":"%s","previous_response_id":"%s","input":"next"}`, promptCacheKey, responseID)
	next := newChannelAffinityRequestContext(t, body, tokenID)
	channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9250, channelID)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(next) })
}

func TestResponsesAffinityCascadePrioritizesConversationOverPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suffix := time.Now().UnixNano()
	conversationID := fmt.Sprintf("conversation-hit-%d", suffix)
	promptCacheKey := fmt.Sprintf("prompt-hit-%d", suffix)
	tokenID := 8251

	conversationSeed := newChannelAffinityRequestContext(t, fmt.Sprintf(`{"model":"gpt-5","conversation":{"id":"%s"},"input":"first"}`, conversationID), tokenID)
	_, found := GetPreferredChannelByAffinity(conversationSeed, "gpt-5", "gptpro")
	require.False(t, found)
	MarkChannelAffinityRequestSucceeded(conversationSeed)
	RecordChannelAffinity(conversationSeed, 9251)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(conversationSeed) })

	promptSeed := newChannelAffinityRequestContext(t, fmt.Sprintf(`{"model":"gpt-5","prompt_cache_key":"%s","input":"first"}`, promptCacheKey), tokenID)
	_, found = GetPreferredChannelByAffinity(promptSeed, "gpt-5", "gptpro")
	require.False(t, found)
	MarkChannelAffinityRequestSucceeded(promptSeed)
	RecordChannelAffinity(promptSeed, 9252)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(promptSeed) })

	body := fmt.Sprintf(`{"model":"gpt-5","conversation":{"id":"%s"},"prompt_cache_key":"%s","input":"next"}`, conversationID, promptCacheKey)
	next := newChannelAffinityRequestContext(t, body, tokenID)
	channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9251, channelID)
}

func TestProvisionalResponseChainAffinityAlsoRecordsPrimaryIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suffix := time.Now().UnixNano()
	responseID := fmt.Sprintf("resp-provisional-primary-%d", suffix)
	promptCacheKey := fmt.Sprintf("prompt-provisional-primary-%d", suffix)
	tokenID := 8252
	first := newChannelAffinityRequestContext(t, fmt.Sprintf(`{"model":"gpt-5","prompt_cache_key":"%s","input":"first"}`, promptCacheKey), tokenID)
	_, found := GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
	require.False(t, found)

	RecordProvisionalResponseChainAffinity(first, 9252, responseID)

	primary := newChannelAffinityRequestContext(t, fmt.Sprintf(`{"model":"gpt-5","prompt_cache_key":"%s","input":"next"}`, promptCacheKey), tokenID)
	channelID, found := GetPreferredChannelByAffinity(primary, "gpt-5", "gptpro")
	require.True(t, found)
	assert.Equal(t, 9252, channelID)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(primary) })
}

func TestResponsesChainAffinityScopesAndSeparatesSharedTokenChains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suffix := time.Now().UnixNano()
	scopedResponseID := fmt.Sprintf("resp-scope-%d", suffix)
	recordResponseChainAffinityForTest(t, scopedResponseID, 8210, "gpt-5", "gptpro", 9210)

	tests := []struct {
		name       string
		tokenID    int
		modelName  string
		usingGroup string
	}{
		{"other token", 8211, "gpt-5", "gptpro"},
		{"other group", 8210, "gpt-5", "other"},
		{"other model", 8210, "gpt-5-mini", "gptpro"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := responseChainLookupContext(t, scopedResponseID, tt.tokenID)
			channelID, found := GetPreferredChannelByAffinity(ctx, tt.modelName, tt.usingGroup)
			require.False(t, found)
			assert.Zero(t, channelID)
		})
	}

	chainA := fmt.Sprintf("resp-chain-a-%d", suffix)
	chainB := fmt.Sprintf("resp-chain-b-%d", suffix)
	recordResponseChainAffinityForTest(t, chainA, 8220, "gpt-5", "gptpro", 9221)
	recordResponseChainAffinityForTest(t, chainB, 8220, "gpt-5", "gptpro", 9222)

	lookupA := responseChainLookupContext(t, chainA, 8220)
	channelA, foundA := GetPreferredChannelByAffinity(lookupA, "gpt-5", "gptpro")
	require.True(t, foundA)
	assert.Equal(t, 9221, channelA)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(lookupA) })

	lookupB := responseChainLookupContext(t, chainB, 8220)
	channelB, foundB := GetPreferredChannelByAffinity(lookupB, "gpt-5", "gptpro")
	require.True(t, foundB)
	assert.Equal(t, 9222, channelB)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(lookupB) })
}

func TestResponsesChainAffinityRequiresAuthoritativeSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseID := fmt.Sprintf("resp-unconfirmed-%d", time.Now().UnixNano())
	first := newChannelAffinityRequestContext(t, `{"model":"gpt-5","input":"first"}`, 8230)
	_, found := GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
	require.False(t, found)
	SetChannelAffinityResponseID(first, responseID)
	RecordChannelAffinity(first, 9230)

	next := responseChainLookupContext(t, responseID, 8230)
	channelID, found := GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
	require.False(t, found)
	assert.Zero(t, channelID)
}

func TestDefaultCodexAffinityMarksMissingStableKeyWithoutIdentifiers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newChannelAffinityRequestContext(t, `{"model":"gpt-5","input":"hello"}`, 8240)

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "gptpro")
	require.False(t, found)
	assert.Zero(t, channelID)

	adminInfo := map[string]interface{}{}
	AppendChannelAffinityAdminInfo(ctx, adminInfo)
	affinityInfo, ok := adminInfo["channel_affinity"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, affinityInfo["missing_key"])
	assert.Equal(t, "none", affinityInfo["key_source"])
	assert.NotContains(t, affinityInfo, "key_hint")
}
