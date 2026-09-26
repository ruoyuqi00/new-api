package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

var ErrSHKeeperInvoiceNotFound = errors.New("SHKeeper invoice not found")

type SHKeeperClientConfig struct {
	BaseURL         string
	APIKey          string
	BackendKey      string
	AllowPrivateURL bool
	Timeout         time.Duration
}

type SHKeeperClient struct {
	config     SHKeeperClientConfig
	httpClient *http.Client
}

type SHKeeperCrypto struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}
type SHKeeperQuote struct {
	CryptoAmount string `json:"amount_crypto"`
	ExchangeRate string `json:"exchange_rate"`
	Fiat         string `json:"fiat"`
	AmountFiat   string `json:"amount_fiat"`
	Crypto       string `json:"crypto"`
}
type SHKeeperPaymentRequest struct {
	Crypto      string `json:"-"`
	ExternalID  string `json:"external_id"`
	Fiat        string `json:"fiat"`
	Amount      string `json:"amount"`
	CallbackURL string `json:"callback_url"`
}

// The creation serializer may omit request metadata; explicit values are retained for validation.
type SHKeeperInvoice struct {
	ProviderID int64  `json:"id"`
	ExternalID string `json:"external_id"`
	Crypto     string `json:"crypto"`
	Fiat       string `json:"fiat"`
	AmountFiat string `json:"amount_fiat"`
	Amount     string `json:"amount"`
	Wallet     string `json:"wallet"`
}
type SHKeeperInvoiceTransaction struct {
	AmountUSDT string `json:"amount"`
	Crypto     string `json:"crypto"`
	Address    string `json:"addr"`
	TxID       string `json:"txid"`
	Status     string `json:"status"`
}

// Lookup follows v2.5.32 Invoice.to_json(), which has no invoice crypto amount or wallet.
type SHKeeperInvoiceLookup struct {
	ExternalID   string                       `json:"external_id"`
	Fiat         string                       `json:"fiat"`
	AmountFiat   string                       `json:"amount_fiat"`
	BalanceFiat  string                       `json:"balance_fiat"`
	Status       string                       `json:"status"`
	Transactions []SHKeeperInvoiceTransaction `json:"txs"`
}

func NewSHKeeperClient(config SHKeeperClientConfig, client *http.Client) (*SHKeeperClient, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.BackendKey = strings.TrimSpace(config.BackendKey)
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid SHKeeper base URL")
	}
	if config.APIKey == "" {
		return nil, errors.New("SHKeeper API key is required")
	}
	if err := validateSHKeeperHost(context.Background(), parsed.Hostname(), config.AllowPrivateURL); err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	if config.Timeout > 0 {
		copyClient.Timeout = config.Timeout
	} else if copyClient.Timeout == 0 {
		copyClient.Timeout = 15 * time.Second
	}
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0].Method != http.MethodGet {
			return errors.New("SHKeeper mutation redirects are forbidden")
		}
		if len(via) >= 10 || !strings.EqualFold(req.URL.Host, parsed.Host) || req.URL.Scheme != parsed.Scheme || req.URL.User != nil {
			return errors.New("SHKeeper redirect is not allowed")
		}
		return validateSHKeeperHost(req.Context(), req.URL.Hostname(), config.AllowPrivateURL)
	}
	transport := copyClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if base, ok := transport.(*http.Transport); ok {
		safe := base.Clone()
		safe.Proxy = nil
		safe.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, errors.New("SHKeeper hostname resolution failed")
			}
			for _, ip := range ips {
				if err := validateSHKeeperIP(ip.IP, config.AllowPrivateURL); err != nil {
					return nil, err
				}
			}
			dialer := net.Dialer{Timeout: 10 * time.Second}
			for _, ip := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if dialErr == nil {
					return conn, nil
				}
			}
			return nil, errors.New("SHKeeper connection failed")
		}
		safe.DialTLSContext = nil
		safe.DialTLS = nil
		copyClient.Transport = safe
	}
	return &SHKeeperClient{config: config, httpClient: &copyClient}, nil
}

