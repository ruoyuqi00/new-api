package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AffiliateRewardSourceTopUp      = "topup"
	AffiliateRewardSourceRedemption = "redemption"
	AffiliateRewardSourceAdminAdd   = "admin_add"
)

var ErrInvalidAffiliateCredit = errors.New("invalid affiliate credit")

type AffiliateReward struct {
	Id               int    `json:"id"`
	SourceType       string `json:"source_type" gorm:"type:varchar(32);uniqueIndex:idx_affiliate_reward_source"`
	SourceId         string `json:"source_id" gorm:"type:varchar(255);uniqueIndex:idx_affiliate_reward_source"`
	InviteeId        int    `json:"invitee_id" gorm:"index"`
	InviterId        int    `json:"inviter_id" gorm:"index"`
	CreditedQuota    int    `json:"credited_quota"`
	RatioBasisPoints int    `json:"ratio_basis_points"`
	RewardQuota      int    `json:"reward_quota"`
	CreatedTime      int64  `json:"created_time" gorm:"index"`
}

func CreditUserQuotaWithAffiliateRewardTx(
	tx *gorm.DB,
	userId int,
	creditedQuota int,
	sourceType string,
	sourceId string,
) (*AffiliateReward, error) {
	if tx == nil || userId <= 0 || creditedQuota <= 0 || strings.TrimSpace(sourceId) == "" {
		return nil, ErrInvalidAffiliateCredit
	}
	switch sourceType {
	case AffiliateRewardSourceTopUp, AffiliateRewardSourceRedemption, AffiliateRewardSourceAdminAdd:
	default:
		return nil, ErrInvalidAffiliateCredit
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").First(&invitee, userId).Error; err != nil {
		return nil, err
	}
	result := tx.Model(&User{}).
		Where("id = ?", userId).
		Update("quota", gorm.Expr("quota + ?", creditedQuota))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}

	basisPoints := common.AffiliateCreditRebateBasisPoints
	if !common.AffiliateCreditRebateEnabled || basisPoints <= 0 || basisPoints > 10_000 {
		return nil, nil
	}
	if invitee.InviterId <= 0 || invitee.InviterId == userId {
		return nil, nil
	}

	var inviter User
	if err := tx.Select("id").First(&inviter, invitee.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	rewardQuota := decimal.NewFromInt(int64(creditedQuota)).
		Mul(decimal.NewFromInt(int64(basisPoints))).
		Div(decimal.NewFromInt(10_000)).
		IntPart()
	if rewardQuota <= 0 {
		return nil, nil
	}

	reward := &AffiliateReward{
		SourceType:       sourceType,
		SourceId:         sourceId,
		InviteeId:        userId,
		InviterId:        inviter.Id,
		CreditedQuota:    creditedQuota,
		RatioBasisPoints: basisPoints,
		RewardQuota:      int(rewardQuota),
		CreatedTime:      common.GetTimestamp(),
	}
	if err := tx.Create(reward).Error; err != nil {
		return nil, err
	}

	result = tx.Model(&User{}).
		Where("id = ?", inviter.Id).
		Updates(map[string]interface{}{
			"aff_quota":   gorm.Expr("aff_quota + ?", reward.RewardQuota),
			"aff_history": gorm.Expr("aff_history + ?", reward.RewardQuota),
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return reward, nil
}
