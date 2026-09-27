package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenPaySignatureMatchesOfficialHMACVector(t *testing.T) {
	fields := map[string]string{
		"ActualAmount": "15", "Currency": "TRX", "NotifyUrl": "http://localhost:1011/pay/tokenpay/notify_url",
		"OrderUserKey": "admin@qq.com", "OutOrderId": "AJIHK72N34BR2CWG",
		"RedirectUrl": "http://localhost:1011/pay/tokenpay/return_url?order_id=AJIHK72N34BR2CWG",
	}
	assert.Equal(t, "c879776795a9e85ce674aa10c8315de323d6f8a20bf157f92595b71ad77f1e12", SignTokenPayParameters("666", fields))
}

func TestTokenPayCallbackPreservesMonetaryTextAndRejectsTampering(t *testing.T) {
	canonical := "ActualAmount=10.00&Amount=10.00&BaseCurrency=USD&BlockChainName=TRON&BlockTransactionId=abc&Currency=USDT_TRC20&Id=invoice-1&IsCustomAmount=false&IsDynamicAmount=false&OrderUserKey=order-1&OutOrderId=order-1&PayAmount=10.00&Status=1&ToAddress=Taddress"
	mac := hmac.New(sha256.New, []byte("secret"))
	_, err := mac.Write([]byte(canonical))
	require.NoError(t, err)
	signature := hex.EncodeToString(mac.Sum(nil))
	fields := map[string]any{
		"ActualAmount": "10.00", "Amount": "10.00", "BaseCurrency": "USD", "BlockChainName": "TRON",
		"BlockTransactionId": "abc", "Currency": "USDT_TRC20", "Id": "invoice-1", "IsCustomAmount": false,
		"IsDynamicAmount": false, "OrderUserKey": "order-1", "OutOrderId": "order-1", "PayAmount": "10.00",
		"Status": 1, "ToAddress": "Taddress", "Signature": signature,
	}
	body, err := common.Marshal(fields)
	require.NoError(t, err)
	callback, err := VerifyTokenPayCallback("secret", body)
	require.NoError(t, err)
	assert.Equal(t, "10.00", callback.PayAmount)
	assert.Equal(t, "order-1", callback.OutOrderID)
	fields["PayAmount"] = "9.00"
	body, err = common.Marshal(fields)
	require.NoError(t, err)
	_, err = VerifyTokenPayCallback("secret", body)
	require.Error(t, err)
	_, err = VerifyTokenPayCallback("secret", []byte(`{"Status":0,"Status":1,"Signature":"abc"}`))
	require.Error(t, err)
}

func TestTokenPayCreateOrderValidatesFixedQuoteAndHostedCheckout(t *testing.T) {
	for _, test := range []struct {
		name       string
		amount     string
		checkout   string
		omitExpiry bool
		wantError  bool
	}{
		{name: "exact fixed amount", amount: "10"},
		{name: "tail added", amount: "10.0001", wantError: true},
		{name: "external checkout", amount: "10", checkout: "https://other.example.com/Pay", wantError: true},
		{name: "timezone-less legacy expiry", amount: "10", omitExpiry: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/CreateOrder", r.URL.Path)
				assert.Equal(t, http.MethodPost, r.Method)
				body, readErr := io.ReadAll(r.Body)
				require.NoError(t, readErr)
				var request map[string]string
				require.NoError(t, common.Unmarshal(body, &request))
				assert.Equal(t, "order-1-TRON", request["OrderUserKey"])
				assert.Equal(t, "10", request["ActualAmount"])
				assert.Equal(t, "USDT_TRC20", request["Currency"])
				checkout := "https://" + r.Host + "/Pay?Id=invoice-1"
				if test.checkout != "" {
					checkout = test.checkout
				}
				info := map[string]any{
					"Id": "invoice-1", "OutOrderId": "order-1", "OrderUserKey": "order-1-TRON",
					"ActualAmount": "10", "Amount": test.amount, "BaseCurrency": "USD", "BlockChainName": "TRON",
					"CurrencyName": "USDT", "ToAddress": "TKGTx4pCKiKQbk8evXHTborfZn754TGViP", "ExpireTimeUnix": time.Now().Add(30 * time.Minute).Unix(),
					"IsCustomAmount": false, "MinCustomAmount": nil, "MaxCustomAmount": nil,
				}
				if test.omitExpiry {
					delete(info, "ExpireTimeUnix")
					info["ExpireTime"] = "2030-01-01 00:00:00"
				}
				response := map[string]any{"success": true, "data": checkout, "info": info}
				data, marshalErr := common.Marshal(response)
				require.NoError(t, marshalErr)
				_, writeErr := w.Write(data)
				require.NoError(t, writeErr)
			}))
			defer server.Close()
			client, err := NewTokenPayClient(server.URL, "secret", true, server.Client())
			require.NoError(t, err)
			result, err := client.CreateOrder(context.Background(), TokenPayCreateRequest{
				OutOrderID: "order-1", OrderUserKey: "order-1-TRON", ActualAmount: "10", Currency: "USDT_TRC20",
				NotifyURL: "https://api.example.com/api/tokenpay/webhook", RedirectURL: "https://api.example.com/wallet",
			})
			if test.wantError {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrTokenPayInvoiceInvalid)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "10", result.PayAmountUSDT)
			assert.Equal(t, "invoice-1", result.ProviderOrderID)
		})
	}
}