func validateSHKeeperIP(ip net.IP, allowPrivate bool) error {
	if ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.Equal(net.ParseIP("100.100.100.200")) {
		return errors.New("SHKeeper metadata or link-local destination is forbidden")
	}
	if !allowPrivate && (ip.IsPrivate() || ip.IsLoopback()) {
		return errors.New("private SHKeeper URL requires explicit opt-in")
	}
	return nil
}

func validateSHKeeperHost(ctx context.Context, host string, allowPrivate bool) error {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "metadata.google.internal") {
		return errors.New("SHKeeper metadata destination is forbidden")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return errors.New("SHKeeper hostname resolution failed")
	}
	for _, ip := range ips {
		if err := validateSHKeeperIP(ip.IP, allowPrivate); err != nil {
			return err
		}
	}
	return nil
}

func (client *SHKeeperClient) request(ctx context.Context, method, path string, body any, backend bool, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return errors.New("invalid SHKeeper request")
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, client.config.BaseURL+path, reader)
	if err != nil {
		return errors.New("invalid SHKeeper request URL")
	}
	req.Header.Set("Content-Type", "application/json")
	if backend {
		req.Header.Set("X-Shkeeper-Backend-Key", client.config.BackendKey)
	} else {
		req.Header.Set("X-Shkeeper-Api-Key", client.config.APIKey)
	}
	response, err := client.httpClient.Do(req)
	if err != nil {
		return errors.New("SHKeeper request failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		return errors.New("cannot read SHKeeper response")
	}
	if len(data) > 2<<20 {
		return errors.New("SHKeeper response exceeds size limit")
	}
	var envelope struct {
		Status string `json:"status"`
	}
	// Never return provider bodies, transport errors, or JSON errors that could contain credentials.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("SHKeeper request rejected (HTTP %d)", response.StatusCode)
	}
	if common.Unmarshal(data, &envelope) != nil || envelope.Status != "success" {
		return errors.New("SHKeeper returned an unsuccessful or invalid response")
	}
	if out != nil && common.Unmarshal(data, out) != nil {
		return errors.New("invalid SHKeeper response payload")
	}
	return nil
}

func (client *SHKeeperClient) ListCrypto(ctx context.Context) ([]SHKeeperCrypto, error) {
	var response struct {
		Crypto     []string         `json:"crypto"`
		CryptoList []SHKeeperCrypto `json:"crypto_list"`
	}
	if err := client.request(ctx, http.MethodGet, "/api/v1/crypto", nil, false, &response); err != nil {
		return nil, err
	}
	if len(response.CryptoList) > 0 {
		return response.CryptoList, nil
	}
	result := make([]SHKeeperCrypto, 0, len(response.Crypto))
	for _, name := range response.Crypto {
		result = append(result, SHKeeperCrypto{Name: name, DisplayName: name})
	}
	return result, nil
}

func NormalizeSHKeeperCrypto(crypto string) (string, error) {
	crypto = strings.ToUpper(strings.TrimSpace(crypto))
	switch crypto {
	case operation_setting.SHKeeperCryptoUSDT, operation_setting.SHKeeperCryptoBNBUSDT, operation_setting.SHKeeperCryptoPolygonUSDT:
		return crypto, nil
	}
	return "", errors.New("unsupported SHKeeper crypto")
}

func (client *SHKeeperClient) Quote(ctx context.Context, crypto, fiat string, amount decimal.Decimal) (*SHKeeperQuote, error) {
	crypto, err := NormalizeSHKeeperCrypto(crypto)
	if err != nil {
		return nil, err
	}
	fiat = strings.ToUpper(strings.TrimSpace(fiat))
	if fiat != "USD" || !amount.IsPositive() {
		return nil, errors.New("invalid SHKeeper quote currency or amount")
	}
	result := &SHKeeperQuote{}
	err = client.request(ctx, http.MethodPost, "/api/v1/"+crypto+"/quote", map[string]string{"fiat": fiat, "amount": amount.String()}, false, result)
	return result, err
}

