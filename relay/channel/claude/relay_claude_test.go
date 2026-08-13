package claude

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commonPointer[T any](value T) *T {
	return &value
}

type claudeStreamWriteSignal struct {
	gin.ResponseWriter
	wrote chan struct{}
	once  sync.Once
}

type claudeStreamFailWriter struct {
	gin.ResponseWriter
}

func (w *claudeStreamFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("downstream write failed")
}

func (w *claudeStreamFailWriter) WriteString(string) (int, error) {
	return 0, errors.New("downstream write failed")
}

func (w *claudeStreamWriteSignal) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func (w *claudeStreamWriteSignal) WriteString(data string) (int, error) {
	return w.ResponseWriter.WriteString(data)
}

func (w *claudeStreamWriteSignal) Flush() {
	w.ResponseWriter.Flush()
	w.once.Do(func() { close(w.wrote) })
}

type claudeStreamHandlerResult struct {
	usage *dto.Usage
	err   *types.NewAPIError
}

func configureClaudeStreamRecoveryTest(t *testing.T) {
	t.Helper()

	oldEnabled := constant.StreamUsageDrainEnabled
	oldMaxConcurrency := constant.StreamUsageDrainMaxConcurrency
	oldMaxPerChannel := constant.StreamUsageDrainMaxPerChannel
	oldTimeout := constant.StreamUsageDrainTimeoutSeconds
	oldMaxBytes := constant.StreamUsageDrainMaxBytesMB
	oldStreamingTimeout := constant.StreamingTimeout
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = oldEnabled
		constant.StreamUsageDrainMaxConcurrency = oldMaxConcurrency
		constant.StreamUsageDrainMaxPerChannel = oldMaxPerChannel
		constant.StreamUsageDrainTimeoutSeconds = oldTimeout
		constant.StreamUsageDrainMaxBytesMB = oldMaxBytes
		constant.StreamingTimeout = oldStreamingTimeout
	})

	constant.StreamUsageDrainEnabled = true
	constant.StreamUsageDrainMaxConcurrency = 4
	constant.StreamUsageDrainMaxPerChannel = 4
	constant.StreamUsageDrainTimeoutSeconds = 30
	constant.StreamUsageDrainMaxBytesMB = 32
	constant.StreamingTimeout = 30
}

func newClaudeStreamRecoveryTest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, context.CancelFunc) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	requestContext, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(requestContext)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
	}
	info.EnableStreamRecovery()
	require.NotNil(t, info.StreamRecovery)
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)
	return c, recorder, info, cancel
}

func waitForClaudeStreamWrite(t *testing.T, wrote <-chan struct{}) {
	t.Helper()
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first downstream Claude stream write")
	}
}

func waitForClaudeStreamResult(t *testing.T, result <-chan claudeStreamHandlerResult) claudeStreamHandlerResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Claude stream recovery")
		return claudeStreamHandlerResult{}
	}
}

