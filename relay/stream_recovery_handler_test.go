package relay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func configureStreamRecoveryBillingTest(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func newResponsesStreamRecoveryFixture(t *testing.T, serverURL string, isStream bool) (*gin.Context, *relaycommon.RelayInfo, context.CancelFunc) {
	t.Helper()

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test"}`)).WithContext(requestContext)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, serverURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")

	stream := isStream
	info := &relaycommon.RelayInfo{
		Request:         &dto.OpenAIResponsesRequest{Model: "gpt-test", Stream: &stream},
		OriginModelName: "gpt-test",
		IsStream:        isStream,
		DisablePing:     true,
		RelayMode:       relayconstant.RelayModeResponses,
		RequestURLPath:  "/v1/responses",
	}
	return c, info, cancel
}

func newClaudeStreamRecoveryFixture(t *testing.T, serverURL string, isStream bool) (*gin.Context, *relaycommon.RelayInfo, context.CancelFunc) {
	t.Helper()

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`)).WithContext(requestContext)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, serverURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-test")

	stream := isStream
	maxTokens := uint(128)
	info := &relaycommon.RelayInfo{
		Request:         &dto.ClaudeRequest{Model: "claude-test", Stream: &stream, MaxTokens: &maxTokens},
		OriginModelName: "claude-test",
		IsStream:        isStream,
		DisablePing:     true,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatClaude,
		RequestURLPath:  "/v1/messages",
	}
	return c, info, cancel
}

func requireStreamRecoveryAccepted(t *testing.T, info *relaycommon.RelayInfo) *relaycommon.StreamRecovery {
	t.Helper()

	deadline := time.NewTimer(2 * time.Second)
	t.Cleanup(func() { deadline.Stop() })
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if info.StreamRecovery != nil && info.GetStreamRecoverySnapshot().Accepted {
			return info.StreamRecovery
		}
		select {
		case <-deadline.C:
			t.Fatal("stream recovery was not accepted before response parsing")
		case <-ticker.C:
		}
	}
}

func requireAcceptedStreamRecoveryFinished(t *testing.T, info *relaycommon.RelayInfo) {
	t.Helper()

	finishedContext := info.StartStreamRecoveryAttempt(context.Background())
	require.ErrorIs(t, finishedContext.Err(), context.Canceled)
}

func TestHelperStreamRecoveryGating(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	tests := []struct {
		name          string
		enabled       bool
		isStream      bool
		createFixture func(*testing.T, string, bool) (*gin.Context, *relaycommon.RelayInfo, context.CancelFunc)
		invoke        func(*gin.Context, *relaycommon.RelayInfo) *types.NewAPIError
	}{
		{
			name: "responses feature disabled stream", enabled: false, isStream: true,
			createFixture: newResponsesStreamRecoveryFixture, invoke: ResponsesHelper,
		},
		{
			name: "responses feature enabled non-stream", enabled: true, isStream: false,
			createFixture: newResponsesStreamRecoveryFixture, invoke: ResponsesHelper,
		},
		{
			name: "claude feature disabled stream", enabled: false, isStream: true,
			createFixture: newClaudeStreamRecoveryFixture, invoke: ClaudeHelper,
		},
		{
			name: "claude feature enabled non-stream", enabled: true, isStream: false,
			createFixture: newClaudeStreamRecoveryFixture, invoke: ClaudeHelper,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			constant.StreamUsageDrainEnabled = test.enabled
			upstreamStarted := make(chan struct{})
			releaseHandler := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseHandler) }) }
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				close(upstreamStarted)
				select {
				case <-r.Context().Done():
				case <-releaseHandler:
				}
			}))
			t.Cleanup(func() {
				release()
				server.CloseClientConnections()
				server.Close()
			})
			t.Cleanup(release)

			c, info, cancel := test.createFixture(t, server.URL, test.isStream)
			done := make(chan *types.NewAPIError, 1)
			go func() { done <- test.invoke(c, info) }()
			select {
			case <-upstreamStarted:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream request did not start")
			}
			require.Nil(t, info.StreamRecovery)
			cancel()
			select {
			case newAPIError := <-done:
				require.NotNil(t, newAPIError)
			case <-time.After(2 * time.Second):
				t.Fatal("helper did not preserve downstream request cancellation")
			}
			require.Nil(t, info.StreamRecovery)
		})
	}
}

func TestResponsesHelperStreamRecovery201(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"error":{"message":"created response is not protocol success","type":"upstream_error"}}`))
	}))
	t.Cleanup(server.Close)
	c, info, _ := newResponsesStreamRecoveryFixture(t, server.URL, true)

	newAPIError := ResponsesHelper(c, info)
	require.NotNil(t, newAPIError)
	require.Contains(t, newAPIError.Error(), "created response is not protocol success")
	require.True(t, info.GetStreamRecoverySnapshot().Accepted)
	requireAcceptedStreamRecoveryFinished(t, info)
}

