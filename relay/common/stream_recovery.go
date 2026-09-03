package common

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/constant"
)

type StreamUsageState string

const (
	StreamUsageStatePending StreamUsageState = "pending"
	StreamUsageStateExact   StreamUsageState = "exact"
	StreamUsageStatePartial StreamUsageState = "partial"
	StreamUsageStateUnknown StreamUsageState = "unknown"
)

type StreamDrainResult string

const (
	StreamDrainResultNone          StreamDrainResult = ""
	StreamDrainResultCompleted     StreamDrainResult = "completed"
	StreamDrainResultCapacity      StreamDrainResult = "capacity"
	StreamDrainResultTimeout       StreamDrainResult = "timeout"
	StreamDrainResultSizeLimit     StreamDrainResult = "size_limit"
	StreamDrainResultUpstreamError StreamDrainResult = "upstream_error"
)

type StreamBillingBasis string

const (
	StreamBillingBasisNone                StreamBillingBasis = ""
	StreamBillingBasisPreconsume          StreamBillingBasis = "preconsume"
	StreamBillingBasisEstimatedPreconsume StreamBillingBasis = "estimated_preconsume"
)

type StreamRecoverySnapshot struct {
	Enabled      bool
	Accepted     bool
	Detached     bool
	UsageState   StreamUsageState
	DrainResult  StreamDrainResult
	BillingBasis StreamBillingBasis
	DrainedBytes int64
}

type StreamRecovery struct {
	mu sync.Mutex

	enabled      bool
	accepted     bool
	detached     bool
	usageState   StreamUsageState
	drainResult  StreamDrainResult
	billingBasis StreamBillingBasis
	drainedBytes int64
	finished     bool
	channelID    int

	upstreamContext    context.Context
	upstreamCancel     context.CancelFunc
	stopParentWatcher  func() bool
	drainTimer         *time.Timer
	limiterRelease     func()
	testTransitionHook func(streamRecoveryTransition)
}

type streamRecoveryTransition uint8

const (
	streamRecoveryTransitionAccept streamRecoveryTransition = iota
	streamRecoveryTransitionParentDone
	streamRecoveryTransitionTerminal
	streamRecoveryTransitionUnknown
	streamRecoveryTransitionTimeout
)

type streamRecoveryLimiter struct {
	mu         sync.Mutex
	total      int
	perChannel map[int]int
}

type streamRecoveryCleanup struct {
	stopParentWatcher func() bool
	drainTimer        *time.Timer
	cancelUpstream    context.CancelFunc
	limiterRelease    func()
}

var streamRecoveryAdmission = streamRecoveryLimiter{
	perChannel: make(map[int]int),
}

func (limiter *streamRecoveryLimiter) tryAcquire(channelID int) func() {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	maxConcurrency := constant.StreamUsageDrainMaxConcurrency
	maxPerChannel := constant.StreamUsageDrainMaxPerChannel
	if maxConcurrency <= 0 || maxPerChannel <= 0 || limiter.total >= maxConcurrency || limiter.perChannel[channelID] >= maxPerChannel {
		return nil
	}

	limiter.total++
	limiter.perChannel[channelID]++
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			limiter.mu.Lock()
			defer limiter.mu.Unlock()

			if limiter.total > 0 {
				limiter.total--
			}
			if limiter.perChannel[channelID] <= 1 {
				delete(limiter.perChannel, channelID)
				return
			}
			limiter.perChannel[channelID]--
		})
	}
}

func (info *RelayInfo) EnableStreamRecovery() {
	if info == nil || !constant.StreamUsageDrainEnabled || info.StreamRecovery != nil {
		return
	}
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	info.StreamRecovery = &StreamRecovery{
		enabled:    true,
		usageState: StreamUsageStatePending,
		channelID:  channelID,
	}
}

func (info *RelayInfo) StartStreamRecoveryAttempt(parent context.Context) context.Context {
	if info == nil || info.StreamRecovery == nil {
		return parent
	}
	if parent == nil {
		parent = context.Background()
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	if !recovery.enabled {
		recovery.mu.Unlock()
		return parent
	}
	if recovery.finished && !recovery.accepted {
		recovery.finished = false
		recovery.upstreamContext = nil
	}
	if recovery.upstreamContext != nil {
		upstream := recovery.upstreamContext
		recovery.mu.Unlock()
		return upstream
	}
	upstream, cancelUpstream := context.WithCancel(context.WithoutCancel(parent))
	recovery.upstreamContext = upstream
	recovery.upstreamCancel = cancelUpstream
	recovery.mu.Unlock()

	stopParentWatcher := context.AfterFunc(parent, recovery.handleParentDone)
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		stopParentWatcher()
		cancelUpstream()
		return upstream
	}
	recovery.stopParentWatcher = stopParentWatcher
	recovery.mu.Unlock()
	return upstream
}

