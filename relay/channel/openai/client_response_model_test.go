package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func mappedClientResponseInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "public-model",
		RelayFormat:     types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "upstream-model",
		},
	}
}

func clientResponseTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return ctx, recorder
}

func TestSendStreamDataReturnsPublicModelForMappedResponse(t *testing.T) {
	ctx, recorder := clientResponseTestContext()

	err := sendStreamData(ctx, mappedClientResponseInfo(), `{"id":"chatcmpl_1","model":"upstream-model","choices":[]}`, false, false)

	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"model":"public-model"`)
	require.NotContains(t, recorder.Body.String(), `"model":"upstream-model"`)
}

func TestOaiResponsesHandlerReturnsPublicModelForMappedResponse(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","model":"upstream-model","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)),
	}

	_, relayErr := OaiResponsesHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "public-model", body["model"])
}

func TestOpenaiHandlerReturnsPublicModelForMappedResponse(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)),
	}

	_, relayErr := OpenaiHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "public-model", body["model"])
}

func TestOpenaiHandlerReplacesAmplifiedUsageWithEstimate(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	info := mappedClientResponseInfo()
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_1","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10000001,"completion_tokens":9000000,"total_tokens":19000001}}`,
		)),
	}

	usage, relayErr := OpenaiHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.Less(t, usage.CompletionTokens, 100)
	require.True(t, info.PreservePreConsumedQuota)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.NotContains(t, recorder.Body.String(), "9000000")
}

func TestOpenaiLegacyCompletionsReplacesAmplifiedUsageWithEstimate(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/completions"
	info := mappedClientResponseInfo()
	info.RelayMode = relayconstant.RelayModeCompletions
	info.RequestURLPath = "/v1/completions"
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"cmpl_1","model":"upstream-model","choices":[{"index":0,"text":"ok","finish_reason":"stop"}],"usage":{"prompt_tokens":10000001,"completion_tokens":1,"total_tokens":10000002}}`,
		)),
	}

	usage, relayErr := OpenaiHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.True(t, info.PreservePreConsumedQuota)
}

func TestOaiChatToResponsesHandlerReturnsPublicModelForMappedResponse(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)),
	}

	_, relayErr := OaiChatToResponsesHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "public-model", body["model"])
}

func TestOaiResponsesToChatHandlerReturnsPublicModelForMappedResponse(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","model":"upstream-model","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)),
	}

	_, relayErr := OaiResponsesToChatHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "public-model", body["model"])
}

func TestOaiResponsesStreamHandlerReturnsPublicModelForMappedEvent(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\ndata: [DONE]\n\n")),
	}

	_, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	require.Contains(t, recorder.Body.String(), `"model":"public-model"`)
	require.NotContains(t, recorder.Body.String(), `"model":"upstream-model"`)
}
