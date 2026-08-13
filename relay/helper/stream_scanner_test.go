package helper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
	if constant.StreamingTimeout == 0 {
		constant.StreamingTimeout = 30
	}
}

func setupStreamTest(t *testing.T, body io.Reader) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	return c, resp, info
}

func buildSSEBody(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "data: {\"id\":%d,\"choices\":[{\"delta\":{\"content\":\"token_%d\"}}]}\n", i, i)
	}
	b.WriteString("data: [DONE]\n")
	return b.String()
}

func configureStreamScannerRecoveryTest(t *testing.T) {
	t.Helper()

	originalEnabled := constant.StreamUsageDrainEnabled
	originalMaxConcurrency := constant.StreamUsageDrainMaxConcurrency
	originalMaxPerChannel := constant.StreamUsageDrainMaxPerChannel
	originalTimeoutSeconds := constant.StreamUsageDrainTimeoutSeconds
	originalMaxBytesMB := constant.StreamUsageDrainMaxBytesMB
	t.Cleanup(func() {
		constant.StreamUsageDrainEnabled = originalEnabled
		constant.StreamUsageDrainMaxConcurrency = originalMaxConcurrency
		constant.StreamUsageDrainMaxPerChannel = originalMaxPerChannel
		constant.StreamUsageDrainTimeoutSeconds = originalTimeoutSeconds
		constant.StreamUsageDrainMaxBytesMB = originalMaxBytesMB
	})

	constant.StreamUsageDrainEnabled = true
	constant.StreamUsageDrainMaxConcurrency = 2
	constant.StreamUsageDrainMaxPerChannel = 1
	constant.StreamUsageDrainTimeoutSeconds = 30
	constant.StreamUsageDrainMaxBytesMB = 1
}

func waitStreamScannerSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	guard := time.NewTimer(2 * time.Second)
	defer guard.Stop()
	select {
	case <-signal:
	case <-guard.C:
		require.FailNow(t, message)
	}
}

func waitStreamScannerDetached(t *testing.T, info *relaycommon.RelayInfo) {
	t.Helper()
	guard := time.NewTimer(2 * time.Second)
	defer guard.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if info.GetStreamRecoverySnapshot().Detached {
			return
		}
		select {
		case <-ticker.C:
		case <-guard.C:
			require.FailNow(t, "stream did not detach")
		}
	}
}

type synchronizedStreamWriter struct {
	gin.ResponseWriter
	firstModelStarted chan struct{}
	releaseFirstModel chan struct{}
	secondModelWrote  chan struct{}
	firstModelOnce    sync.Once
	secondModelOnce   sync.Once
}

type pendingOnTryLock struct {
	pending  *atomic.Int64
	unlocked atomic.Bool
}

type stagedCountingBody struct {
	first         []byte
	rest          []byte
	release       chan struct{}
	closed        chan struct{}
	closeOnce     sync.Once
	info          *relaycommon.RelayInfo
	detachedBytes atomic.Int64
}

type terminalCancellationBody struct {
	data []byte
	done <-chan struct{}
	err  error
}

type eagerTerminalErrorBody struct {
	data        []byte
	err         error
	errReturned chan struct{}
	errOnce     sync.Once
}

func (b *eagerTerminalErrorBody) Read(p []byte) (int, error) {
	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		return n, nil
	}
	b.errOnce.Do(func() { close(b.errReturned) })
	return 0, b.err
}

func (b *eagerTerminalErrorBody) Close() error {
	return nil
}

func (b *terminalCancellationBody) Read(p []byte) (int, error) {
	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		return n, nil
	}
	<-b.done
	return 0, b.err
}

func (b *terminalCancellationBody) Close() error {
	return nil
}

func (b *stagedCountingBody) Read(p []byte) (int, error) {
	if len(b.first) > 0 {
		n := copy(p, b.first)
		b.first = b.first[n:]
		return n, nil
	}
	select {
	case <-b.release:
	case <-b.closed:
		return 0, io.ErrClosedPipe
	}
	if len(b.rest) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.rest)
	b.rest = b.rest[n:]
	if b.info.IsStreamDetached() {
		b.detachedBytes.Add(int64(n))
	}
	return n, nil
}

