package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SHKeeperOrderStatusPending    = "pending_provider"
	SHKeeperOrderStatusUnpaid     = "unpaid"
	SHKeeperOrderStatusPartial    = "partial"
	SHKeeperOrderStatusConfirming = "confirming"
	SHKeeperOrderStatusPaid       = "paid"
	SHKeeperOrderStatusOverpaid   = "overpaid"
	SHKeeperOrderStatusLate       = "late"
	SHKeeperOrderStatusFailed     = "failed"

	SHKeeperSettlementModeFixedPackage = "fixed_package"
)

type SHKeeperTopUpOrder struct {
	ID                int64  `json:"id" gorm:"primaryKey"`
	TopUpID           int    `json:"top_up_id" gorm:"index"`
	TradeNo           string `json:"trade_no" gorm:"size:255;uniqueIndex"`
	UserID            int    `json:"user_id" gorm:"index"`
	ExternalID        string `json:"external_id" gorm:"size:255;uniqueIndex"`
	Crypto            string `json:"crypto" gorm:"size:32;index"`
	SettlementMode    string `json:"settlement_mode" gorm:"size:32;index"`
	RequestedUSDT     string `json:"requested_usdt" gorm:"size:64"`
	PackageBalance    string `json:"package_balance" gorm:"size:64"`
	PackageQuota      int64  `json:"package_quota" gorm:"type:bigint"`
	RequestedBalance  string `json:"requested_balance" gorm:"size:64"`
	LockedRate        string `json:"locked_rate" gorm:"size:64"`
	QuotedUSDT        string `json:"quoted_usdt" gorm:"size:64"`
	InvoiceAddress    string `json:"invoice_address" gorm:"size:255"`
	CallbackURL       string `json:"-" gorm:"size:512"`
	ProviderInvoiceID string `json:"provider_invoice_id" gorm:"size:255"`
	ReceivedUSDT      string `json:"received_usdt" gorm:"size:64"`
	CreditedBalance   string `json:"credited_balance" gorm:"size:64"`
	CreditedQuota     int64  `json:"credited_quota" gorm:"type:bigint"`
	Status            string `json:"status" gorm:"size:32;index"`
	ProviderSummary   string `json:"-" gorm:"type:text"`
	CreatedAt         int64  `json:"created_at"`
	ExpiresAt         int64  `json:"expires_at"`
	CompletedAt       int64  `json:"completed_at"`
	LastReconciledAt  int64  `json:"last_reconciled_at" gorm:"index"`
}

type SHKeeperCreditedTransaction struct {
	ID                  int64  `json:"id" gorm:"primaryKey"`
	OrderID             int64  `json:"order_id" gorm:"uniqueIndex:idx_shkeeper_credit_tx,priority:3"`
	Crypto              string `json:"crypto" gorm:"size:32;uniqueIndex:idx_shkeeper_credit_tx,priority:1"`
	TxID                string `json:"tx_id" gorm:"size:255;uniqueIndex:idx_shkeeper_credit_tx,priority:2"`
	ConfirmedUSDT       string `json:"confirmed_usdt" gorm:"size:64"`
	BalanceContribution string `json:"balance_contribution" gorm:"size:64"`
	QuotaContribution   int64  `json:"quota_contribution" gorm:"type:bigint"`
	ProcessedAt         int64  `json:"processed_at"`
}

type SHKeeperSettlementTransaction struct {
	TxID       string
	AmountUSDT string
}

type SHKeeperSettlementInput struct {
	TradeNo         string
	UserID          int
	Crypto          string
	InvoiceAddress  string
	ProviderStatus  string
	ReceivedUSDT    string
	Transactions    []SHKeeperSettlementTransaction
	ProviderSummary string
	ReconciledAt    int64
}

type SHKeeperSettlementResult struct {
	OrderID              int64
	UserID               int
	TradeNo              string
	Status               string
	ReceivedUSDT         string
	CreditedBalance      string
	CreditedBalanceDelta string
	CreditedQuota        int64
	CreditedQuotaDelta   int64
	AuditMarker          string
}

type SHKeeperInvoiceDetails struct {
	QuotedUSDT        string
	InvoiceAddress    string
	ProviderInvoiceID string
	Status            string
	ExpiresAt         int64
}

