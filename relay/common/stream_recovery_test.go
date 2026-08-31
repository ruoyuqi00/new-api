package common

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureStreamRecoveryTest(t *testing.T) {
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
	constant.StreamUsageDrainTimeoutSeconds = 300
	constant.StreamUsageDrainMaxBytesMB = 1
}

func newStreamRecoveryTestInfo(channelID int) *RelayInfo {
	return &RelayInfo{ChannelMeta: &ChannelMeta{ChannelId: channelID}}
}

func streamRecoveryLimiterCounts() (int, map[int]int) {
	streamRecoveryAdmission.mu.Lock()
	defer streamRecoveryAdmission.mu.Unlock()

	perChannel := make(map[int]int, len(streamRecoveryAdmission.perChannel))
	for channelID, count := range streamRecoveryAdmission.perChannel {
		perChannel[channelID] = count
	}
	return streamRecoveryAdmission.total, perChannel
}

func waitForStreamRecoverySignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	guard, cancelGuard := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelGuard()
	select {
	case <-signal:
	case <-guard.Done():
		require.FailNow(t, message)
	}
}

func TestStreamRecoveryPreAcceptCancellationCancelsUpstream(t *testing.T) {
	configureStreamRecoveryTest(t)

	constant.StreamUsageDrainEnabled = false
	disabled := newStreamRecoveryTestInfo(11)
	disabledParent := context.Background()
	disabled.EnableStreamRecovery()
	assert.Nil(t, disabled.StreamRecovery)
	assert.Equal(t, disabledParent, disabled.StartStreamRecoveryAttempt(disabledParent))
	constant.StreamUsageDrainEnabled = true

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	parent, cancelParent := context.WithCancel(context.Background())
	upstream := info.StartStreamRecoveryAttempt(parent)

	cancelParent()
	guard, cancelGuard := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelGuard()
	select {
	case <-upstream.Done():
	case <-guard.Done():
		require.FailNow(t, "upstream context was not canceled before stream acceptance")
	}

	snapshot := info.GetStreamRecoverySnapshot()
	assert.True(t, snapshot.Enabled)
	assert.False(t, snapshot.Accepted)
	assert.False(t, snapshot.Detached)
	assert.Equal(t, StreamUsageStatePending, snapshot.UsageState)
}

func TestStreamRecoveryPostAcceptCancellationKeepsUpstreamAlive(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	parent, cancelParent := context.WithCancel(context.Background())
	upstream := info.StartStreamRecoveryAttempt(parent)
	info.MarkStreamAccepted()

	cancelParent()
	info.StreamRecovery.handleParentDone()
	select {
	case <-upstream.Done():
		require.FailNow(t, "accepted stream upstream context was canceled with its parent")
	default:
	}

	info.SetStreamBillingBasis(StreamBillingBasisPreconsume)
	info.MarkStreamAuthoritativeUsage()
	info.MarkStreamTerminalUsage()
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultCompleted, snapshot.DrainResult)
	assert.Equal(t, StreamBillingBasisPreconsume, snapshot.BillingBasis)
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
}

func TestStreamRecoveryPostAcceptCancellationAutomaticallyDetaches(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(12)
	info.EnableStreamRecovery()
	parent, cancelParent := context.WithCancel(context.Background())
	upstream := info.StartStreamRecoveryAttempt(parent)
	info.MarkStreamAccepted()

	cancelParent()
	info.StreamRecovery.handleParentDone()

	snapshot := info.GetStreamRecoverySnapshot()
	assert.True(t, snapshot.Accepted)
	assert.True(t, snapshot.Detached)
	select {
	case <-upstream.Done():
		require.FailNow(t, "automatically detached stream canceled the accepted upstream")
	default:
	}
	info.MarkStreamUsageUnknown(StreamDrainResultUpstreamError)
}

func TestStreamRecoveryDoneTracksCurrentUpstreamAttempt(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	assert.Nil(t, info.StreamRecoveryDone())
	info.EnableStreamRecovery()
	assert.Nil(t, info.StreamRecoveryDone())

	upstream := info.StartStreamRecoveryAttempt(context.Background())
	require.NotNil(t, info.StreamRecoveryDone())
	info.FinishStreamRecovery()
	waitForStreamRecoverySignal(t, info.StreamRecoveryDone(), "stream recovery completion did not cancel the upstream attempt")
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
}

