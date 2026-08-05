package model

import (
	"errors"

	"gorm.io/gorm"
)

const affiliateCountReconciledOptionKey = "AffiliateCountReconciledV1"

type affiliateInvitationCount struct {
	InviterId int
	Count     int64
}

func ReconcileAffiliateCounts() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		if err := tx.Where(&Option{Key: affiliateCountReconciledOptionKey}).First(&marker).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var groupedCounts []affiliateInvitationCount
		if err := tx.Model(&User{}).
			Select("inviter_id, COUNT(*) AS count").
			Where("inviter_id > ?", 0).
			Group("inviter_id").
			Scan(&groupedCounts).Error; err != nil {
			return err
		}

		expectedByInviter := make(map[int]int, len(groupedCounts))
		for _, groupedCount := range groupedCounts {
			expectedByInviter[groupedCount.InviterId] = int(groupedCount.Count)
		}

		var users []User
		if err := tx.Select("id", "aff_count").Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			expected := expectedByInviter[user.Id]
			if user.AffCount == expected {
				continue
			}
			if err := tx.Model(&User{}).
				Where("id = ?", user.Id).
				Update("aff_count", expected).Error; err != nil {
				return err
			}
		}
		return tx.Create(&Option{
			Key:   affiliateCountReconciledOptionKey,
			Value: "true",
		}).Error
	})
}
