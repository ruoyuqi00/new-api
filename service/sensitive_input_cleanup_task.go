package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

const sensitiveInputCleanupBatchSize = 200

type SensitiveInputCleanupPayload struct {
	RetentionDays int `json:"retention_days"`
	BatchSize     int `json:"batch_size"`
}

type SensitiveInputCleanupState struct {
	PurgedCount int64 `json:"purged_count"`
	Batches     int   `json:"batches"`
	Progress    int   `json:"progress"`
}

type SensitiveInputCleanupResult struct {
	PurgedCount int64 `json:"purged_count"`
}

type sensitiveInputCleanupHandler struct{}

func (sensitiveInputCleanupHandler) Type() string {
	return model.SystemTaskTypeSensitiveInputCleanup
}

func (sensitiveInputCleanupHandler) Enabled() bool {
	return setting.SensitiveInputRetentionDays > 0
}

func (sensitiveInputCleanupHandler) Interval() time.Duration {
	return 24 * time.Hour
}

func (sensitiveInputCleanupHandler) NewPayload() any {
	return SensitiveInputCleanupPayload{
		RetentionDays: setting.SensitiveInputRetentionDays,
		BatchSize:     sensitiveInputCleanupBatchSize,
	}
}

func (sensitiveInputCleanupHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := SensitiveInputCleanupPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if _, err := setting.ParseSensitiveInputRetentionDays(strconv.Itoa(payload.RetentionDays)); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if payload.BatchSize <= 0 {
		payload.BatchSize = sensitiveInputCleanupBatchSize
	}

	state := SensitiveInputCleanupState{}
	if err := task.DecodeState(&state); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	purgedAt := common.GetTimestamp()
	cutoff := purgedAt - int64(payload.RetentionDays*24*60*60)

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		batch, err := model.PurgeExpiredSensitiveInputAuditBatch(ctx, cutoff, payload.BatchSize, purgedAt)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			failSystemTask(task, runnerID, err)
			return
		}
		if batch.Purged == 0 {
			break
		}
		state.PurgedCount += batch.Purged
		state.Batches++
		if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
			logSystemTaskLockError(ctx, task, err)
			return
		}
	}

	state.Progress = 100
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		logSystemTaskLockError(ctx, task, err)
		return
	}
	result := SensitiveInputCleanupResult{PurgedCount: state.PurgedCount}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

func StartSensitiveInputCleanupTask() (*model.SystemTask, error) {
	handler := sensitiveInputCleanupHandler{}
	task, _, err := EnqueueSystemTask(handler.Type(), handler.NewPayload())
	return task, err
}

func init() {
	RegisterSystemTaskHandler(sensitiveInputCleanupHandler{})
}
