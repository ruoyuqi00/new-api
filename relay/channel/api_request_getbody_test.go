package channel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	basecommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestApplyUpstreamBodyMetadataSetsReplayableMetadata(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"hello"}`)
	body, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })

	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", body)
	require.NoError(t, err)
	assert.Nil(t, req.GetBody)
	assert.Zero(t, req.ContentLength)

	ApplyUpstreamBodyMetadata(req, body)

	assert.EqualValues(t, len(payload), req.ContentLength)
	require.NotNil(t, req.GetBody)
	sent, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, sent)

	for range 2 {
		replayBody, err := req.GetBody()
		require.NoError(t, err)
		replay, err := io.ReadAll(replayBody)
		require.NoError(t, err)
		require.NoError(t, replayBody.Close())
		assert.Equal(t, payload, replay)
	}
}

func TestApplyUpstreamBodyMetadataLeavesNativeReplayAlone(t *testing.T) {
	body := strings.NewReader("native body")
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", body)
	require.NoError(t, err)
	require.NotNil(t, req.GetBody)

	ApplyUpstreamBodyMetadata(req, body)

	assert.EqualValues(t, len("native body"), req.ContentLength)
	replayBody, err := req.GetBody()
	require.NoError(t, err)
	replay, err := io.ReadAll(replayBody)
	require.NoError(t, err)
	require.NoError(t, replayBody.Close())
	assert.Equal(t, "native body", string(replay))
}

func TestApplyUpstreamBodyMetadataHidesRawStorageCloser(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"raw storage"}`)
	storage, err := basecommon.CreateBodyStorage(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", storage)
	require.NoError(t, err)

	ApplyUpstreamBodyMetadata(req, storage)
	require.NoError(t, req.Body.Close())

	replayBody, err := req.GetBody()
	require.NoError(t, err, "the transport must not take ownership of replay storage")
	replay, err := io.ReadAll(replayBody)
	require.NoError(t, err)
	require.NoError(t, replayBody.Close())
	assert.Equal(t, payload, replay)
}

func TestApplyUpstreamBodyMetadataKeepsExistingGetBody(t *testing.T) {
	payload := []byte(`{"model":"test-model"}`)
	body, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", body)
	require.NoError(t, err)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("existing replay")), nil
	}

	ApplyUpstreamBodyMetadata(req, body)

	replayBody, err := req.GetBody()
	require.NoError(t, err)
	replay, err := io.ReadAll(replayBody)
	require.NoError(t, err)
	require.NoError(t, replayBody.Close())
	assert.Equal(t, "existing replay", string(replay))
}

type replayTaskAdaptor struct {
	TaskAdaptor
	baseURL     string
	capturedReq *http.Request
}

func (a *replayTaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/video/generations", nil
}

func (a *replayTaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	a.capturedReq = req
	return nil
}

func TestDoTaskApiRequestKeepsNativeReplayableGetBody(t *testing.T) {
	service.InitHttpClient()
	payload := []byte(`{"model":"test-model","prompt":"hello"}`)
	type bodyReadResult struct {
		body []byte
		err  error
	}
	received := make(chan bodyReadResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		received <- bodyReadResult{body: body, err: err}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(payload))
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	adaptor := &replayTaskAdaptor{baseURL: server.URL}

	resp, err := DoTaskApiRequest(adaptor, c, info, bytes.NewReader(payload))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	upstreamResult := <-received
	require.NoError(t, upstreamResult.err)
	assert.Equal(t, payload, upstreamResult.body)
	assert.True(t, info.TaskRelayInfo.RequestWritten)

	require.NotNil(t, adaptor.capturedReq)
	require.NotNil(t, adaptor.capturedReq.GetBody)
	for range 2 {
		replayBody, err := adaptor.capturedReq.GetBody()
		require.NoError(t, err)
		replay, err := io.ReadAll(replayBody)
		require.NoError(t, err)
		require.NoError(t, replayBody.Close())
		assert.Equal(t, payload, replay)
	}
}

func TestDoRequestMarksWrittenPostWithoutResponseHeadersAmbiguous(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		require.NoError(t, err)
		hijacker, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hijacker.Hijack()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}))
	t.Cleanup(server.Close)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"hello"}`))
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"input":"hello"}`))
	require.NoError(t, err)

	resp, err := doRequest(c, req, info)

	require.Error(t, err)
	require.Nil(t, resp)
	assert.True(t, info.UpstreamRequestWasWritten())
	assert.False(t, info.UpstreamResponseHeadersWereReceived())
	assert.True(t, info.HasAmbiguousUpstreamSubmission())
}

