package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayInputModerationBlocksBeforeChannelSelectionAndChargesRequest(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}))

	restoreRelayInputModerationGlobals(t)
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	common.MemoryCacheEnabled = false
	common.RetryTimes = 0
	constant.CountToken = false
	setting.CheckSensitiveEnabled = false
	inputModerationEnabled = func() bool { return true }
	checkInputModeration = func(_ context.Context, input string) (service.InputModerationResult, error) {
		assert.Contains(t, input, "controlled audit phrase")
		return service.InputModerationResult{
			Flagged:    true,
			Model:      "omni-moderation-2024-09-26",
			Categories: []string{"illicit"},
		}, nil
	}

	const (
		userID        = 9901
		tokenID       = 9902
		startingQuota = 20_000_000
	)
	seedRelayInputModerationUserAndToken(t, db, userID, tokenID, startingQuota)
	c, recorder := newRelayInputModerationContext(t, userID, tokenID, startingQuota, false)

	Relay(c, types.RelayFormatOpenAI)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Error types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, string(types.ErrorCodeViolationFeeInputModeration), fmt.Sprint(response.Error.Code))

	var user model.User
	require.NoError(t, db.First(&user, userID).Error)
	chargedQuota := startingQuota - user.Quota
	require.Greater(t, chargedQuota, 0)
	assert.Equal(t, chargedQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)

	var token model.Token
	require.NoError(t, db.First(&token, tokenID).Error)
	assert.Equal(t, startingQuota-chargedQuota, token.RemainQuota)
	assert.Equal(t, chargedQuota, token.UsedQuota)

	var channelCount int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&channelCount).Error)
	assert.Zero(t, channelCount)

	var log model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).First(&log).Error)
	assert.Equal(t, chargedQuota, log.Quota)
	assert.Zero(t, log.ChannelId)
	assert.NotContains(t, log.Other, "controlled audit phrase")
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, string(types.ErrorCodeViolationFeeInputModeration), other["violation_fee_code"])
	assert.Equal(t, float64(chargedQuota), other["requested_quota"])
	assert.Equal(t, true, other["charge_succeeded"])
}

func TestRelayInputModerationFailureFailsOpenToChannelSelection(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}))

	restoreRelayInputModerationGlobals(t)
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	common.MemoryCacheEnabled = false
	common.RetryTimes = 0
	constant.CountToken = false
	setting.CheckSensitiveEnabled = false
	inputModerationEnabled = func() bool { return true }
	checkInputModeration = func(_ context.Context, input string) (service.InputModerationResult, error) {
		assert.Contains(t, input, "controlled audit phrase")
		return service.InputModerationResult{}, errors.New("moderation unavailable")
	}
	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable","type":"upstream_error","code":"upstream_error"}}`))
	}))
	t.Cleanup(upstream.Close)
	service.InitHttpClient()

	startingQuota := common.GetTrustQuota() + 100_000
	const (
		userID  = 9911
		tokenID = 9912
	)
	seedRelayInputModerationUserAndToken(t, db, userID, tokenID, startingQuota)
	c, recorder := newRelayInputModerationContext(t, userID, tokenID, startingQuota, true)
	common.SetContextKey(c, constant.ContextKeyChannelId, 9913)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelName, "moderation-fail-open-upstream")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-upstream-key")
	c.Set("auto_ban", false)

	Relay(c, types.RelayFormatOpenAI)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	select {
	case <-upstreamCalled:
	default:
		t.Fatal("normal upstream relay was not attempted after moderation failure")
	}
	var response struct {
		Error types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.NotEqual(t, string(types.ErrorCodeViolationFeeInputModeration), fmt.Sprint(response.Error.Code))

	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Count(&logCount).Error)
	assert.Zero(t, logCount)
}

func restoreRelayInputModerationGlobals(t *testing.T) {
	t.Helper()
	previousLogConsumeEnabled := common.LogConsumeEnabled
	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRetryTimes := common.RetryTimes
	previousCountToken := constant.CountToken
	previousSensitiveEnabled := setting.CheckSensitiveEnabled
	previousModerationEnabled := inputModerationEnabled
	previousCheckModeration := checkInputModeration
	t.Cleanup(func() {
		common.LogConsumeEnabled = previousLogConsumeEnabled
		common.BatchUpdateEnabled = previousBatchUpdateEnabled
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RetryTimes = previousRetryTimes
		constant.CountToken = previousCountToken
		setting.CheckSensitiveEnabled = previousSensitiveEnabled
		inputModerationEnabled = previousModerationEnabled
		checkInputModeration = previousCheckModeration
	})
}

func seedRelayInputModerationUserAndToken(t *testing.T, db *gorm.DB, userID int, tokenID int, quota int) {
	t.Helper()
	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("moderation-user-%d", userID),
		Password: "unused-password",
		Quota:    quota,
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	token := &model.Token{
		Id:          tokenID,
		UserId:      userID,
		Key:         fmt.Sprintf("moderation-token-%d", tokenID),
		Name:        "moderation-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: quota,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(token).Error)
}

func newRelayInputModerationContext(t *testing.T, userID int, tokenID int, quota int, tokenUnlimited bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	body := []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"controlled audit phrase"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(constant.ContextKeyUserId), userID)
	c.Set(string(constant.ContextKeyUserQuota), quota)
	c.Set(string(constant.ContextKeyUserGroup), "default")
	c.Set(string(constant.ContextKeyUsingGroup), "default")
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-4.1-mini")
	c.Set(string(constant.ContextKeyTokenId), tokenID)
	c.Set(string(constant.ContextKeyTokenKey), fmt.Sprintf("moderation-token-%d", tokenID))
	c.Set(string(constant.ContextKeyTokenUnlimited), tokenUnlimited)
	c.Set(string(constant.ContextKeyTokenGroup), "default")
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(string(constant.ContextKeyUserSetting), dto.UserSetting{
		BillingPreference:     "wallet_only",
		AcceptUnsetRatioModel: true,
	})
	c.Set("token_quota", quota)
	c.Set("token_name", "moderation-token")
	c.Set("username", fmt.Sprintf("moderation-user-%d", userID))
	c.Set(common.RequestIdKey, fmt.Sprintf("moderation-request-%d", userID))
	return c, recorder
}
