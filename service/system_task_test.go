package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSystemTaskRegistry swaps the package registry for the given handlers for
// the duration of a test and restores the original registry afterward.
func withSystemTaskRegistry(t *testing.T, handlers ...SystemTaskHandler) {
	t.Helper()
	systemTaskHandlersMu.Lock()
	saved := systemTaskHandlers
	systemTaskHandlers = map[string]SystemTaskHandler{}
	for _, h := range handlers {
		systemTaskHandlers[h.Type()] = h
	}
	systemTaskHandlersMu.Unlock()
	t.Cleanup(func() {
		systemTaskHandlersMu.Lock()
		systemTaskHandlers = saved
		systemTaskHandlersMu.Unlock()
	})
}

type stubScheduledHandler struct {
	taskType string
	enabled  bool
	interval time.Duration
	onRun    func(ctx context.Context, task *model.SystemTask, runnerID string)
}

type stubSystemTaskRunResult struct {
	taskID   string
	taskType string
	err      error
}

func (h *stubScheduledHandler) Type() string { return h.taskType }

func (h *stubScheduledHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if h.onRun != nil {
		h.onRun(ctx, task, runnerID)
	}
}

func (h *stubScheduledHandler) Enabled() bool           { return h.enabled }
func (h *stubScheduledHandler) Interval() time.Duration { return h.interval }
func (h *stubScheduledHandler) NewPayload() any         { return nil }

func countSystemTasks(t *testing.T, taskType string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Where("type = ?", taskType).Count(&count).Error)
	return count
}

func TestSystemTaskSchedulerCreatesWhenDueAndDedups(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_scheduled", enabled: true, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// An active (pending) row already exists, so a second pass must not create
	// another row.
	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Finish the run; with a fresh updated_at the next run is not due yet.
	latest, err := model.GetLatestSystemTask(handler.taskType)
	require.NoError(t, err)
	require.NotNil(t, latest)
	_, claimed, err := model.ClaimSystemTask(latest.ID, handler.taskType, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(latest.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Backdate the finished row beyond the interval -> the job becomes due again.
	require.NoError(t, model.DB.Model(&model.SystemTask{}).
		Where("task_id = ?", latest.TaskID).
		Update("updated_at", common.GetTimestamp()-120).Error)

	runSystemTaskScheduler()
	require.Equal(t, int64(2), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskRunnerHonorsExplicitDesignatedNode(t *testing.T) {
	previousMaster := common.IsMasterNode
	previousNodeName := common.NodeName
	t.Cleanup(func() {
		common.IsMasterNode = previousMaster
		common.NodeName = previousNodeName
	})

	common.IsMasterNode = true
	t.Setenv("SYSTEM_TASK_RUNNER_ENABLED", "true")
	t.Setenv("SYSTEM_TASK_RUNNER_NODE_NAME", "yuapi-primary")
	common.NodeName = "yuapi-primary"
	assert.True(t, systemTaskRunnerEnabled())

	common.NodeName = "stale-candidate"
	assert.False(t, systemTaskRunnerEnabled())
}

func TestLeaseRenewalRetriesTransientDatabaseFailure(t *testing.T) {
	attempts := 0
	err := retryLeaseRenewal(func() error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary database error")
		}
		return nil
	}, 50*time.Millisecond, time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestRunLogCleanupTaskAdjustsCountersAndDeletesDashboardRows(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	cutoff := now - 100
	user := &model.User{Id: 501, Username: "cleanup-user", Password: "password", Status: common.UserStatusEnabled, Quota: 1000, UsedQuota: 300, RequestCount: 3}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 511, UserId: user.Id, Key: "cleanup-token", Status: common.TokenStatusEnabled, UsedQuota: 300, RemainQuota: 700}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 521, Key: "cleanup-channel", Name: "cleanup", Status: common.ChannelStatusEnabled, UsedQuota: 300}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: user.Id, Username: user.Username, CreatedAt: cutoff - 1, Type: model.LogTypeConsume,
		Quota: 300, PromptTokens: 100, CompletionTokens: 50, TokenId: 511, ChannelId: 521,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: user.Id, Username: user.Username, CreatedAt: cutoff - 2, Type: model.LogTypeError,
		Quota: 999, TokenId: 511, ChannelId: 521,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: user.Id, Username: user.Username, CreatedAt: cutoff + 1, Type: model.LogTypeError,
		Quota: 888, TokenId: 511, ChannelId: 521,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: user.Id, Username: user.Username, CreatedAt: cutoff, Type: model.LogTypeManage,
		Quota: 777, TokenId: 511, ChannelId: 521,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: user.Id, Username: user.Username, CreatedAt: cutoff - 1, Count: 1, Quota: 300, TokenUsed: 150}).Error)

	task, err := model.CreateSystemTask(model.SystemTaskTypeLogCleanup, LogCleanupPayload{TargetTimestamp: cutoff, BatchSize: 10}, LogCleanupState{})
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "cleanup-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimed)

	runLogCleanupTask(context.Background(), claimed, "cleanup-runner")

	var gotUser model.User
	require.NoError(t, model.DB.First(&gotUser, user.Id).Error)
	assert.Equal(t, 0, gotUser.UsedQuota)
	assert.Equal(t, 2, gotUser.RequestCount)
	assert.Equal(t, 1000, gotUser.Quota)
	var gotToken model.Token
	require.NoError(t, model.DB.First(&gotToken, 511).Error)
	assert.Equal(t, 0, gotToken.UsedQuota)
	assert.Equal(t, 700, gotToken.RemainQuota)
	var gotChannel model.Channel
	require.NoError(t, model.DB.First(&gotChannel, 521).Error)
	assert.Equal(t, int64(0), gotChannel.UsedQuota)

	var oldLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("created_at < ?", cutoff).Count(&oldLogCount).Error)
	assert.Zero(t, oldLogCount)
	var retainedLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("created_at >= ?", cutoff).Count(&retainedLogCount).Error)
	assert.Equal(t, int64(2), retainedLogCount)
	var oldQuotaDataCount int64
	require.NoError(t, model.DB.Model(&model.QuotaData{}).Where("created_at < ?", cutoff).Count(&oldQuotaDataCount).Error)
	assert.Zero(t, oldQuotaDataCount)
	var retainedQuotaDataCount int64
	require.NoError(t, model.DB.Model(&model.QuotaData{}).Where("created_at >= ?", cutoff).Count(&retainedQuotaDataCount).Error)
	assert.Zero(t, retainedQuotaDataCount)

	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, finished)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	var result LogCleanupResult
	require.NoError(t, finished.DecodeState(&LogCleanupState{}))
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	assert.Equal(t, int64(2), result.DeletedCount)
	assert.Equal(t, int64(1), result.DeletedQuotaDataCount)
}

