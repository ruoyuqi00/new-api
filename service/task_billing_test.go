package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
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

type recordingTaskBillingSettler struct {
	preConsumed int
	settled     []int
	refunds     int
	settleErr   error
}

func (s *recordingTaskBillingSettler) Settle(quota int) error {
	s.settled = append(s.settled, quota)
	return s.settleErr
}
func (s *recordingTaskBillingSettler) Refund(*gin.Context) { s.refunds++ }
func (s *recordingTaskBillingSettler) NeedsRefund() bool {
	return s.refunds == 0 && len(s.settled) == 0
}
func (s *recordingTaskBillingSettler) GetPreConsumedQuota() int {
	return s.preConsumed
}
func (s *recordingTaskBillingSettler) Reserve(int) error { return nil }

func TestTaskSubmissionRefundability(t *testing.T) {
	tests := []struct {
		state dto.TaskSubmissionState
		want  bool
	}{
		{state: dto.TaskSubmissionNotSent, want: true},
		{state: dto.TaskSubmissionRejected, want: true},
		{state: dto.TaskSubmissionAmbiguous, want: false},
		{state: dto.TaskSubmissionAccepted, want: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			taskErr := (&dto.TaskError{}).WithSubmissionState(tt.state)
			assert.Equal(t, tt.want, ShouldRefundTaskSubmission(taskErr))
		})
	}
}

func TestTaskErrorSubmissionStateDefaultsAndOverrides(t *testing.T) {
	var nilTaskErr *dto.TaskError
	assert.Equal(t, dto.TaskSubmissionNotSent, nilTaskErr.SubmissionState())
	assert.False(t, ShouldRefundTaskSubmission(nilTaskErr))

	direct := &dto.TaskError{LocalError: true}
	assert.Equal(t, dto.TaskSubmissionNotSent, direct.SubmissionState())
	assert.True(t, ShouldRefundTaskSubmission(direct))
	assert.Equal(t, dto.TaskSubmissionAccepted, direct.WithSubmissionState(dto.TaskSubmissionAccepted).SubmissionState())

	wrapped := TaskErrorWrapper(errors.New("local failure"), "local_failure", http.StatusInternalServerError)
	assert.Equal(t, dto.TaskSubmissionNotSent, wrapped.SubmissionState())
	local := TaskErrorWrapperLocal(errors.New("controller failure"), "controller_failure", http.StatusBadRequest)
	assert.Equal(t, dto.TaskSubmissionNotSent, local.SubmissionState())
	fromAPI := TaskErrorFromAPIError(types.NewErrorWithStatusCode(errors.New("preconsume failure"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError))
	assert.Equal(t, dto.TaskSubmissionNotSent, fromAPI.SubmissionState())
}

func TestTaskSubmissionStateDoesNotLeakThroughJSON(t *testing.T) {
	taskErr := (&dto.TaskError{Code: "upstream_error", Message: "safe"}).WithSubmissionState(dto.TaskSubmissionAmbiguous)
	payload, err := common.Marshal(taskErr)
	require.NoError(t, err)
	assert.JSONEq(t, `{"code":"upstream_error","message":"safe","data":null}`, string(payload))
}

func TestFinalizeRetainedTaskSubmissionSettlesFrozenQuotaOnce(t *testing.T) {
	for _, state := range []dto.TaskSubmissionState{dto.TaskSubmissionAmbiguous, dto.TaskSubmissionAccepted} {
		t.Run(string(state), func(t *testing.T) {
			truncate(t)
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(nil)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			billing := &recordingTaskBillingSettler{preConsumed: 300_000}
			info := &relaycommon.RelayInfo{
				TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "generate"},
				Billing:         billing,
				UserId:          101,
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 201},
				TokenId:         301,
				UsingGroup:      "default",
				OriginModelName: "veo-3.1-fast",
				PriceData: types.PriceData{
					ModelPrice:     0.5,
					Quota:          300_000,
					GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.2},
				},
			}
			taskErr := (&dto.TaskError{}).WithSubmissionState(state)

			require.NoError(t, FinalizeTaskSubmissionBilling(c, info, taskErr, 0))
			require.NoError(t, FinalizeTaskSubmissionBilling(c, info, taskErr, 0))

			assert.Equal(t, []int{300_000}, billing.settled)
			assert.Zero(t, billing.refunds)
			assert.Equal(t, int64(1), countLogs(t))
		})
	}
}

