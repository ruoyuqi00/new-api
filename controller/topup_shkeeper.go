package controller

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type shkeeperPayRequest struct {
	USDTAmount int64  `json:"usdt_amount"`
	Crypto     string `json:"crypto"`
}
type SHKeeperTopUpResponse struct {
	TradeNo         string   `json:"trade_no"`
	Network         string   `json:"network"`
	Crypto          string   `json:"crypto"`
	USDTAmount      string   `json:"usdt_amount"`
	BalanceAmount   string   `json:"balance_amount"`
	Address         string   `json:"address"`
	QRPayload       string   `json:"qr_payload"`
	Status          string   `json:"status"`
	ExpiresAt       int64    `json:"expires_at"`
	ReceivedUSDT    string   `json:"received_usdt"`
	CreditedBalance string   `json:"credited_balance"`
	TransactionIDs  []string `json:"transaction_ids,omitempty"`
}

func isSHKeeperWebhookEnabled() bool {
	settings := operation_setting.GetSHKeeperPaymentSetting()
	return strings.TrimSpace(settings.BaseURL) != "" && strings.TrimSpace(settings.APIKey) != ""
}

func isSHKeeperTopUpEnabled() bool {
	settings := operation_setting.GetSHKeeperPaymentSetting()
	return operation_setting.IsPaymentComplianceConfirmed() && settings.Enabled && isSHKeeperWebhookEnabled() && len(settings.Packages) > 0 && len(settings.EnabledNetworks) > 0
}

func shkeeperNetworkLabel(crypto string) string {
	switch crypto {
	case "USDT":
		return "TRON (TRC20)"
	case "BNB-USDT":
		return "BSC (BEP20)"
	case "POLYGON-USDT":
		return "Polygon"
	}
	return crypto
}

func respondSHKeeperOrder(c *gin.Context, order *model.SHKeeperTopUpOrder) {
	transactions, err := model.ListSHKeeperCreditedTransactions(order.ID)
	if err != nil {
		common.ApiErrorMsg(c, "Unable to read SHKeeper transactions")
		return
	}
	ids := make([]string, 0, len(transactions))
	for _, tx := range transactions {
		ids = append(ids, tx.TxID)
	}
	common.ApiSuccess(c, SHKeeperTopUpResponse{TradeNo: order.TradeNo, Network: shkeeperNetworkLabel(order.Crypto), Crypto: order.Crypto, USDTAmount: order.RequestedUSDT, BalanceAmount: order.PackageBalance, Address: order.InvoiceAddress, QRPayload: order.InvoiceAddress, Status: order.Status, ExpiresAt: order.ExpiresAt, ReceivedUSDT: order.ReceivedUSDT, CreditedBalance: order.CreditedBalance, TransactionIDs: ids})
}

func RequestSHKeeperPay(c *gin.Context) {
	var input shkeeperPayRequest
	if common.DecodeJson(c.Request.Body, &input) != nil || c.GetInt("id") <= 0 {
		common.ApiErrorMsg(c, "Invalid SHKeeper payment request")
		return
	}
	if !isSHKeeperTopUpEnabled() {
		common.ApiErrorMsg(c, "SHKeeper top-up is not enabled")
		return
	}
	settings := *operation_setting.GetSHKeeperPaymentSetting()
	if err := settings.Normalize(); err != nil {
		common.ApiErrorMsg(c, "Invalid SHKeeper payment settings")
		return
	}
	crypto, err := service.NormalizeSHKeeperCrypto(input.Crypto)
	if err != nil || !slices.Contains(settings.EnabledNetworks, crypto) {
		common.ApiErrorMsg(c, "Selected SHKeeper network is unavailable")
		return
	}
	item, ok := settings.FindPackage(input.USDTAmount)
	if !ok {
		common.ApiErrorMsg(c, "Selected SHKeeper package is unavailable")
		return
	}
	client, err := service.NewConfiguredSHKeeperClient()
	if err != nil {
		common.ApiErrorMsg(c, "SHKeeper is unavailable")
		return
	}
	key, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		common.ApiErrorMsg(c, "Unable to create SHKeeper order")
		return
	}
	tradeNo := fmt.Sprintf("USDT%d%s", c.GetInt("id"), key)
	now := time.Now().Unix()
	order := &model.SHKeeperTopUpOrder{TradeNo: tradeNo, ExternalID: tradeNo, UserID: c.GetInt("id"), Crypto: crypto, SettlementMode: model.SHKeeperSettlementModeFixedPackage, RequestedUSDT: strconv.FormatInt(item.USDT, 10), PackageBalance: item.Balance, ReceivedUSDT: "0", CreditedBalance: "0", Status: model.SHKeeperOrderStatusPending, CreatedAt: now, ExpiresAt: now + int64(settings.InvoiceExpiryMinutes)*60, CallbackURL: strings.TrimRight(service.GetCallbackAddress(), "/") + "/api/shkeeper/webhook"}
	balance, _ := decimal.NewFromString(item.Balance)
	// The legacy integer amount column stores the whole balance portion; history
	// uses the package snapshot to retain any fractional balance exactly.
	topUp := &model.TopUp{TradeNo: tradeNo, UserId: order.UserID, Amount: balance.IntPart(), Money: float64(item.USDT), PaymentMethod: model.PaymentMethodSHKeeper, PaymentProvider: model.PaymentProviderSHKeeper, Status: common.TopUpStatusPending, CreateTime: now}
	if err := model.InsertSHKeeperTopUp(topUp, order); err != nil {
		common.ApiErrorMsg(c, "Unable to create SHKeeper order or user quota limit exceeded")
		return
	}
	invoice, err := client.CreatePaymentRequest(c.Request.Context(), service.SHKeeperPaymentRequest{Crypto: crypto, ExternalID: tradeNo, Fiat: "USD", Amount: order.RequestedUSDT, CallbackURL: order.CallbackURL})
	if err != nil {
		// A timeout may have created an invoice. Keep its identity for lookup-only recovery.
		_, _ = service.ReconcileSHKeeperOrder(c.Request.Context(), tradeNo)
		respondSHKeeperOrder(c, order)
		return
	}
	amount, amountErr := decimal.NewFromString(invoice.Amount)
	fiatAmount, fiatErr := decimal.NewFromString(invoice.AmountFiat)
	if amountErr != nil || fiatErr != nil || !amount.Equal(decimal.NewFromInt(item.USDT)) || !fiatAmount.Equal(decimal.NewFromInt(item.USDT)) || invoice.ExternalID != tradeNo || invoice.Crypto != crypto || invoice.Fiat != "USD" || strings.TrimSpace(invoice.Wallet) == "" {
		common.ApiErrorMsg(c, "SHKeeper invoice identity or amount mismatch; order retained for recovery")
		return
	}
	details := model.SHKeeperInvoiceDetails{QuotedUSDT: amount.String(), InvoiceAddress: strings.TrimSpace(invoice.Wallet), ProviderInvoiceID: strconv.FormatInt(invoice.ProviderID, 10), Status: model.SHKeeperOrderStatusUnpaid, ExpiresAt: order.ExpiresAt}
	if err := model.UpdateSHKeeperInvoiceDetails(tradeNo, order.UserID, details); err != nil {
		common.ApiErrorMsg(c, "Unable to save SHKeeper invoice; order retained for recovery")
		return
	}
	order.InvoiceAddress = details.InvoiceAddress
	order.Status = details.Status
	respondSHKeeperOrder(c, order)
}

