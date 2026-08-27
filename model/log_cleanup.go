package model

import (
	"context"
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// LogCleanupUsageDelta is the historical usage represented by old consume
// logs. It is snapshotted before those logs are removed so counters can be
// reduced exactly once during cleanup recovery.
type LogCleanupUsageDelta struct {
	UserQuota    map[int]int64 `json:"user_quota,omitempty"`
	UserRequests map[int]int64 `json:"user_requests,omitempty"`
	TokenQuota   map[int]int64 `json:"token_quota,omitempty"`
	ChannelQuota map[int]int64 `json:"channel_quota,omitempty"`
}

type logCleanupUserDelta struct {
	UserID   int   `gorm:"column:user_id"`
	Quota    int64 `gorm:"column:quota"`
	Requests int64 `gorm:"column:requests"`
}

type logCleanupEntityDelta struct {
	ID    int   `gorm:"column:id"`
	Quota int64 `gorm:"column:quota"`
}

// AggregateOldConsumeLogDeltas returns counter adjustments for consume logs
// older than cutoff. Non-consume logs and logs at the cutoff are excluded.
func AggregateOldConsumeLogDeltas(ctx context.Context, cutoff int64) (LogCleanupUsageDelta, error) {
	delta := LogCleanupUsageDelta{
		UserQuota:    make(map[int]int64),
		UserRequests: make(map[int]int64),
		TokenQuota:   make(map[int]int64),
		ChannelQuota: make(map[int]int64),
	}

	var users []logCleanupUserDelta
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("user_id, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS requests").
		Where("type = ? AND created_at < ?", LogTypeConsume, cutoff).
		Group("user_id").Find(&users).Error; err != nil {
		return delta, err
	}
	for _, row := range users {
		if row.UserID <= 0 {
			continue
		}
		delta.UserQuota[row.UserID] = row.Quota
		delta.UserRequests[row.UserID] = row.Requests
	}

	var tokens []logCleanupEntityDelta
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("token_id AS id, COALESCE(SUM(quota), 0) AS quota").
		Where("type = ? AND created_at < ? AND token_id > 0", LogTypeConsume, cutoff).
		Group("token_id").Find(&tokens).Error; err != nil {
		return delta, err
	}
	for _, row := range tokens {
		delta.TokenQuota[row.ID] = row.Quota
	}

	var channels []logCleanupEntityDelta
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("channel_id AS id, COALESCE(SUM(quota), 0) AS quota").
		Where("type = ? AND created_at < ? AND channel_id > 0", LogTypeConsume, cutoff).
		Group("channel_id").Find(&channels).Error; err != nil {
		return delta, err
	}
	for _, row := range channels {
		delta.ChannelQuota[row.ID] = row.Quota
	}

	return delta, nil
}

// ApplyLogCleanupUsageAdjustment applies a cleanup snapshot in the same
// database transaction as its durable marker. Replays therefore become a
// no-op, while each counter is updated directly instead of entering the
// asynchronous batch-update queue.
func ApplyLogCleanupUsageAdjustment(taskID, runnerID string, delta LogCleanupUsageDelta) error {
	if taskID == "" || runnerID == "" {
		return errors.New("system task identity is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var task SystemTask
		if err := lockForUpdate(tx).Where(
			"task_id = ? AND type = ? AND status = ? AND locked_by = ?",
			taskID, SystemTaskTypeLogCleanup, SystemTaskStatusRunning, runnerID,
		).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSystemTaskLockLost
			}
			return err
		}

		now := common.GetTimestamp()
		var lock SystemTaskLock
		if err := tx.Where("task_id = ? AND locked_by = ? AND locked_until >= ?", taskID, runnerID, now).First(&lock).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSystemTaskLockLost
			}
			return err
		}

		state := map[string]any{}
		if task.State != "" {
			if err := common.UnmarshalJsonStr(task.State, &state); err != nil {
				return err
			}
			if state == nil {
				state = map[string]any{}
			}
		}
		if applied, ok := state["usage_adjustment_applied"].(bool); ok && applied {
			return nil
		}

		for _, id := range sortedPositiveIntKeys(delta.UserQuota) {
			amount := delta.UserQuota[id]
			if amount <= 0 {
				continue
			}
			if err := decrementCounter(tx, &User{}, "id = ?", id, "used_quota", amount); err != nil {
				return err
			}
		}
		for _, id := range sortedPositiveIntKeys(delta.UserRequests) {
			amount := delta.UserRequests[id]
			if amount <= 0 {
				continue
			}
			if err := decrementCounter(tx, &User{}, "id = ?", id, "request_count", amount); err != nil {
				return err
			}
		}
		for _, id := range sortedPositiveIntKeys(delta.TokenQuota) {
			amount := delta.TokenQuota[id]
			if amount <= 0 {
				continue
			}
			if err := decrementCounter(tx, &Token{}, "id = ?", id, "used_quota", amount); err != nil {
				return err
			}
		}
		for _, id := range sortedPositiveIntKeys(delta.ChannelQuota) {
			amount := delta.ChannelQuota[id]
			if amount <= 0 {
				continue
			}
			if err := decrementCounter(tx, &Channel{}, "id = ?", id, "used_quota", amount); err != nil {
				return err
			}
		}

		state["usage_adjustment_applied"] = true
		stateText, err := common.Marshal(state)
		if err != nil {
			return err
		}
		result := tx.Model(&SystemTask{}).Where("id = ?", task.ID).Updates(map[string]any{
			"state":      string(stateText),
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSystemTaskLockLost
		}
		return nil
	})
}

func decrementCounter(tx *gorm.DB, model any, where string, id int, column string, amount int64) error {
	result := tx.Model(model).Where(where, id).Update(column,
		gorm.Expr("CASE WHEN "+column+" >= ? THEN "+column+" - ? ELSE 0 END", amount, amount),
	)
	return result.Error
}

func sortedPositiveIntKeys(values map[int]int64) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		if key > 0 {
			keys = append(keys, key)
		}
	}
	sort.Ints(keys)
	return keys
}
