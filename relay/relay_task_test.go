package relay

import (
	"math"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestApplyTaskOtherRatiosQuota(t *testing.T) {
	require.Equal(t, 2000, applyTaskOtherRatiosQuota(1000, map[string]float64{
		"seconds": 2,
		"size":    1,
	}))
	require.Equal(t, math.MaxInt32, applyTaskOtherRatiosQuota(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	}))

	quota, clamp := applyTaskOtherRatiosQuotaChecked(1000, map[string]float64{"seconds": 2})
	require.Equal(t, 2000, quota)
	require.Nil(t, clamp)

	quota, clamp = applyTaskOtherRatiosQuotaChecked(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	require.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "overflow", clamp.Kind)
}

func TestRecalcQuotaFromRatiosSaturates(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota:       6000,
			OtherRatios: map[string]float64{"seconds": 3},
		},
	}

	require.Equal(t, 4000, recalcQuotaFromRatios(info, map[string]float64{
		"seconds": 2,
	}))
	require.Equal(t, math.MaxInt32, recalcQuotaFromRatios(info, map[string]float64{
		"seconds": 1.8446744073686647e19,
	}))

	quota, clamp := recalcQuotaFromRatiosChecked(info, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	require.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "overflow", clamp.Kind)
}

func TestNoteTaskQuotaClampStoresFirstOnly(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	_, first := applyTaskOtherRatiosQuotaChecked(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	_, second := applyTaskOtherRatiosQuotaChecked(-2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})

	noteTaskQuotaClamp(info, first, "first_op")
	noteTaskQuotaClamp(info, second, "second_op")

	require.NotNil(t, info.TaskQuotaClamp)
	require.Equal(t, "first_op", info.TaskQuotaClamp.Op)
	require.Equal(t, "overflow", info.TaskQuotaClamp.Kind)
}
