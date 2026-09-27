package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenPayPaymentSettingNormalizeFixedPackages(t *testing.T) {
	setting := TokenPayPaymentSetting{
		Enabled:         true,
		BaseURL:         "https://pay.example.com/",
		APIToken:        "secret",
		Packages:        []TokenPayTopUpPackage{{USDT: 20, Balance: "132.0"}, {USDT: 10, Balance: "66.0"}},
		EnabledNetworks: []string{"USDT_TRC20", "EVM_BSC_USDT_BEP20", "EVM_Polygon_USDT_ERC20"},
	}

	require.NoError(t, setting.Normalize())
	assert.Equal(t, "https://pay.example.com", setting.BaseURL)
	assert.Equal(t, int64(10), setting.Packages[0].USDT)
	assert.Equal(t, "66", setting.Packages[0].Balance)
	assert.Equal(t, int64(20), setting.Packages[1].USDT)
	assert.Equal(t, "132", setting.Packages[1].Balance)
	_, ok := setting.FindPackage(30)
	assert.False(t, ok)
}

func TestTokenPayPaymentSettingRejectsInvalidPackagesAndNetworks(t *testing.T) {
	tests := []struct {
		name     string
		packages []TokenPayTopUpPackage
		networks []string
	}{
		{name: "zero USDT", packages: []TokenPayTopUpPackage{{USDT: 0, Balance: "66"}}, networks: []string{"USDT_TRC20"}},
		{name: "duplicate USDT", packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}, {USDT: 10, Balance: "70"}}, networks: []string{"USDT_TRC20"}},
		{name: "zero balance", packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "0"}}, networks: []string{"USDT_TRC20"}},
		{name: "invalid balance", packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "abc"}}, networks: []string{"USDT_TRC20"}},
		{name: "unsupported network", packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}}, networks: []string{"EVM_ETH_USDT_ERC20"}},
		{name: "no network", packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting := TokenPayPaymentSetting{Enabled: true, BaseURL: "https://pay.example.com", APIToken: "secret", Packages: test.packages, EnabledNetworks: test.networks}
			require.Error(t, setting.Normalize())
		})
	}
}

func TestTokenPayPaymentSettingRejectsUnsafeOrIncompleteEndpoint(t *testing.T) {
	for _, rawURL := range []string{"http://pay.example.com", "http://127.0.0.1:8080", "https://user:pass@pay.example.com", "https://pay.example.com/path"} {
		t.Run(rawURL, func(t *testing.T) {
			setting := TokenPayPaymentSetting{Enabled: true, BaseURL: rawURL, APIToken: "secret", Packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}}, EnabledNetworks: []string{"USDT_TRC20"}}
			require.Error(t, setting.Normalize())
		})
	}
	setting := TokenPayPaymentSetting{Enabled: true, BaseURL: "https://pay.example.com", Packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}}, EnabledNetworks: []string{"USDT_TRC20"}}
	require.Error(t, setting.Normalize())
}