func (b *stagedCountingBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

func (l *pendingOnTryLock) TryLock() bool {
	l.pending.Add(1)
	return true
}

func (l *pendingOnTryLock) Unlock() {
	l.unlocked.Store(true)
}

func (w *synchronizedStreamWriter) Write(data []byte) (int, error) {
	if strings.HasPrefix(string(data), "data: first") {
		w.firstModelOnce.Do(func() { close(w.firstModelStarted) })
		<-w.releaseFirstModel
	}
	n, err := w.ResponseWriter.Write(data)
	if strings.HasPrefix(string(data), "data: second") {
		w.secondModelOnce.Do(func() { close(w.secondModelWrote) })
	}
	return n, err
}

func TestTryWriteStreamPing(t *testing.T) {
	t.Run("pending before acquisition", func(t *testing.T) {
		var writeMutex sync.Mutex
		var pending atomic.Int64
		pending.Store(1)
		called := false

		wrote, err := tryWriteStreamPing(&writeMutex, &pending, func() error {
			called = true
			return nil
		})

		require.NoError(t, err)
		assert.False(t, wrote)
		assert.False(t, called)
	})

	t.Run("pending during acquisition", func(t *testing.T) {
		var pending atomic.Int64
		writeMutex := &pendingOnTryLock{pending: &pending}
		called := false

		wrote, err := tryWriteStreamPing(writeMutex, &pending, func() error {
			called = true
			return nil
		})

		require.NoError(t, err)
		assert.False(t, wrote)
		assert.False(t, called)
		assert.True(t, writeMutex.unlocked.Load())
	})

	t.Run("idle", func(t *testing.T) {
		var writeMutex sync.Mutex
		var pending atomic.Int64
		called := false

		wrote, err := tryWriteStreamPing(&writeMutex, &pending, func() error {
			called = true
			return nil
		})

		require.NoError(t, err)
		assert.True(t, wrote)
		assert.True(t, called)
	})
}

func TestStreamScannerHandlerPingErrorArbitration(t *testing.T) {
	t.Run("client cancellation", func(t *testing.T) {
		requestContext, cancelRequest := context.WithCancel(context.Background())
		cancelRequest()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(requestContext)
		info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}

		assert.False(t, handleStreamPingError(c, info, fmt.Errorf("ping: %w", context.Canceled)))
		assert.Equal(t, relaycommon.StreamEndReasonNone, info.StreamStatus.EndReason)
	})

	t.Run("cancellation error with live request", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
		info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}

		assert.True(t, handleStreamPingError(c, info, fmt.Errorf("ping: %w", context.Canceled)))
		assert.Equal(t, relaycommon.StreamEndReasonPingFail, info.StreamStatus.EndReason)
	})

	t.Run("deadline error with live request", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
		info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}

		assert.True(t, handleStreamPingError(c, info, fmt.Errorf("ping: %w", context.DeadlineExceeded)))
		assert.Equal(t, relaycommon.StreamEndReasonPingFail, info.StreamStatus.EndReason)
	})

	t.Run("live client write failure", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
		info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
		pingErr := errors.New("broken response writer")

		assert.True(t, handleStreamPingError(c, info, pingErr))
		assert.Equal(t, relaycommon.StreamEndReasonPingFail, info.StreamStatus.EndReason)
		assert.ErrorIs(t, info.StreamStatus.EndError, pingErr)
	})
}

// ---------- Basic correctness ----------

func TestStreamScannerHandler_NilInputs(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	StreamScannerHandler(c, nil, info, func(data string, sr *StreamResult) {})
	StreamScannerHandler(c, &http.Response{Body: io.NopCloser(strings.NewReader(""))}, info, nil)
}

func TestCopyCodexSSEHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{Header: http.Header{
		"X-Reasoning-Included": {"true"},
		"X-Codex-Turn-State":   {"state-a", "state-b"},
		"X-Unrelated":          {"ignored"},
	}}

	copyCodexSSEHeaders(c, resp)

	assert.Equal(t, "true", recorder.Header().Get("X-Reasoning-Included"))
	assert.Equal(t, []string{"state-a", "state-b"}, recorder.Header().Values("X-Codex-Turn-State"))
	assert.Empty(t, recorder.Header().Get("X-Unrelated"))
}

func TestNewStreamScanner_AllowsLargeStreamLine(t *testing.T) {
	oldBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = oldBufferMB
	})

	payload := strings.Repeat("x", 128<<10)
	scanner := NewStreamScanner(strings.NewReader("data: " + payload + "\n"))
	scanner.Split(bufio.ScanLines)

	require.True(t, scanner.Scan())
	assert.Equal(t, "data: "+payload, scanner.Text())
	require.NoError(t, scanner.Err())
}

