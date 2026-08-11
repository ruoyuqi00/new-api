package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type recordingControllerBilling struct {
	settled []int
	refunds int
}

func (b *recordingControllerBilling) Settle(quota int) error {
	b.settled = append(b.settled, quota)
	return nil
}
func (b *recordingControllerBilling) Refund(*gin.Context)      { b.refunds++ }
func (b *recordingControllerBilling) NeedsRefund() bool        { return true }
func (b *recordingControllerBilling) GetPreConsumedQuota() int { return 300_000 }
func (b *recordingControllerBilling) Reserve(int) error        { return nil }

func TestShouldRetryTaskRelayDoesNotRetryUnprocessableEntity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	retry := shouldRetryTaskRelay(c, 2360, &dto.TaskError{StatusCode: http.StatusUnprocessableEntity}, 2)

	require.False(t, retry)
}

func TestAmbiguousTaskSubmissionPersistsUnknownTaskWithFrozenBilling(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123", Action: "generate"},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 17, UpstreamModelName: "internal-seedance"},
		UserId:          23,
		UsingGroup:      "premium",
		OriginModelName: "seedance-2.5-720p",
		BillingSource:   "wallet",
		TokenId:         31,
		PriceData: types.PriceData{
			ModelPrice:     0.35,
			Quota:          1_050_000,
			OtherRatios:    map[string]float64{"seconds": 5},
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.2},
		},
	}

	require.NoError(t, persistTaskSubmission(info, constant.TaskPlatform("cangyuan"), model.TaskStatusUnknown, 1_050_000, "", json.RawMessage(nil)))
	require.NoError(t, persistTaskSubmission(info, constant.TaskPlatform("cangyuan"), model.TaskStatusUnknown, 1_050_000, "", json.RawMessage(nil)))
	assert.True(t, db.Migrator().HasIndex(&model.Task{}, "idx_tasks_submission_key"))
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_public_123").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusUnknown, task.Status)
	assert.Equal(t, 1_050_000, task.Quota)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.Equal(t, 0.35, task.PrivateData.BillingContext.ModelPrice)
	assert.Equal(t, 1.2, task.PrivateData.BillingContext.GroupRatio)
	assert.Equal(t, map[string]float64{"seconds": 5}, task.PrivateData.BillingContext.OtherRatios)

	taskErr := (&dto.TaskError{}).WithSubmissionState(dto.TaskSubmissionAmbiguous)
	setUnknownTaskSubmissionData(taskErr, info.PublicTaskID)
	assert.Equal(t, map[string]any{"task_id": "task_public_123", "submission_state": "unknown"}, taskErr.Data)

	info.PriceData.Quota = 999_999
	require.Error(t, persistTaskSubmission(info, constant.TaskPlatform("cangyuan"), model.TaskStatusUnknown, 999_999, "", nil))
	info.PriceData.Quota = 1_050_000
	require.Error(t, persistTaskSubmission(info, constant.TaskPlatform("cangyuan"), model.TaskStatusNotStart, 1_050_000, "upstream-id", nil))
	var unchanged model.Task
	require.NoError(t, db.Where("submission_key = ?", "task_public_123").First(&unchanged).Error)
	assert.EqualValues(t, model.TaskStatusUnknown, unchanged.Status)
	assert.Empty(t, unchanged.PrivateData.UpstreamTaskID)
}

func TestTaskSubmissionKeyMigrationPreservesLegacyDuplicateTaskIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))

	legacyA := &model.Task{TaskID: "legacy-duplicate", UserId: 1}
	legacyB := &model.Task{TaskID: "legacy-duplicate", UserId: 1}
	require.NoError(t, db.Create(legacyA).Error)
	require.NoError(t, db.Create(legacyB).Error)

	submissionKey := "task_new_submission"
	first := &model.Task{TaskID: "task_a", UserId: 1, SubmissionKey: &submissionKey}
	second := &model.Task{TaskID: "task_b", UserId: 2, SubmissionKey: &submissionKey}
	require.NoError(t, db.Create(first).Error)
	require.Error(t, db.Create(second).Error)
}

func TestTaskPersistenceFailurePreventsTerminalBilling(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	billing := &recordingControllerBilling{}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_untracked", Action: "generate"},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7},
		UserId:          9,
		UsingGroup:      "default",
		OriginModelName: "paid-model",
		Billing:         billing,
		PriceData:       types.PriceData{Quota: 300_000},
	}
	taskErr := (&dto.TaskError{}).WithSubmissionState(dto.TaskSubmissionAmbiguous)

	err = persistAndFinalizeTaskSubmission(nil, info, constant.TaskPlatform("cangyuan"), model.TaskStatusUnknown, 300_000, "", nil, taskErr)
	require.Error(t, err)
	assert.Empty(t, billing.settled)
	assert.Zero(t, billing.refunds)
}

func TestRetryTaskRelayHonorsSubmissionStateBeforeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	tests := []struct {
		name  string
		state dto.TaskSubmissionState
		want  bool
	}{
		{name: "not sent follows transport retry rules", state: dto.TaskSubmissionNotSent, want: true},
		{name: "ambiguous is never retried", state: dto.TaskSubmissionAmbiguous, want: false},
		{name: "accepted is never retried", state: dto.TaskSubmissionAccepted, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskErr := (&dto.TaskError{StatusCode: http.StatusInternalServerError}).WithSubmissionState(tt.state)
			require.Equal(t, tt.want, shouldRetryTaskRelay(c, 2360, taskErr, 2))
		})
	}
}
