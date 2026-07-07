package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
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