func TestStreamScannerHandler_EmptyBody(t *testing.T) {
	t.Parallel()

	c, resp, info := setupStreamTest(t, strings.NewReader(""))

	var called atomic.Bool
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		called.Store(true)
	})

	assert.False(t, called.Load(), "handler should not be called for empty body")
}

func TestStreamScannerHandlerExactTerminalCancellationIsDone(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	c, _, info := setupStreamTest(t, nil)
	info.DisablePing = true
	info.ChannelMeta.ChannelId = 32
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(c.Request.Context())
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	body := &terminalCancellationBody{
		data: []byte("data: terminal\n"),
		done: upstream.Done(),
		err:  errors.New("http2: response body closed"),
	}
	StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, _ *StreamResult) {
		if data == "terminal" {
			info.MarkStreamTerminalUsage()
		}
	})

	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.False(t, info.StreamStatus.HasErrors())
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestStreamScannerHandlerLateExactTerminalOverridesQueuedScannerError(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	c, _, info := setupStreamTest(t, nil)
	info.DisablePing = true
	info.ChannelMeta.ChannelId = 33
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(c.Request.Context())
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	errReturned := make(chan struct{})
	body := &eagerTerminalErrorBody{
		data:        []byte("data: terminal\n"),
		err:         errors.New("http2: response body closed"),
		errReturned: errReturned,
	}
	StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, _ *StreamResult) {
		if data == "terminal" {
			<-errReturned
			info.MarkStreamTerminalUsage()
		}
	})

	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.NoError(t, info.StreamStatus.EndError)
}

func TestStreamScannerHandlerPreTerminalReadErrorRemainsScannerError(t *testing.T) {
	c, _, info := setupStreamTest(t, nil)
	reader, writer := io.Pipe()
	require.NoError(t, writer.CloseWithError(errors.New("upstream read failed")))

	StreamScannerHandler(c, &http.Response{Body: reader}, info, func(string, *StreamResult) {})

	assert.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
	require.Error(t, info.StreamStatus.EndError)
}

func TestStreamScannerHandlerCapturesRawUpstreamModel(t *testing.T) {
	body := "data: {\"type\":\"response.created\",\"response\":{\"model\":\"upstream-model\"}}\n" +
		"data: [DONE]\n"
	c, resp, info := setupStreamTest(t, strings.NewReader(body))
	info.UpstreamModelName = "forwarded-model"

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	assert.Equal(t, "forwarded-model", info.ForwardedModelName)
	assert.Equal(t, "upstream-model", info.ActualResponseModel)
}

func TestStreamScannerHandler_1000Chunks(t *testing.T) {
	t.Parallel()

	const numChunks = 1000
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(numChunks), count.Load())
	assert.Equal(t, numChunks, info.ReceivedResponseCount)
}

func TestStreamScannerHandler_OrderPreserved(t *testing.T) {
	t.Parallel()

	const numChunks = 500
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var mu sync.Mutex
	received := make([]string, 0, numChunks)

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		mu.Lock()
		received = append(received, data)
		mu.Unlock()
	})

	require.Equal(t, numChunks, len(received))
	for i := 0; i < numChunks; i++ {
		expected := fmt.Sprintf("{\"id\":%d,\"choices\":[{\"delta\":{\"content\":\"token_%d\"}}]}", i, i)
		assert.Equal(t, expected, received[i], "chunk %d out of order", i)
	}
}

func TestStreamScannerHandler_DoneStopsScanner(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(50) + "data: should_not_appear\n"
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(50), count.Load(), "data after [DONE] must not be processed")
}

func TestStreamScannerHandler_StopStopsStream(t *testing.T) {
	t.Parallel()

	const numChunks = 200
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	const stopAt int64 = 50
	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= stopAt {
			sr.Stop(fmt.Errorf("fatal at %d", n))
		}
	})

	assert.Equal(t, stopAt, count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
}

func TestStreamScannerHandler_SkipsNonDataLines(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	b.WriteString(": comment line\n")
	b.WriteString("event: message\n")
	b.WriteString("id: 12345\n")
	b.WriteString("retry: 5000\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "data: payload_%d\n", i)
		b.WriteString(": interleaved comment\n")
	}
	b.WriteString("data: [DONE]\n")

	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(100), count.Load())
}

