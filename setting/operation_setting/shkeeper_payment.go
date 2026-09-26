package operation_setting

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const (
	SHKeeperCryptoBNBUSDT           = "BNB-USDT"
	SHKeeperCryptoUSDT              = "USDT"
	SHKeeperCryptoPolygonUSDT       = "POLYGON-USDT"
	SHKeeperInvoiceFiat             = "USD"
	SHKeeperMaxUserQuota      int64 = 2147483647
)

type SHKeeperTopUpPackage struct {
	USDT    int64  `json:"usdt"`
	Balance string `json:"balance"`
	Label   string `json:"label,omitempty"`
}

type SHKeeperPaymentSetting struct {
	Enabled                  bool                   `json:"enabled"`
	BaseURL                  string                 `json:"base_url"`
	APIKey                   string                 `json:"api_key"`
	BackendKey               string                 `json:"backend_api_key"`
	Packages                 []SHKeeperTopUpPackage `json:"packages"`
	EnabledNetworks          []string               `json:"enabled_networks"`
	InvoiceExpiryMinutes     int                    `json:"invoice_expiry_minutes"`
	ReconcileIntervalSeconds int                    `json:"reconcile_interval_seconds"`
	AllowPrivateURL          bool                   `json:"allow_private_url"`
	Rate                     string                 `json:"rate"`
	MinTopUp                 int                    `json:"min_top_up"`
	MaxTopUp                 int                    `json:"max_top_up"`
}

var shkeeperPaymentSetting = SHKeeperPaymentSetting{
	Packages:                 []SHKeeperTopUpPackage{},
	EnabledNetworks:          []string{},
	InvoiceExpiryMinutes:     30,
	ReconcileIntervalSeconds: 60,
}

func init() {
	config.GlobalConfig.Register("shkeeper_payment", &shkeeperPaymentSetting)
}

func GetSHKeeperPaymentSetting() *SHKeeperPaymentSetting {
	return &shkeeperPaymentSetting
}

func (setting *SHKeeperPaymentSetting) FindPackage(usdt int64) (SHKeeperTopUpPackage, bool) {
	for _, item := range setting.Packages {
		if item.USDT == usdt {
			return item, true
		}
	}
	return SHKeeperTopUpPackage{}, false
}

func (setting *SHKeeperPaymentSetting) Normalize() error {
	if setting == nil {
		return errors.New("SHKeeper payment setting is required")
	}

	setting.BaseURL = strings.TrimRight(strings.TrimSpace(setting.BaseURL), "/")
	setting.APIKey = strings.TrimSpace(setting.APIKey)
	setting.BackendKey = strings.TrimSpace(setting.BackendKey)
	packages := make([]SHKeeperTopUpPackage, len(setting.Packages))
	copy(packages, setting.Packages)
	seenPackages := make(map[int64]struct{}, len(packages))
	for index := range packages {
		item := &packages[index]
		if item.USDT <= 0 {
			return errors.New("SHKeeper package USDT amount must be positive")
		}
		if _, exists := seenPackages[item.USDT]; exists {
			return fmt.Errorf("duplicate SHKeeper package for USDT amount %d", item.USDT)
		}
		seenPackages[item.USDT] = struct{}{}
		balance, err := decimal.NewFromString(strings.TrimSpace(item.Balance))
		if err != nil {
			return fmt.Errorf("invalid SHKeeper package balance for USDT amount %d", item.USDT)
		}
		if _, err := SHKeeperPackageQuota(balance); err != nil {
			return err
		}
		item.Balance = balance.String()
		item.Label = strings.TrimSpace(item.Label)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].USDT < packages[j].USDT })
	setting.Packages = packages

	selected := make(map[string]struct{}, len(setting.EnabledNetworks))
	for _, network := range setting.EnabledNetworks {
		network = strings.ToUpper(strings.TrimSpace(network))
		if network == "" {
			continue
		}
		switch network {
		case SHKeeperCryptoBNBUSDT, SHKeeperCryptoUSDT, SHKeeperCryptoPolygonUSDT:
			selected[network] = struct{}{}
		default:
			return fmt.Errorf("unsupported SHKeeper network: %s", network)
		}
	}
	setting.EnabledNetworks = make([]string, 0, len(selected))
	for _, network := range []string{SHKeeperCryptoBNBUSDT, SHKeeperCryptoUSDT, SHKeeperCryptoPolygonUSDT} {
		if _, exists := selected[network]; exists {
			setting.EnabledNetworks = append(setting.EnabledNetworks, network)
		}
	}

	if setting.InvoiceExpiryMinutes <= 0 {
		setting.InvoiceExpiryMinutes = 30
	}
	if setting.ReconcileIntervalSeconds < 60 {
		setting.ReconcileIntervalSeconds = 60
	}
	if setting.BaseURL != "" {
		parsed, err := url.Parse(setting.BaseURL)
		if err != nil || parsed.Host == "" {
			return errors.New("invalid SHKeeper base URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return errors.New("SHKeeper base URL must use http or https")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("SHKeeper base URL must not contain credentials, query, or fragment")
		}
		host := strings.TrimSpace(parsed.Hostname())
		ip := net.ParseIP(host)
		privateHost := strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost")
		if ip != nil {
			privateHost = ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
		}
		if privateHost && !setting.AllowPrivateURL {
			return errors.New("private SHKeeper URL requires explicit opt-in")
		}
	}
	if setting.Enabled {
		if setting.BaseURL == "" || setting.APIKey == "" || len(setting.EnabledNetworks) == 0 || len(setting.Packages) == 0 {
			return errors.New("enabled SHKeeper requires base URL, API key, network, and package")
		}
	}
	return nil
}
