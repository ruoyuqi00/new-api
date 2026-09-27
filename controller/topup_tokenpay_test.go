package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTokenPayController(t *testing.T, quoted string, failFirst ...int) (*gin.Engine, *model.User) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.TokenPayTopUpOrder{}, &model.TokenPayCreditedTransaction{}, &model.TokenPayRecoveryClaim{}, &model.AffiliateReward{}, &model.Log{}, &model.Option{}))
	confirmPaymentComplianceForTest(t)
	oldQuota := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = oldQuota })
	settings := operation_setting.GetTokenPayPaymentSetting()
	old := *settings
	t.Cleanup(func() { *settings = old })
	oldCallbackAddress := operation_setting.CustomCallbackAddress
	operation_setting.CustomCallbackAddress = "https://api.example.com"
	t.Cleanup(func() { operation_setting.CustomCallbackAddress = oldCallbackAddress })
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	attempts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/CreateOrder" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		attempts++
		if len(failFirst) > 0 && attempts <= failFirst[0] {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var request map[string]string
		require.NoError(t, common.DecodeJson(r.Body, &request))
		checkout := "https://" + r.Host + "/Pay?Id=invoice-1"
		response := map[string]any{"success": true, "data": checkout, "info": map[string]any{
			"Id": "invoice-1", "OutOrderId": request["OutOrderId"], "OrderUserKey": request["OrderUserKey"],
			"ActualAmount": "10", "Amount": quoted, "BaseCurrency": "USD", "BlockChainName": "TRON",
			"CurrencyName": "USDT", "ToAddress": "TKGTx4pCKiKQbk8evXHTborfZn754TGViP",
			"ExpireTimeUnix": time.Now().Add(30 * time.Minute).Unix(),
			"IsCustomAmount": false, "MinCustomAmount": nil, "MaxCustomAmount": nil,
		}}
		data, err := common.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	trustedTransport := server.Client().Transport.(*http.Transport)
	previousTransport := http.DefaultTransport
	http.DefaultTransport = trustedTransport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	*settings = operation_setting.TokenPayPaymentSetting{Enabled: true, BaseURL: server.URL, APIToken: "test-secret", AllowPrivateURL: true,
		Packages:        []operation_setting.TokenPayTopUpPackage{{USDT: 10, Balance: "66"}},
		EnabledNetworks: []string{operation_setting.TokenPayNetworkTRON}}
	user := &model.User{Username: fmt.Sprintf("tokenpay-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("tpay-%d", time.Now().UnixNano()), Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", user.Id); c.Next() })
	r.POST("/pay", RequestTokenPay)
	r.GET("/order/:trade_no", GetTokenPayOrder)
	r.POST("/order/:trade_no/transaction", SubmitTokenPayTransaction)
	r.POST("/webhook", TokenPayWebhook)
	r.GET("/status", GetTokenPayStatus)
	r.GET("/claims", ListTokenPayRecoveryClaims)
	r.POST("/save", SaveTokenPaySettings)
	r.GET("/info", GetTopUpInfo)
	r.GET("/options", GetOptions)
	return r, user
}

func callTokenPayController(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func decodeTokenPayControllerResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	return response
}

func TestTokenPayPayExposesOnlyExactConfiguredInvoice(t *testing.T) {
	r, user := setupTokenPayController(t, "10")
	bad := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":11,"network":"USDT_TRC20"}`))
	assert.Equal(t, false, bad["success"])
	good := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20","balance_amount":"999"}`))
	require.Equal(t, true, good["success"])
	data := good["data"].(map[string]any)
	assert.Equal(t, "10", data["usdt_amount"])
	assert.Equal(t, "66", data["balance_amount"])
	assert.Equal(t, "TKGTx4pCKiKQbk8evXHTborfZn754TGViP", data["address"])
	order, err := model.GetTokenPayOrder(user.Id, data["trade_no"].(string))
	require.NoError(t, err)
	assert.EqualValues(t, 6600, order.PackageQuota)
	assert.Equal(t, model.TokenPayOrderStatusUnpaid, order.Status)
	badOwner := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/order/other", ""))
	assert.Equal(t, false, badOwner["success"])
}

