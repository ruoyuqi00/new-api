package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

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
	info.RelayMode = relayconstant.RelayModeResponses
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, 1200, usage.PromptTokens)
	require.Equal(t, 25, usage.CompletionTokens)
	require.Equal(t, 1024, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 128, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, "upstream", usage.UsageSource)
	require.True(t, info.StreamTerminalMarkersRequired)
	require.True(t, info.StreamTerminalSuccess)
	require.True(t, info.StreamTerminalUsageSeen)
}

func TestOaiResponsesStreamHandlerRejectsAmplifiedTerminalUsage(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	info := mappedClientResponseInfo()
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.done\",\"response\":{\"id\":\"resp_bad_usage\",\"model\":\"upstream-model\",\"output\":[],\"usage\":{\"input_tokens\":10000001,\"output_tokens\":1,\"total_tokens\":10000002}}}\n\ndata: [DONE]\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.False(t, info.StreamTerminalUsageSeen)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.NotContains(t, recorder.Body.String(), "10000002")
}

func TestOaiResponsesHandlerLeavesImageUsageUnchanged(t *testing.T) {
	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	info := mappedClientResponseInfo()
	body := `{"id":"resp_image","model":"upstream-model","output":[{"type":"image_generation_call","id":"img_1","status":"completed","result":"data"}],"usage":{"input_tokens":10000001,"output_tokens":1,"total_tokens":10000002}}`
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}

	usage, relayErr := OaiResponsesHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, 10_000_001, usage.PromptTokens)
	require.Contains(t, recorder.Body.String(), `"input_tokens":10000001`)
	require.False(t, info.PreservePreConsumedQuota)
}

func TestOaiResponsesStreamHandlerEmitsFixedCodexPreludeFirstForGPT(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	ctx.Request.Header.Set("Originator", "codex_cli_rs")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Codex-Turn-State":   {"turn-state"},
			"X-Reasoning-Included": {"true"},
		},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"codex.rate_limits\",\"plan_type\":\"team\",\"rate_limits\":{\"allowed\":false,\"limit_reached\":true,\"primary\":{\"used_percent\":100,\"window_minutes\":10080,\"reset_at\":1787126358},\"secondary\":null},\"credits\":{\"has_credits\":false,\"unlimited\":false,\"balance\":null}}\n\n" +
				"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_codex\",\"model\":\"upstream-model\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_codex\",\"model\":\"upstream-model\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}
	info := mappedClientResponseInfo()
	info.OriginModelName = "gpt-test"

	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	publicBody := recorder.Body.String()
	const fixedData = `{"type":"codex.rate_limits","plan_type":"pro","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":null},"credits":null}`
	require.Equal(t, 1, strings.Count(publicBody, fixedData))
	require.NotContains(t, publicBody, `"plan_type":"team"`)
	require.Contains(t, publicBody, "event: response.created")
	require.Contains(t, publicBody, "event: response.completed")
	require.True(t, strings.HasPrefix(publicBody, "event: codex.response.metadata\n"))
	require.Less(t,
		strings.Index(publicBody, "event: codex.response.metadata"),
		strings.Index(publicBody, "event: response.created"),
	)
	require.Equal(t, "turn-state", recorder.Header().Get("X-Codex-Turn-State"))
	require.Equal(t, "true", recorder.Header().Get("X-Reasoning-Included"))
	require.Equal(t, 2, usage.TotalTokens)
}

