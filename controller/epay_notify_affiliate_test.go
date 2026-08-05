package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEpayNotifyReturnsFailWhenRechargeTransactionRollsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalEnabled := common.AffiliateCreditRebateEnabled
	originalBasisPoints := common.AffiliateCreditRebateBasisPoints
	originalQuotaPerUnit := common.QuotaPerUnit
	originalPayAddress := operation_setting.PayAddress
	originalEpayId := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	paymentSetting := operation_setting.GetPaymentSetting()
	originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
	originalComplianceVersion := paymentSetting.ComplianceTermsVersion

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}))

	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 1_000
	common.QuotaPerUnit = 1_000
	operation_setting.PayAddress = "https://epay.example"
	operation_setting.EpayId = "test-partner"
	operation_setting.EpayKey = "test-secret"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.AffiliateCreditRebateEnabled = originalEnabled
		common.AffiliateCreditRebateBasisPoints = originalBasisPoints
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayId
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = originalComplianceVersion
	})

	inviter := model.User{Username: "epay-inviter", AffCode: "epay-inviter-code", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&inviter).Error)
	invitee := model.User{Username: "epay-invitee", AffCode: "epay-invitee-code", InviterId: inviter.Id, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&invitee).Error)
	topUp := model.TopUp{
		UserId:          invitee.Id,
		Amount:          2,
		Money:           2,
		TradeNo:         "epay-rollback-order",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(&topUp).Error)

	params := epay.GenerateParams(map[string]string{
		"pid":          operation_setting.EpayId,
		"type":         "alipay",
		"trade_no":     "provider-order",
		"out_trade_no": topUp.TradeNo,
		"name":         "quota",
		"money":        "2.00",
		"trade_status": epay.StatusTradeSuccess,
	}, operation_setting.EpayKey)
	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/epay/notify", strings.NewReader(form.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	EpayNotify(ctx)

	assert.Equal(t, "fail", recorder.Body.String())
	var storedTopUp model.TopUp
	require.NoError(t, db.First(&storedTopUp, topUp.Id).Error)
	assert.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var storedInvitee model.User
	require.NoError(t, db.First(&storedInvitee, invitee.Id).Error)
	assert.Zero(t, storedInvitee.Quota)
	var storedInviter model.User
	require.NoError(t, db.First(&storedInviter, inviter.Id).Error)
	assert.Zero(t, storedInviter.AffQuota)
}