func TestFinalizePersistedTaskSubmissionBillingRepeatedCallSettlesAndLogsOnce(t *testing.T) {
	tests := []struct {
		name    string
		taskErr *dto.TaskError
	}{
		{name: "successful submission"},
		{name: "ambiguous submission", taskErr: (&dto.TaskError{}).WithSubmissionState(dto.TaskSubmissionAmbiguous)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, 101, 0)
			seedChannel(t, 201)
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(nil)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

			submissionKey := "task_durable_billing_" + strings.ReplaceAll(tt.name, " ", "_")
			pending := model.TaskSubmissionBillingPending
			completed := model.TaskSubmissionStageCompleted
			require.NoError(t, model.DB.Create(&model.Task{
				SubmissionKey:          &submissionKey,
				SubmissionBillingState: &pending,
				SubmissionStage:        &completed,
				TaskID:                 submissionKey,
				Status:                 model.TaskStatusUnknown,
			}).Error)

			billing := &recordingTaskBillingSettler{preConsumed: 300_000}
			info := newPersistedTaskBillingTestInfo(submissionKey, billing)
			info.TaskRelayInfo.SubmissionIntentPersisted = true
			require.NoError(t, FinalizePersistedTaskSubmissionBilling(c, info, tt.taskErr, 300_000))
			require.NoError(t, FinalizePersistedTaskSubmissionBilling(c, info, tt.taskErr, 300_000))

			assert.Equal(t, []int{300_000}, billing.settled)
			assert.Zero(t, billing.refunds)
			assert.Equal(t, int64(1), countLogs(t))
			var task model.Task
			require.NoError(t, model.DB.Where("submission_key = ?", submissionKey).First(&task).Error)
			require.NotNil(t, task.SubmissionBillingState)
			assert.Equal(t, model.TaskSubmissionBillingFinalized, *task.SubmissionBillingState)
		})
	}
}

func TestFinalizePersistedTaskSubmissionBillingConcurrentSameRequestSettlesOnce(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	gin.SetMode(gin.TestMode)

	submissionKey := "task_durable_billing_concurrent"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusNotStart,
	}).Error)

	const goroutines = 8
	billing := &recordingTaskBillingSettler{preConsumed: 300_000}
	info := newPersistedTaskBillingTestInfo(submissionKey, billing)
	info.TaskRelayInfo.SubmissionIntentPersisted = true
	errs := make(chan error, goroutines)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range goroutines {
		c, _ := gin.CreateTestContext(nil)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- FinalizePersistedTaskSubmissionBilling(c, info, nil, 300_000)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Len(t, billing.settled, 1)

	var task model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", submissionKey).First(&task).Error)
	require.NotNil(t, task.SubmissionBillingState)
	assert.Equal(t, model.TaskSubmissionBillingFinalized, *task.SubmissionBillingState)
}

func TestFinalizePersistedTaskSubmissionBillingConcurrentProcessesHaveOneDurableClaimant(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeClickHouse)
	t.Cleanup(func() {
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	})

	submissionKey := "task_durable_billing_cross_process"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusNotStart,
	}).Error)

	firstBilling := &recordingTaskBillingSettler{preConsumed: 300_000}
	secondBilling := &recordingTaskBillingSettler{preConsumed: 300_000}
	infos := []*relaycommon.RelayInfo{
		newPersistedTaskBillingTestInfo(submissionKey, firstBilling),
		newPersistedTaskBillingTestInfo(submissionKey, secondBilling),
	}
	for _, info := range infos {
		info.TaskRelayInfo.SubmissionIntentPersisted = true
	}

	start := make(chan struct{})
	errs := make(chan error, len(infos))
	var wg sync.WaitGroup
	for _, info := range infos {
		info := info
		c, _ := gin.CreateTestContext(nil)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- FinalizePersistedTaskSubmissionBilling(c, info, nil, 300_000)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, 1, len(firstBilling.settled)+len(secondBilling.settled))
	assert.Equal(t, int64(1), countLogs(t), "ClickHouse count-then-insert must have only one durable writer")
}

