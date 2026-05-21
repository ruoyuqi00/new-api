package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/gin-gonic/gin"
)

const providerAdapterAdminResponseMaxBytes = 1 << 20

type ProviderAdapterAdminResponse struct {
	Provider       string `json:"provider"`
	Endpoint       string `json:"endpoint"`
	Path           string `json:"path"`
	OK             bool   `json:"ok"`
	UpstreamStatus int    `json:"upstream_status"`
	FetchedAt      string `json:"fetched_at"`
	Data           any    `json:"data,omitempty"`
}

type KiroRuntimeSummary struct {
	AccountsTotal       int `json:"accounts_total"`
	AccountsAvailable   int `json:"accounts_available"`
	AccountsCooldown    int `json:"accounts_cooldown"`
	AccountsQuotaDone   int `json:"accounts_quota_exhausted"`
	AccountsProfileARN  int `json:"accounts_with_profile_arn"`
	ModelsDiscovered    int `json:"models_discovered"`
	ModelsSmokePassed   int `json:"models_smoke_passed"`
	ModelsPublicEnabled int `json:"models_public_enabled"`
}

type KiroRuntimeStatusResponse struct {
	Provider        string             `json:"provider"`
	Runtime         string             `json:"runtime"`
	Engine          string             `json:"engine"`
	Status          string             `json:"status"`
	Configured      bool               `json:"configured"`
	PublicEntryOnly bool               `json:"public_entry_only"`
	FetchedAt       string             `json:"fetched_at"`
	Summary         KiroRuntimeSummary `json:"summary"`
	Credentials     *adapterProbeState `json:"credentials,omitempty"`
	Models          *adapterProbeState `json:"models,omitempty"`
	Routing         KiroRuntimeRouting `json:"routing"`
}

type adapterProbeState struct {
	OK             bool   `json:"ok"`
	UpstreamStatus int    `json:"upstream_status"`
	Path           string `json:"path"`
	Error          string `json:"error,omitempty"`
}

type KiroRuntimeAccountsResponse struct {
	Provider  string               `json:"provider"`
	Runtime   string               `json:"runtime"`
	FetchedAt string               `json:"fetched_at"`
	Total     int                  `json:"total"`
	Accounts  []KiroRuntimeAccount `json:"accounts"`
	Raw       any                  `json:"raw,omitempty"`
}

type KiroRuntimeAccount struct {
	ID                  string   `json:"id"`
	Label               string   `json:"label"`
	Email               string   `json:"email,omitempty"`
	AuthMethod          string   `json:"auth_method,omitempty"`
	Engine              string   `json:"engine"`
	Region              string   `json:"region,omitempty"`
	PlanName            string   `json:"plan_name,omitempty"`
	PlanTier            string   `json:"plan_tier,omitempty"`
	ProfileARNPresent   bool     `json:"profile_arn_present"`
	TokenStatus         string   `json:"token_status"`
	RuntimeStatus       string   `json:"runtime_status"`
	UsageCurrent        *float64 `json:"usage_current,omitempty"`
	UsageLimit          *float64 `json:"usage_limit,omitempty"`
	UsageResetAt        *int64   `json:"usage_reset_at,omitempty"`
	ErrorCount          *int     `json:"error_count,omitempty"`
	CooldownUntil       *int64   `json:"cooldown_until,omitempty"`
	LastUsedAt          *int64   `json:"last_used_at,omitempty"`
	SupportedModelCount int      `json:"supported_model_count"`
}

type KiroRuntimeModelsResponse struct {
	Provider  string             `json:"provider"`
	Runtime   string             `json:"runtime"`
	FetchedAt string             `json:"fetched_at"`
	Total     int                `json:"total"`
	Models    []KiroRuntimeModel `json:"models"`
	Raw       any                `json:"raw,omitempty"`
}

type KiroRuntimeModel struct {
	ID                    string `json:"id"`
	DisplayName           string `json:"display_name,omitempty"`
	Source                string `json:"source"`
	SupportedAccountCount int    `json:"supported_account_count"`
	LastSmokeStatus       string `json:"last_smoke_status"`
	PublicEnabled         bool   `json:"public_enabled"`
	DisabledReason        string `json:"disabled_reason,omitempty"`
}

type KiroRuntimeRouting struct {
	DefaultStrategy   string   `json:"default_strategy"`
	SessionSticky     bool     `json:"session_sticky"`
	ModelAwareRouting bool     `json:"model_aware_routing"`
	AutoSwitchOnQuota bool     `json:"auto_switch_on_quota"`
	AllowOverage      bool     `json:"allow_overage"`
	Capabilities      []string `json:"capabilities"`
}

