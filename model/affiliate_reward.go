package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
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

var (
	ErrInvalidAffiliateCredit             = errors.New("invalid affiliate credit")
	ErrInvalidAffiliateCreditRebateConfig = errors.New("invalid affiliate credit rebate configuration")
	ErrAffiliateCreditQuotaLimit          = errors.New("user quota limit exceeded")
)

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
	return creditUserQuotaWithAffiliateRewardTx(tx, userId, creditedQuota, sourceType, sourceId, affiliateCreditOptions{})
}

type affiliateCreditOptions struct {
	maxQuota int64
	// Legacy SHKeeper invoices can credit multiple confirmed installments.
	// Their one stable source row accumulates quota with cumulative rounding.
	accumulate bool
}

func creditUserQuotaWithAffiliateRewardTx(tx *gorm.DB, userId, creditedQuota int, sourceType, sourceId string, options affiliateCreditOptions) (*AffiliateReward, error) {
	if tx == nil || userId <= 0 || creditedQuota <= 0 || strings.TrimSpace(sourceId) == "" {
		return nil, ErrInvalidAffiliateCredit
	}
	switch sourceType {
	case AffiliateRewardSourceTopUp, AffiliateRewardSourceRedemption, AffiliateRewardSourceAdminAdd:
	default:
		return nil, ErrInvalidAffiliateCredit
	}

	rebateEnabled, basisPoints, err := getAffiliateCreditRebateConfigTx(tx)
	if err != nil {
		return nil, err
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").First(&invitee, userId).Error; err != nil {
		return nil, err
	}
	query := tx.Model(&User{}).Where("id = ?", userId)
	if options.maxQuota > 0 {
		if int64(creditedQuota) > options.maxQuota {
			return nil, ErrAffiliateCreditQuotaLimit
		}
		query = query.Where("quota <= ?", options.maxQuota-int64(creditedQuota))
	}
	result := query.Update("quota", gorm.Expr("quota + ?", creditedQuota))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		if options.maxQuota > 0 {
			return nil, ErrAffiliateCreditQuotaLimit
		}
		return nil, gorm.ErrRecordNotFound
	}

	if !rebateEnabled {
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

	sourceHash := sha256.Sum256([]byte(sourceId))
	sourceKey := hex.EncodeToString(sourceHash[:])
	var previous AffiliateReward
	if options.accumulate {
		err := lockForUpdate(tx).Where("source_type = ? AND source_key = ?", sourceType, sourceKey).First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			if previous.InviteeId != userId || previous.InviterId != inviter.Id {
				return nil, ErrInvalidAffiliateCredit
			}
			basisPoints = previous.RatioBasisPoints
		}
	}
	totalCredited := previous.CreditedQuota + creditedQuota
	rewardQuota := decimal.NewFromInt(int64(totalCredited)).
		Mul(decimal.NewFromInt(int64(basisPoints))).
		Div(decimal.NewFromInt(10_000)).
		IntPart()
	rewardDelta := int(rewardQuota) - previous.RewardQuota
	if rewardDelta <= 0 && previous.Id == 0 && !options.accumulate {
		return nil, nil
	}

	reward := &AffiliateReward{
		SourceType:       sourceType,
		SourceKey:        sourceKey,
		SourceId:         sourceId,
		InviteeId:        userId,
		InviterId:        inviter.Id,
		CreditedQuota:    totalCredited,
		RatioBasisPoints: basisPoints,
		RewardQuota:      int(rewardQuota),
		CreatedTime:      common.GetTimestamp(),
	}
	if previous.Id > 0 {
		reward.Id = previous.Id
		reward.CreatedTime = previous.CreatedTime
		if err := tx.Save(reward).Error; err != nil {
			return nil, err
		}
	} else {
		if err := tx.Create(reward).Error; err != nil {
			return nil, err
		}
	}
	if rewardDelta == 0 {
		return nil, nil
	}

	result = tx.Model(&User{}).
		Where("id = ?", inviter.Id).
		Updates(map[string]interface{}{
			"aff_quota":   gorm.Expr("aff_quota + ?", rewardDelta),
			"aff_history": gorm.Expr("aff_history + ?", rewardDelta),
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	// The committed row is cumulative; the audit log describes only this credit.
	reward.CreditedQuota = creditedQuota
	reward.RewardQuota = rewardDelta
	return reward, nil
}

func getAffiliateCreditRebateConfigTx(tx *gorm.DB) (bool, int, error) {
	var options []Option
	if err := tx.Where(commonKeyCol+" IN ?", []string{
		"AffiliateCreditRebateEnabled",
		"AffiliateCreditRebateBasisPoints",
	}).Find(&options).Error; err != nil {
		return false, 0, err
	}

	enabled := false
	basisPoints := 0
	for _, option := range options {
		switch option.Key {
		case "AffiliateCreditRebateEnabled":
			parsed, err := strconv.ParseBool(strings.TrimSpace(option.Value))
			if err != nil {
				return false, 0, ErrInvalidAffiliateCreditRebateConfig
			}
			enabled = parsed
		case "AffiliateCreditRebateBasisPoints":
			parsed, err := strconv.Atoi(strings.TrimSpace(option.Value))
			if err != nil {
				return false, 0, ErrInvalidAffiliateCreditRebateConfig
			}
			basisPoints = parsed
		}
	}
	if basisPoints < 0 || basisPoints > 10_000 || (enabled && basisPoints == 0) {
		return false, 0, ErrInvalidAffiliateCreditRebateConfig
	}
	return enabled, basisPoints, nil
}
