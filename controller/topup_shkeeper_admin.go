package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func GetSHKeeperStatus(c *gin.Context) {
	settings := operation_setting.GetSHKeeperPaymentSetting()
	common.ApiSuccess(c, gin.H{
		"enabled": settings.Enabled, "base_url": settings.BaseURL, "api_key_configured": strings.TrimSpace(settings.APIKey) != "", "backend_key_configured": strings.TrimSpace(settings.BackendKey) != "",
		"packages": append([]operation_setting.SHKeeperTopUpPackage{}, settings.Packages...), "enabled_networks": append([]string{}, settings.EnabledNetworks...),
		"invoice_expiry_minutes": settings.InvoiceExpiryMinutes, "reconcile_interval_seconds": settings.ReconcileIntervalSeconds, "allow_private_url": settings.AllowPrivateURL,
	})
}

// The dedicated API accepts backend_key while persistence uses backend_api_key,
// whose suffix is recognized by the general option endpoint's secret filter.
type shkeeperAdminRequest struct {
	operation_setting.SHKeeperPaymentSetting
	BackendSecret string `json:"backend_key"`
}

func SaveSHKeeperSettings(c *gin.Context) {
	var request shkeeperAdminRequest
	if common.DecodeJson(c.Request.Body, &request) != nil {
		common.ApiErrorMsg(c, "Invalid SHKeeper settings")
		return
	}
	current := operation_setting.GetSHKeeperPaymentSetting()
	candidate := request.SHKeeperPaymentSetting
	if strings.TrimSpace(request.BackendSecret) != "" {
		candidate.BackendKey = request.BackendSecret
	}
	if strings.TrimSpace(candidate.APIKey) == "" {
		candidate.APIKey = current.APIKey
	}
	if strings.TrimSpace(candidate.BackendKey) == "" {
		candidate.BackendKey = current.BackendKey
	}
	candidate.Rate = current.Rate
	candidate.MinTopUp = current.MinTopUp
	candidate.MaxTopUp = current.MaxTopUp
	if err := candidate.Normalize(); err != nil {
		common.ApiError(c, err)
		return
	}
	values, err := config.ConfigToMap(&candidate)
	if err != nil {
		common.ApiErrorMsg(c, "Unable to serialize SHKeeper settings")
		return
	}
	prefixed := make(map[string]string, len(values))
	for key, value := range values {
		prefixed["shkeeper_payment."+key] = value
	}
	if err := model.UpdateOptionsBulk(prefixed); err != nil {
		common.ApiErrorMsg(c, "Unable to save SHKeeper settings")
		return
	}
	*current = candidate
	GetSHKeeperStatus(c)
}

func TestSHKeeperConnection(c *gin.Context) {
	request := shkeeperAdminRequest{SHKeeperPaymentSetting: *operation_setting.GetSHKeeperPaymentSetting()}
	if common.DecodeJson(c.Request.Body, &request) != nil {
		common.ApiErrorMsg(c, "Invalid SHKeeper settings")
		return
	}
	candidate := request.SHKeeperPaymentSetting
	if strings.TrimSpace(candidate.APIKey) == "" {
		candidate.APIKey = operation_setting.GetSHKeeperPaymentSetting().APIKey
	}
	candidate.Enabled = true
	if err := candidate.Normalize(); err != nil {
		common.ApiError(c, err)
		return
	}
	client, err := service.NewSHKeeperClient(service.SHKeeperClientConfig{BaseURL: candidate.BaseURL, APIKey: candidate.APIKey, AllowPrivateURL: candidate.AllowPrivateURL}, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cryptos, err := client.ListCrypto(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	available := make(map[string]bool, len(cryptos))
	for _, crypto := range cryptos {
		available[strings.ToUpper(strings.TrimSpace(crypto.Name))] = true
	}
	expected := decimal.NewFromInt(candidate.Packages[0].USDT)
	ready := true
	apiKeyValid := false
	networks := make([]gin.H, 0, len(candidate.EnabledNetworks))
	for _, crypto := range candidate.EnabledNetworks {
		result := gin.H{"crypto": crypto, "available": available[crypto], "quote_ok": false, "amount_matches": false}
		if !available[crypto] {
			ready = false
			networks = append(networks, result)
			continue
		}
		quote, err := client.Quote(c.Request.Context(), crypto, "USD", expected)
		if err != nil {
			ready = false
			result["message"] = "SHKeeper quote failed"
			networks = append(networks, result)
			continue
		}
		apiKeyValid = true
		result["quote_ok"] = true
		result["crypto_amount"] = quote.CryptoAmount
		amount, amountErr := decimal.NewFromString(quote.CryptoAmount)
		fiatAmount, fiatErr := decimal.NewFromString(quote.AmountFiat)
		matches := amountErr == nil && fiatErr == nil && amount.Equal(expected) && fiatAmount.Equal(expected) && quote.Crypto == crypto && quote.Fiat == "USD"
		result["amount_matches"] = matches
		ready = ready && matches
		networks = append(networks, result)
	}
	common.ApiSuccess(c, gin.H{"reachable": true, "api_key_valid": apiKeyValid, "ready": ready, "networks": networks})
}