func TestSystemTaskSchedulerSkipsDisabled(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_disabled", enabled: false, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	assert.Equal(t, int64(0), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskClaimPassDispatchesByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 1)
	handler := &stubScheduledHandler{
		taskType: "test_dispatch",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskType: task.Type,
				err:      model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handler)

	_, err := model.CreateSystemTask(handler.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	select {
	case got := <-ran:
		require.NoError(t, got.err)
		assert.Equal(t, handler.taskType, got.taskType)
	case <-time.After(2 * time.Second):
		t.Fatal("claimed task was not dispatched to its handler")
	}

	require.Eventually(t, func() bool {
		latest, err := model.GetLatestSystemTask(handler.taskType)
		return err == nil && latest != nil && latest.Status == model.SystemTaskStatusSucceeded
	}, 2*time.Second, 20*time.Millisecond)
}

func TestSystemTaskClaimPassDispatchesEarliestPendingByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 2)
	handlerA := &stubScheduledHandler{
		taskType: "test_dispatch_a",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	handlerB := &stubScheduledHandler{
		taskType: "test_dispatch_b",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handlerA, handlerB)

	firstA, err := model.CreateSystemTask(handlerA.taskType, nil, nil)
	require.NoError(t, err)
	secondTaskID, err := model.GenerateSystemTaskID()
	require.NoError(t, err)
	secondA := &model.SystemTask{
		TaskID: secondTaskID,
		Type:   handlerA.taskType,
		Status: model.SystemTaskStatusPending,
	}
	require.NoError(t, model.DB.Create(secondA).Error)
	firstB, err := model.CreateSystemTask(handlerB.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	got := map[string]bool{}
	for range 2 {
		select {
		case result := <-ran:
			require.NoError(t, result.err)
			got[result.taskID] = true
		case <-time.After(2 * time.Second):
			t.Fatal("claimed tasks were not dispatched to their handlers")
		}
	}

	assert.True(t, got[firstA.TaskID])
	assert.True(t, got[firstB.TaskID])
	assert.False(t, got[secondA.TaskID])

	require.Eventually(t, func() bool {
		reloaded, err := model.GetSystemTaskByTaskID(secondA.TaskID)
		return err == nil && reloaded != nil && reloaded.Status == model.SystemTaskStatusPending
	}, 2*time.Second, 20*time.Millisecond)
}

func TestEnqueueSystemTaskReportsCreatedAndExistingActive(t *testing.T) {
	truncate(t)

	first, created, err := EnqueueSystemTask("test_enqueue", map[string]bool{"manual": true})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, first)

	existing, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.False(t, created)
	require.NotNil(t, existing)
	assert.Equal(t, first.TaskID, existing.TaskID)

	_, claimed, err := model.ClaimSystemTask(first.ID, first.Type, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(first.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	second, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, second)
	assert.NotEqual(t, first.TaskID, second.TaskID)
}
