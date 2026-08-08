package channel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestDoRequestCancelsUpstreamWhenClientDisconnects(t *testing.T) {
	service.InitHttpClient()

	upstreamStarted := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamStarted)
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)

	upstreamRequest, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		resp, requestErr := DoRequest(ctx, upstreamRequest, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		done <- requestErr
	}()

	select {
	case <-upstreamStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request did not start")
	}

	cancel()

	select {
	case requestErr := <-done:
		require.Error(t, requestErr)
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request was not canceled with the client")
	}

	select {
	case <-upstreamCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream handler did not observe cancellation")
	}
}

func TestDoRequestEligibleStreamCancelsBeforeAcceptance(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	upstreamStarted := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	releaseHandler := make(chan struct{})
	var releaseHandlerOnce sync.Once
	releaseUpstreamHandler := func() {
		releaseHandlerOnce.Do(func() { close(releaseHandler) })
	}
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(upstreamStarted)
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
		case <-releaseHandler:
		}
	}))
	t.Cleanup(func() {
		releaseUpstreamHandler()
		server.CloseClientConnections()
		server.Close()
	})
	t.Cleanup(releaseUpstreamHandler)

	requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)

	upstreamRequest, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
		Timings:     relaycommon.NewRelayTimings(),
	}
	info.EnableStreamRecovery()

	done := make(chan error, 1)
	go func() {
		resp, requestErr := DoRequest(ctx, upstreamRequest, info)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		done <- requestErr
	}()

	select {
	case <-upstreamStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()

	select {
	case requestErr := <-done:
		require.Error(t, requestErr)
	case <-time.After(2 * time.Second):
		t.Fatal("eligible upstream request was not canceled before acceptance")
	}
	select {
	case <-upstreamCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream handler did not observe pre-acceptance cancellation")
	}
	require.False(t, info.GetStreamRecoverySnapshot().Accepted)
}

func TestDoRequestEligibleStreamSurvivesClientCancelAfterHeaders(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	terminalReady := make(chan struct{})
	terminalFlushed := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	releaseHandler := make(chan struct{})
	var terminalReadyOnce sync.Once
	var releaseHandlerOnce sync.Once
	releaseTerminal := func() {
		terminalReadyOnce.Do(func() { close(terminalReady) })
	}
	releaseUpstreamHandler := func() {
		releaseHandlerOnce.Do(func() { close(releaseHandler) })
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-terminalReady:
		case <-releaseHandler:
			return
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
		close(terminalFlushed)
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
		case <-releaseHandler:
		}
	}))
	t.Cleanup(func() {
		releaseTerminal()
		releaseUpstreamHandler()
		server.CloseClientConnections()
		server.Close()
	})
	t.Cleanup(releaseUpstreamHandler)
	t.Cleanup(releaseTerminal)

	requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)

	upstreamRequest, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(""))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
		Timings:     relaycommon.NewRelayTimings(),
	}
	info.EnableStreamRecovery()
	t.Cleanup(info.FinishStreamRecovery)

	resp, err := DoRequest(ctx, upstreamRequest, info)
	require.NoError(t, err)
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.True(t, info.GetStreamRecoverySnapshot().Accepted)

	cancel()
	select {
	case <-upstreamCanceled:
		t.Fatal("accepted upstream request was canceled with the downstream client")
	case <-time.After(100 * time.Millisecond):
	}

	releaseTerminal()
	select {
	case <-terminalFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted upstream request did not reach its terminal body")
	}
	require.Equal(t, int32(1), requests.Load())

	info.FinishStreamRecovery()
	select {
	case <-upstreamCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("finishing stream recovery did not cancel the upstream request")
	}
}

func TestDoRequestEligibleStreamDoesNotAcceptNon2xx(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)

	upstreamRequest, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(""))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
		Timings:     relaycommon.NewRelayTimings(),
	}
	info.EnableStreamRecovery()
	t.Cleanup(info.FinishStreamRecovery)

	resp, err := DoRequest(ctx, upstreamRequest, info)
	require.NoError(t, err)
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.False(t, info.GetStreamRecoverySnapshot().Accepted)
}

func TestDoRequestRetryResetsRecoveryAttempt(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	service.InitHttpClient()

	closedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := closedServer.URL
	closedServer.Close()

	requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
		Timings:     relaycommon.NewRelayTimings(),
	}
	info.EnableStreamRecovery()

	failedRequest, err := http.NewRequest(http.MethodPost, closedURL, nil)
	require.NoError(t, err)
	resp, err := DoRequest(ctx, failedRequest, info)
	require.Error(t, err)
	require.Nil(t, resp)
	require.False(t, info.GetStreamRecoverySnapshot().Accepted)

	upstreamCanceled := make(chan struct{})
	releaseHandler := make(chan struct{})
	var releaseHandlerOnce sync.Once
	releaseUpstreamHandler := func() {
		releaseHandlerOnce.Do(func() { close(releaseHandler) })
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
		case <-releaseHandler:
		}
	}))
	t.Cleanup(func() {
		releaseUpstreamHandler()
		server.CloseClientConnections()
		server.Close()
	})
	t.Cleanup(releaseUpstreamHandler)

	retryRequest, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(""))
	require.NoError(t, err)
	resp, err = DoRequest(ctx, retryRequest, info)
	require.NoError(t, err)
	require.NotNil(t, resp)
	retryResp := resp
	t.Cleanup(func() { _ = retryResp.Body.Close() })
	require.True(t, info.GetStreamRecoverySnapshot().Accepted)
	require.Equal(t, int32(1), requests.Load())

	info.FinishStreamRecovery()
	select {
	case <-upstreamCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted retry attempt was not finished")
	}

	requestAfterAcceptance, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(""))
	require.NoError(t, err)
	resp, err = DoRequest(ctx, requestAfterAcceptance, info)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, int32(1), requests.Load(), "accepted recovery must not start another upstream attempt")
}

func TestDoRequestRecordsRelayTimings(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("{}"))
	upstreamRequest, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		Timings:     relaycommon.NewRelayTimings(),
	}
	info.Timings.MarkRequestConversionStart(time.Now().Add(-time.Millisecond))
	resp, err := DoRequest(c, upstreamRequest, info)
	require.NoError(t, err)
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })

	timings := info.Timings.SnapshotMilliseconds()
	require.Greater(t, timings["request_conversion_ms"], 0.0)
	require.Greater(t, timings["upstream_headers_ms"], 0.0)
}