func CreateSHKeeperTopUp(tx *gorm.DB, topUp *TopUp, order *SHKeeperTopUpOrder) error {
	if tx == nil || topUp == nil || order == nil {
		return errors.New("SHKeeper top-up transaction, top-up, and order are required")
	}
	if strings.TrimSpace(topUp.TradeNo) == "" || topUp.TradeNo != order.TradeNo || topUp.UserId != order.UserID {
		return errors.New("SHKeeper top-up identity mismatch")
	}
	if order.SettlementMode == SHKeeperSettlementModeFixedPackage {
		balance, err := decimal.NewFromString(order.PackageBalance)
		if err != nil {
			return errors.New("invalid SHKeeper package balance")
		}
		quota, err := operation_setting.SHKeeperPackageQuota(balance)
		if err != nil {
			return err
		}
		var user User
		if err := tx.Select("id", "quota").First(&user, order.UserID).Error; err != nil {
			return err
		}
		if int64(user.Quota) > operation_setting.SHKeeperMaxUserQuota-quota {
			return errors.New("SHKeeper package would exceed the user quota limit")
		}
		order.PackageQuota = quota
	}
	if err := tx.Create(topUp).Error; err != nil {
		return err
	}
	order.TopUpID = topUp.Id
	return tx.Create(order).Error
}

func InsertSHKeeperTopUp(topUp *TopUp, order *SHKeeperTopUpOrder) error {
	return DB.Transaction(func(tx *gorm.DB) error { return CreateSHKeeperTopUp(tx, topUp, order) })
}

