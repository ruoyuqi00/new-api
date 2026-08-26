package groupavailability

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAndQueryKeepsOnlyRecentAvailabilitySamples(t *testing.T) {
	server := miniredis.RunT(t)
	originalRedisEnabled, originalRedis := common.RedisEnabled, common.RDB
	originalMonitoring := ratio_setting.AvailabilityMonitoring2JSONString()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(`{"china":true}`))
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRedis
		require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(originalMonitoring))
	})

	require.NoError(t, Record("china", true))
	require.NoError(t, Record("china", false))
	require.NoError(t, Record("china", true))

	summary, err := Query("china")
	require.NoError(t, err)
	assert.Equal(t, int64(3), summary.RequestCount)
	assert.Equal(t, int64(2), summary.SuccessCount)
	assert.InDelta(t, 66.67, summary.SuccessRate, 0.01)
	assert.Equal(t, AvailabilityObserving, summary.Status)

	for i := 0; i < 301; i++ {
		require.NoError(t, Record("china", true))
	}
	summary, err = Query("china")
	require.NoError(t, err)
	assert.Equal(t, int64(300), summary.RequestCount)
	assert.Equal(t, int64(300), summary.SuccessCount)
	assert.Equal(t, AvailabilityStable, summary.Status)

	keys, err := common.RDB.Keys(t.Context(), "group-availability:v1:*").Result()
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestAvailabilityStatusRequiresTwentySamplesAndUsesTolerantThresholds(t *testing.T) {
	tests := []struct {
		name         string
		requestCount int64
		successRate  float64
		want         string
	}{
		{name: "no data", requestCount: 0, successRate: 0, want: AvailabilityNoData},
		{name: "observing before twenty", requestCount: 19, successRate: 0, want: AvailabilityObserving},
		{name: "stable at ninety", requestCount: 20, successRate: 90, want: AvailabilityStable},
		{name: "degraded below ninety", requestCount: 20, successRate: 89.99, want: AvailabilityDegraded},
		{name: "degraded at sixty", requestCount: 20, successRate: 60, want: AvailabilityDegraded},
		{name: "unavailable below sixty", requestCount: 20, successRate: 59.99, want: AvailabilityUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, statusFor(test.requestCount, test.successRate))
		})
	}
}

func TestDisabledGroupDoesNotRecordAndRedisFailureIsNoData(t *testing.T) {
	server := miniredis.RunT(t)
	originalRedisEnabled, originalRedis := common.RedisEnabled, common.RDB
	originalMonitoring := ratio_setting.AvailabilityMonitoring2JSONString()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(`{"private":false}`))
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRedis
		require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(originalMonitoring))
	})

	require.NoError(t, Record("private", false))
	summary, err := Query("private")
	require.NoError(t, err)
	assert.Equal(t, AvailabilityNoData, summary.Status)
	assert.Zero(t, summary.RequestCount)

	common.RedisEnabled = false
	common.RDB = nil
	summary, err = Query("private")
	require.NoError(t, err)
	assert.Equal(t, AvailabilityNoData, summary.Status)
	assert.Zero(t, summary.RequestCount)
}

func TestIsTextRequestPathExcludesMediaAndTasks(t *testing.T) {
	assert.True(t, IsTextRequestPath("/v1/chat/completions"))
	assert.True(t, IsTextRequestPath("/v1/responses"))
	assert.True(t, IsTextRequestPath("/v1/responses/compact"))
	assert.True(t, IsTextRequestPath("/v1/completions"))
	assert.True(t, IsTextRequestPath("/v1/messages"))
	assert.True(t, IsTextRequestPath("/v1/messages?beta=true"))
	assert.False(t, IsTextRequestPath("/v1/images/generations"))
	assert.False(t, IsTextRequestPath("/v1/videos"))
	assert.False(t, IsTextRequestPath("/api/yucore/media/tasks"))
}