type providerAdapterAdminFetchConfig struct {
	Provider      string
	Endpoint      string
	BaseURL       string
	APIKey        string
	Path          string
	Timeout       time.Duration
	Sanitize      func([]byte) any
	SensitiveKeys []string
}

func (h *AccountHandler) GetWindsurfAdapterHealth(c *gin.Context) {
	cfg := loadWindsurfAdapterConfigFromEnv()
	if err := validateWindsurfAdapterConfig(cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := fetchProviderAdapterAdminJSON(c.Request.Context(), providerAdapterAdminFetchConfig{
		Provider:      "windsurf",
		Endpoint:      "health",
		BaseURL:       cfg.InternalBaseURL,
		APIKey:        cfg.InternalAPIKey,
		Path:          "/health",
		Timeout:       cfg.Timeout,
		Sanitize:      sanitizeWindsurfAdapterResponse,
		SensitiveKeys: windsurfSensitiveKeys,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) GetWindsurfAdapterAccounts(c *gin.Context) {
	cfg := loadWindsurfAdapterConfigFromEnv()
	if err := validateWindsurfAdapterConfig(cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := fetchProviderAdapterAdminJSON(c.Request.Context(), providerAdapterAdminFetchConfig{
		Provider:      "windsurf",
		Endpoint:      "accounts",
		BaseURL:       cfg.InternalBaseURL,
		APIKey:        cfg.InternalAPIKey,
		Path:          "/auth/accounts",
		Timeout:       cfg.Timeout,
		Sanitize:      sanitizeWindsurfAdapterResponse,
		SensitiveKeys: windsurfSensitiveKeys,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) GetKiroAdapterCredentials(c *gin.Context) {
	result, err := fetchKiroAdapterAdmin(c.Request.Context(), "credentials", "/api/admin/credentials")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) GetKiroRuntimeStatus(c *gin.Context) {
	cfg := loadKiroAdapterConfigFromEnv()
	routing := defaultKiroRuntimeRouting()
	status := KiroRuntimeStatusResponse{
		Provider:        "kiro",
		Runtime:         "kiro-runtime",
		Engine:          "legacy-kiro-rs",
		Status:          "not_configured",
		Configured:      false,
		PublicEntryOnly: true,
		FetchedAt:       time.Now().UTC().Format(time.RFC3339),
		Routing:         routing,
	}
	if err := validateKiroAdapterConfig(cfg); err != nil {
		status.Credentials = &adapterProbeState{OK: false, Path: "/api/admin/credentials", Error: "kiro adapter is not configured"}
		status.Models = &adapterProbeState{OK: false, Path: "/v1/models", Error: "kiro adapter is not configured"}
		response.Success(c, status)
		return
	}
	status.Configured = true

	credentials, credErr := fetchKiroAdapterAdminWithConfig(c.Request.Context(), cfg, "credentials", "/api/admin/credentials")
	if credErr != nil {
		status.Credentials = &adapterProbeState{OK: false, Path: "/api/admin/credentials", Error: logredact.RedactText(credErr.Error(), kiroSensitiveKeys...)}
	} else {
		status.Credentials = adapterProbeFromResponse(credentials)
		accounts := normalizeKiroRuntimeAccounts(credentials.Data)
		status.Summary = summarizeKiroRuntimeAccounts(accounts, credentials.Data)
	}

	models, modelErr := fetchKiroAdapterAdminWithConfig(c.Request.Context(), cfg, "models", "/v1/models")
	if modelErr != nil {
		status.Models = &adapterProbeState{OK: false, Path: "/v1/models", Error: logredact.RedactText(modelErr.Error(), kiroSensitiveKeys...)}
	} else {
		status.Models = adapterProbeFromResponse(models)
		normalizedModels := normalizeKiroRuntimeModels(models.Data)
		status.Summary.ModelsDiscovered = len(normalizedModels)
	}

	switch {
	case status.Credentials != nil && status.Credentials.OK && status.Models != nil && status.Models.OK:
		status.Status = "online"
	case status.Credentials != nil && status.Credentials.OK:
		status.Status = "degraded"
	default:
		status.Status = "offline"
	}
	response.Success(c, status)
}

func (h *AccountHandler) GetKiroRuntimeAccounts(c *gin.Context) {
	result, err := fetchKiroAdapterAdmin(c.Request.Context(), "credentials", "/api/admin/credentials")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	accounts := normalizeKiroRuntimeAccounts(result.Data)
	response.Success(c, KiroRuntimeAccountsResponse{
		Provider:  "kiro",
		Runtime:   "kiro-runtime",
		FetchedAt: result.FetchedAt,
		Total:     len(accounts),
		Accounts:  accounts,
		Raw:       result.Data,
	})
}

func (h *AccountHandler) GetKiroRuntimeModels(c *gin.Context) {
	result, err := fetchKiroAdapterAdmin(c.Request.Context(), "models", "/v1/models")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	models := normalizeKiroRuntimeModels(result.Data)
	response.Success(c, KiroRuntimeModelsResponse{
		Provider:  "kiro",
		Runtime:   "kiro-runtime",
		FetchedAt: result.FetchedAt,
		Total:     len(models),
		Models:    models,
		Raw:       result.Data,
	})
}

func (h *AccountHandler) GetKiroRuntimeRouting(c *gin.Context) {
	response.Success(c, defaultKiroRuntimeRouting())
}

func fetchProviderAdapterAdminJSON(ctx context.Context, cfg providerAdapterAdminFetchConfig) (ProviderAdapterAdminResponse, error) {
	result := ProviderAdapterAdminResponse{
		Provider:  cfg.Provider,
		Endpoint:  cfg.Endpoint,
		Path:      cfg.Path,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + cfg.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result, infraerrors.InternalServer("PROVIDER_ADAPTER_ADMIN_REQUEST_FAILED", "failed to build provider adapter admin request").WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("x-api-key", cfg.APIKey)

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return result, infraerrors.Newf(http.StatusBadGateway, "PROVIDER_ADAPTER_ADMIN_UPSTREAM_REQUEST_FAILED", "%s adapter admin request failed: %s", cfg.Provider, logredact.RedactText(err.Error(), cfg.SensitiveKeys...))
	}
	defer resp.Body.Close()

	body, err := readLimitedProviderAdapterAdminResponse(resp.Body, cfg.Provider, cfg.SensitiveKeys)
	if err != nil {
		return result, err
	}
	result.UpstreamStatus = resp.StatusCode
	result.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
	if cfg.Sanitize != nil {
		result.Data = cfg.Sanitize(body)
	} else {
		result.Data = map[string]any{"body": logredact.RedactText(string(body), cfg.SensitiveKeys...)}
	}
	return result, nil
}

func fetchKiroAdapterAdmin(ctx context.Context, endpoint, path string) (ProviderAdapterAdminResponse, error) {
	cfg := loadKiroAdapterConfigFromEnv()
	if err := validateKiroAdapterConfig(cfg); err != nil {
		return ProviderAdapterAdminResponse{}, err
	}
	return fetchKiroAdapterAdminWithConfig(ctx, cfg, endpoint, path)
}

func fetchKiroAdapterAdminWithConfig(ctx context.Context, cfg kiroAdapterConfig, endpoint, path string) (ProviderAdapterAdminResponse, error) {
	apiKey := cfg.AdminAPIKey
	if isKiroRuntimeAPIPath(path) {
		apiKey = firstNonEmptyAdapterString(cfg.InternalAPIKey, cfg.AdminAPIKey)
	}
	return fetchProviderAdapterAdminJSON(ctx, providerAdapterAdminFetchConfig{
		Provider:      "kiro",
		Endpoint:      endpoint,
		BaseURL:       cfg.InternalBaseURL,
		APIKey:        apiKey,
		Path:          path,
		Timeout:       cfg.Timeout,
		Sanitize:      sanitizeKiroAdapterResponse,
		SensitiveKeys: kiroSensitiveKeys,
	})
}

func isKiroRuntimeAPIPath(path string) bool {
	return strings.HasPrefix(path, "/v1/")
}

func adapterProbeFromResponse(result ProviderAdapterAdminResponse) *adapterProbeState {
	return &adapterProbeState{
		OK:             result.OK,
		UpstreamStatus: result.UpstreamStatus,
		Path:           result.Path,
	}
}

func defaultKiroRuntimeRouting() KiroRuntimeRouting {
	return KiroRuntimeRouting{
		DefaultStrategy:   "round-robin",
		SessionSticky:     true,
		ModelAwareRouting: true,
		AutoSwitchOnQuota: true,
		AllowOverage:      false,
		Capabilities: []string{
			"runtime_status",
			"account_pool_summary",
			"model_discovery_proxy",
			"redacted_adapter_admin",
			"planned_refresh_single_flight",
			"planned_smoke_sync",
		},
	}
}

func normalizeKiroRuntimeAccounts(value any) []KiroRuntimeAccount {
	items := firstAdapterSlice(value, "credentials", "accounts", "items", "data")
	accounts := make([]KiroRuntimeAccount, 0, len(items))
	for i, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		email := firstAdapterString(record, "email", "login_hint", "loginHint")
		id := firstAdapterString(record, "id", "account_id", "accountId", "user_id", "userId")
		if id == "" {
			id = fmt.Sprintf("kiro_account_%d", i+1)
		}
		label := email
		if label == "" {
			label = id
		}
		account := KiroRuntimeAccount{
			ID:                  id,
			Label:               label,
			Email:               email,
			AuthMethod:          firstAdapterString(record, "auth_method", "authMethod", "provider"),
			Engine:              firstNonEmptyAdapterString(firstAdapterString(record, "engine", "endpoint"), "legacy-kiro-rs"),
			Region:              firstAdapterString(record, "region", "auth_region", "authRegion", "api_region", "apiRegion"),
			PlanName:            firstAdapterString(record, "plan_name", "planName", "subscription_title", "subscriptionTitle"),
			PlanTier:            firstAdapterString(record, "plan_tier", "planTier", "subscription_type", "subscriptionType"),
			ProfileARNPresent:   hasProfileARN(record),
			TokenStatus:         tokenStatusFromRecord(record),
			RuntimeStatus:       runtimeStatusFromRecord(record),
			UsageCurrent:        firstAdapterFloatPtr(record, "credits_used", "usage_current", "usageCurrent", "currentUsage", "currentUsageWithPrecision"),
			UsageLimit:          firstAdapterFloatPtr(record, "credits_total", "usage_limit", "usageLimit", "usageLimitWithPrecision"),
			UsageResetAt:        firstAdapterInt64Ptr(record, "usage_reset_at", "usageResetAt", "nextDateReset", "reset_at", "resetAt"),
			ErrorCount:          firstAdapterIntPtr(record, "error_count", "errorCount", "errors"),
			CooldownUntil:       firstAdapterInt64Ptr(record, "cooldown_until", "cooldownUntil"),
			LastUsedAt:          firstAdapterInt64Ptr(record, "last_used", "lastUsed", "last_used_at", "lastUsedAt"),
			SupportedModelCount: len(firstAdapterStringSlice(record, "availableModels", "available_models", "models", "model_set", "modelSet")),
		}
		accounts = append(accounts, account)
	}
	return accounts
}

func normalizeKiroRuntimeModels(value any) []KiroRuntimeModel {
	items := firstAdapterSlice(value, "models", "data", "availableModels", "available_models", "items")
	models := make([]KiroRuntimeModel, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		var model KiroRuntimeModel
		switch v := item.(type) {
		case string:
			model.ID = strings.TrimSpace(v)
		case map[string]any:
			model.ID = firstAdapterString(v, "id", "model", "model_id", "modelId", "modelID")
			model.DisplayName = firstAdapterString(v, "display_name", "displayName", "name", "modelName", "model_name")
			model.Source = firstNonEmptyAdapterString(firstAdapterString(v, "source"), "official_discovery")
			model.SupportedAccountCount = firstAdapterInt(v, "supported_account_count", "supportedAccountCount")
			model.LastSmokeStatus = firstNonEmptyAdapterString(firstAdapterString(v, "last_smoke_status", "lastSmokeStatus"), "not_run")
			model.PublicEnabled = firstAdapterBool(v, "public_enabled", "publicEnabled")
			model.DisabledReason = firstAdapterString(v, "disabled_reason", "disabledReason")
		}
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || seen[model.ID] {
			continue
		}
		if model.Source == "" {
			model.Source = "official_discovery"
		}
		if model.LastSmokeStatus == "" {
			model.LastSmokeStatus = "not_run"
		}
		seen[model.ID] = true
		models = append(models, model)
	}
	return models
}

func summarizeKiroRuntimeAccounts(accounts []KiroRuntimeAccount, raw any) KiroRuntimeSummary {
	summary := KiroRuntimeSummary{AccountsTotal: len(accounts)}
	if total := firstAdapterInt(adapterMap(raw), "total"); total > summary.AccountsTotal {
		summary.AccountsTotal = total
	}
	if available := firstAdapterInt(adapterMap(raw), "available"); available > 0 {
		summary.AccountsAvailable = available
	}
	if withProfile := firstAdapterInt(adapterMap(raw), "with_profile_arn", "withProfileArn", "with_profileArn"); withProfile > 0 {
		summary.AccountsProfileARN = withProfile
	}
	for _, account := range accounts {
		if account.RuntimeStatus == "available" || account.RuntimeStatus == "normal" {
			summary.AccountsAvailable++
		}
		if account.RuntimeStatus == "cooldown" {
			summary.AccountsCooldown++
		}
		if account.RuntimeStatus == "quota_exhausted" {
			summary.AccountsQuotaDone++
		}
		if account.ProfileARNPresent {
			summary.AccountsProfileARN++
		}
	}
	if summary.AccountsAvailable > summary.AccountsTotal {
		summary.AccountsAvailable = summary.AccountsTotal
	}
	if summary.AccountsProfileARN > summary.AccountsTotal {
		summary.AccountsProfileARN = summary.AccountsTotal
	}
	return summary
}

func firstAdapterSlice(value any, keys ...string) []any {
	if slice, ok := value.([]any); ok {
		return slice
	}
	record := adapterMap(value)
	for _, key := range keys {
		if slice, ok := record[key].([]any); ok {
			return slice
		}
	}
	return nil
}

func adapterMap(value any) map[string]any {
	if record, ok := value.(map[string]any); ok {
		return record
	}
	return map[string]any{}
}

func firstAdapterString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key]; ok {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return strings.TrimSpace(v.String())
				}
			}
		}
	}
	return ""
}

