package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

type TokenPayClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

type TokenPayCreateRequest struct {
	OutOrderID   string `json:"OutOrderId"`
	OrderUserKey string `json:"OrderUserKey"`
	ActualAmount string `json:"ActualAmount"`
	Currency     string `json:"Currency"`
	NotifyURL    string `json:"NotifyUrl"`
	RedirectURL  string `json:"RedirectUrl"`
}

type TokenPayCreateResult struct {
	ProviderOrderID string
	OutOrderID      string
	OrderUserKey    string
	PayAmountUSDT   string
	ReceiveAddress  string
	PaymentURL      string
	ExpiresAt       int64
}

type TokenPayCallback struct {
	ID                 string `json:"Id"`
	OutOrderID         string `json:"OutOrderId"`
	OrderUserKey       string `json:"OrderUserKey"`
	Currency           string `json:"Currency"`
	BaseCurrency       string `json:"BaseCurrency"`
	BlockChainName     string `json:"BlockChainName"`
	CurrencyName       string `json:"CurrencyName"`
	Amount             string `json:"Amount"`
	ActualAmount       string `json:"ActualAmount"`
	PayAmount          string `json:"PayAmount"`
	ToAddress          string `json:"ToAddress"`
	BlockTransactionID string `json:"BlockTransactionId"`
	PayTime            string `json:"PayTime"`
	SignatureType      string `json:"SignatureType"`
	Status             int    `json:"Status"`
	IsCustomAmount     bool   `json:"IsCustomAmount"`
	IsDynamicAmount    bool   `json:"IsDynamicAmount"`
}

var tokenPayFieldBoundary = regexp.MustCompile(`&[A-Za-z_][A-Za-z0-9_]*=`)
var errTokenPaySubmissionUnknown = errors.New("TokenPay request failed; submission status may be unknown")
var ErrTokenPayInvoiceInvalid = errors.New("invalid TokenPay invoice")

func SignTokenPayParameters(secret string, fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if key != "Signature" && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join(parts, "&")))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyTokenPayCallback(secret string, body []byte) (TokenPayCallback, error) {
	var callback TokenPayCallback
	if secret == "" || len(body) == 0 || len(body) > 64<<10 {
		return callback, errors.New("invalid TokenPay callback")
	}
	if err := common.ValidateJSONTopLevelObjectUniqueKeys(body); err != nil {
		return callback, errors.New("invalid TokenPay callback object")
	}
	var raw map[string]json.RawMessage
	if err := common.Unmarshal(body, &raw); err != nil || len(raw) == 0 {
		return callback, errors.New("invalid TokenPay callback object")
	}
	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		kind := common.GetJsonType(value)
		if kind == "object" || kind == "array" || kind == "unknown" {
			return callback, errors.New("invalid TokenPay callback field")
		}
		text := common.JsonRawMessageToString(value)
		if tokenPayFieldBoundary.MatchString(text) || strings.ContainsAny(key, "&=") {
			return callback, errors.New("ambiguous TokenPay callback field")
		}
		fields[key] = text
	}
	received, err := hex.DecodeString(fields["Signature"])
	if err != nil || len(received) != sha256.Size {
		return callback, errors.New("invalid TokenPay callback signature")
	}
	expected, _ := hex.DecodeString(SignTokenPayParameters(secret, fields))
	if !hmac.Equal(received, expected) {
		return callback, errors.New("invalid TokenPay callback signature")
	}
	if err := common.Unmarshal(body, &callback); err != nil {
		return TokenPayCallback{}, errors.New("invalid TokenPay callback payload")
	}
	return callback, nil
}

