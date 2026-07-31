package openai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeCompletedImageGenerationStatus(t *testing.T) {
	input := []byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","status":"generating","result":"image-data"}}`)

	got, changed := normalizeCompletedImageGenerationStatus(input)

	require.True(t, changed)
	require.Equal(t, "completed", gjson.GetBytes(got, "item.status").String())
}

func TestNormalizeCompletedImageGenerationStatusLeavesPendingItemWithoutResult(t *testing.T) {
	input := []byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","status":"generating"}}`)

	got, changed := normalizeCompletedImageGenerationStatus(input)

	require.False(t, changed)
	require.Equal(t, input, got)
}

func TestOaiResponsesStreamHandlerParsesResponseDoneUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, _ := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.done\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"output\":[],\"usage\":{\"input_tokens\":1200,\"output_tokens\":25,\"total_tokens\":1225,\"input_tokens_details\":{\"cached_tokens\":1024,\"cache_write_tokens\":128}}}}\n\ndata: [DONE]\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	require.Equal(t, 1200, usage.PromptTokens)
	require.Equal(t, 25, usage.CompletionTokens)
	require.Equal(t, 1024, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 128, usage.PromptTokensDetails.CacheWriteTokens)
}
