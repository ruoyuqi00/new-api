package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHKeeperReconciliationQueriesAgeOrders(t *testing.T) {
	_, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	now := time.Now().Unix()
	order.Status = SHKeeperOrderStatusPaid
	order.CompletedAt = now - 60
	order.LastReconciledAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, DB.Save(order).Error)
	require.NoError(t, DB.Create(&[]SHKeeperTopUpOrder{
		{TradeNo: "shkeeper-open", ExternalID: "shkeeper-open", Status: SHKeeperOrderStatusPartial},
		{TradeNo: "shkeeper-old-final", ExternalID: "shkeeper-old-final", Status: SHKeeperOrderStatusPaid, CompletedAt: now - 90_000},
	}).Error)

	due, err := ListSHKeeperOrdersForReconciliation(now, 10)
	require.NoError(t, err)
	tradeNos := make([]string, 0, len(due))
	for _, candidate := range due {
		tradeNos = append(tradeNos, candidate.TradeNo)
	}
	assert.ElementsMatch(t, []string{order.TradeNo, "shkeeper-open"}, tradeNos)
}

func TestSHKeeperOlderReconciliationCannotMoveAttemptTimestampBackward(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	const olderAttempt int64 = 1800000000
	const newerAttempt int64 = olderAttempt + 60
	require.NoError(t, RecordSHKeeperReconciliationAttempt(order.ID, olderAttempt))
	require.NoError(t, RecordSHKeeperReconciliationAttempt(order.ID, newerAttempt))
	// Complete the older provider lookup after a newer attempt has already been
	// recorded, without timing or concurrency assumptions in the fixture.
	result, err := SettleSHKeeperTopUp(SHKeeperSettlementInput{
		TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto,
		InvoiceAddress: order.InvoiceAddress, ProviderStatus: SHKeeperOrderStatusPaid,
		ReceivedUSDT: "10", ReconciledAt: olderAttempt,
		Transactions: []SHKeeperSettlementTransaction{{TxID: "confirmed-payment", AmountUSDT: "10"}},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 6600, result.CreditedQuotaDelta)
	require.NoError(t, DB.First(order, order.ID).Error)
	assert.Equal(t, newerAttempt, order.LastReconciledAt)
	assert.Equal(t, olderAttempt, order.CompletedAt, "funding time stays separate from scheduling freshness")
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 6600, user.Quota)
}