func (client *SHKeeperClient) CreatePaymentRequest(ctx context.Context, input SHKeeperPaymentRequest) (*SHKeeperInvoice, error) {
	crypto, err := NormalizeSHKeeperCrypto(input.Crypto)
	if err != nil {
		return nil, err
	}
	amount, err := decimal.NewFromString(input.Amount)
	if err != nil || !amount.IsPositive() || input.ExternalID == "" || input.Fiat != "USD" || input.CallbackURL == "" {
		return nil, errors.New("invalid SHKeeper payment request")
	}
	invoice := &SHKeeperInvoice{}
	if err := client.request(ctx, http.MethodPost, "/api/v1/"+crypto+"/payment_request", input, false, invoice); err != nil {
		return nil, err
	}
	if invoice.ExternalID == "" {
		invoice.ExternalID = input.ExternalID
	}
	if invoice.Fiat == "" {
		invoice.Fiat = input.Fiat
	}
	if invoice.Crypto == "" {
		invoice.Crypto = crypto
	}
	if invoice.AmountFiat == "" {
		invoice.AmountFiat = input.Amount
	}
	return invoice, nil
}

func (client *SHKeeperClient) GetInvoiceByExternalID(ctx context.Context, crypto, externalID string) (*SHKeeperInvoiceLookup, error) {
	if _, err := NormalizeSHKeeperCrypto(crypto); err != nil {
		return nil, err
	}
	if strings.TrimSpace(externalID) == "" {
		return nil, errors.New("SHKeeper external ID is required")
	}
	var response struct {
		Invoices []SHKeeperInvoiceLookup `json:"invoices"`
	}
	if err := client.request(ctx, http.MethodGet, "/api/v1/invoices/"+url.PathEscape(externalID), nil, false, &response); err != nil {
		return nil, err
	}
	if len(response.Invoices) == 0 {
		return nil, ErrSHKeeperInvoiceNotFound
	}
	if len(response.Invoices) != 1 || response.Invoices[0].ExternalID != externalID {
		return nil, errors.New("SHKeeper invoice identity mismatch")
	}
	return &response.Invoices[0], nil
}

func NormalizeSHKeeperTransactionID(crypto, transactionID string) (string, error) {
	crypto, err := NormalizeSHKeeperCrypto(crypto)
	if err != nil {
		return "", err
	}
	transactionID = strings.ToLower(strings.TrimSpace(transactionID))
	digits := transactionID
	if crypto != "USDT" {
		if !strings.HasPrefix(digits, "0x") {
			return "", errors.New("EVM transaction hash requires 0x prefix")
		}
		digits = digits[2:]
	}
	if len(digits) != 64 {
		return "", errors.New("invalid SHKeeper transaction hash length")
	}
	if _, err := hex.DecodeString(digits); err != nil {
		return "", errors.New("invalid SHKeeper transaction hash")
	}
	return transactionID, nil
}

func (client *SHKeeperClient) NotifyTransaction(ctx context.Context, crypto, transactionID string) error {
	crypto, err := NormalizeSHKeeperCrypto(crypto)
	if err != nil {
		return err
	}
	transactionID, err = NormalizeSHKeeperTransactionID(crypto, transactionID)
	if err != nil {
		return err
	}
	if client.config.BackendKey == "" {
		return errors.New("SHKeeper backend key is not configured")
	}
	var response struct {
		Message string `json:"message"`
	}
	if err := client.request(ctx, http.MethodPost, "/api/v1/walletnotify/"+crypto+"/"+transactionID, nil, true, &response); err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(response.Message), "not related") {
		return errors.New("SHKeeper transaction is not related to this invoice")
	}
	return nil
}

func VerifySHKeeperWebhook(apiKey, timestamp, signature string, rawBody []byte, now time.Time) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("SHKeeper webhook key is missing")
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid SHKeeper webhook timestamp")
	}
	age := now.Sub(time.Unix(unix, 0))
	if age > 5*time.Minute || age < -5*time.Minute {
		return errors.New("stale SHKeeper webhook timestamp")
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return errors.New("invalid SHKeeper webhook signature")
	}
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(timestamp + "."))
	mac.Write(rawBody)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return errors.New("invalid SHKeeper webhook signature")
	}
	return nil
}
