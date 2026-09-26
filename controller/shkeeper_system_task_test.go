package controller

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHKeeperScheduledRepairRunsWhenCreationDisabled(t *testing.T) {
	_, user := setupSHKeeperController(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	insertSHKeeperControllerOrder(t, user)
	settings := operation_setting.GetSHKeeperPaymentSetting()
	settings.Enabled = false
	settings.ReconcileIntervalSeconds = 120
	handler := shkeeperReconcileHandler{}
	assert.True(t, handler.Enabled())
	assert.Equal(t, 120*time.Second, handler.Interval())
	task, err := model.CreateSystemTask(handler.Type(), nil, nil)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(task).Updates(map[string]any{"status": model.SystemTaskStatusRunning, "locked_by": "test"}).Error)
	require.NoError(t, model.DB.Create(&model.SystemTaskLock{Type: handler.Type(), TaskID: task.TaskID, LockedBy: "test", LockedUntil: time.Now().Add(time.Hour).Unix()}).Error)
	handler.Run(context.Background(), task, "test")
	require.NoError(t, model.DB.First(task, task.ID).Error)
	assert.Equal(t, model.SystemTaskStatusSucceeded, task.Status)
	var result map[string]int
	require.NoError(t, common.UnmarshalJsonStr(task.Result, &result))
	assert.Equal(t, 1, result["processed"])
	assert.Equal(t, 1, result["failed"])
	assert.Zero(t, result["credited"])
}