func TestStreamScannerHandler_DataWithExtraSpaces(t *testing.T) {
	t.Parallel()

	body := "data:   {\"trimmed\":true}  \ndata: [DONE]\n"
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var got string
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		got = data
	})

	assert.Equal(t, "{\"trimmed\":true}", got)
}

func TestStreamScannerHandlerAcceptedDisconnectContinuesDetached(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 21},
	}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	firstHandled := make(chan struct{})
	secondHandled := make(chan struct{})
	callbackErrors := make(chan error, 2)
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		defer close(callbackErrors)
		StreamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			if !info.IsStreamDetached() {
				callbackErrors <- StringData(c, data)
			}
			switch data {
			case "first":
				close(firstHandled)
			case "second":
				info.MarkStreamTerminalUsage()
				close(secondHandled)
			}
		})
	}()

	_, err := fmt.Fprint(writer, "data: first\n")
	require.NoError(t, err)
	waitStreamScannerSignal(t, firstHandled, "first model event was not forwarded")
	cancelRequest()
	_, err = fmt.Fprint(writer, "data: second\ndata: [DONE]\n")
	require.NoError(t, err)
	waitStreamScannerSignal(t, secondHandled, "detached model event was not parsed")
	waitStreamScannerSignal(t, handlerDone, "accepted detached stream did not finish")
	for callbackErr := range callbackErrors {
		require.NoError(t, callbackErr)
	}

	assert.Contains(t, recorder.Body.String(), "first")
	assert.NotContains(t, recorder.Body.String(), "second")
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	snapshot := info.GetStreamRecoverySnapshot()
	assert.True(t, snapshot.Detached)
	assert.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)

	probe := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 21}}
	probe.EnableStreamRecovery()
	probe.StartStreamRecoveryAttempt(context.Background())
	probe.MarkStreamAccepted()
	require.True(t, probe.TryDetachStream(), "completed drain did not release its slot")
	probe.FinishStreamRecovery()
}

func TestStreamScannerHandlerCountsRawDetachedLines(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 26}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			if data == "first" {
				close(firstHandled)
			}
		})
	}()
	require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: first\n"); return err }())
	waitStreamScannerSignal(t, firstHandled, "raw-line test did not handle its first event")
	cancelRequest()
	waitStreamScannerDetached(t, info)

	detachedBody := ": lf\n: crlf\r\ndata: [DONE]\r\n"
	require.NoError(t, func() error { _, err := fmt.Fprint(writer, detachedBody); return err }())
	waitStreamScannerSignal(t, handlerDone, "raw-line drain did not finish")

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, int64(len(detachedBody)), snapshot.DrainedBytes)
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultUpstreamError, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestStreamScannerHandlerDetachedUpstreamEndWithoutUsageIsUnknown(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		closeWrite func(*io.PipeWriter) error
	}{
		{name: "EOF", closeWrite: func(writer *io.PipeWriter) error { return writer.Close() }},
		{name: "scanner error", closeWrite: func(writer *io.PipeWriter) error { return writer.CloseWithError(errors.New("upstream read failed")) }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			configureStreamScannerRecoveryTest(t)

			requestContext, cancelRequest := context.WithCancel(context.Background())
			t.Cleanup(cancelRequest)
			reader, writer := io.Pipe()
			t.Cleanup(func() {
				_ = reader.Close()
				_ = writer.Close()
			})
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
			info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 27}}
			info.EnableStreamRecovery()
			info.StartStreamRecoveryAttempt(requestContext)
			info.MarkStreamAccepted()
			t.Cleanup(info.FinishStreamRecovery)

			firstHandled := make(chan struct{})
			handlerDone := make(chan struct{})
			go func() {
				defer close(handlerDone)
				StreamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
					if data == "first" {
						close(firstHandled)
					}
				})
			}()
			require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: first\n"); return err }())
			waitStreamScannerSignal(t, firstHandled, "upstream-end test did not handle its first event")
			cancelRequest()
			waitStreamScannerDetached(t, info)
			require.NoError(t, testCase.closeWrite(writer))
			waitStreamScannerSignal(t, handlerDone, "detached upstream end did not finish")

			snapshot := info.GetStreamRecoverySnapshot()
			assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
			assert.Equal(t, relaycommon.StreamDrainResultUpstreamError, snapshot.DrainResult)
			assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
		})
	}
}