func newPersistedTaskBillingTestInfo(submissionKey string, billing *recordingTaskBillingSettler) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: submissionKey, Action: "generate"},
		Billing:         billing,
		UserId:          101,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 201},
		TokenId:         301,
		UsingGroup:      "default",
		OriginModelName: "veo-3.1-fast",
		PriceData: types.PriceData{
			ModelPrice:     0.5,
			Quota:          300_000,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.2},
		},
	}
}

func TestFinalizePersistedTaskSubmissionBillingSettlementErrorRemainsClaimed(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	submissionKey := "task_durable_billing_error"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusUnknown,
	}).Error)
	billing := &recordingTaskBillingSettler{preConsumed: 300_000, settleErr: errors.New("settlement unavailable")}
	info := newPersistedTaskBillingTestInfo(submissionKey, billing)
	taskErr := (&dto.TaskError{}).WithSubmissionState(dto.TaskSubmissionAmbiguous)

	info.TaskRelayInfo.SubmissionIntentPersisted = true
	require.Error(t, FinalizePersistedTaskSubmissionBilling(c, info, taskErr, 300_000))
	unfinished, err := model.GetUnfinishedTaskSubmissionBillings(10)
	require.NoError(t, err)
	require.Len(t, unfinished, 1)
	require.NotNil(t, unfinished[0].SubmissionBillingState)
	assert.Equal(t, model.TaskSubmissionBillingFinalizing, *unfinished[0].SubmissionBillingState)
	assert.Equal(t, []int{300_000}, billing.settled)
	assert.Zero(t, countLogs(t))
}

func TestFinalizePersistedTaskSubmissionBillingLogErrorRemainsClaimed(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	submissionKey := "task_durable_billing_log_error"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusUnknown,
	}).Error)
	billing := &recordingTaskBillingSettler{preConsumed: 300_000}
	info := newPersistedTaskBillingTestInfo(submissionKey, billing)
	info.TaskRelayInfo.SubmissionIntentPersisted = true

	originalLogDB := model.LOG_DB
	brokenLogDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.LOG_DB = brokenLogDB
	t.Cleanup(func() { model.LOG_DB = originalLogDB })

	require.Error(t, FinalizePersistedTaskSubmissionBilling(c, info, nil, 300_000))
	var task model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", submissionKey).First(&task).Error)
	require.NotNil(t, task.SubmissionBillingState)
	assert.Equal(t, model.TaskSubmissionBillingFinalizing, *task.SubmissionBillingState)
}

func TestFinalizePersistedTaskSubmissionBillingUsageIsTransactionalAndIdempotent(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	submissionKey := "task_durable_usage_once"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusNotStart,
	}).Error)
	billing := &recordingTaskBillingSettler{preConsumed: 300_000}
	info := newPersistedTaskBillingTestInfo(submissionKey, billing)
	info.TaskRelayInfo.SubmissionIntentPersisted = true

	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = true
	t.Cleanup(func() { common.BatchUpdateEnabled = originalBatchUpdateEnabled })
	require.NoError(t, FinalizePersistedTaskSubmissionBilling(c, info, nil, 300_000))

	var user model.User
	require.NoError(t, model.DB.First(&user, 101).Error)
	assert.Equal(t, 300_000, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, 201).Error)
	assert.Equal(t, int64(300_000), channel.UsedQuota)
	assert.Equal(t, int64(1), countLogs(t))

	restarted := newPersistedTaskBillingTestInfo(submissionKey, &recordingTaskBillingSettler{preConsumed: 300_000})
	restarted.TaskRelayInfo.SubmissionIntentPersisted = true
	require.NoError(t, FinalizePersistedTaskSubmissionBilling(c, restarted, nil, 300_000))
	assert.Empty(t, restarted.Billing.(*recordingTaskBillingSettler).settled)
	require.NoError(t, model.DB.First(&user, 101).Error)
	assert.Equal(t, 300_000, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	require.NoError(t, model.DB.First(&channel, 201).Error)
	assert.Equal(t, int64(300_000), channel.UsedQuota)
	assert.Equal(t, int64(1), countLogs(t))
}

