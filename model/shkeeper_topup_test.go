package model

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFixedSHKeeperOrder(t *testing.T, requestedUSDT string, balance string, quota int64) (*User, *SHKeeperTopUpOrder) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&SHKeeperTopUpOrder{}, &SHKeeperCreditedTransaction{}, &Option{}, &AffiliateReward{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SHKeeperCreditedTransaction{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SHKeeperTopUpOrder{}).Error)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100

	suffix := time.Now().UnixNano()
	user := &User{
		Username: fmt.Sprintf("shkeeper-%d", suffix), AffCode: fmt.Sprintf("shk-%d", suffix),
		Status: common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
	topUp := &TopUp{
		UserId: user.Id, Amount: 10, TradeNo: fmt.Sprintf("shkeeper-%d", suffix),
		PaymentMethod: "shkeeper", PaymentProvider: PaymentProviderSHKeeper,
		CreateTime: time.Now().Unix(), Status: common.TopUpStatusPending,
	}
	order := &SHKeeperTopUpOrder{
		TradeNo: topUp.TradeNo, UserID: user.Id,
		ExternalID: fmt.Sprintf("external-%d", suffix), Crypto: "USDT",
		SettlementMode: SHKeeperSettlementModeFixedPackage,
		RequestedUSDT:  requestedUSDT, PackageBalance: balance, LockedRate: "6.6",
		InvoiceAddress: "TAddress", ReceivedUSDT: "0", CreditedBalance: "0",
		Status: SHKeeperOrderStatusPending, CreatedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error { return CreateSHKeeperTopUp(tx, topUp, order) }))
	assert.EqualValues(t, quota, order.PackageQuota)
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SHKeeperCreditedTransaction{})
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SHKeeperTopUpOrder{})
		DB.Delete(&TopUp{}, topUp.Id)
		DB.Unscoped().Delete(&User{}, user.Id)
	})
	return user, order
}

func settleSHKeeper(t *testing.T, order *SHKeeperTopUpOrder, user *User, received, status, txID string) *SHKeeperSettlementResult {
	t.Helper()
	result, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: status, ReceivedUSDT: received,
		Transactions: []SHKeeperSettlementTransaction{{TxID: txID, AmountUSDT: received}},
		ReconciledAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	require.NoError(t, DB.First(user, user.Id).Error)
	return result
}

func TestSettleSHKeeperFixedPackageCreditsOnlyOnceAtThreshold(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	partial := settleSHKeeper(t, order, user, "9", SHKeeperOrderStatusPartial, "tx-1")
	assert.Zero(t, partial.CreditedQuotaDelta)
	assert.Equal(t, "0", partial.CreditedBalance)

	paid, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{
			{TxID: "tx-1", AmountUSDT: "9"}, {TxID: "tx-2", AmountUSDT: "1"},
		}, ReconciledAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.EqualValues(t, 6600, paid.CreditedQuotaDelta)
	assert.Equal(t, 6600, user.Quota)

	original := common.QuotaPerUnit
	common.QuotaPerUnit = 200
	t.Cleanup(func() { common.QuotaPerUnit = original })
	replayed, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusOverpaid, ReceivedUSDT: "12",
		Transactions: []SHKeeperSettlementTransaction{
			{TxID: "tx-1", AmountUSDT: "9"}, {TxID: "tx-2", AmountUSDT: "1"}, {TxID: "tx-3", AmountUSDT: "2"},
		}, ReconciledAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Zero(t, replayed.CreditedQuotaDelta)
	assert.EqualValues(t, 6600, replayed.CreditedQuota)
	assert.Equal(t, 6600, user.Quota)
}

func TestManualCompleteTopUpRejectsSHKeeper(t *testing.T) {
	_, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	err := ManualCompleteTopUp(order.TradeNo, "127.0.0.1")
	require.ErrorContains(t, err, "SHKeeper")
}

