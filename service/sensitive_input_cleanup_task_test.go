package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitiveInputCleanupTaskConfiguration(t *testing.T) {
	originalDays := setting.SensitiveInputRetentionDays
	setting.SensitiveInputRetentionDays = 7
	t.Cleanup(func() { setting.SensitiveInputRetentionDays = originalDays })

	handler := sensitiveInputCleanupHandler{}
	assert.Equal(t, model.SystemTaskTypeSensitiveInputCleanup, handler.Type())
	assert.True(t, handler.Enabled())
	assert.Equal(t, 24*time.Hour, handler.Interval())
	assert.Equal(t, SensitiveInputCleanupPayload{
		RetentionDays: 7,
		BatchSize:     sensitiveInputCleanupBatchSize,
	}, handler.NewPayload())
}

func TestSensitiveInputCleanupTaskDrainsBatchesAndKeepsOnlyCounts(t *testing.T) {
	truncate(t)
	now := common.GetTimestamp()
	logs := []model.Log{
		{
			UserId:    991,
			CreatedAt: now - 10*24*60*60,
			Type:      model.LogTypeConsume,
			Content:   "first blocked input",
			Other: common.MapToJsonStr(map[string]any{
				"violation_fee_reason": model.SensitiveInputViolationReason,
				"sensitive_words":      []string{"first"},
			}),
		},
		{
			UserId:    992,
			CreatedAt: now - 9*24*60*60,
			Type:      model.LogTypeConsume,
			Content:   "second blocked input",
			Other: common.MapToJsonStr(map[string]any{
				"violation_fee_reason": model.SensitiveInputViolationReason,
				"sensitive_words":      []string{"second"},
			}),
		},
	}
	require.NoError(t, model.LOG_DB.Create(&logs).Error)

	task, err := model.CreateSystemTask(
		model.SystemTaskTypeSensitiveInputCleanup,
		SensitiveInputCleanupPayload{RetentionDays: 7, BatchSize: 1},
		SensitiveInputCleanupState{},
	)
	require.NoError(t, err)
	claimedTask, claimed, err := model.ClaimSystemTask(task.ID, task.Type, "cleanup-runner", now+60)
	require.NoError(t, err)
	require.True(t, claimed)

	sensitiveInputCleanupHandler{}.Run(context.Background(), claimedTask, "cleanup-runner")

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	var result SensitiveInputCleanupResult
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	assert.Equal(t, int64(2), result.PurgedCount)
	assert.NotContains(t, finished.State, "blocked input")
	assert.NotContains(t, finished.Result, "blocked input")

	var saved []model.Log
	require.NoError(t, model.LOG_DB.Order("user_id asc").Find(&saved).Error)
	require.Len(t, saved, 2)
	for _, log := range saved {
		assert.Equal(t, model.SensitiveInputBlockedLogContent, log.Content)
		assert.NotContains(t, log.Other, "sensitive_words")
	}
}

func TestStartSensitiveInputCleanupTaskReusesActiveTask(t *testing.T) {
	truncate(t)

	first, err := StartSensitiveInputCleanupTask()
	require.NoError(t, err)
	second, err := StartSensitiveInputCleanupTask()
	require.NoError(t, err)
	assert.Equal(t, first.TaskID, second.TaskID)
}
