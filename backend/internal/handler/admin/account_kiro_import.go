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
	"sort"
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
	kiroAdapterResponseMaxBytes = 1 << 20

	envKiroAdapterBaseURL             = "KIRO_ADAPTER_INTERNAL_BASE_URL"
	envKiroAdapterRuntimeBaseURL      = "KIRO_ADAPTER_RUNTIME_BASE_URL"
	envKiroAdapterAdminAPIKey         = "KIRO_ADAPTER_ADMIN_API_KEY"
	envKiroAdapterInternalAPIKey      = "KIRO_ADAPTER_INTERNAL_API_KEY"
	envKiroAdapterTimeoutSeconds      = "KIRO_ADAPTER_TIMEOUT_SECONDS"
	envProviderAdaptersKiroBaseURL    = "PROVIDER_ADAPTERS_KIRO_INTERNAL_BASE_URL"
	envProviderAdaptersKiroRuntimeURL = "PROVIDER_ADAPTERS_KIRO_RUNTIME_BASE_URL"
	envProviderAdaptersKiroAPIKey     = "PROVIDER_ADAPTERS_KIRO_ADMIN_API_KEY"
	envProviderAdaptersKiroTimeoutSec = "PROVIDER_ADAPTERS_KIRO_TIMEOUT_SECONDS"
)

var kiroSensitiveKeys = []string{
	"authorization",
	"x-api-key",
	"token",
	"access_token",
	"accessToken",
	"refresh_token",
	"refreshToken",
	"client_secret",
	"clientSecret",
	"api_key",
	"apiKey",
	"apikey",
	"kiro_api_key",
	"kiroApiKey",
	"proxy_password",
	"proxyPassword",
}

type kiroAdapterConfig struct {
	InternalBaseURL string
	RuntimeBaseURL  string
	AdminAPIKey     string
	InternalAPIKey  string
	Timeout         time.Duration
}

type KiroImportRequest struct {
	RefreshToken       string              `json:"refresh_token"`
	RefreshTokenCamel  string              `json:"refreshToken"`
	RefreshTokens      []string            `json:"refresh_tokens"`
	RefreshTokensCamel []string            `json:"refreshTokens"`
	KiroAPIKey         string              `json:"kiro_api_key"`
	KiroAPIKeyCamel    string              `json:"kiroApiKey"`
	APIKey             string              `json:"api_key"`
	APIKeyCamel        string              `json:"apiKey"`
	APIKeyFlat         string              `json:"apikey"`
	APIKeys            []string            `json:"api_keys"`
	APIKeysCamel       []string            `json:"apiKeys"`
	KiroAPIKeys        []string            `json:"kiro_api_keys"`
	KiroAPIKeysCamel   []string            `json:"kiroApiKeys"`
	Raw                string              `json:"raw"`
	Accounts           []KiroImportAccount `json:"accounts"`
	GroupIDs           []int64             `json:"group_ids"`
}

type KiroImportAccount struct {
	RefreshToken       string             `json:"refresh_token,omitempty"`
	RefreshTokenCamel  string             `json:"refreshToken,omitempty"`
	AccessToken        string             `json:"access_token,omitempty"`
	AccessTokenCamel   string             `json:"accessToken,omitempty"`
	KiroAPIKey         string             `json:"kiro_api_key,omitempty"`
	KiroAPIKeyCamel    string             `json:"kiroApiKey,omitempty"`
	APIKey             string             `json:"api_key,omitempty"`
	APIKeyCamel        string             `json:"apiKey,omitempty"`
	APIKeyFlat         string             `json:"apikey,omitempty"`
	AuthMethod         string             `json:"auth_method,omitempty"`
	AuthMethodCamel    string             `json:"authMethod,omitempty"`
	ClientID           string             `json:"client_id,omitempty"`
	ClientIDCamel      string             `json:"clientId,omitempty"`
	ClientSecret       string             `json:"client_secret,omitempty"`
	ClientSecretCamel  string             `json:"clientSecret,omitempty"`
	Priority           uint32             `json:"priority,omitempty"`
	Region             string             `json:"region,omitempty"`
	AuthRegion         string             `json:"auth_region,omitempty"`
	AuthRegionCamel    string             `json:"authRegion,omitempty"`
	APIRegion          string             `json:"api_region,omitempty"`
	APIRegionCamel     string             `json:"apiRegion,omitempty"`
	MachineID          string             `json:"machine_id,omitempty"`
	MachineIDCamel     string             `json:"machineId,omitempty"`
	Email              string             `json:"email,omitempty"`
	LoginHint          string             `json:"login_hint,omitempty"`
	LoginHintCamel     string             `json:"loginHint,omitempty"`
	ProfileARN         string             `json:"profile_arn,omitempty"`
	ProfileARNCamel    string             `json:"profileArn,omitempty"`
	ExpiresAt          kiroFlexibleString `json:"expires_at,omitempty"`
	ExpiresAtCamel     kiroFlexibleString `json:"expiresAt,omitempty"`
	ProxyURL           string             `json:"proxy_url,omitempty"`
	ProxyURLCamel      string             `json:"proxyUrl,omitempty"`
	ProxyUsername      string             `json:"proxy_username,omitempty"`
	ProxyUsernameCamel string             `json:"proxyUsername,omitempty"`
	ProxyPassword      string             `json:"proxy_password,omitempty"`
	ProxyPasswordCamel string             `json:"proxyPassword,omitempty"`
	Endpoint           string             `json:"endpoint,omitempty"`
	AuthTokenRaw       KiroAuthTokenRaw   `json:"kiro_auth_token_raw,omitempty"`
}

