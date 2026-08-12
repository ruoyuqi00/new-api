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

	info := mappedClientResponseInfo()
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, 1200, usage.PromptTokens)
	require.Equal(t, 25, usage.CompletionTokens)
	require.Equal(t, 1024, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 128, usage.PromptTokensDetails.CacheWriteTokens)
	require.True(t, info.StreamTerminalMarkersRequired)
	require.True(t, info.StreamTerminalSuccess)
	require.True(t, info.StreamTerminalUsageSeen)
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

	info := mappedClientResponseInfo()
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: response.failed"))
	require.Contains(t, recorder.Body.String(), `"sequence_number":6`)
	require.Contains(t, recorder.Body.String(), `"code":"upstream_stream_incomplete"`)
	require.Contains(t, recorder.Body.String(), "The stream ended before completion. Please retry later.")
	require.True(t, info.PreservePreConsumedQuota)
}

func TestOaiResponsesStreamHandlerSanitizesUpstreamTerminalFailure(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	for _, eventType := range []string{"response.failed", "response.incomplete", "error"} {
		t.Run(eventType, func(t *testing.T) {
			ctx, recorder := clientResponseTestContext()
			ctx.Request.URL.Path = "/v1/responses"
			body := "data: {\"type\":\"" + eventType + "\",\"error\":{\"message\":\"top-level-secret\"},\"response\":{\"id\":\"resp_private_request_id\",\"model\":\"upstream-model\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"POST https://secret-upstream.example/v1/responses via 10.20.30.40 channel #73 Authorization Bearer sk-upstream-secret returned <html>private</html>\"},\"incomplete_details\":{\"reason\":\"redirect https://private-redirect.example\"},\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"private-output-marker\"}]}],\"metadata\":{\"private\":\"private-metadata-marker\"}}}\n\n"
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}

			info := mappedClientResponseInfo()
			_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

			require.Nil(t, relayErr)
			publicBody := recorder.Body.String()
			require.Equal(t, 1, strings.Count(publicBody, "event: "+eventType))
			require.Contains(t, publicBody, `"code":"upstream_response_failed"`)
			require.Contains(t, publicBody, "The response failed before completion. Please retry later.")
			for _, secret := range []string{
				"secret-upstream.example",
				"10.20.30.40",
				"channel #73",
				"Authorization",
				"sk-upstream-secret",
				"<html>private</html>",
				"private-redirect.example",
				"private-output-marker",
				"private-metadata-marker",
				"top-level-secret",
				"resp_private_request_id",
			} {
				require.NotContains(t, publicBody, secret)
			}
		})
	}
}

func TestOaiResponsesStreamHandlerTreatsEmptyUsageAsUnconfirmed(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, _ := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"estimated output must remain diagnostic\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty_usage\",\"usage\":{}}}\n\n",
		)),
	}

	info := mappedClientResponseInfo()
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.NotZero(t, usage.CompletionTokens)
	require.False(t, info.StreamTerminalUsageSeen)
	require.True(t, info.PreservePreConsumedQuota)
}

func TestOaiResponsesStreamHandlerClientGoneDoesNotWriteSyntheticFailure(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	requestContext, cancel := context.WithCancel(ctx.Request.Context())
	cancel()
	ctx.Request = ctx.Request.WithContext(requestContext)
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_client_gone\"}}\n\n",
		)),
	}

	info := mappedClientResponseInfo()
	_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.NotContains(t, recorder.Body.String(), "event: response.failed")
	require.True(t, info.PreservePreConsumedQuota)
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
		terminalSuccess bool
		terminalUsage   bool
		wantPreserve    bool
	}{
		{
			name:            "completed",
			body:            "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n",
			terminalEvent:   "event: response.completed",
			failureEventCnt: 0,
			terminalSuccess: true,
			terminalUsage:   true,
		},
		{
			name:            "completed without usage",
			body:            "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"completed\"}}\n\n",
			terminalEvent:   "event: response.completed",
			failureEventCnt: 0,
			terminalSuccess: true,
			wantPreserve:    true,
		},
		{
			name:            "incomplete",
			body:            "data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"incomplete\",\"usage\":{\"input_tokens\":321,\"output_tokens\":12,\"total_tokens\":333,\"input_tokens_details\":{\"cached_tokens\":300}}}}\n\n",
			terminalEvent:   "event: response.incomplete",
			failureEventCnt: 0,
			wantPreserve:    true,
			terminalUsage:   true,
		},
		{
			name:            "failed",
			body:            "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"upstream failed\"}}}\n\n",
			terminalEvent:   "event: response.failed",
			failureEventCnt: 1,
			wantPreserve:    true,
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

			info := mappedClientResponseInfo()
			usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

			require.Nil(t, relayErr)
			require.Equal(t, 1, strings.Count(recorder.Body.String(), tt.terminalEvent))
			require.Equal(t, tt.failureEventCnt, strings.Count(recorder.Body.String(), "event: response.failed"))
			require.Equal(t, tt.wantPreserve, info.PreservePreConsumedQuota)
			require.Equal(t, tt.terminalSuccess, info.StreamTerminalSuccess)
			require.Equal(t, tt.terminalUsage, info.StreamTerminalUsageSeen)
			if tt.name == "incomplete" {
				require.Equal(t, 321, usage.PromptTokens)
				require.Equal(t, 12, usage.CompletionTokens)
				require.Equal(t, 300, usage.PromptTokensDetails.CachedTokens)
			}
		})
	}
}

func TestOaiResponsesStreamHandlerPreservesPublishedResponseIDOnFailure(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_public\",\"model\":\"upstream-model\"}}\n\n" +
				"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_private_failure\",\"error\":{\"message\":\"secret\"}}}\n\n",
		)),
	}

	_, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	publicBody := recorder.Body.String()
	require.Contains(t, publicBody, "resp_public")
	require.NotContains(t, publicBody, "resp_private_failure")
	require.NotContains(t, publicBody, "secret")
}

func TestOaiResponsesHandlerCapturesAffinityResponseID(t *testing.T) {
	ctx, _ := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_buffered","object":"response","status":"completed","model":"upstream-model","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		)),
	}
	info := mappedClientResponseInfo()

	_, relayErr := OaiResponsesHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "resp_buffered", info.ChannelAffinityResponseID)
}

func TestOaiResponsesStreamHandlerCapturesObservedAffinityResponseID(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	tests := []struct {
		name         string
		body         string
		wantID       string
		wantObserved bool
		wantPreserve bool
	}{
		{
			name: "completed",
			body: "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_ok\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ok\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n" +
				"data: [DONE]\n\n",
			wantID:       "resp_ok",
			wantObserved: true,
		},
		{
			name: "incomplete",
			body: "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_incomplete\"}}\n\n" +
				"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_incomplete\"}}\n\n",
			wantID:       "resp_incomplete",
			wantObserved: true,
			wantPreserve: true,
		},
		{
			name:         "eof without terminal",
			body:         "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_eof\"}}\n\n",
			wantID:       "resp_eof",
			wantObserved: true,
			wantPreserve: true,
		},
		{
			name:         "eof without real response id",
			body:         "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n",
			wantPreserve: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := clientResponseTestContext()
			ctx.Request.URL.Path = "/v1/responses"
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			info := mappedClientResponseInfo()

			_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

			require.Nil(t, relayErr)
			require.Equal(t, tt.wantID, info.ChannelAffinityResponseID)
			require.Equal(t, tt.wantObserved, info.ChannelAffinityResponseIDObserved)
			require.Equal(t, tt.wantPreserve, info.PreservePreConsumedQuota)
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