func TestStreamScannerHandlerCapacityFallbackClosesUpstream(t *testing.T) {
	configureStreamScannerRecoveryTest(t)
	constant.StreamUsageDrainMaxConcurrency = 1

	holder := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 22}}
	holder.EnableStreamRecovery()
	holder.StartStreamRecoveryAttempt(context.Background())
	holder.MarkStreamAccepted()
	require.True(t, holder.TryDetachStream())
	t.Cleanup(holder.FinishStreamRecovery)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 22}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()

	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			close(firstHandled)
		})
	}()
	require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: first\n"); return err }())
	waitStreamScannerSignal(t, firstHandled, "capacity test did not handle its first event")
	cancelRequest()
	waitStreamScannerSignal(t, handlerDone, "capacity fallback queued instead of returning")
	_, err := fmt.Fprint(writer, "data: second\n")
	require.ErrorIs(t, err, io.ErrClosedPipe)

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultCapacity, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)

	holder.FinishStreamRecovery()
	probe := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 22}}
	probe.EnableStreamRecovery()
	probe.StartStreamRecoveryAttempt(context.Background())
	probe.MarkStreamAccepted()
	require.True(t, probe.TryDetachStream(), "capacity accounting did not return to baseline")
	probe.FinishStreamRecovery()
}

func TestStreamScannerHandlerDrainSizeLimitClosesUpstream(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 23}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)
	body := &stagedCountingBody{
		first:   []byte("data: first\n"),
		rest:    []byte(strings.Repeat("x", (1<<20)+1) + "\n"),
		release: make(chan struct{}),
		closed:  make(chan struct{}),
		info:    info,
	}
	t.Cleanup(func() { _ = body.Close() })

	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, sr *StreamResult) {
			if data == "first" {
				close(firstHandled)
			}
		})
	}()
	waitStreamScannerSignal(t, firstHandled, "size test did not handle its first event")
	cancelRequest()
	waitStreamScannerDetached(t, info)
	close(body.release)
	waitStreamScannerSignal(t, handlerDone, "size-limited drain did not return")
	select {
	case <-body.closed:
	default:
		require.FailNow(t, "size-limited drain did not close the upstream body")
	}

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultSizeLimit, snapshot.DrainResult)
	assert.Equal(t, int64(1<<20), snapshot.DrainedBytes)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestStreamDrainReaderCapsUpstreamReadsAfterDetach(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 29}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	body := &stagedCountingBody{
		first:   []byte("data: first\n"),
		rest:    []byte(strings.Repeat("x", 2<<20) + "\r\n"),
		release: make(chan struct{}),
		closed:  make(chan struct{}),
		info:    info,
	}
	t.Cleanup(func() { _ = body.Close() })
	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, sr *StreamResult) {
			if data == "first" {
				close(firstHandled)
			}
		})
	}()
	waitStreamScannerSignal(t, firstHandled, "drain-reader test did not handle its first event")
	cancelRequest()
	waitStreamScannerDetached(t, info)
	close(body.release)
	waitStreamScannerSignal(t, handlerDone, "drain reader did not stop at its byte budget")

	const oneMegabyte = int64(1 << 20)
	assert.Equal(t, oneMegabyte, body.detachedBytes.Load())
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, oneMegabyte, snapshot.DrainedBytes)
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultSizeLimit, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestStreamDrainReaderExactBudgetTerminalUsageStaysExact(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 30}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	const oneMegabyte = 1 << 20
	const prefix = "data: "
	const suffix = "terminal\n"
	exactTerminalLine := prefix + strings.Repeat("x", oneMegabyte-len(prefix)-len(suffix)) + suffix
	require.Len(t, exactTerminalLine, oneMegabyte)
	body := &stagedCountingBody{
		first:   []byte("data: first\n"),
		rest:    []byte(exactTerminalLine),
		release: make(chan struct{}),
		closed:  make(chan struct{}),
		info:    info,
	}
	t.Cleanup(func() { _ = body.Close() })
	firstHandled := make(chan struct{})
	terminalHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, sr *StreamResult) {
			if data == "first" {
				close(firstHandled)
			}
			if strings.HasSuffix(data, "terminal") {
				info.MarkStreamTerminalUsage()
				close(terminalHandled)
			}
		})
	}()
	waitStreamScannerSignal(t, firstHandled, "exact-budget test did not handle its first event")
	cancelRequest()
	waitStreamScannerDetached(t, info)
	close(body.release)
	waitStreamScannerSignal(t, terminalHandled, "exact-budget terminal usage was not handled")
	waitStreamScannerSignal(t, handlerDone, "exact-budget terminal stream did not finish")

	assert.Equal(t, int64(oneMegabyte), body.detachedBytes.Load())
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, int64(oneMegabyte), snapshot.DrainedBytes)
	assert.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestStreamDrainReaderExactBudgetWithoutTerminalIsSizeLimit(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 31}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	const oneMegabyte = 1 << 20
	body := &stagedCountingBody{
		first:   []byte("data: first\n"),
		rest:    []byte(strings.Repeat("x", oneMegabyte)),
		release: make(chan struct{}),
		closed:  make(chan struct{}),
		info:    info,
	}
	t.Cleanup(func() { _ = body.Close() })
	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, sr *StreamResult) {
			if data == "first" {
				close(firstHandled)
			}
		})
	}()
	waitStreamScannerSignal(t, firstHandled, "exact nonterminal test did not handle its first event")
	cancelRequest()
	waitStreamScannerDetached(t, info)
	close(body.release)
	waitStreamScannerSignal(t, handlerDone, "exact nonterminal stream did not finish")

	assert.Equal(t, int64(oneMegabyte), body.detachedBytes.Load())
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, int64(oneMegabyte), snapshot.DrainedBytes)
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultSizeLimit, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestStreamScannerHandlerDrainTimeoutClosesUpstream(t *testing.T) {
	configureStreamScannerRecoveryTest(t)
	constant.StreamUsageDrainTimeoutSeconds = 1

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 24}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	firstHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		StreamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			close(firstHandled)
		})
	}()
	require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: first\n"); return err }())
	waitStreamScannerSignal(t, firstHandled, "timeout test did not handle its first event")
	cancelRequest()
	waitStreamScannerSignal(t, handlerDone, "recovery timeout did not wake the scanner handler")
	_, err := fmt.Fprint(writer, "data: second\n")
	require.ErrorIs(t, err, io.ErrClosedPipe)

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, relaycommon.StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultTimeout, snapshot.DrainResult)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)

	probe := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 24}}
	probe.EnableStreamRecovery()
	probe.StartStreamRecoveryAttempt(context.Background())
	probe.MarkStreamAccepted()
	require.True(t, probe.TryDetachStream(), "timed-out drain did not release its slot")
	probe.FinishStreamRecovery()
}

