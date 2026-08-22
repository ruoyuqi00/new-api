package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestXAIStreamChunkHasSignalRejectsEmptyChunk(t *testing.T) {
	require.False(t, xAIStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{}))
}

func TestXAIStreamChunkHasSignalAcceptsContent(t *testing.T) {
	content := "ok"
	require.True(t, xAIStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content}},
		},
	}))
}

func TestXAITextResponseHasSignalRejectsEmptyResponse(t *testing.T) {
	require.False(t, xAITextResponseHasSignal(ChatCompletionResponse{}))
}

func TestXAITextResponseHasSignalAcceptsContentWithoutUsage(t *testing.T) {
	resp := ChatCompletionResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{Role: "assistant", Content: "ok"},
			},
		},
	}

	require.True(t, xAITextResponseHasSignal(resp))
	require.Equal(t, "ok", xAIResponseText(resp))
}

func TestXAIHandlerSanitizesAmplifiedTextUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "gpt-test",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"xai-1","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10000001,"completion_tokens":1,"total_tokens":10000002}}`)),
	}

	usage, relayErr := xAIHandler(c, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.True(t, info.PreservePreConsumedQuota)
}

func TestXAIStreamHandlerSanitizesAmplifiedTextUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		DisablePing:     true,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "gpt-test",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"xai-1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
				"data: {\"id\":\"xai-1\",\"choices\":[],\"usage\":{\"prompt_tokens\":10000001,\"completion_tokens\":1,\"total_tokens\":10000002}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}

	usage, relayErr := xAIStreamHandler(c, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.True(t, info.PreservePreConsumedQuota)
}