func TestStreamRecoveryDoneChangesOnPreAcceptRetry(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	firstParent, cancelFirstParent := context.WithCancel(context.Background())
	info.StartStreamRecoveryAttempt(firstParent)
	firstDone := info.StreamRecoveryDone()
	cancelFirstParent()
	waitForStreamRecoverySignal(t, firstDone, "first upstream attempt was not canceled")

	second := info.StartStreamRecoveryAttempt(context.Background())
	secondDone := info.StreamRecoveryDone()
	require.NotNil(t, secondDone)
	assert.NotEqual(t, firstDone, secondDone)
	assert.Equal(t, second.Done(), secondDone)
	select {
	case <-secondDone:
		require.FailNow(t, "pre-accept retry exposed the completed prior attempt")
	default:
	}
	info.FinishStreamRecovery()
}

func TestStreamRecoveryAdmissionIsNonBlockingAndPerChannel(t *testing.T) {
	configureStreamRecoveryTest(t)

	first := newStreamRecoveryTestInfo(11)
	first.EnableStreamRecovery()
	firstUpstream := first.StartStreamRecoveryAttempt(context.Background())
	first.MarkStreamAccepted()
	require.True(t, first.TryDetachStream())

	blocked := newStreamRecoveryTestInfo(11)
	blocked.EnableStreamRecovery()
	blockedUpstream := blocked.StartStreamRecoveryAttempt(context.Background())
	blocked.MarkStreamAccepted()
	result := make(chan bool, 1)
	go func() {
		result <- blocked.TryDetachStream()
	}()
	guard, cancelGuard := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelGuard()
	select {
	case detached := <-result:
		assert.False(t, detached)
	case <-guard.Done():
		require.FailNow(t, "stream recovery admission queued behind the per-channel limit")
	}
	assert.ErrorIs(t, blockedUpstream.Err(), context.Canceled)
	assert.Equal(t, StreamDrainResultCapacity, blocked.GetStreamRecoverySnapshot().DrainResult)

	otherChannel := newStreamRecoveryTestInfo(12)
	otherChannel.EnableStreamRecovery()
	otherUpstream := otherChannel.StartStreamRecoveryAttempt(context.Background())
	otherChannel.MarkStreamAccepted()
	require.True(t, otherChannel.TryDetachStream())

	total, perChannel := streamRecoveryLimiterCounts()
	assert.Equal(t, 2, total)
	assert.Equal(t, map[int]int{11: 1, 12: 1}, perChannel)

	first.FinishStreamRecovery()
	otherChannel.FinishStreamRecovery()
	total, perChannel = streamRecoveryLimiterCounts()
	assert.Zero(t, total)
	assert.Empty(t, perChannel)
	assert.ErrorIs(t, firstUpstream.Err(), context.Canceled)
	assert.ErrorIs(t, otherUpstream.Err(), context.Canceled)
}

func TestStreamRecoveryReleaseIsIdempotent(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	require.True(t, info.TryDetachStream())
	assert.True(t, info.IsStreamDetached())

	info.FinishStreamRecovery()
	info.FinishStreamRecovery()
	total, perChannel := streamRecoveryLimiterCounts()
	assert.Zero(t, total)
	assert.Empty(t, perChannel)
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
}

func TestStreamRecoveryTimeoutMarksUsageUnknown(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	require.True(t, info.TryDetachStream())

	cleanupDone := make(chan struct{})
	info.StreamRecovery.mu.Lock()
	require.True(t, info.StreamRecovery.drainTimer.Stop())
	release := info.StreamRecovery.limiterRelease
	info.StreamRecovery.limiterRelease = func() {
		release()
		close(cleanupDone)
	}
	info.StreamRecovery.drainTimer = time.AfterFunc(20*time.Millisecond, info.StreamRecovery.handleDrainTimeout)
	info.StreamRecovery.mu.Unlock()

	waitForStreamRecoverySignal(t, cleanupDone, "stream recovery timeout did not complete cleanup")
	assert.ErrorIs(t, upstream.Err(), context.Canceled)

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultTimeout, snapshot.DrainResult)
	total, perChannel := streamRecoveryLimiterCounts()
	assert.Zero(t, total)
	assert.Empty(t, perChannel)
}

