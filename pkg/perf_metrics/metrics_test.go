package perfmetrics

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/groupavailability"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestRecordRelaySampleRecordsClaudeWhenRequestPathWasRewritten(t *testing.T) {
	server := miniredis.RunT(t)
	originalRedisEnabled, originalRedis := common.RedisEnabled, common.RDB
	originalMonitoring := ratio_setting.AvailabilityMonitoring2JSONString()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(`{"专享Claude":true}`))
	hotBuckets = sync.Map{}
	t.Cleanup(func() {
		hotBuckets = sync.Map{}
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRedis
		require.NoError(t, ratio_setting.UpdateAvailabilityMonitoringByJSONString(originalMonitoring))
	})

	RecordRelaySample(&relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatClaude,
		RequestURLPath:  "",
		UsingGroup:      "专享Claude",
		OriginModelName: "claude-sonnet-4-6",
		StartTime:       time.Now().Add(-time.Second),
	}, true, 1)

	summary, err := groupavailability.Query("专享Claude")
	require.NoError(t, err)
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, int64(1), summary.SuccessCount)
}

func TestIsTextRelayForAvailabilityExcludesMediaFormats(t *testing.T) {
	require.True(t, isTextRelayForAvailability(&relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
	}))
	require.False(t, isTextRelayForAvailability(&relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIImage,
	}))
	require.False(t, isTextRelayForAvailability(&relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatTask,
	}))
}
