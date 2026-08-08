package common

import (
	"context"
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

	info.StreamRecovery.mu.Lock()
	require.True(t, info.StreamRecovery.drainTimer.Stop())
	info.StreamRecovery.drainTimer = time.AfterFunc(20*time.Millisecond, info.StreamRecovery.handleDrainTimeout)
	info.StreamRecovery.mu.Unlock()

	guard, cancelGuard := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelGuard()
	select {
	case <-upstream.Done():
	case <-guard.Done():
		require.FailNow(t, "stream recovery timeout did not cancel the upstream context")
	}

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