type KiroAuthTokenRaw struct {
	AccessToken       string             `json:"access_token,omitempty"`
	AccessTokenCamel  string             `json:"accessToken,omitempty"`
	RefreshToken      string             `json:"refresh_token,omitempty"`
	RefreshTokenCamel string             `json:"refreshToken,omitempty"`
	Email             string             `json:"email,omitempty"`
	LoginHint         string             `json:"login_hint,omitempty"`
	LoginHintCamel    string             `json:"loginHint,omitempty"`
	ProfileARN        string             `json:"profile_arn,omitempty"`
	ProfileARNCamel   string             `json:"profileArn,omitempty"`
	ExpiresAt         kiroFlexibleString `json:"expires_at,omitempty"`
	ExpiresAtCamel    kiroFlexibleString `json:"expiresAt,omitempty"`
	UserID            string             `json:"user_id,omitempty"`
	UserIDCamel       string             `json:"userId,omitempty"`
}

type kiroFlexibleString string

func (value *kiroFlexibleString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*value = ""
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*value = kiroFlexibleString(asString)
		return nil
	}

	var asNumber json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&asNumber); err == nil {
		*value = kiroFlexibleString(asNumber.String())
		return nil
	}

	return fmt.Errorf("expected string, number, or null")
}

func (value kiroFlexibleString) text() string {
	return strings.TrimSpace(string(value))
}

type KiroImportResult struct {
	Total          int                              `json:"total"`
	Forwarded      int                              `json:"forwarded"`
	Succeeded      int                              `json:"succeeded"`
	Failed         int                              `json:"failed"`
	DuplicateCount int                              `json:"duplicate_count"`
	Items          []KiroImportItem                 `json:"items"`
	Upstream       *KiroImportUpstreamAccountResult `json:"upstream,omitempty"`
}

type KiroImportUpstreamAccountResult struct {
	AccountID int64   `json:"account_id"`
	Action    string  `json:"action"`
	GroupIDs  []int64 `json:"group_ids,omitempty"`
	BaseURL   string  `json:"base_url,omitempty"`
}

type KiroImportItem struct {
	Index          int    `json:"index"`
	Kind           string `json:"kind"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	Upstream       any    `json:"upstream,omitempty"`
}

type kiroForwardCredential struct {
	AccessToken   string `json:"accessToken,omitempty"`
	RefreshToken  string `json:"refreshToken,omitempty"`
	ProfileARN    string `json:"profileArn,omitempty"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
	AuthMethod    string `json:"authMethod,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	ClientSecret  string `json:"clientSecret,omitempty"`
	Priority      uint32 `json:"priority,omitempty"`
	Region        string `json:"region,omitempty"`
	AuthRegion    string `json:"authRegion,omitempty"`
	APIRegion     string `json:"apiRegion,omitempty"`
	MachineID     string `json:"machineId,omitempty"`
	Email         string `json:"email,omitempty"`
	ProxyURL      string `json:"proxyUrl,omitempty"`
	ProxyUsername string `json:"proxyUsername,omitempty"`
	ProxyPassword string `json:"proxyPassword,omitempty"`
	KiroAPIKey    string `json:"kiroApiKey,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
}

