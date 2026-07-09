package common

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuotaFromFloat(t *testing.T) {
	require.Equal(t, 42, QuotaFromFloat(42.4))
	require.Equal(t, -42, QuotaFromFloat(-42.4))
	require.Equal(t, math.MaxInt32, QuotaFromFloat(math.Inf(1)))
	require.Equal(t, math.MinInt32, QuotaFromFloat(math.Inf(-1)))
	require.Equal(t, math.MaxInt32, QuotaFromFloat(2000*1.8446744073686647e19))
	require.Equal(t, math.MinInt32, QuotaFromFloat(-2000*1.8446744073686647e19))
	require.Equal(t, 0, QuotaFromFloat(math.NaN()))
}
