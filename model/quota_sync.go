package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QuotaSyncCodeStatusActive = 1
	QuotaSyncCodeStatusUsed   = 2
)

const (
	QuotaSyncTokenStatusEnabled  = 1
	QuotaSyncTokenStatusDisabled = 2
)

// QuotaSyncCode is a short-lived, one-time code a YUAPI user can paste into
// UAG to create a quota sync binding.
type QuotaSyncCode struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"index"`
	CodeHash     string `json:"-" gorm:"type:char(64);uniqueIndex"`
	Status       int    `json:"status" gorm:"default:1"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	ExpiresTime  int64  `json:"expires_time" gorm:"bigint"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint"`
	RedeemedBy   string `json:"redeemed_by" gorm:"type:varchar(64)"`
	TokenId      int    `json:"token_id"`
	DeletedAt    gorm.DeletedAt
}

type QuotaSyncToken struct {
	Id             int    `json:"id"`
	UserId         int    `json:"user_id" gorm:"index"`
	TokenHash      string `json:"-" gorm:"type:char(64);uniqueIndex"`
	Status         int    `json:"status" gorm:"default:1"`
	Name           string `json:"name" gorm:"type:varchar(64)"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	AccessedTime   int64  `json:"accessed_time" gorm:"bigint"`
	LastSyncTime   int64  `json:"last_sync_time" gorm:"bigint"`
	LastSyncCodeId int    `json:"last_sync_code_id"`
	DeletedAt      gorm.DeletedAt
}

type QuotaSyncDebit struct {
	Id             int    `json:"id"`
	TokenId        int    `json:"token_id" gorm:"index"`
	UserId         int    `json:"user_id" gorm:"index"`
	ExternalId     string `json:"external_id" gorm:"type:varchar(128);uniqueIndex"`
	Amount         int    `json:"amount"`
	Reason         string `json:"reason" gorm:"type:varchar(255)"`
	Provider       string `json:"provider" gorm:"type:varchar(32)"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	AppliedTime    int64  `json:"applied_time" gorm:"bigint"`
	QuotaBefore    int    `json:"quota_before"`
	QuotaAfter     int    `json:"quota_after"`
	UsedQuotaAfter int    `json:"used_quota_after"`
	DeletedAt      gorm.DeletedAt
}

type QuotaSyncSnapshot struct {
	UserId       int   `json:"user_id"`
	Quota        int   `json:"quota"`
	UsedQuota    int   `json:"used_quota"`
	RequestCount int   `json:"request_count"`
	SyncedTime   int64 `json:"synced_time"`
}

type QuotaSyncRedeemResult struct {
	Token       string `json:"token"`
	UserId      int    `json:"user_id"`
	Quota       int    `json:"quota"`
	UsedQuota   int    `json:"used_quota"`
	ExpiresTime int64  `json:"expires_time"`
}

type QuotaSyncDebitResult struct {
	UserId         int    `json:"user_id"`
	Amount         int    `json:"amount"`
	ExternalId     string `json:"external_id"`
	Duplicate      bool   `json:"duplicate"`
	QuotaBefore    int    `json:"quota_before"`
	QuotaAfter     int    `json:"quota_after"`
	UsedQuotaAfter int    `json:"used_quota_after"`
}

func RedeemQuotaSyncCode(code string, redeemedBy string) (*QuotaSyncRedeemResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("sync code is required")
	}
	if redeemedBy == "" {
		redeemedBy = "external"
	}
	if len(redeemedBy) > 64 {
		redeemedBy = redeemedBy[:64]
	}
	codeHash := quotaSyncHash(code)
	var out *QuotaSyncRedeemResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var row QuotaSyncCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("code_hash = ?", codeHash).First(&row).Error; err != nil {
			return errors.New("invalid sync code")
		}
		now := common.GetTimestamp()
		if row.Status != QuotaSyncCodeStatusActive {
			return errors.New("sync code has already been used")
		}
		if row.ExpiresTime > 0 && row.ExpiresTime < now {
			return errors.New("sync code has expired")
		}

		tokenPlain, err := newQuotaSyncToken()
		if err != nil {
			return err
		}
		token := &QuotaSyncToken{
			UserId:         row.UserId,
			TokenHash:      quotaSyncHash(tokenPlain),
			Status:         QuotaSyncTokenStatusEnabled,
			Name:           "UAG 生图站额度同步",
			CreatedTime:    now,
			AccessedTime:   now,
			LastSyncTime:   now,
			LastSyncCodeId: row.Id,
		}
		if err := tx.Create(token).Error; err != nil {
			return err
		}

		row.Status = QuotaSyncCodeStatusUsed
		row.RedeemedTime = now
		row.RedeemedBy = redeemedBy
		row.TokenId = token.Id
		if err := tx.Save(&row).Error; err != nil {
			return err
		}

		var user User
		if err := tx.Select("id", "quota", "used_quota").Where("id = ?", row.UserId).First(&user).Error; err != nil {
			return err
		}
		out = &QuotaSyncRedeemResult{
			Token:       tokenPlain,
			UserId:      user.Id,
			Quota:       user.Quota,
			UsedQuota:   user.UsedQuota,
			ExpiresTime: row.ExpiresTime,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func GetQuotaSyncSnapshotByToken(token string) (*QuotaSyncSnapshot, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return nil, errors.New("sync token is required")
	}
	var syncToken QuotaSyncToken
	if err := DB.Where("token_hash = ? AND status = ?", quotaSyncHash(token), QuotaSyncTokenStatusEnabled).First(&syncToken).Error; err != nil {
		return nil, errors.New("invalid sync token")
	}
	var user User
	if err := DB.Select("id", "quota", "used_quota", "request_count").Where("id = ?", syncToken.UserId).First(&user).Error; err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	_ = DB.Model(&QuotaSyncToken{}).Where("id = ?", syncToken.Id).Updates(map[string]any{
		"accessed_time":  now,
		"last_sync_time": now,
	}).Error
	return &QuotaSyncSnapshot{
		UserId:       user.Id,
		Quota:        user.Quota,
		UsedQuota:    user.UsedQuota,
		RequestCount: user.RequestCount,
		SyncedTime:   now,
	}, nil
}

