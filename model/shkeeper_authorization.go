package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

// SHKeeperTransactionAuthorization records confirmation-threshold evidence from
// a verified signed trigger callback. Lookup transaction status alone is not
// threshold evidence in SHKeeper v2.5.32.
type SHKeeperTransactionAuthorization struct {
	ID           int64  `gorm:"primaryKey"`
	OrderID      int64  `gorm:"uniqueIndex:idx_shkeeper_authorized_tx,priority:1"`
	Crypto       string `gorm:"size:32;uniqueIndex:idx_shkeeper_authorized_tx,priority:2"`
	TxID         string `gorm:"size:255;uniqueIndex:idx_shkeeper_authorized_tx,priority:3"`
	AuthorizedAt int64
}

// AuthorizeSHKeeperTransactions must only receive normalized IDs from an
// authenticated callback with matching order/network/address identity. Commit
// separately from reconciliation so a failed lookup/settlement remains retryable.
func AuthorizeSHKeeperTransactions(orderID int64, crypto string, transactionIDs []string) error {
	if orderID <= 0 || strings.TrimSpace(crypto) == "" || len(transactionIDs) == 0 {
		return errors.New("incomplete SHKeeper transaction authorization")
	}
	rows := make([]SHKeeperTransactionAuthorization, 0, len(transactionIDs))
	for _, id := range transactionIDs {
		if strings.TrimSpace(id) == "" {
			return errors.New("empty SHKeeper transaction authorization")
		}
		rows = append(rows, SHKeeperTransactionAuthorization{OrderID: orderID, Crypto: crypto, TxID: id, AuthorizedAt: common.GetTimestamp()})
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func ListSHKeeperAuthorizedTransactionIDs(orderID int64, crypto string) ([]string, error) {
	var ids []string
	if err := DB.Model(&SHKeeperTransactionAuthorization{}).Where("order_id = ? AND crypto = ?", orderID, crypto).Pluck("tx_id", &ids).Error; err != nil {
		return nil, err
	}
	// Already credited evidence predates this additive authorization ledger and
	// remains valid on upgrade. It never authorizes a different transaction.
	var credited []string
	if err := DB.Model(&SHKeeperCreditedTransaction{}).Where("order_id = ? AND crypto = ?", orderID, crypto).Pluck("tx_id", &credited).Error; err != nil {
		return nil, err
	}
	return append(ids, credited...), nil
}