func TestSHKeeperFixedPackageQuotaSnapshotCapacity(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(user).Update("quota", int(operation_setting.SHKeeperMaxUserQuota-6600)).Error)
	result := settleSHKeeper(t, order, user, "10", SHKeeperOrderStatusPaid, "capacity-ok")
	assert.EqualValues(t, operation_setting.SHKeeperMaxUserQuota, user.Quota)
	assert.EqualValues(t, 6600, result.CreditedQuotaDelta)

	user, order = setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(user).Update("quota", int(operation_setting.SHKeeperMaxUserQuota-6599)).Error)
	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "capacity-over", AmountUSDT: "10"}},
	})
	require.ErrorContains(t, err, "quota limit")
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.EqualValues(t, operation_setting.SHKeeperMaxUserQuota-6599, user.Quota)
}

func TestSettleSHKeeperRejectsConflictingTransactionAmount(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	settleSHKeeper(t, order, user, "9", SHKeeperOrderStatusPartial, " tx-1 ")
	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "tx-1", AmountUSDT: "10"}},
	})
	require.ErrorContains(t, err, "different amount")
}

func TestSettleSHKeeperDeduplicatesNormalizedTransactionIDs(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	result, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{
			{TxID: "tx-duplicate", AmountUSDT: "10.0"},
			{TxID: " tx-duplicate ", AmountUSDT: "10.000"},
		}, ReconciledAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	assert.EqualValues(t, 6600, result.CreditedQuotaDelta)
	transactions, err := ListSHKeeperCreditedTransactions(order.ID)
	require.NoError(t, err)
	assert.Len(t, transactions, 1)
}

func TestSettleSHKeeperRejectsReceivedTotalThatDoesNotMatchTransactions(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{
			{TxID: "tx-a", AmountUSDT: "4"}, {TxID: "tx-b", AmountUSDT: "5"},
		}, ReconciledAt: time.Now().Unix(),
	})
	require.ErrorContains(t, err, "transaction total")
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
}

func TestSettleSHKeeperIncludesPersistedEvidenceInTransactionTotal(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	settleSHKeeper(t, order, user, "9", SHKeeperOrderStatusPartial, "persisted")

	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "submitted", AmountUSDT: "10"}},
		ReconciledAt: time.Now().Unix(),
	})
	require.ErrorContains(t, err, "transaction total")
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Zero(t, user.Quota)
	stored, err := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, "9", stored.ReceivedUSDT)
	transactions, err := ListSHKeeperCreditedTransactions(order.ID)
	require.NoError(t, err)
	require.Len(t, transactions, 1)
	assert.Equal(t, "persisted", transactions[0].TxID)
}

func TestSettleSHKeeperConcurrentReplayCreditsOnce(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	input := SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "concurrent", AmountUSDT: "10"}},
		ReconciledAt: time.Now().Unix(),
	}
	results := make([]*SHKeeperSettlementResult, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for index := range results {
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = SettleSHKeeperTopUp(input)
		}(index)
	}
	wait.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.EqualValues(t, 6600, results[0].CreditedQuotaDelta+results[1].CreditedQuotaDelta)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 6600, user.Quota)
}

func TestSettleSHKeeperPreservesFirstCompletion(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	first := time.Now().Unix()
	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "first", AmountUSDT: "10"}}, ReconciledAt: first,
	})
	require.NoError(t, err)
	_, err = SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusOverpaid, ReceivedUSDT: "11",
		Transactions: []SHKeeperSettlementTransaction{
			{TxID: "first", AmountUSDT: "10"}, {TxID: "later", AmountUSDT: "1"},
		}, ReconciledAt: first + 60,
	})
	require.NoError(t, err)
	stored, err := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, first, stored.CompletedAt)
	assert.Equal(t, SHKeeperOrderStatusPaid, stored.Status)
}

func TestSettleSHKeeperUnderfundedExpiredOrderRemainsOpen(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(order).Updates(map[string]any{
		"expires_at": time.Now().Add(-time.Hour).Unix(), "status": SHKeeperOrderStatusLate, "completed_at": time.Now().Unix(),
	}).Error)
	result := settleSHKeeper(t, order, user, "9", SHKeeperOrderStatusLate, "underfunded")
	assert.Equal(t, SHKeeperOrderStatusPartial, result.Status)
	stored, err := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.Zero(t, stored.CompletedAt)
	assert.Equal(t, common.TopUpStatusPending, GetTopUpByTradeNo(order.TradeNo).Status)
}

