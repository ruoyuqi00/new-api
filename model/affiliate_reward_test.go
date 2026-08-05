package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffiliateRewardTest(t *testing.T) (inviter *User, invitee *User) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AffiliateReward{}, &Option{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateReward{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	require.NoError(t, DB.Where("key IN ?", []string{
		"AffiliateCreditRebateEnabled",
		"AffiliateCreditRebateBasisPoints",
	}).Delete(&Option{}).Error)

	originalEnabled := common.AffiliateCreditRebateEnabled
	originalBasisPoints := common.AffiliateCreditRebateBasisPoints
	t.Cleanup(func() {
		common.AffiliateCreditRebateEnabled = originalEnabled
		common.AffiliateCreditRebateBasisPoints = originalBasisPoints
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateReward{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
		require.NoError(t, DB.Where("key IN ?", []string{
			"AffiliateCreditRebateEnabled",
			"AffiliateCreditRebateBasisPoints",
		}).Delete(&Option{}).Error)
	})

	inviter = &User{Username: "affiliate-inviter", Status: common.UserStatusEnabled, AffCode: "aff-inviter"}
	require.NoError(t, DB.Create(inviter).Error)
	invitee = &User{Username: "affiliate-invitee", Status: common.UserStatusEnabled, AffCode: "aff-invitee", InviterId: inviter.Id}
	require.NoError(t, DB.Create(invitee).Error)
	return inviter, invitee
}

func setAffiliateCreditRebateOptions(t *testing.T, enabled bool, basisPoints int) {
	t.Helper()
	values := map[string]string{
		"AffiliateCreditRebateEnabled":     strconv.FormatBool(enabled),
		"AffiliateCreditRebateBasisPoints": strconv.Itoa(basisPoints),
	}
	for key, value := range values {
		option := Option{Key: key}
		require.NoError(t, DB.FirstOrCreate(&option, Option{Key: key}).Error)
		option.Value = value
		require.NoError(t, DB.Save(&option).Error)
	}
	common.AffiliateCreditRebateEnabled = enabled
	common.AffiliateCreditRebateBasisPoints = basisPoints
}

func getAffiliateRewardUser(t *testing.T, userId int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	return user
}

func TestCreditUserQuotaWithAffiliateRewardTxAwardsConfiguredPercentage(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, true, 525)

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

func TestCreditUserQuotaWithAffiliateRewardTxReadsCommittedDatabaseConfiguration(t *testing.T) {
	_, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, true, 525)
	common.AffiliateCreditRebateEnabled = false
	common.AffiliateCreditRebateBasisPoints = 0

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			10_000,
			AffiliateRewardSourceTopUp,
			"database-config-enabled",
		)
		return err
	}))

	require.NotNil(t, reward)
	assert.Equal(t, 525, reward.RewardQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxIgnoresStaleEnabledCache(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, false, 500)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 500

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			10_000,
			AffiliateRewardSourceTopUp,
			"database-config-disabled",
		)
		return err
	}))

	assert.Nil(t, reward)
	assert.Zero(t, getAffiliateRewardUser(t, inviter.Id).AffQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxRollsBackInvalidDatabaseConfiguration(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	require.NoError(t, DB.Create(&Option{
		Key:   "AffiliateCreditRebateEnabled",
		Value: "true",
	}).Error)
	require.NoError(t, DB.Create(&Option{
		Key:   "AffiliateCreditRebateBasisPoints",
		Value: "invalid",
	}).Error)
	common.AffiliateCreditRebateEnabled = true
	common.AffiliateCreditRebateBasisPoints = 500

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, creditErr := CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			10_000,
			AffiliateRewardSourceTopUp,
			"invalid-database-config",
		)
		return creditErr
	})

	require.Error(t, err)
	assert.Zero(t, getAffiliateRewardUser(t, invitee.Id).Quota)
	assert.Zero(t, getAffiliateRewardUser(t, inviter.Id).AffQuota)
}

func TestCreditUserQuotaWithAffiliateRewardTxSkipsDisabledReward(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, false, 500)

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
	setAffiliateCreditRebateOptions(t, true, 1)

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
	setAffiliateCreditRebateOptions(t, true, 500)

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
	setAffiliateCreditRebateOptions(t, true, 500)

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

func TestCreditUserQuotaWithAffiliateRewardTxHashesLongSourceId(t *testing.T) {
	_, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, true, 1_000)
	sourceId := strings.Repeat("a", 255)

	var reward *AffiliateReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			invitee.Id,
			10_000,
			AffiliateRewardSourceTopUp,
			sourceId,
		)
		return err
	}))

	require.NotNil(t, reward)
	assert.Equal(t, sourceId, reward.SourceId)
	assert.Len(t, reward.SourceKey, 64)
}

func TestIncreaseUserQuotaDoesNotCreateAffiliateReward(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, true, 1_000)

	require.NoError(t, IncreaseUserQuota(invitee.Id, 10_000, true))

	assert.Equal(t, 10_000, getAffiliateRewardUser(t, invitee.Id).Quota)
	assert.Zero(t, getAffiliateRewardUser(t, inviter.Id).AffQuota)
	var rewardCount int64
	require.NoError(t, DB.Model(&AffiliateReward{}).Count(&rewardCount).Error)
	assert.Zero(t, rewardCount)
}

func TestTransferAffQuotaToQuotaDoesNotCreateAffiliateReward(t *testing.T) {
	inviter, invitee := setupAffiliateRewardTest(t)
	setAffiliateCreditRebateOptions(t, true, 1_000)
	transferQuota := int(common.QuotaPerUnit)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", invitee.Id).Update("aff_quota", transferQuota).Error)

	require.NoError(t, invitee.TransferAffQuotaToQuota(transferQuota))

	updatedInvitee := getAffiliateRewardUser(t, invitee.Id)
	assert.Equal(t, transferQuota, updatedInvitee.Quota)
	assert.Zero(t, updatedInvitee.AffQuota)
	assert.Zero(t, getAffiliateRewardUser(t, inviter.Id).AffQuota)
	var rewardCount int64
	require.NoError(t, DB.Model(&AffiliateReward{}).Count(&rewardCount).Error)
	assert.Zero(t, rewardCount)
}
