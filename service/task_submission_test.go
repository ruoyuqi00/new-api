package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistTaskSubmissionIntentRejectsDifferentRequestBeforeBilling(t *testing.T) {
	truncate(t)
	platform := constant.TaskPlatform("cangyuan")
	first := newSubmissionIntentTestInfo("task_unique_intent")
	second := newSubmissionIntentTestInfo("task_unique_intent")

	inserted, err := PersistTaskSubmissionIntent(first, platform)
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = PersistTaskSubmissionIntent(second, platform)
	require.ErrorContains(t, err, "already exists")
	assert.False(t, inserted)
	assert.False(t, second.TaskRelayInfo.SubmissionIntentPersisted)

	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("submission_key = ?", first.PublicTaskID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestRejectedSubmissionDeletesOnlyOwnedPendingIntent(t *testing.T) {
	truncate(t)
	info := newSubmissionIntentTestInfo("task_rejected_intent")
	_, err := PersistTaskSubmissionIntent(info, constant.TaskPlatform("cangyuan"))
	require.NoError(t, err)

	require.NoError(t, DeleteTaskSubmissionIntent(info))
	assert.False(t, info.TaskRelayInfo.SubmissionIntentPersisted)
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("submission_key = ?", info.PublicTaskID).Count(&count).Error)
	assert.Zero(t, count)
}

func TestTaskSubmissionIntentTracksPossibleUpstreamWrite(t *testing.T) {
	truncate(t)
	platform := constant.TaskPlatform("cangyuan")
	info := newSubmissionIntentTestInfo("task_write_possible")
	_, err := PersistTaskSubmissionIntent(info, platform)
	require.NoError(t, err)
	_, err = PersistTaskSubmissionIntent(info, platform)
	require.NoError(t, err)

	require.NoError(t, MarkTaskSubmissionWritePossible(info))
	var submitted model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", info.PublicTaskID).First(&submitted).Error)
	require.NotNil(t, submitted.SubmissionStage)
	assert.Equal(t, model.TaskSubmissionStageWritePossible, *submitted.SubmissionStage)

	require.NoError(t, CompleteTaskSubmission(info, platform, model.TaskStatusNotStart, 300_000, "upstream-id", nil))
	var completed model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", info.PublicTaskID).First(&completed).Error)
	assert.EqualValues(t, model.TaskStatusNotStart, completed.Status)
	assert.Equal(t, "upstream-id", completed.PrivateData.UpstreamTaskID)
	require.NotNil(t, completed.SubmissionStage)
	assert.Equal(t, model.TaskSubmissionStageCompleted, *completed.SubmissionStage)
}

func TestTaskSubmissionRetryReturnsWritePossibleIntentToPreWriteState(t *testing.T) {
	truncate(t)
	platform := constant.TaskPlatform("cangyuan")
	info := newSubmissionIntentTestInfo("task_retry_prewrite")
	_, err := PersistTaskSubmissionIntent(info, platform)
	require.NoError(t, err)
	require.NoError(t, MarkTaskSubmissionWritePossible(info))
	require.NoError(t, MarkTaskSubmissionRejected(info))

	retryPlatform := constant.TaskPlatform("alternate-provider")
	info.ChannelMeta.ChannelId = 202
	_, err = PersistTaskSubmissionIntent(info, retryPlatform)
	require.NoError(t, err)
	_, err = PersistTaskSubmissionIntent(info, retryPlatform)
	require.NoError(t, err)
	var refreshed model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", info.PublicTaskID).First(&refreshed).Error)
	require.NotNil(t, refreshed.SubmissionStage)
	assert.Equal(t, model.TaskSubmissionStagePreWrite, *refreshed.SubmissionStage)
	assert.Equal(t, retryPlatform, refreshed.Platform)
	assert.Equal(t, 202, refreshed.ChannelId)

	require.NoError(t, MarkTaskSubmissionWritePossible(info))
	require.NoError(t, MarkTaskSubmissionRejected(info))
	var rejected model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", info.PublicTaskID).First(&rejected).Error)
	require.NotNil(t, rejected.SubmissionStage)
	assert.Equal(t, model.TaskSubmissionStageRejected, *rejected.SubmissionStage)
	require.NoError(t, DeleteTaskSubmissionIntent(info))
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("submission_key = ?", info.PublicTaskID).Count(&count).Error)
	assert.Zero(t, count)
}

func newSubmissionIntentTestInfo(taskID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: taskID, Action: "generate"},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 201, UpstreamModelName: "provider-model"},
		UserId:          101,
		UsingGroup:      "default",
		OriginModelName: "public-model",
		PriceData: types.PriceData{
			ModelPrice:     0.5,
			Quota:          300_000,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.2},
		},
	}
}
