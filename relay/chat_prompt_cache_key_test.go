package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatHelperInjectsScopedPromptCacheKey(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	globalSettings := model_setting.GetGlobalSettings()
	originalPassThrough := globalSettings.PassThroughRequestEnabled
	originalCompatibilityPolicy := globalSettings.ChatCompletionsToResponsesPolicy
	globalSettings.PassThroughRequestEnabled = false
	globalSettings.ChatCompletionsToResponsesPolicy.Enabled = false
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = originalPassThrough
		globalSettings.ChatCompletionsToResponsesPolicy = originalCompatibilityPolicy
	})

	tests := []struct {
		name         string
		body         string
		sessionID    string
		passThrough  bool
		wantInjected bool
		wantExplicit string
		wantUnknown  bool
	}{
		{
			name:         "converted request",
			body:         `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`,
			sessionID:    "stable-chat-session-converted",
			wantInjected: true,
		},
		{
			name:         "raw passthrough preserves unknown fields",
			body:         `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"unknown_passthrough_field":"kept"}`,
			sessionID:    "stable-chat-session-passthrough",
			passThrough:  true,
			wantInjected: true,
			wantUnknown:  true,
		},
		{
			name:         "explicit client key wins",
			body:         `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"prompt_cache_key":"client-key"}`,
			sessionID:    "stable-chat-session-explicit",
			wantExplicit: "client-key",
		},
		{
			name: "missing stable source",
			body: `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`,
		},
	}
	privateUpstreamMessage := "POST https://private-upstream.example/v1 via 10.20.30.40 Authorization Bearer sk-private raw-body"
	type capturedRequest struct {
		body map[string]any
		err  error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := operation_setting.GetChannelAffinitySetting()
			originalRules := setting.Rules
			setting.Rules = []operation_setting.ChannelAffinityRule{{
				Name:                 "chat-prompt-cache-test",
				ModelRegex:           []string{"^gpt-.*$"},
				PathRegex:            []string{"^/v1/chat/completions$"},
				KeySources:           []operation_setting.ChannelAffinityKeySource{{Type: "gjson", Path: "prompt_cache_key"}, {Type: "request_header", Key: "Session_id"}},
				InjectPromptCacheKey: true,
				IncludeUsingGroup:    true,
				IncludeModelName:     true,
				IncludeRuleName:      true,
			}}
			t.Cleanup(func() { setting.Rules = originalRules })

			upstreamRequest := make(chan capturedRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				decodeErr := common.DecodeJson(r.Body, &body)
				upstreamRequest <- capturedRequest{body: body, err: decodeErr}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"` + privateUpstreamMessage + `","type":"invalid_request_error"}}`))
			}))
			t.Cleanup(server.Close)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			if tt.sessionID != "" {
				c.Request.Header.Set("Session_id", tt.sessionID)
			}
			common.SetContextKey(c, constant.ContextKeyTokenId, 8751)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "gptpro")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
			common.SetContextKey(c, constant.ContextKeyChannelId, 9751)
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tt.passThrough})

			var request dto.GeneralOpenAIRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			_, found := service.GetPreferredChannelByAffinity(c, request.Model, "gptpro")
			require.False(t, found)
			expectedDerived, derived := service.GetChannelAffinityPromptCacheKey(c)
			require.Equal(t, tt.wantInjected, derived)

			info := relaycommon.GenRelayInfoOpenAI(c, &request)
			newAPIError := TextHelper(c, info)
			require.NotNil(t, newAPIError)
			publicError := newAPIError.ToPublicOpenAIError("req-chat-cache")
			for _, privateValue := range []string{"private-upstream.example", "10.20.30.40", "Authorization", "sk-private", "raw-body"} {
				assert.NotContains(t, publicError.Message, privateValue)
			}

			captured := <-upstreamRequest
			require.NoError(t, captured.err)
			body := captured.body
			gotKey, hasKey := body["prompt_cache_key"].(string)
			switch {
			case tt.wantInjected:
				require.True(t, hasKey)
				assert.Equal(t, expectedDerived, gotKey)
				assert.NotContains(t, gotKey, tt.sessionID)
			case tt.wantExplicit != "":
				require.True(t, hasKey)
				assert.Equal(t, tt.wantExplicit, gotKey)
			default:
				assert.False(t, hasKey)
			}
			if tt.wantUnknown {
				assert.Equal(t, "kept", body["unknown_passthrough_field"])
			}
		})
	}
}
