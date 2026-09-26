package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func NewConfiguredSHKeeperClient() (*SHKeeperClient, error) {
	settings := operation_setting.GetSHKeeperPaymentSetting()
	return NewSHKeeperClient(SHKeeperClientConfig{BaseURL: settings.BaseURL, APIKey: settings.APIKey, BackendKey: settings.BackendKey, AllowPrivateURL: settings.AllowPrivateURL}, nil)
}

func ReconcileSHKeeperOrder(ctx context.Context, tradeNo string) (*model.SHKeeperSettlementResult, error) {
	order, err := model.GetSHKeeperTopUpByTradeNo(0, tradeNo)
	if err != nil {
		return nil, err
	}
	attemptedAt := common.GetTimestamp()
	if attemptedAt <= order.LastReconciledAt {
		attemptedAt = order.LastReconciledAt + 1
	}
	if err := model.RecordSHKeeperReconciliationAttempt(order.ID, attemptedAt); err != nil {
		return nil, err
	}
	client, err := NewConfiguredSHKeeperClient()
	if err != nil {
		return nil, err
	}
	invoice, err := client.GetInvoiceByExternalID(ctx, order.Crypto, order.ExternalID)
	if err != nil {
		return nil, err
	}
	expected, err := decimal.NewFromString(order.RequestedUSDT)
	if err != nil || !expected.IsPositive() || order.SettlementMode != model.SHKeeperSettlementModeFixedPackage {
		return nil, errors.New("SHKeeper order requires operator recovery")
	}
	amount, err := decimal.NewFromString(invoice.AmountFiat)
	if err != nil || !amount.Equal(expected) || invoice.Fiat != "USD" || invoice.ExternalID != order.ExternalID {
		return nil, errors.New("SHKeeper invoice identity or amount mismatch")
	}
	if strings.TrimSpace(order.InvoiceAddress) == "" {
		return nil, errors.New("SHKeeper invoice address unavailable; provider recovery required")
	}
	status := ""
	switch strings.ToUpper(strings.TrimSpace(invoice.Status)) {
	case "UNPAID":
		status = model.SHKeeperOrderStatusUnpaid
	case "PARTIAL":
		status = model.SHKeeperOrderStatusPartial
	case "PAID":
		status = model.SHKeeperOrderStatusPaid
	case "OVERPAID":
		status = model.SHKeeperOrderStatusOverpaid
	default:
		return nil, errors.New("unsupported SHKeeper invoice status")
	}
	total := decimal.Zero
	seen := make(map[string]decimal.Decimal)
	transactions := make([]model.SHKeeperSettlementTransaction, 0, len(invoice.Transactions))
	for _, tx := range invoice.Transactions {
		if !strings.EqualFold(strings.TrimSpace(tx.Status), "CONFIRMED") {
			continue
		}
		if tx.Crypto != order.Crypto || tx.Address != order.InvoiceAddress {
			return nil, errors.New("SHKeeper transaction network or address mismatch")
		}
		amount, err := decimal.NewFromString(strings.TrimSpace(tx.AmountUSDT))
		if err != nil || amount.IsNegative() {
			return nil, errors.New("invalid SHKeeper confirmed amount")
		}
		txID, err := NormalizeSHKeeperTransactionID(order.Crypto, tx.TxID)
		if err != nil {
			return nil, err
		}
		if previous, ok := seen[txID]; ok {
			if !previous.Equal(amount) {
				return nil, errors.New("conflicting SHKeeper transaction amount")
			}
			continue
		}
		seen[txID] = amount
		total = total.Add(amount)
		transactions = append(transactions, model.SHKeeperSettlementTransaction{TxID: txID, AmountUSDT: amount.String()})
	}
	result, err := model.SettleSHKeeperTopUp(model.SHKeeperSettlementInput{TradeNo: order.TradeNo, UserID: order.UserID, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress, ProviderStatus: status, ReceivedUSDT: total.String(), Transactions: transactions, ReconciledAt: attemptedAt})
	if err != nil {
		return nil, err
	}
	if result.CreditedQuotaDelta > 0 {
		model.RecordTopupLog(result.UserID, fmt.Sprintf("SHKeeper top-up credited: marker=%s balance=%s", result.AuditMarker, result.CreditedBalanceDelta), "", model.PaymentMethodSHKeeper, model.PaymentProviderSHKeeper)
	}
	return result, nil
}

type SHKeeperReconcileSummary struct {
	Processed int `json:"processed"`
	Credited  int `json:"credited"`
	Failed    int `json:"failed"`
}

func ReconcileSHKeeperOrders(ctx context.Context, limit int) (SHKeeperReconcileSummary, error) {
	summary := SHKeeperReconcileSummary{}
	orders, err := model.ListSHKeeperOrdersForReconciliation(common.GetTimestamp(), limit)
	if err != nil {
		return summary, err
	}
	for _, order := range orders {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		summary.Processed++
		result, err := ReconcileSHKeeperOrder(ctx, order.TradeNo)
		if err != nil {
			summary.Failed++
			continue
		}
		if result.CreditedQuotaDelta > 0 {
			summary.Credited++
		}
	}
	return summary, nil
}