func TestFinalizePersistedTaskSubmissionBillingLeavesPartialUsageForOperatorWithoutReexecution(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	submissionKey := "task_durable_partial_usage"
	pending := model.TaskSubmissionBillingPending
	completed := model.TaskSubmissionStageCompleted
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		SubmissionStage:        &completed,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusNotStart,
	}).Error)

	firstBilling := &recordingTaskBillingSettler{preConsumed: 300_000}
	first := newPersistedTaskBillingTestInfo(submissionKey, firstBilling)
	first.TaskRelayInfo.SubmissionIntentPersisted = true
	require.Error(t, FinalizePersistedTaskSubmissionBilling(c, first, nil, 300_000))
	assert.Equal(t, int64(1), countLogs(t))
	var user model.User
	require.NoError(t, model.DB.First(&user, 101).Error)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)

	seedChannel(t, 201)
	restartedBilling := &recordingTaskBillingSettler{preConsumed: 300_000}
	restarted := newPersistedTaskBillingTestInfo(submissionKey, restartedBilling)
	restarted.TaskRelayInfo.SubmissionIntentPersisted = true
	require.NoError(t, FinalizePersistedTaskSubmissionBilling(c, restarted, nil, 300_000))
	assert.Equal(t, int64(1), countLogs(t))
	assert.Empty(t, restartedBilling.settled)
	require.NoError(t, model.DB.First(&user, 101).Error)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, 201).Error)
	assert.Zero(t, channel.UsedQuota)

	var task model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", submissionKey).First(&task).Error)
	require.NotNil(t, task.SubmissionBillingState)
	assert.Equal(t, model.TaskSubmissionBillingFinalizing, *task.SubmissionBillingState)
}

func TestFinalizePersistedTaskSubmissionBillingRejectsQuotaAdjustment(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	submissionKey := "task_durable_billing_adjustment"
	pending := model.TaskSubmissionBillingPending
	require.NoError(t, model.DB.Create(&model.Task{
		SubmissionKey:          &submissionKey,
		SubmissionBillingState: &pending,
		TaskID:                 submissionKey,
		Status:                 model.TaskStatusNotStart,
	}).Error)
	billing := &recordingTaskBillingSettler{preConsumed: 300_000}
	info := newPersistedTaskBillingTestInfo(submissionKey, billing)

	info.TaskRelayInfo.SubmissionIntentPersisted = true
	err := FinalizePersistedTaskSubmissionBilling(c, info, nil, 300_001)
	require.ErrorContains(t, err, "does not match")
	assert.Empty(t, billing.settled)
	assert.Zero(t, countLogs(t))

	var task model.Task
	require.NoError(t, model.DB.Where("submission_key = ?", submissionKey).First(&task).Error)
	require.NotNil(t, task.SubmissionBillingState)
	assert.Equal(t, model.TaskSubmissionBillingPending, *task.SubmissionBillingState)
}

func TestFinalizeRefundableTaskSubmissionRefundsWithoutSettlement(t *testing.T) {
	for _, state := range []dto.TaskSubmissionState{dto.TaskSubmissionRejected, dto.TaskSubmissionNotSent} {
		t.Run(string(state), func(t *testing.T) {
			billing := &recordingTaskBillingSettler{preConsumed: 1_050_000}
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, Billing: billing}
			taskErr := (&dto.TaskError{}).WithSubmissionState(state)

			require.NoError(t, FinalizeTaskSubmissionBilling(nil, info, taskErr, 0))

			assert.Empty(t, billing.settled)
			assert.Equal(t, 1, billing.refunds)
		})
	}
}

