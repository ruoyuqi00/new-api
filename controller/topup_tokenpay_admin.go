package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetTokenPayStatus(c *gin.Context) {
	settings := operation_setting.GetTokenPayPaymentSetting()
	common.ApiSuccess(c, gin.H{
		"enabled": settings.Enabled, "base_url": settings.BaseURL,
		"api_token_configured": strings.TrimSpace(settings.APIToken) != "",
		"packages":             append([]operation_setting.TokenPayTopUpPackage{}, settings.Packages...),
		"enabled_networks":     append([]string{}, settings.EnabledNetworks...),
		"allow_private_url":    settings.AllowPrivateURL,
	})
}

func SaveTokenPaySettings(c *gin.Context) {
	var candidate operation_setting.TokenPayPaymentSetting
	if common.DecodeJson(c.Request.Body, &candidate) != nil {
		common.ApiErrorMsg(c, "Invalid TokenPay settings")
		return
	}
	current := operation_setting.GetTokenPayPaymentSetting()
	if strings.TrimSpace(candidate.APIToken) == "" {
		candidate.APIToken = current.APIToken
	}
	if err := candidate.Normalize(); err != nil {
		common.ApiError(c, err)
		return
	}
	values, err := config.ConfigToMap(&candidate)
	if err != nil {
		common.ApiErrorMsg(c, "Unable to serialize TokenPay settings")
		return
	}
	prefixed := make(map[string]string, len(values))
	for key, value := range values {
		prefixed["tokenpay_payment."+key] = value
	}
	if err := model.UpdateOptionsBulk(prefixed); err != nil {
		common.ApiErrorMsg(c, "Unable to save TokenPay settings")
		return
	}
	*current = candidate
	GetTokenPayStatus(c)
}