func TestClaudeStreamHandlerRecoversUsageAfterClientGone(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, recorder, info, cancel := newClaudeStreamRecoveryTest(t)
	wrote := make(chan struct{})
	c.Writer = &claudeStreamWriteSignal{ResponseWriter: c.Writer, wrote: wrote}

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	result := make(chan claudeStreamHandlerResult, 1)
	go func() {
		usage, relayErr := ClaudeStreamHandler(c, resp, info)
		result <- claudeStreamHandlerResult{usage: usage, err: relayErr}
	}()

	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":320000,"output_tokens":1,"cache_read_input_tokens":300000,"cache_creation_input_tokens":12000,"cache_creation":{"ephemeral_5m_input_tokens":8000,"ephemeral_1h_input_tokens":4000}}}}`
	_, err := io.WriteString(writer, "data: "+messageStart+"\n\n")
	require.NoError(t, err)
	waitForClaudeStreamWrite(t, wrote)
	cancel()
	messageDelta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":800}}`
	_, err = io.WriteString(writer, "data: "+messageDelta+"\n\ndata: {\"type\":\"message_stop\"}\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	got := waitForClaudeStreamResult(t, result)
	require.Nil(t, got.err)
	require.NotNil(t, got.usage)
	require.Equal(t, 320000, got.usage.PromptTokens)
	require.Equal(t, 800, got.usage.CompletionTokens)
	require.Equal(t, 320800, got.usage.TotalTokens)
	require.Equal(t, 300000, got.usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 12000, got.usage.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, 8000, got.usage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 4000, got.usage.ClaudeCacheCreation1hTokens)
	require.NotContains(t, recorder.Body.String(), messageDelta)
	snapshot := info.GetStreamRecoverySnapshot()
	require.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	require.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestClaudeStreamHandlerPartialUsageKeepsAuthoritativeCache(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, _, info, cancel := newClaudeStreamRecoveryTest(t)
	info.SetEstimatePromptTokens(999999)
	wrote := make(chan struct{})
	c.Writer = &claudeStreamWriteSignal{ResponseWriter: c.Writer, wrote: wrote}

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	result := make(chan claudeStreamHandlerResult, 1)
	go func() {
		usage, relayErr := ClaudeStreamHandler(c, resp, info)
		result <- claudeStreamHandlerResult{usage: usage, err: relayErr}
	}()

	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":320000,"output_tokens":1,"cache_read_input_tokens":300000,"cache_creation_input_tokens":12000,"cache_creation":{"ephemeral_5m_input_tokens":8000,"ephemeral_1h_input_tokens":4000}}}}`
	_, err := io.WriteString(writer, "data: "+messageStart+"\n\n")
	require.NoError(t, err)
	waitForClaudeStreamWrite(t, wrote)
	cancel()
	require.NoError(t, writer.Close())

	got := waitForClaudeStreamResult(t, result)
	require.Nil(t, got.err)
	require.NotNil(t, got.usage)
	require.Equal(t, 320000, got.usage.PromptTokens)
	require.Equal(t, 300000, got.usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 12000, got.usage.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, 8000, got.usage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 4000, got.usage.ClaudeCacheCreation1hTokens)
	snapshot := info.GetStreamRecoverySnapshot()
	require.Equal(t, relaycommon.StreamUsageStatePartial, snapshot.UsageState)
	require.Equal(t, relaycommon.StreamDrainResultUpstreamError, snapshot.DrainResult)
}

func TestClaudeStreamHandlerUnknownUsageDoesNotEstimatePrompt(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, _, info, cancel := newClaudeStreamRecoveryTest(t)
	info.SetEstimatePromptTokens(320000)

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	result := make(chan claudeStreamHandlerResult, 1)
	go func() {
		usage, relayErr := ClaudeStreamHandler(c, resp, info)
		result <- claudeStreamHandlerResult{usage: usage, err: relayErr}
	}()

	cancel()
	_, err := io.WriteString(writer, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	got := waitForClaudeStreamResult(t, result)
	require.Nil(t, got.err)
	require.NotNil(t, got.usage)
	require.Zero(t, got.usage.PromptTokens)
	require.Zero(t, got.usage.CompletionTokens)
	require.Zero(t, got.usage.TotalTokens)
	require.Equal(t, relaycommon.StreamUsageStateUnknown, info.GetStreamRecoverySnapshot().UsageState)
}

func TestClaudeStreamHandlerCompletedBytesRemainUnchanged(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, recorder, info, _ := newClaudeStreamRecoveryTest(t)
	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":1}}}`
	messageDelta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`
	patchedMessageDelta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3,"input_tokens":2}}`
	messageStop := `{"type":"message_stop"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: " + messageStart + "\n\ndata: " + messageDelta + "\n\ndata: " + messageStop + "\n\n",
		)),
	}

	usage, relayErr := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.PromptTokens)
	require.Equal(t, 3, usage.CompletionTokens)
	require.Equal(t,
		"event: message_start\ndata: "+messageStart+"\n\n\n"+
			"event: message_delta\ndata: "+patchedMessageDelta+"\n\n\n"+
			"event: message_stop\ndata: "+messageStop+"\n\n\n",
		recorder.Body.String(),
	)
	snapshot := info.GetStreamRecoverySnapshot()
	require.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	require.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestClaudeStreamHandlerPropagatesForegroundWriteFailure(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, _, info, _ := newClaudeStreamRecoveryTest(t)
	c.Writer = &claudeStreamFailWriter{ResponseWriter: c.Writer}
	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":1}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("data: " + messageStart + "\n\n")),
	}

	usage, relayErr := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.ErrorContains(t, relayErr, "downstream write failed")
}

func TestClaudeStreamHandlerConvertedDoesNotWriteAfterClientGone(t *testing.T) {
	configureClaudeStreamRecoveryTest(t)
	c, recorder, info, cancel := newClaudeStreamRecoveryTest(t)
	info.RelayFormat = types.RelayFormatOpenAI
	info.ShouldIncludeUsage = true
	wrote := make(chan struct{})
	c.Writer = &claudeStreamWriteSignal{ResponseWriter: c.Writer, wrote: wrote}

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	result := make(chan claudeStreamHandlerResult, 1)
	go func() {
		usage, relayErr := ClaudeStreamHandler(c, resp, info)
		result <- claudeStreamHandlerResult{usage: usage, err: relayErr}
	}()

	messageStart := `{"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":320000,"output_tokens":1,"cache_read_input_tokens":300000,"cache_creation_input_tokens":12000}}}`
	_, err := io.WriteString(writer, "data: "+messageStart+"\n\n")
	require.NoError(t, err)
	waitForClaudeStreamWrite(t, wrote)
	beforeCancel := recorder.Body.String()
	cancel()
	require.Eventually(t, info.IsStreamDetached, 2*time.Second, time.Millisecond)
	messageDelta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":800}}`
	_, err = io.WriteString(writer, "data: "+messageDelta+"\n\ndata: {\"type\":\"message_stop\"}\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	got := waitForClaudeStreamResult(t, result)
	require.Nil(t, got.err)
	require.NotNil(t, got.usage)
	require.Equal(t, 800, got.usage.CompletionTokens)
	require.Equal(t, beforeCancel, recorder.Body.String())
	require.Equal(t, relaycommon.StreamUsageStateExact, info.GetStreamRecoverySnapshot().UsageState)
}

func TestResponseOpenAI2ClaudeToolUseInputIsObject(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]interface{}
	}{
		{name: "object", args: `{"q":"x"}`, want: map[string]interface{}{"q": "x"}},
		{name: "empty", args: "", want: map[string]interface{}{}},
		{name: "invalid", args: "{", want: map[string]interface{}{}},
		{name: "null", args: "null", want: map[string]interface{}{}},
		{name: "array", args: `["x"]`, want: map[string]interface{}{}},
		{name: "string", args: `"x"`, want: map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dto.Message{Role: "assistant"}
			msg.SetToolCalls([]dto.ToolCallRequest{
				{
					ID:   "call_1",
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      "lookup",
						Arguments: tt.args,
					},
				},
			})
			resp := service.ResponseOpenAI2Claude(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{Message: msg, FinishReason: "tool_calls"},
				},
			}, nil)

			require.Len(t, resp.Content, 1)
			assert.Equal(t, "tool_use", resp.Content[0].Type)
			assert.Equal(t, tt.want, resp.Content[0].Input)
		})
	}
}

