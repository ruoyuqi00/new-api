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
}