func TestFrozenTaskSubmissionQuotaNeverRetainsZeroForPaidSubmission(t *testing.T) {
	tests := []struct {
		name string
		info *relaycommon.RelayInfo
		want int
	}{
		{
			name: "billing session reservation",
			info: &relaycommon.RelayInfo{Billing: &recordingTaskBillingSettler{preConsumed: 300_000}, PriceData: types.PriceData{Quota: 100}},
			want: 300_000,
		},
		{
			name: "below cost session reservation",
			info: &relaycommon.RelayInfo{Billing: &recordingTaskBillingSettler{preConsumed: 1}, FinalPreConsumedQuota: 2, PriceData: types.PriceData{Quota: 300_000}},
			want: 300_000,
		},
		{
			name: "negative values are ignored",
			info: &relaycommon.RelayInfo{Billing: &recordingTaskBillingSettler{preConsumed: -3}, FinalPreConsumedQuota: -2, PriceData: types.PriceData{Quota: -1}},
			want: 0,
		},
		{
			name: "maximum integer is preserved",
			info: &relaycommon.RelayInfo{Billing: &recordingTaskBillingSettler{preConsumed: math.MaxInt}, PriceData: types.PriceData{Quota: 300_000}},
			want: math.MaxInt,
		},
		{
			name: "legacy frozen reservation",
			info: &relaycommon.RelayInfo{FinalPreConsumedQuota: 1_050_000, PriceData: types.PriceData{Quota: 999_999}},
			want: 1_050_000,
		},
		{
			name: "paid defensive fallback",
			info: &relaycommon.RelayInfo{PriceData: types.PriceData{Quota: 300_000}},
			want: 300_000,
		},
		{
			name: "free model",
			info: &relaycommon.RelayInfo{PriceData: types.PriceData{FreeModel: true}},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FrozenTaskSubmissionQuota(tt.info))
		})
	}
}

func TestUnknownTaskIsExcludedFromPollingAndTimeoutRefundLifecycle(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()
	unknown := makeTask(1, 1, 300_000, 0, BillingSourceWallet, 0)
	unknown.TaskID = "task_unknown_submission"
	unknown.Status = model.TaskStatusUnknown
	unknown.Progress = "0%"
	unknown.SubmitTime = now - 3600
	require.NoError(t, model.DB.Create(unknown).Error)

	active := makeTask(1, 1, 300_000, 0, BillingSourceWallet, 0)
	active.TaskID = "task_active_submission"
	active.Status = model.TaskStatusInProgress
	active.Progress = "50%"
	active.SubmitTime = now - 3600
	require.NoError(t, model.DB.Create(active).Error)

	timedOut := model.GetTimedOutUnfinishedTasks(now, 10)
	require.Len(t, timedOut, 1)
	assert.Equal(t, active.TaskID, timedOut[0].TaskID)
	pollable := model.GetAllUnFinishSyncTasks(10)
	require.Len(t, pollable, 1)
	assert.Equal(t, active.TaskID, pollable[0].TaskID)
	assert.True(t, model.HasUnfinishedSyncTasks())

	require.NoError(t, model.DB.Delete(active).Error)
	assert.False(t, model.HasUnfinishedSyncTasks())
}

func TestPendingSubmissionBillingIsExcludedFromAutomatedTaskLifecycle(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()
	pendingState := model.TaskSubmissionBillingPending
	pending := makeTask(1, 1, 300_000, 0, BillingSourceWallet, 0)
	pending.TaskID = "task_pending_submission_billing"
	pending.Status = model.TaskStatusNotStart
	pending.Progress = "0%"
	pending.SubmitTime = now - 3600
	pending.UpdatedAt = now - 3600
	pending.SubmissionKey = &pending.TaskID
	pending.SubmissionBillingState = &pendingState
	require.NoError(t, model.DB.Create(pending).Error)

	assert.Empty(t, model.GetTimedOutUnfinishedTasks(now, 10))
	assert.Empty(t, model.GetAllUnFinishSyncTasks(10))
	assert.False(t, model.HasUnfinishedSyncTasks())

	require.NoError(t, model.DB.Model(pending).Update("status", model.TaskStatusFailure).Error)
	assert.Empty(t, model.GetUnrefundedFailedTasks(now, 10))
}