func TestFormatClaudeResponseInfo_MessageStart(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_start",
		Message: &dto.ClaudeMediaMessage{
			Id:    "msg_123",
			Model: "claude-3-5-sonnet",
			Usage: &dto.ClaudeUsage{
				InputTokens:              100,
				OutputTokens:             1,
				CacheCreationInputTokens: 50,
				CacheReadInputTokens:     30,
			},
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens != 50 {
		t.Errorf("CachedCreationTokens = %d, want 50", claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	}
	if claudeInfo.ResponseId != "msg_123" {
		t.Errorf("ResponseId = %s, want msg_123", claudeInfo.ResponseId)
	}
	if claudeInfo.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = %s, want claude-3-5-sonnet", claudeInfo.Model)
	}
}

func TestFormatClaudeResponseInfo_MessageDelta_FullUsage(t *testing.T) {
	// message_start 先积累 usage
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens: 1,
		},
	}

	// message_delta 带完整 usage（原生 Anthropic 场景）
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			InputTokens:              100,
			OutputTokens:             200,
			CacheCreationInputTokens: 50,
			CacheReadInputTokens:     30,
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", claudeInfo.Usage.CompletionTokens)
	}
	if claudeInfo.Usage.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", claudeInfo.Usage.TotalTokens)
	}
	if !claudeInfo.Done {
		t.Error("expected Done = true")
	}
}

