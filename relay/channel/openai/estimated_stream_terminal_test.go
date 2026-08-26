package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func estimatedGPTUsageForTest() *dto.Usage {
	return &dto.Usage{
		PromptTokens:     1200,
		CompletionTokens: 25,
		TotalTokens:      1225,
		UsageSource:      "estimated",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 1024,
		},
	}
}

func estimatedGPTStreamContext(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, recorder := clientResponseTestContext()
	c.Request = c.Request.WithContext(context.Background())
	c.Request.URL.Path = path
	return c, recorder
}

func TestEmitEstimatedGPTStreamTerminalChatUsesLengthAndUsage(t *testing.T) {
	c, recorder := estimatedGPTStreamContext(t, "/v1/chat/completions")
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.ShouldIncludeUsage = true

	err := EmitEstimatedGPTStreamTerminal(c, info, estimatedGPTUsageForTest(), "chatcmpl_gateway", 123, "gpt-test", "fp-test", 0)

	require.NoError(t, err)
	body := recorder.Body.String()
	require.Contains(t, body, `"finish_reason":"length"`)
	require.Contains(t, body, `"prompt_tokens":1200`)
	require.Contains(t, body, `"completion_tokens":25`)
	require.Contains(t, body, "[DONE]")
	require.NotContains(t, body, "upstream-secret")
}

func TestEmitEstimatedGPTStreamTerminalResponsesUsesIncompleteUsage(t *testing.T) {
	c, recorder := estimatedGPTStreamContext(t, "/v1/responses")
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeResponses
	info.RelayFormat = types.RelayFormatOpenAI

	err := EmitEstimatedGPTStreamTerminal(c, info, estimatedGPTUsageForTest(), "resp_gateway", 0, "gpt-test", "", 8)

	require.NoError(t, err)
	body := recorder.Body.String()
	require.Contains(t, body, "event: response.incomplete")
	require.Contains(t, body, `"status":"incomplete"`)
	require.Contains(t, body, `"input_tokens":1200`)
	require.Contains(t, body, `"output_tokens":25`)
	require.Contains(t, body, `"total_tokens":1225`)
	require.Contains(t, body, `"sequence_number":8`)
	require.NotContains(t, body, "upstream-secret")
	require.NotContains(t, body, "Authorization")
}

func TestEmitEstimatedGPTStreamTerminalSkipsDetachedClient(t *testing.T) {
	c, recorder := estimatedGPTStreamContext(t, "/v1/chat/completions")
	requestContext, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(requestContext)
	cancel()
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI

	err := EmitEstimatedGPTStreamTerminal(c, info, estimatedGPTUsageForTest(), "resp_gateway", 0, "gpt-test", "", 0)

	require.NoError(t, err)
	require.Empty(t, recorder.Body.String())
}

func TestEmitEstimatedGPTStreamTerminalLeavesMediaModesUntouched(t *testing.T) {
	c, recorder := estimatedGPTStreamContext(t, "/v1/images/generations")
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeImagesGenerations
	info.RelayFormat = types.RelayFormatOpenAI

	err := EmitEstimatedGPTStreamTerminal(c, info, estimatedGPTUsageForTest(), "resp_gateway", 0, "gpt-test", "", 0)

	require.NoError(t, err)
	require.Empty(t, recorder.Body.String())
	require.Equal(t, http.MethodPost, c.Request.Method)
	require.True(t, strings.HasPrefix(c.Request.URL.Path, "/v1/images"))
}

func TestOaiStreamHandlerEmitsEstimatedTerminalAfterUpstreamError(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/chat/completions"
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(1200)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_public\",\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n" +
				"data: {\"error\":{\"message\":\"POST https://upstream.invalid Authorization Bearer secret\"}}\n\n",
		)),
	}

	usage, relayErr := OaiStreamHandler(ctx, info, resp)

	require.Error(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
	require.Contains(t, recorder.Body.String(), `"prompt_tokens":1200`)
	require.Contains(t, recorder.Body.String(), "[DONE]")
	require.NotContains(t, recorder.Body.String(), "upstream.invalid")
	require.NotContains(t, recorder.Body.String(), "Authorization")
	require.NotContains(t, recorder.Body.String(), "secret")
}

func TestOaiStreamHandlerEmitsEstimatedTerminalAfterEOFWithoutUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(1200)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_public\",\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n" +
				"data: {\"id\":\"chatcmpl_public\",\"choices\":[]}\n\n" + "data: [DONE]\n\n",
		)),
	}

	usage, relayErr := OaiStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "[DONE]"))
	require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
	require.Contains(t, recorder.Body.String(), `"prompt_tokens":1200`)
}

func TestOaiResponsesStreamHandlerEmitsEstimatedIncompleteAfterMissingUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	info := mappedClientResponseInfo()
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeResponses
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(1200)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"response\":{\"id\":\"resp_public\",\"model\":\"upstream-model\"},\"delta\":\"partial\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_public\",\"model\":\"upstream-model\",\"status\":\"completed\",\"usage\":{}}}\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Contains(t, recorder.Body.String(), "event: response.incomplete")
	require.Contains(t, recorder.Body.String(), `"input_tokens":1200`)
	require.Contains(t, recorder.Body.String(), `"output_tokens":`)
	require.NotContains(t, recorder.Body.String(), "upstream-model")
	require.NotContains(t, recorder.Body.String(), "Authorization")
}
