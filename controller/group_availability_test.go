package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/groupavailability"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupAvailabilityFiltersByPermissionAndMonitorSwitch(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := miniredis.RunT(t)
	originalRedisEnabled, originalRedis := common.RedisEnabled, common.RDB
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalMonitoring := ratio_setting.AvailabilityMonitoring2JSONString()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","private":"Private"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"private":1,"secret":1}`))
	require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(`{"default":true,"private":false,"secret":true}`))
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRedis
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(originalMonitoring))
	})

	user := &model.User{Username: "availability-user", Password: "password", Group: "vip", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, groupavailability.Record("default", true))
	require.NoError(t, groupavailability.Record("private", false))
	require.NoError(t, groupavailability.Record("secret", false))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/group-availability", nil)
	ctx.Set("id", user.Id)

	GetUserGroupAvailability(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Group        string  `json:"group"`
			Description  string  `json:"description"`
			RequestCount int64   `json:"request_count"`
			SuccessCount int64   `json:"success_count"`
			SuccessRate  float64 `json:"success_rate"`
			Status       string  `json:"status"`
			ObservedAt   int64   `json:"observed_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "default", response.Data[0].Group)
	assert.Equal(t, "Default", response.Data[0].Description)
	assert.Equal(t, int64(1), response.Data[0].RequestCount)
	assert.Equal(t, int64(1), response.Data[0].SuccessCount)
	assert.Equal(t, 100.0, response.Data[0].SuccessRate)
	assert.Equal(t, groupavailability.AvailabilityObserving, response.Data[0].Status)

	var raw map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &raw))
	assert.NotContains(t, string(recorder.Body.Bytes()), "latency")
	assert.NotContains(t, string(recorder.Body.Bytes()), "channel")
	assert.NotContains(t, string(recorder.Body.Bytes()), "secret")
}