func TestStreamScannerHandlerModelWriteHasPriorityOverPing(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	synchronizedWriter := &synchronizedStreamWriter{
		ResponseWriter:    c.Writer,
		firstModelStarted: make(chan struct{}),
		releaseFirstModel: make(chan struct{}),
		secondModelWrote:  make(chan struct{}),
	}
	c.Writer = synchronizedWriter
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 25}}
	firstCallbackBlocked := make(chan struct{})
	releaseFirstCallback := make(chan struct{})
	secondQueued := make(chan struct{})
	pingTicks := make(chan time.Time, 1)
	pingTickDone := make(chan struct{})
	cleanupPending := make(chan int64, 1)
	var pingCalled atomic.Bool
	callbackErrors := make(chan error, 2)

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		defer close(callbackErrors)
		streamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			callbackErrors <- StringData(c, data)
			if data == "first" {
				close(firstCallbackBlocked)
				<-releaseFirstCallback
			}
		}, streamScannerOptions{
			pingTicks: pingTicks,
			writePing: func(c *gin.Context) error {
				pingCalled.Store(true)
				return PingData(c)
			},
			pingTickDone: func() {
				close(pingTickDone)
			},
			dataQueued: func(data string) {
				if data == "second" {
					close(secondQueued)
				}
			},
			cleanupDone: func(pending int64) {
				cleanupPending <- pending
			},
		})
	}()
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		_, _ = fmt.Fprint(writer, "data: first\ndata: second\n")
	}()
	waitStreamScannerSignal(t, synchronizedWriter.firstModelStarted, "first model write did not reach the synchronized writer")
	close(synchronizedWriter.releaseFirstModel)
	waitStreamScannerSignal(t, firstCallbackBlocked, "first model callback did not hold write contention")
	waitStreamScannerSignal(t, secondQueued, "second model event was not queued")
	pingTicks <- time.Time{}
	waitStreamScannerSignal(t, pingTickDone, "priority ping tick was not handled")
	close(releaseFirstCallback)
	waitStreamScannerSignal(t, writeDone, "priority stream input was not scanned")
	waitStreamScannerSignal(t, synchronizedWriter.secondModelWrote, "second model write did not complete")
	_, err := fmt.Fprint(writer, "data: [DONE]\n")
	require.NoError(t, err)
	waitStreamScannerSignal(t, handlerDone, "priority stream did not finish")
	for callbackErr := range callbackErrors {
		require.NoError(t, callbackErr)
	}
	assert.False(t, pingCalled.Load())
	assert.Equal(t, int64(0), <-cleanupPending)

	body := recorder.Body.String()
	firstIndex := strings.Index(body, "data: first")
	secondIndex := strings.Index(body, "data: second")
	require.NotEqual(t, -1, firstIndex)
	require.NotEqual(t, -1, secondIndex)
	assert.NotContains(t, body[firstIndex:secondIndex], ": PING", "heartbeat overtook a ready model event")
}

