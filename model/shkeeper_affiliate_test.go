package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSHKeeperAffiliateSettlementAtomicity(t *testing.T) {
	for _, scenario := range []string{"enabled", "disabled", "missing-inviter", "cap-boundary", "cap-exceeded", "order-save-failure", "legacy-installments"} {
		t.Run(scenario, func(t *testing.T) {
			user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
			require.NoError(t, DB.AutoMigrate(&Option{}, &AffiliateReward{}))
			setAffiliateCreditRebateOptions(t, scenario != "disabled", 525)
			t.Cleanup(func() {
				require.NoError(t, DB.Where("source_id = ?", order.TradeNo).Delete(&AffiliateReward{}).Error)
				setAffiliateCreditRebateOptions(t, false, 0)
			})
			inviter := &User{Username: "shkeeper-affiliate", AffCode: "shkeeper-affiliate"}
			require.NoError(t, DB.Create(inviter).Error)
			t.Cleanup(func() { require.NoError(t, DB.Unscoped().Delete(inviter).Error) })
			inviterID := inviter.Id
			if scenario == "missing-inviter" {
				inviterID = 999999
			}
			require.NoError(t, DB.Model(user).Update("inviter_id", inviterID).Error)
			initial := 0
			if scenario == "cap-boundary" || scenario == "cap-exceeded" {
				initial = int(operation_setting.SHKeeperMaxUserQuota) - 6600
				if scenario == "cap-exceeded" {
					initial++
				}
				require.NoError(t, DB.Model(user).Update("quota", initial).Error)
			}
			if scenario == "order-save-failure" {
				require.NoError(t, DB.Callback().Update().Before("gorm:update").Register("shkeeper_fail_order", func(tx *gorm.DB) {
					if tx.Statement.Table == "sh_keeper_top_up_orders" {
						tx.AddError(assert.AnError)
					}
				}))
				t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove("shkeeper_fail_order")) })
			}
			input := SHKeeperSettlementInput{TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress, ReceivedUSDT: "10", ProviderStatus: SHKeeperOrderStatusPaid, Transactions: []SHKeeperSettlementTransaction{{TxID: "tx-a", AmountUSDT: "10"}}}
			if scenario == "legacy-installments" {
				require.NoError(t, DB.Model(order).Updates(map[string]any{"settlement_mode": "", "locked_rate": "6.6"}).Error)
				input.ReceivedUSDT = "4"
				input.Transactions[0].AmountUSDT = "4"
			}
			_, err := SettleSHKeeperTopUp(input)
			failed := scenario == "cap-exceeded" || scenario == "order-save-failure"
			if failed {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if scenario == "legacy-installments" {
					input.ReceivedUSDT = "10"
					input.Transactions = append(input.Transactions, SHKeeperSettlementTransaction{TxID: "tx-b", AmountUSDT: "6"})
					_, err = SettleSHKeeperTopUp(input)
					require.NoError(t, err)
				}
				replay, err := SettleSHKeeperTopUp(input)
				require.NoError(t, err)
				assert.Zero(t, replay.CreditedQuotaDelta)
			}
			require.NoError(t, DB.First(user, user.Id).Error)
			require.NoError(t, DB.First(inviter, inviter.Id).Error)
			require.NoError(t, DB.First(order, order.ID).Error)
			var rewards []AffiliateReward
			require.NoError(t, DB.Where("source_id = ?", order.TradeNo).Find(&rewards).Error)
			var topUp TopUp
			require.NoError(t, DB.First(&topUp, order.TopUpID).Error)
			if failed {
				assert.Equal(t, initial, user.Quota)
				assert.Zero(t, order.CreditedQuota)
				assert.Equal(t, "0", order.ReceivedUSDT)
				assert.Equal(t, common.TopUpStatusPending, topUp.Status)
				var count int64
				require.NoError(t, DB.Model(&SHKeeperCreditedTransaction{}).Where("order_id = ?", order.ID).Count(&count).Error)
				assert.Zero(t, count)
			} else {
				assert.Equal(t, initial+6600, user.Quota)
				assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
			}
			if failed || scenario == "disabled" || scenario == "missing-inviter" {
				assert.Empty(t, rewards)
				assert.Zero(t, inviter.AffQuota)
			} else {
				require.Len(t, rewards, 1)
				assert.Equal(t, AffiliateRewardSourceTopUp, rewards[0].SourceType)
				assert.Equal(t, 6600, rewards[0].CreditedQuota)
				assert.Equal(t, 346, rewards[0].RewardQuota)
				assert.Equal(t, 346, inviter.AffQuota)
				assert.Equal(t, 346, inviter.AffHistoryQuota)
			}
		})
	}
}

func TestSHKeeperLegacyAffiliateCarriesFractionalReward(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	setAffiliateCreditRebateOptions(t, true, 525)
	t.Cleanup(func() {
		setAffiliateCreditRebateOptions(t, false, 0)
		require.NoError(t, DB.Where("source_id = ?", order.TradeNo).Delete(&AffiliateReward{}).Error)
	})
	inviter := &User{Username: "shkeeper-small-affiliate", AffCode: "shkeeper-small-affiliate"}
	require.NoError(t, DB.Create(inviter).Error)
	t.Cleanup(func() { require.NoError(t, DB.Unscoped().Delete(inviter).Error) })
	require.NoError(t, DB.Model(user).Update("inviter_id", inviter.Id).Error)
	require.NoError(t, DB.Model(order).Updates(map[string]any{"settlement_mode": "", "locked_rate": "1"}).Error)
	input := SHKeeperSettlementInput{TradeNo: order.TradeNo, UserID: user.Id, Crypto: order.Crypto, InvoiceAddress: order.InvoiceAddress, ReceivedUSDT: "0.1", ProviderStatus: SHKeeperOrderStatusPartial, Transactions: []SHKeeperSettlementTransaction{{TxID: "small-a", AmountUSDT: "0.1"}}}
	_, err := SettleSHKeeperTopUp(input)
	require.NoError(t, err)
	input.ReceivedUSDT = "0.2"
	input.Transactions = append(input.Transactions, SHKeeperSettlementTransaction{TxID: "small-b", AmountUSDT: "0.1"})
	_, err = SettleSHKeeperTopUp(input)
	require.NoError(t, err)
	require.NoError(t, DB.First(inviter, inviter.Id).Error)
	assert.Equal(t, 1, inviter.AffQuota, "20 total credited quota at 5.25 percent yields one reward quota")
}