func NewTokenPayClient(baseURL, secret string, allowPrivateURL bool, client *http.Client) (*TokenPayClient, error) {
	setting := operation_setting.TokenPayPaymentSetting{BaseURL: baseURL, APIToken: secret, AllowPrivateURL: allowPrivateURL}
	if err := setting.Normalize(); err != nil || setting.BaseURL == "" || setting.APIToken == "" {
		return nil, errors.New("invalid TokenPay client configuration")
	}
	parsed, _ := url.Parse(setting.BaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validateSHKeeperHost(ctx, parsed.Hostname(), allowPrivateURL); err != nil {
		return nil, errors.New("TokenPay destination is unavailable or forbidden")
	}
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	copyClient.Timeout = 15 * time.Second
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return errors.New("TokenPay redirects are forbidden")
	}
	transport := copyClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	base, ok := transport.(*http.Transport)
	if !ok {
		return nil, errors.New("TokenPay requires a guarded HTTP transport")
	}
	safe := base.Clone()
	safe.Proxy = nil
	safe.DialTLSContext = nil
	safe.DialTLS = nil
	safe.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid TokenPay destination")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("TokenPay destination resolution failed")
		}
		for _, ip := range ips {
			if err := validateSHKeeperIP(ip.IP, allowPrivateURL); err != nil {
				return nil, errors.New("forbidden TokenPay destination")
			}
		}
		dialer := net.Dialer{Timeout: 5 * time.Second}
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("TokenPay connection failed")
	}
	copyClient.Transport = safe
	return &TokenPayClient{baseURL: setting.BaseURL, secret: setting.APIToken, httpClient: &copyClient}, nil
}

