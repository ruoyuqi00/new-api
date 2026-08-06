package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type affiliateOptionResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AffiliateCreditRebateEnabled     bool `json:"affiliate_credit_rebate_enabled"`
		AffiliateCreditRebateBasisPoints int  `json:"affiliate_credit_rebate_basis_points"`
	} `json:"data"`
}

func setupAffiliateOptionTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalEnabled := common.AffiliateCreditRebateEnabled
	originalBasisPoints := common.AffiliateCreditRebateBasisPoints
	originalRedisEnabled := common.RedisEnabled
	paymentSetting := operation_setting.GetPaymentSetting()
	originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
	originalComplianceVersion := paymentSetting.ComplianceTermsVersion

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "affiliate-option-admin",
		AffCode:  "affiliate-option-admin-code",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	model.InitOptionMap()

	common.RedisEnabled = false
	common.AffiliateCreditRebateEnabled = false
	common.AffiliateCreditRebateBasisPoints = 0
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.AffiliateCreditRebateEnabled = originalEnabled
		common.AffiliateCreditRebateBasisPoints = originalBasisPoints
		common.RedisEnabled = originalRedisEnabled
		paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = originalComplianceVersion
	})
}

func performAffiliateOptionRequest(t *testing.T, body string) affiliateOptionResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	UpdateOption(ctx)

	var response affiliateOptionResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestAffiliateCreditRebateRejectsInvalidBasisPoints(t *testing.T) {
	for _, basisPoints := range []int{-1, 10_001} {
		t.Run(fmt.Sprintf("basis-points-%d", basisPoints), func(t *testing.T) {
			setupAffiliateOptionTest(t)
			response := performAffiliateOptionRequest(
				t,
				fmt.Sprintf(`{"key":"AffiliateCreditRebateBasisPoints","value":%d}`, basisPoints),
			)
			assert.False(t, response.Success)
			assert.Zero(t, common.AffiliateCreditRebateBasisPoints)
		})
	}
}

func TestAffiliateCreditRebateRequiresComplianceBeforeEnabling(t *testing.T) {
	setupAffiliateOptionTest(t)
	common.AffiliateCreditRebateBasisPoints = 500

	response := performAffiliateOptionRequest(
		t,
		`{"key":"AffiliateCreditRebateEnabled","value":true}`,
	)

	assert.False(t, response.Success)
	assert.False(t, common.AffiliateCreditRebateEnabled)
}

func TestAffiliateCreditRebateAcceptsValidConfiguration(t *testing.T) {
	setupAffiliateOptionTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	ratioResponse := performAffiliateOptionRequest(
		t,
		`{"key":"AffiliateCreditRebateBasisPoints","value":525}`,
	)
	require.True(t, ratioResponse.Success)
	assert.Equal(t, 525, common.AffiliateCreditRebateBasisPoints)

	enabledResponse := performAffiliateOptionRequest(
		t,
		`{"key":"AffiliateCreditRebateEnabled","value":true}`,
	)
	require.True(t, enabledResponse.Success)
	assert.True(t, common.AffiliateCreditRebateEnabled)
}

func TestAffiliateCreditRebateWalletInfoExposesCurrentConfiguration(t *testing.T) {
	setupAffiliateOptionTest(t)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 525

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	GetTopUpInfo(ctx)

	var response affiliateOptionResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.True(t, response.Data.AffiliateCreditRebateEnabled)
	assert.Equal(t, 525, response.Data.AffiliateCreditRebateBasisPoints)
}
