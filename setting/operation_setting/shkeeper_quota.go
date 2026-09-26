package operation_setting

import (
	"errors"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

func SHKeeperPackageQuota(balance decimal.Decimal) (int64, error) {
	if !balance.IsPositive() {
		return 0, errors.New("SHKeeper package balance must be greater than zero")
	}
	if balance.Exponent() < -6 || !balance.Equal(balance.Truncate(6)) {
		return 0, errors.New("SHKeeper package balance supports at most six decimal places")
	}
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return 0, errors.New("invalid SHKeeper quota conversion")
	}
	quota := balance.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0)
	if quota.LessThan(decimal.NewFromInt(1)) {
		return 0, errors.New("SHKeeper package must grant at least one quota")
	}
	if quota.GreaterThan(decimal.NewFromInt(SHKeeperMaxUserQuota)) {
		return 0, errors.New("SHKeeper package exceeds the supported quota limit")
	}
	return quota.IntPart(), nil
}
