package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenPayOrder(t *testing.T, suffix string, network string) (*User, *TokenPayTopUpOrder) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenPayTopUpOrder{}, &TokenPayCreditedTransaction{}, &Option{}, &AffiliateReward{}))
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	tradeNo := fmt.Sprintf("tokenpay-%s-%d", suffix, time.Now().UnixNano())
	user := &User{Username: tradeNo, AffCode: "aff-" + tradeNo, Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	topup := &TopUp{UserId: user.Id, TradeNo: tradeNo, Amount: 66, Money: 10,
		PaymentMethod: PaymentMethodTokenPay, PaymentProvider: PaymentProviderTokenPay,
		Status: common.TopUpStatusPending, CreateTime: time.Now().Unix()}
	providerID := "provider-" + tradeNo
	order := &TokenPayTopUpOrder{TradeNo: tradeNo, UserID: user.Id, Network: network,
		RequestedUSDT: "10", PackageBalance: "66", OrderUserKey: tradeNo + "-" + network,
		Status:    TokenPayOrderStatusPending,
		CreatedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, CreateTokenPayTopUp(topup, order))
	require.NoError(t, SaveTokenPayInvoice(tradeNo, user.Id, providerID, "10", "TKGTx4pCKiKQbk8evXHTborfZn754TGViP", "https://pay.example.com/Pay?Id="+providerID, order.ExpiresAt))
	order.ProviderOrderID = &providerID
	order.ReceiveAddress = "TKGTx4pCKiKQbk8evXHTborfZn754TGViP"
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		DB.Where("order_id = ?", order.ID).Delete(&TokenPayCreditedTransaction{})
		DB.Delete(&TokenPayTopUpOrder{}, order.ID)
		DB.Delete(&TopUp{}, topup.Id)
		DB.Unscoped().Delete(&User{}, user.Id)
	})
	return user, order
}

func TestTokenPaySettlesExactPackageOnceAfterRateChanges(t *testing.T) {
	user, order := setupTokenPayOrder(t, "once", operation_setting.TokenPayNetworkTRON)
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 200
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })
	input := TokenPaySettlementInput{TradeNo: order.TradeNo, ProviderOrderID: *order.ProviderOrderID,
		Network: order.Network, ReceiveAddress: order.ReceiveAddress, ReceivedUSDT: "10.00", TransactionID: "tx-once"}
	result, err := SettleTokenPayTopUp(input)
	require.NoError(t, err)
	assert.EqualValues(t, 6600, result.CreditedQuotaDelta)
	again, err := SettleTokenPayTopUp(input)
	require.NoError(t, err)
	assert.Zero(t, again.CreditedQuotaDelta)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.Equal(t, 6600, user.Quota)
	var rows int64
	require.NoError(t, DB.Model(&TokenPayCreditedTransaction{}).Where("order_id = ?", order.ID).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)
}

func TestTokenPayRejectsShortAndReusedTransactionsWithoutCrediting(t *testing.T) {
	userOne, orderOne := setupTokenPayOrder(t, "first", operation_setting.TokenPayNetworkTRON)
	userTwo, orderTwo := setupTokenPayOrder(t, "second", operation_setting.TokenPayNetworkTRON)
	short := TokenPaySettlementInput{TradeNo: orderOne.TradeNo, ProviderOrderID: *orderOne.ProviderOrderID,
		Network: orderOne.Network, ReceiveAddress: orderOne.ReceiveAddress, ReceivedUSDT: "9", TransactionID: "tx-short"}
	require.Error(t, func() error { _, err := SettleTokenPayTopUp(short); return err }())
	paid := short
	paid.ReceivedUSDT = "10"
	paid.TransactionID = "tx-shared"
	_, err := SettleTokenPayTopUp(paid)
	require.NoError(t, err)
	paid.TradeNo = orderTwo.TradeNo
	paid.ProviderOrderID = *orderTwo.ProviderOrderID
	_, err = SettleTokenPayTopUp(paid)
	require.Error(t, err)
	require.NoError(t, DB.First(userOne, userOne.Id).Error)
	require.NoError(t, DB.First(userTwo, userTwo.Id).Error)
	assert.Equal(t, 6600, userOne.Quota)
	assert.Zero(t, userTwo.Quota)
}