func TestDoRequestPreservesTaskRequestWriteTrace(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{"prompt":"hello"}`))
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	adaptor := &replayTaskAdaptor{baseURL: server.URL}

	resp, err := DoTaskApiRequest(adaptor, c, info, strings.NewReader(`{"prompt":"hello"}`))

	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assert.True(t, info.TaskRelayInfo.RequestWritten)
	assert.True(t, info.UpstreamRequestWasWritten())
	assert.True(t, info.UpstreamResponseHeadersWereReceived())
	assert.False(t, info.HasAmbiguousUpstreamSubmission())
}

func TestUpstreamAttemptStateCannotLeakIntoNextRetry(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	first := info.BeginUpstreamRequestAttempt()
	second := info.BeginUpstreamRequestAttempt()

	first.MarkRequestWritten()
	first.MarkAmbiguousIfPotentiallySent()

	assert.False(t, second.RequestWasWritten())
	assert.False(t, second.IsAmbiguous())
	assert.False(t, info.HasAmbiguousUpstreamSubmission())
}

func TestUpstreamAttemptRequiresPotentialNetworkSubmission(t *testing.T) {
	attempt := (&relaycommon.RelayInfo{}).BeginUpstreamRequestAttempt()

	attempt.MarkAmbiguousIfPotentiallySent()
	assert.False(t, attempt.IsAmbiguous())

	attempt.MarkConnectionObtained()
	attempt.MarkAmbiguousIfPotentiallySent()
	assert.True(t, attempt.IsAmbiguous())
}

type h2ReplayResult struct {
	err           error
	attemptBodies [][]byte
}

func acceptH2ReplayConnection(listener net.Listener) (net.Conn, *http2.Framer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	preface := make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(conn, preface); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("read client preface: %w", err)
	}
	if !bytes.Equal(preface, []byte(http2.ClientPreface)) {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("unexpected client preface")
	}

	framer := http2.NewFramer(conn, conn)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if err := framer.WriteSettings(); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, framer, nil
}

func readH2ReplayRequest(framer *http2.Framer) (uint32, []byte, error) {
	var streamID uint32
	var body []byte
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return 0, nil, fmt.Errorf("read frame: %w", err)
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() {
				if err := framer.WriteSettingsAck(); err != nil {
					return 0, nil, err
				}
			}
		case *http2.MetaHeadersFrame:
			streamID = frame.Header().StreamID
			if frame.StreamEnded() {
				return streamID, body, nil
			}
		case *http2.DataFrame:
			if streamID == 0 {
				streamID = frame.Header().StreamID
			}
			if frame.Header().StreamID != streamID {
				continue
			}
			body = append(body, frame.Data()...)
			if frame.StreamEnded() {
				return streamID, body, nil
			}
		}
	}
}

func writeH2ReplayResponse(framer *http2.Framer, streamID uint32) error {
	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
		return err
	}
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: headerBlock.Bytes(),
		EndHeaders:    true,
	}); err != nil {
		return err
	}
	return framer.WriteData(streamID, true, []byte(`{}`))
}

func runH2GoAwayReplayServer(listener net.Listener) (<-chan h2ReplayResult, func()) {
	resultCh := make(chan h2ReplayResult, 1)
	done := make(chan struct{})
	go func() {
		result := h2ReplayResult{}
		connections := make([]net.Conn, 0, 2)
		defer func() {
			for _, conn := range connections {
				_ = conn.Close()
			}
		}()
		for attempt := 0; attempt < 2; attempt++ {
			conn, framer, err := acceptH2ReplayConnection(listener)
			if err != nil {
				result.err = err
				resultCh <- result
				return
			}
			connections = append(connections, conn)
			streamID, body, err := readH2ReplayRequest(framer)
			if err != nil {
				result.err = err
				resultCh <- result
				return
			}
			result.attemptBodies = append(result.attemptBodies, body)

			if attempt == 0 {
				err = framer.WriteGoAway(0, http2.ErrCodeNo, nil)
				if err != nil {
					result.err = err
					resultCh <- result
					return
				}
				continue
			}

			result.err = writeH2ReplayResponse(framer, streamID)
			resultCh <- result
			if result.err == nil {
				<-done
			}
			return
		}
	}()
	return resultCh, func() { close(done) }
}

func newH2ReplayClient(listener net.Listener) (*http.Client, *http2.Transport) {
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.Config) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, listener.Addr().String())
		},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}, transport
}

func TestUpstreamRequestBodyReplaysAfterHTTP2GoAway(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"retry after go away"}`)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	resultCh, stopServer := runH2GoAwayReplayServer(listener)
	t.Cleanup(stopServer)
	client, transport := newH2ReplayClient(listener)
	t.Cleanup(transport.CloseIdleConnections)

	body, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })
	req, err := http.NewRequest(http.MethodPost, "http://upstream.test/v1/responses", body)
	require.NoError(t, err)
	ApplyUpstreamBodyMetadata(req, body)

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Len(t, result.attemptBodies, 2)
		assert.Equal(t, payload, result.attemptBodies[0])
		assert.Equal(t, payload, result.attemptBodies[1])
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for HTTP/2 replay server")
	}
}
