package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryParamExcludesFailedChannelWithoutSkippingNextPriority(t *testing.T) {
	retry := 0
	param := RetryParam{Retry: &retry}

	param.PrepareForRetry(42, false)
	param.IncreaseRetry()

	assert.Zero(t, param.GetRetry())
	assert.Contains(t, param.ChannelSelectionOptions().SkipChannelIDs, 42)
}

func TestRetryParamKeepsPooledChannelEligibleForAnotherAccount(t *testing.T) {
	retry := 2
	param := RetryParam{Retry: &retry}

	param.PrepareForRetry(42, true)
	param.IncreaseRetry()

	assert.Equal(t, 2, param.GetRetry())
	assert.NotContains(t, param.ChannelSelectionOptions().SkipChannelIDs, 42)
	assert.Equal(t, 42, param.TakeProviderRetryChannelID())
	assert.Zero(t, param.TakeProviderRetryChannelID())
}

func TestRetryParamClearsProviderChannelWhenChannelMustFailOver(t *testing.T) {
	retry := 0
	param := RetryParam{Retry: &retry}

	param.PrepareForRetry(42, true)
	param.PrepareForRetry(42, false)

	assert.Zero(t, param.TakeProviderRetryChannelID())
	assert.Contains(t, param.ChannelSelectionOptions().SkipChannelIDs, 42)
}