func DebitQuotaSyncByToken(token string, amount int, externalId string, reason string) (*QuotaSyncDebitResult, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	externalId = strings.TrimSpace(externalId)
	reason = strings.TrimSpace(reason)
	if token == "" {
		return nil, errors.New("sync token is required")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be greater than 0")
	}
	if externalId == "" {
		return nil, errors.New("external_id is required")
	}
	if len(externalId) > 128 {
		return nil, errors.New("external_id is too long")
	}
	if len(reason) > 255 {
		reason = reason[:255]
	}
	var out *QuotaSyncDebitResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var syncToken QuotaSyncToken
		if err := tx.Where("token_hash = ? AND status = ?", quotaSyncHash(token), QuotaSyncTokenStatusEnabled).First(&syncToken).Error; err != nil {
			return errors.New("invalid sync token")
		}

		var existing QuotaSyncDebit
		err := tx.Where("external_id = ?", externalId).First(&existing).Error
		if err == nil {
			if existing.TokenId != syncToken.Id || existing.UserId != syncToken.UserId || existing.Amount != amount {
				return errors.New("external_id has already been used with different debit data")
			}
			out = &QuotaSyncDebitResult{
				UserId:         existing.UserId,
				Amount:         existing.Amount,
				ExternalId:     existing.ExternalId,
				Duplicate:      true,
				QuotaBefore:    existing.QuotaBefore,
				QuotaAfter:     existing.QuotaAfter,
				UsedQuotaAfter: existing.UsedQuotaAfter,
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "quota", "used_quota").Where("id = ?", syncToken.UserId).First(&user).Error; err != nil {
			return err
		}
		if user.Quota < amount {
			return fmt.Errorf("YUAPI quota is insufficient")
		}
		before := user.Quota
		after := before - amount
		usedAfter := user.UsedQuota + amount
		if err := tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
			"quota":         after,
			"used_quota":    usedAfter,
			"request_count": gorm.Expr("request_count + ?", 1),
		}).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		row := &QuotaSyncDebit{
			TokenId:        syncToken.Id,
			UserId:         user.Id,
			ExternalId:     externalId,
			Amount:         amount,
			Reason:         reason,
			Provider:       "uag",
			CreatedTime:    now,
			AppliedTime:    now,
			QuotaBefore:    before,
			QuotaAfter:     after,
			UsedQuotaAfter: usedAfter,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		_ = tx.Model(&QuotaSyncToken{}).Where("id = ?", syncToken.Id).Updates(map[string]any{
			"accessed_time":  now,
			"last_sync_time": now,
		}).Error
		out = &QuotaSyncDebitResult{
			UserId:         user.Id,
			Amount:         amount,
			ExternalId:     externalId,
			Duplicate:      false,
			QuotaBefore:    before,
			QuotaAfter:     after,
			UsedQuotaAfter: usedAfter,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out != nil && !out.Duplicate {
		_ = cacheDecrUserQuota(out.UserId, int64(out.Amount))
		RecordTaskBillingLog(RecordTaskBillingLogParams{
			UserId:    out.UserId,
			LogType:   LogTypeConsume,
			Content:   fmt.Sprintf("UAG 同步补扣 %s", logger.LogQuota(out.Amount)),
			ModelName: "uag-quota-sync",
			Quota:     out.Amount,
			Other: map[string]interface{}{
				"external_id": out.ExternalId,
				"provider":    "uag",
			},
		})
	}
	return out, nil
}

func newQuotaSyncToken() (string, error) {
	secret, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return "", err
	}
	return "nqsrt_" + secret, nil
}

func quotaSyncHash(value string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(h[:])
}
