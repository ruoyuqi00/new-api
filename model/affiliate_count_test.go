package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffiliateCountTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	originalInviterQuota := common.QuotaForInviter
	originalInviteeQuota := common.QuotaForInvitee
	originalNewUserQuota := common.QuotaForNewUser
	paymentSetting := operation_setting.GetPaymentSetting()
	originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
	originalComplianceVersion := paymentSetting.ComplianceTermsVersion
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	common.QuotaForNewUser = 0
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
	require.NoError(t, DB.Where(&Option{Key: affiliateCountReconciledOptionKey}).Delete(&Option{}).Error)
	t.Cleanup(func() {
		common.QuotaForInviter = originalInviterQuota
		common.QuotaForInvitee = originalInviteeQuota
		common.QuotaForNewUser = originalNewUserQuota
		paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = originalComplianceVersion
		require.NoError(t, DB.Where(&Option{Key: affiliateCountReconciledOptionKey}).Delete(&Option{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	})
}

func createAffiliateCountUser(t *testing.T, username string, affCode string, inviterId int) *User {
	t.Helper()
	user := &User{
		Username:  username,
		AffCode:   affCode,
		InviterId: inviterId,
		Status:    common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestInsertPersistsInvitationAndCountWithoutFixedReward(t *testing.T) {
	setupAffiliateCountTest(t)
	inviter := createAffiliateCountUser(t, "count-inviter", "count-inviter-code", 0)
	invitee := &User{Username: "count-invitee", Status: common.UserStatusEnabled}

	require.NoError(t, invitee.Insert(inviter.Id))

	assert.Equal(t, inviter.Id, getAffiliateRewardUser(t, invitee.Id).InviterId)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 1, updatedInviter.AffCount)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
	var rewardCount int64
	require.NoError(t, DB.Model(&AffiliateReward{}).Count(&rewardCount).Error)
	assert.Zero(t, rewardCount)
}

func TestInsertWithTxPersistsOAuthInvitationAndCount(t *testing.T) {
	setupAffiliateCountTest(t)
	inviter := createAffiliateCountUser(t, "oauth-count-inviter", "oauth-count-inviter-code", 0)
	invitee := &User{Username: "oauth-count-invitee", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return invitee.InsertWithTx(tx, inviter.Id)
	}))

	invitee.FinalizeOAuthUserCreation(inviter.Id)

	assert.Equal(t, inviter.Id, getAffiliateRewardUser(t, invitee.Id).InviterId)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 1, updatedInviter.AffCount)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
}

func TestInsertWithTxRollsBackWhenInviterCannotBeUpdated(t *testing.T) {
	setupAffiliateCountTest(t)
	invitee := &User{Username: "missing-count-inviter", Status: common.UserStatusEnabled}

	err := DB.Transaction(func(tx *gorm.DB) error {
		return invitee.InsertWithTx(tx, 999_999)
	})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var userCount int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", invitee.Username).Count(&userCount).Error)
	assert.Zero(t, userCount)
}

func TestInsertAppliesFixedInvitationRewardsWithoutCreatingRebate(t *testing.T) {
	setupAffiliateCountTest(t)
	common.QuotaForNewUser = 100
	common.QuotaForInvitee = 200
	common.QuotaForInviter = 300
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = "v1"
	inviter := createAffiliateCountUser(t, "fixed-reward-inviter", "fixed-reward-code", 0)
	invitee := &User{Username: "fixed-reward-invitee", Status: common.UserStatusEnabled}

	require.NoError(t, invitee.Insert(inviter.Id))

	updatedInvitee := getAffiliateRewardUser(t, invitee.Id)
	assert.Equal(t, inviter.Id, updatedInvitee.InviterId)
	assert.Equal(t, 300, updatedInvitee.Quota)
	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 1, updatedInviter.AffCount)
	assert.Equal(t, 300, updatedInviter.AffQuota)
	assert.Equal(t, 300, updatedInviter.AffHistoryQuota)
	var rewardCount int64
	require.NoError(t, DB.Model(&AffiliateReward{}).Count(&rewardCount).Error)
	assert.Zero(t, rewardCount)
}

func TestReconcileAffiliateCountsUsesActiveInvitationBindings(t *testing.T) {
	setupAffiliateCountTest(t)
	firstInviter := createAffiliateCountUser(t, "first-count-inviter", "first-count-code", 0)
	secondInviter := createAffiliateCountUser(t, "second-count-inviter", "second-count-code", 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", firstInviter.Id).Update("aff_count", 99).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", secondInviter.Id).Update("aff_count", 99).Error)

	createAffiliateCountUser(t, "first-active-one", "first-active-one-code", firstInviter.Id)
	createAffiliateCountUser(t, "first-active-two", "first-active-two-code", firstInviter.Id)
	createAffiliateCountUser(t, "second-active", "second-active-code", secondInviter.Id)
	deletedInvitee := createAffiliateCountUser(t, "deleted-invitee", "deleted-invitee-code", firstInviter.Id)
	require.NoError(t, DB.Delete(deletedInvitee).Error)

	require.NoError(t, ReconcileAffiliateCounts())
	require.NoError(t, ReconcileAffiliateCounts())

	assert.Equal(t, 2, getAffiliateRewardUser(t, firstInviter.Id).AffCount)
	assert.Equal(t, 1, getAffiliateRewardUser(t, secondInviter.Id).AffCount)
}

func TestReconcileAffiliateCountsRunsOnlyOnce(t *testing.T) {
	setupAffiliateCountTest(t)
	inviter := createAffiliateCountUser(t, "once-count-inviter", "once-count-code", 0)
	createAffiliateCountUser(t, "once-count-invitee", "once-invitee-code", inviter.Id)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviter.Id).Update("aff_count", 99).Error)

	require.NoError(t, ReconcileAffiliateCounts())
	assert.Equal(t, 1, getAffiliateRewardUser(t, inviter.Id).AffCount)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviter.Id).Update("aff_count", 77).Error)
	require.NoError(t, ReconcileAffiliateCounts())
	assert.Equal(t, 77, getAffiliateRewardUser(t, inviter.Id).AffCount)
}
