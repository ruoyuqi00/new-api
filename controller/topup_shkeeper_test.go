package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func setupSHKeeperController(t *testing.T, handler http.HandlerFunc) (*gin.Engine, *model.User) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SHKeeperTopUpOrder{}, &model.SHKeeperCreditedTransaction{}, &model.Log{}, &model.Option{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	confirmPaymentComplianceForTest(t)
	oldQuota := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = oldQuota })
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	settings := operation_setting.GetSHKeeperPaymentSetting()
	old := *settings
	t.Cleanup(func() { *settings = old })
	*settings = operation_setting.SHKeeperPaymentSetting{Enabled: true, BaseURL: server.URL, APIKey: "api-secret", BackendKey: "backend-secret", AllowPrivateURL: true, Packages: []operation_setting.SHKeeperTopUpPackage{{USDT: 10, Balance: "66"}, {USDT: 20, Balance: "140"}}, EnabledNetworks: []string{"USDT"}, InvoiceExpiryMinutes: 30, ReconcileIntervalSeconds: 60}
	user := &model.User{Username: "shkeeper", AffCode: "shkeeper", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", user.Id); c.Next() })
	r.POST("/pay", RequestSHKeeperPay)
	r.GET("/order/:trade_no", GetSHKeeperOrder)
	r.POST("/order/:trade_no/transaction", SubmitSHKeeperTransaction)
	r.POST("/webhook", SHKeeperWebhook)
	r.GET("/status", GetSHKeeperStatus)
	r.POST("/save", SaveSHKeeperSettings)
	r.POST("/test", TestSHKeeperConnection)
	r.GET("/info", GetTopUpInfo)
	r.GET("/options", GetOptions)
	return r, user
}

func callSHKeeper(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func decodeSHKeeperResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	return response
}

func TestSHKeeperPayResolvesPackageAndRejectsTamperingBeforeCreate(t *testing.T) {
	creates := 0
	r, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
		creates++
		assert.Equal(t, "/api/v1/USDT/payment_request", r.URL.Path)
		var input map[string]string
		require.NoError(t, common.DecodeJson(r.Body, &input))
		assert.Equal(t, "10", input["amount"])
		assert.Equal(t, "USD", input["fiat"])
		fmt.Fprint(w, `{"status":"success","id":1,"amount":"10","wallet":"TAddress"}`)
	})
	response := decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/pay", `{"usdt_amount":11,"crypto":"USDT"}`))
	assert.Equal(t, false, response["success"])
	assert.Zero(t, creates)
	response = decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/pay", `{"usdt_amount":10,"crypto":"USDT","balance_amount":"9999999"}`))
	require.Equal(t, true, response["success"])
	data := response["data"].(map[string]any)
	assert.Equal(t, "66", data["balance_amount"])
	assert.Equal(t, "10", data["usdt_amount"])
	assert.Equal(t, "TAddress", data["address"])
	assert.Equal(t, 1, creates)
	order, err := model.GetSHKeeperTopUpByTradeNo(user.Id, data["trade_no"].(string))
	require.NoError(t, err)
	assert.EqualValues(t, 6600, order.PackageQuota)
	require.NoError(t, model.DB.Model(user).Update("quota", operation_setting.SHKeeperMaxUserQuota-6599).Error)
	response = decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/pay", `{"usdt_amount":10,"crypto":"USDT"}`))
	assert.Equal(t, false, response["success"])
	assert.Equal(t, 1, creates)
}

func TestSHKeeperAmbiguousCreateNeverRetriesAndInvalidInvoiceNeverExposesAddress(t *testing.T) {
	for _, scenario := range []string{"ambiguous", "amount", "crypto", "identity"} {
		t.Run(scenario, func(t *testing.T) {
			creates := 0
			lookups := 0
			r, _ := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/invoices/") {
					lookups++
					fmt.Fprint(w, `{"status":"success","invoices":[]}`)
					return
				}
				creates++
				if scenario == "ambiguous" {
					w.WriteHeader(504)
					return
				}
				invoice := map[string]any{"status": "success", "amount": "10", "wallet": "BAD-ADDRESS"}
				switch scenario {
				case "amount":
					invoice["amount"] = "11"
				case "crypto":
					invoice["crypto"] = "BNB-USDT"
				case "identity":
					invoice["external_id"] = "wrong"
				}
				data, err := common.Marshal(invoice)
				require.NoError(t, err)
				w.Write(data)
			})
			response := callSHKeeper(t, r, "POST", "/pay", `{"usdt_amount":10,"crypto":"USDT"}`)
			assert.NotContains(t, response.Body.String(), "BAD-ADDRESS")
			assert.Equal(t, 1, creates)
			assert.LessOrEqual(t, lookups, 1)
			var orders []model.SHKeeperTopUpOrder
			require.NoError(t, model.DB.Find(&orders).Error)
			require.Len(t, orders, 1)
			assert.Empty(t, orders[0].InvoiceAddress)
			callSHKeeper(t, r, "GET", "/order/"+orders[0].TradeNo, "")
			assert.Equal(t, 1, creates)
		})
	}
}