func TestStreamRecoverySizeLimitMarksUsageUnknown(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	require.True(t, info.TryDetachStream())

	const oneMegabyte = int64(1024 * 1024)
	require.True(t, info.AddDrainedStreamBytes(oneMegabyte))
	assert.False(t, info.AddDrainedStreamBytes(1))
	assert.ErrorIs(t, upstream.Err(), context.Canceled)

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultSizeLimit, snapshot.DrainResult)
	assert.Equal(t, oneMegabyte, snapshot.DrainedBytes)
	total, perChannel := streamRecoveryLimiterCounts()
	assert.Zero(t, total)
	assert.Empty(t, perChannel)
}

func TestStreamRecoveryAcceptanceWinsParentCancellation(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	t.Cleanup(info.FinishStreamRecovery)

	acceptanceLocked := make(chan struct{})
	releaseAcceptance := make(chan struct{})
	info.StreamRecovery.testTransitionHook = func(transition streamRecoveryTransition) {
		if transition != streamRecoveryTransitionAccept {
			return
		}
		close(acceptanceLocked)
		<-releaseAcceptance
	}
	acceptanceDone := make(chan struct{})
	go func() {
		info.MarkStreamAccepted()
		close(acceptanceDone)
	}()
	waitForStreamRecoverySignal(t, acceptanceLocked, "acceptance did not enter its critical section")

	parentDone := make(chan struct{})
	go func() {
		info.StreamRecovery.handleParentDone()
		close(parentDone)
	}()
	waitForStreamRecoverySignal(t, parentDone, "parent cancellation did not complete")
	close(releaseAcceptance)
	waitForStreamRecoverySignal(t, acceptanceDone, "acceptance did not complete")

	assert.True(t, info.GetStreamRecoverySnapshot().Accepted)
	assert.NoError(t, upstream.Err())
}

func TestStreamRecoveryParentCancellationWinsAcceptance(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())

	parentLocked := make(chan struct{})
	releaseParent := make(chan struct{})
	info.StreamRecovery.testTransitionHook = func(transition streamRecoveryTransition) {
		if transition != streamRecoveryTransitionParentDone {
			return
		}
		close(parentLocked)
		<-releaseParent
	}
	parentDone := make(chan struct{})
	go func() {
		info.StreamRecovery.handleParentDone()
		close(parentDone)
	}()
	waitForStreamRecoverySignal(t, parentLocked, "parent cancellation did not enter its critical section")

	acceptanceDone := make(chan struct{})
	go func() {
		info.MarkStreamAccepted()
		close(acceptanceDone)
	}()
	waitForStreamRecoverySignal(t, acceptanceDone, "acceptance did not complete")
	close(releaseParent)
	waitForStreamRecoverySignal(t, parentDone, "parent cancellation did not complete")

	assert.False(t, info.GetStreamRecoverySnapshot().Accepted)
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
}