func TestTokenPayCreateOrderUsesAbsoluteExpiryNotTimezoneLessDisplay(t *testing.T) {
	expires := time.Now().Add(25 * time.Minute).Unix()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{"success": true, "data": "https://" + r.Host + "/Pay?Id=invoice-utc", "info": map[string]any{
			"Id": "invoice-utc", "OutOrderId": "order-utc", "OrderUserKey": "order-utc-TRON",
			"Amount": "10", "ActualAmount": "10", "BaseCurrency": "USD", "BlockChainName": "TRON",
			"CurrencyName": "USDT", "ToAddress": "TKGTx4pCKiKQbk8evXHTborfZn754TGViP",
			"ExpireTime": "2000-01-01 00:00:00", "ExpireTimeUnix": expires,
		}}
		payload, err := common.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(payload)
		require.NoError(t, err)
	}))
	defer server.Close()
	client, err := NewTokenPayClient(server.URL, "secret", true, server.Client())
	require.NoError(t, err)
	result, err := client.CreateOrder(context.Background(), TokenPayCreateRequest{
		OutOrderID: "order-utc", OrderUserKey: "order-utc-TRON", ActualAmount: "10", Currency: "USDT_TRC20",
		NotifyURL: "https://api.example.com/api/tokenpay/webhook", RedirectURL: "https://api.example.com/wallet",
	})
	require.NoError(t, err)
	assert.Equal(t, expires, result.ExpiresAt)
}

func TestTokenPayCreateOrderRetriesAmbiguousFailureWithSameOrderIdentity(t *testing.T) {
	var attempts int
	var orderIDs []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var request map[string]string
		require.NoError(t, common.DecodeJson(r.Body, &request))
		orderIDs = append(orderIDs, request["OutOrderId"])
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		response := map[string]any{"success": true, "data": "https://" + r.Host + "/Pay?Id=invoice-reused", "info": map[string]any{
			"Id": "invoice-reused", "OutOrderId": request["OutOrderId"], "OrderUserKey": request["OrderUserKey"],
			"Amount": "10", "ActualAmount": "10", "BaseCurrency": "USD", "BlockChainName": "TRON",
			"CurrencyName": "USDT", "ToAddress": "TKGTx4pCKiKQbk8evXHTborfZn754TGViP",
			"ExpireTimeUnix": time.Now().Add(30 * time.Minute).Unix(), "IsCustomAmount": false, "MinCustomAmount": nil,
		}}
		payload, err := common.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(payload)
		require.NoError(t, err)
	}))
	defer server.Close()
	client, err := NewTokenPayClient(server.URL, "secret", true, server.Client())
	require.NoError(t, err)

	result, err := client.CreateOrder(context.Background(), TokenPayCreateRequest{
		OutOrderID: "order-once", OrderUserKey: "order-once-TRON", ActualAmount: "10", Currency: "USDT_TRC20",
		NotifyURL: "https://api.example.com/api/tokenpay/webhook", RedirectURL: "https://api.example.com/wallet",
	})

	require.NoError(t, err)
	assert.Equal(t, "invoice-reused", result.ProviderOrderID)
	assert.Equal(t, []string{"order-once", "order-once"}, orderIDs)
}

func TestNormalizeTokenPayTransactionIDByNetwork(t *testing.T) {
	tronHash := strings.Repeat("a", 64)
	evmHash := "0x" + strings.Repeat("b", 64)
	got, err := NormalizeTokenPayTransactionID("USDT_TRC20", " "+tronHash+" ")
	require.NoError(t, err)
	assert.Equal(t, tronHash, got)
	got, err = NormalizeTokenPayTransactionID("EVM_BSC_USDT_BEP20", strings.ToUpper(evmHash))
	require.NoError(t, err)
	assert.Equal(t, evmHash, got)
	_, err = NormalizeTokenPayTransactionID("USDT_TRC20", evmHash)
	require.Error(t, err)
	_, err = NormalizeTokenPayTransactionID("EVM_Polygon_USDT_ERC20", "bad")
	require.Error(t, err)
}
