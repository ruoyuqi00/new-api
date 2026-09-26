package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHKeeperSettingsWriteOnlySecretsAndEmptyCollections(t *testing.T) {
	r, _ := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected provider request") })
	oldOptions := common.OptionMap
	common.OptionMap = map[string]string{"shkeeper_payment.api_key": "api-secret", "shkeeper_payment.backend_api_key": "backend-secret", "shkeeper_payment.rate": "6.6", "shkeeper_payment.min_top_up": "1", "shkeeper_payment.max_top_up": "500"}
	t.Cleanup(func() { common.OptionMap = oldOptions })
	for _, path := range []string{"/status", "/options"} {
		response := callSHKeeper(t, r, "GET", path, "")
		assert.NotContains(t, response.Body.String(), "api-secret")
		assert.NotContains(t, response.Body.String(), "backend-secret")
		assert.NotContains(t, response.Body.String(), "min_top_up")
		assert.NotContains(t, response.Body.String(), "max_top_up")
		assert.NotContains(t, response.Body.String(), `"rate"`)
		assert.NotContains(t, response.Body.String(), "shkeeper_payment.rate")
	}
	saved := decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/save", `{"enabled":false,"api_key":" ","backend_api_key":"","enabled_networks":[],"packages":[]}`))
	require.Equal(t, true, saved["success"])
	settings := operation_setting.GetSHKeeperPaymentSetting()
	assert.Equal(t, "api-secret", settings.APIKey)
	assert.Equal(t, "backend-secret", settings.BackendKey)
	response := decodeSHKeeperResponse(t, callSHKeeper(t, r, "GET", "/status", ""))
	data := response["data"].(map[string]any)
	assert.Equal(t, []any{}, data["packages"])
	assert.Equal(t, []any{}, data["enabled_networks"])
	assert.Equal(t, true, data["api_key_configured"])
	assert.Equal(t, true, data["backend_key_configured"])
	var option model.Option
	require.NoError(t, model.DB.Where("key = ?", "shkeeper_payment.packages").First(&option).Error)
	assert.Equal(t, "[]", option.Value)
}

func TestSHKeeperConnectionSmallestPackageAndExactQuote(t *testing.T) {
	mismatch := false
	missingNetwork := false
	quotes := 0
	r, _ := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/crypto" {
			if missingNetwork {
				fmt.Fprint(w, `{"status":"success","crypto":["BNB-USDT"]}`)
			} else {
				fmt.Fprint(w, `{"status":"success","crypto":["USDT"]}`)
			}
			return
		}
		quotes++
		assert.Equal(t, "/api/v1/USDT/quote", r.URL.Path)
		var input map[string]string
		require.NoError(t, common.DecodeJson(r.Body, &input))
		assert.Equal(t, "10", input["amount"])
		amount := "10"
		if mismatch {
			amount = "10.01"
		}
		fmt.Fprintf(w, `{"status":"success","amount_crypto":"%s","amount_fiat":"10","fiat":"USD","crypto":"USDT","exchange_rate":"1"}`, amount)
	})
	settings := operation_setting.GetSHKeeperPaymentSetting()
	settings.Packages[0], settings.Packages[1] = settings.Packages[1], settings.Packages[0]
	response := decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/test", `{}`))
	require.Equal(t, true, response["success"])
	data := response["data"].(map[string]any)
	assert.Equal(t, true, data["ready"])
	assert.Contains(t, fmt.Sprint(data), "amount_matches:true")
	mismatch = true
	response = decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/test", `{}`))
	assert.Equal(t, false, response["data"].(map[string]any)["ready"])
	mismatch = false
	missingNetwork = true
	response = decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/test", `{}`))
	assert.Equal(t, false, response["data"].(map[string]any)["ready"])
	assert.GreaterOrEqual(t, quotes, 2)
}

func TestSHKeeperPublicInfoUsesPackages(t *testing.T) {
	r, _ := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected provider request") })
	response := callSHKeeper(t, r, "GET", "/info", "")
	data := decodeSHKeeperResponse(t, response)["data"].(map[string]any)
	assert.Equal(t, true, data["enable_shkeeper_topup"])
	assert.Len(t, data["shkeeper_packages"], 2)
	assert.Len(t, data["shkeeper_networks"], 1)
	assert.EqualValues(t, 30, data["shkeeper_invoice_expiry_minutes"])
	assert.Equal(t, true, data["shkeeper_transaction_recovery_enabled"])
	for _, secret := range []string{"api-secret", "backend-secret", "shkeeper_rate", "shkeeper_min_topup", "shkeeper_max_topup"} {
		assert.False(t, strings.Contains(response.Body.String(), secret))
	}
}
