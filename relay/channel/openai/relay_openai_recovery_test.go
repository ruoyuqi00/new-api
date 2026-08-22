package openai

import (
	"context"
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
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIStreamWriteSignal struct {
	gin.ResponseWriter
	wrote chan struct{}
	once  sync.Once
}

func (w *openAIStreamWriteSignal) Flush() {
	w.ResponseWriter.Flush()
	w.once.Do(func() { close(w.wrote) })
}

func TestOaiStreamHandlerRecoversTerminalUsageAfterClientGone(t *testing.T) {
	oldEnabled := constant.StreamUsageDrainEnabled
	oldMaxConcurrency := constant.StreamUsageDrainMaxConcurrency
	oldMaxPerChannel := constant.StreamUsageDrainMaxPerChannel
	oldTimeout := constant.StreamUsageDrainTimeoutSeconds
	oldStreamingTimeout := constant.StreamingTimeout
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = oldEnabled
		constant.StreamUsageDrainMaxConcurrency = oldMaxConcurrency
		constant.StreamUsageDrainMaxPerChannel = oldMaxPerChannel
		constant.StreamUsageDrainTimeoutSeconds = oldTimeout
		constant.StreamingTimeout = oldStreamingTimeout
	})
	constant.StreamUsageDrainEnabled = true
	constant.StreamUsageDrainMaxConcurrency = 4
	constant.StreamUsageDrainMaxPerChannel = 4
	constant.StreamUsageDrainTimeoutSeconds = 30
	constant.StreamingTimeout = 30

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	wrote := make(chan struct{})
	c.Writer = &openAIStreamWriteSignal{ResponseWriter: c.Writer, wrote: wrote}
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         77,
			UpstreamModelName: "gpt-test",
		},
	}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	type streamResult struct {
		usage *dto.Usage
		err   error
	}
	done := make(chan streamResult, 1)
	go func() {
		usage, relayErr := OaiStreamHandler(c, info, resp)
		if relayErr != nil {
			done <- streamResult{err: relayErr}
			return
		}
		done <- streamResult{usage: usage}
	}()

	first := `{"id":"chatcmpl_1","choices":[{"delta":{"content":"hello"}}]}`
	second := `{"id":"chatcmpl_1","choices":[{"delta":{"content":" world"}}]}`
	terminal := `{"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":25,"total_tokens":1225,"prompt_tokens_details":{"cached_tokens":1024}}}`
	_, err := io.WriteString(writer, "data: "+first+"\n\ndata: "+second+"\n\n")
	require.NoError(t, err)
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first downstream stream write")
	}
	beforeCancel := recorder.Body.String()
	cancel()
	require.Eventually(t, info.IsStreamDetached, 2*time.Second, time.Millisecond)
	_, err = io.WriteString(writer, "data: "+terminal+"\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.NotNil(t, result.usage)
		require.Equal(t, 1200, result.usage.PromptTokens)
		require.Equal(t, 25, result.usage.CompletionTokens)
		require.Equal(t, 1024, result.usage.PromptTokensDetails.CachedTokens)
		require.Equal(t, "upstream", result.usage.UsageSource)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recovered terminal usage")
	}
	require.Equal(t, beforeCancel, recorder.Body.String())
	require.Equal(t, relaycommon.StreamUsageStateExact, info.GetStreamRecoverySnapshot().UsageState)
	require.NotContains(t, recorder.Body.String(), strings.Trim(terminal, " "))
}

func TestOaiStreamHandlerSanitizesAmplifiedTerminalUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	info := mappedClientResponseInfo()
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
				"data: {\"id\":\"chatcmpl_1\",\"choices\":[],\"usage\":{\"prompt_tokens\":10000001,\"completion_tokens\":1,\"total_tokens\":10000002}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}

	usage, relayErr := OaiStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.False(t, info.StreamTerminalUsageSeen)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.NotContains(t, recorder.Body.String(), "10000002")
}

func TestOaiResponsesToChatStreamRecoversTerminalUsageAfterClientGone(t *testing.T) {
	oldEnabled := constant.StreamUsageDrainEnabled
	oldMaxConcurrency := constant.StreamUsageDrainMaxConcurrency
	oldMaxPerChannel := constant.StreamUsageDrainMaxPerChannel
	oldTimeout := constant.StreamUsageDrainTimeoutSeconds
	oldStreamingTimeout := constant.StreamingTimeout
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = oldEnabled
		constant.StreamUsageDrainMaxConcurrency = oldMaxConcurrency
		constant.StreamUsageDrainMaxPerChannel = oldMaxPerChannel
		constant.StreamUsageDrainTimeoutSeconds = oldTimeout
		constant.StreamingTimeout = oldStreamingTimeout
	})
	constant.StreamUsageDrainEnabled = true
	constant.StreamUsageDrainMaxConcurrency = 4
	constant.StreamUsageDrainMaxPerChannel = 4
	constant.StreamUsageDrainTimeoutSeconds = 30
	constant.StreamingTimeout = 30

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	wrote := make(chan struct{})
	c.Writer = &openAIStreamWriteSignal{ResponseWriter: c.Writer, wrote: wrote}
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 77, UpstreamModelName: "gpt-test"},
	}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	reader, writer := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: reader}
	type streamResult struct {
		usage *dto.Usage
		err   error
	}
	done := make(chan streamResult, 1)
	go func() {
		usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
		if relayErr != nil {
			done <- streamResult{err: relayErr}
			return
		}
		done <- streamResult{usage: usage}
	}()

	first := `{"type":"response.output_text.delta","delta":"hello","response":{"id":"resp_1","model":"gpt-test"}}`
	terminal := `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","output":[],"usage":{"input_tokens":1200,"output_tokens":25,"total_tokens":1225,"input_tokens_details":{"cached_tokens":1024}}}}`
	_, err := io.WriteString(writer, "data: "+first+"\n\n")
	require.NoError(t, err)
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for converted downstream stream write")
	}
	beforeCancel := recorder.Body.String()
	cancel()
	require.Eventually(t, info.IsStreamDetached, 2*time.Second, time.Millisecond)
	_, err = io.WriteString(writer, "data: "+terminal+"\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.NotNil(t, result.usage)
		require.Equal(t, 1200, result.usage.PromptTokens)
		require.Equal(t, 25, result.usage.CompletionTokens)
		require.Equal(t, 1024, result.usage.PromptTokensDetails.CachedTokens)
		require.Equal(t, "upstream", result.usage.UsageSource)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for converted terminal usage recovery")
	}
	require.Equal(t, beforeCancel, recorder.Body.String())
	require.Equal(t, relaycommon.StreamUsageStateExact, info.GetStreamRecoverySnapshot().UsageState)
}
