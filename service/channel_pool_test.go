package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newChannelPoolTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "gpt-plus")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5")
	return ctx
}

func disableRedisForChannelPoolTest(t *testing.T) {
	t.Helper()
	oldRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
	})
}

func limitedChannelPoolTestChannel(id int) *model.Channel {
	return &model.Channel{
		Id:            id,
		OtherSettings: `{"channel_pool_concurrency_limit":1}`,
	}
}

func TestEffectiveChannelPoolCooldownSeconds(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		want       int
	}{
		{name: "zero uses default", configured: 0, want: 10},
		{name: "positive overrides default", configured: 25, want: 25},
		{name: "negative disables cooldown", configured: -1, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, effectiveChannelPoolCooldownSeconds(test.configured))
		})
	}
}

func TestTryAcquireChannelPoolLeaseReusesSameChannelLease(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	ctx := newChannelPoolTestContext(t)
	channel := limitedChannelPoolTestChannel(3101)

	acquired, err := TryAcquireChannelPoolLease(ctx, channel)
	require.NoError(t, err)
	require.True(t, acquired)

	status := model.ChannelPoolCandidateStatusFor(channel, "gpt-plus", "gpt-5")
	require.Equal(t, model.ChannelPoolCandidateReasonFull, status.Reason)
	require.Equal(t, 1, status.Inflight)

	acquired, err = TryAcquireChannelPoolLease(ctx, channel)
	require.NoError(t, err)
	require.True(t, acquired)

	status = model.ChannelPoolCandidateStatusFor(channel, "gpt-plus", "gpt-5")
	require.Equal(t, model.ChannelPoolCandidateReasonFull, status.Reason)
	require.Equal(t, 1, status.Inflight)

	ReleaseCurrentChannelPoolLease(ctx)
	status = model.ChannelPoolCandidateStatusFor(channel, "gpt-plus", "gpt-5")
	require.True(t, status.Available)
	require.Equal(t, 0, status.Inflight)

	ReleaseCurrentChannelPoolLease(ctx)
	status = model.ChannelPoolCandidateStatusFor(channel, "gpt-plus", "gpt-5")
	require.True(t, status.Available)
	require.Equal(t, 0, status.Inflight)
}

func TestTryAcquireChannelPoolLeaseReplacesDifferentChannelLease(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	ctx := newChannelPoolTestContext(t)
	first := limitedChannelPoolTestChannel(3102)
	second := limitedChannelPoolTestChannel(3103)

	acquired, err := TryAcquireChannelPoolLease(ctx, first)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, model.ChannelPoolCandidateReasonFull, model.ChannelPoolCandidateStatusFor(first, "gpt-plus", "gpt-5").Reason)

	acquired, err = TryAcquireChannelPoolLease(ctx, second)
	require.NoError(t, err)
	require.True(t, acquired)

	firstStatus := model.ChannelPoolCandidateStatusFor(first, "gpt-plus", "gpt-5")
	secondStatus := model.ChannelPoolCandidateStatusFor(second, "gpt-plus", "gpt-5")
	require.True(t, firstStatus.Available)
	require.Equal(t, 0, firstStatus.Inflight)
	require.Equal(t, model.ChannelPoolCandidateReasonFull, secondStatus.Reason)
	require.Equal(t, 1, secondStatus.Inflight)

	ReleaseCurrentChannelPoolLease(ctx)
	secondStatus = model.ChannelPoolCandidateStatusFor(second, "gpt-plus", "gpt-5")
	require.True(t, secondStatus.Available)
	require.Equal(t, 0, secondStatus.Inflight)
}

func TestIsChannelPoolTemporarilyUnavailableWithContextReportsFull(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	ctx := newChannelPoolTestContext(t)
	channel := limitedChannelPoolTestChannel(3104)

	lease, acquired, err := model.AcquireChannelPoolLease(channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, lease)
	t.Cleanup(lease.Release)

	require.True(t, IsChannelPoolTemporarilyUnavailableWithContext(ctx, channel, "gpt-plus", "gpt-5"))
	require.True(t, IsChannelPoolTemporarilyUnavailable(channel, "gpt-plus", "gpt-5"))

	lease.Release()
	require.False(t, IsChannelPoolTemporarilyUnavailableWithContext(ctx, channel, "gpt-plus", "gpt-5"))
}

func TestMaybeCooldownSelectedChannelPoolDefaultsTextRequestsToTenSeconds(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	ctx := newChannelPoolTestContext(t)
	const channelID = 3110
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		ChannelPoolCooldownSeconds: 0,
	})
	ctx.Set("relay_mode", relayconstant.RelayModeResponses)

	upstreamErr := types.NewErrorWithStatusCode(
		errors.New("upstream unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	MaybeCooldownSelectedChannelPool(ctx, upstreamErr)

	status := model.ChannelPoolCandidateStatusFor(&model.Channel{Id: channelID}, "gpt-plus", "gpt-5")
	require.True(t, status.CoolingDown)
}

func TestMaybeCooldownSelectedChannelPoolAllowsExplicitDisable(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	ctx := newChannelPoolTestContext(t)
	const channelID = 3111
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		ChannelPoolCooldownSeconds: -1,
	})
	ctx.Set("relay_mode", relayconstant.RelayModeResponses)

	upstreamErr := types.NewErrorWithStatusCode(
		errors.New("upstream unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	MaybeCooldownSelectedChannelPool(ctx, upstreamErr)

	status := model.ChannelPoolCandidateStatusFor(&model.Channel{Id: channelID}, "gpt-plus", "gpt-5")
	require.False(t, status.CoolingDown)
}

func TestMaybeCooldownSelectedChannelPoolSkipsMediaRequests(t *testing.T) {
	disableRedisForChannelPoolTest(t)
	upstreamErr := types.NewErrorWithStatusCode(
		errors.New("upstream unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	tests := []struct {
		name      string
		relayMode int
	}{
		{name: "image generation", relayMode: relayconstant.RelayModeImagesGenerations},
		{name: "image edit", relayMode: relayconstant.RelayModeImagesEdits},
		{name: "midjourney imagine", relayMode: relayconstant.RelayModeMidjourneyImagine},
		{name: "midjourney task fetch", relayMode: relayconstant.RelayModeMidjourneyTaskFetch},
		{name: "midjourney video", relayMode: relayconstant.RelayModeMidjourneyVideo},
		{name: "midjourney edit", relayMode: relayconstant.RelayModeMidjourneyEdits},
		{name: "video submit", relayMode: relayconstant.RelayModeVideoSubmit},
		{name: "video fetch", relayMode: relayconstant.RelayModeVideoFetchByID},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := newChannelPoolTestContext(t)
			channelID := 3120 + i
			common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
			common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
				ChannelPoolCooldownSeconds: 10,
			})
			ctx.Set("relay_mode", test.relayMode)

			MaybeCooldownSelectedChannelPool(ctx, upstreamErr)

			status := model.ChannelPoolCandidateStatusFor(&model.Channel{Id: channelID}, "gpt-plus", "gpt-5")
			require.False(t, status.CoolingDown)
		})
	}
}