func TestResponsesHelperStreamRecoverySuccess(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	originalStreamingTimeout := constant.StreamingTimeout
	constant.StreamUsageDrainEnabled = true
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = originalEnabled
		constant.StreamingTimeout = originalStreamingTimeout
	})
	service.InitHttpClient()
	configureStreamRecoveryBillingTest(t)

	headersFlushed := make(chan struct{})
	releaseBody := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBody) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(headersFlushed)
		<-releaseBody
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-test\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0,\"total_tokens\":0}}}\n\ndata: [DONE]\n\n"))
	}))
	t.Cleanup(func() { release(); server.CloseClientConnections(); server.Close() })
	t.Cleanup(release)
	c, info, _ := newResponsesStreamRecoveryFixture(t, server.URL, true)

	done := make(chan *types.NewAPIError, 1)
	go func() { done <- ResponsesHelper(c, info) }()
	select {
	case <-headersFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("responses helper did not receive 200 headers")
	}
	recovery := requireStreamRecoveryAccepted(t, info)
	release()
	select {
	case newAPIError := <-done:
		require.Nil(t, newAPIError)
	case <-time.After(2 * time.Second):
		t.Fatal("responses helper did not finish parsing")
	}
	require.Same(t, recovery, info.StreamRecovery)
	requireAcceptedStreamRecoveryFinished(t, info)
}

func TestClaudeHelperStreamRecoverySuccess(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	originalStreamingTimeout := constant.StreamingTimeout
	constant.StreamUsageDrainEnabled = true
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = originalEnabled
		constant.StreamingTimeout = originalStreamingTimeout
	})
	service.InitHttpClient()
	configureStreamRecoveryBillingTest(t)

	headersFlushed := make(chan struct{})
	releaseBody := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBody) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(headersFlushed)
		<-releaseBody
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-test\",\"content\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	t.Cleanup(func() { release(); server.CloseClientConnections(); server.Close() })
	t.Cleanup(release)
	c, info, _ := newClaudeStreamRecoveryFixture(t, server.URL, true)

	done := make(chan *types.NewAPIError, 1)
	go func() { done <- ClaudeHelper(c, info) }()
	select {
	case <-headersFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("Claude helper did not receive 200 headers")
	}
	recovery := requireStreamRecoveryAccepted(t, info)
	release()
	select {
	case newAPIError := <-done:
		require.Nil(t, newAPIError)
	case <-time.After(2 * time.Second):
		t.Fatal("Claude helper did not finish parsing")
	}
	require.Same(t, recovery, info.StreamRecovery)
	requireAcceptedStreamRecoveryFinished(t, info)
}

func TestChatCompletionsViaResponsesStreamRecoveryConvertedSuccess(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	headersFlushed := make(chan struct{})
	releaseBody := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBody) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(headersFlushed)
		<-releaseBody
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","model":"gpt-test","output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(func() { release(); server.CloseClientConnections(); server.Close() })
	t.Cleanup(release)

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-test"}`)).WithContext(requestContext)
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		DisablePing:    true,
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RequestURLPath: "/v1/messages",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ChannelBaseUrl:    server.URL,
			ApiType:           constant.APITypeOpenAI,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-test",
		},
	}
	info.EnableStreamRecovery()
	recovery := info.StreamRecovery
	t.Cleanup(info.FinishStreamRecovery)
	adaptor := &openaichannel.Adaptor{}
	adaptor.Init(info)
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Stream:   &stream,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}

	type convertedResult struct {
		usage *dto.Usage
		err   *types.NewAPIError
	}
	done := make(chan convertedResult, 1)
	go func() {
		usage, newAPIError := chatCompletionsViaResponses(c, info, adaptor, request)
		done <- convertedResult{usage: usage, err: newAPIError}
	}()
	select {
	case <-headersFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("converted helper did not receive 200 headers")
	}
	require.Same(t, recovery, requireStreamRecoveryAccepted(t, info))
	release()
	select {
	case result := <-done:
		require.Nil(t, result.err, "converted error: %#v", result.err)
		require.NotNil(t, result.usage)
	case <-time.After(2 * time.Second):
		t.Fatal("converted helper did not finish parsing")
	}
	require.Same(t, recovery, info.StreamRecovery)
	requireAcceptedStreamRecoveryFinished(t, info)
}

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

type streamRecoveryBillingRecorder struct {
	preConsumed int
	settled     []int
}

func (b *streamRecoveryBillingRecorder) Settle(quota int) error {
	b.settled = append(b.settled, quota)
	return nil
}

