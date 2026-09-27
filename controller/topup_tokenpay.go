package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type tokenPayRequest struct {
	USDTAmount int64  `json:"usdt_amount"`
	Network    string `json:"network"`
}

func isTokenPayWebhookEnabled() bool {
	settings := operation_setting.GetTokenPayPaymentSetting()
	return strings.TrimSpace(settings.BaseURL) != "" && strings.TrimSpace(settings.APIToken) != ""
}

func isTokenPayTopUpEnabled() bool {
	settings := operation_setting.GetTokenPayPaymentSetting()
	return operation_setting.IsPaymentComplianceConfirmed() && settings.Enabled && isTokenPayWebhookEnabled() &&
		len(settings.Packages) > 0 && len(settings.EnabledNetworks) > 0
}

func respondTokenPayOrder(c *gin.Context, order *model.TokenPayTopUpOrder) {
	common.ApiSuccess(c, gin.H{
		"trade_no": order.TradeNo, "network": order.Network, "usdt_amount": order.RequestedUSDT,
		"balance_amount": order.PackageBalance, "address": order.ReceiveAddress,
		"payment_url": order.PaymentURL, "status": order.Status, "expires_at": order.ExpiresAt,
		"recovery_submitted": order.RecoveryHash != "",
	})
}

func RequestTokenPay(c *gin.Context) {
	var input tokenPayRequest
	if c.ShouldBindJSON(&input) != nil || !isTokenPayTopUpEnabled() {
		common.ApiErrorMsg(c, "TokenPay top-up is unavailable")
		return
	}
	settings := *operation_setting.GetTokenPayPaymentSetting()
	if err := settings.Normalize(); err != nil {
		common.ApiErrorMsg(c, "Invalid TokenPay settings")
		return
	}
	allowedNetwork := false
	for _, network := range settings.EnabledNetworks {
		if input.Network == network {
			allowedNetwork = true
			break
		}
	}
	if !allowedNetwork {
		common.ApiErrorMsg(c, "Selected TokenPay network is unavailable")
		return
	}
	item, exists := settings.FindPackage(input.USDTAmount)
	if !exists {
		common.ApiErrorMsg(c, "Selected TokenPay package is unavailable")
		return
	}
	baseURL := strings.TrimRight(service.GetCallbackAddress(), "/")
	parsedCallback, callbackErr := url.Parse(baseURL)
	if callbackErr != nil || parsedCallback.Scheme != "https" || parsedCallback.Host == "" {
		common.ApiErrorMsg(c, "Public HTTPS callback address is required")
		return
	}
	client, err := service.NewTokenPayClient(settings.BaseURL, settings.APIToken, settings.AllowPrivateURL, nil)
	if err != nil {
		common.ApiErrorMsg(c, "TokenPay is unavailable")
		return
	}
	userID := c.GetInt("id")
	tradeNo := fmt.Sprintf("USR%dTP%s%d", userID, common.GetRandomString(8), time.Now().Unix())
	now := time.Now().Unix()
	balance, _ := decimal.NewFromString(item.Balance)
	order := &model.TokenPayTopUpOrder{TradeNo: tradeNo, UserID: userID, Network: input.Network,
		RequestedUSDT: service.TokenPayPaymentAmount(item.USDT), PackageBalance: item.Balance,
		OrderUserKey: service.TokenPayOrderUserKey(tradeNo, input.Network), Status: model.TokenPayOrderStatusPending,
		CreatedAt: now, ExpiresAt: now + 30*60}
	topUp := &model.TopUp{TradeNo: tradeNo, UserId: userID, Amount: balance.IntPart(), Money: float64(item.USDT),
		PaymentMethod: model.PaymentMethodTokenPay, PaymentProvider: model.PaymentProviderTokenPay,
		Status: common.TopUpStatusPending, CreateTime: now}
	if err := model.CreateTokenPayTopUp(topUp, order); err != nil {
		common.ApiErrorMsg(c, "Unable to create TokenPay top-up order")
		return
	}
	invoice, err := client.CreateOrder(c.Request.Context(), service.TokenPayCreateRequest{
		OutOrderID: tradeNo, OrderUserKey: order.OrderUserKey, ActualAmount: order.RequestedUSDT, Currency: order.Network,
		NotifyURL: baseURL + "/api/tokenpay/webhook", RedirectURL: baseURL + "/wallet",
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("TokenPay invoice creation failed trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "Unable to create an exact TokenPay invoice; order retained for review")
		return
	}
	if err := model.SaveTokenPayInvoice(tradeNo, userID, invoice.ProviderOrderID, invoice.PayAmountUSDT,
		invoice.ReceiveAddress, invoice.PaymentURL, invoice.ExpiresAt); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("TokenPay invoice save failed trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "Unable to save TokenPay invoice; order retained for review")
		return
	}
	order.ProviderOrderID = &invoice.ProviderOrderID
	order.ProviderAmountUSDT = invoice.PayAmountUSDT
	order.ReceiveAddress = invoice.ReceiveAddress
	order.PaymentURL = invoice.PaymentURL
	order.ExpiresAt = invoice.ExpiresAt
	order.Status = model.TokenPayOrderStatusUnpaid
	respondTokenPayOrder(c, order)
}

func GetTokenPayOrder(c *gin.Context) {
	order, err := model.GetTokenPayOrder(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		common.ApiErrorMsg(c, "TokenPay order not found")
		return
	}
	respondTokenPayOrder(c, order)
}

func SubmitTokenPayTransaction(c *gin.Context) {
	var input struct {
		TransactionID string `json:"transaction_id"`
	}
	if c.ShouldBindJSON(&input) != nil {
		common.ApiErrorMsg(c, "Invalid TokenPay transaction claim")
		return
	}
	order, err := model.GetTokenPayOrder(c.GetInt("id"), c.Param("trade_no"))
	if err != nil || order.ProviderOrderID == nil || order.Status != model.TokenPayOrderStatusUnpaid {
		common.ApiErrorMsg(c, "TokenPay order is unavailable for review")
		return
	}
	txID, err := service.NormalizeTokenPayTransactionID(order.Network, input.TransactionID)
	if err != nil {
		common.ApiErrorMsg(c, "Invalid TokenPay transaction hash")
		return
	}
	if err := model.SaveTokenPayRecoveryClaim(order.ID, order.UserID, txID); err != nil {
		common.ApiErrorMsg(c, "Unable to submit TokenPay transaction for review")
		return
	}
	common.ApiSuccess(c, gin.H{"trade_no": order.TradeNo, "review_required": true})
}

func TokenPayWebhook(c *gin.Context) {
	if !isTokenPayWebhookEnabled() {
		c.String(http.StatusServiceUnavailable, "fail")
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	callback, err := service.VerifyTokenPayCallback(operation_setting.GetTokenPayPaymentSetting().APIToken, body)
	if err != nil || callback.SignatureType != "HmacSha256" || callback.OutOrderID == "" {
		c.String(http.StatusUnauthorized, "fail")
		return
	}
	if callback.Status != 1 {
		c.String(http.StatusOK, "ok")
		return
	}
	order, err := model.GetTokenPayOrder(0, callback.OutOrderID)
	if err != nil || order.ProviderOrderID == nil || callback.ID != *order.ProviderOrderID ||
		callback.OrderUserKey != order.OrderUserKey || callback.Currency != order.Network || callback.BaseCurrency != "USD" ||
		callback.CurrencyName != "USDT" || callback.IsCustomAmount || callback.IsDynamicAmount {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	expectedChain := "TRON"
	if order.Network == operation_setting.TokenPayNetworkBSC {
		expectedChain = "BSC"
	} else if order.Network == operation_setting.TokenPayNetworkPolygon {
		expectedChain = "Polygon"
	}
	if callback.BlockChainName != expectedChain {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	txID, err := service.NormalizeTokenPayTransactionID(order.Network, callback.BlockTransactionID)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	amount, amountErr := decimal.NewFromString(callback.Amount)
	actual, actualErr := decimal.NewFromString(callback.ActualAmount)
	requested, requestedErr := decimal.NewFromString(order.RequestedUSDT)
	if amountErr != nil || actualErr != nil || requestedErr != nil || !amount.Equal(requested) || !actual.Equal(requested) {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	_, err = model.SettleTokenPayTopUp(model.TokenPaySettlementInput{
		TradeNo: order.TradeNo, ProviderOrderID: callback.ID, Network: order.Network,
		ReceiveAddress: callback.ToAddress, ReceivedUSDT: callback.PayAmount, TransactionID: txID, CallerIP: c.ClientIP(),
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("TokenPay settlement failed trade_no=%s error=%q", order.TradeNo, err.Error()))
		c.String(http.StatusInternalServerError, "fail")
		return
	}
	c.String(http.StatusOK, "ok")
}
