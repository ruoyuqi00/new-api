package relay

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nonMaterializingBodyStorage struct {
	data       []byte
	reader     *bytes.Reader
	bytesCalls atomic.Int64
}

func newNonMaterializingBodyStorage(data []byte) *nonMaterializingBodyStorage {
	return &nonMaterializingBodyStorage{data: data, reader: bytes.NewReader(data)}
}

func (s *nonMaterializingBodyStorage) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *nonMaterializingBodyStorage) Seek(offset int64, whence int) (int64, error) {
	return s.reader.Seek(offset, whence)
}

func (s *nonMaterializingBodyStorage) Close() error {
	return nil
}

func (s *nonMaterializingBodyStorage) Bytes() ([]byte, error) {
	s.bytesCalls.Add(1)
	return nil, errors.New("body must remain stream-backed")
}

func (s *nonMaterializingBodyStorage) Size() int64 {
	return int64(len(s.data))
}

func (s *nonMaterializingBodyStorage) IsDisk() bool {
	return true
}

func (s *nonMaterializingBodyStorage) NewReader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func TestResponsesHelperInjectsScopedPromptCacheKey(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		body             string
		sessionID        string
		passThrough      bool
		injectionEnabled bool
		wantInjected     bool
		wantExplicit     string
		wantUnknown      bool
	}{
		{
			name:             "converted request",
			body:             `{"model":"gpt-test","input":"hello"}`,
			sessionID:        "stable-session-converted",
			injectionEnabled: true,
			wantInjected:     true,
		},
		{
			name:             "raw passthrough preserves unknown fields",
			body:             `{"model":"gpt-test","input":"hello","unknown_passthrough_field":"kept"}`,
			sessionID:        "stable-session-passthrough",
			passThrough:      true,
			injectionEnabled: true,
			wantInjected:     true,
			wantUnknown:      true,
		},
		{
			name:             "explicit client key wins",
			body:             `{"model":"gpt-test","input":"hello","prompt_cache_key":"client-key"}`,
			sessionID:        "stable-session-explicit",
			passThrough:      true,
			injectionEnabled: true,
			wantExplicit:     "client-key",
		},
		{
			name:             "disabled rule",
			body:             `{"model":"gpt-test","input":"hello"}`,
			sessionID:        "stable-session-disabled",
			injectionEnabled: false,
		},
		{
			name:             "missing stable source",
			body:             `{"model":"gpt-test","input":"hello"}`,
			injectionEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := operation_setting.GetChannelAffinitySetting()
			originalRules := setting.Rules
			setting.Rules = []operation_setting.ChannelAffinityRule{{
				Name:                 "responses-prompt-cache-test",
				ModelRegex:           []string{"^gpt-.*$"},
				PathRegex:            []string{"^/v1/responses$"},
				KeySources:           []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
				InjectPromptCacheKey: tt.injectionEnabled,
				IncludeUsingGroup:    true,
				IncludeModelName:     true,
				IncludeRuleName:      true,
			}}
			t.Cleanup(func() { setting.Rules = originalRules })

			upstreamBody := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				upstreamBody <- body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"fixture rejected after capture","type":"invalid_request_error"}}`))
			}))
			t.Cleanup(server.Close)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			if tt.sessionID != "" {
				c.Request.Header.Set("Session_id", tt.sessionID)
			}
			common.SetContextKey(c, constant.ContextKeyTokenId, 8501)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "gptpro")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tt.passThrough})

			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			_, found := service.GetPreferredChannelByAffinity(c, request.Model, "gptpro")
			require.False(t, found)
			expectedDerived, derived := service.GetChannelAffinityPromptCacheKey(c)
			require.Equal(t, tt.wantInjected, derived)

			info := relaycommon.GenRelayInfoResponses(c, &request)
			newAPIError := ResponsesHelper(c, info)
			require.NotNil(t, newAPIError)

			body := <-upstreamBody
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

func TestResponsesHelperMarksEmbeddedUpstreamErrorForPublicProjection(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	privateMessage := "POST https://private-upstream.example/v1 IP 10.0.0.8 Authorization Bearer sk-private raw-body"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"message":"` + privateMessage + `","type":"server_error","code":"server_error"}}`))
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test","input":"hello"}`))
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")

	request := &dto.OpenAIResponsesRequest{Model: "gpt-test", Input: []byte(`"hello"`)}
	info := relaycommon.GenRelayInfoResponses(c, request)
	newAPIError := ResponsesHelper(c, info)
	require.NotNil(t, newAPIError)
	require.Equal(t, privateMessage, newAPIError.Error())

	public := newAPIError.ToPublicOpenAIError("req-embedded")
	assert.Equal(t, "upstream_error", public.Type)
	assert.Contains(t, public.Message, "req-embedded")
	assert.NotContains(t, public.Message, "private-upstream")
	assert.NotContains(t, public.Message, "10.0.0.8")
	assert.NotContains(t, public.Message, "sk-private")
	assert.NotContains(t, public.Message, "raw-body")
}

func TestResponsesHelperPassthroughWithoutInjectionDoesNotMaterializeBody(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelAffinitySetting()
	originalRules := setting.Rules
	setting.Rules = []operation_setting.ChannelAffinityRule{{
		Name:                 "missing-session-does-not-inject",
		ModelRegex:           []string{"^gpt-.*$"},
		PathRegex:            []string{"^/v1/responses$"},
		KeySources:           []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "Session_id"}},
		InjectPromptCacheKey: true,
	}}
	t.Cleanup(func() { setting.Rules = originalRules })

	requestJSON := []byte(`{"model":"gpt-test","input":"hello","unknown_passthrough_field":"kept"}`)
	upstreamReached := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, string(requestJSON), string(body))
		upstreamReached <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"fixture rejected after capture","type":"invalid_request_error"}}`))
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestJSON))
	storage := newNonMaterializingBodyStorage(requestJSON)
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyTokenId, 8601)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "gptpro")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})

	request := &dto.OpenAIResponsesRequest{Model: "gpt-test", Input: []byte(`"hello"`)}
	_, found := service.GetPreferredChannelByAffinity(c, request.Model, "gptpro")
	require.False(t, found)
	info := relaycommon.GenRelayInfoResponses(c, request)
	newAPIError := ResponsesHelper(c, info)
	require.NotNil(t, newAPIError)
	select {
	case <-upstreamReached:
	default:
		t.Fatal("passthrough request did not reach upstream")
	}
	assert.Zero(t, storage.bytesCalls.Load())
}
