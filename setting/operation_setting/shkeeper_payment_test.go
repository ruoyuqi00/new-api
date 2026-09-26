package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHKeeperPaymentSettingNormalizePackages(t *testing.T) {
	setting := SHKeeperPaymentSetting{
		Packages: []SHKeeperTopUpPackage{
			{USDT: 50, Balance: "330.000000", Label: " Common "},
			{USDT: 10, Balance: "66", Label: "Starter"},
		},
		EnabledNetworks: []string{"USDT", "BNB-USDT", "USDT"},
	}
	require.NoError(t, setting.Normalize())
	assert.Equal(t, []SHKeeperTopUpPackage{
		{USDT: 10, Balance: "66", Label: "Starter"},
		{USDT: 50, Balance: "330", Label: "Common"},
	}, setting.Packages)
	assert.Equal(t, []string{"BNB-USDT", "USDT"}, setting.EnabledNetworks)
}

func TestSHKeeperPaymentSettingRejectsInvalidPackages(t *testing.T) {
	tests := []struct {
		name     string
		packages []SHKeeperTopUpPackage
		want     string
	}{
		{name: "zero USDT", packages: []SHKeeperTopUpPackage{{USDT: 0, Balance: "1"}}, want: "USDT"},
		{name: "duplicate USDT", packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "1"}, {USDT: 10, Balance: "2"}}, want: "duplicate"},
		{name: "nonpositive balance", packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "0"}}, want: "balance"},
		{name: "excess precision", packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "1.0000001"}}, want: "six decimal"},
		{name: "excess trailing-zero precision", packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "1.0000000"}}, want: "six decimal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := SHKeeperPaymentSetting{Packages: tt.packages}
			require.ErrorContains(t, setting.Normalize(), tt.want)
		})
	}
}

func TestSHKeeperPaymentSettingNetworkAndURLValidation(t *testing.T) {
	setting := SHKeeperPaymentSetting{EnabledNetworks: []string{"BTC"}}
	require.ErrorContains(t, setting.Normalize(), "network")

	setting = SHKeeperPaymentSetting{BaseURL: "http://127.0.0.1:8080"}
	require.ErrorContains(t, setting.Normalize(), "private")
	setting.AllowPrivateURL = true
	require.NoError(t, setting.Normalize())
}

func TestSHKeeperPaymentSettingRequiresPackagesWhenEnabled(t *testing.T) {
	setting := SHKeeperPaymentSetting{Enabled: true}
	require.ErrorContains(t, setting.Normalize(), "package")
}

func TestSHKeeperPaymentSettingIgnoresLegacyPricingFields(t *testing.T) {
	setting := SHKeeperPaymentSetting{
		Packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "66"}},
		Rate:     "not-a-rate", MinTopUp: -100, MaxTopUp: -1,
	}
	require.NoError(t, setting.Normalize())
	assert.Equal(t, "not-a-rate", setting.Rate)
	assert.Equal(t, -100, setting.MinTopUp)
	assert.Equal(t, -1, setting.MaxTopUp)
}

func TestSHKeeperPaymentSettingFindPackage(t *testing.T) {
	setting := SHKeeperPaymentSetting{Packages: []SHKeeperTopUpPackage{{USDT: 10, Balance: "66"}}}
	require.NoError(t, setting.Normalize())
	pkg, ok := setting.FindPackage(10)
	require.True(t, ok)
	assert.Equal(t, "66", pkg.Balance)
	_, ok = setting.FindPackage(11)
	assert.False(t, ok)
}