func TestTokenPayRejectsChangedProviderAmountAndHidesCheckout(t *testing.T) {
	r, user := setupTokenPayController(t, "10.0001")
	w := callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`)
	response := decodeTokenPayControllerResponse(t, w)
	assert.Equal(t, false, response["success"])
	assert.NotContains(t, w.Body.String(), "/Pay?Id=")
	var order model.TokenPayTopUpOrder
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&order).Error)
	assert.Empty(t, order.PaymentURL)
	assert.Nil(t, order.ProviderOrderID)
	assert.Equal(t, model.TokenPayOrderStatusFailed, order.Status)
	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusFailed, topUp.Status)
}

func TestTokenPayAmbiguousCreationResumesSameOrderWithoutExposingEmptyAddress(t *testing.T) {
	r, user := setupTokenPayController(t, "10", 2)
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	pending := created["data"].(map[string]any)
	tradeNo := pending["trade_no"].(string)
	assert.Equal(t, model.TokenPayOrderStatusPending, pending["status"])
	assert.Empty(t, pending["address"])
	assert.Empty(t, pending["payment_url"])

	resumed := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/order/"+tradeNo, ""))
	require.Equal(t, true, resumed["success"])
	invoice := resumed["data"].(map[string]any)
	assert.Equal(t, tradeNo, invoice["trade_no"])
	assert.Equal(t, model.TokenPayOrderStatusUnpaid, invoice["status"])
	assert.NotEmpty(t, invoice["address"])
	var count int64
	require.NoError(t, model.DB.Model(&model.TokenPayTopUpOrder{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestTokenPayUnissuedInvoiceExpiresWithoutCrediting(t *testing.T) {
	r, user := setupTokenPayController(t, "10", 2)
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	tradeNo := created["data"].(map[string]any)["trade_no"].(string)
	require.NoError(t, model.DB.Model(&model.TokenPayTopUpOrder{}).
		Where("trade_no = ?", tradeNo).Update("expires_at", time.Now().Unix()-1).Error)

	response := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/order/"+tradeNo, ""))
	require.Equal(t, true, response["success"])
	data := response["data"].(map[string]any)
	assert.Equal(t, model.TokenPayOrderStatusFailed, data["status"])
	assert.Empty(t, data["address"])
	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusFailed, topUp.Status)
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}

func tokenPaySignedCallback(t *testing.T, fields map[string]any) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if key != "Signature" && value != nil && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, fields[key]))
	}
	mac := hmac.New(sha256.New, []byte("test-secret"))
	_, err := mac.Write([]byte(strings.Join(parts, "&")))
	require.NoError(t, err)
	fields["Signature"] = hex.EncodeToString(mac.Sum(nil))
	body, err := common.Marshal(fields)
	require.NoError(t, err)
	return string(body)
}

func TestTokenPayWebhookCreditsOnceAndRejectsTampering(t *testing.T) {
	r, user := setupTokenPayController(t, "10")
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	tradeNo := created["data"].(map[string]any)["trade_no"].(string)
	order, err := model.GetTokenPayOrder(user.Id, tradeNo)
	require.NoError(t, err)
	fields := map[string]any{"Id": "invoice-1", "OutOrderId": tradeNo, "OrderUserKey": order.OrderUserKey,
		"Currency": "USDT_TRC20", "BaseCurrency": "USD", "BlockChainName": "TRON", "CurrencyName": "USDT",
		"Amount": "10", "ActualAmount": "10", "PayAmount": "10", "ToAddress": order.ReceiveAddress,
		"BlockTransactionId": strings.Repeat("a", 64), "Status": 1, "IsDynamicAmount": false, "IsCustomAmount": false,
		"SignatureType": "HmacSha256"}
	valid := tokenPaySignedCallback(t, fields)
	invalidFields := map[string]any{}
	for key, value := range fields {
		invalidFields[key] = value
	}
	invalidFields["PayAmount"] = "9"
	invalid, err := common.Marshal(invalidFields)
	require.NoError(t, err)
	assert.NotEqual(t, "ok", callTokenPayController(t, r, "POST", "/webhook", string(invalid)).Body.String())
	assert.Equal(t, "ok", callTokenPayController(t, r, "POST", "/webhook", valid).Body.String())
	assert.Equal(t, "ok", callTokenPayController(t, r, "POST", "/webhook", valid).Body.String())
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Equal(t, 6600, user.Quota)
}

func TestTokenPaySignedShortPaymentAndWrongAddressNeverCredit(t *testing.T) {
	r, user := setupTokenPayController(t, "10")
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	tradeNo := created["data"].(map[string]any)["trade_no"].(string)
	order, err := model.GetTokenPayOrder(user.Id, tradeNo)
	require.NoError(t, err)
	fields := map[string]any{"Id": "invoice-1", "OutOrderId": tradeNo, "OrderUserKey": order.OrderUserKey,
		"Currency": "USDT_TRC20", "BaseCurrency": "USD", "BlockChainName": "TRON", "CurrencyName": "USDT",
		"Amount": "10", "ActualAmount": "10", "PayAmount": "9", "ToAddress": order.ReceiveAddress,
		"BlockTransactionId": strings.Repeat("b", 64), "Status": 1, "IsDynamicAmount": false, "IsCustomAmount": false,
		"SignatureType": "HmacSha256"}
	short := tokenPaySignedCallback(t, fields)
	assert.NotEqual(t, "ok", callTokenPayController(t, r, "POST", "/webhook", short).Body.String())
	fields["PayAmount"] = "10"
	fields["ToAddress"] = "TWrongAddress"
	wrongAddress := tokenPaySignedCallback(t, fields)
	assert.NotEqual(t, "ok", callTokenPayController(t, r, "POST", "/webhook", wrongAddress).Body.String())
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}

func TestTokenPayTransactionClaimOnlyRequestsReview(t *testing.T) {
	r, user := setupTokenPayController(t, "10")
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	tradeNo := created["data"].(map[string]any)["trade_no"].(string)
	txHash := strings.Repeat("c", 64)
	review := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/order/"+tradeNo+"/transaction", `{"transaction_id":"`+txHash+`"}`))
	require.Equal(t, true, review["success"])
	assert.Equal(t, true, review["data"].(map[string]any)["review_required"])
	order, err := model.GetTokenPayOrder(user.Id, tradeNo)
	require.NoError(t, err)
	assert.Equal(t, txHash, order.RecoveryHash)
	assert.Equal(t, model.TokenPayOrderStatusUnpaid, order.Status)
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}

func TestTokenPayAdminCanFindSubmittedHashWithoutCrediting(t *testing.T) {
	r, user := setupTokenPayController(t, "10")
	created := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/pay", `{"usdt_amount":10,"network":"USDT_TRC20"}`))
	require.Equal(t, true, created["success"])
	tradeNo := created["data"].(map[string]any)["trade_no"].(string)
	txHash := strings.Repeat("d", 64)
	submitted := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "POST", "/order/"+tradeNo+"/transaction", `{"transaction_id":"`+txHash+`"}`))
	require.Equal(t, true, submitted["success"])

	response := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/claims", ""))
	require.Equal(t, true, response["success"])
	items := response["data"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, tradeNo, item["trade_no"])
	assert.Equal(t, txHash, item["transaction_id"])
	assert.NotEmpty(t, item["receive_address"])
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}

func TestTokenPayAdminStatusAndGeneralOptionsHideSecret(t *testing.T) {
	r, _ := setupTokenPayController(t, "10")
	common.OptionMapRWMutex.Lock()
	previousMap := common.OptionMap
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap["tokenpay_payment.api_token"]
	common.OptionMap["tokenpay_payment.api_token"] = "test-secret"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap["tokenpay_payment.api_token"] = previous
		} else {
			delete(common.OptionMap, "tokenpay_payment.api_token")
		}
		common.OptionMap = previousMap
	})
	status := callTokenPayController(t, r, "GET", "/status", "")
	assert.NotContains(t, status.Body.String(), "test-secret")
	assert.Contains(t, status.Body.String(), "api_token_configured")
	options := callTokenPayController(t, r, "GET", "/options", "")
	assert.NotContains(t, options.Body.String(), "test-secret")
	assert.NotContains(t, options.Body.String(), "tokenpay_payment.api_token")
}

func TestTokenPaySettingsSavePreservesWriteOnlySecret(t *testing.T) {
	r, _ := setupTokenPayController(t, "10")
	w := callTokenPayController(t, r, "POST", "/save", `{"enabled":true,"base_url":"https://pay.example.com","api_token":"","packages":[{"usdt":10,"balance":"66"}],"enabled_networks":["USDT_TRC20"]}`)
	response := decodeTokenPayControllerResponse(t, w)
	assert.Equal(t, true, response["success"])
	assert.NotContains(t, w.Body.String(), "test-secret")
	assert.Equal(t, "test-secret", operation_setting.GetTokenPayPaymentSetting().APIToken)
	w = callTokenPayController(t, r, "POST", "/save", `{"enabled":true,"base_url":"https://pay.example.com","api_token":"","packages":[{"usdt":10,"balance":"66"}],"enabled_networks":["EVM_ETH_USDT_ERC20"]}`)
	response = decodeTokenPayControllerResponse(t, w)
	assert.Equal(t, false, response["success"])
}

func TestTokenPayTopUpInfoOnlyOffersConfiguredPackagesWhenEnabled(t *testing.T) {
	r, _ := setupTokenPayController(t, "10")
	response := decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/info", ""))
	require.Equal(t, true, response["success"])
	data := response["data"].(map[string]any)
	assert.Equal(t, true, data["enable_tokenpay_topup"])
	assert.Len(t, data["tokenpay_packages"], 1)
	assert.Equal(t, []any{"USDT_TRC20"}, data["tokenpay_networks"])
	operation_setting.GetTokenPayPaymentSetting().Enabled = false
	response = decodeTokenPayControllerResponse(t, callTokenPayController(t, r, "GET", "/info", ""))
	data = response["data"].(map[string]any)
	assert.Equal(t, false, data["enable_tokenpay_topup"])
}
