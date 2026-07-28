package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type grokOAuthExchangeInput struct {
	SessionId   string `json:"session_id"`
	CallbackURL string `json:"callback_url"`
}

func GenerateAccountPoolGrokOAuthAuthorization(c *gin.Context) {
	authorization, err := service.GenerateGrokOAuthAuthorization()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": authorization})
}

func ExchangeAccountPoolGrokOAuthAuthorization(c *gin.Context) {
	var input grokOAuthExchangeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := service.ExchangeGrokOAuthAuthorization(c.Request.Context(), input.SessionId, input.CallbackURL)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.BuildGrokOAuthCredential(token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(token.Email)
	if name == "" {
		name = "Grok OAuth"
	}
	account := providerAccountInput{
		Name: name, Type: "oauth_json", Credential: credential, BaseURL: constant.GrokOAuthBaseURL,
		Status: model.ProviderAccountEnabled, Weight: 100, ConcurrencyLimit: 1,
		CooldownSeconds: 20, ExpiresAt: token.ExpiresAt, AdapterType: constant.ChannelTypeXai,
	}
	recordManageAudit(c, "account_pool.grok_oauth.exchange", map[string]interface{}{
		"account_name": name, "expires_at": token.ExpiresAt,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": account})
}
