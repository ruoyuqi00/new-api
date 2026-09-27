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
	TokenPayOrderStatusPending = "pending_provider"
	TokenPayOrderStatusUnpaid  = "unpaid"
	TokenPayOrderStatusPaid    = "paid"
	TokenPayOrderStatusFailed  = "failed"
)

type TokenPayTopUpOrder struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	TopUpID            int     `json:"top_up_id" gorm:"uniqueIndex"`
	TradeNo            string  `json:"trade_no" gorm:"size:255;uniqueIndex"`
	UserID             int     `json:"user_id" gorm:"index"`
	Network            string  `json:"network" gorm:"size:64;index"`
	RequestedUSDT      string  `json:"requested_usdt" gorm:"size:64"`
	PackageBalance     string  `json:"package_balance" gorm:"size:64"`
	PackageQuota       int64   `json:"package_quota" gorm:"type:bigint"`
	ProviderOrderID    *string `json:"provider_order_id" gorm:"size:128;uniqueIndex"`
	OrderUserKey       string  `json:"order_user_key" gorm:"size:255;uniqueIndex"`
	ProviderAmountUSDT string  `json:"provider_amount_usdt" gorm:"size:64"`
	ReceiveAddress     string  `json:"receive_address" gorm:"size:255"`
	PaymentURL         string  `json:"payment_url" gorm:"size:1024"`
	RecoveryHash       string  `json:"recovery_hash" gorm:"size:255"`
	Status             string  `json:"status" gorm:"size:32;index"`
	CreatedAt          int64   `json:"created_at"`
	ExpiresAt          int64   `json:"expires_at"`
	CompletedAt        int64   `json:"completed_at"`
}

type TokenPayCreditedTransaction struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	OrderID     int64  `json:"order_id" gorm:"uniqueIndex"`
	Network     string `json:"network" gorm:"size:64;uniqueIndex:idx_tokenpay_chain_tx,priority:1"`
	TxID        string `json:"tx_id" gorm:"size:255;uniqueIndex:idx_tokenpay_chain_tx,priority:2"`
	AmountUSDT  string `json:"amount_usdt" gorm:"size:64"`
	ProcessedAt int64  `json:"processed_at"`
}

type TokenPaySettlementInput struct {
	TradeNo         string
	ProviderOrderID string
	Network         string
	ReceiveAddress  string
	ReceivedUSDT    string
	TransactionID   string
	CallerIP        string
}

type TokenPaySettlementResult struct {
	OrderID            int64
	UserID             int
	TradeNo            string
	CreditedQuotaDelta int64
	CreditedQuota      int64
}

func CreateTokenPayTopUp(topUp *TopUp, order *TokenPayTopUpOrder) error {
	return DB.Transaction(func(tx *gorm.DB) error { return CreateTokenPayTopUpTx(tx, topUp, order) })
}

func CreateTokenPayTopUpTx(tx *gorm.DB, topUp *TopUp, order *TokenPayTopUpOrder) error {
	if tx == nil || topUp == nil || order == nil || strings.TrimSpace(topUp.TradeNo) == "" ||
		topUp.TradeNo != order.TradeNo || topUp.UserId != order.UserID || topUp.PaymentProvider != PaymentProviderTokenPay ||
		topUp.PaymentMethod != PaymentMethodTokenPay || topUp.Status != common.TopUpStatusPending {
		return errors.New("TokenPay top-up identity mismatch")
	}
	requested, err := decimal.NewFromString(strings.TrimSpace(order.RequestedUSDT))
	if err != nil || !requested.IsPositive() || !requested.Equal(requested.Truncate(0)) {
		return errors.New("invalid TokenPay requested USDT")
	}
	balance, err := decimal.NewFromString(strings.TrimSpace(order.PackageBalance))
	if err != nil {
		return errors.New("invalid TokenPay package balance")
	}
	quota, err := operation_setting.TokenPayPackageQuota(balance)
	if err != nil {
		return err
	}
	var user User
	if err := tx.Select("id", "quota").First(&user, order.UserID).Error; err != nil {
		return err
	}
	if int64(user.Quota) > operation_setting.TokenPayMaxUserQuota-quota {
		return errors.New("TokenPay package would exceed the user quota limit")
	}
	order.RequestedUSDT = requested.String()
	order.PackageBalance = balance.String()
	order.PackageQuota = quota
	if err := tx.Create(topUp).Error; err != nil {
		return err
	}
	order.TopUpID = topUp.Id
	return tx.Create(order).Error
}

func SaveTokenPayInvoice(tradeNo string, userID int, providerID, providerAmount, address, paymentURL string, expiresAt int64) error {
	amount, err := decimal.NewFromString(providerAmount)
	if err != nil || !amount.IsPositive() || strings.TrimSpace(providerID) == "" || strings.TrimSpace(address) == "" || strings.TrimSpace(paymentURL) == "" || expiresAt <= 0 {
		return errors.New("invalid TokenPay invoice")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order TokenPayTopUpOrder
		if err := lockForUpdate(tx).Where("trade_no = ? AND user_id = ?", tradeNo, userID).First(&order).Error; err != nil {
			return err
		}
		requested, err := decimal.NewFromString(order.RequestedUSDT)
		if err != nil || !amount.Equal(requested) || order.ProviderOrderID != nil || order.Status != TokenPayOrderStatusPending {
			return errors.New("TokenPay invoice cannot replace the pending fixed package")
		}
		order.ProviderOrderID = &providerID
		order.ProviderAmountUSDT = amount.String()
		order.ReceiveAddress = strings.TrimSpace(address)
		order.PaymentURL = paymentURL
		order.ExpiresAt = expiresAt
		order.Status = TokenPayOrderStatusUnpaid
		return tx.Save(&order).Error
	})
}

