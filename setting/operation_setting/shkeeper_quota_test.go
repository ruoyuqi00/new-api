package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustPackageQuota(t *testing.T, balance string) int64 {
	t.Helper()
	quota, err := SHKeeperPackageQuota(decimal.RequireFromString(balance))
	require.NoError(t, err)
	return quota
}

func TestSHKeeperPackageQuotaBoundaries(t *testing.T) {
	original := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = original })

	_, err := SHKeeperPackageQuota(decimal.RequireFromString("0.004999"))
	require.ErrorContains(t, err, "at least one quota")
	assert.EqualValues(t, 1, mustPackageQuota(t, "0.005"))
	assert.EqualValues(t, SHKeeperMaxUserQuota, mustPackageQuota(t, "21474836.474999"))
	_, err = SHKeeperPackageQuota(decimal.RequireFromString("21474836.475"))
	require.ErrorContains(t, err, "quota limit")
}