func TestFinalizedSubmissionFailureNeverEntersAutomaticRefund(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()
	finalizedState := model.TaskSubmissionBillingFinalized
	completedStage := model.TaskSubmissionStageCompleted
	task := makeTask(1, 1, 300_000, 0, BillingSourceWallet, 0)
	task.TaskID = "task_upstream_charged_failure"
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.SubmitTime = now
	task.UpdatedAt = now - 60
	task.SubmissionKey = &task.TaskID
	task.SubmissionBillingState = &finalizedState
	task.SubmissionStage = &completedStage
	require.NoError(t, model.DB.Create(task).Error)

	assert.Empty(t, model.GetUnrefundedFailedTasks(now, 10))
}

func TestUpstreamChargedSubmissionRejectsDirectRefundAndQuotaReduction(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 500_000)
	seedChannel(t, 201)
	task := makeTask(101, 201, 300_000, 0, BillingSourceWallet, 0)
	task.TaskID = "task_direct_refund_guard"
	task.SubmissionKey = &task.TaskID
	require.NoError(t, model.DB.Create(task).Error)

	ctx := context.Background()
	assert.False(t, RefundTaskQuota(ctx, task, "should not refund"))
	assert.Equal(t, 300_000, task.Quota)
	RecalculateTaskQuota(ctx, task, 100_000, "should not reduce")
	assert.Equal(t, 300_000, task.Quota)

	var user model.User
	require.NoError(t, model.DB.First(&user, 101).Error)
	assert.Equal(t, 500_000, user.Quota)
}

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	logDSN, hadLogDSN := os.LookupEnv("LOG_SQL_DSN")
	_ = os.Unsetenv("LOG_SQL_DSN")
	if err := model.InitLogDB(); err != nil {
		panic("failed to initialize test database columns: " + err.Error())
	}
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Task{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.QuotaData{},
		&model.TopUp{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.SystemTask{},
		&model.SystemTaskLock{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	exitCode := m.Run()
	if hadLogDSN {
		_ = os.Setenv("LOG_SQL_DSN", logDSN)
	}
	os.Exit(exitCode)
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM tasks")
		model.DB.Exec("DELETE FROM users")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM logs")
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM quota_data")
		model.DB.Exec("DELETE FROM top_ups")
		model.DB.Exec("DELETE FROM subscription_plans")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM subscription_pre_consume_records")
		model.DB.Exec("DELETE FROM system_task_locks")
		model.DB.Exec("DELETE FROM system_tasks")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:          id,
		UserId:      userId,
		Key:         key,
		Name:        "test_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedSubscription(t *testing.T, id int, userId int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &model.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(ch).Error)
}

func makeTask(userId, channelId, quota, tokenId int, billingSource string, subscriptionId int) *model.Task {
	return &model.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userId,
		ChannelId: channelId,
		Quota:     quota,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionId,
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Read-back helpers
// ---------------------------------------------------------------------------

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getTaskQuota(t *testing.T, id int64) int {
	t.Helper()
	var task model.Task
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&task).Error)
	return task.Quota
}

func getLastLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	model.LOG_DB.Model(&model.Log{}).Count(&count)
	return count
}

// ===========================================================================
// RefundTaskQuota tests
// ===========================================================================

func TestRefundTaskQuota_Wallet(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 1, 1, 1
	const initQuota, preConsumed = 10000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-key", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "task failed: upstream error")

	// User quota should increase by preConsumed
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Token remain_quota should increase, used_quota should decrease
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -preConsumed, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, 0, task.Quota)

	// A refund log should be created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "test-model", log.ModelName)
}

func TestRefundTaskQuota_PersistsZeroQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 5, 5, 5
	const initQuota, preConsumed = 10000, 2500
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-persist-refund", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	require.NoError(t, model.DB.Create(task).Error)

	RefundTaskQuota(ctx, task, "persist refund quota")

	assert.Equal(t, 0, task.Quota)
	assert.Equal(t, 0, getTaskQuota(t, task.ID))
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
}

