package model

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareAffiliateSourceTest(t *testing.T) (inviter *User, invitee *User) {
	t.Helper()
	inviter, invitee = setupAffiliateRewardTest(t)
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &Redemption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TopUp{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Redemption{}).Error)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1_000
	setAffiliateCreditRebateOptions(t, true, 1_000)
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TopUp{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Redemption{}).Error)
	})
	return inviter, invitee
}

func requireAffiliateSourceBalances(t *testing.T, inviteeId int, inviterId int, creditedQuota int, rewardQuota int) {
	t.Helper()
	invitee := getAffiliateRewardUser(t, inviteeId)
	assert.Equal(t, creditedQuota, invitee.Quota)
	inviter := getAffiliateRewardUser(t, inviterId)
	assert.Equal(t, rewardQuota, inviter.AffQuota)
	assert.Equal(t, rewardQuota, inviter.AffHistoryQuota)
}

func requireAffiliateRewardCount(t *testing.T, sourceType string, sourceId string, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&AffiliateReward{}).
		Where("source_type = ? AND source_id = ?", sourceType, sourceId).
		Count(&count).Error)
	assert.Equal(t, expected, count)
}

func TestEligibleTopUpProvidersAwardAffiliateRewardExactlyOnce(t *testing.T) {
	testCases := []struct {
		name            string
		paymentProvider string
		amount          int64
		money           float64
		expectedQuota   int
		complete        func(tradeNo string) error
	}{
		{
			name:            "stripe",
			paymentProvider: PaymentProviderStripe,
			amount:          2,
			money:           2,
			expectedQuota:   2_000,
			complete: func(tradeNo string) error {
				return Recharge(tradeNo, "cus-affiliate", "127.0.0.1")
			},
		},
		{
			name:            "creem",
			paymentProvider: PaymentProviderCreem,
			amount:          2_000,
			money:           2,
			expectedQuota:   2_000,
			complete: func(tradeNo string) error {
				return RechargeCreem(tradeNo, "", "", "127.0.0.1")
			},
		},
		{
			name:            "waffo",
			paymentProvider: PaymentProviderWaffo,
			amount:          2,
			money:           2,
			expectedQuota:   2_000,
			complete: func(tradeNo string) error {
				return RechargeWaffo(tradeNo, "127.0.0.1")
			},
		},
		{
			name:            "waffo pancake",
			paymentProvider: PaymentProviderWaffoPancake,
			amount:          2,
			money:           2,
			expectedQuota:   2_000,
			complete: func(tradeNo string) error {
				return RechargeWaffoPancake(tradeNo)
			},
		},
		{
			name:            "epay",
			paymentProvider: PaymentProviderEpay,
			amount:          2,
			money:           2,
			expectedQuota:   2_000,
			complete: func(tradeNo string) error {
				return RechargeEpay(tradeNo, "alipay", "127.0.0.1")
			},
		},
	}

	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			inviter, invitee := prepareAffiliateSourceTest(t)
			tradeNo := fmt.Sprintf("affiliate-topup-%d", index)
			topUp := &TopUp{
				UserId:          invitee.Id,
				Amount:          testCase.amount,
				Money:           testCase.money,
				TradeNo:         tradeNo,
				PaymentMethod:   testCase.paymentProvider,
				PaymentProvider: testCase.paymentProvider,
				CreateTime:      common.GetTimestamp(),
				Status:          common.TopUpStatusPending,
			}
			require.NoError(t, DB.Create(topUp).Error)

			require.NoError(t, testCase.complete(tradeNo))
			require.NoError(t, testCase.complete(tradeNo))

			requireAffiliateSourceBalances(t, invitee.Id, inviter.Id, testCase.expectedQuota, testCase.expectedQuota/10)
			requireAffiliateRewardCount(t, AffiliateRewardSourceTopUp, tradeNo, 1)
			assert.Equal(t, common.TopUpStatusSuccess, GetTopUpByTradeNo(tradeNo).Status)
		})
	}
}

func TestManualCompleteTopUpAwardsAffiliateRewardExactlyOnce(t *testing.T) {
	inviter, invitee := prepareAffiliateSourceTest(t)
	topUp := &TopUp{
		UserId:          invitee.Id,
		Amount:          2,
		Money:           2,
		TradeNo:         "affiliate-manual-topup",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(topUp).Error)

	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))
	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))

	requireAffiliateSourceBalances(t, invitee.Id, inviter.Id, 2_000, 200)
	requireAffiliateRewardCount(t, AffiliateRewardSourceTopUp, topUp.TradeNo, 1)
}

func TestRedeemAwardsAffiliateRewardExactlyOnce(t *testing.T) {
	inviter, invitee := prepareAffiliateSourceTest(t)
	redemption := &Redemption{
		Name:        "affiliate-redeem",
		Key:         "affiliate-redeem-key",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       10_000,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(redemption).Error)

	credited, err := Redeem(redemption.Key, invitee.Id)
	require.NoError(t, err)
	assert.Equal(t, 10_000, credited)
	_, err = Redeem(redemption.Key, invitee.Id)
	require.Error(t, err)

	requireAffiliateSourceBalances(t, invitee.Id, inviter.Id, 10_000, 1_000)
	requireAffiliateRewardCount(t, AffiliateRewardSourceRedemption, strconv.Itoa(redemption.Id), 1)
}

func TestAdminAddQuotaAwardsAffiliateReward(t *testing.T) {
	inviter, invitee := prepareAffiliateSourceTest(t)

	reward, err := AddUserQuotaWithAffiliateReward(invitee.Id, 5_000, "admin-event-1")
	require.NoError(t, err)
	require.NotNil(t, reward)

	requireAffiliateSourceBalances(t, invitee.Id, inviter.Id, 5_000, 500)
	requireAffiliateRewardCount(t, AffiliateRewardSourceAdminAdd, "admin-event-1", 1)
}
