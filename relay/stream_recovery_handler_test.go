package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesHelperFinishesNon200RecoveryAfterParsingError(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"structured upstream failure","type":"upstream_error"}}`))
	}))
	t.Cleanup(server.Close)

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test","stream":true}`)).WithContext(requestContext)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")

	stream := true
	info := &relaycommon.RelayInfo{
		Request:         &dto.OpenAIResponsesRequest{Model: "gpt-test", Stream: &stream},
		OriginModelName: "gpt-test",
		IsStream:        true,
		DisablePing:     true,
		RelayMode:       relayconstant.RelayModeResponses,
		RequestURLPath:  "/v1/responses",
		Timings:         relaycommon.NewRelayTimings(),
	}

	newAPIError := ResponsesHelper(c, info)
	require.NotNil(t, newAPIError)
	require.Contains(t, newAPIError.Error(), "structured upstream failure")

	retryContext := info.StartStreamRecoveryAttempt(context.Background())
	t.Cleanup(info.FinishStreamRecovery)
	cancel()
	select {
	case <-retryContext.Done():
		t.Fatal("retry reused the non-200 attempt context")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestClaudeHelperFinishesNon200RecoveryAfterParsingError(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"structured claude failure","type":"upstream_error"}}`))
	}))
	t.Cleanup(server.Close)

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test","stream":true}`)).WithContext(requestContext)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-test")

	stream := true
	maxTokens := uint(128)
	info := &relaycommon.RelayInfo{
		Request:         &dto.ClaudeRequest{Model: "claude-test", Stream: &stream, MaxTokens: &maxTokens},
		OriginModelName: "claude-test",
		IsStream:        true,
		DisablePing:     true,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatClaude,
		RequestURLPath:  "/v1/messages",
		Timings:         relaycommon.NewRelayTimings(),
	}

	newAPIError := ClaudeHelper(c, info)
	require.NotNil(t, newAPIError)
	require.Contains(t, newAPIError.Error(), "structured claude failure")

	retryContext := info.StartStreamRecoveryAttempt(context.Background())
	t.Cleanup(info.FinishStreamRecovery)
	cancel()
	select {
	case <-retryContext.Done():
		t.Fatal("retry reused the Claude non-200 attempt context")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestChatCompletionsViaResponsesFinishesNon200AfterParsingError(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	headersFlushed := make(chan struct{})
	allowBody := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	var allowBodyOnce sync.Once
	releaseBody := func() {
		allowBodyOnce.Do(func() { close(allowBody) })
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.(http.Flusher).Flush()
		close(headersFlushed)
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
			return
		case <-allowBody:
		}
		_, _ = w.Write([]byte(`{"error":{"message":"converted structured failure","type":"upstream_error"}}`))
	}))
	t.Cleanup(func() {
		releaseBody()
		server.CloseClientConnections()
		server.Close()
	})
	t.Cleanup(releaseBody)

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-test","stream":true}`)).WithContext(requestContext)

	info := &relaycommon.RelayInfo{
		IsStream:       true,
		DisablePing:    true,
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RequestURLPath: "/v1/messages",
		Timings:        relaycommon.NewRelayTimings(),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ChannelBaseUrl:    server.URL,
			ApiType:           constant.APITypeOpenAI,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-test",
		},
	}
	info.EnableStreamRecovery()
	t.Cleanup(info.FinishStreamRecovery)

	adaptor := &openaichannel.Adaptor{}
	adaptor.Init(info)
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Stream:   &stream,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}

	done := make(chan *types.NewAPIError, 1)
	go func() {
		_, newAPIError := chatCompletionsViaResponses(c, info, adaptor, request)
		done <- newAPIError
	}()
	select {
	case <-headersFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("converted request did not receive upstream headers")
	}
	select {
	case <-upstreamCanceled:
		t.Fatal("converted request canceled upstream before parsing the error body")
	case <-time.After(100 * time.Millisecond):
	}
	releaseBody()

	select {
	case newAPIError := <-done:
		require.NotNil(t, newAPIError)
		require.Contains(t, newAPIError.Error(), "converted structured failure")
	case <-time.After(2 * time.Second):
		t.Fatal("converted request did not finish parsing the error body")
	}
}
