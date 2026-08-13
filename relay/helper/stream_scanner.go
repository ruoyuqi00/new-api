package helper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10
	DefaultMaxScannerBufferSize = 128 << 20
	DefaultPingInterval         = 10 * time.Second
	streamWriteTimeout          = 30 * time.Second
)

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func NewStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	return scanner
}

var errStreamDrainLimit = errors.New("stream drain byte limit reached")

type streamDrainReader struct {
	reader    io.Reader
	info      *relaycommon.RelayInfo
	exhausted bool
}

func (r *streamDrainReader) Read(p []byte) (int, error) {
	if r.exhausted {
		return 0, errStreamDrainLimit
	}
	if len(p) == 0 {
		return 0, nil
	}

	snapshot := r.info.GetStreamRecoverySnapshot()
	if !snapshot.Enabled {
		return r.reader.Read(p)
	}
	limit := int64(math.MaxInt64)
	maxBytesMB := constant.StreamUsageDrainMaxBytesMB
	if maxBytesMB <= 0 {
		maxBytesMB = 32
	}
	if int64(maxBytesMB) <= math.MaxInt64/(1024*1024) {
		limit = int64(maxBytesMB) * 1024 * 1024
	}

	detachedBeforeRead := snapshot.Detached
	remaining := limit
	if detachedBeforeRead {
		remaining -= snapshot.DrainedBytes
		if remaining <= 0 {
			r.exhausted = true
			return 0, errStreamDrainLimit
		}
	}
	// Eligible connected reads are also capped so a detach that occurs while
	// the underlying Read is blocked cannot consume more than a fresh budget.
	if int64(len(p)) > remaining {
		p = p[:int(remaining)]
	}

	n, err := r.reader.Read(p)
	if n <= 0 || (!detachedBeforeRead && !r.info.IsStreamDetached()) {
		return n, err
	}
	if !r.info.AddDrainedStreamBytes(int64(n)) {
		if r.info.GetStreamRecoverySnapshot().DrainResult == relaycommon.StreamDrainResultSizeLimit {
			r.exhausted = true
			return n, errStreamDrainLimit
		}
		return n, err
	}
	if r.info.GetStreamRecoverySnapshot().DrainedBytes < limit {
		return n, err
	}

	// No additional byte is read to distinguish exact exhaustion from an EOF.
	// The request goroutine adjudicates size_limit after buffered events drain.
	r.exhausted = true
	return n, errStreamDrainLimit
}

type streamWriteTryLocker interface {
	TryLock() bool
	Unlock()
}

func tryWriteStreamPing(writeMutex streamWriteTryLocker, pendingData *atomic.Int64, write func() error) (bool, error) {
	if pendingData.Load() > 0 || !writeMutex.TryLock() {
		return false, nil
	}
	defer writeMutex.Unlock()
	if pendingData.Load() > 0 {
		return false, nil
	}
	return true, write()
}

func handleStreamPingError(c *gin.Context, info *relaycommon.RelayInfo, err error) bool {
	if requestContextDone(c) {
		return false
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, err)
	return true
}

func copyCodexSSEHeaders(c *gin.Context, resp *http.Response) {
	if c == nil || c.Writer == nil || resp == nil {
		return
	}
	for _, name := range []string{"X-Reasoning-Included", "X-Codex-Turn-State"} {
		values := resp.Header.Values(name)
		if !service.ShouldCopyUpstreamHeader(c, name, values) {
			continue
		}
		for _, value := range values {
			if value != "" {
				c.Writer.Header().Add(name, value)
			}
		}
	}
}

type streamScannerOptions struct {
	pingTicks    <-chan time.Time
	writePing    func(*gin.Context) error
	pingTickDone func()
	dataQueued   func(string)
	dataHandled  func()
	cleanupDone  func(int64)
}

func ExtendWriteDeadline(c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(streamWriteTimeout))
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {
	streamScannerHandler(c, resp, info, dataHandler, streamScannerOptions{})
}

func streamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult), options streamScannerOptions) {
	if resp == nil || dataHandler == nil {
		return
	}

	info.StreamStatus = relaycommon.NewStreamStatus()

	ctx, cancel := context.WithCancel(context.Background())
	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second

	var (
		stopChan       = make(chan bool, 3)
		detachReady    = make(chan struct{})
		pingErrors     = make(chan error, 1)
		drainReader    = &streamDrainReader{reader: resp.Body, info: info}
		scanner        = NewStreamScanner(drainReader)
		ticker         = time.NewTicker(streamingTimeout)
		pingTicker     *time.Ticker
		writeMutex     sync.Mutex
		pendingData    atomic.Int64 // transient priority hint, not stream state
		drainLimit     atomic.Bool
		scannerReadErr error
		wg             sync.WaitGroup
		cleanupOnce    sync.Once
		stopOnce       sync.Once
	)

	stop := func() {
		stopOnce.Do(func() {
			close(stopChan)
		})
	}

	generalSettings := operation_setting.GetGeneralSetting()
	pingEnabled := (generalSettings.PingIntervalEnabled || options.pingTicks != nil) && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}
	pingTicks := options.pingTicks
	if pingEnabled && pingTicks == nil {
		pingTicker = time.NewTicker(pingInterval)
		pingTicks = pingTicker.C
	}
	writePing := options.writePing
	if writePing == nil {
		writePing = PingData
	}

	logger.LogDebug(c, "relay timeout seconds: %d", common.RelayTimeout)
	logger.LogDebug(c, "relay max idle conns: %d", common.RelayMaxIdleConns)
	logger.LogDebug(c, "relay max idle conns per host: %d", common.RelayMaxIdleConnsPerHost)
	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))

	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			stop()
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			ticker.Stop()
			if pingTicker != nil {
				pingTicker.Stop()
			}
			wg.Wait()
			pendingData.Store(0)
			if options.cleanupDone != nil {
				options.cleanupDone(pendingData.Load())
			}
		})
	}
	// Do not return the gin.Context to Gin's pool until all stream goroutines
	// have stopped using it.
	defer cleanup()

	scanner.Split(bufio.ScanLines)
	copyCodexSSEHeaders(c, resp)
	SetEventStreamHeaders(c)

	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	if pingEnabled && pingTicks != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					stop()
				}
				logger.LogDebug(c, "ping goroutine exited")
				wg.Done()
			}()

			pingTimeout := time.NewTimer(30 * time.Minute)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicks:
					if requestContextDone(c) || info.IsStreamDetached() {
						return
					}
					wrote, err := tryWriteStreamPing(&writeMutex, &pendingData, func() error {
						ExtendWriteDeadline(c)
						return writePing(c)
					})
					if options.pingTickDone != nil {
						options.pingTickDone()
					}
					if !wrote {
						continue
					}
					if err != nil {
						select {
						case pingErrors <- err:
						default:
						}
						return
					}
					logger.LogDebug(c, "ping data sent")
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan string, 10)

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			stop()
			wg.Done()
		}()

		sr := newStreamResult(info.StreamStatus)
		for data := range dataChan {
			sr.reset()
			func() {
				defer pendingData.Add(-1)
				writeMutex.Lock()
				defer writeMutex.Unlock()
				ExtendWriteDeadline(c)
				dataHandler(data, sr)
			}()
			if options.dataHandled != nil {
				options.dataHandled()
			}
			if sr.IsStopped() {
				return
			}
		}
	})

	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			if errors.Is(scanner.Err(), errStreamDrainLimit) {
				drainLimit.Store(true)
			}
			close(dataChan)
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			stop()
			logger.LogDebug(c, "scanner goroutine exited")
			wg.Done()
		}()

		for scanner.Scan() {
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			default:
			}
			select {
			case <-c.Request.Context().Done():
				select {
				case <-detachReady:
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				}
			default:
			}

			ticker.Reset(streamingTimeout)
			data := scanner.Text()
			logger.LogDebug(c, "stream scanner data: %s", data)

			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = data[5:]
			data = strings.TrimSpace(data)
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				info.SetFirstResponseTime()
				info.ReceivedResponseCount++
				CaptureActualResponseModelJSON(info, common.StringToByteSlice(data))

				pendingData.Add(1)
				select {
				case dataChan <- data:
				case <-ctx.Done():
					pendingData.Add(-1)
					return
				case <-stopChan:
					pendingData.Add(-1)
					return
				}
				if options.dataQueued != nil {
					options.dataQueued(data)
				}
			} else {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				logger.LogDebug(c, "received [DONE], stopping scanner")
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF && !errors.Is(err, errStreamDrainLimit) {
				logger.LogError(c, "scanner error: "+err.Error())
				scannerReadErr = err
			}
		}
	})

	clientDone := c.Request.Context().Done()
	streamingTimeoutDone := ticker.C
	var recoveryDone <-chan struct{}
waitForStream:
	for {
		if clientDone != nil {
			select {
			case <-clientDone:
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				if !info.TryDetachStream() {
					break waitForStream
				}
				clientDone = nil
				streamingTimeoutDone = nil
				recoveryDone = info.StreamRecoveryDone()
				close(detachReady)
				continue
			default:
			}
		}
		select {
		case <-streamingTimeoutDone:
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
			break waitForStream
		case <-stopChan:
			break waitForStream
		case <-clientDone:
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
			if !info.TryDetachStream() {
				break waitForStream
			}
			clientDone = nil
			streamingTimeoutDone = nil
			recoveryDone = info.StreamRecoveryDone()
			close(detachReady)
		case pingErr := <-pingErrors:
			if handleStreamPingError(c, info, pingErr) {
				logger.LogError(c, "ping data error: "+pingErr.Error())
				break waitForStream
			}
		case <-recoveryDone:
			break waitForStream
		}
	}

	cleanup()
	snapshot := info.GetStreamRecoverySnapshot()
	terminalCompleted := snapshot.Enabled && snapshot.UsageState == relaycommon.StreamUsageStateExact && snapshot.DrainResult == relaycommon.StreamDrainResultCompleted
	info.StreamStatus.FinalizeAfterWorkers(c.Request.Context().Err(), scannerReadErr, terminalCompleted)
	if drainLimit.Load() {
		snapshot := info.GetStreamRecoverySnapshot()
		if snapshot.UsageState != relaycommon.StreamUsageStateExact || snapshot.DrainResult != relaycommon.StreamDrainResultCompleted {
			info.MarkStreamDrainIncomplete(relaycommon.StreamDrainResultSizeLimit)
		}
	}
	if info.IsStreamDetached() {
		snapshot := info.GetStreamRecoverySnapshot()
		if snapshot.UsageState != relaycommon.StreamUsageStateExact || snapshot.DrainResult != relaycommon.StreamDrainResultCompleted {
			info.MarkStreamDrainIncomplete(relaycommon.StreamDrainResultUpstreamError)
		}
	}
	if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
	} else {
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
	}
}