func TestStreamScannerHandlerCleanupClearsBufferedPendingData(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		panic bool
	}{
		{name: "handler stop"},
		{name: "handler panic", panic: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			body := strings.NewReader("data: first\ndata: second\ndata: third\ndata: [DONE]\n")
			c, resp, info := setupStreamTest(t, body)
			thirdQueued := make(chan struct{})
			releaseFirst := make(chan struct{})
			cleanupPending := make(chan int64, 1)
			handlerDone := make(chan struct{})
			var callbackCount atomic.Int64

			go func() {
				defer close(handlerDone)
				streamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
					callbackCount.Add(1)
					<-releaseFirst
					if testCase.panic {
						panic("handler failed")
					}
					sr.Stop(nil)
				}, streamScannerOptions{
					dataQueued: func(data string) {
						if data == "third" {
							close(thirdQueued)
						}
					},
					cleanupDone: func(pending int64) {
						cleanupPending <- pending
					},
				})
			}()
			waitStreamScannerSignal(t, thirdQueued, "buffered data was not queued")
			close(releaseFirst)
			waitStreamScannerSignal(t, handlerDone, "buffered-data cleanup did not finish")

			assert.Equal(t, int64(1), callbackCount.Load())
			assert.Equal(t, int64(0), <-cleanupPending)
		})
	}
}

func TestStreamScannerHandlerClientCancellationWinsPingError(t *testing.T) {
	configureStreamScannerRecoveryTest(t)

	requestContext, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 28}}
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(requestContext)
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	pingTicks := make(chan time.Time, 1)
	pingEntered := make(chan struct{})
	releasePing := make(chan struct{})
	firstHandled := make(chan struct{})
	firstProcessingDone := make(chan struct{})
	var firstProcessingOnce sync.Once
	terminalHandled := make(chan struct{})
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		streamScannerHandler(c, &http.Response{Body: reader}, info, func(data string, sr *StreamResult) {
			switch data {
			case "first":
				close(firstHandled)
			case "terminal":
				info.MarkStreamTerminalUsage()
				close(terminalHandled)
			}
		}, streamScannerOptions{
			pingTicks: pingTicks,
			dataHandled: func() {
				firstProcessingOnce.Do(func() { close(firstProcessingDone) })
			},
			writePing: func(c *gin.Context) error {
				close(pingEntered)
				<-releasePing
				return fmt.Errorf("ping canceled: %w", c.Request.Context().Err())
			},
		})
	}()

	require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: first\n"); return err }())
	waitStreamScannerSignal(t, firstHandled, "ping-race test did not handle its first event")
	waitStreamScannerSignal(t, firstProcessingDone, "ping-race test did not finish its first event")
	pingTicks <- time.Time{}
	waitStreamScannerSignal(t, pingEntered, "ping callback was not entered")
	cancelRequest()
	close(releasePing)
	waitStreamScannerDetached(t, info)
	require.NoError(t, func() error { _, err := fmt.Fprint(writer, "data: terminal\ndata: [DONE]\n"); return err }())
	waitStreamScannerSignal(t, terminalHandled, "detached terminal event was not handled")
	waitStreamScannerSignal(t, handlerDone, "ping-race stream did not finish")

	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	snapshot := info.GetStreamRecoverySnapshot()
	assert.True(t, snapshot.Detached)
	assert.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestStreamScannerHandlerIneligibleDisconnectStillAborts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	var count atomic.Int64
	firstHandled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
			_ = StringData(c, data)
			if data == "first" {
				close(firstHandled)
			}
		})
		close(done)
	}()

	_, err := fmt.Fprint(pw, "data: first\n")
	require.NoError(t, err)

	select {
	case <-firstHandled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first chunk")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}

	_, err = fmt.Fprint(pw, "data: second\n")
	require.ErrorIs(t, err, io.ErrClosedPipe, "upstream body should be closed after client disconnect")

	assert.Equal(t, int64(1), count.Load(), "no chunk after disconnect should be processed")
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)

	body := recorder.Body.String()
	assert.Contains(t, body, "first")
	assert.NotContains(t, body, "second")
}

