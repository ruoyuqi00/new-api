package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

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

func TestOaiResponsesStreamHandlerEmitsFailureForEOFWithoutTerminalEvent(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"sequence_number\":4,\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"sequence_number\":5,\"delta\":\"partial\"}\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: response.failed"))
	require.Contains(t, recorder.Body.String(), `"sequence_number":6`)
	require.Contains(t, recorder.Body.String(), `"code":"server_error"`)
	require.Contains(t, recorder.Body.String(), "Upstream stream ended before completion.")
}

func TestOaiResponsesStreamHandlerDoesNotDuplicateTerminalEvent(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	tests := []struct {
		name            string
		body            string
		terminalEvent   string
		failureEventCnt int
	}{
		{
			name:            "completed",
			body:            "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n",
			terminalEvent:   "event: response.completed",
			failureEventCnt: 0,
		},
		{
			name:            "incomplete",
			body:            "data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"incomplete\"}}\n\n",
			terminalEvent:   "event: response.incomplete",
			failureEventCnt: 0,
		},
		{
			name:            "failed",
			body:            "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"upstream failed\"}}}\n\n",
			terminalEvent:   "event: response.failed",
			failureEventCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := clientResponseTestContext()
			ctx.Request.URL.Path = "/v1/responses"
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			_, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

			require.Nil(t, relayErr)
			require.Equal(t, 1, strings.Count(recorder.Body.String(), tt.terminalEvent))
			require.Equal(t, tt.failureEventCnt, strings.Count(recorder.Body.String(), "event: response.failed"))
		})
	}
}

func TestSendResponsesStreamDataReturnsWriteError(t *testing.T) {
	ctx, _ := clientResponseTestContext()
	requestContext, cancel := context.WithCancel(ctx.Request.Context())
	cancel()
	ctx.Request = ctx.Request.WithContext(requestContext)

	err := sendResponsesStreamData(ctx, dto.ResponsesStreamResponse{Type: "response.output_text.delta"}, `{"type":"response.output_text.delta","delta":"ignored"}`)

	require.Error(t, err)
}