func firstNonEmptyAdapterString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstAdapterStringSlice(record map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case []string:
			return v
		case []any:
			out := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
			return out
		}
	}
	return nil
}

func firstAdapterFloatPtr(record map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := parseAdapterFloat(record[key]); ok {
			return &value
		}
	}
	return nil
}

func parseAdapterFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func firstAdapterIntPtr(record map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := parseAdapterInt64(record[key]); ok {
			asInt := int(value)
			return &asInt
		}
	}
	return nil
}

func firstAdapterInt(record map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := parseAdapterInt64(record[key]); ok {
			return int(value)
		}
	}
	return 0
}

func firstAdapterInt64Ptr(record map[string]any, keys ...string) *int64 {
	for _, key := range keys {
		if value, ok := parseAdapterInt64(record[key]); ok {
			return &value
		}
	}
	return nil
}

func parseAdapterInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func firstAdapterBool(record map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch v := record[key].(type) {
		case bool:
			return v
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err == nil {
				return parsed
			}
		}
	}
	return false
}

func hasProfileARN(record map[string]any) bool {
	if firstAdapterString(record, "profile_arn", "profileArn") != "" {
		return true
	}
	return firstAdapterBool(record, "has_profile_arn", "hasProfileArn", "profile_arn_present", "profileArnPresent")
}

