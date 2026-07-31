package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViolationFeeReason(t *testing.T) {
	tests := []struct {
		name       string
		apiErr     *types.NewAPIError
		wantReason violationFeeReason
		wantCharge bool
	}{
		{
			name: "local sensitive word",
			apiErr: types.NewErrorWithStatusCode(
				errors.New("sensitive words detected"),
				types.ErrorCodeSensitiveWordsDetected,
				http.StatusBadRequest,
			),
			wantReason: violationFeeReasonLocalSensitiveWord,
			wantCharge: true,
		},
		{
			name: "normalized Grok CSAM",
			apiErr: types.NewErrorWithStatusCode(
				errors.New("upstream safety rejection"),
				types.ErrorCodeViolationFeeGrokCSAM,
				http.StatusBadRequest,
			),
			wantReason: violationFeeReasonGrokCSAM,
			wantCharge: true,
		},
		{
			name: "unrelated upstream error",
			apiErr: types.NewErrorWithStatusCode(
				errors.New("upstream unavailable"),
				types.ErrorCodeBadResponseStatusCode,
				http.StatusBadGateway,
			),
			wantCharge: false,
		},
		{
			name:       "nil error",
			apiErr:     nil,
			wantCharge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, charge := classifyViolationFee(tt.apiErr)
			assert.Equal(t, tt.wantCharge, charge)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestCalcViolationFeeQuota(t *testing.T) {
	require.Greater(t, common.QuotaPerUnit, 0.0)

	assert.Equal(t, int(common.QuotaPerUnit/10), calcViolationFeeQuota(0.05, 2))
	assert.Equal(t, 0, calcViolationFeeQuota(0, 2))
	assert.Equal(t, 0, calcViolationFeeQuota(0.05, 0))
}

func TestChargeLocalViolationFeeChargesWalletOnceWithoutChannelUsage(t *testing.T) {
	truncate(t)

	settings := model_setting.GetGrokSettings()
	previousSettings := *settings
	*settings = model_setting.GrokSettings{
		ViolationDeductionEnabled: true,
		ViolationDeductionAmount:  0.05,
	}
	t.Cleanup(func() { *settings = previousSettings })

	const (
		userID        = 901
		tokenID       = 902
		channelID     = 903
		startingQuota = 200_000
	)
	seedUser(t, userID, startingQuota)
	seedToken(t, tokenID, userID, "violation-token", startingQuota)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", 17).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("token_name", "test_token")
	c.Set("username", "test_user")

	relayInfo := &relaycommon.RelayInfo{
		TokenId:         tokenID,
		TokenKey:        "violation-token",
		UserId:          userID,
		UsingGroup:      "premium",
		StartTime:       time.Now(),
		OriginModelName: "claude-test",
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 2},
		},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channelID},
	}
	apiErr := types.NewErrorWithStatusCode(
		errors.New("sensitive words detected"),
		types.ErrorCodeSensitiveWordsDetected,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)

	require.True(t, ChargeLocalViolationFee(c, relayInfo, apiErr))
	assert.False(t, ChargeViolationFeeIfNeeded(c, relayInfo, apiErr), "local violations must not be charged again by the upstream failure path")

	feeQuota := calcViolationFeeQuota(0.05, 2)
	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	assert.Equal(t, startingQuota-feeQuota, user.Quota)
	assert.Equal(t, feeQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)

	var token model.Token
	require.NoError(t, model.DB.First(&token, tokenID).Error)
	assert.Equal(t, startingQuota-feeQuota, token.RemainQuota)
	assert.Equal(t, feeQuota, token.UsedQuota)

	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, channelID).Error)
	assert.Equal(t, int64(17), channel.UsedQuota)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, feeQuota, logs[0].Quota)
	assert.Equal(t, 0, logs[0].ChannelId)
	assert.NotContains(t, logs[0].Other, "sensitive words detected")

	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
	assert.Equal(t, true, other["violation_fee"])
	assert.Equal(t, string(types.ErrorCodeSensitiveWordsDetected), other["violation_fee_code"])
	assert.Equal(t, string(violationFeeReasonLocalSensitiveWord), other["violation_fee_reason"])
	assert.NotContains(t, other, "violation_fee_marker")
}

