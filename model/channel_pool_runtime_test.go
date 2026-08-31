package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetChannelPoolRuntimeForTest(t *testing.T) {
	t.Helper()
	oldRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	channelPoolMemoryMu.Lock()
	channelPoolMemoryInflight = map[int]int{}
	channelPoolMemoryCooldown = map[string]time.Time{}
	channelPoolMemoryMu.Unlock()
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		channelPoolMemoryMu.Lock()
		channelPoolMemoryInflight = map[int]int{}
		channelPoolMemoryCooldown = map[string]time.Time{}
		channelPoolMemoryMu.Unlock()
	})
}

func resetChannelPoolSelectionCacheForTest(t *testing.T) {
	t.Helper()
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	oldChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	group2model2channels = map[string]map[string][]int{}
	channelsIDM = map[int]*Channel{}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channel2advancedCustomConfig = oldChannel2AdvancedCustomConfig
		channelSyncLock.Unlock()
	})
}

func TestChannelPoolMemoryLeaseHonorsLimitAndRelease(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)

	channel := &Channel{
		Id:            7,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
	lease, acquired, err := AcquireChannelPoolLease(channel)
	if err != nil {
		t.Fatalf("AcquireChannelPoolLease returned error: %v", err)
	}
	if !acquired || lease == nil {
		t.Fatalf("first lease should be acquired")
	}

	secondLease, acquired, err := AcquireChannelPoolLease(channel)
	if err != nil {
		t.Fatalf("second AcquireChannelPoolLease returned error: %v", err)
	}
	if acquired || secondLease != nil {
		t.Fatalf("second lease should be rejected while first lease is active")
	}

	lease.Release()
	thirdLease, acquired, err := AcquireChannelPoolLease(channel)
	if err != nil {
		t.Fatalf("third AcquireChannelPoolLease returned error: %v", err)
	}
	if !acquired || thirdLease == nil {
		t.Fatalf("third lease should be acquired after release")
	}
	thirdLease.Release()
}

func TestChannelPoolCandidateStatusReportsReasons(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)

	nilStatus := ChannelPoolCandidateStatusFor(nil, "gpt-plus", "gpt-5")
	if nilStatus.Available {
		t.Fatalf("nil channel should not be available")
	}
	if nilStatus.Reason != ChannelPoolCandidateReasonNoChannel {
		t.Fatalf("nil channel reason mismatch: %s", nilStatus.Reason)
	}

	unlimited := &Channel{Id: 10}
	unlimitedStatus := ChannelPoolCandidateStatusFor(unlimited, "gpt-plus", "gpt-5")
	if !unlimitedStatus.Available || unlimitedStatus.Reason != ChannelPoolCandidateReasonAvailable {
		t.Fatalf("unlimited channel should be available, got %+v", unlimitedStatus)
	}
	if unlimitedStatus.HasHardLimit || unlimitedStatus.Limit != 0 || unlimitedStatus.Inflight != 0 {
		t.Fatalf("unlimited channel should not report hard limit state, got %+v", unlimitedStatus)
	}

	limited := &Channel{
		Id:            11,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
	initialStatus := ChannelPoolCandidateStatusFor(limited, "gpt-plus", "gpt-5")
	if !initialStatus.Available || initialStatus.Reason != ChannelPoolCandidateReasonAvailable {
		t.Fatalf("limited channel should start available, got %+v", initialStatus)
	}
	if !initialStatus.HasHardLimit || initialStatus.Limit != 1 || initialStatus.Inflight != 0 {
		t.Fatalf("limited channel should report empty hard limit state, got %+v", initialStatus)
	}

	lease, acquired, err := AcquireChannelPoolLease(limited)
	if err != nil {
		t.Fatalf("AcquireChannelPoolLease returned error: %v", err)
	}
	if !acquired || lease == nil {
		t.Fatalf("limited channel lease should be acquired")
	}
	fullStatus := ChannelPoolCandidateStatusFor(limited, "gpt-plus", "gpt-5")
	if fullStatus.Available || fullStatus.Reason != ChannelPoolCandidateReasonFull {
		t.Fatalf("limited channel should report full, got %+v", fullStatus)
	}
	if fullStatus.Limit != 1 || fullStatus.Inflight != 1 || !fullStatus.HasHardLimit {
		t.Fatalf("limited channel full status mismatch: %+v", fullStatus)
	}
	lease.Release()
}