func TestRefundTaskQuota_NodeAdminInfo(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 6, 6, 6
	const initQuota, preConsumed = 10000, 2500
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-refund-node", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.NodeName = "submit-node-a"

	RefundTaskQuota(ctx, task, "node metadata refund")

	log := getLastLog(t)
	require.NotNil(t, log)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "submit-node-a", adminInfo["node_name"])
}

func TestRefundTaskQuota_Subscription(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 2, 2, 2, 1
	const preConsumed = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-key", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RefundTaskQuota(ctx, task, "subscription task failed")

	// Subscription used should decrease by preConsumed
	assert.Equal(t, subUsed-int64(preConsumed), getSubscriptionUsed(t, subID))

	// Token should also be refunded
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 0, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRefundTaskQuota_ZeroQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 3
	seedUser(t, userID, 5000)

	task := makeTask(userID, 0, 0, 0, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "zero quota task")

	// No change to user quota
	assert.Equal(t, 5000, getUserQuota(t, userID))

	// No log created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRefundTaskQuota_NoToken(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 4, 4
	const initQuota, preConsumed = 10000, 1500

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0) // TokenId=0

	RefundTaskQuota(ctx, task, "no token task failed")

	// User quota refunded
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Log created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRefundTaskQuotaFundingFailureKeepsPendingMarker(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, preConsumed = 15, 1200
	seedUser(t, userID, 5000)
	task := makeTask(userID, 0, preConsumed, 0, BillingSourceSubscription, 9999)
	task.Status = model.TaskStatusFailure
	require.NoError(t, model.DB.Create(task).Error)

	assert.False(t, RefundTaskQuota(ctx, task, "subscription missing"))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, preConsumed, getTaskQuota(t, task.ID))
	assert.Equal(t, int64(0), countLogs(t))
}

// ===========================================================================
// RecalculateTaskQuota tests
// ===========================================================================

func TestRecalculate_PositiveDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 10, 10, 10
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000 // under-charged by 1000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-pos", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should decrease by the delta (1000 additional charge)
	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))

	// Token should also be charged the delta
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Consume (additional charge)
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, actualQuota-preConsumed, log.Quota)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.NotContains(t, other, "admin_info")
}

func TestRecalculate_QuotaClampAdminInfo(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 16, 16, 16
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-clamp", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.NodeName = "submit-node-b"
	clamp := (&common.QuotaClamp{
		Kind:     "overflow",
		Original: "+Inf",
		Clamped:  math.MaxInt32,
	}).WithOp("task_submit_other_ratios")

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment", clamp)

	log := getLastLog(t)
	require.NotNil(t, log)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	saturation, ok := adminInfo["quota_saturation"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "task_submit_other_ratios", saturation["op"])
	assert.Equal(t, "overflow", saturation["kind"])
	assert.Equal(t, "+Inf", saturation["original"])
	assert.Equal(t, float64(math.MaxInt32), saturation["clamped"])
	assert.Equal(t, "submit-node-b", adminInfo["node_name"])
}

func TestRecalculate_PersistsActualQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 15, 15, 15
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3500
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-persist", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	require.NoError(t, model.DB.Create(task).Error)

	RecalculateTaskQuota(ctx, task, actualQuota, "persist actual quota")

	assert.Equal(t, actualQuota, task.Quota)
	assert.Equal(t, actualQuota, getTaskQuota(t, task.ID))
}

func TestRecalculate_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 11, 11, 11
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged by 2000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-neg", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should increase by abs(delta) = 2000 (refund overpayment)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))

	// Token should be refunded the difference
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota updated
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Refund
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
}