func GetTokenPayOrder(userID int, tradeNo string) (*TokenPayTopUpOrder, error) {
	var order TokenPayTopUpOrder
	query := DB.Where("trade_no = ?", strings.TrimSpace(tradeNo))
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func SettleTokenPayTopUp(input TokenPaySettlementInput) (*TokenPaySettlementResult, error) {
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	input.ProviderOrderID = strings.TrimSpace(input.ProviderOrderID)
	input.Network = strings.TrimSpace(input.Network)
	input.ReceiveAddress = strings.TrimSpace(input.ReceiveAddress)
	input.TransactionID = strings.ToLower(strings.TrimSpace(input.TransactionID))
	if input.TradeNo == "" || input.ProviderOrderID == "" || input.TransactionID == "" || input.ReceiveAddress == "" {
		return nil, errors.New("incomplete TokenPay settlement identity")
	}
	received, err := decimal.NewFromString(strings.TrimSpace(input.ReceivedUSDT))
	if err != nil || !received.IsPositive() {
		return nil, errors.New("invalid TokenPay received amount")
	}
	result := &TokenPaySettlementResult{}
	var reward *AffiliateReward
	err = DB.Transaction(func(tx *gorm.DB) error {
		var order TokenPayTopUpOrder
		if err := lockForUpdate(tx).Where("trade_no = ?", input.TradeNo).First(&order).Error; err != nil {
			return ErrTopUpNotFound
		}
		if order.ProviderOrderID == nil || *order.ProviderOrderID != input.ProviderOrderID || order.Network != input.Network ||
			!strings.EqualFold(order.ReceiveAddress, input.ReceiveAddress) {
			return errors.New("TokenPay settlement identity mismatch")
		}
		requested, parseErr := decimal.NewFromString(order.RequestedUSDT)
		quoted, quoteErr := decimal.NewFromString(order.ProviderAmountUSDT)
		if parseErr != nil || quoteErr != nil || !received.Equal(requested) || !quoted.Equal(requested) {
			return errors.New("TokenPay settlement amount mismatch")
		}
		var topUp TopUp
		if err := lockForUpdate(tx).Where("id = ?", order.TopUpID).First(&topUp).Error; err != nil {
			return err
		}
		if topUp.TradeNo != order.TradeNo || topUp.UserId != order.UserID || topUp.PaymentProvider != PaymentProviderTokenPay ||
			topUp.PaymentMethod != PaymentMethodTokenPay {
			return ErrPaymentMethodMismatch
		}
		*result = TokenPaySettlementResult{OrderID: order.ID, UserID: order.UserID, TradeNo: order.TradeNo, CreditedQuota: order.PackageQuota}
		if order.Status == TokenPayOrderStatusPaid && topUp.Status == common.TopUpStatusSuccess {
			var existing TokenPayCreditedTransaction
			if err := tx.Where("order_id = ? AND network = ? AND tx_id = ?", order.ID, order.Network, input.TransactionID).First(&existing).Error; err != nil {
				return errors.New("TokenPay paid order has no matching transaction")
			}
			return nil
		}
		if order.Status != TokenPayOrderStatusUnpaid || topUp.Status != common.TopUpStatusPending || order.PackageQuota <= 0 {
			return ErrTopUpStatusInvalid
		}
		ledger := TokenPayCreditedTransaction{OrderID: order.ID, Network: order.Network, TxID: input.TransactionID,
			AmountUSDT: received.String(), ProcessedAt: common.GetTimestamp()}
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected != 1 {
			return errors.New("TokenPay transaction already belongs to an order")
		}
		reward, err = creditUserQuotaWithAffiliateRewardTx(tx, order.UserID, int(order.PackageQuota), AffiliateRewardSourceTopUp, order.TradeNo,
			affiliateCreditOptions{maxQuota: operation_setting.TokenPayMaxUserQuota})
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		order.Status = TokenPayOrderStatusPaid
		order.CompletedAt = now
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		topUp.Status = common.TopUpStatusSuccess
		topUp.CompleteTime = now
		if err := tx.Save(&topUp).Error; err != nil {
			return err
		}
		result.CreditedQuotaDelta = order.PackageQuota
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.CreditedQuotaDelta > 0 {
		RecordTopupLog(result.UserID, fmt.Sprintf("TokenPay fixed package %s USDT credited %s balance", received.String(), input.TradeNo), input.CallerIP, PaymentMethodTokenPay, PaymentProviderTokenPay)
		RecordAffiliateRewardLog(reward)
	}
	return result, nil
}