func (b *streamRecoveryBillingRecorder) Refund(*gin.Context) {}
func (b *streamRecoveryBillingRecorder) NeedsRefund() bool   { return len(b.settled) == 0 }
func (b *streamRecoveryBillingRecorder) GetPreConsumedQuota() int {
	return b.preConsumed
}
func (b *streamRecoveryBillingRecorder) Reserve(int) error { return nil }

func TestAcceptedStreamErrorSettlementIsEstimatedAndSingleShot(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	configureStreamRecoveryBillingTest(t)
	seed := &model.User{Id: 801, Username: "stream-user"}
	require.NoError(t, model.DB.Create(seed).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 802, Name: "stream-channel"}).Error)

	billing := &streamRecoveryBillingRecorder{preConsumed: 1250}
	info := &relaycommon.RelayInfo{
		UserId: 801, TokenId: 803, OriginModelName: "gpt-test", UsingGroup: "default",
		StartTime: time.Now(), IsStream: true, Billing: billing, FinalPreConsumedQuota: 1250,
		PriceData: types.PriceData{
			ModelRatio: 1, CompletionRatio: 1, QuotaToPreConsume: 1250,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 802, UpstreamModelName: "gpt-test"},
	}
	info.SetEstimatePromptTokens(400)
	info.EnableStreamRecovery()
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	relayErr := types.NewError(errors.New("malformed accepted stream"), types.ErrorCodeBadResponse)

	result := settleAcceptedStreamError(c, info, nil, relayErr)

	require.True(t, types.IsSkipRetryError(result))
	require.Equal(t, []int{400}, billing.settled)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", info.UserId).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, 400, logs[0].PromptTokens)
	require.Zero(t, logs[0].CompletionTokens)
	require.Contains(t, logs[0].Other, `"usage_source":"estimated"`)
}

func TestResponsesAndClaudeAcceptedMalformedStreamsSettleEstimatedUsage(t *testing.T) {
	originalStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = originalStreamingTimeout })
	tests := []struct {
		name    string
		path    string
		channel int
		request dto.Request
		invoke  func(*gin.Context, *relaycommon.RelayInfo) *types.NewAPIError
		wantErr bool
		want    int
	}{
		{
			name: "responses", path: "/v1/responses", channel: constant.ChannelTypeOpenAI,
			request: &dto.OpenAIResponsesRequest{Model: "gpt-test", Stream: common.GetPointer(true)},
			invoke:  ResponsesHelper,
			want:    400,
		},
		{
			name: "claude", path: "/v1/messages", channel: constant.ChannelTypeAnthropic,
			request: &dto.ClaudeRequest{Model: "claude-test", Stream: common.GetPointer(true), MaxTokens: common.GetPointer(uint(128))},
			invoke:  ClaudeHelper,
			wantErr: true,
			want:    1250,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalEnabled := constant.StreamUsageDrainEnabled
			constant.StreamUsageDrainEnabled = true
			t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
			configureStreamRecoveryBillingTest(t)
			service.InitHttpClient()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("data: {malformed\n\n"))
			}))
			t.Cleanup(server.Close)

			userID := 811
			channelID := 812
			require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "handler-user"}).Error)
			require.NoError(t, model.DB.Create(&model.Channel{Id: channelID, Name: "handler-channel"}).Error)
			billing := &streamRecoveryBillingRecorder{preConsumed: 1250}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(`{"model":"gpt-test","stream":true}`))
			common.SetContextKey(c, constant.ContextKeyChannelType, test.channel)
			common.SetContextKey(c, constant.ContextKeyChannelId, channelID)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
			info := &relaycommon.RelayInfo{
				Request: test.request, UserId: userID, TokenId: 813, OriginModelName: "gpt-test", UsingGroup: "default",
				StartTime: time.Now(), IsStream: true, DisablePing: true, Billing: billing, FinalPreConsumedQuota: 1250,
				RelayMode: relayconstant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI, RequestURLPath: test.path,
				PriceData: types.PriceData{ModelRatio: 1, CompletionRatio: 1, QuotaToPreConsume: 1250, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}},
			}
			if test.name == "responses" {
				info.RelayMode = relayconstant.RelayModeResponses
			}
			info.SetEstimatePromptTokens(400)

			relayErr := test.invoke(c, info)

			if test.wantErr {
				require.NotNil(t, relayErr)
				require.True(t, types.IsSkipRetryError(relayErr))
			} else {
				require.Nil(t, relayErr)
			}
			require.Equal(t, []int{test.want}, billing.settled)
			var logs []model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", userID).Find(&logs).Error)
			require.Len(t, logs, 1)
			require.Equal(t, 400, logs[0].PromptTokens)
			require.Contains(t, logs[0].Other, `"usage_source":"estimated"`)
		})
	}
}