func TestSettleSHKeeperPreSnapshotOrdersFailClosedOrRepair(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(order).Update("package_quota", 0).Error)
	_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
		ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
		Transactions: []SHKeeperSettlementTransaction{{TxID: "missing-snapshot", AmountUSDT: "10"}},
	})
	require.ErrorContains(t, err, "snapshot missing")

	user, order = setupFixedSHKeeperOrder(t, "10", "66", 6600)
	settleSHKeeper(t, order, user, "10", SHKeeperOrderStatusPaid, "credited")
	require.NoError(t, DB.Model(order).Update("package_quota", 0).Error)
	replayed := settleSHKeeper(t, order, user, "10", SHKeeperOrderStatusPaid, "credited")
	assert.Zero(t, replayed.CreditedQuotaDelta)
	stored, err := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.EqualValues(t, 6600, stored.PackageQuota)
}

func TestSettleSHKeeperRejectsIncoherentCreditedZeroSnapshot(t *testing.T) {
	tests := []struct {
		name          string
		topUpStatus   string
		creditedQuota int64
		wantError     string
	}{
		{name: "pending generic top-up", topUpStatus: common.TopUpStatusPending, creditedQuota: 6600, wantError: "ledger recovery"},
		{name: "failed generic top-up", topUpStatus: common.TopUpStatusFailed, creditedQuota: 6600, wantError: "ledger recovery"},
		{name: "credited quota above ceiling", topUpStatus: common.TopUpStatusSuccess, creditedQuota: operation_setting.SHKeeperMaxUserQuota + 1, wantError: "quota limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
			require.NoError(t, DB.Model(&TopUp{}).Where("id = ?", order.TopUpID).Update("status", test.topUpStatus).Error)
			require.NoError(t, DB.Model(order).Updates(map[string]any{
				"package_quota": 0, "received_usdt": "10", "credited_balance": "66",
				"credited_quota": test.creditedQuota, "status": SHKeeperOrderStatusPaid,
			}).Error)
			originalQuota := user.Quota

			_, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
				TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress,
				ProviderStatus: SHKeeperOrderStatusPaid, ReceivedUSDT: "10",
				Transactions: []SHKeeperSettlementTransaction{{TxID: "historical", AmountUSDT: "10"}},
				ReconciledAt: time.Now().Unix(),
			})
			require.ErrorContains(t, err, test.wantError)
			stored, loadErr := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
			require.NoError(t, loadErr)
			assert.Zero(t, stored.PackageQuota)
			assert.Equal(t, test.creditedQuota, stored.CreditedQuota)
			require.NoError(t, DB.First(user, user.Id).Error)
			assert.Equal(t, originalQuota, user.Quota)
			transactions, listErr := ListSHKeeperCreditedTransactions(order.ID)
			require.NoError(t, listErr)
			assert.Empty(t, transactions)
		})
	}
}

func TestSettleSHKeeperEmptyModeUsesLegacyRate(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(order).Update("settlement_mode", "").Error)
	result := settleSHKeeper(t, order, user, "1", SHKeeperOrderStatusPartial, "legacy")
	assert.Equal(t, "6.6", result.CreditedBalanceDelta)
	assert.EqualValues(t, 660, result.CreditedQuotaDelta)
	assert.Equal(t, 660, user.Quota)
}

func TestSettleSHKeeperLegacyReplayPreservesHistoricalQuotaConversion(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	require.NoError(t, DB.Model(order).Update("settlement_mode", "").Error)
	first := settleSHKeeper(t, order, user, "1", SHKeeperOrderStatusPartial, "legacy-replay")
	require.EqualValues(t, 660, first.CreditedQuotaDelta)

	common.QuotaPerUnit = 200
	replayed := settleSHKeeper(t, order, user, "1", SHKeeperOrderStatusPartial, "legacy-replay")
	assert.Zero(t, replayed.CreditedQuotaDelta)
	assert.EqualValues(t, 660, replayed.CreditedQuota)
	assert.Equal(t, 660, user.Quota)
	stored, err := GetSHKeeperTopUpByTradeNo(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.EqualValues(t, 660, stored.CreditedQuota)
}

func TestSHKeeperPackageBalanceMatchesQuotaSnapshot(t *testing.T) {
	_, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	assert.True(t, decimal.RequireFromString(order.PackageBalance).Equal(decimal.NewFromInt(66)))
}
