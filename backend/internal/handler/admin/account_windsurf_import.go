package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/gin-gonic/gin"
)

const (
	windsurfAdapterResponseMaxBytes = 1 << 20

	envWindsurfAdapterBaseURL        = "WINDSURF_ADAPTER_INTERNAL_BASE_URL"
	envWindsurfAdapterAPIKey         = "WINDSURF_ADAPTER_INTERNAL_API_KEY"
	envWindsurfAdapterTimeoutSeconds = "WINDSURF_ADAPTER_TIMEOUT_SECONDS"

	envProviderAdaptersWindsurfBaseURL        = "PROVIDER_ADAPTERS_WINDSURF_INTERNAL_BASE_URL"
	envProviderAdaptersWindsurfAPIKey         = "PROVIDER_ADAPTERS_WINDSURF_INTERNAL_API_KEY"
	envProviderAdaptersWindsurfTimeoutSeconds = "PROVIDER_ADAPTERS_WINDSURF_TIMEOUT_SECONDS"
)

var windsurfSensitiveKeys = []string{
	"token",
	"api_key",
	"apikey",
	"apiKey",
	"authorization",
	"x-api-key",
	"session_token",
}

type windsurfAdapterConfig struct {
	InternalBaseURL string
	InternalAPIKey  string
	Timeout         time.Duration
}

type WindsurfImportRequest struct {
	Email    string                  `json:"email"`
	Password string                  `json:"password"`
	Token    string                  `json:"token"`
	Tokens   []string                `json:"tokens"`
	Raw      string                  `json:"raw"`
	Accounts []WindsurfImportAccount `json:"accounts"`
}

type WindsurfImportAccount struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Label    string `json:"label,omitempty"`
	Proxy    string `json:"proxy,omitempty"`
}

type WindsurfImportResult struct {
	Total          int `json:"total"`
	Forwarded      int `json:"forwarded"`
	DuplicateCount int `json:"duplicate_count"`
	UpstreamStatus int `json:"upstream_status"`
	Upstream       any `json:"upstream,omitempty"`
}

type windsurfForwardAccount struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Label    string `json:"label,omitempty"`
	Proxy    string `json:"proxy,omitempty"`
}

