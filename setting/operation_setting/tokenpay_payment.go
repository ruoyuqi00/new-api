package operation_setting

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const (
	TokenPayNetworkTRON    = "USDT_TRC20"
	TokenPayNetworkBSC     = "EVM_BSC_USDT_BEP20"
	TokenPayNetworkPolygon = "EVM_Polygon_USDT_ERC20"
	TokenPayMaxUserQuota   = int64(2147483647)
)

type TokenPayTopUpPackage struct {
	USDT    int64  `json:"usdt"`
	Balance string `json:"balance"`
	Label   string `json:"label,omitempty"`
}

type TokenPayPaymentSetting struct {
	Enabled         bool                   `json:"enabled"`
	BaseURL         string                 `json:"base_url"`
	APIToken        string                 `json:"api_token"`
	Packages        []TokenPayTopUpPackage `json:"packages"`
	EnabledNetworks []string               `json:"enabled_networks"`
	AllowPrivateURL bool                   `json:"allow_private_url"`
}

var tokenPayPaymentSetting = TokenPayPaymentSetting{
	Packages:        []TokenPayTopUpPackage{},
	EnabledNetworks: []string{},
}

func init() {
	config.GlobalConfig.Register("tokenpay_payment", &tokenPayPaymentSetting)
}

func GetTokenPayPaymentSetting() *TokenPayPaymentSetting {
	return &tokenPayPaymentSetting
}

func (setting *TokenPayPaymentSetting) FindPackage(usdt int64) (TokenPayTopUpPackage, bool) {
	for _, item := range setting.Packages {
		if item.USDT == usdt {
			return item, true
		}
	}
	return TokenPayTopUpPackage{}, false
}

func TokenPayPackageQuota(balance decimal.Decimal) (int64, error) {
	if !balance.IsPositive() {
		return 0, errors.New("TokenPay package balance must be greater than zero")
	}
	if balance.Exponent() < -6 || !balance.Equal(balance.Truncate(6)) {
		return 0, errors.New("TokenPay package balance supports at most six decimal places")
	}
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return 0, errors.New("invalid TokenPay quota conversion")
	}
	quota := balance.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0)
	if quota.LessThan(decimal.NewFromInt(1)) || quota.GreaterThan(decimal.NewFromInt(TokenPayMaxUserQuota)) {
		return 0, errors.New("TokenPay package quota is outside the supported range")
	}
	return quota.IntPart(), nil
}

func (setting *TokenPayPaymentSetting) Normalize() error {
	if setting == nil {
		return errors.New("TokenPay payment setting is required")
	}
	setting.BaseURL = strings.TrimRight(strings.TrimSpace(setting.BaseURL), "/")
	setting.APIToken = strings.TrimSpace(setting.APIToken)

	packages := append([]TokenPayTopUpPackage(nil), setting.Packages...)
	seen := make(map[int64]struct{}, len(packages))
	for index := range packages {
		item := &packages[index]
		if item.USDT <= 0 {
			return errors.New("TokenPay package USDT amount must be positive")
		}
		if _, duplicate := seen[item.USDT]; duplicate {
			return fmt.Errorf("duplicate TokenPay package for USDT amount %d", item.USDT)
		}
		seen[item.USDT] = struct{}{}
		balance, err := decimal.NewFromString(strings.TrimSpace(item.Balance))
		if err != nil {
			return fmt.Errorf("invalid TokenPay package balance for USDT amount %d", item.USDT)
		}
		if _, err := TokenPayPackageQuota(balance); err != nil {
			return err
		}
		item.Balance = balance.String()
		item.Label = strings.TrimSpace(item.Label)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].USDT < packages[j].USDT })
	setting.Packages = packages

	selected := make(map[string]struct{}, len(setting.EnabledNetworks))
	for _, network := range setting.EnabledNetworks {
		network = strings.TrimSpace(network)
		switch network {
		case TokenPayNetworkTRON, TokenPayNetworkBSC, TokenPayNetworkPolygon:
			selected[network] = struct{}{}
		default:
			return fmt.Errorf("unsupported TokenPay network: %s", network)
		}
	}
	setting.EnabledNetworks = make([]string, 0, len(selected))
	for _, network := range []string{TokenPayNetworkTRON, TokenPayNetworkBSC, TokenPayNetworkPolygon} {
		if _, enabled := selected[network]; enabled {
			setting.EnabledNetworks = append(setting.EnabledNetworks, network)
		}
	}

	if setting.BaseURL != "" {
		parsed, err := url.Parse(setting.BaseURL)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
			return errors.New("invalid TokenPay base URL")
		}
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		privateHost := strings.EqualFold(host, "localhost") || ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast())
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && privateHost && setting.AllowPrivateURL) {
			return errors.New("TokenPay base URL must use HTTPS")
		}
		if privateHost && !setting.AllowPrivateURL {
			return errors.New("private TokenPay URL requires explicit opt-in")
		}
	}
	if setting.Enabled && (setting.BaseURL == "" || setting.APIToken == "" || len(setting.Packages) == 0 || len(setting.EnabledNetworks) == 0) {
		return errors.New("enabled TokenPay requires base URL, API token, package, and network")
	}
	return nil
}