func TestFormatClaudeResponseInfo_MessageDelta_OnlyOutputTokens(t *testing.T) {
	// 模拟 Bedrock: message_start 已积累 usage
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens:            1,
			ClaudeCacheCreation5mTokens: 10,
			ClaudeCacheCreation1hTokens: 20,
		},
	}

	// Bedrock 的 message_delta 只有 output_tokens，缺少 input_tokens 和 cache 字段
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			OutputTokens: 200,
			// InputTokens, CacheCreationInputTokens, CacheReadInputTokens 都是 0
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	// PromptTokens 应保持 message_start 的值（因为 message_delta 的 InputTokens=0，不更新）
	if claudeInfo.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", claudeInfo.Usage.PromptTokens)
	}
	if claudeInfo.Usage.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", claudeInfo.Usage.CompletionTokens)
	}
	if claudeInfo.Usage.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", claudeInfo.Usage.TotalTokens)
	}
	// cache 字段应保持 message_start 的值
	if claudeInfo.Usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	}
	if claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens != 50 {
		t.Errorf("CachedCreationTokens = %d, want 50", claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	}
	if claudeInfo.Usage.ClaudeCacheCreation5mTokens != 10 {
		t.Errorf("ClaudeCacheCreation5mTokens = %d, want 10", claudeInfo.Usage.ClaudeCacheCreation5mTokens)
	}
	if claudeInfo.Usage.ClaudeCacheCreation1hTokens != 20 {
		t.Errorf("ClaudeCacheCreation1hTokens = %d, want 20", claudeInfo.Usage.ClaudeCacheCreation1hTokens)
	}
	if !claudeInfo.Done {
		t.Error("expected Done = true")
	}
}

func TestFormatClaudeResponseInfo_NilClaudeInfo(t *testing.T) {
	claudeResponse := &dto.ClaudeResponse{Type: "message_start"}
	ok := FormatClaudeResponseInfo(claudeResponse, nil, nil)
	if ok {
		t.Error("expected false for nil claudeInfo")
	}
}

func TestFormatClaudeResponseInfo_ContentBlockDelta(t *testing.T) {
	text := "hello"
	claudeInfo := &ClaudeResponseInfo{
		Usage:        &dto.Usage{},
		ResponseText: strings.Builder{},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Text: &text,
		},
	}

	ok := FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo)
	if !ok {
		t.Fatal("expected true")
	}
	if claudeInfo.ResponseText.String() != "hello" {
		t.Errorf("ResponseText = %q, want %q", claudeInfo.ResponseText.String(), "hello")
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsage(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
		UsageSemantic:               "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

	if openAIUsage.PromptTokens != 180 {
		t.Fatalf("PromptTokens = %d, want 180", openAIUsage.PromptTokens)
	}
	if openAIUsage.InputTokens != 180 {
		t.Fatalf("InputTokens = %d, want 180", openAIUsage.InputTokens)
	}
	if openAIUsage.TotalTokens != 200 {
		t.Fatalf("TotalTokens = %d, want 200", openAIUsage.TotalTokens)
	}
	if openAIUsage.UsageSemantic != "openai" {
		t.Fatalf("UsageSemantic = %s, want openai", openAIUsage.UsageSemantic)
	}
	if openAIUsage.UsageSource != "anthropic" {
		t.Fatalf("UsageSource = %s, want anthropic", openAIUsage.UsageSource)
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsagePreservesCacheCreationRemainder(t *testing.T) {
	tests := []struct {
		name                    string
		cachedCreationTokens    int
		cacheCreationTokens5m   int
		cacheCreationTokens1h   int
		expectedTotalInputToken int
	}{
		{
			name:                    "prefers aggregate when it includes remainder",
			cachedCreationTokens:    50,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 180,
		},
		{
			name:                    "falls back to split tokens when aggregate missing",
			cachedCreationTokens:    0,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 160,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &dto.Usage{
				PromptTokens:     100,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: tt.cachedCreationTokens,
				},
				ClaudeCacheCreation5mTokens: tt.cacheCreationTokens5m,
				ClaudeCacheCreation1hTokens: tt.cacheCreationTokens1h,
				UsageSemantic:               "anthropic",
			}

			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

			if openAIUsage.PromptTokens != tt.expectedTotalInputToken {
				t.Fatalf("PromptTokens = %d, want %d", openAIUsage.PromptTokens, tt.expectedTotalInputToken)
			}
			if openAIUsage.InputTokens != tt.expectedTotalInputToken {
				t.Fatalf("InputTokens = %d, want %d", openAIUsage.InputTokens, tt.expectedTotalInputToken)
			}
		})
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsageDefaultsAggregateCacheCreationTo5m(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		UsageSemantic: "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)

	require.Equal(t, 50, openAIUsage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 0, openAIUsage.ClaudeCacheCreation1hTokens)
}

func TestRequestOpenAI2ClaudeMessage_ClaudeOpus48HighUsesAdaptiveThinking(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-high",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}

func TestRequestOpenAI2ClaudeMessage_ClaudeOpus48ThinkingUsesAdaptiveHighEffort(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-thinking",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}