func UpdateSHKeeperInvoiceDetails(tradeNo string, userID int, details SHKeeperInvoiceDetails) error {
	result := DB.Model(&SHKeeperTopUpOrder{}).
		Where("trade_no = ? AND user_id = ?", strings.TrimSpace(tradeNo), userID).
		Updates(map[string]any{
			"quoted_usdt":         strings.TrimSpace(details.QuotedUSDT),
			"invoice_address":     strings.TrimSpace(details.InvoiceAddress),
			"provider_invoice_id": strings.TrimSpace(details.ProviderInvoiceID),
			"status":              strings.TrimSpace(details.Status),
			"expires_at":          details.ExpiresAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrTopUpNotFound
	}
	return nil
}

func ListSHKeeperCreditedTransactions(orderID int64) ([]SHKeeperCreditedTransaction, error) {
	transactions := make([]SHKeeperCreditedTransaction, 0)
	err := DB.Where("order_id = ?", orderID).Order("id asc").Find(&transactions).Error
	return transactions, err
}

func RecordSHKeeperReconciliationAttempt(orderID int64, attemptedAt int64) error {
	return DB.Model(&SHKeeperTopUpOrder{}).Where("id = ? AND last_reconciled_at < ?", orderID, attemptedAt).
		Update("last_reconciled_at", attemptedAt).Error
}

func ListSHKeeperOrdersForReconciliation(now int64, limit int) ([]SHKeeperTopUpOrder, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	finalStatuses := []string{SHKeeperOrderStatusPaid, SHKeeperOrderStatusOverpaid, SHKeeperOrderStatusLate, SHKeeperOrderStatusFailed}
	orders := make([]SHKeeperTopUpOrder, 0)
	err := DB.Where("status NOT IN ? OR completed_at >= ?", finalStatuses, now-24*60*60).
		Order("last_reconciled_at asc, id asc").Limit(limit).Find(&orders).Error
	return orders, err
}

func GetSHKeeperTopUpByTradeNo(userID int, tradeNo string) (*SHKeeperTopUpOrder, error) {
	order := &SHKeeperTopUpOrder{}
	query := DB.Where("trade_no = ?", strings.TrimSpace(tradeNo))
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func SettleSHKeeperTopUp(input SHKeeperSettlementInput) (*SHKeeperSettlementResult, error) {
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	input.Crypto = strings.TrimSpace(input.Crypto)
	input.InvoiceAddress = strings.TrimSpace(input.InvoiceAddress)
	if input.TradeNo == "" || input.UserID <= 0 || input.Crypto == "" || input.InvoiceAddress == "" {
		return nil, errors.New("incomplete SHKeeper settlement identity")
	}
	if input.ReconciledAt <= 0 {
		input.ReconciledAt = common.GetTimestamp()
	}
	received, err := decimal.NewFromString(strings.TrimSpace(input.ReceivedUSDT))
	if err != nil || received.IsNegative() {
		return nil, errors.New("invalid SHKeeper received amount")
	}

	result := &SHKeeperSettlementResult{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		order := &SHKeeperTopUpOrder{}
		if err := lockForUpdate(tx).Where("trade_no = ?", input.TradeNo).First(order).Error; err != nil {
			return err
		}
		if order.UserID != input.UserID || order.Crypto != input.Crypto || order.InvoiceAddress != input.InvoiceAddress {
			return errors.New("SHKeeper settlement identity mismatch")
		}
		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", order.TopUpID).First(topUp).Error; err != nil {
			return err
		}
		if topUp.UserId != order.UserID || topUp.TradeNo != order.TradeNo ||
			topUp.PaymentProvider != PaymentProviderSHKeeper || topUp.PaymentMethod != PaymentMethodSHKeeper {
			return ErrPaymentMethodMismatch
		}

		previousReceived, err := decimal.NewFromString(order.ReceivedUSDT)
		if err != nil {
			return fmt.Errorf("invalid stored SHKeeper received amount: %w", err)
		}
		if received.LessThan(previousReceived) {
			return errors.New("SHKeeper received amount cannot decrease")
		}
		legacyRate := decimal.Zero
		if order.SettlementMode != SHKeeperSettlementModeFixedPackage {
			legacyRate, err = decimal.NewFromString(order.LockedRate)
			if err != nil || !legacyRate.IsPositive() {
				return errors.New("invalid stored SHKeeper rate")
			}
		}

		persistedTransactions := make([]SHKeeperCreditedTransaction, 0)
		if err := tx.Where("order_id = ?", order.ID).Find(&persistedTransactions).Error; err != nil {
			return err
		}
		evidence := make(map[string]decimal.Decimal, len(persistedTransactions)+len(input.Transactions))
		transactionTotal := decimal.Zero
		for _, persisted := range persistedTransactions {
			txID := strings.TrimSpace(persisted.TxID)
			amount, parseErr := decimal.NewFromString(persisted.ConfirmedUSDT)
			if txID == "" || parseErr != nil || amount.IsNegative() {
				return errors.New("invalid stored SHKeeper transaction")
			}
			if storedAmount, exists := evidence[txID]; exists {
				if !storedAmount.Equal(amount) {
					return errors.New("SHKeeper transaction ID has a different amount")
				}
				continue
			}
			evidence[txID] = amount
			transactionTotal = transactionTotal.Add(amount)
		}

		newTransactions := int64(0)
		for _, providerTransaction := range input.Transactions {
			txID := strings.TrimSpace(providerTransaction.TxID)
			amount, parseErr := decimal.NewFromString(strings.TrimSpace(providerTransaction.AmountUSDT))
			if txID == "" || parseErr != nil || amount.IsNegative() {
				return errors.New("invalid SHKeeper provider transaction")
			}
			if storedAmount, exists := evidence[txID]; exists {
				if !storedAmount.Equal(amount) {
					return errors.New("SHKeeper transaction ID has a different amount")
				}
				continue
			}
			evidence[txID] = amount
			transactionTotal = transactionTotal.Add(amount)
			contribution := decimal.Zero
			if order.SettlementMode != SHKeeperSettlementModeFixedPackage {
				contribution = amount.Mul(legacyRate).Round(1)
			}
			record := SHKeeperCreditedTransaction{
				OrderID: order.ID, Crypto: order.Crypto, TxID: txID, ConfirmedUSDT: amount.String(),
				BalanceContribution: contribution.String(),
				QuotaContribution:   contribution.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart(),
				ProcessedAt:         input.ReconciledAt,
			}
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
			if insert.Error != nil {
				return insert.Error
			}
			newTransactions += insert.RowsAffected
		}
		if !transactionTotal.Equal(received) {
			return errors.New("SHKeeper transaction total does not match received amount")
		}
		if received.GreaterThan(previousReceived) && newTransactions == 0 {
			return errors.New("increased SHKeeper balance requires a new confirmed transaction")
		}

		creditedBalance, err := decimal.NewFromString(order.CreditedBalance)
		if err != nil {
			return fmt.Errorf("invalid stored SHKeeper credited balance: %w", err)
		}
		entitlement := decimal.Zero
		settlementStatus := strings.TrimSpace(input.ProviderStatus)
		if order.SettlementMode == SHKeeperSettlementModeFixedPackage {
			requested, parseErr := decimal.NewFromString(order.RequestedUSDT)
			if parseErr != nil || !requested.IsPositive() || !requested.Equal(requested.Truncate(0)) {
				return errors.New("invalid stored SHKeeper package amount")
			}
			packageBalance, parseErr := decimal.NewFromString(order.PackageBalance)
			if parseErr != nil || !packageBalance.IsPositive() {
				return errors.New("invalid stored SHKeeper package balance")
			}
			if received.GreaterThanOrEqual(requested) {
				entitlement = packageBalance
				if order.CreditedQuota > 0 && (order.Status == SHKeeperOrderStatusPaid || order.Status == SHKeeperOrderStatusOverpaid || order.Status == SHKeeperOrderStatusLate) {
					settlementStatus = order.Status
				} else if order.ExpiresAt > 0 && input.ReconciledAt > order.ExpiresAt {
					settlementStatus = SHKeeperOrderStatusLate
				} else if received.GreaterThan(requested) {
					settlementStatus = SHKeeperOrderStatusOverpaid
				} else {
					settlementStatus = SHKeeperOrderStatusPaid
				}
			} else {
				settlementStatus = SHKeeperOrderStatusUnpaid
				if received.IsPositive() {
					settlementStatus = SHKeeperOrderStatusPartial
				}
				order.CompletedAt = 0
			}
		} else {
			entitlement = received.Mul(legacyRate).Round(1)
		}
		if entitlement.LessThan(creditedBalance) {
			return errors.New("SHKeeper entitlement cannot decrease")
		}
		balanceDelta := entitlement.Sub(creditedBalance)
		targetQuota := int64(0)
		if order.SettlementMode == SHKeeperSettlementModeFixedPackage {
			if order.CreditedQuota > 0 || creditedBalance.IsPositive() {
				if order.CreditedQuota <= 0 || !creditedBalance.Equal(entitlement) {
					return errors.New("SHKeeper credited entitlement requires ledger recovery")
				}
				targetQuota = order.CreditedQuota
				if order.PackageQuota == 0 {
					if topUp.Status != common.TopUpStatusSuccess {
						return errors.New("SHKeeper credited zero-snapshot order requires ledger recovery")
					}
					if targetQuota > operation_setting.SHKeeperMaxUserQuota {
						return errors.New("SHKeeper credited quota exceeds the supported quota limit")
					}
					order.PackageQuota = targetQuota
				}
			} else if entitlement.IsPositive() {
				if topUp.Status != common.TopUpStatusPending {
					return errors.New("SHKeeper order requires ledger recovery")
				}
				if order.PackageQuota <= 0 {
					return errors.New("SHKeeper package quota snapshot missing; operator recovery required")
				}
				targetQuota = order.PackageQuota
			}
		} else {
			quotaDelta := balanceDelta.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart()
			targetQuota = order.CreditedQuota + quotaDelta
		}
		if targetQuota < order.CreditedQuota {
			return errors.New("SHKeeper credited quota cannot decrease")
		}
		quotaDelta := targetQuota - order.CreditedQuota
		if quotaDelta > 0 {
			query := tx.Model(&User{}).Where("id = ?", order.UserID)
			if order.SettlementMode == SHKeeperSettlementModeFixedPackage {
				query = query.Where("quota <= ?", operation_setting.SHKeeperMaxUserQuota-quotaDelta)
			}
			update := query.Update("quota", gorm.Expr("quota + ?", quotaDelta))
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errors.New("SHKeeper user not found or user quota limit exceeded")
			}
			if topUp.Status == common.TopUpStatusPending {
				topUp.Status = common.TopUpStatusSuccess
				topUp.CompleteTime = input.ReconciledAt
			}
			if err := tx.Save(topUp).Error; err != nil {
				return err
			}
		}

		order.ReceivedUSDT = received.String()
		order.CreditedBalance = entitlement.String()
		order.CreditedQuota = targetQuota
		order.Status = settlementStatus
		order.ProviderSummary = input.ProviderSummary
		order.LastReconciledAt = input.ReconciledAt
		if order.CompletedAt == 0 && (settlementStatus == SHKeeperOrderStatusPaid || settlementStatus == SHKeeperOrderStatusOverpaid || settlementStatus == SHKeeperOrderStatusLate) {
			order.CompletedAt = input.ReconciledAt
		}
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		*result = SHKeeperSettlementResult{
			OrderID:              order.ID,
			UserID:               order.UserID,
			TradeNo:              order.TradeNo,
			Status:               order.Status,
			ReceivedUSDT:         order.ReceivedUSDT,
			CreditedBalance:      order.CreditedBalance,
			CreditedBalanceDelta: balanceDelta.String(),
			CreditedQuota:        order.CreditedQuota,
			CreditedQuotaDelta:   quotaDelta,
			AuditMarker:          fmt.Sprintf("shkeeper:%s:%d", order.TradeNo, order.CreditedQuota),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