func TestTokenPayRejectsGenericAdministratorCompletion(t *testing.T) {
	_, order := setupTokenPayOrder(t, "admin", operation_setting.TokenPayNetworkTRON)
	require.Error(t, ManualCompleteTopUp(order.TradeNo, "127.0.0.1"))
	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	assert.Equal(t, common.TopUpStatusPending, topup.Status)
}

func TestTokenPaySettlementRechecksUserQuotaCeiling(t *testing.T) {
	user, order := setupTokenPayOrder(t, "ceiling", operation_setting.TokenPayNetworkTRON)
	require.NoError(t, DB.Model(user).Update("quota", int(operation_setting.TokenPayMaxUserQuota-6599)).Error)
	_, err := SettleTokenPayTopUp(TokenPaySettlementInput{TradeNo: order.TradeNo, ProviderOrderID: *order.ProviderOrderID,
		Network: order.Network, ReceiveAddress: order.ReceiveAddress, ReceivedUSDT: "10", TransactionID: "tx-ceiling"})
	require.Error(t, err)
	require.NoError(t, DB.First(user, user.Id).Error)
	assert.EqualValues(t, operation_setting.TokenPayMaxUserQuota-6599, user.Quota)
	var rows int64
	require.NoError(t, DB.Model(&TokenPayCreditedTransaction{}).Where("order_id = ?", order.ID).Count(&rows).Error)
	assert.Zero(t, rows)
}

func TestTokenPayHistoryShowsExactPackageBalance(t *testing.T) {
	user, order := setupTokenPayOrder(t, "history", operation_setting.TokenPayNetworkTRON)
	require.NoError(t, DB.Model(order).Update("package_balance", "66.5").Error)
	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	items, err := GetTopUpHistoryItems([]*TopUp{&topup})
	require.NoError(t, err)
	data, err := common.Marshal(items[0])
	require.NoError(t, err)
	assert.Contains(t, string(data), `"amount":66.5`)
	assert.Contains(t, string(data), `"money":10`)
	assert.Equal(t, user.Id, topup.UserId)
}

func TestTokenPayCreateOrderRollsBackOnInvalidQuota(t *testing.T) {
	user, order := setupTokenPayOrder(t, "invalid", operation_setting.TokenPayNetworkTRON)
	topup := &TopUp{UserId: user.Id, TradeNo: order.TradeNo + "-bad", PaymentProvider: PaymentProviderTokenPay,
		PaymentMethod: PaymentMethodTokenPay, Status: common.TopUpStatusPending}
	invalid := &TokenPayTopUpOrder{TradeNo: topup.TradeNo, UserID: user.Id, Network: order.Network,
		RequestedUSDT: "10", PackageBalance: "0", Status: TokenPayOrderStatusPending}
	require.Error(t, DB.Transaction(func(tx *gorm.DB) error { return CreateTokenPayTopUpTx(tx, topup, invalid) }))
	var count int64
	require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ?", topup.TradeNo).Count(&count).Error)
	assert.Zero(t, count)
}

func TestTokenPayRecoveryHashOnlyRecordsReviewClaim(t *testing.T) {
	user, order := setupTokenPayOrder(t, "recovery", operation_setting.TokenPayNetworkTRON)
	require.NoError(t, SaveTokenPayRecoveryClaim(order.ID, user.Id, "abc"))
	fresh, err := GetTokenPayOrder(user.Id, order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, "abc", fresh.RecoveryHash)
	assert.Equal(t, TokenPayOrderStatusUnpaid, fresh.Status)
	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	assert.Equal(t, common.TopUpStatusPending, topup.Status)
	require.Error(t, SaveTokenPayRecoveryClaim(order.ID, user.Id+1, "other"))
}