func TestOaiResponsesStreamHandlerUsesGPTCodexClientSignals(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	tests := []struct {
		name       string
		headerName string
		headerVal  string
		wantEvent  bool
	}{
		{name: "originator", headerName: "Originator", headerVal: "Codex CLI", wantEvent: true},
		{name: "user agent", headerName: "User-Agent", headerVal: "codex-cli/1.0", wantEvent: true},
		{name: "turn metadata", headerName: "X-Codex-Turn-Metadata", headerVal: `{"session_id":"test"}`, wantEvent: true},
		{name: "beta features", headerName: "X-Codex-Beta-Features", headerVal: "responses", wantEvent: true},
		{name: "session id alone", headerName: "Session_id", headerVal: "session-test", wantEvent: false},
		{name: "ordinary client", wantEvent: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := clientResponseTestContext()
			ctx.Request.URL.Path = "/v1/responses"
			if tt.headerName != "" {
				ctx.Request.Header.Set(tt.headerName, tt.headerVal)
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_codex\",\"model\":\"upstream-model\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n" +
						"data: [DONE]\n\n",
				)),
			}
			info := mappedClientResponseInfo()
			info.OriginModelName = "gpt-test"

			_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

			require.Nil(t, relayErr)
			require.Equal(t, tt.wantEvent, strings.Contains(recorder.Body.String(), "event: codex.response.metadata"))
		})
	}
}

func TestOaiResponsesStreamHandlerSuppressesUpstreamCodexRateLimitsForOrdinaryGPTClient(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"codex.rate_limits\",\"plan_type\":\"team\",\"rate_limits\":{\"primary\":null,\"secondary\":null},\"credits\":null}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_gpt\",\"model\":\"upstream-model\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}
	info := mappedClientResponseInfo()
	info.OriginModelName = "gpt-test"

	_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	publicBody := recorder.Body.String()
	require.NotContains(t, publicBody, "event: codex.response.metadata")
	require.NotContains(t, publicBody, `"type":"codex.rate_limits"`)
	require.NotContains(t, publicBody, `"plan_type":"team"`)
	require.Contains(t, publicBody, "event: response.completed")
}

func TestOaiResponsesStreamHandlerLeavesNonGPTChannelUnchanged(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	ctx.Request.Header.Set("Originator", "codex_cli_rs")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"codex.rate_limits\",\"plan_type\":\"team\",\"rate_limits\":{\"primary\":null,\"secondary\":null},\"credits\":null}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_other\",\"model\":\"upstream-model\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n" +
				"data: [DONE]\n\n",
		)),
	}
	info := mappedClientResponseInfo()
	info.OriginModelName = "claude-test"

	_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	publicBody := recorder.Body.String()
	require.NotContains(t, publicBody, "event: codex.response.metadata")
	require.Contains(t, publicBody, `"type":"codex.rate_limits"`)
	require.Contains(t, publicBody, `"plan_type":"team"`)
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
	info.RelayMode = relayconstant.RelayModeResponses
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: response.incomplete"))
	require.Contains(t, recorder.Body.String(), `"sequence_number":6`)
	require.Contains(t, recorder.Body.String(), `"code":"upstream_stream_incomplete"`)
	require.Contains(t, recorder.Body.String(), "The stream ended before completion. Please retry later.")
	require.Contains(t, recorder.Body.String(), `"input_tokens":`)
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
			if eventType == "error" {
				body = "data: {\"type\":\"error\",\"code\":\"server_error\",\"message\":\"POST https://secret-upstream.example Authorization Bearer sk-upstream-secret\",\"param\":\"private-param\",\"sequence_number\":7}\n\n"
			}
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}

			info := mappedClientResponseInfo()
			info.RelayMode = relayconstant.RelayModeResponses
			_, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

			require.Nil(t, relayErr)
			publicBody := recorder.Body.String()
			require.Equal(t, 1, strings.Count(publicBody, "event: response.incomplete"))
			require.Contains(t, publicBody, `"code":"upstream_stream_incomplete"`)
			require.Contains(t, publicBody, "The stream ended before completion. Please retry later.")
			require.Contains(t, publicBody, `"input_tokens":`)
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
				"private-param",
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
	info.SetEstimatePromptTokens(1234)
	usage, relayErr := OaiResponsesStreamHandler(ctx, info, resp)

	require.Nil(t, relayErr)
	require.NotZero(t, usage.CompletionTokens)
	require.Equal(t, 1234, usage.PromptTokens)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Zero(t, usage.PromptTokensDetails.CachedTokens)
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
			terminalEvent:   "event: response.incomplete",
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
			terminalEvent:   "event: response.incomplete",
			failureEventCnt: 0,
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
			info.RelayMode = relayconstant.RelayModeResponses
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
