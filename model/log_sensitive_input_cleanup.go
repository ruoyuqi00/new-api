package model

import (
	"context"

	"github.com/QuantumNous/new-api/common"
)

type SensitiveInputCleanupBatchResult struct {
	Scanned int64
	Purged  int64
}

func PurgeExpiredSensitiveInputAuditBatch(
	ctx context.Context,
	cutoff int64,
	batchSize int,
	purgedAt int64,
) (SensitiveInputCleanupBatchResult, error) {
	result := SensitiveInputCleanupBatchResult{}
	if cutoff <= 0 || purgedAt <= 0 {
		return result, nil
	}
	if batchSize <= 0 {
		batchSize = 200
	}

	var logs []Log
	err := LOG_DB.WithContext(ctx).
		Where("type = ? AND created_at < ?", LogTypeConsume, cutoff).
		Where("other LIKE ?", "%\"violation_fee_reason\":\""+SensitiveInputViolationReason+"\"%").
		Where("content <> ? OR other LIKE ?", SensitiveInputBlockedLogContent, "%\"sensitive_words\"%").
		Order("id asc").
		Limit(batchSize).
		Find(&logs).Error
	if err != nil {
		return result, err
	}
	result.Scanned = int64(len(logs))

	for i := range logs {
		other, err := common.StrToMap(logs[i].Other)
		if err != nil || other["violation_fee_reason"] != SensitiveInputViolationReason {
			continue
		}
		delete(other, "sensitive_words")
		other["sensitive_input_purged"] = true
		other["sensitive_input_purged_at"] = purgedAt

		update := LOG_DB.WithContext(ctx).
			Model(&Log{}).
			Where("id = ?", logs[i].Id).
			Updates(map[string]any{
				"content": SensitiveInputBlockedLogContent,
				"other":   common.MapToJsonStr(other),
			})
		if update.Error != nil {
			return result, update.Error
		}
		result.Purged += update.RowsAffected
	}

	return result, nil
}