func GetSHKeeperOrder(c *gin.Context) {
	if c.GetInt("id") <= 0 {
		common.ApiErrorMsg(c, "SHKeeper order not found")
		return
	}
	order, err := model.GetSHKeeperTopUpByTradeNo(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		common.ApiErrorMsg(c, "SHKeeper order not found")
		return
	}
	// Status reads stay available during provider outages; the scheduler repairs missed callbacks.
	respondSHKeeperOrder(c, order)
}

func SubmitSHKeeperTransaction(c *gin.Context) {
	var input struct {
		TransactionID string `json:"txid"`
	}
	if common.DecodeJson(c.Request.Body, &input) != nil || c.GetInt("id") <= 0 {
		common.ApiErrorMsg(c, "Invalid SHKeeper transaction request")
		return
	}
	order, err := model.GetSHKeeperTopUpByTradeNo(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		common.ApiErrorMsg(c, "SHKeeper order not found")
		return
	}
	if order.InvoiceAddress == "" || (order.Status != model.SHKeeperOrderStatusUnpaid && order.Status != model.SHKeeperOrderStatusPartial && order.Status != model.SHKeeperOrderStatusConfirming) {
		common.ApiErrorMsg(c, "SHKeeper transaction recovery is unavailable for this order")
		return
	}
	txID, err := service.NormalizeSHKeeperTransactionID(order.Crypto, input.TransactionID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	client, err := service.NewConfiguredSHKeeperClient()
	if err != nil {
		common.ApiErrorMsg(c, "SHKeeper is unavailable")
		return
	}
	if err := client.NotifyTransaction(c.Request.Context(), order.Crypto, txID); err != nil {
		common.ApiErrorMsg(c, "SHKeeper could not rescan this transaction")
		return
	}
	common.ApiSuccess(c, gin.H{"trade_no": order.TradeNo, "txid": txID, "status": "submitted"})
}

func SHKeeperWebhook(c *gin.Context) {
	if !isSHKeeperWebhookEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	if service.VerifySHKeeperWebhook(operation_setting.GetSHKeeperPaymentSetting().APIKey, c.GetHeader("X-Shkeeper-Timestamp"), c.GetHeader("X-Shkeeper-Signature"), body, time.Now()) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false})
		return
	}
	var payload struct {
		ExternalID string `json:"external_id"`
		Crypto     string `json:"crypto"`
		Address    string `json:"addr"`
	}
	if common.Unmarshal(body, &payload) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	order, err := model.GetSHKeeperTopUpByTradeNo(0, payload.ExternalID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false})
		return
	}
	if payload.ExternalID != order.ExternalID || payload.Crypto != order.Crypto || payload.Address != order.InvoiceAddress || order.InvoiceAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	if _, err := service.ReconcileSHKeeperOrder(c.Request.Context(), order.TradeNo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "SHKeeper reconciliation failed"})
		return
	}
	c.Status(http.StatusAccepted)
}