func insertSHKeeperControllerOrder(t *testing.T, user *model.User) *model.SHKeeperTopUpOrder {
	t.Helper()
	order := &model.SHKeeperTopUpOrder{TradeNo: "USDT1abc", ExternalID: "USDT1abc", UserID: user.Id, Crypto: "USDT", SettlementMode: model.SHKeeperSettlementModeFixedPackage, RequestedUSDT: "10", PackageBalance: "66", InvoiceAddress: "TAddress", ReceivedUSDT: "0", CreditedBalance: "0", Status: model.SHKeeperOrderStatusUnpaid, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, model.InsertSHKeeperTopUp(&model.TopUp{TradeNo: order.TradeNo, UserId: user.Id, PaymentMethod: model.PaymentMethodSHKeeper, PaymentProvider: model.PaymentProviderSHKeeper, Status: common.TopUpStatusPending}, order))
	return order
}

func TestSHKeeperManualRescanAndOwnership(t *testing.T) {
	rescans := 0
	r, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
		rescans++
		assert.Equal(t, "/api/v1/walletnotify/USDT/"+strings.Repeat("a", 64), r.URL.Path)
		assert.Equal(t, "backend-secret", r.Header.Get("X-Shkeeper-Backend-Key"))
		fmt.Fprint(w, `{"status":"success"}`)
	})
	order := insertSHKeeperControllerOrder(t, user)
	response := decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/order/"+order.TradeNo+"/transaction", `{"txid":"`+strings.Repeat("a", 64)+`"}`))
	assert.Equal(t, true, response["success"])
	assert.Equal(t, 1, rescans)
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
	require.NoError(t, model.DB.Model(order).Update("user_id", user.Id+1).Error)
	assert.Equal(t, false, decodeSHKeeperResponse(t, callSHKeeper(t, r, "GET", "/order/"+order.TradeNo, ""))["success"])
	assert.Equal(t, false, decodeSHKeeperResponse(t, callSHKeeper(t, r, "POST", "/order/"+order.TradeNo+"/transaction", `{"txid":"`+strings.Repeat("a", 64)+`"}`))["success"])
	assert.Equal(t, 1, rescans)
}

func TestSHKeeperWebhookQueriesBeforeCreditAndAcknowledgement(t *testing.T) {
	queries := 0
	fail := false
	unfunded := false
	r, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
		queries++
		assert.Equal(t, "/api/v1/invoices/USDT1abc", r.URL.Path)
		if fail {
			w.WriteHeader(503)
			return
		}
		if unfunded {
			fmt.Fprint(w, `{"status":"success","invoices":[{"external_id":"USDT1abc","fiat":"USD","amount_fiat":"10","balance_fiat":"0","status":"UNPAID","txs":[]}]}`)
			return
		}
		fmt.Fprintf(w, `{"status":"success","invoices":[{"external_id":"USDT1abc","fiat":"USD","amount_fiat":"10","balance_fiat":"10","status":"PAID","txs":[{"amount":"10","crypto":"USDT","addr":"TAddress","txid":"%s","status":"CONFIRMED"}]}]}`, strings.Repeat("a", 64))
	})
	insertSHKeeperControllerOrder(t, user)
	body := `{"external_id":"USDT1abc","crypto":"USDT","addr":"TAddress","status":"PAID"}`
	timestamp := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("api-secret"))
	mac.Write([]byte(timestamp + "." + body))
	signature := hex.EncodeToString(mac.Sum(nil))
	for _, scenario := range []string{"invalid", "failed", "unfunded", "valid", "replay"} {
		t.Run(scenario, func(t *testing.T) {
			fail = scenario == "failed"
			unfunded = scenario == "unfunded"
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
			req.Header.Set("X-Shkeeper-Timestamp", timestamp)
			req.Header.Set("X-Shkeeper-Signature", signature)
			if scenario == "invalid" {
				req.Header.Set("X-Shkeeper-Signature", "bad")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if scenario == "valid" || scenario == "replay" || scenario == "unfunded" {
				assert.Equal(t, http.StatusAccepted, w.Code)
			} else {
				assert.NotEqual(t, http.StatusAccepted, w.Code)
			}
			require.NoError(t, model.DB.First(user, user.Id).Error)
			if scenario == "invalid" || scenario == "failed" || scenario == "unfunded" {
				assert.Zero(t, user.Quota)
			} else {
				assert.Equal(t, 6600, user.Quota)
			}
		})
	}
	assert.Equal(t, 4, queries)
}

func TestSHKeeperWebhookRejectsOversizedAndMismatchedIdentity(t *testing.T) {
	r, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid callback must not reach the provider")
	})
	insertSHKeeperControllerOrder(t, user)
	for _, item := range []struct {
		name   string
		body   string
		status int
	}{
		{"oversized", strings.Repeat("x", (1<<20)+1), http.StatusBadRequest},
		{"invalid-json", "{", http.StatusBadRequest},
		{"address", `{"external_id":"USDT1abc","crypto":"USDT","addr":"wrong"}`, http.StatusBadRequest},
		{"network", `{"external_id":"USDT1abc","crypto":"BNB-USDT","addr":"TAddress"}`, http.StatusBadRequest},
		{"external-id", `{"external_id":"wrong","crypto":"USDT","addr":"TAddress"}`, http.StatusNotFound},
	} {
		t.Run(item.name, func(t *testing.T) {
			timestamp := fmt.Sprint(time.Now().Unix())
			mac := hmac.New(sha256.New, []byte("api-secret"))
			mac.Write([]byte(timestamp + "." + item.body))
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(item.body))
			req.Header.Set("X-Shkeeper-Timestamp", timestamp)
			req.Header.Set("X-Shkeeper-Signature", hex.EncodeToString(mac.Sum(nil)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, item.status, w.Code)
		})
	}
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}
