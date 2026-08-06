package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffiliateRewardTest(t *testing.T) (inviter *User, invitee *User) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AffiliateReward{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateReward{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)

	originalEnabled := common.AffiliateCreditRebateEnabled
	originalBasisPoints := common.AffiliateCreditRebateBasisPoints
	t.Cleanup(func() {
		common.AffiliateCreditRebateEnabled = originalEnabled
		common.AffiliateCreditRebateBasisPoints = originalBasisPoints
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateReward{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	})

	inviter = &User{Username: "affiliate-inviter", Status: common.UserStatusEnabled, AffCode: "aff-inviter"}
	require.NoError(t, DB.Create(inviter).Error)
	invitee = &User{Username: "affiliate-invitee", Status: common.UserStatusEnabled, AffCode: "aff-invitee", InviterId: inviter.Id}
	require.NoError(t, DB.Create(invitee).Error)
	return inviter, invitee
}

func getAffiliateRewardUser(t *testing.T, userId int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	return user
}

func TestCreditUserQuotaWithAffiliateRewardTxAwardsConfiguredPercentage(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 525

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			10_000,
			AffiliateRewardSourceTopUp,
			"order-1",
		)
		return err
	}))

	require.NotNil(t, reward)
	assert.Equal(t, 10_000, reward.CreditedQuota)
	assert.Equal(t, 525, reward.RatioBasisPoints)
	assert.Equal(t, 525, reward.RewardQuota)
	assert.Equal(t, 10_000, getAffiliateRewardUser(t, invitee.Id).Quota)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 525, updatedInviter.AffQuota)
	assert.Equal(t, 525, updatedInviter.AffHistoryQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxSkipsDisabledReward(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	common.AffiliateCreditRebateEnabled = false
	common.AffiliateCreditRebateBasisPoints = 500

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			2_000,
			AffiliateRewardSourceTopUp,
			"order-disabled",
		)
		return err
	}))

	assert.Nil(t, reward)
	assert.Equal(t, 2_000, getAffiliateRewardUser(t, invitee.Id).Quota)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxSkipsRewardBelowOneQuota(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 1

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			9_999,
			AffiliateRewardSourceRedemption,
			"redemption-small",
		)
		return err
	}))

	assert.Nil(t, reward)
	assert.Equal(t, 9_999, getAffiliateRewardUser(t, invitee.Id).Quota)
	assert.Zero(t, getAffiliateRewardUser(t, inviter.Id).AffQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxSkipsMissingInviter(t *testing.T) {
	_, invitee := setupAffiliateRewardTest(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", invitee.Id).Update("inviter_id", 999_999).Error)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 500

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			1_000,
			AffiliateRewardSourceAdminAdd,
			"admin-missing-inviter",
		)
		return err
	}))

	assert.Nil(t, reward)
	assert.Equal(t, 1_000, getAffiliateRewardUser(t, invitee.Id).Quota)
}

func TestCreditUserQuotaWithAffiliateRewardTxDuplicateSourceRollsBackCredit(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 500

	credit := func() error {
		return DB.Transaction(func(tx *gorm.DB) error {
			_, err := CreditUserQuotaWithAffiliateRewardTx(
				tx,
				invitee.Id,
				1_000,
				AffiliateRewardSourceTopUp,
				"duplicate-order",
			)
			return err
		})
	}

	require.NoError(t, credit())
	require.Error(t, credit())
	assert.Equal(t, 1_000, getAffiliateRewardUser(t, invitee.Id).Quota)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 50, updatedInviter.AffQuota)
	assert.Equal(t, 50, updatedInviter.AffHistoryQuota)

	var rewardCount int64
	require.NoError(t, DB.Model(&AffiliateReward{}).Count(&rewardCount).Error)
	assert.EqualValues(t, 1, rewardCount)
}