func (client *TokenPayClient) request(ctx context.Context, method, path string, fields map[string]string) ([]byte, error) {
	for _, value := range fields {
		if tokenPayFieldBoundary.MatchString(value) {
			return nil, errors.New("ambiguous TokenPay request field")
		}
	}
	fields["Signature"] = SignTokenPayParameters(client.secret, fields)
	var reader io.Reader
	requestURL := client.baseURL + path
	if method == http.MethodGet {
		query := url.Values{}
		for key, value := range fields {
			query.Set(key, value)
		}
		requestURL += "?" + query.Encode()
	} else {
		data, err := common.Marshal(fields)
		if err != nil {
			return nil, errors.New("invalid TokenPay request")
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, errors.New("invalid TokenPay request URL")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, errTokenPaySubmissionUnknown
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("%w (HTTP %d)", errTokenPaySubmissionUnknown, resp.StatusCode)
		}
		return nil, fmt.Errorf("TokenPay request rejected (HTTP %d)", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(body) > 256<<10 {
		return nil, errors.New("invalid TokenPay response size")
	}
	var envelope struct {
		Success bool `json:"success"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil || !envelope.Success {
		return nil, errors.New("TokenPay returned an unsuccessful response")
	}
	return body, nil
}

func (client *TokenPayClient) CreateOrder(ctx context.Context, input TokenPayCreateRequest) (TokenPayCreateResult, error) {
	var result TokenPayCreateResult
	amount, err := decimal.NewFromString(input.ActualAmount)
	if err != nil || !amount.IsPositive() || !amount.Equal(amount.Truncate(0)) || input.OutOrderID == "" || input.OrderUserKey == "" {
		return result, errors.New("invalid TokenPay fixed-package request")
	}
	chain := ""
	switch input.Currency {
	case operation_setting.TokenPayNetworkTRON:
		chain = "TRON"
	case operation_setting.TokenPayNetworkBSC:
		chain = "BSC"
	case operation_setting.TokenPayNetworkPolygon:
		chain = "Polygon"
	default:
		return result, errors.New("unsupported TokenPay network")
	}
	fields := map[string]string{
		"OutOrderId": input.OutOrderID, "OrderUserKey": input.OrderUserKey, "ActualAmount": input.ActualAmount,
		"Currency": input.Currency, "NotifyUrl": input.NotifyURL, "RedirectUrl": input.RedirectURL,
	}
	var data []byte
	for attempt := 0; attempt < 2; attempt++ {
		data, err = client.request(ctx, http.MethodPost, "/CreateOrder", fields)
		if err == nil || !errors.Is(err, errTokenPaySubmissionUnknown) || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return result, err
	}
	var response struct {
		Data string `json:"data"`
		Info struct {
			ID             string `json:"Id"`
			OutOrderID     string `json:"OutOrderId"`
			OrderUserKey   string `json:"OrderUserKey"`
			Amount         string `json:"Amount"`
			ActualAmount   string `json:"ActualAmount"`
			BaseCurrency   string `json:"BaseCurrency"`
			BlockChainName string `json:"BlockChainName"`
			CurrencyName   string `json:"CurrencyName"`
			ToAddress      string `json:"ToAddress"`
			ExpireTime     string `json:"ExpireTime"`
			ExpireTimeUnix int64  `json:"ExpireTimeUnix"`
			IsCustomAmount bool   `json:"IsCustomAmount"`
		} `json:"info"`
	}
	if common.Unmarshal(data, &response) != nil {
		return result, fmt.Errorf("%w: response", ErrTokenPayInvoiceInvalid)
	}
	info := response.Info
	quoted, quoteErr := decimal.NewFromString(info.Amount)
	original, originalErr := decimal.NewFromString(info.ActualAmount)
	if quoteErr != nil || originalErr != nil || !quoted.Equal(amount) || !original.Equal(amount) || info.BaseCurrency != "USD" || info.CurrencyName != "USDT" || info.BlockChainName != chain || info.OutOrderID != input.OutOrderID || info.OrderUserKey != input.OrderUserKey || info.ID == "" || info.IsCustomAmount {
		return result, fmt.Errorf("%w: identity or fixed amount mismatch", ErrTokenPayInvoiceInvalid)
	}
	address := info.ToAddress
	if chain == "TRON" && (len(address) != 34 || !strings.HasPrefix(address, "T")) || chain != "TRON" && !regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`).MatchString(address) {
		return result, fmt.Errorf("%w: receiving address", ErrTokenPayInvoiceInvalid)
	}
	checkout, checkoutErr := url.Parse(response.Data)
	base, _ := url.Parse(client.baseURL)
	if checkoutErr != nil || checkout.Scheme != "https" || !strings.EqualFold(checkout.Host, base.Host) || checkout.User != nil || checkout.Fragment != "" || checkout.Path != "/Pay" || checkout.Query().Get("Id") != info.ID {
		return result, fmt.Errorf("%w: hosted checkout URL", ErrTokenPayInvoiceInvalid)
	}
	if info.ExpireTimeUnix <= time.Now().Unix() {
		return result, fmt.Errorf("%w: expiry", ErrTokenPayInvoiceInvalid)
	}
	return TokenPayCreateResult{ProviderOrderID: info.ID, OutOrderID: input.OutOrderID, OrderUserKey: input.OrderUserKey,
		PayAmountUSDT: quoted.String(), ReceiveAddress: address, PaymentURL: response.Data, ExpiresAt: info.ExpireTimeUnix}, nil
}

func (client *TokenPayClient) QueryOrder(ctx context.Context, providerID string) (TokenPayCallback, error) {
	var result TokenPayCallback
	if providerID == "" {
		return result, errors.New("TokenPay provider order ID is required")
	}
	body, err := client.request(ctx, http.MethodGet, "/Query", map[string]string{"Id": providerID})
	if err != nil {
		return result, err
	}
	var response struct {
		Data TokenPayCallback `json:"data"`
	}
	if common.Unmarshal(body, &response) != nil || response.Data.ID != providerID {
		return result, errors.New("invalid TokenPay order query")
	}
	return response.Data, nil
}

func TokenPayOrderUserKey(tradeNo, network string) string {
	return tradeNo + "-" + network
}

func TokenPayPaymentAmount(usdt int64) string {
	return strconv.FormatInt(usdt, 10)
}

func NormalizeTokenPayTransactionID(network, transactionID string) (string, error) {
	transactionID = strings.ToLower(strings.TrimSpace(transactionID))
	digits := transactionID
	switch network {
	case operation_setting.TokenPayNetworkTRON:
		if strings.HasPrefix(digits, "0x") {
			return "", errors.New("TRON transaction hash must not have 0x prefix")
		}
	case operation_setting.TokenPayNetworkBSC, operation_setting.TokenPayNetworkPolygon:
		if !strings.HasPrefix(digits, "0x") {
			return "", errors.New("EVM transaction hash requires 0x prefix")
		}
		digits = digits[2:]
	default:
		return "", errors.New("unsupported TokenPay network")
	}
	if len(digits) != 64 {
		return "", errors.New("invalid TokenPay transaction hash length")
	}
	if _, err := hex.DecodeString(digits); err != nil {
		return "", errors.New("invalid TokenPay transaction hash")
	}
	return transactionID, nil
}
