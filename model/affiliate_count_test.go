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
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	originalInviterQuota := common.QuotaForInviter
	originalInviteeQuota := common.QuotaForInvitee
	paymentSetting := operation_setting.GetPaymentSetting()
	originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
	originalComplianceVersion := paymentSetting.ComplianceTermsVersion
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		common.QuotaForInviter = originalInviterQuota
		common.QuotaForInvitee = originalInviteeQuota
		paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = originalComplianceVersion
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

func TestFinishInsertIncrementsInviteCountWithoutFixedReward(t *testing.T) {
	setupAffiliateCountTest(t)
	inviter := createAffiliateCountUser(t, "count-inviter", "count-inviter-code", 0)
	invitee := createAffiliateCountUser(t, "count-invitee", "count-invitee-code", inviter.Id)

	invitee.finishInsert(inviter.Id)

	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 1, updatedInviter.AffCount)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
}

func TestFinalizeOAuthUserCreationIncrementsInviteCountWithoutFixedReward(t *testing.T) {
	setupAffiliateCountTest(t)
	inviter := createAffiliateCountUser(t, "oauth-count-inviter", "oauth-count-inviter-code", 0)
	invitee := createAffiliateCountUser(t, "oauth-count-invitee", "oauth-count-invitee-code", inviter.Id)

	invitee.FinalizeOAuthUserCreation(inviter.Id)

	updatedInviter := getAffiliateRewardUser(t, inviter.Id)
	assert.Equal(t, 1, updatedInviter.AffCount)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
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
