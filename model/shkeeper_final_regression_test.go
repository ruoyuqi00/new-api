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
