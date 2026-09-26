package controller

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
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func signedSHKeeperCallback(t *testing.T, router *gin.Engine, status string, transactions []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(map[string]any{"external_id": "USDT1abc", "crypto": "USDT", "addr": "TAddress", "status": status, "transactions": transactions})
	require.NoError(t, err)
	timestamp := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("api-secret"))
	mac.Write(append([]byte(timestamp+"."), body...))
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Shkeeper-Timestamp", timestamp)
	req.Header.Set("X-Shkeeper-Signature", hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestSHKeeperConfirmationAuthorization(t *testing.T) {
	for _, scenario := range []string{"threshold-and-replay", "reconcile-db-failure", "unconfirmed", "missing-trigger", "wrong-trigger-network", "invalid-trigger-id", "only-trigger-is-authorized"} {
		t.Run(scenario, func(t *testing.T) {
			queries := 0
			router, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) {
				queries++
				assert.Equal(t, "api-secret", r.Header.Get("X-Shkeeper-Api-Key"))
				amount := "10"
				extra := ""
				if scenario == "only-trigger-is-authorized" {
					amount = "4"
					extra = fmt.Sprintf(`,{"amount":"6","crypto":"USDT","addr":"TAddress","txid":"%s","status":"CONFIRMED"}`, strings.Repeat("b", 64))
				}
				fmt.Fprintf(w, `{"status":"success","invoices":[{"external_id":"USDT1abc","fiat":"USD","amount_fiat":"10","balance_fiat":"10","status":"PAID","txs":[{"amount":"%s","crypto":"USDT","addr":"TAddress","txid":"%s","status":"CONFIRMED"}%s]}]}`, amount, strings.Repeat("a", 64), extra)
			})
			order := insertSHKeeperControllerOrder(t, user)
			// v2.5.32 already calls a stored first-confirmation row CONFIRMED.
			before, err := service.ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
			require.NoError(t, err)
			assert.Zero(t, before.CreditedQuotaDelta)
			assert.Equal(t, "0", before.ReceivedUSDT)
			require.NoError(t, model.DB.First(user, user.Id).Error)
			assert.Zero(t, user.Quota)

			trigger := map[string]any{"txid": " " + strings.Repeat("A", 64) + " ", "crypto": "USDT", "trigger": true}
			transactions := []map[string]any{trigger}
			status := "confirmed"
			expectedCode := http.StatusAccepted
			switch scenario {
			case "unconfirmed":
				status = "unconfirmed"
			case "missing-trigger":
				delete(trigger, "trigger")
				expectedCode = http.StatusBadRequest
			case "wrong-trigger-network":
				trigger["crypto"] = "BNB-USDT"
				expectedCode = http.StatusBadRequest
			case "invalid-trigger-id":
				trigger["txid"] = "bad"
				expectedCode = http.StatusBadRequest
			case "reconcile-db-failure":
				// Fail the real settlement transaction after the authorization commit.
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("shkeeper_credit_failure", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" {
						tx.AddError(assert.AnError)
					}
				}))
				t.Cleanup(func() { model.DB.Callback().Update().Remove("shkeeper_credit_failure") })
				expectedCode = http.StatusInternalServerError
			case "only-trigger-is-authorized":
				transactions = append(transactions, map[string]any{"txid": strings.Repeat("b", 64), "crypto": "USDT", "trigger": false})
			}
			w := signedSHKeeperCallback(t, router, status, transactions)
			assert.Equal(t, expectedCode, w.Code, w.Body.String())
			if scenario == "reconcile-db-failure" {
				require.NoError(t, model.DB.Callback().Update().Remove("shkeeper_credit_failure"))
			}
			summary, err := service.ReconcileSHKeeperOrders(context.Background(), 10)
			require.NoError(t, err)
			assert.Zero(t, summary.Failed)
			require.NoError(t, model.DB.First(user, user.Id).Error)
			if scenario == "threshold-and-replay" || scenario == "reconcile-db-failure" {
				assert.Equal(t, 6600, user.Quota)
				if scenario == "reconcile-db-failure" {
					assert.Equal(t, 1, summary.Credited)
				}
				replay := signedSHKeeperCallback(t, router, status, transactions)
				assert.Equal(t, http.StatusAccepted, replay.Code)
				require.NoError(t, model.DB.First(user, user.Id).Error)
				assert.Equal(t, 6600, user.Quota)
			} else {
				assert.Zero(t, user.Quota)
				if scenario != "only-trigger-is-authorized" {
					assert.Equal(t, 2, queries, "rejected/unconfirmed callback does not query or settle")
				}
			}
		})
	}
}