type windsurfImportIdempotencyItem struct {
	SecretHash string `json:"secret_hash"`
	EmailHash  string `json:"email_hash,omitempty"`
	Kind       string `json:"kind"`
	Label      string `json:"label,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
}

type windsurfImportIdempotencyPayload struct {
	Provider       string                          `json:"provider"`
	Accounts       []windsurfImportIdempotencyItem `json:"accounts"`
	DuplicateCount int                             `json:"duplicate_count"`
}

func (h *AccountHandler) ImportWindsurfTokens(c *gin.Context) {
	var req WindsurfImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accounts, duplicateCount, err := parseWindsurfImportAccounts(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(accounts) == 0 {
		response.BadRequest(c, "请输入 Windsurf token 或 api_key")
		return
	}

	cfg := loadWindsurfAdapterConfigFromEnv()
	if err := validateWindsurfAdapterConfig(cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	idempotencyPayload := buildWindsurfImportIdempotencyPayload(accounts, duplicateCount)
	executeAdminIdempotentJSON(c, "admin.accounts.import_windsurf", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return importWindsurfAccounts(ctx, cfg, accounts, duplicateCount)
	})
}

func importWindsurfAccounts(ctx context.Context, cfg windsurfAdapterConfig, accounts []windsurfForwardAccount, duplicateCount int) (WindsurfImportResult, error) {
	result := WindsurfImportResult{
		Total:          len(accounts),
		Forwarded:      len(accounts),
		DuplicateCount: duplicateCount,
	}

	body, err := json.Marshal(map[string]any{"accounts": accounts})
	if err != nil {
		return result, infraerrors.InternalServer("WINDSURF_IMPORT_MARSHAL_FAILED", "failed to build windsurf import request").WithCause(err)
	}

	endpoint := strings.TrimRight(cfg.InternalBaseURL, "/") + "/auth/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, infraerrors.InternalServer("WINDSURF_IMPORT_REQUEST_FAILED", "failed to build windsurf import request").WithCause(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.InternalAPIKey)
	req.Header.Set("x-api-key", cfg.InternalAPIKey)

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return result, infraerrors.Newf(http.StatusBadGateway, "WINDSURF_IMPORT_UPSTREAM_REQUEST_FAILED", "windsurf adapter request failed: %s", logredact.RedactText(err.Error(), windsurfSensitiveKeys...))
	}
	defer resp.Body.Close()

	respBody, err := readLimitedWindsurfAdapterResponse(resp.Body)
	if err != nil {
		return result, err
	}
	result.UpstreamStatus = resp.StatusCode
	result.Upstream = sanitizeWindsurfAdapterResponse(respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, infraerrors.Newf(http.StatusBadGateway, "WINDSURF_IMPORT_UPSTREAM_FAILED", "windsurf adapter returned status %d", resp.StatusCode)
	}
	return result, nil
}

func parseWindsurfImportAccounts(req WindsurfImportRequest) ([]windsurfForwardAccount, int, error) {
	accounts := make([]windsurfForwardAccount, 0, 1+len(req.Tokens)+len(req.Accounts))
	seen := map[string]struct{}{}
	duplicateCount := 0

	add := func(account windsurfForwardAccount) error {
		account.Token = strings.TrimSpace(account.Token)
		account.APIKey = strings.TrimSpace(account.APIKey)
		account.Email = strings.TrimSpace(account.Email)
		account.Password = strings.TrimSpace(account.Password)
		account.Label = strings.TrimSpace(account.Label)
		account.Proxy = strings.TrimSpace(account.Proxy)
		if account.Token == "" && account.APIKey == "" && account.Email == "" && account.Password == "" {
			return nil
		}

		kind := "token"
		secret := account.Token
		email := ""
		methods := 0
		if account.Token != "" {
			methods++
		}
		if account.APIKey != "" {
			methods++
			kind = "api_key"
			secret = account.APIKey
		}
		if account.Email != "" || account.Password != "" {
			methods++
			kind = "email_password"
			email = strings.ToLower(account.Email)
			secret = account.Password
			if account.Email == "" || account.Password == "" {
				return fmt.Errorf("Windsurf email/password 导入必须同时提供 email 和 password")
			}
		}
		if methods > 1 {
			return fmt.Errorf("同一个 Windsurf account 只能提供 token、api_key 或 email/password 其中一种")
		}
		key := kind + ":" + secret
		if email != "" {
			key = kind + ":" + email + ":" + secret
		}
		if _, ok := seen[key]; ok {
			duplicateCount++
			return nil
		}
		seen[key] = struct{}{}
		accounts = append(accounts, account)
		return nil
	}

	if err := add(windsurfForwardAccount{Token: req.Token}); err != nil {
		return nil, duplicateCount, err
	}
	if err := add(windsurfForwardAccount{Email: req.Email, Password: req.Password}); err != nil {
		return nil, duplicateCount, err
	}
	for _, token := range req.Tokens {
		if err := add(windsurfForwardAccount{Token: token}); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, account := range req.Accounts {
		if err := add(windsurfForwardAccount{
			Email:    account.Email,
			Password: account.Password,
			Token:    account.Token,
			APIKey:   account.APIKey,
			Label:    account.Label,
			Proxy:    account.Proxy,
		}); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, account := range parseWindsurfRawAccounts(req.Raw) {
		if err := add(account); err != nil {
			return nil, duplicateCount, err
		}
	}

	return accounts, duplicateCount, nil
}

func parseWindsurfRawAccounts(raw string) []windsurfForwardAccount {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	accounts := make([]windsurfForwardAccount, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" {
			continue
		}
		if email, password, ok := strings.Cut(item, "----"); ok {
			accounts = append(accounts, windsurfForwardAccount{
				Email:    strings.TrimSpace(email),
				Password: strings.TrimSpace(password),
			})
			continue
		}
		accounts = append(accounts, windsurfForwardAccount{Token: item})
	}
	return accounts
}

func buildWindsurfImportIdempotencyPayload(accounts []windsurfForwardAccount, duplicateCount int) windsurfImportIdempotencyPayload {
	items := make([]windsurfImportIdempotencyItem, 0, len(accounts))
	for _, account := range accounts {
		kind := "token"
		secret := account.Token
		emailHash := ""
		if account.APIKey != "" {
			kind = "api_key"
			secret = account.APIKey
		}
		if account.Email != "" {
			kind = "email_password"
			secret = account.Password
			emailHash = hashWindsurfSecret(strings.ToLower(strings.TrimSpace(account.Email)))
		}
		items = append(items, windsurfImportIdempotencyItem{
			SecretHash: hashWindsurfSecret(secret),
			EmailHash:  emailHash,
			Kind:       kind,
			Label:      account.Label,
			Proxy:      account.Proxy,
		})
	}
	return windsurfImportIdempotencyPayload{
		Provider:       "windsurf",
		Accounts:       items,
		DuplicateCount: duplicateCount,
	}
}

func hashWindsurfSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func loadWindsurfAdapterConfigFromEnv() windsurfAdapterConfig {
	timeout := 30 * time.Second
	if raw := firstEnv(envWindsurfAdapterTimeoutSeconds, envProviderAdaptersWindsurfTimeoutSeconds); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	return windsurfAdapterConfig{
		InternalBaseURL: firstEnv(envWindsurfAdapterBaseURL, envProviderAdaptersWindsurfBaseURL),
		InternalAPIKey:  firstEnv(envWindsurfAdapterAPIKey, envProviderAdaptersWindsurfAPIKey),
		Timeout:         timeout,
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func validateWindsurfAdapterConfig(cfg windsurfAdapterConfig) error {
	if strings.TrimSpace(cfg.InternalBaseURL) == "" {
		return infraerrors.ServiceUnavailable("WINDSURF_ADAPTER_NOT_CONFIGURED", "windsurf adapter internal base URL is not configured")
	}
	if strings.TrimSpace(cfg.InternalAPIKey) == "" {
		return infraerrors.ServiceUnavailable("WINDSURF_ADAPTER_NOT_CONFIGURED", "windsurf adapter internal API key is not configured")
	}
	if cfg.Timeout <= 0 {
		return infraerrors.BadRequest("WINDSURF_ADAPTER_INVALID_TIMEOUT", "windsurf adapter timeout must be greater than 0")
	}
	return nil
}

func readLimitedWindsurfAdapterResponse(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, windsurfAdapterResponseMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "WINDSURF_IMPORT_UPSTREAM_READ_FAILED", "failed to read windsurf adapter response: %s", logredact.RedactText(err.Error(), windsurfSensitiveKeys...))
	}
	if len(data) > windsurfAdapterResponseMaxBytes {
		return nil, infraerrors.New(http.StatusBadGateway, "WINDSURF_IMPORT_UPSTREAM_RESPONSE_TOO_LARGE", "windsurf adapter response is too large")
	}
	return data, nil
}

func sanitizeWindsurfAdapterResponse(raw []byte) any {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{
			"body": logredact.RedactText(string(raw), windsurfSensitiveKeys...),
		}
	}
	return sanitizeWindsurfAdapterValue(value)
}

func sanitizeWindsurfAdapterValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			if isWindsurfSensitiveKey(key) {
				out[key] = "***"
				continue
			}
			out[key] = sanitizeWindsurfAdapterValue(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = sanitizeWindsurfAdapterValue(item)
		}
		return out
	default:
		return value
	}
}

func isWindsurfSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "token", "api_key", "apikey", "authorization", "x_api_key", "session_token":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password")
}
