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

func TestQuotaFromFloatChecked(t *testing.T) {
	quota, clamp := QuotaFromFloatChecked(42.4)
	require.Equal(t, 42, quota)
	require.Nil(t, clamp)

	quota, clamp = QuotaFromFloatChecked(math.Inf(1))
	require.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "overflow", clamp.Kind)
	require.Equal(t, "+Inf", clamp.Original)
	require.Equal(t, math.MaxInt32, clamp.Clamped)
	require.NotContains(t, clamp.AuditMap(), "op")

	audit := clamp.WithOp("task_submit").AuditMap()
	require.Equal(t, "task_submit", audit["op"])
	require.Equal(t, "overflow", audit["kind"])
	require.Equal(t, "+Inf", audit["original"])
	require.Equal(t, math.MaxInt32, audit["clamped"])

	quota, clamp = QuotaFromFloatChecked(math.NaN())
	require.Equal(t, 0, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "nan", clamp.Kind)
	require.Equal(t, "NaN", clamp.Original)
	require.Equal(t, 0, clamp.Clamped)
}

func TestStrictQuotaConversionsRejectSaturation(t *testing.T) {
	quota, err := QuotaFromFloatStrict(math.Inf(1))
	require.Zero(t, quota)
	require.Error(t, err)

	quota, err = QuotaRoundStrict(float64(math.MaxInt32) + 1)
	require.Zero(t, quota)
	require.Error(t, err)

	quota, err = QuotaRoundStrict(42.5)
	require.NoError(t, err)
	require.Equal(t, 43, quota)
}