func TestChargeLocalViolationFeeInsufficientQuotaKeepsBlockWithoutConsumeLog(t *testing.T) {
	truncate(t)

	settings := model_setting.GetGrokSettings()
	previousSettings := *settings
	*settings = model_setting.GrokSettings{
		ViolationDeductionEnabled: true,
		ViolationDeductionAmount:  0.05,
	}
	t.Cleanup(func() { *settings = previousSettings })

	feeQuota := calcViolationFeeQuota(0.05, 1)
	const (
		userID  = 911
		tokenID = 912
	)
	seedUser(t, userID, feeQuota-1)
	seedToken(t, tokenID, userID, "insufficient-token", feeQuota*2)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		TokenId:         tokenID,
		TokenKey:        "insufficient-token",
		UserId:          userID,
		UsingGroup:      "default",
		StartTime:       time.Now(),
		OriginModelName: "claude-test",
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	apiErr := types.NewErrorWithStatusCode(
		errors.New("sensitive words detected"),
		types.ErrorCodeSensitiveWordsDetected,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)

	assert.False(t, ChargeLocalViolationFee(c, relayInfo, apiErr))

	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	assert.Equal(t, feeQuota-1, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)

	var token model.Token
	require.NoError(t, model.DB.First(&token, tokenID).Error)
	assert.Equal(t, feeQuota*2, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Count(&logCount).Error)
	assert.Zero(t, logCount)
}

func TestChargeLocalViolationFeeHonorsSubscriptionOnlyPreference(t *testing.T) {
	truncate(t)

	settings := model_setting.GetGrokSettings()
	previousSettings := *settings
	*settings = model_setting.GrokSettings{
		ViolationDeductionEnabled: true,
		ViolationDeductionAmount:  0.05,
	}
	t.Cleanup(func() { *settings = previousSettings })

	const (
		userID            = 921
		tokenID           = 922
		subscriptionID    = 923
		planID            = 924
		walletQuota       = 100_000
		tokenQuota        = 100_000
		subscriptionTotal = int64(200_000)
		subscriptionUsed  = int64(10_000)
	)
	seedUser(t, userID, walletQuota)
	seedToken(t, tokenID, userID, "subscription-token", tokenQuota)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:               planID,
		Title:            "Test plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      subscriptionTotal,
		QuotaResetPeriod: model.SubscriptionResetNever,
	}).Error)
	seedSubscription(t, subscriptionID, userID, subscriptionTotal, subscriptionUsed)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", subscriptionID).Update("plan_id", planID).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		TokenId:         tokenID,
		TokenKey:        "subscription-token",
		UserId:          userID,
		UsingGroup:      "default",
		StartTime:       time.Now(),
		OriginModelName: "claude-test",
		RequestId:       "local-sensitive-subscription",
		ForcePreConsume: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_only",
		},
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	apiErr := types.NewErrorWithStatusCode(
		errors.New("sensitive words detected"),
		types.ErrorCodeSensitiveWordsDetected,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)

	require.True(t, ChargeLocalViolationFee(c, relayInfo, apiErr))

	feeQuota := calcViolationFeeQuota(0.05, 1)
	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	assert.Equal(t, walletQuota, user.Quota)
	assert.Equal(t, feeQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)

	var subscription model.UserSubscription
	require.NoError(t, model.DB.First(&subscription, subscriptionID).Error)
	assert.Equal(t, subscriptionUsed+int64(feeQuota), subscription.AmountUsed)
	assert.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)

	var token model.Token
	require.NoError(t, model.DB.First(&token, tokenID).Error)
	assert.Equal(t, tokenQuota-feeQuota, token.RemainQuota)
	assert.Equal(t, feeQuota, token.UsedQuota)

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
}

func TestChargeViolationFeeIfNeededPreservesGrokFallbackMetadata(t *testing.T) {
	truncate(t)

	settings := model_setting.GetGrokSettings()
	previousSettings := *settings
	*settings = model_setting.GrokSettings{
		ViolationDeductionEnabled: true,
		ViolationDeductionAmount:  0.05,
	}
	t.Cleanup(func() { *settings = previousSettings })

	const (
		userID        = 931
		tokenID       = 932
		channelID     = 933
		startingQuota = 100_000
	)
	seedUser(t, userID, startingQuota)
	seedToken(t, tokenID, userID, "grok-violation-token", startingQuota)
	seedChannel(t, channelID)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		TokenId:         tokenID,
		TokenKey:        "grok-violation-token",
		UserId:          userID,
		UsingGroup:      "default",
		StartTime:       time.Now(),
		OriginModelName: "grok-test",
		UserQuota:       startingQuota,
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channelID},
	}
	apiErr := types.NewErrorWithStatusCode(
		errors.New(CSAMViolationMarker),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	require.True(t, ChargeViolationFeeIfNeeded(c, relayInfo, apiErr))

	feeQuota := calcViolationFeeQuota(0.05, 1)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, channelID).Error)
	assert.Equal(t, int64(feeQuota), channel.UsedQuota)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).First(&log).Error)
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, string(types.ErrorCodeViolationFeeGrokCSAM), other["violation_fee_code"])
	assert.Equal(t, string(violationFeeReasonGrokCSAM), other["violation_fee_reason"])
	assert.Equal(t, CSAMViolationMarker, other["violation_fee_marker"])
}
