package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
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
	SourceType       string `json:"source_type" gorm:"type:varchar(32);uniqueIndex:idx_affiliate_reward_source_key"`
	SourceKey        string `json:"-" gorm:"type:char(64);uniqueIndex:idx_affiliate_reward_source_key"`
	SourceId         string `json:"source_id" gorm:"type:varchar(255)"`
	InviteeId        int    `json:"invitee_id" gorm:"index"`
	InviterId        int    `json:"inviter_id" gorm:"index"`
	CreditedQuota    int    `json:"credited_quota"`
	RatioBasisPoints int    `json:"ratio_basis_points"`
	RewardQuota      int    `json:"reward_quota"`
	CreatedTime      int64  `json:"created_time" gorm:"index"`
}

func AddUserQuotaWithAffiliateReward(userId int, quota int, eventId string) (*AffiliateReward, error) {
	var reward *AffiliateReward
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = CreditUserQuotaWithAffiliateRewardTx(
			tx,
			userId,
			quota,
			AffiliateRewardSourceAdminAdd,
			eventId,
		)
		return err
	})
	return reward, err
}

func RecordAffiliateRewardLog(reward *AffiliateReward) {
	if reward == nil {
		return
	}
	RecordLog(
		reward.InviterId,
		LogTypeSystem,
		fmt.Sprintf(
			"Affiliate reward from %s: invited user %d credited %s, reward %s",
			reward.SourceType,
			reward.InviteeId,
			logger.LogQuota(reward.CreditedQuota),
			logger.LogQuota(reward.RewardQuota),
		),
	)
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
		if invitee.InviterId == userId {
			common.SysLog(fmt.Sprintf("affiliate reward skipped: invitee_id=%d inviter_id=%d", userId, invitee.InviterId))
		}
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

	sourceHash := sha256.Sum256([]byte(sourceId))
	reward := &AffiliateReward{
		SourceType:       sourceType,
		SourceKey:        hex.EncodeToString(sourceHash[:]),
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