func tokenStatusFromRecord(record map[string]any) string {
	if status := firstAdapterString(record, "token_status", "tokenStatus"); status != "" {
		return status
	}
	expiresAt := firstAdapterInt64Ptr(record, "expires_at", "expiresAt")
	if expiresAt == nil || *expiresAt <= 0 {
		return "unknown"
	}
	now := time.Now().Unix()
	switch {
	case *expiresAt <= now:
		return "expired"
	case *expiresAt <= now+300:
		return "expiring"
	default:
		return "valid"
	}
}

func runtimeStatusFromRecord(record map[string]any) string {
	if status := firstAdapterString(record, "runtime_status", "runtimeStatus", "status"); status != "" {
		normalized := strings.ToLower(strings.ReplaceAll(status, "-", "_"))
		switch normalized {
		case "normal", "ok", "active":
			return "available"
		default:
			return normalized
		}
	}
	if firstAdapterBool(record, "disabled", "is_disabled", "isDisabled") {
		return "disabled"
	}
	if firstAdapterBool(record, "available", "is_available", "isAvailable") {
		return "available"
	}
	return "unknown"
}

func readLimitedProviderAdapterAdminResponse(body io.Reader, provider string, sensitiveKeys []string) ([]byte, error) {
	limited := io.LimitReader(body, providerAdapterAdminResponseMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "PROVIDER_ADAPTER_ADMIN_UPSTREAM_READ_FAILED", "failed to read %s adapter admin response: %s", provider, logredact.RedactText(err.Error(), sensitiveKeys...))
	}
	if len(data) > providerAdapterAdminResponseMaxBytes {
		return nil, infraerrors.New(http.StatusBadGateway, "PROVIDER_ADAPTER_ADMIN_UPSTREAM_RESPONSE_TOO_LARGE", fmt.Sprintf("%s adapter admin response is too large", provider))
	}
	return data, nil
}
