package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateOldConsumeLogDeltasIgnoresNewAndNonConsumeRows(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	cutoff := now - 100
	logs := []Log{
		{UserId: 1, CreatedAt: cutoff - 1, Type: LogTypeConsume, Quota: 100, TokenId: 11, ChannelId: 21},
		{UserId: 1, CreatedAt: cutoff - 2, Type: LogTypeConsume, Quota: 50, TokenId: 11, ChannelId: 21},
		{UserId: 2, CreatedAt: cutoff - 3, Type: LogTypeConsume, Quota: 75, TokenId: 12, ChannelId: 22},
		{UserId: 1, CreatedAt: cutoff - 4, Type: LogTypeError, Quota: 999, TokenId: 11, ChannelId: 21},
		{UserId: 1, CreatedAt: cutoff + 1, Type: LogTypeConsume, Quota: 1000, TokenId: 11, ChannelId: 21},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	delta, err := AggregateOldConsumeLogDeltas(t.Context(), cutoff)
	require.NoError(t, err)
	assert.Equal(t, map[int]int64{1: 150, 2: 75}, delta.UserQuota)
	assert.Equal(t, map[int]int64{1: 2, 2: 1}, delta.UserRequests)
	assert.Equal(t, map[int]int64{11: 150, 12: 75}, delta.TokenQuota)
	assert.Equal(t, map[int]int64{21: 150, 22: 75}, delta.ChannelQuota)
}

func TestDeleteOldQuotaDataBatchAndPruneCache(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	cutoff := now - 100
	require.NoError(t, DB.Create(&[]QuotaData{
		{UserID: 1, CreatedAt: cutoff - 1, Count: 1, Quota: 10},
		{UserID: 1, CreatedAt: cutoff, Count: 1, Quota: 20},
	}).Error)

	CacheQuotaDataLock.Lock()
	CacheQuotaData = map[string]*QuotaData{
		"old":       {CreatedAt: cutoff - 1},
		"at-cutoff": {CreatedAt: cutoff},
	}
	CacheQuotaDataLock.Unlock()

	PruneQuotaDataCacheBefore(cutoff)
	CacheQuotaDataLock.Lock()
	_, oldPresent := CacheQuotaData["old"]
	_, cutoffPresent := CacheQuotaData["at-cutoff"]
	CacheQuotaDataLock.Unlock()
	assert.False(t, oldPresent)
	assert.True(t, cutoffPresent)

	deleted, err := DeleteOldQuotaDataBatch(t.Context(), cutoff, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var remaining []QuotaData
	require.NoError(t, DB.Order("created_at asc").Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, cutoff, remaining[0].CreatedAt)
}

func TestApplyLogCleanupUsageAdjustmentIsIdempotent(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 101, Username: "cleanup-user", Password: "password", Status: common.UserStatusEnabled, Quota: 1000, UsedQuota: 1000, RequestCount: 10}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Create(&Token{Id: 111, UserId: user.Id, Key: "cleanup-token", Name: "cleanup", Status: common.TokenStatusEnabled, RemainQuota: 500, UsedQuota: 500}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 121, Key: "cleanup-channel", Name: "cleanup", Status: common.ChannelStatusEnabled, UsedQuota: 700}).Error)

	task, err := CreateSystemTask(SystemTaskTypeLogCleanup, LogCleanupPayloadForModelTest{}, map[string]any{"usage_adjustment_applied": false})
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(task.ID, task.Type, "cleanup-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	delta := LogCleanupUsageDelta{
		UserQuota:    map[int]int64{user.Id: 150},
		UserRequests: map[int]int64{user.Id: 2},
		TokenQuota:   map[int]int64{111: 150},
		ChannelQuota: map[int]int64{121: 150},
	}
	require.NoError(t, ApplyLogCleanupUsageAdjustment(task.TaskID, "cleanup-runner", delta))
	require.NoError(t, ApplyLogCleanupUsageAdjustment(task.TaskID, "cleanup-runner", delta))

	var gotUser User
	require.NoError(t, DB.First(&gotUser, user.Id).Error)
	assert.Equal(t, 850, gotUser.UsedQuota)
	assert.Equal(t, 8, gotUser.RequestCount)
	var gotToken Token
	require.NoError(t, DB.First(&gotToken, 111).Error)
	assert.Equal(t, 350, gotToken.UsedQuota)
	var gotChannel Channel
	require.NoError(t, DB.First(&gotChannel, 121).Error)
	assert.Equal(t, int64(550), gotChannel.UsedQuota)

	var stored SystemTask
	require.NoError(t, DB.First(&stored, task.ID).Error)
	var state map[string]any
	require.NoError(t, common.Unmarshal([]byte(stored.State), &state))
	assert.Equal(t, true, state["usage_adjustment_applied"])
}

func TestRequeueFailedLogCleanupPreservesAppliedState(t *testing.T) {
	truncateTables(t)

	task, err := CreateSystemTask(SystemTaskTypeLogCleanup, LogCleanupPayloadForModelTest{}, map[string]any{
		"usage_adjustment_applied": true,
	})
	require.NoError(t, err)
	_, claimed, err := ClaimSystemTask(task.ID, task.Type, "cleanup-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, FinishSystemTask(task.TaskID, "cleanup-runner", SystemTaskStatusFailed, nil, "delete failed"))

	requeued, err := RequeueFailedLogCleanupTask(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, requeued)
	assert.Equal(t, SystemTaskStatusPending, requeued.Status)
	assert.Equal(t, "", requeued.Error)
	require.NotNil(t, requeued.ActiveKey)
	assert.Equal(t, SystemTaskTypeLogCleanup, *requeued.ActiveKey)
	var state map[string]any
	require.NoError(t, common.UnmarshalJsonStr(requeued.State, &state))
	assert.Equal(t, true, state["usage_adjustment_applied"])
}

// LogCleanupPayloadForModelTest keeps this model-package test independent of
// the service package's payload type.
type LogCleanupPayloadForModelTest struct{}