func TestRecalculate_ZeroDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 12
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, preConsumed, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, preConsumed, "exact match")

	// No change to user quota
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No log created (delta is zero)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_ActualQuotaZero(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 13
	const initQuota = 10000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, 5000, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, 0, "zero actual")

	// No change (early return)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_Subscription_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 14, 14, 14, 2
	const preConsumed = 5000
	const actualQuota = 2000 // over-charged by 3000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-recalc", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RecalculateTaskQuota(ctx, task, actualQuota, "subscription over-charge")

	// Subscription used should decrease by delta (refund 3000)
	assert.Equal(t, subUsed-int64(preConsumed-actualQuota), getSubscriptionUsed(t, subID))

	// Token refunded
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestTaskTokenRecalculatedQuotaSaturates(t *testing.T) {
	assert.Equal(t, 3000, taskTokenRecalculatedQuota(1000, 1.5, 2, 1))
	assert.Equal(t, math.MaxInt32, taskTokenRecalculatedQuota(math.MaxInt32, 1.5, 2, 1.8446744073686647e19))

	quota, clamp := taskTokenRecalculatedQuotaChecked(math.MaxInt32, 1.5, 2, 1.8446744073686647e19)
	assert.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	assert.Equal(t, "overflow", clamp.Kind)
}

func TestAttachQuotaClampAdminInfoPreservesExistingAdminInfo(t *testing.T) {
	other := map[string]interface{}{
		"admin_info": map[string]interface{}{
			"existing": true,
		},
	}
	clamp := (&common.QuotaClamp{
		Kind:     "nan",
		Original: "NaN",
		Clamped:  0,
	}).WithOp("task_token_recalculation")

	attachQuotaClampAdminInfo(other, clamp)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, adminInfo["existing"])
	saturation, ok := adminInfo["quota_saturation"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "task_token_recalculation", saturation["op"])
	assert.Equal(t, "nan", saturation["kind"])
	assert.Equal(t, "NaN", saturation["original"])
	assert.Equal(t, 0, saturation["clamped"])
}

// ===========================================================================
// CAS + Billing integration tests
// Simulates the flow in updateVideoSingleTask (service/task_polling.go)
// ===========================================================================

// simulatePollBilling reproduces the CAS + billing logic from updateVideoSingleTask.
// It takes a persisted task (already in DB), applies the new status, and performs
// the conditional update + billing exactly as the polling loop does.
func simulatePollBilling(ctx context.Context, task *model.Task, newStatus model.TaskStatus, actualQuota int) {
	snap := task.Snapshot()

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = newStatus
	switch string(newStatus) {
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
		shouldSettle = true
	case model.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
		if quota != 0 {
			shouldRefund = true
		}
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == model.TaskStatus(model.TaskStatusSuccess) || task.Status == model.TaskStatus(model.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}

	if shouldSettle && actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "test settle")
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS wins: task in DB should now be FAILURE
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)

	// Refund should have happened
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	// Create task with IN_PROGRESS in DB
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate another process already transitioning to FAILURE
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("status", model.TaskStatusFailure)

	// Our process still has the old in-memory state (IN_PROGRESS) and tries to transition
	// task.Status is still IN_PROGRESS in the snapshot
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS lost: user quota should NOT change (no double refund)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// No billing log should be created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged, should get partial refund
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusSuccess), actualQuota)

	// CAS wins: task should be SUCCESS
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)

	// Settlement should refund the over-charge (5000 - 3000 = 2000 back to user)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate a non-terminal poll update (still IN_PROGRESS, progress changed)
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusInProgress), 0)

	// User quota should NOT change
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No billing log
	assert.Equal(t, int64(0), countLogs(t))

	// Task progress should be updated in DB
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

// ===========================================================================
// Mock adaptor for settleTaskBillingOnComplete tests
// ===========================================================================

type mockAdaptor struct {
	adjustReturn int
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}
func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return nil, nil
}
func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }
func (m *mockAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}

// ===========================================================================
// PerCallBilling tests — settleTaskBillingOnComplete
// ===========================================================================

func TestSettle_PerCallBilling_SkipsAdaptorAdjust(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 2000}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no adjustment despite adaptor returning 2000
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_PerCallBilling_SkipsTotalTokens(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 9999}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no recalculation by tokens
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_NonPerCallBilling_AppliesAdaptorAdjustment(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	// PerCallBilling defaults to false

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Non-per-call: adaptor adjustment applies (refund 2000)
	assert.Equal(t, initQuota+(preConsumed-adaptorQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-adaptorQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, adaptorQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}
