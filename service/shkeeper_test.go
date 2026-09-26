package service

import (
	"context"
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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHKeeperClientPinnedContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/walletnotify/") {
			assert.Equal(t, "backend-secret", r.Header.Get("X-Shkeeper-Backend-Key"))
			assert.Empty(t, r.Header.Get("X-Shkeeper-Api-Key"))
			fmt.Fprint(w, `{"status":"success"}`)
			return
		}
		assert.Equal(t, "api-secret", r.Header.Get("X-Shkeeper-Api-Key"))
		switch r.URL.Path {
		case "/api/v1/crypto":
			fmt.Fprint(w, `{"status":"success","crypto":["USDT"],"crypto_list":[{"name":"USDT","display_name":"Tether"}]}`)
		case "/api/v1/USDT/quote":
			assert.Equal(t, http.MethodPost, r.Method)
			var payload map[string]string
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, map[string]string{"fiat": "USD", "amount": "10"}, payload)
			fmt.Fprint(w, `{"status":"success","amount_crypto":"10","exchange_rate":"1","fiat":"USD","amount_fiat":"10","crypto":"USDT"}`)
		case "/api/v1/USDT/payment_request":
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, map[string]any{"external_id": "USDT1abc", "fiat": "USD", "amount": "10", "callback_url": "https://gateway.example/api/shkeeper/webhook"}, payload)
			fmt.Fprint(w, `{"status":"success","id":1,"amount":"10","wallet":"TAddress"}`)
		case "/api/v1/invoices/USDT1abc":
			fmt.Fprint(w, `{"status":"success","invoices":[{"external_id":"USDT1abc","fiat":"USD","amount_fiat":"10","balance_fiat":"10","status":"PAID","txs":[{"amount":"10","crypto":"USDT","addr":"TAddress","txid":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"CONFIRMED"}]}]}`)
		default:
			t.Errorf("unexpected provider request %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	original := server.Client()
	original.Timeout = time.Second
	client, err := NewSHKeeperClient(SHKeeperClientConfig{BaseURL: server.URL, APIKey: "api-secret", BackendKey: "backend-secret", AllowPrivateURL: true}, original)
	require.NoError(t, err)
	assert.Equal(t, time.Second, original.Timeout)
	assert.Nil(t, original.CheckRedirect)
	cryptos, err := client.ListCrypto(context.Background())
	require.NoError(t, err)
	require.Len(t, cryptos, 1)
	assert.Equal(t, "USDT", cryptos[0].Name)
	quote, err := client.Quote(context.Background(), "USDT", "USD", decimal.NewFromInt(10))
	require.NoError(t, err)
	assert.Equal(t, "10", quote.CryptoAmount)
	invoice, err := client.CreatePaymentRequest(context.Background(), SHKeeperPaymentRequest{Crypto: "USDT", ExternalID: "USDT1abc", Fiat: "USD", Amount: "10", CallbackURL: "https://gateway.example/api/shkeeper/webhook"})
	require.NoError(t, err)
	assert.Equal(t, "TAddress", invoice.Wallet)
	lookup, err := client.GetInvoiceByExternalID(context.Background(), "USDT", "USDT1abc")
	require.NoError(t, err)
	assert.Equal(t, "10", lookup.AmountFiat)
	require.Len(t, lookup.Transactions, 1)
	require.NoError(t, client.NotifyTransaction(context.Background(), "USDT", strings.Repeat("a", 64)))
}

func TestSHKeeperClientRejectsUnsafeTargetsAndResponses(t *testing.T) {
	for _, target := range []string{"http://169.254.169.254", "http://[fe80::1]", "http://100.100.100.200", "http://user:pass@example.com", "http://localhost"} {
		_, err := NewSHKeeperClient(SHKeeperClientConfig{BaseURL: target, APIKey: "secret", AllowPrivateURL: target != "http://localhost"}, nil)
		assert.Error(t, err, target)
	}
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("cross-host redirect followed") }))
	defer destination.Close()
	for _, mode := range []string{"redirect", "large", "error"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "redirect":
					http.Redirect(w, r, destination.URL, 302)
				case "large":
					fmt.Fprint(w, strings.Repeat("x", (2<<20)+1))
				case "error":
					w.WriteHeader(500)
					fmt.Fprint(w, "api-secret backend-secret")
				}
			}))
			defer server.Close()
			client, err := NewSHKeeperClient(SHKeeperClientConfig{BaseURL: server.URL, APIKey: "api-secret", BackendKey: "backend-secret", AllowPrivateURL: true}, server.Client())
			require.NoError(t, err)
			_, err = client.ListCrypto(context.Background())
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "api-secret")
			assert.NotContains(t, err.Error(), "backend-secret")
		})
	}
}

func TestSHKeeperClientDoesNotReplayCreateOnSameHostRedirect(t *testing.T) {
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creates++
		if r.URL.Path == "/api/v1/USDT/payment_request" {
			http.Redirect(w, r, "/redirected-create", http.StatusTemporaryRedirect)
			return
		}
		fmt.Fprint(w, `{"status":"success","amount":"10","wallet":"TAddress"}`)
	}))
	defer server.Close()
	client, err := NewSHKeeperClient(SHKeeperClientConfig{BaseURL: server.URL, APIKey: "secret", AllowPrivateURL: true}, server.Client())
	require.NoError(t, err)
	_, err = client.CreatePaymentRequest(context.Background(), SHKeeperPaymentRequest{Crypto: "USDT", ExternalID: "USDT1abc", Fiat: "USD", Amount: "10", CallbackURL: "https://gateway.example/api/shkeeper/webhook"})
	assert.Error(t, err)
	assert.Equal(t, 1, creates, "a redirected POST must not create another invoice")
}

func TestNormalizeSHKeeperCryptoAndTransaction(t *testing.T) {
	for _, crypto := range []string{"usdt", " bnb-usdt ", "POLYGON-USDT"} {
		_, err := NormalizeSHKeeperCrypto(crypto)
		require.NoError(t, err)
	}
	_, err := NormalizeSHKeeperCrypto("ETH")
	require.Error(t, err)
	for _, item := range []struct {
		crypto, hash string
		valid        bool
	}{{"USDT", strings.Repeat("A", 64), true}, {"BNB-USDT", "0x" + strings.Repeat("a", 64), true}, {"POLYGON-USDT", "0x" + strings.Repeat("a", 64), true}, {"USDT", "bad", false}, {"BNB-USDT", strings.Repeat("a", 64), false}} {
		hash, err := NormalizeSHKeeperTransactionID(item.crypto, item.hash)
		if item.valid {
			require.NoError(t, err)
			assert.Equal(t, strings.ToLower(item.hash), hash)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestVerifySHKeeperWebhookRawBodyAndReplay(t *testing.T) {
	now := time.Unix(1800000000, 0)
	body := []byte(`{"external_id":"USDT1abc"}`)
	timestamp := fmt.Sprint(now.Unix())
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	require.NoError(t, VerifySHKeeperWebhook("secret", timestamp, signature, body, now))
	assert.Error(t, VerifySHKeeperWebhook("secret", timestamp, signature, append(body, ' '), now))
	assert.Error(t, VerifySHKeeperWebhook("secret", timestamp, signature, body, now.Add(301*time.Second)))
	assert.Error(t, VerifySHKeeperWebhook("secret", timestamp, signature, body, now.Add(-301*time.Second)))
	assert.Error(t, VerifySHKeeperWebhook("", timestamp, signature, body, now))
}
