package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelaySensitiveWordViolationChargesBeforeChannelSelection(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}))

	previousLogConsumeEnabled := common.LogConsumeEnabled
	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	previousSensitiveEnabled := setting.CheckSensitiveEnabled
	previousPromptSensitiveEnabled := setting.CheckSensitiveOnPromptEnabled
	previousSensitiveWords := append([]string(nil), setting.SensitiveWords...)
	settings := model_setting.GetGrokSettings()
	previousGrokSettings := *settings
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	setting.CheckSensitiveEnabled = true
	setting.CheckSensitiveOnPromptEnabled = true
	setting.SensitiveWords = []string{"test_sensitive"}
	*settings = model_setting.GrokSettings{
		ViolationDeductionEnabled: true,
		ViolationDeductionAmount:  0.05,
	}
	t.Cleanup(func() {
		common.LogConsumeEnabled = previousLogConsumeEnabled
		common.BatchUpdateEnabled = previousBatchUpdateEnabled
		setting.CheckSensitiveEnabled = previousSensitiveEnabled
		setting.CheckSensitiveOnPromptEnabled = previousPromptSensitiveEnabled
		setting.SensitiveWords = previousSensitiveWords
		*settings = previousGrokSettings
	})

	const (
		userID        = 9801
		tokenID       = 9802
		startingQuota = 200_000
	)
	user := &model.User{
		Id:       userID,
		Username: "relay-sensitive-user",
		Password: "unused-password",
		Quota:    startingQuota,
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	token := &model.Token{
		Id:          tokenID,
		UserId:      userID,
		Key:         "relay-sensitive-token",
		Name:        "relay-sensitive-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: startingQuota,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(token).Error)

	body := []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"test_sensitive"}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(constant.ContextKeyUserId), userID)
	c.Set(string(constant.ContextKeyUserQuota), startingQuota)
	c.Set(string(constant.ContextKeyUserGroup), "default")
	c.Set(string(constant.ContextKeyUsingGroup), "default")
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-4.1-mini")
	c.Set(string(constant.ContextKeyTokenId), tokenID)
	c.Set(string(constant.ContextKeyTokenKey), "relay-sensitive-token")
	c.Set(string(constant.ContextKeyTokenUnlimited), false)
	c.Set(string(constant.ContextKeyTokenGroup), "default")
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(string(constant.ContextKeyUserSetting), dto.UserSetting{
		BillingPreference:     "wallet_only",
		AcceptUnsetRatioModel: true,
	})
	c.Set("token_quota", startingQuota)
	c.Set("token_name", "relay-sensitive-token")
	c.Set("username", "relay-sensitive-user")
	c.Set(common.RequestIdKey, "relay-sensitive-request")

	Relay(c, types.RelayFormatOpenAI)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Error types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, string(types.ErrorCodeSensitiveWordsDetected), fmt.Sprint(response.Error.Code))

	feeQuota := int(0.05 * common.QuotaPerUnit)
	require.NoError(t, db.First(user, userID).Error)
	assert.Equal(t, startingQuota-feeQuota, user.Quota)
	assert.Equal(t, feeQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)

	require.NoError(t, db.First(token, tokenID).Error)
	assert.Equal(t, startingQuota-feeQuota, token.RemainQuota)
	assert.Equal(t, feeQuota, token.UsedQuota)

	var channelCount int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&channelCount).Error)
	assert.Zero(t, channelCount)

	var consumeLogs []model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Find(&consumeLogs).Error)
	require.Len(t, consumeLogs, 1)
	assert.Equal(t, 0, consumeLogs[0].ChannelId)
	assert.Equal(t, feeQuota, consumeLogs[0].Quota)
}