// ---------- Ping tests ----------

func TestStreamScannerHandler_PingSentDuringSlowUpstream(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldEnabled := setting.PingIntervalEnabled
	oldSeconds := setting.PingIntervalSeconds
	setting.PingIntervalEnabled = true
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for i := 0; i < 4; i++ {
			fmt.Fprintf(pw, "data: chunk_%d\n", i)
			time.Sleep(400 * time.Millisecond)
		}
		fmt.Fprint(pw, "data: [DONE]\n")
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	var count atomic.Int64
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream to finish")
	}

	assert.Equal(t, int64(4), count.Load())

	body := recorder.Body.String()
	pingCount := strings.Count(body, ": PING")
	assert.GreaterOrEqual(t, pingCount, 1,
		"expected at least 1 ping during slow stream with 1s interval; got %d", pingCount)
}

func TestStreamScannerHandler_PingDisabledByRelayInfo(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldEnabled := setting.PingIntervalEnabled
	oldSeconds := setting.PingIntervalSeconds
	setting.PingIntervalEnabled = true
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(buildSSEBody(5)))}
	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	var count atomic.Int64
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	assert.Equal(t, int64(5), count.Load())

	body := recorder.Body.String()
	pingCount := strings.Count(body, ": PING")
	assert.Equal(t, 0, pingCount, "pings should be disabled when DisablePing=true")
}

// ---------- StreamStatus integration ----------

func TestStreamScannerHandler_StreamStatus_DoneReason(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(10)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Nil(t, info.StreamStatus.EndError)
	assert.True(t, info.StreamStatus.IsNormalEnd())
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_EOFWithoutDone(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, "data: {\"id\":%d}\n", i)
	}
	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.IsNormalEnd())
}

func TestStreamScannerHandler_StreamStatus_HandlerStop(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(100)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= 10 {
			sr.Stop(fmt.Errorf("stop at 10"))
		}
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_HandlerDone(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(20)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= 5 {
			sr.Done()
		}
	})

	assert.Equal(t, int64(5), count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_Timeout(t *testing.T) {
	// Not parallel: modifies global constant.StreamingTimeout
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	pr, pw := io.Pipe()
	go func() {
		fmt.Fprint(pw, "data: {\"id\":1}\n")
		time.Sleep(2 * time.Second)
		pw.Close()
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream timeout")
	}

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonTimeout, info.StreamStatus.EndReason)
	assert.False(t, info.StreamStatus.IsNormalEnd())
}

func TestStreamScannerHandler_StreamStatus_SoftErrors(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(10)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		sr.Error(fmt.Errorf("soft error for chunk"))
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.HasErrors())
	assert.Equal(t, 10, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_MultipleErrorsPerChunk(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(5)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		sr.Error(fmt.Errorf("error A"))
		sr.Error(fmt.Errorf("error B"))
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, 10, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_ErrorThenStop(t *testing.T) {
	t.Parallel()

	// Use a large body without [DONE] to avoid race between scanner's [DONE]
	// and handler's Stop on the sync.Once EndReason.
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "data: {\"id\":%d}\n", i)
	}
	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
		sr.Error(fmt.Errorf("soft error"))
		sr.Stop(fmt.Errorf("fatal"))
	})

	assert.Equal(t, int64(1), count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	assert.Equal(t, 2, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_InitializedIfNil(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(1)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	assert.Nil(t, info.StreamStatus)

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	assert.NotNil(t, info.StreamStatus)
}

func TestStreamScannerHandler_StreamStatus_ReplacesPreInitialized(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(5)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.RecordError("pre-existing error")

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, 0, info.StreamStatus.TotalErrorCount())
}