func (info *RelayInfo) MarkStreamAccepted() {
	if info == nil || info.StreamRecovery == nil {
		return
	}
	recovery := info.StreamRecovery
	recovery.mu.Lock()
	hook := recovery.testTransitionHook
	upstreamContext := recovery.upstreamContext
	if !recovery.finished {
		recovery.accepted = true
	}
	recovery.mu.Unlock()
	if upstreamContext != nil && upstreamContext.Err() != nil {
		recovery.tryDetach()
	}
	if hook != nil {
		hook(streamRecoveryTransitionAccept)
	}
}

func (info *RelayInfo) TryDetachStream() bool {
	if info == nil || info.StreamRecovery == nil {
		return false
	}
	return info.StreamRecovery.tryDetach()
}

func (recovery *StreamRecovery) tryDetach() bool {
	recovery.mu.Lock()
	if !recovery.enabled || !recovery.accepted || recovery.finished {
		recovery.mu.Unlock()
		return false
	}
	if recovery.detached {
		recovery.mu.Unlock()
		return true
	}
	release := streamRecoveryAdmission.tryAcquire(recovery.channelID)
	if release == nil {
		recovery.usageState = StreamUsageStateUnknown
		recovery.drainResult = StreamDrainResultCapacity
		hook := recovery.testTransitionHook
		cleanup := recovery.finishLocked()
		recovery.mu.Unlock()
		if hook != nil {
			hook(streamRecoveryTransitionUnknown)
		}
		cleanup.run()
		return false
	}

	timeoutSeconds := constant.StreamUsageDrainTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	recovery.detached = true
	recovery.limiterRelease = release
	recovery.drainTimer = time.AfterFunc(streamRecoveryTimeoutDuration(timeoutSeconds), recovery.handleDrainTimeout)
	recovery.mu.Unlock()
	return true
}

func (info *RelayInfo) IsStreamDetached() bool {
	if info == nil || info.StreamRecovery == nil {
		return false
	}
	info.StreamRecovery.mu.Lock()
	defer info.StreamRecovery.mu.Unlock()
	return info.StreamRecovery.detached
}

func (info *RelayInfo) StreamRecoveryDone() <-chan struct{} {
	if info == nil || info.StreamRecovery == nil {
		return nil
	}
	recovery := info.StreamRecovery
	recovery.mu.Lock()
	defer recovery.mu.Unlock()
	if recovery.upstreamContext == nil {
		return nil
	}
	return recovery.upstreamContext.Done()
}