type kiroImportIdempotencyItem struct {
	SecretHash string `json:"secret_hash"`
	EmailHash  string `json:"email_hash,omitempty"`
	Kind       string `json:"kind"`
	AuthMethod string `json:"auth_method,omitempty"`
	Priority   uint32 `json:"priority,omitempty"`
	Region     string `json:"region,omitempty"`
	AuthRegion string `json:"auth_region,omitempty"`
	APIRegion  string `json:"api_region,omitempty"`
	ProxyURL   string `json:"proxy_url,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
}

type kiroImportIdempotencyPayload struct {
	Provider       string                      `json:"provider"`
	Credentials    []kiroImportIdempotencyItem `json:"credentials"`
	DuplicateCount int                         `json:"duplicate_count"`
	GroupIDs       []int64                     `json:"group_ids,omitempty"`
}

func (h *AccountHandler) ImportKiroCredentials(c *gin.Context) {
	var req KiroImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	credentials, duplicateCount, err := parseKiroImportCredentials(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(credentials) == 0 {
		response.BadRequest(c, "请输入 Kiro refreshToken 或 API Key")
		return
	}

	cfg := loadKiroAdapterConfigFromEnv()
	if err := validateKiroAdapterConfig(cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	groupIDs := normalizeKiroGroupIDs(req.GroupIDs)
	idempotencyPayload := buildKiroImportIdempotencyPayload(credentials, duplicateCount, groupIDs)
	executeAdminIdempotentJSON(c, "admin.accounts.import_kiro", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := importKiroCredentials(ctx, cfg, credentials, duplicateCount)
		if err != nil {
			return result, err
		}
		upstream, err := h.ensureKiroRuntimeUpstreamAccount(ctx, cfg, groupIDs)
		if err != nil {
			return result, err
		}
		result.Upstream = upstream
		return result, nil
	})
}

func importKiroCredentials(ctx context.Context, cfg kiroAdapterConfig, credentials []kiroForwardCredential, duplicateCount int) (KiroImportResult, error) {
	result := KiroImportResult{
		Total:          len(credentials),
		Forwarded:      len(credentials),
		DuplicateCount: duplicateCount,
		Items:          make([]KiroImportItem, 0, len(credentials)),
	}

	endpoint := strings.TrimRight(cfg.InternalBaseURL, "/") + "/api/admin/credentials"
	client := &http.Client{Timeout: cfg.Timeout}
	for i, credential := range credentials {
		normalized, ok, err := normalizeKiroForwardCredential(credential)
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, KiroImportItem{Index: i, Kind: "unknown", Error: err.Error()})
			continue
		}
		if !ok {
			result.Failed++
			result.Items = append(result.Items, KiroImportItem{Index: i, Kind: "unknown", Error: "empty kiro credential"})
			continue
		}
		credential = normalized
		kind, _ := kiroCredentialKindAndSecret(credential)
		item := KiroImportItem{Index: i, Kind: kind}

		body, err := json.Marshal(credential)
		if err != nil {
			item.Error = "failed to build kiro import request"
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return result, infraerrors.InternalServer("KIRO_IMPORT_REQUEST_FAILED", "failed to build kiro import request").WithCause(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.AdminAPIKey)
		req.Header.Set("x-api-key", cfg.AdminAPIKey)

		resp, err := client.Do(req)
		if err != nil {
			item.Error = fmt.Sprintf("kiro adapter request failed: %s", logredact.RedactText(err.Error(), kiroSensitiveKeys...))
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		respBody, readErr := readLimitedKiroAdapterResponse(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			item.Error = readErr.Error()
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		item.UpstreamStatus = resp.StatusCode
		item.Upstream = sanitizeKiroAdapterResponse(respBody)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			item.Error = fmt.Sprintf("kiro adapter returned status %d", resp.StatusCode)
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		item.Success = true
		result.Succeeded++
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func parseKiroImportCredentials(req KiroImportRequest) ([]kiroForwardCredential, int, error) {
	credentials := make([]kiroForwardCredential, 0, 1+len(req.RefreshTokens)+len(req.Accounts))
	seen := map[string]struct{}{}
	duplicateCount := 0

	add := func(credential kiroForwardCredential) error {
		normalized, ok, err := normalizeKiroForwardCredential(credential)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		kind, secret := kiroCredentialKindAndSecret(normalized)
		key := kind + ":" + secret
		if _, exists := seen[key]; exists {
			duplicateCount++
			return nil
		}
		seen[key] = struct{}{}
		credentials = append(credentials, normalized)
		return nil
	}

	if refreshToken := firstNonEmptyString(req.RefreshToken, req.RefreshTokenCamel); refreshToken != "" {
		if err := add(kiroForwardCredential{RefreshToken: refreshToken}); err != nil {
			return nil, duplicateCount, err
		}
	}
	if apiKey := firstNonEmptyString(req.KiroAPIKey, req.KiroAPIKeyCamel, req.APIKey, req.APIKeyCamel, req.APIKeyFlat); apiKey != "" {
		if err := add(kiroForwardCredential{KiroAPIKey: apiKey}); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, refreshToken := range append(req.RefreshTokens, req.RefreshTokensCamel...) {
		if err := add(kiroForwardCredential{RefreshToken: refreshToken}); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, apiKey := range append(append(req.KiroAPIKeys, req.KiroAPIKeysCamel...), append(req.APIKeys, req.APIKeysCamel...)...) {
		if err := add(kiroForwardCredential{KiroAPIKey: apiKey}); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, account := range req.Accounts {
		if err := add(normalizeKiroImportAccount(account)); err != nil {
			return nil, duplicateCount, err
		}
	}
	for _, credential := range parseKiroRawCredentials(req.Raw) {
		if err := add(credential); err != nil {
			return nil, duplicateCount, err
		}
	}

	return credentials, duplicateCount, nil
}

func normalizeKiroImportAccount(account KiroImportAccount) kiroForwardCredential {
	nested := account.AuthTokenRaw
	return kiroForwardCredential{
		AccessToken:   firstNonEmptyString(account.AccessToken, account.AccessTokenCamel, nested.AccessToken, nested.AccessTokenCamel),
		RefreshToken:  firstNonEmptyString(account.RefreshToken, account.RefreshTokenCamel, nested.RefreshToken, nested.RefreshTokenCamel),
		ProfileARN:    firstNonEmptyString(account.ProfileARN, account.ProfileARNCamel, nested.ProfileARN, nested.ProfileARNCamel),
		ExpiresAt:     normalizeKiroExpiresAt(firstNonEmptyKiroFlexibleString(account.ExpiresAt, account.ExpiresAtCamel, nested.ExpiresAt, nested.ExpiresAtCamel)),
		AuthMethod:    firstNonEmptyString(account.AuthMethod, account.AuthMethodCamel),
		ClientID:      firstNonEmptyString(account.ClientID, account.ClientIDCamel),
		ClientSecret:  firstNonEmptyString(account.ClientSecret, account.ClientSecretCamel),
		Priority:      account.Priority,
		Region:        account.Region,
		AuthRegion:    firstNonEmptyString(account.AuthRegion, account.AuthRegionCamel),
		APIRegion:     firstNonEmptyString(account.APIRegion, account.APIRegionCamel),
		MachineID:     firstNonEmptyString(account.MachineID, account.MachineIDCamel),
		Email:         firstNonEmptyString(account.Email, account.LoginHint, account.LoginHintCamel, nested.Email, nested.LoginHint, nested.LoginHintCamel),
		ProxyURL:      firstNonEmptyString(account.ProxyURL, account.ProxyURLCamel),
		ProxyUsername: firstNonEmptyString(account.ProxyUsername, account.ProxyUsernameCamel),
		ProxyPassword: firstNonEmptyString(account.ProxyPassword, account.ProxyPasswordCamel),
		KiroAPIKey:    firstNonEmptyString(account.KiroAPIKey, account.KiroAPIKeyCamel, account.APIKey, account.APIKeyCamel, account.APIKeyFlat),
		Endpoint:      account.Endpoint,
	}
}

func normalizeKiroForwardCredential(credential kiroForwardCredential) (kiroForwardCredential, bool, error) {
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.RefreshToken = strings.TrimSpace(credential.RefreshToken)
	credential.ProfileARN = strings.TrimSpace(credential.ProfileARN)
	credential.ExpiresAt = normalizeKiroExpiresAt(credential.ExpiresAt)
	credential.AuthMethod = canonicalKiroAuthMethod(strings.TrimSpace(credential.AuthMethod))
	credential.ClientID = strings.TrimSpace(credential.ClientID)
	credential.ClientSecret = strings.TrimSpace(credential.ClientSecret)
	credential.Region = strings.TrimSpace(credential.Region)
	credential.AuthRegion = strings.TrimSpace(credential.AuthRegion)
	credential.APIRegion = strings.TrimSpace(credential.APIRegion)
	credential.MachineID = strings.TrimSpace(credential.MachineID)
	credential.Email = strings.TrimSpace(credential.Email)
	credential.ProxyURL = strings.TrimSpace(credential.ProxyURL)
	credential.ProxyUsername = strings.TrimSpace(credential.ProxyUsername)
	credential.ProxyPassword = strings.TrimSpace(credential.ProxyPassword)
	credential.KiroAPIKey = strings.TrimSpace(credential.KiroAPIKey)
	credential.Endpoint = strings.TrimSpace(credential.Endpoint)

	if credential.RefreshToken == "" && credential.KiroAPIKey == "" {
		return credential, false, nil
	}
	if credential.RefreshToken != "" && credential.KiroAPIKey != "" {
		return credential, false, fmt.Errorf("同一个 Kiro credential 只能提供 refreshToken 或 kiroApiKey 其中一种")
	}
	if credential.KiroAPIKey != "" {
		if credential.AuthMethod == "" || credential.AuthMethod == "social" {
			credential.AuthMethod = "api_key"
		}
		return credential, true, nil
	}
	if credential.AuthMethod == "" {
		credential.AuthMethod = "social"
	}
	if credential.AuthMethod == "idc" && (credential.ClientID == "" || credential.ClientSecret == "") {
		return credential, false, fmt.Errorf("Kiro idc 导入必须同时提供 clientId 和 clientSecret")
	}
	if credential.AuthMethod == "api_key" {
		return credential, false, fmt.Errorf("Kiro api_key 导入必须提供 kiroApiKey")
	}
	return credential, true, nil
}

func firstNonEmptyKiroFlexibleString(values ...kiroFlexibleString) string {
	for _, value := range values {
		if text := value.text(); text != "" {
			return text
		}
	}
	return ""
}

func normalizeKiroExpiresAt(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds > 1e12 {
			seconds = seconds / 1000
		}
		wholeSeconds := int64(seconds)
		nanoseconds := int64((seconds - float64(wholeSeconds)) * 1e9)
		return time.Unix(wholeSeconds, nanoseconds).UTC().Format(time.RFC3339Nano)
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano)
	}
	return value
}

func canonicalKiroAuthMethod(method string) string {
	method = strings.TrimSpace(method)
	switch {
	case method == "":
		return ""
	case strings.EqualFold(method, "builder-id"), strings.EqualFold(method, "iam"):
		return "idc"
	case strings.EqualFold(method, "api_key"), strings.EqualFold(method, "apikey"):
		return "api_key"
	default:
		return strings.ToLower(method)
	}
}

func parseKiroRawCredentials(raw string) []kiroForwardCredential {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	credentials := make([]kiroForwardCredential, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" {
			continue
		}
		email := ""
		if left, right, ok := strings.Cut(item, "----"); ok {
			email = strings.TrimSpace(left)
			item = strings.TrimSpace(right)
		}
		if strings.HasPrefix(strings.ToLower(item), "ksk_") {
			credentials = append(credentials, kiroForwardCredential{KiroAPIKey: item, Email: email})
			continue
		}
		credentials = append(credentials, kiroForwardCredential{RefreshToken: item, Email: email})
	}
	return credentials
}

func kiroCredentialKindAndSecret(credential kiroForwardCredential) (string, string) {
	if credential.KiroAPIKey != "" {
		return "api_key", credential.KiroAPIKey
	}
	return "refresh_token", credential.RefreshToken
}

func buildKiroImportIdempotencyPayload(credentials []kiroForwardCredential, duplicateCount int, groupIDs []int64) kiroImportIdempotencyPayload {
	items := make([]kiroImportIdempotencyItem, 0, len(credentials))
	for _, credential := range credentials {
		kind, secret := kiroCredentialKindAndSecret(credential)
		emailHash := ""
		if credential.Email != "" {
			emailHash = hashKiroSecret(strings.ToLower(strings.TrimSpace(credential.Email)))
		}
		items = append(items, kiroImportIdempotencyItem{
			SecretHash: hashKiroSecret(secret),
			EmailHash:  emailHash,
			Kind:       kind,
			AuthMethod: credential.AuthMethod,
			Priority:   credential.Priority,
			Region:     credential.Region,
			AuthRegion: credential.AuthRegion,
			APIRegion:  credential.APIRegion,
			ProxyURL:   credential.ProxyURL,
			Endpoint:   credential.Endpoint,
		})
	}
	return kiroImportIdempotencyPayload{
		Provider:       "kiro",
		Credentials:    items,
		DuplicateCount: duplicateCount,
		GroupIDs:       append([]int64(nil), groupIDs...),
	}
}

func hashKiroSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

const (
	kiroRuntimeUpstreamAccountName = "kiro-gateway-internal-anthropic"
	kiroRuntimeDefaultBaseURL      = "http://kiro-gateway:8000"
)

var kiroRuntimeDefaultModelMapping = map[string]any{
	"deepseek-3.2":     "deepseek-3.2",
	"glm-5":            "glm-5",
	"minimax-m2.1":     "minimax-m2.1",
	"minimax-m2.5":     "minimax-m2.5",
	"qwen3-coder-next": "qwen3-coder-next",
}

func (h *AccountHandler) ensureKiroRuntimeUpstreamAccount(ctx context.Context, cfg kiroAdapterConfig, groupIDs []int64) (*KiroImportUpstreamAccountResult, error) {
	if h == nil || h.adminService == nil {
		return nil, nil
	}

	groupIDs = normalizeKiroGroupIDs(groupIDs)
	runtimeBaseURL := resolveKiroRuntimeBaseURL(cfg)
	runtimeAPIKey := strings.TrimSpace(cfg.InternalAPIKey)
	if runtimeBaseURL == "" || runtimeAPIKey == "" {
		return nil, nil
	}

	credentials := map[string]any{
		"api_key":       runtimeAPIKey,
		"base_url":      runtimeBaseURL,
		"model_mapping": cloneKiroMap(kiroRuntimeDefaultModelMapping),
	}
	extra := map[string]any{
		"provider_adapter":         "kiro",
		"provider_adapter_runtime": "kiro-gateway",
		"anthropic_passthrough":    true,
	}

	existing, err := h.findKiroRuntimeUpstreamAccount(ctx)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		mergedCredentials := mergeKiroRuntimeCredentials(existing.Credentials, credentials)
		mergedExtra := mergeKiroRuntimeExtra(existing.Extra, extra)
		updateInput := &service.UpdateAccountInput{
			Credentials:           mergedCredentials,
			Extra:                 mergedExtra,
			SkipMixedChannelCheck: true,
		}
		if len(groupIDs) > 0 {
			updateInput.GroupIDs = &groupIDs
		}
		updated, updateErr := h.adminService.UpdateAccount(ctx, existing.ID, updateInput)
		if updateErr != nil {
			return nil, updateErr
		}
		accountID := existing.ID
		if updated != nil {
			accountID = updated.ID
		}
		return &KiroImportUpstreamAccountResult{
			AccountID: accountID,
			Action:    "updated",
			GroupIDs:  append([]int64(nil), groupIDs...),
			BaseURL:   runtimeBaseURL,
		}, nil
	}

	account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
		Name:                  kiroRuntimeUpstreamAccountName,
		Platform:              service.PlatformAnthropic,
		Type:                  service.AccountTypeAPIKey,
		Credentials:           credentials,
		Extra:                 extra,
		Concurrency:           5,
		Priority:              0,
		GroupIDs:              groupIDs,
		SkipDefaultGroupBind:  len(groupIDs) == 0,
		SkipMixedChannelCheck: true,
	})
	if err != nil {
		return nil, err
	}
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	return &KiroImportUpstreamAccountResult{
		AccountID: accountID,
		Action:    "created",
		GroupIDs:  append([]int64(nil), groupIDs...),
		BaseURL:   runtimeBaseURL,
	}, nil
}

func (h *AccountHandler) findKiroRuntimeUpstreamAccount(ctx context.Context) (*service.Account, error) {
	accounts, _, err := h.adminService.ListAccounts(ctx, 1, 10000, service.PlatformAnthropic, service.AccountTypeAPIKey, "", "", 0, "", "created_at", "desc")
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		account := accounts[i]
		if account.Name == kiroRuntimeUpstreamAccountName || account.GetExtraString("provider_adapter") == "kiro" {
			return &account, nil
		}
	}
	return nil, nil
}

func resolveKiroRuntimeBaseURL(cfg kiroAdapterConfig) string {
	if runtimeBaseURL := strings.TrimSpace(cfg.RuntimeBaseURL); runtimeBaseURL != "" {
		return strings.TrimRight(runtimeBaseURL, "/")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.InternalBaseURL), "/")
	if baseURL == "" {
		return ""
	}
	if strings.Contains(baseURL, "kiro-rs") || strings.HasSuffix(baseURL, ":8990") {
		return kiroRuntimeDefaultBaseURL
	}
	return baseURL
}

func normalizeKiroGroupIDs(groupIDs []int64) []int64 {
	if len(groupIDs) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(groupIDs))
	result := make([]int64, 0, len(groupIDs))
	for _, id := range groupIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func mergeKiroRuntimeCredentials(existing, incoming map[string]any) map[string]any {
	out := cloneKiroMap(existing)
	for key, value := range incoming {
		if key == "model_mapping" {
			out[key] = mergeKiroRuntimeModelMapping(out[key], value)
			continue
		}
		out[key] = value
	}
	return out
}

func mergeKiroRuntimeModelMapping(existing, incoming any) map[string]any {
	out := map[string]any{}
	for key, value := range mapFromKiroAny(existing) {
		out[key] = value
	}
	for key, value := range mapFromKiroAny(incoming) {
		if _, ok := out[key]; !ok {
			out[key] = value
		}
	}
	return out
}

func mergeKiroRuntimeExtra(existing, incoming map[string]any) map[string]any {
	out := cloneKiroMap(existing)
	for key, value := range incoming {
		out[key] = value
	}
	return out
}

func cloneKiroMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func mapFromKiroAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			out[key] = val
		}
		return out
	default:
		return nil
	}
}

func loadKiroAdapterConfigFromEnv() kiroAdapterConfig {
	timeout := 30 * time.Second
	if raw := firstEnv(envKiroAdapterTimeoutSeconds, envProviderAdaptersKiroTimeoutSec); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	return kiroAdapterConfig{
		InternalBaseURL: firstEnv(envKiroAdapterBaseURL, envProviderAdaptersKiroBaseURL),
		RuntimeBaseURL:  firstEnv(envKiroAdapterRuntimeBaseURL, envProviderAdaptersKiroRuntimeURL),
		AdminAPIKey:     firstEnv(envKiroAdapterAdminAPIKey, envProviderAdaptersKiroAPIKey),
		InternalAPIKey:  firstEnv(envKiroAdapterInternalAPIKey, envKiroAdapterAdminAPIKey, envProviderAdaptersKiroAPIKey),
		Timeout:         timeout,
	}
}

func validateKiroAdapterConfig(cfg kiroAdapterConfig) error {
	if strings.TrimSpace(cfg.InternalBaseURL) == "" {
		return infraerrors.ServiceUnavailable("KIRO_ADAPTER_NOT_CONFIGURED", "kiro adapter internal base URL is not configured")
	}
	if strings.TrimSpace(cfg.AdminAPIKey) == "" {
		return infraerrors.ServiceUnavailable("KIRO_ADAPTER_NOT_CONFIGURED", "kiro adapter admin API key is not configured")
	}
	if cfg.Timeout <= 0 {
		return infraerrors.BadRequest("KIRO_ADAPTER_INVALID_TIMEOUT", "kiro adapter timeout must be greater than 0")
	}
	return nil
}

func readLimitedKiroAdapterResponse(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, kiroAdapterResponseMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIRO_IMPORT_UPSTREAM_READ_FAILED", "failed to read kiro adapter response: %s", logredact.RedactText(err.Error(), kiroSensitiveKeys...))
	}
	if len(data) > kiroAdapterResponseMaxBytes {
		return nil, infraerrors.New(http.StatusBadGateway, "KIRO_IMPORT_UPSTREAM_RESPONSE_TOO_LARGE", "kiro adapter response is too large")
	}
	return data, nil
}

func sanitizeKiroAdapterResponse(raw []byte) any {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{
			"body": logredact.RedactText(string(raw), kiroSensitiveKeys...),
		}
	}
	return sanitizeKiroAdapterValue(value)
}

func sanitizeKiroAdapterValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			if isKiroSensitiveKey(key) {
				out[key] = "***"
				continue
			}
			out[key] = sanitizeKiroAdapterValue(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = sanitizeKiroAdapterValue(item)
		}
		return out
	default:
		return value
	}
}

func isKiroSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "token", "authorization", "x_api_key", "api_key", "apikey", "kiro_api_key", "client_secret", "proxy_password":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "api_key")
}