func TestStreamRecoveryTerminalUsageWinsTimeout(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	info.MarkStreamAuthoritativeUsage()

	terminalLocked := make(chan struct{})
	releaseTerminal := make(chan struct{})
	info.StreamRecovery.testTransitionHook = func(transition streamRecoveryTransition) {
		if transition != streamRecoveryTransitionTerminal {
			return
		}
		close(terminalLocked)
		<-releaseTerminal
	}
	terminalDone := make(chan struct{})
	go func() {
		info.MarkStreamTerminalUsage()
		close(terminalDone)
	}()
	waitForStreamRecoverySignal(t, terminalLocked, "terminal usage did not enter its critical section")

	timeoutDone := make(chan struct{})
	go func() {
		info.StreamRecovery.handleDrainTimeout()
		close(timeoutDone)
	}()
	waitForStreamRecoverySignal(t, timeoutDone, "timeout did not complete")
	close(releaseTerminal)
	waitForStreamRecoverySignal(t, terminalDone, "terminal usage did not complete")

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestStreamRecoveryTimeoutWinsTerminalUsage(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	info.MarkStreamAuthoritativeUsage()

	timeoutLocked := make(chan struct{})
	releaseTimeout := make(chan struct{})
	info.StreamRecovery.testTransitionHook = func(transition streamRecoveryTransition) {
		if transition != streamRecoveryTransitionTimeout {
			return
		}
		close(timeoutLocked)
		<-releaseTimeout
	}
	timeoutDone := make(chan struct{})
	go func() {
		info.StreamRecovery.handleDrainTimeout()
		close(timeoutDone)
	}()
	waitForStreamRecoverySignal(t, timeoutLocked, "timeout did not enter its critical section")

	terminalDone := make(chan struct{})
	go func() {
		info.MarkStreamTerminalUsage()
		close(terminalDone)
	}()
	waitForStreamRecoverySignal(t, terminalDone, "terminal usage did not complete")
	close(releaseTimeout)
	waitForStreamRecoverySignal(t, timeoutDone, "timeout did not complete")

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateUnknown, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultTimeout, snapshot.DrainResult)
}

func TestStreamRecoveryExcessiveTimeoutDoesNotExpireImmediately(t *testing.T) {
	configureStreamRecoveryTest(t)
	constant.StreamUsageDrainTimeoutSeconds = int(^uint(0) >> 1)

	expectedDuration := time.Duration(math.MaxInt64/int64(time.Second)) * time.Second
	assert.Equal(t, expectedDuration, streamRecoveryTimeoutDuration(constant.StreamUsageDrainTimeoutSeconds))

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	require.True(t, info.TryDetachStream())
	t.Cleanup(info.FinishStreamRecovery)
	assert.NoError(t, upstream.Err())
}

func TestStreamRecoveryAuthoritativeUsageMarksPartialWithoutFinishing(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	t.Cleanup(info.FinishStreamRecovery)

	info.MarkStreamAuthoritativeUsage()
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStatePartial, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultNone, snapshot.DrainResult)
	assert.NoError(t, upstream.Err())
}

func TestStreamRecoveryIncompleteDrainPreservesAuthoritativePartialUsage(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	info.MarkStreamAuthoritativeUsage()

	info.MarkStreamDrainIncomplete(StreamDrainResultUpstreamError)

	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStatePartial, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultUpstreamError, snapshot.DrainResult)
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
}

func TestStreamRecoveryTerminalUsageFromPendingMarksExactAndFinishes(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	upstream := info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	require.True(t, info.TryDetachStream())

	info.MarkStreamTerminalUsage()
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultCompleted, snapshot.DrainResult)
	assert.ErrorIs(t, upstream.Err(), context.Canceled)
	total, perChannel := streamRecoveryLimiterCounts()
	assert.Zero(t, total)
	assert.Empty(t, perChannel)
}

func TestStreamRecoveryAuthoritativeThenTerminalProgressesPartialToExact(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()

	info.MarkStreamAuthoritativeUsage()
	assert.Equal(t, StreamUsageStatePartial, info.GetStreamRecoverySnapshot().UsageState)

	info.MarkStreamTerminalUsage()
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultCompleted, snapshot.DrainResult)
}

func TestStreamRecoveryAuthoritativeUsageCannotDowngradeExactTerminalState(t *testing.T) {
	configureStreamRecoveryTest(t)

	info := newStreamRecoveryTestInfo(11)
	info.EnableStreamRecovery()
	info.StartStreamRecoveryAttempt(context.Background())
	info.MarkStreamAccepted()
	info.MarkStreamTerminalUsage()

	info.MarkStreamAuthoritativeUsage()
	snapshot := info.GetStreamRecoverySnapshot()
	assert.Equal(t, StreamUsageStateExact, snapshot.UsageState)
	assert.Equal(t, StreamDrainResultCompleted, snapshot.DrainResult)
}