func (info *RelayInfo) AddDrainedStreamBytes(n int64) bool {
	if info == nil || info.StreamRecovery == nil {
		return false
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	if !recovery.detached || recovery.finished {
		recovery.mu.Unlock()
		return false
	}
	maxBytesMB := constant.StreamUsageDrainMaxBytesMB
	if maxBytesMB <= 0 {
		maxBytesMB = 32
	}
	maxBytes := int64(math.MaxInt64)
	if int64(maxBytesMB) <= math.MaxInt64/(1024*1024) {
		maxBytes = int64(maxBytesMB) * 1024 * 1024
	}
	if n < 0 || n > math.MaxInt64-recovery.drainedBytes || recovery.drainedBytes+n > maxBytes {
		recovery.mu.Unlock()
		info.MarkStreamUsageUnknown(StreamDrainResultSizeLimit)
		return false
	}
	recovery.drainedBytes += n
	recovery.mu.Unlock()
	return true
}

func (info *RelayInfo) MarkStreamAuthoritativeUsage() {
	if info == nil || info.StreamRecovery == nil {
		return
	}
	info.StreamRecovery.mu.Lock()
	defer info.StreamRecovery.mu.Unlock()
	if !info.StreamRecovery.finished {
		info.StreamRecovery.usageState = StreamUsageStatePartial
	}
}

func (info *RelayInfo) MarkStreamTerminalUsage() {
	if info == nil {
		return
	}
	info.StreamTerminalUsageSeen = true
	if info.StreamRecovery == nil {
		return
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	recovery.usageState = StreamUsageStateExact
	recovery.drainResult = StreamDrainResultCompleted
	hook := recovery.testTransitionHook
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	if hook != nil {
		hook(streamRecoveryTransitionTerminal)
	}
	cleanup.run()
}

func (info *RelayInfo) MarkStreamUsageUnknown(result StreamDrainResult) {
	if info == nil || info.StreamRecovery == nil {
		return
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	recovery.usageState = StreamUsageStateUnknown
	recovery.drainResult = result
	hook := recovery.testTransitionHook
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	if hook != nil {
		hook(streamRecoveryTransitionUnknown)
	}
	cleanup.run()
}

func (info *RelayInfo) MarkStreamDrainIncomplete(result StreamDrainResult) {
	if info == nil || info.StreamRecovery == nil {
		return
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	if recovery.usageState != StreamUsageStatePartial {
		recovery.usageState = StreamUsageStateUnknown
	}
	recovery.drainResult = result
	hook := recovery.testTransitionHook
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	if hook != nil {
		hook(streamRecoveryTransitionUnknown)
	}
	cleanup.run()
}

func (info *RelayInfo) FinishStreamRecovery() {
	if info == nil || info.StreamRecovery == nil {
		return
	}
	info.StreamRecovery.finish()
}

func (info *RelayInfo) GetStreamRecoverySnapshot() StreamRecoverySnapshot {
	if info == nil || info.StreamRecovery == nil {
		return StreamRecoverySnapshot{}
	}

	recovery := info.StreamRecovery
	recovery.mu.Lock()
	defer recovery.mu.Unlock()
	return StreamRecoverySnapshot{
		Enabled:      recovery.enabled,
		Accepted:     recovery.accepted,
		Detached:     recovery.detached,
		UsageState:   recovery.usageState,
		DrainResult:  recovery.drainResult,
		BillingBasis: recovery.billingBasis,
		DrainedBytes: recovery.drainedBytes,
	}
}

func (info *RelayInfo) SetStreamBillingBasis(basis StreamBillingBasis) {
	if info == nil || info.StreamRecovery == nil {
		return
	}
	info.StreamRecovery.mu.Lock()
	info.StreamRecovery.billingBasis = basis
	info.StreamRecovery.mu.Unlock()
}

func (recovery *StreamRecovery) handleParentDone() {
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	if recovery.accepted {
		recovery.mu.Unlock()
		recovery.tryDetach()
		return
	}
	hook := recovery.testTransitionHook
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	if hook != nil {
		hook(streamRecoveryTransitionParentDone)
	}
	cleanup.run()
}

func (recovery *StreamRecovery) handleDrainTimeout() {
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	recovery.usageState = StreamUsageStateUnknown
	recovery.drainResult = StreamDrainResultTimeout
	hook := recovery.testTransitionHook
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	if hook != nil {
		hook(streamRecoveryTransitionTimeout)
	}
	cleanup.run()
}

func streamRecoveryTimeoutDuration(timeoutSeconds int) time.Duration {
	seconds := int64(timeoutSeconds)
	maxSeconds := int64(math.MaxInt64) / int64(time.Second)
	if seconds > maxSeconds {
		seconds = maxSeconds
	}
	return time.Duration(seconds) * time.Second
}

func (recovery *StreamRecovery) finish() {
	recovery.mu.Lock()
	if recovery.finished {
		recovery.mu.Unlock()
		return
	}
	cleanup := recovery.finishLocked()
	recovery.mu.Unlock()
	cleanup.run()
}

func (recovery *StreamRecovery) finishLocked() streamRecoveryCleanup {
	recovery.finished = true
	cleanup := streamRecoveryCleanup{
		stopParentWatcher: recovery.stopParentWatcher,
		drainTimer:        recovery.drainTimer,
		cancelUpstream:    recovery.upstreamCancel,
		limiterRelease:    recovery.limiterRelease,
	}
	recovery.stopParentWatcher = nil
	recovery.drainTimer = nil
	recovery.upstreamCancel = nil
	recovery.limiterRelease = nil
	return cleanup
}

func (cleanup streamRecoveryCleanup) run() {
	if cleanup.stopParentWatcher != nil {
		cleanup.stopParentWatcher()
	}
	if cleanup.drainTimer != nil {
		cleanup.drainTimer.Stop()
	}
	if cleanup.cancelUpstream != nil {
		cleanup.cancelUpstream()
	}
	if cleanup.limiterRelease != nil {
		cleanup.limiterRelease()
	}
}