func TestChannelPoolSelectionSnapshotCountsCandidateReasons(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)
	resetChannelPoolSelectionCacheForTest(t)

	skipped := &Channel{
		Id:            21,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
	full := &Channel{
		Id:            22,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
	cooling := &Channel{
		Id:            23,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
	available := &Channel{
		Id:            25,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}

	lease, acquired, err := AcquireChannelPoolLease(full)
	if err != nil {
		t.Fatalf("AcquireChannelPoolLease returned error: %v", err)
	}
	if !acquired || lease == nil {
		t.Fatalf("full channel lease should be acquired")
	}
	t.Cleanup(lease.Release)
	CooldownChannelPool(cooling.Id, "gpt-plus", "gpt-5", 30, "test")

	channelSyncLock.Lock()
	group2model2channels["gpt-plus"] = map[string][]int{
		"gpt-5": {skipped.Id, full.Id, cooling.Id, 24, available.Id},
	}
	channelsIDM[skipped.Id] = skipped
	channelsIDM[full.Id] = full
	channelsIDM[cooling.Id] = cooling
	channelsIDM[available.Id] = available
	channelSyncLock.Unlock()

	snapshot := ChannelPoolSelectionSnapshotFor("gpt-plus", "gpt-5", "", ChannelSelectionOptions{
		SkipChannelIDs: map[int]struct{}{skipped.Id: {}},
	})

	if !snapshot.CacheEnabled {
		t.Fatalf("snapshot should report memory cache enabled")
	}
	if snapshot.CandidateCount != 3 {
		t.Fatalf("candidate count mismatch: %+v", snapshot)
	}
	if snapshot.AvailableCount != 1 || snapshot.FullCount != 1 || snapshot.CooldownCount != 1 {
		t.Fatalf("reason counts mismatch: %+v", snapshot)
	}
	if snapshot.MissingChannelCount != 1 || snapshot.SelectionSkippedCount != 1 || snapshot.PathSkippedCount != 0 {
		t.Fatalf("filter counts mismatch: %+v", snapshot)
	}
}

func TestChannelPoolCooldownMakesCandidateUnavailable(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)

	channel := &Channel{Id: 8}
	if !ChannelPoolCandidateAvailable(channel, "gpt-plus", "gpt-5") {
		t.Fatalf("candidate should be available before cooldown")
	}
	CooldownChannelPool(channel.Id, "gpt-plus", "gpt-5", 30, "test")
	if ChannelPoolCandidateAvailable(channel, "gpt-plus", "gpt-5") {
		t.Fatalf("candidate should be unavailable during cooldown")
	}
	status := ChannelPoolCandidateStatusFor(channel, "gpt-plus", "gpt-5")
	if status.Available || status.Reason != ChannelPoolCandidateReasonCooldown || !status.CoolingDown {
		t.Fatalf("candidate should report cooldown, got %+v", status)
	}
	if !ChannelPoolCandidateAvailable(channel, "gpt-team", "gpt-5") {
		t.Fatalf("cooldown should be scoped by group and model")
	}
}

func TestChannelPoolMalformedSettingsDoNotMutateChannel(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)

	channel := &Channel{
		Id:            9,
		OtherSettings: `{"channel_pool_concurrency_limit":`,
	}
	if got := ChannelPoolConcurrencyLimit(channel); got != 0 {
		t.Fatalf("malformed settings should fall back to disabled limit, got %d", got)
	}
	if channel.OtherSettings != `{"channel_pool_concurrency_limit":` {
		t.Fatalf("malformed settings should not be rewritten")
	}
}

func TestGetChannelWithOptionsSelectsHighestPriorityAfterExcludingFailedChannel(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = oldMemoryCacheEnabled })
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))

	priorities := []int64{100, 50, 0}
	channels := make([]Channel, len(priorities))
	group := "retry-exclusion-group"
	modelName := "retry-exclusion-model"
	for index, priority := range priorities {
		channels[index] = Channel{
			Name: "retry-channel", Key: "key", Status: common.ChannelStatusEnabled,
			Group: group, Models: modelName, Priority: &priority,
		}
		require.NoError(t, DB.Create(&channels[index]).Error)
		require.NoError(t, DB.Create(&Ability{
			Group: group, Model: modelName, ChannelId: channels[index].Id,
			Enabled: true, Priority: &priority, Weight: 100,
		}).Error)
	}
	t.Cleanup(func() {
		for _, channel := range channels {
			_ = DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
			_ = DB.Delete(&Channel{}, "id = ?", channel.Id).Error
		}
	})

	selected, err := GetChannelWithOptions(group, modelName, 0, "", ChannelSelectionOptions{
		SkipChannelIDs: map[int]struct{}{channels[0].Id: {}},
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, channels[1].Id, selected.Id)
}

func TestGetRandomSatisfiedChannelWithOptionsFindsLegacyImageAliasForCanonicalModel(t *testing.T) {
	resetChannelPoolRuntimeForTest(t)
	resetChannelPoolSelectionCacheForTest(t)
	channel := &Channel{Id: 701, OtherSettings: `{"image_dimension_support":"any"}`}
	channelSyncLock.Lock()
	group2model2channels["image-group"] = map[string][]int{"gpt-image-2-2k": {channel.Id}}
	channelsIDM[channel.Id] = channel
	channelSyncLock.Unlock()

	selected, err := GetRandomSatisfiedChannelWithOptions("image-group", "gpt-image-2", 0, "", ChannelSelectionOptions{
		ImageModelName: "gpt-image-2",
		ImageRequirements: &ImageSelectionRequirements{
			CanonicalModel: "gpt-image-2",
			Size:           "2k",
			Tier:           "2k",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, channel.Id, selected.Id)
}
