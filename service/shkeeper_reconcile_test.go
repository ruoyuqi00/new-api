package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSHKeeperReconcile(t *testing.T) (*model.User, *model.SHKeeperTopUpOrder, *SHKeeperInvoiceLookup) {
	t.Helper()
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SHKeeperTopUpOrder{}, &model.SHKeeperCreditedTransaction{}))
	t.Cleanup(func() {
		model.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SHKeeperCreditedTransaction{})
		model.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SHKeeperTopUpOrder{})
	})
	oldQuota := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = oldQuota })
	user := &model.User{Username: "shkeeper-reconcile", AffCode: "shkeeper-reconcile", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	order := &model.SHKeeperTopUpOrder{TradeNo: "USDT1abc", ExternalID: "USDT1abc", UserID: user.Id, Crypto: "USDT", SettlementMode: model.SHKeeperSettlementModeFixedPackage, RequestedUSDT: "10", PackageBalance: "66", InvoiceAddress: "TAddress", ReceivedUSDT: "0", CreditedBalance: "0", Status: model.SHKeeperOrderStatusUnpaid, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, model.InsertSHKeeperTopUp(&model.TopUp{TradeNo: order.TradeNo, UserId: user.Id, PaymentMethod: model.PaymentMethodSHKeeper, PaymentProvider: model.PaymentProviderSHKeeper, Status: common.TopUpStatusPending}, order))
	lookup := &SHKeeperInvoiceLookup{ExternalID: order.ExternalID, Fiat: "USD", AmountFiat: "10", BalanceFiat: "10", Status: "PAID", Transactions: []SHKeeperInvoiceTransaction{{Crypto: "USDT", Address: "TAddress", AmountUSDT: "10", TxID: strings.Repeat("a", 64), Status: "CONFIRMED"}}}
	return user, order, lookup
}

func serveSHKeeperLookup(t *testing.T, lookup *SHKeeperInvoiceLookup, fail bool) *int {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, http.MethodGet, r.Method)
		assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/invoices/"))
		assert.Equal(t, "secret", r.Header.Get("X-Shkeeper-Api-Key"))
		if fail {
			w.WriteHeader(503)
			return
		}
		data, err := common.Marshal(map[string]any{"status": "success", "invoices": []*SHKeeperInvoiceLookup{lookup}})
		require.NoError(t, err)
		w.Write(data)
	}))
	t.Cleanup(server.Close)
	settings := operation_setting.GetSHKeeperPaymentSetting()
	old := *settings
	t.Cleanup(func() { *settings = old })
	*settings = operation_setting.SHKeeperPaymentSetting{BaseURL: server.URL, APIKey: "secret", AllowPrivateURL: true}
	return &requests
}

func TestSHKeeperReconcileThresholdReplayAndAudit(t *testing.T) {
	user, order, lookup := setupSHKeeperReconcile(t)
	serveSHKeeperLookup(t, lookup, false)
	lookup.Transactions[0].AmountUSDT = "9"
	lookup.BalanceFiat = "9"
	lookup.Status = "PARTIAL"
	result, err := ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
	require.NoError(t, err)
	assert.Zero(t, result.CreditedQuotaDelta)
	assert.Equal(t, "partial", result.Status)
	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
	lookup.Transactions = append(lookup.Transactions, SHKeeperInvoiceTransaction{Crypto: "USDT", Address: "TAddress", AmountUSDT: "1", TxID: strings.Repeat("b", 64), Status: "CONFIRMED"})
	lookup.BalanceFiat = "10"
	lookup.Status = "PAID"
	duplicate := lookup.Transactions[0]
	duplicate.TxID = " " + strings.ToUpper(duplicate.TxID) + " "
	duplicate.AmountUSDT = "9.00"
	lookup.Transactions = append(lookup.Transactions, duplicate)
	result, err = ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
	require.NoError(t, err)
	assert.EqualValues(t, 6600, result.CreditedQuotaDelta)
	result, err = ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
	require.NoError(t, err)
	assert.Zero(t, result.CreditedQuotaDelta)
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Equal(t, 6600, user.Quota)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestSHKeeperReconcileFullLateAndOverpaid(t *testing.T) {
	for _, scenario := range []string{"full", "late", "overpaid"} {
		t.Run(scenario, func(t *testing.T) {
			user, order, lookup := setupSHKeeperReconcile(t)
			serveSHKeeperLookup(t, lookup, false)
			expected := "paid"
			if scenario == "late" {
				expected = "late"
				require.NoError(t, model.DB.Model(order).Update("expires_at", time.Now().Add(-time.Hour).Unix()).Error)
			}
			if scenario == "overpaid" {
				expected = "overpaid"
				lookup.Transactions[0].AmountUSDT = "12"
				lookup.BalanceFiat = "12"
			}
			result, err := ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
			require.NoError(t, err)
			assert.Equal(t, expected, result.Status)
			assert.EqualValues(t, 6600, result.CreditedQuotaDelta)
			_, err = ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
			require.NoError(t, err)
			require.NoError(t, model.DB.First(user, user.Id).Error)
			assert.Equal(t, 6600, user.Quota)
		})
	}
}

func TestSHKeeperReconcileRejectsMismatchedEvidence(t *testing.T) {
	for _, scenario := range []string{"external", "fiat", "amount", "address", "network", "conflict", "addressless"} {
		t.Run(scenario, func(t *testing.T) {
			user, order, lookup := setupSHKeeperReconcile(t)
			requests := serveSHKeeperLookup(t, lookup, false)
			switch scenario {
			case "external":
				lookup.ExternalID = "other"
			case "fiat":
				lookup.Fiat = "EUR"
			case "amount":
				lookup.AmountFiat = "11"
			case "address":
				lookup.Transactions[0].Address = "other"
			case "network":
				lookup.Transactions[0].Crypto = "BNB-USDT"
			case "conflict":
				duplicate := lookup.Transactions[0]
				duplicate.AmountUSDT = "11"
				lookup.Transactions = append(lookup.Transactions, duplicate)
			case "addressless":
				require.NoError(t, model.DB.Model(order).Update("invoice_address", "").Error)
			}
			_, err := ReconcileSHKeeperOrder(context.Background(), order.TradeNo)
			require.Error(t, err)
			require.NoError(t, model.DB.First(user, user.Id).Error)
			assert.Zero(t, user.Quota)
			require.NoError(t, model.DB.First(order, order.ID).Error)
			assert.Positive(t, order.LastReconciledAt)
			assert.LessOrEqual(t, *requests, 1)
		})
	}
}

func TestSHKeeperReconcileFailedBatchRotates(t *testing.T) {
	_, order, lookup := setupSHKeeperReconcile(t)
	serveSHKeeperLookup(t, lookup, true)
	other := *order
	other.ID = 0
	other.TradeNo = "USDT2abc"
	other.ExternalID = other.TradeNo
	require.NoError(t, model.DB.Create(&other).Error)
	first, err := ReconcileSHKeeperOrders(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, first.Processed)
	assert.Equal(t, 1, first.Failed)
	require.NoError(t, model.DB.First(order, order.ID).Error)
	assert.Positive(t, order.LastReconciledAt)
	second, err := ReconcileSHKeeperOrders(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, second.Failed)
	require.NoError(t, model.DB.First(&other, other.ID).Error)
	assert.Positive(t, other.LastReconciledAt, fmt.Sprint(second))
}
