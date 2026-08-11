package model

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const (
	YucoreMediaTaskStatusPending    = "pending"
	YucoreMediaTaskStatusProcessing = "processing"
	YucoreMediaTaskStatusCompleted  = "completed"
	YucoreMediaTaskStatusFailed     = "failed"
	YucoreMediaTaskStatusCanceled   = "canceled"

	YucoreMediaAdapterMock             = "mock"
	YucoreMediaAdapterOpenAICompatible = "openai-compatible"
	YucoreMediaAdapterYuAPIChannel     = "yuapi-channel"
	YucoreMediaAdapterUAGProxy         = "uag-proxy"
)

type YucoreMediaAdapterInfo struct {
	Adapter                     string   `json:"adapter"`
	Configured                  bool     `json:"configured"`
	BaseURLConfigured           bool     `json:"base_url_configured"`
	APIKeyConfigured            bool     `json:"api_key_configured"`
	AuthMode                    string   `json:"auth_mode,omitempty"`
	Status                      string   `json:"status"`
	Message                     string   `json:"message,omitempty"`
	SupportsImage               bool     `json:"supports_image"`
	SupportsVideo               bool     `json:"supports_video"`
	RequireRealAssets           bool     `json:"require_real_assets"`
	MockFallback                bool     `json:"mock_fallback"`
	UpstreamVerified            bool     `json:"upstream_verified"`
	UpstreamVerificationStatus  string   `json:"upstream_verification_status"`
	UpstreamVerificationMessage string   `json:"upstream_verification_message,omitempty"`
	RealWorkflowReady           bool     `json:"real_workflow_ready"`
	VerificationBlockers        []string `json:"verification_blockers,omitempty"`
}

type yucoreMediaAdapterConfig struct {
	Adapter             string
	BaseURL             string
	APIKey              string
	TimeoutSeconds      int
	RequireRealAssets   bool
	ModelCapabilities   map[string]YucoreMediaModelCapability
	ManagedTokenGroup   string
	UAGModelMap         map[string]string
	UAGAllowedProviders map[string]struct{}
	UAGAllowedModels    map[string]struct{}
	UpstreamVerified    bool
}

type YucoreMediaUAGProxyHeaders map[string]string

type YucoreMediaAssets string

func (YucoreMediaAssets) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db != nil && db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}

type YucoreMediaAsset struct {
	Id         string         `json:"id"`
	Kind       string         `json:"kind"`
	Url        string         `json:"url"`
	ThumbUrl   string         `json:"thumb_url,omitempty"`
	CachedUrl  string         `json:"cached_url,omitempty"`
	SourceUrl  string         `json:"source_url,omitempty"`
	Label      string         `json:"label"`
	Width      int            `json:"width,omitempty"`
	Height     int            `json:"height,omitempty"`
	DurationMs int            `json:"duration_ms,omitempty"`
	MimeType   string         `json:"mime_type,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type openAICompatibleImageResponse struct {
	Data []struct {
		URL           string `json:"url"`
		B64JSON       string `json:"b64_json"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
	Images []struct {
		URL           string `json:"url"`
		B64JSON       string `json:"b64_json"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"images"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type YucoreMediaTask struct {
	Id             int               `json:"id" gorm:"primary_key"`
	TaskId         string            `json:"task_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId         int               `json:"user_id" gorm:"index"`
	SessionId      string            `json:"session_id" gorm:"type:varchar(96);index;default:''"`
	BillingGroup   string            `json:"group" gorm:"column:billing_group;type:varchar(64);index;default:''"`
	Kind           string            `json:"kind" gorm:"type:varchar(24);index;default:'image'"`
	Mode           string            `json:"mode" gorm:"type:varchar(48);default:'text-to-image'"`
	ModelId        string            `json:"model_id" gorm:"type:varchar(96);index;default:'gpt-image-2'"`
	Prompt         string            `json:"prompt" gorm:"type:text"`
	NegativePrompt string            `json:"negative_prompt" gorm:"type:text"`
	AspectRatio    string            `json:"aspect_ratio" gorm:"type:varchar(24);default:'auto'"`
	Size           string            `json:"size" gorm:"type:varchar(24);default:'1k'"`
	Quality        string            `json:"quality" gorm:"type:varchar(24);default:'high'"`
	Format         string            `json:"format" gorm:"type:varchar(24);default:'png'"`
	Count          int               `json:"count" gorm:"default:1"`
	Status         string            `json:"status" gorm:"type:varchar(24);index;default:'processing'"`
	Progress       int               `json:"progress" gorm:"default:0"`
	Cost           int               `json:"cost" gorm:"default:0"`
	Assets         YucoreMediaAssets `json:"assets"`
	Inputs         string            `json:"inputs" gorm:"type:text"`
	Metadata       string            `json:"metadata" gorm:"type:text"`
	Error          string            `json:"error" gorm:"type:varchar(512);default:''"`
	CreatedTime    int64             `json:"created_time" gorm:"bigint;index"`
	UpdatedTime    int64             `json:"updated_time" gorm:"bigint;index"`
	DeletedAt      gorm.DeletedAt    `json:"-" gorm:"index"`
}

func GenerateYucoreMediaTaskID() string {
	key, err := common.GenerateRandomCharsKey(18)
	if err != nil || key == "" {
		return fmt.Sprintf("yu_%d", common.GetTimestamp())
	}
	return "yu_" + key
}

func normalizeYucoreMediaAdapterName(adapter string) string {
	adapter = strings.ToLower(strings.TrimSpace(adapter))
	switch adapter {
	case "", "local":
		return YucoreMediaAdapterMock
	case "openai", "openai_compatible", "openai-compatible":
		return YucoreMediaAdapterOpenAICompatible
	case "yuapi", "yuapi_channel", "yuapi-channel":
		return YucoreMediaAdapterYuAPIChannel
	case "uag", "uag_proxy", "uag-proxy":
		return YucoreMediaAdapterUAGProxy
	default:
		return adapter
	}
}

func getYucoreMediaOptionString(key string, envKey string, fallback string) string {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	if ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(common.GetEnvOrDefaultString(envKey, fallback))
}

func getYucoreMediaOptionInt(key string, envKey string, fallback int) int {
	raw := getYucoreMediaOptionString(key, envKey, "")
	if raw == "" {
		return common.GetEnvOrDefault(envKey, fallback)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getYucoreMediaOptionBool(key string, envKey string, fallback bool) bool {
	raw := strings.ToLower(getYucoreMediaOptionString(key, envKey, ""))
	switch raw {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return common.GetEnvOrDefaultBool(envKey, fallback)
	}
}

func getYucoreMediaAdapterConfig() yucoreMediaAdapterConfig {
	config, _ := getYucoreMediaAdapterConfigChecked()
	return config
}

func getYucoreMediaAdapterConfigChecked() (yucoreMediaAdapterConfig, error) {
	timeout := getYucoreMediaOptionInt("yucore_media.timeout_seconds", "YUCORE_MEDIA_TIMEOUT_SECONDS", 90)
	if timeout <= 0 {
		timeout = 90
	}
	adapter := normalizeYucoreMediaAdapterName(getYucoreMediaOptionString("yucore_media.adapter", "YUCORE_MEDIA_ADAPTER", YucoreMediaAdapterMock))
	baseURL := strings.TrimRight(getYucoreMediaOptionString("yucore_media.base_url", "YUCORE_MEDIA_BASE_URL", ""), "/")
	if adapter == YucoreMediaAdapterUAGProxy && baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(common.GetEnvOrDefaultString("YUCORE_MEDIA_UAG_BASE_URL", "")), "/")
	}
	embeddedCapabilities, configErr := loadCangyuanMediaCatalog()
	modelCapabilities := embeddedCapabilities
	if configErr == nil {
		modelCapabilities, configErr = mergeYucoreMediaModelCapabilities(
			embeddedCapabilities,
			common.GetEnvOrDefaultString("YUCORE_MEDIA_MODEL_CAPABILITIES", ""),
		)
	}
	common.OptionMapRWMutex.RLock()
	operatorCapabilities, hasOperatorCapabilities := common.OptionMap["yucore_media.model_capabilities"]
	common.OptionMapRWMutex.RUnlock()
	if hasOperatorCapabilities && configErr == nil {
		modelCapabilities, configErr = mergeYucoreMediaModelCapabilities(modelCapabilities, operatorCapabilities)
	}
	config := yucoreMediaAdapterConfig{
		Adapter:             adapter,
		BaseURL:             baseURL,
		APIKey:              getYucoreMediaOptionString("yucore_media.api_key", "YUCORE_MEDIA_API_KEY", ""),
		TimeoutSeconds:      timeout,
		RequireRealAssets:   getYucoreMediaOptionBool("yucore_media.require_real_assets", "YUCORE_MEDIA_REQUIRE_REAL_ASSETS", false),
		ModelCapabilities:   modelCapabilities,
		ManagedTokenGroup:   getYucoreMediaOptionString("yucore_media.managed_token_group", "YUCORE_MEDIA_MANAGED_TOKEN_GROUP", ""),
		UAGModelMap:         parseYucoreMediaUAGModelMap(getYucoreMediaOptionString("yucore_media.uag_model_map", "YUCORE_MEDIA_UAG_MODEL_MAP", "")),
		UAGAllowedProviders: parseYucoreMediaUAGAllowlist(getYucoreMediaOptionString("yucore_media.uag_allowed_providers", "YUCORE_MEDIA_UAG_ALLOWED_PROVIDERS", "")),
		UAGAllowedModels:    parseYucoreMediaUAGAllowlist(getYucoreMediaOptionString("yucore_media.uag_allowed_models", "YUCORE_MEDIA_UAG_ALLOWED_MODELS", "")),
		UpstreamVerified:    getYucoreMediaOptionBool("yucore_media.upstream_verified", "YUCORE_MEDIA_UPSTREAM_VERIFIED", false),
	}
	return config, configErr
}

func GetYucoreMediaCatalogSettings() (string, map[string]YucoreMediaModelCapability) {
	config := getYucoreMediaAdapterConfig()
	return config.ManagedTokenGroup, cloneYucoreMediaModelCapabilities(config.ModelCapabilities)
}

func YucoreMediaConfiguredModelIDs() map[string]struct{} {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterOpenAICompatible && config.Adapter != YucoreMediaAdapterYuAPIChannel {
		return nil
	}
	capabilities := config.ModelCapabilities
	if len(capabilities) == 0 {
		return nil
	}
	configured := make(map[string]struct{}, len(capabilities))
	for modelId, capability := range capabilities {
		if strings.EqualFold(strings.TrimSpace(capability.Availability), YucoreMediaAvailabilityProbe) {
			continue
		}
		configured[strings.ToLower(strings.TrimSpace(modelId))] = struct{}{}
	}
	return configured
}

func parseYucoreMediaUAGAllowlist(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := make([]string, 0)
	if strings.HasPrefix(raw, "[") {
		_ = common.Unmarshal([]byte(raw), &values)
	} else {
		values = strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
		})
	}
	allowed := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func parseYucoreMediaUAGModelMap(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := map[string]string{}
	if strings.HasPrefix(raw, "{") {
		_ = common.Unmarshal([]byte(raw), &values)
	} else {
		for _, pair := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n'
		}) {
			key, value, ok := strings.Cut(pair, "=")
			if !ok {
				key, value, ok = strings.Cut(pair, ":")
			}
			if ok {
				values[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
	}
	for key, value := range values {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			delete(values, key)
			continue
		}
		normalizedKey := strings.TrimSpace(key)
		normalizedValue := strings.TrimSpace(value)
		if normalizedKey != key {
			delete(values, key)
		}
		values[normalizedKey] = normalizedValue
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func yucoreMediaUAGAllowed(allowlist map[string]struct{}, value string) bool {
	if len(allowlist) == 0 {
		return true
	}
	_, ok := allowlist[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func yucoreMediaUpstreamVerification(config yucoreMediaAdapterConfig, configured bool) (bool, string, string) {
	if config.Adapter == YucoreMediaAdapterMock {
		return false, "not_applicable", "YuCore media is using local mock assets."
	}
	if !configured {
		return false, "unavailable", "YuCore media adapter is not configured."
	}
	if config.UpstreamVerified {
		return true, "verified", "YuCore media upstream provider has been explicitly verified."
	}
	return false, "unverified", "YuCore media adapter is configured, but a real upstream provider has not been verified end to end."
}

func yucoreMediaVerificationBlockers(config yucoreMediaAdapterConfig, configured bool, upstreamVerified bool) []string {
	blockers := make([]string, 0)
	if config.Adapter == YucoreMediaAdapterMock {
		blockers = append(blockers, "YuCore media adapter is still using mock assets.")
	}
	if !configured {
		blockers = append(blockers, "YuCore media adapter is not configured for real media generation.")
	}
	if !config.RequireRealAssets {
		blockers = append(blockers, "yucore_media.require_real_assets must stay enabled for real workflow verification.")
	}
	if !upstreamVerified {
		blockers = append(blockers, "An ordinary-user Studio generation has not been verified against a real upstream provider.")
	}
	return blockers
}

func yucoreMediaUAGModelID(config yucoreMediaAdapterConfig, modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return modelID
	}
	if mapped := strings.TrimSpace(config.UAGModelMap[modelID]); mapped != "" {
		return mapped
	}
	for key, value := range config.UAGModelMap {
		if strings.EqualFold(strings.TrimSpace(key), modelID) && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return modelID
}

func GetYucoreMediaAdapterInfo() YucoreMediaAdapterInfo {
	config := getYucoreMediaAdapterConfig()
	configured := (config.Adapter == YucoreMediaAdapterMock && !config.RequireRealAssets) ||
		(config.Adapter == YucoreMediaAdapterOpenAICompatible && config.BaseURL != "" && config.APIKey != "") ||
		(config.Adapter == YucoreMediaAdapterYuAPIChannel && config.BaseURL != "" && config.ManagedTokenGroup != "") ||
		(config.Adapter == YucoreMediaAdapterUAGProxy && config.BaseURL != "")
	status := "unavailable"
	message := ""
	authMode := ""
	switch config.Adapter {
	case YucoreMediaAdapterMock:
		if configured {
			status = "development"
			message = "YuCore media is using local mock assets."
		} else {
			message = "YUCORE_MEDIA_REQUIRE_REAL_ASSETS=true forbids mock YuCore media assets."
		}
	case YucoreMediaAdapterOpenAICompatible:
		authMode = "bearer"
		if configured {
			status = "configured"
		} else {
			message = "YUCORE_MEDIA_BASE_URL and YUCORE_MEDIA_API_KEY are required for openai-compatible media."
		}
	case YucoreMediaAdapterYuAPIChannel:
		authMode = "managed per-user YuAPI token"
		if configured {
			status = "configured"
		} else {
			message = "YUCORE_MEDIA_BASE_URL and YUCORE_MEDIA_MANAGED_TOKEN_GROUP are required for yuapi-channel media."
		}
	case YucoreMediaAdapterUAGProxy:
		authMode = "server API key or browser Authorization passthrough"
		if configured {
			status = "configured"
			if config.APIKey == "" {
				message = "UAG proxy base URL is configured. Server API key is not set, so requests require browser-side UAG Authorization passthrough or public upstream access."
			}
		} else {
			message = "YUCORE_MEDIA_BASE_URL or YUCORE_MEDIA_UAG_BASE_URL is required for uag-proxy media."
		}
	default:
		message = "Unsupported YUCORE_MEDIA_ADAPTER."
	}
	upstreamVerified, upstreamVerificationStatus, upstreamVerificationMessage := yucoreMediaUpstreamVerification(config, configured)
	verificationBlockers := yucoreMediaVerificationBlockers(config, configured, upstreamVerified)
	if message == "" && !upstreamVerified && upstreamVerificationStatus == "unverified" {
		message = upstreamVerificationMessage
	}
	return YucoreMediaAdapterInfo{
		Adapter:                     config.Adapter,
		Configured:                  configured,
		BaseURLConfigured:           config.BaseURL != "",
		APIKeyConfigured:            config.APIKey != "",
		AuthMode:                    authMode,
		Status:                      status,
		Message:                     message,
		SupportsImage:               config.Adapter == YucoreMediaAdapterMock || (configured && (config.Adapter == YucoreMediaAdapterOpenAICompatible || config.Adapter == YucoreMediaAdapterYuAPIChannel || config.Adapter == YucoreMediaAdapterUAGProxy)),
		SupportsVideo:               config.Adapter == YucoreMediaAdapterMock || (configured && (config.Adapter == YucoreMediaAdapterOpenAICompatible || config.Adapter == YucoreMediaAdapterYuAPIChannel || config.Adapter == YucoreMediaAdapterUAGProxy)),
		RequireRealAssets:           config.RequireRealAssets,
		MockFallback:                config.Adapter == YucoreMediaAdapterMock && !config.RequireRealAssets,
		UpstreamVerified:            upstreamVerified,
		UpstreamVerificationStatus:  upstreamVerificationStatus,
		UpstreamVerificationMessage: upstreamVerificationMessage,
		RealWorkflowReady:           len(verificationBlockers) == 0,
		VerificationBlockers:        verificationBlockers,
	}
}

func resolveYucoreMediaTaskAdapter(task *YucoreMediaTask) (string, error) {
	config := getYucoreMediaAdapterConfig()
	switch config.Adapter {
	case YucoreMediaAdapterMock:
		if config.RequireRealAssets {
			return "", errors.New("YUCORE_MEDIA_REQUIRE_REAL_ASSETS=true forbids mock YuCore media tasks")
		}
		return YucoreMediaAdapterMock, nil
	case YucoreMediaAdapterOpenAICompatible:
		if config.BaseURL == "" || config.APIKey == "" {
			return "", errors.New("YUCORE_MEDIA_BASE_URL and YUCORE_MEDIA_API_KEY are required for openai-compatible YuCore media tasks")
		}
		return YucoreMediaAdapterOpenAICompatible, nil
	case YucoreMediaAdapterYuAPIChannel:
		if config.BaseURL == "" || config.ManagedTokenGroup == "" {
			return "", errors.New("YUCORE_MEDIA_BASE_URL and YUCORE_MEDIA_MANAGED_TOKEN_GROUP are required for yuapi-channel YuCore media tasks")
		}
		return YucoreMediaAdapterYuAPIChannel, nil
	case YucoreMediaAdapterUAGProxy:
		if config.BaseURL == "" {
			return "", errors.New("YUCORE_MEDIA_BASE_URL or YUCORE_MEDIA_UAG_BASE_URL is required for uag-proxy YuCore media tasks")
		}
		return YucoreMediaAdapterUAGProxy, nil
	default:
		return "", errors.New("YUCORE_MEDIA_ADAPTER must be mock, openai-compatible, yuapi-channel, or uag-proxy")
	}
}

func yucoreMediaMetadataMap(value string) map[string]any {
	metadata := map[string]any{}
	if value != "" {
		_ = common.Unmarshal([]byte(value), &metadata)
	}
	return metadata
}

func mergeYucoreMediaMetadata(value string, patch map[string]any) string {
	metadata := yucoreMediaMetadataMap(value)
	for key, val := range patch {
		metadata[key] = val
	}
	raw, err := common.Marshal(metadata)
	if err != nil {
		return value
	}
	return string(raw)
}

func yucoreMediaTaskAdapter(task *YucoreMediaTask) string {
	if task == nil {
		return YucoreMediaAdapterMock
	}
	if adapter, ok := yucoreMediaMetadataMap(task.Metadata)["adapter"].(string); ok {
		return normalizeYucoreMediaAdapterName(adapter)
	}
	return YucoreMediaAdapterMock
}

func IsYucoreMediaMockTask(task *YucoreMediaTask) bool {
	return yucoreMediaTaskAdapter(task) == YucoreMediaAdapterMock
}

func IsYucoreMediaUAGProxyTask(task *YucoreMediaTask) bool {
	return yucoreMediaTaskAdapter(task) == YucoreMediaAdapterUAGProxy
}

func IsYucoreMediaUAGProxyConfigured() bool {
	config := getYucoreMediaAdapterConfig()
	return config.Adapter == YucoreMediaAdapterUAGProxy && config.BaseURL != ""
}

func hideYucoreMediaTaskInRealAssetMode(task *YucoreMediaTask, config yucoreMediaAdapterConfig) bool {
	return config.RequireRealAssets && IsYucoreMediaMockTask(task)
}

func YucoreMediaUAGProxyAuthorizationHeader() string {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.APIKey == "" {
		return ""
	}
	return "Bearer " + config.APIKey
}

func normalizeYucoreMediaUAGProxyHeaders(headers YucoreMediaUAGProxyHeaders) YucoreMediaUAGProxyHeaders {
	normalized := YucoreMediaUAGProxyHeaders{}
	for key, value := range headers {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "authorization":
			normalized["Authorization"] = value
		case "cookie":
			normalized["Cookie"] = value
		case "x-request-id":
			normalized["X-Request-Id"] = value
		case "x-demo-user":
			normalized["X-Demo-User"] = value
		case "x-yucore-canvas-identity":
			normalized["X-YuCore-Canvas-Identity"] = value
		case "x-yucore-canvas-session":
			normalized["X-YuCore-Canvas-Session"] = value
		case "idempotency-key":
			normalized["Idempotency-Key"] = value
		}
	}
	return normalized
}

func cloneYucoreMediaUAGProxyHeaders(headers YucoreMediaUAGProxyHeaders) YucoreMediaUAGProxyHeaders {
	return normalizeYucoreMediaUAGProxyHeaders(headers)
}

func normalizeYucoreMediaTask(task *YucoreMediaTask) {
	task.SessionId = strings.TrimSpace(task.SessionId)
	task.BillingGroup = strings.TrimSpace(task.BillingGroup)
	task.Kind = strings.TrimSpace(task.Kind)
	if task.Kind != "video" {
		task.Kind = "image"
	}
	task.Mode = strings.TrimSpace(task.Mode)
	if task.Mode == "" {
		if task.Kind == "video" {
			task.Mode = "text-to-video"
		} else {
			task.Mode = "text-to-image"
		}
	}
	task.ModelId = strings.TrimSpace(task.ModelId)
	if task.ModelId == "" {
		if task.Kind == "video" {
			task.ModelId = "veo-3.1-generate-preview"
		} else {
			task.ModelId = "gpt-image-2"
		}
	}
	task.Prompt = strings.TrimSpace(task.Prompt)
	task.NegativePrompt = strings.TrimSpace(task.NegativePrompt)
	task.AspectRatio = strings.TrimSpace(task.AspectRatio)
	if task.AspectRatio == "" {
		task.AspectRatio = "auto"
	}
	task.Size = strings.TrimSpace(task.Size)
	if task.Size == "" {
		task.Size = "1k"
	}
	task.Quality = strings.TrimSpace(task.Quality)
	if task.Quality == "" {
		task.Quality = "high"
	}
	task.Format = strings.TrimSpace(strings.ToLower(task.Format))
	if task.Format == "" {
		task.Format = "png"
	}
	if task.Count <= 0 {
		task.Count = 1
	}
	if task.Count > 10 {
		task.Count = 10
	}
	if task.Kind == "video" {
		task.Count = 1
	}
	if task.Status == "" {
		task.Status = YucoreMediaTaskStatusProcessing
	}
	if task.Progress < 0 {
		task.Progress = 0
	}
	if task.Progress > 100 {
		task.Progress = 100
	}
	if task.Inputs == "" {
		task.Inputs = "[]"
	}
	if task.Metadata == "" {
		task.Metadata = "{}"
	}
	if task.Assets == "" {
		task.Assets = "[]"
	}
	task.Cost = estimateYucoreMediaTaskCost(task)
}

func estimateYucoreMediaTaskCost(task *YucoreMediaTask) int {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter == YucoreMediaAdapterYuAPIChannel {
		if price, ok := yucoreMediaModelUnitPriceForGroup(task.ModelId, yucoreMediaTaskBillingGroup(task, config)); ok {
			units := max(task.Count, 1)
			if task.Kind == "video" && !YucoreMediaModelUsesPerCallPricing(task.ModelId) {
				units = yucoreMediaTaskDuration(task)
				if units <= 0 {
					capability := yucoreMediaCapabilityForTask(task, config)
					units = capability.FixedDurationSeconds
				}
				if units <= 0 {
					units = 1
				}
			}
			return int(math.Ceil(price * common.QuotaPerUnit * float64(units)))
		}
	}
	if task.Kind == "video" {
		return 3000
	}
	sizeBase := map[string]int{
		"auto":      220,
		"1k":        200,
		"2k":        320,
		"4k":        480,
		"1024":      200,
		"2048":      320,
		"4096":      480,
		"1024x1024": 200,
		"1536x1024": 260,
		"1024x1536": 260,
		"2048x2048": 420,
		"2048x1152": 360,
		"3840x2160": 760,
		"2160x3840": 760,
		"custom":    260,
	}
	base := sizeBase[strings.ToLower(task.Size)]
	if base == 0 {
		base = 200
	}
	multiplier := 1.0
	if strings.EqualFold(task.Quality, "high") {
		multiplier = 1.25
	}
	return int(math.Ceil(float64(base*task.Count) * multiplier))
}

// YucoreMediaModelUsesPerCallPricing reports whether task duration must not multiply the configured model price.
func YucoreMediaModelUsesPerCallPricing(modelID string) bool {
	return constant.TaskPricePatchApplies(modelID)
}

func YucoreMediaModelUnitPrice(modelId string) (float64, bool) {
	config := getYucoreMediaAdapterConfig()
	return yucoreMediaModelUnitPriceForGroup(modelId, config.ManagedTokenGroup)
}

func yucoreMediaModelUnitPriceForGroup(modelId string, group string) (float64, bool) {
	price, ok := ratio_setting.GetModelPrice(modelId, false)
	if !ok {
		return 0, false
	}
	config := getYucoreMediaAdapterConfig()
	if config.Adapter == YucoreMediaAdapterYuAPIChannel {
		price *= ratio_setting.GetGroupRatio(strings.TrimSpace(group))
	}
	return price, true
}

func yucoreMediaTaskBillingGroup(task *YucoreMediaTask, config yucoreMediaAdapterConfig) string {
	if task != nil {
		if group := strings.TrimSpace(task.BillingGroup); group != "" {
			return group
		}
	}
	return strings.TrimSpace(config.ManagedTokenGroup)
}

func buildYucoreMediaAssets(task *YucoreMediaTask) []YucoreMediaAsset {
	count := task.Count
	if count <= 0 {
		count = 1
	}
	if task.Kind == "video" {
		count = 1
	}
	assets := make([]YucoreMediaAsset, 0, count)
	for i := 0; i < count; i++ {
		asset := YucoreMediaAsset{
			Id:       fmt.Sprintf("%s_asset_%d", task.TaskId, i),
			Kind:     task.Kind,
			Url:      fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, i),
			Label:    fmt.Sprintf("%s result %d", task.ModelId, i+1),
			Width:    1024,
			Height:   1024,
			MimeType: "image/svg+xml",
			Metadata: map[string]any{"mock": true},
		}
		if task.Kind == "video" {
			asset.Label = fmt.Sprintf("%s preview", task.ModelId)
			asset.Width = 1280
			asset.Height = 720
		}
		assets = append(assets, asset)
	}
	return assets
}

func YucoreMediaTaskAssets(task *YucoreMediaTask) []YucoreMediaAsset {
	var assets []YucoreMediaAsset
	if task == nil || task.Assets == "" {
		return assets
	}
	if err := common.Unmarshal([]byte(task.Assets), &assets); err != nil {
		return []YucoreMediaAsset{}
	}
	return assets
}

func YucoreMediaAssetSource(asset YucoreMediaAsset) string {
	for _, value := range []string{asset.SourceUrl, asset.CachedUrl} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	for _, key := range []string{"source_url", "sourceUrl", "cached_url", "cachedUrl", "url"} {
		if asset.Metadata != nil {
			if value := yucoreMediaStringValue(asset.Metadata[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func ResolveYucoreMediaAssetSourceURL(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("missing YuCore media asset source URL")
	}
	if strings.HasPrefix(source, "data:") {
		return source, nil
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", errors.New("unsupported YuCore media asset source URL scheme")
		}
		return parsed.String(), nil
	}
	config := getYucoreMediaAdapterConfig()
	if config.BaseURL == "" {
		return "", errors.New("relative YuCore media asset source URL requires YUCORE_MEDIA_BASE_URL")
	}
	base, err := url.Parse(config.BaseURL + "/")
	if err != nil {
		return "", err
	}
	return base.ResolveReference(parsed).String(), nil
}

func YucoreMediaUpstreamAssetProxyURL(source string) string {
	source = strings.TrimSpace(source)
	if source == "" || strings.HasPrefix(source, "data:") {
		return source
	}
	config := getYucoreMediaAdapterConfig()
	if config.BaseURL == "" {
		return source
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return source
	}
	if parsed.IsAbs() {
		base, err := url.Parse(config.BaseURL)
		if err != nil || base.Scheme == "" || base.Host == "" || base.Host != parsed.Host || base.Scheme != parsed.Scheme {
			return source
		}
		return "/api/yucore/media/upstream-assets" + parsed.EscapedPath() + optionalYucoreMediaRawQuery(parsed)
	}
	path := source
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/api/yucore/media/upstream-assets" + path
}

func optionalYucoreMediaRawQuery(parsed *url.URL) string {
	if parsed == nil || parsed.RawQuery == "" {
		return ""
	}
	return "?" + parsed.RawQuery
}

func settleYucoreMediaTaskWithAssets(task *YucoreMediaTask, assets []YucoreMediaAsset) error {
	now := common.GetTimestamp()
	rawAssets, _ := common.Marshal(assets)
	task.Status = YucoreMediaTaskStatusCompleted
	task.Progress = 100
	task.Assets = YucoreMediaAssets(rawAssets)
	task.Error = ""
	task.UpdatedTime = now
	err := DB.Model(task).
		Select("status", "progress", "assets", "metadata", "error", "cost", "updated_time").
		Updates(task).Error
	if err != nil {
		return err
	}
	notifyYucoreMediaTaskTerminalBackflow(task)
	return nil
}

func settleYucoreMediaTask(task *YucoreMediaTask) error {
	return settleYucoreMediaTaskWithAssets(task, buildYucoreMediaAssets(task))
}

func failYucoreMediaTask(task *YucoreMediaTask, err error) error {
	if task == nil {
		return nil
	}
	message := "media task failed"
	if err != nil {
		message = err.Error()
	}
	if len(message) > 512 {
		message = message[:512]
	}
	task.Status = YucoreMediaTaskStatusFailed
	task.Progress = 100
	task.Error = message
	task.UpdatedTime = common.GetTimestamp()
	updateErr := DB.Model(task).
		Select("status", "progress", "error", "updated_time").
		Updates(task).Error
	if updateErr != nil {
		return updateErr
	}
	notifyYucoreMediaTaskTerminalBackflow(task)
	return nil
}

func notifyYucoreMediaTaskTerminalBackflow(task *YucoreMediaTask) {
	if task == nil {
		return
	}
	if task.Status != YucoreMediaTaskStatusCompleted && task.Status != YucoreMediaTaskStatusFailed && task.Status != YucoreMediaTaskStatusCanceled {
		return
	}
	if err := ApplyYucoreCanvasAgentMediaBackflow(task); err != nil {
		common.SysError("YuCore media task backflow failed: " + err.Error())
	}
}

func HydrateYucoreMediaTask(task *YucoreMediaTask) (*YucoreMediaTask, error) {
	return HydrateYucoreMediaTaskWithHeaders(task, nil)
}

func HydrateYucoreMediaTaskWithHeaders(task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) (*YucoreMediaTask, error) {
	if task == nil {
		return nil, nil
	}
	if task.Status != YucoreMediaTaskStatusProcessing && task.Status != YucoreMediaTaskStatusPending {
		return task, nil
	}
	now := common.GetTimestamp()
	elapsed := now - task.CreatedTime
	adapter := yucoreMediaTaskAdapter(task)
	if adapter == YucoreMediaAdapterUAGProxy {
		if err := refreshUAGProxyYucoreTaskWithHeaders(task, upstreamHeaders); err != nil {
			task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
				"last_status_error": err.Error(),
				"last_status_at":    now,
			})
			task.UpdatedTime = now
			_ = DB.Model(task).Select("metadata", "updated_time").Updates(task).Error
		}
		if task.Status != YucoreMediaTaskStatusProcessing && task.Status != YucoreMediaTaskStatusPending {
			return task, nil
		}
	}
	if adapter == YucoreMediaAdapterOpenAICompatible || adapter == YucoreMediaAdapterYuAPIChannel {
		config, err := yucoreMediaOpenAIConfigForTask(task, getYucoreMediaAdapterConfig())
		if err == nil {
			err = refreshOpenAICompatibleYucoreTask(task, config)
		}
		if err != nil {
			task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
				"last_status_error": err.Error(),
				"last_status_at":    now,
			})
			task.UpdatedTime = now
			_ = DB.Model(task).Select("metadata", "updated_time").Updates(task).Error
		}
		if task.Status != YucoreMediaTaskStatusProcessing && task.Status != YucoreMediaTaskStatusPending {
			return task, nil
		}
	}
	if adapter == YucoreMediaAdapterMock && elapsed >= 4 {
		if err := settleYucoreMediaTask(task); err != nil {
			return nil, err
		}
		return task, nil
	}
	progress := int(18 + elapsed*12)
	if adapter == YucoreMediaAdapterMock {
		progress = int(18 + elapsed*22)
	}
	if progress > 96 {
		progress = 96
	}
	if progress > task.Progress {
		task.Progress = progress
		task.UpdatedTime = now
		_ = DB.Model(task).Select("progress", "updated_time").Updates(task).Error
	}
	return task, nil
}

func normalizeYucoreMediaImageSize(size string) string {
	size = strings.TrimSpace(size)
	switch strings.ToLower(size) {
	case "", "auto":
		return ""
	case "1k", "1024":
		return "1024x1024"
	case "2k", "2048":
		return "2048x2048"
	case "4k", "4096":
		return "4096x4096"
	default:
		return size
	}
}

func yucoreMediaEndpoint(baseURL string) (string, error) {
	if baseURL == "" {
		return "", errors.New("missing YuCore media base URL")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid YuCore media base URL")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/images/generations") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(path, "/v1") {
		parsed.Path = path + "/images/generations"
	} else {
		parsed.Path = path + "/v1/images/generations"
	}
	return parsed.String(), nil
}

func yucoreMediaImageMimeType(format string) string {
	format = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(format)), "image/")
	switch format {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func yucoreMediaBuildURL(baseURL string, path string) (string, error) {
	if baseURL == "" {
		return "", errors.New("missing YuCore media base URL")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid YuCore media base URL")
	}
	normalizedPath := "/" + strings.TrimLeft(path, "/")
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/api/v1") && strings.HasPrefix(normalizedPath, "/api/v1/") {
		return base + strings.TrimPrefix(normalizedPath, "/api/v1"), nil
	}
	return base + normalizedPath, nil
}

func yucoreMediaModeToUAGMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "text-to-image":
		return "t2i"
	case "image-to-image":
		return "i2i"
	case "text-to-video":
		return "t2v"
	case "image-to-video":
		return "i2v"
	default:
		return strings.TrimSpace(mode)
	}
}

func yucoreMediaStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", typed))
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func yucoreMediaIntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func yucoreMediaBoolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "enabled":
			return true
		case "false", "0", "no", "disabled":
			return false
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed != 0
		}
	}
	return fallback
}

func yucoreMediaMapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func yucoreMediaSliceValue(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func yucoreMediaFirstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := yucoreMediaStringValue(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func yucoreMediaUAGStatus(value any) string {
	switch strings.ToLower(strings.TrimSpace(yucoreMediaStringValue(value))) {
	case "0", "pending", "queued":
		return YucoreMediaTaskStatusPending
	case "2", "completed", "complete", "succeeded", "success":
		return YucoreMediaTaskStatusCompleted
	case "3", "failed", "error":
		return YucoreMediaTaskStatusFailed
	case "4", "canceled", "cancelled":
		return YucoreMediaTaskStatusCanceled
	case "1", "processing", "running", "":
		return YucoreMediaTaskStatusProcessing
	default:
		return YucoreMediaTaskStatusProcessing
	}
}

func yucoreMediaReferenceAssets(task *YucoreMediaTask) []string {
	refs := make([]string, 0)
	var inputs []any
	if task.Inputs != "" {
		_ = common.Unmarshal([]byte(task.Inputs), &inputs)
	}
	for _, input := range inputs {
		switch typed := input.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				refs = append(refs, strings.TrimSpace(typed))
			}
		case map[string]any:
			if value := yucoreMediaFirstString(typed, "dataUrl", "data_url", "cachedUrl", "cached_url", "sourceUrl", "source_url", "url", "path", "id"); value != "" {
				refs = append(refs, value)
			}
		}
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	for _, key := range []string{"ref_assets", "refAssets"} {
		for _, item := range yucoreMediaSliceValue(metadata[key]) {
			if value := yucoreMediaStringValue(item); value != "" {
				refs = append(refs, value)
			}
		}
	}
	return refs
}

func buildUAGProxyCreatePayload(task *YucoreMediaTask, config yucoreMediaAdapterConfig) map[string]any {
	upstreamModelID := yucoreMediaUAGModelID(config, task.ModelId)
	params := map[string]any{
		"size":            task.Size,
		"resolution":      task.Size,
		"image_size":      task.Size,
		"ratio":           task.AspectRatio,
		"aspect_ratio":    task.AspectRatio,
		"quality":         task.Quality,
		"response_format": "url",
		"stream":          false,
		"stream_mode":     "final",
		"yucore_model_id": task.ModelId,
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	for _, key := range []string{"duration", "durationSeconds", "duration_seconds", "style_preset", "seed"} {
		if value, ok := metadata[key]; ok {
			params[key] = value
		}
	}
	payload := map[string]any{
		"model":      upstreamModelID,
		"prompt":     task.Prompt,
		"mode":       yucoreMediaModeToUAGMode(task.Mode),
		"count":      max(task.Count, 1),
		"size":       task.Size,
		"ratio":      task.AspectRatio,
		"ref_assets": yucoreMediaReferenceAssets(task),
		"params":     params,
	}
	if task.NegativePrompt != "" {
		payload["negative_prompt"] = task.NegativePrompt
	}
	return payload
}

func requestUAGProxyJSON(config yucoreMediaAdapterConfig, method string, path string, body any) (map[string]any, []byte, int, error) {
	return requestUAGProxyJSONWithHeaders(config, method, path, body, nil)
}

func requestUAGProxyJSONWithHeaders(config yucoreMediaAdapterConfig, method string, path string, body any, upstreamHeaders YucoreMediaUAGProxyHeaders) (map[string]any, []byte, int, error) {
	endpoint, err := yucoreMediaBuildURL(config.BaseURL, path)
	if err != nil {
		return nil, nil, 0, err
	}
	var reader io.Reader
	if body != nil {
		rawBody, err := common.Marshal(body)
		if err != nil {
			return nil, nil, 0, err
		}
		reader = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range normalizeYucoreMediaUAGProxyHeaders(upstreamHeaders) {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Authorization") == "" && config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}
	client := &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, respBody, resp.StatusCode, fmt.Errorf("YuCore UAG upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var payload map[string]any
	if len(respBody) > 0 {
		if err := common.Unmarshal(respBody, &payload); err != nil {
			return nil, respBody, resp.StatusCode, err
		}
	}
	return payload, respBody, resp.StatusCode, nil
}

func uagProxyDataRow(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if data := yucoreMediaMapValue(payload["data"]); data != nil {
		if task := yucoreMediaMapValue(data["task"]); task != nil {
			return task
		}
		return data
	}
	return payload
}

func uagProxyTaskID(row map[string]any) string {
	return yucoreMediaFirstString(row, "task_id", "taskId", "id", "external_id", "externalId")
}

func uagProxyTaskError(row map[string]any, payload map[string]any) string {
	if value := yucoreMediaFirstString(row, "error", "message", "msg", "error_message", "errorMessage"); value != "" {
		return value
	}
	if value := yucoreMediaFirstString(payload, "error", "message", "msg"); value != "" {
		return value
	}
	return "YuCore UAG task failed"
}

func uagProxyResultItems(row map[string]any) []any {
	for _, key := range []string{"results", "assets", "images", "videos", "outputs", "output"} {
		if items := yucoreMediaSliceValue(row[key]); len(items) > 0 {
			return items
		}
		if value := yucoreMediaStringValue(row[key]); value != "" {
			return []any{value}
		}
	}
	return nil
}

func uagProxyRows(payload map[string]any) []any {
	if payload == nil {
		return nil
	}
	candidates := []any{payload["data"], payload}
	for _, candidate := range candidates {
		if rows := yucoreMediaSliceValue(candidate); len(rows) > 0 {
			return rows
		}
		row := yucoreMediaMapValue(candidate)
		if row == nil {
			continue
		}
		for _, key := range []string{"list", "items", "tasks", "history", "rows", "data"} {
			if rows := yucoreMediaSliceValue(row[key]); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func ListYucoreUAGProxyMediaModels() ([]map[string]any, error) {
	return ListYucoreUAGProxyMediaModelsWithHeaders(nil)
}

func ListYucoreUAGProxyMediaModelsWithHeaders(upstreamHeaders YucoreMediaUAGProxyHeaders) ([]map[string]any, error) {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.BaseURL == "" {
		return nil, errors.New("uag-proxy YuCore media adapter is not configured")
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodGet, "/api/v1/models", nil, upstreamHeaders)
	if err != nil {
		return nil, err
	}
	models := make([]map[string]any, 0)
	for _, item := range uagProxyRows(payload) {
		row := yucoreMediaMapValue(item)
		if row == nil {
			continue
		}
		model, ok := yucoreMediaUAGModelRow(row, config)
		if ok {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return nil, errors.New("YuCore UAG upstream returned no image or video models")
	}
	return models, nil
}

func yucoreMediaUAGModelRow(row map[string]any, config yucoreMediaAdapterConfig) (map[string]any, bool) {
	if !yucoreMediaBoolValue(row["enabled"], true) {
		return nil, false
	}
	modelID := yucoreMediaFirstString(row, "model_code", "model", "id", "modelId")
	if modelID == "" {
		return nil, false
	}
	if !yucoreMediaUAGAllowed(config.UAGAllowedModels, modelID) {
		return nil, false
	}
	kind := strings.ToLower(yucoreMediaFirstString(row, "kind", "type"))
	if kind != "image" && kind != "video" {
		return nil, false
	}
	provider := strings.ToLower(yucoreMediaFirstString(row, "provider", "family"))
	if provider == "" {
		provider = "uag"
	}
	if !yucoreMediaUAGAllowed(config.UAGAllowedProviders, provider) {
		return nil, false
	}
	name := yucoreMediaFirstString(row, "name", "label")
	if name == "" {
		name = modelID
	}
	model := map[string]any{
		"id":             modelID,
		"name":           name,
		"family":         provider,
		"badge":          strings.ToUpper(provider) + " UAG",
		"description":    "Model discovered from the configured UAG media gateway.",
		"kind":           kind,
		"source":         "uag-proxy",
		"upstream_model": yucoreMediaFirstString(row, "upstream_model", "upstreamModel"),
		"pricing": map[string]any{
			"unit":        "uag_points",
			"unit_points": yucoreMediaIntValue(row["unit_points"]),
		},
	}
	if kind == "video" {
		model["modes"] = []string{"text-to-video", "image-to-video"}
		model["sizes"] = []string{"720p", "1080p"}
		model["size_label"] = "resolution"
		model["aspect_ratios"] = []string{"16:9", "9:16", "1:1"}
		model["durations"] = []int{4, 6, 8}
		model["stream_modes"] = []string{"poll"}
		model["style_presets"] = []string{"auto", "cinematic", "commercial", "realistic"}
		model["counts"] = []int{1}
		model["input_limits"] = map[string]any{
			"max_prompt_chars":     1800,
			"max_reference_images": 3,
		}
		return model, true
	}
	model["modes"] = []string{"text-to-image", "image-to-image"}
	model["sizes"] = []string{"1024x1024", "1536x1024", "1024x1536"}
	model["size_label"] = "size"
	model["aspect_ratios"] = []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
	model["qualities"] = []string{"draft", "standard", "hd"}
	model["output_formats"] = []string{"url"}
	model["formats"] = []string{"url"}
	model["stream_modes"] = []string{"final"}
	model["style_presets"] = []string{"auto", "commercial", "product", "editorial", "realistic"}
	model["counts"] = []int{1, 2, 4}
	model["input_limits"] = map[string]any{
		"max_prompt_chars":     4000,
		"max_reference_images": 8,
	}
	return model, true
}

func uagProxyAssetURL(item any) (sourceURL string, cachedURL string) {
	if value := yucoreMediaStringValue(item); value != "" {
		return value, ""
	}
	row := yucoreMediaMapValue(item)
	if row == nil {
		return "", ""
	}
	cachedURL = yucoreMediaFirstString(row, "cachedUrl", "cached_url")
	sourceURL = yucoreMediaFirstString(row, "sourceUrl", "source_url", "url", "download_url", "downloadUrl", "video_url", "videoUrl", "image_url", "imageUrl", "thumb_url", "thumbUrl")
	if sourceURL == "" {
		sourceURL = cachedURL
	}
	return sourceURL, cachedURL
}

func buildUAGProxyAssets(task *YucoreMediaTask, row map[string]any) []YucoreMediaAsset {
	items := uagProxyResultItems(row)
	assets := make([]YucoreMediaAsset, 0, len(items))
	mimeType := "image/png"
	if task.Kind == "video" {
		mimeType = "video/mp4"
	}
	for index, item := range items {
		sourceURL, cachedURL := uagProxyAssetURL(item)
		if sourceURL == "" && cachedURL == "" {
			continue
		}
		itemMap := yucoreMediaMapValue(item)
		width := 0
		height := 0
		durationMs := 0
		if itemMap != nil {
			width = yucoreMediaIntValue(itemMap["width"])
			height = yucoreMediaIntValue(itemMap["height"])
			durationMs = yucoreMediaIntValue(itemMap["duration_ms"])
			if durationMs == 0 {
				durationMs = yucoreMediaIntValue(itemMap["durationMs"])
			}
			if itemMimeType := yucoreMediaFirstString(itemMap, "mime_type", "mimeType", "content_type", "contentType"); itemMimeType != "" {
				mimeType = itemMimeType
			}
		}
		assets = append(assets, YucoreMediaAsset{
			Id:         fmt.Sprintf("%s_asset_%d", task.TaskId, index),
			Kind:       task.Kind,
			Url:        fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
			ThumbUrl:   fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
			CachedUrl:  cachedURL,
			SourceUrl:  sourceURL,
			Label:      fmt.Sprintf("%s result %d", task.ModelId, index+1),
			Width:      width,
			Height:     height,
			DurationMs: durationMs,
			MimeType:   mimeType,
			Metadata: map[string]any{
				"adapter": "uag-proxy",
			},
		})
	}
	return assets
}

func buildUAGProxyHistoryAssets(task *YucoreMediaTask, row map[string]any) []YucoreMediaAsset {
	items := uagProxyResultItems(row)
	assets := make([]YucoreMediaAsset, 0, len(items))
	mimeType := "image/png"
	if task.Kind == "video" {
		mimeType = "video/mp4"
	}
	for index, item := range items {
		sourceURL, cachedURL := uagProxyAssetURL(item)
		if sourceURL == "" && cachedURL == "" {
			continue
		}
		itemMap := yucoreMediaMapValue(item)
		width := 0
		height := 0
		durationMs := 0
		if itemMap != nil {
			width = yucoreMediaIntValue(itemMap["width"])
			height = yucoreMediaIntValue(itemMap["height"])
			durationMs = yucoreMediaIntValue(itemMap["duration_ms"])
			if durationMs == 0 {
				durationMs = yucoreMediaIntValue(itemMap["durationMs"])
			}
			if itemMimeType := yucoreMediaFirstString(itemMap, "mime_type", "mimeType", "content_type", "contentType"); itemMimeType != "" {
				mimeType = itemMimeType
			}
		}
		publicURL := YucoreMediaUpstreamAssetProxyURL(sourceURL)
		if publicURL == "" {
			publicURL = YucoreMediaUpstreamAssetProxyURL(cachedURL)
		}
		assets = append(assets, YucoreMediaAsset{
			Id:         fmt.Sprintf("%s_asset_%d", task.TaskId, index),
			Kind:       task.Kind,
			Url:        publicURL,
			ThumbUrl:   publicURL,
			CachedUrl:  cachedURL,
			SourceUrl:  sourceURL,
			Label:      fmt.Sprintf("%s result %d", task.ModelId, index+1),
			Width:      width,
			Height:     height,
			DurationMs: durationMs,
			MimeType:   mimeType,
			Metadata: map[string]any{
				"adapter": "uag-proxy",
				"history": true,
			},
		})
	}
	return assets
}

func yucoreMediaUAGKind(row map[string]any) string {
	kind := strings.ToLower(yucoreMediaFirstString(row, "kind", "media_type", "mediaType", "type"))
	if kind == "video" {
		return "video"
	}
	mode := strings.ToLower(yucoreMediaFirstString(row, "mode"))
	if mode == "t2v" || mode == "i2v" || strings.Contains(mode, "video") {
		return "video"
	}
	return "image"
}

func yucoreMediaUAGMode(row map[string]any, kind string) string {
	switch strings.ToLower(yucoreMediaFirstString(row, "mode")) {
	case "t2i":
		return "text-to-image"
	case "i2i":
		return "image-to-image"
	case "t2v":
		return "text-to-video"
	case "i2v":
		return "image-to-video"
	default:
		if kind == "video" {
			return "text-to-video"
		}
		return "text-to-image"
	}
}

func yucoreMediaUAGTimestamp(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		parsed, _ := typed.Int64()
		return yucoreMediaNormalizeTimestamp(parsed)
	case float64:
		return yucoreMediaNormalizeTimestamp(int64(typed))
	case int64:
		return yucoreMediaNormalizeTimestamp(typed)
	case int:
		return yucoreMediaNormalizeTimestamp(int64(typed))
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return yucoreMediaNormalizeTimestamp(parsed)
		}
		if parsedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(typed)); err == nil {
			return parsedTime.Unix()
		}
	}
	return common.GetTimestamp()
}

func yucoreMediaNormalizeTimestamp(value int64) int64 {
	if value <= 0 {
		return common.GetTimestamp()
	}
	if value > 1_000_000_000_000 {
		return value / 1000
	}
	return value
}

func uagProxyHistoryTask(row map[string]any, userId int) (*YucoreMediaTask, bool) {
	if row == nil {
		return nil, false
	}
	taskId := uagProxyTaskID(row)
	if taskId == "" {
		return nil, false
	}
	kind := yucoreMediaUAGKind(row)
	mode := yucoreMediaUAGMode(row, kind)
	createdAt := yucoreMediaUAGTimestamp(row["created_at"])
	if updated := yucoreMediaUAGTimestamp(row["updated_at"]); updated > 0 {
		createdAt = min(createdAt, updated)
	}
	task := &YucoreMediaTask{
		UserId:         userId,
		TaskId:         taskId,
		SessionId:      yucoreMediaFirstString(row, "session_id", "sessionId"),
		Kind:           kind,
		Mode:           mode,
		ModelId:        yucoreMediaFirstString(row, "model", "model_id", "modelId"),
		Prompt:         yucoreMediaFirstString(row, "prompt"),
		NegativePrompt: yucoreMediaFirstString(row, "negative_prompt", "negativePrompt"),
		AspectRatio:    yucoreMediaFirstString(row, "aspect_ratio", "aspectRatio", "ratio"),
		Size:           yucoreMediaFirstString(row, "size", "resolution", "image_size", "imageSize"),
		Quality:        yucoreMediaFirstString(row, "quality"),
		Format:         yucoreMediaFirstString(row, "format", "output_format", "outputFormat"),
		Count:          max(yucoreMediaIntValue(row["count"]), 1),
		Status:         yucoreMediaUAGStatus(row["status"]),
		Progress:       yucoreMediaIntValue(row["progress"]),
		Cost:           yucoreMediaIntValue(row["cost_points"]),
		Inputs:         "[]",
		Metadata:       "{}",
		CreatedTime:    createdAt,
		UpdatedTime:    yucoreMediaUAGTimestamp(row["updated_at"]),
	}
	if task.Status == YucoreMediaTaskStatusFailed {
		task.Error = uagProxyTaskError(row, row)
	}
	if task.ModelId == "" {
		if kind == "video" {
			task.ModelId = "veo-3.1-generate-preview"
		} else {
			task.ModelId = "gpt-image-2"
		}
	}
	if task.UpdatedTime <= 0 {
		task.UpdatedTime = task.CreatedTime
	}
	if task.Progress <= 0 && task.Status == YucoreMediaTaskStatusCompleted {
		task.Progress = 100
	}
	if task.Cost <= 0 {
		task.Cost = yucoreMediaIntValue(row["cost"])
	}
	assets := buildUAGProxyHistoryAssets(task, row)
	rawAssets, _ := common.Marshal(assets)
	task.Assets = YucoreMediaAssets(rawAssets)
	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
		"adapter":          YucoreMediaAdapterUAGProxy,
		"upstream_task_id": taskId,
		"history":          true,
	})
	return task, true
}

func ListYucoreUAGProxyMediaTasks(userId int, kind string, status string, startIdx int, num int) ([]*YucoreMediaTask, int64, error) {
	return ListYucoreUAGProxyMediaTasksWithHeaders(userId, kind, status, startIdx, num, nil)
}

func ListYucoreUAGProxyMediaTasksWithHeaders(userId int, kind string, status string, startIdx int, num int, upstreamHeaders YucoreMediaUAGProxyHeaders) ([]*YucoreMediaTask, int64, error) {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.BaseURL == "" {
		return nil, 0, errors.New("uag-proxy YuCore media adapter is not configured")
	}
	pageSize := max(startIdx+num, 100)
	if pageSize > 200 {
		pageSize = 200
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodGet, fmt.Sprintf("/api/v1/gen/history?page=1&page_size=%d", pageSize), nil, upstreamHeaders)
	if err != nil {
		return nil, 0, err
	}
	rows := uagProxyRows(payload)
	tasks := make([]*YucoreMediaTask, 0, len(rows))
	for _, item := range rows {
		task, ok := uagProxyHistoryTask(yucoreMediaMapValue(item), userId)
		if !ok {
			continue
		}
		if kind != "" && task.Kind != kind {
			continue
		}
		if status != "" && task.Status != status {
			continue
		}
		tasks = append(tasks, task)
	}
	total := int64(len(tasks))
	if startIdx >= len(tasks) {
		return []*YucoreMediaTask{}, total, nil
	}
	end := startIdx + num
	if num <= 0 || end > len(tasks) {
		end = len(tasks)
	}
	return tasks[startIdx:end], total, nil
}

func ListYucoreMergedUAGProxyMediaTasks(userId int, sessionId string, kind string, status string, startIdx int, num int, upstreamHeaders YucoreMediaUAGProxyHeaders) ([]*YucoreMediaTask, int64, error) {
	config := getYucoreMediaAdapterConfig()
	pageSize := max(startIdx+num, 100)
	if pageSize > 200 {
		pageSize = 200
	}
	localTasks, localErr := ListYucoreMediaTasksWithHeaders(userId, sessionId, kind, status, 0, pageSize, upstreamHeaders)
	if localErr != nil {
		return nil, 0, localErr
	}
	upstreamTasks, _, upstreamErr := ListYucoreUAGProxyMediaTasksWithHeaders(userId, kind, status, 0, pageSize, upstreamHeaders)
	if upstreamErr != nil && len(localTasks) == 0 {
		return nil, 0, upstreamErr
	}

	combined := make([]*YucoreMediaTask, 0, len(localTasks)+len(upstreamTasks))
	seen := map[string]bool{}
	for _, task := range localTasks {
		if hideYucoreMediaTaskInRealAssetMode(task, config) {
			continue
		}
		combined = append(combined, task)
		if task.TaskId != "" {
			seen[task.TaskId] = true
		}
		if externalID := externalYucoreMediaTaskID(task); externalID != "" {
			seen[externalID] = true
		}
	}
	for _, task := range upstreamTasks {
		if task.TaskId == "" || seen[task.TaskId] {
			continue
		}
		if sessionId != "" && task.SessionId != "" && task.SessionId != sessionId {
			continue
		}
		combined = append(combined, task)
		seen[task.TaskId] = true
	}
	sort.SliceStable(combined, func(i, j int) bool {
		if combined[i].UpdatedTime == combined[j].UpdatedTime {
			return combined[i].Id > combined[j].Id
		}
		return combined[i].UpdatedTime > combined[j].UpdatedTime
	})

	total := int64(len(combined))
	if startIdx >= len(combined) {
		return []*YucoreMediaTask{}, total, nil
	}
	end := startIdx + num
	if num <= 0 || end > len(combined) {
		end = len(combined)
	}
	return combined[startIdx:end], total, nil
}

func GetYucoreUAGProxyMediaTaskByTaskIdWithHeaders(taskId string, userId int, upstreamHeaders YucoreMediaUAGProxyHeaders) (*YucoreMediaTask, error) {
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.BaseURL == "" {
		return nil, errors.New("uag-proxy YuCore media adapter is not configured")
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodGet, "/api/v1/gen/tasks/"+url.PathEscape(taskId), nil, upstreamHeaders)
	if err != nil {
		return nil, err
	}
	row := uagProxyDataRow(payload)
	if row != nil && uagProxyTaskID(row) == "" {
		row["task_id"] = taskId
	}
	task, ok := uagProxyHistoryTask(row, userId)
	if !ok {
		return nil, errors.New("YuCore UAG upstream returned no task data")
	}
	return task, nil
}

func externalYucoreMediaTaskID(task *YucoreMediaTask) string {
	metadata := yucoreMediaMetadataMap(task.Metadata)
	return yucoreMediaFirstString(metadata, "upstream_task_id", "external_task_id", "provider_task_id")
}

func applyUAGProxyTaskRow(task *YucoreMediaTask, payload map[string]any, row map[string]any) error {
	if task == nil || row == nil {
		return errors.New("YuCore UAG upstream returned no task data")
	}
	status := yucoreMediaUAGStatus(row["status"])
	externalID := uagProxyTaskID(row)
	if externalID == "" {
		externalID = externalYucoreMediaTaskID(task)
	}
	now := common.GetTimestamp()
	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
		"upstream_task_id": externalID,
		"upstream_status":  yucoreMediaStringValue(row["status"]),
		"last_status_at":   now,
	})
	switch status {
	case YucoreMediaTaskStatusCompleted:
		assets := buildUAGProxyAssets(task, row)
		if len(assets) == 0 {
			return errors.New("YuCore UAG upstream completed without result assets")
		}
		return settleYucoreMediaTaskWithAssets(task, assets)
	case YucoreMediaTaskStatusFailed:
		return failYucoreMediaTask(task, errors.New(uagProxyTaskError(row, payload)))
	case YucoreMediaTaskStatusCanceled:
		task.Status = YucoreMediaTaskStatusCanceled
		task.Progress = 0
	default:
		task.Status = status
		progress := yucoreMediaIntValue(row["progress"])
		if progress <= 0 && status == YucoreMediaTaskStatusProcessing {
			progress = max(task.Progress, 24)
		}
		if progress < task.Progress {
			progress = task.Progress
		}
		if progress > 96 {
			progress = 96
		}
		task.Progress = progress
	}
	task.UpdatedTime = now
	err := DB.Model(task).
		Select("status", "progress", "metadata", "updated_time").
		Updates(task).Error
	if err != nil {
		return err
	}
	notifyYucoreMediaTaskTerminalBackflow(task)
	return nil
}

func runUAGProxyYucoreTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig) error {
	return runUAGProxyYucoreTaskWithHeaders(task, config, nil)
}

func runUAGProxyYucoreTaskWithHeaders(task *YucoreMediaTask, config yucoreMediaAdapterConfig, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	path := "/api/v1/gen/image"
	if task.Kind == "video" {
		path = "/api/v1/gen/video"
	}
	upstreamModelID := yucoreMediaUAGModelID(config, task.ModelId)
	if upstreamModelID != "" && upstreamModelID != task.ModelId {
		task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
			"yucore_model_id":   task.ModelId,
			"upstream_model_id": upstreamModelID,
		})
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodPost, path, buildUAGProxyCreatePayload(task, config), upstreamHeaders)
	if err != nil {
		return err
	}
	row := uagProxyDataRow(payload)
	if err := applyUAGProxyTaskRow(task, payload, row); err != nil {
		return err
	}
	if task.Status != YucoreMediaTaskStatusCompleted && externalYucoreMediaTaskID(task) == "" {
		return errors.New("YuCore UAG upstream returned no task id")
	}
	return nil
}

func refreshUAGProxyYucoreTask(task *YucoreMediaTask) error {
	return refreshUAGProxyYucoreTaskWithHeaders(task, nil)
}

func refreshUAGProxyYucoreTaskWithHeaders(task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	externalID := externalYucoreMediaTaskID(task)
	if externalID == "" {
		return nil
	}
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.BaseURL == "" {
		return errors.New("uag-proxy YuCore media adapter is not configured")
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodGet, "/api/v1/gen/tasks/"+url.PathEscape(externalID), nil, upstreamHeaders)
	if err != nil {
		return err
	}
	return applyUAGProxyTaskRow(task, payload, uagProxyDataRow(payload))
}

func cancelUAGProxyYucoreTask(task *YucoreMediaTask) error {
	return cancelUAGProxyYucoreTaskWithHeaders(task, nil)
}

func cancelUAGProxyYucoreTaskWithHeaders(task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	externalID := externalYucoreMediaTaskID(task)
	if externalID == "" {
		externalID = task.TaskId
	}
	if externalID == "" {
		return nil
	}
	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy || config.BaseURL == "" {
		return errors.New("uag-proxy YuCore media adapter is not configured")
	}
	payload, _, _, err := requestUAGProxyJSONWithHeaders(config, http.MethodPost, "/api/v1/gen/tasks/"+url.PathEscape(externalID)+"/cancel", nil, upstreamHeaders)
	if err != nil {
		return err
	}
	row := uagProxyDataRow(payload)
	if row == nil {
		if task.Id == 0 {
			task.Status = YucoreMediaTaskStatusCanceled
			task.Progress = 0
			task.UpdatedTime = common.GetTimestamp()
		}
		return nil
	}
	if row != nil && uagProxyTaskID(row) == "" {
		row["task_id"] = externalID
	}
	if task.Id == 0 {
		next, ok := uagProxyHistoryTask(row, task.UserId)
		if !ok {
			task.Status = YucoreMediaTaskStatusCanceled
			task.Progress = 0
			task.UpdatedTime = common.GetTimestamp()
			return nil
		}
		*task = *next
		return nil
	}
	return applyUAGProxyTaskRow(task, payload, row)
}

func appendOpenAICompatibleImageAsset(assets []YucoreMediaAsset, task *YucoreMediaTask, index int, urlValue string, b64Value string, revisedPrompt string) []YucoreMediaAsset {
	mimeType := yucoreMediaImageMimeType(task.Format)
	assetURL := strings.TrimSpace(urlValue)
	if assetURL == "" && b64Value != "" {
		if _, err := base64.StdEncoding.DecodeString(b64Value); err == nil {
			assetURL = fmt.Sprintf("data:%s;base64,%s", mimeType, b64Value)
		}
	}
	if assetURL == "" {
		return assets
	}
	label := fmt.Sprintf("%s result %d", task.ModelId, index+1)
	if revisedPrompt != "" {
		label = fmt.Sprintf("%s · result %d", task.ModelId, index+1)
	}
	return append(assets, YucoreMediaAsset{
		Id:        fmt.Sprintf("%s_asset_%d", task.TaskId, index),
		Kind:      task.Kind,
		Url:       fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
		ThumbUrl:  fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
		SourceUrl: assetURL,
		Label:     label,
		Width:     1024,
		Height:    1024,
		MimeType:  mimeType,
		Metadata: map[string]any{
			"adapter": "openai-compatible",
		},
	})
}

func runOpenAICompatibleYucoreImageTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig, capability YucoreMediaModelCapability) error {
	endpoint, err := yucoreMediaOpenAIURL(config.BaseURL, capability.CreatePath)
	if err != nil {
		return err
	}

	body := map[string]any{
		"model":  yucoreMediaCapabilityModel(task, capability),
		"prompt": task.Prompt,
	}
	if yucoreMediaCapabilityAllowsParameter(capability, "n") {
		body["n"] = task.Count
	}
	if task.NegativePrompt != "" && yucoreMediaCapabilityAllowsParameter(capability, "negative_prompt") {
		body["negative_prompt"] = task.NegativePrompt
	}
	if size := normalizeYucoreMediaImageSize(task.Size); size != "" && yucoreMediaCapabilityAllowsParameter(capability, "size") {
		body["size"] = size
	}
	if task.Quality != "" && !strings.EqualFold(task.Quality, "auto") && yucoreMediaCapabilityAllowsParameter(capability, "quality") {
		body["quality"] = task.Quality
	}
	if task.Format != "" && !strings.EqualFold(task.Format, "url") && !strings.EqualFold(task.Format, "b64_json") && yucoreMediaCapabilityAllowsParameter(capability, "output_format") {
		body["output_format"] = strings.TrimPrefix(task.Format, "image/")
	}
	if task.AspectRatio != "" && !strings.EqualFold(task.AspectRatio, "auto") && yucoreMediaCapabilityAllowsParameter(capability, "aspect_ratio") {
		body["aspect_ratio"] = task.AspectRatio
	}
	responseFormat := strings.TrimSpace(capability.ResponseFormat)
	if responseFormat == "" {
		responseFormat = strings.TrimSpace(common.GetEnvOrDefaultString("YUCORE_MEDIA_RESPONSE_FORMAT", ""))
	}
	if responseFormat != "" && yucoreMediaCapabilityAllowsParameter(capability, "response_format") {
		body["response_format"] = responseFormat
	}

	metadata := yucoreMediaMetadataMap(task.Metadata)
	for _, key := range []string{"background", "moderation", "style_preset", "stream_mode", "partial_images"} {
		if value, ok := metadata[key]; ok && yucoreMediaCapabilityAllowsParameter(capability, key) {
			body[key] = value
		}
	}

	rawBody, err := common.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if header := strings.TrimSpace(common.GetEnvOrDefaultString("YUCORE_MEDIA_ORG_ID", "")); header != "" {
		req.Header.Set("OpenAI-Organization", header)
	}

	client := &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("YuCore media upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload openAICompatibleImageResponse
	if err := common.Unmarshal(respBody, &payload); err != nil {
		return err
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return errors.New(payload.Error.Message)
	}
	assets := make([]YucoreMediaAsset, 0, max(task.Count, 1))
	for index, item := range payload.Data {
		assets = appendOpenAICompatibleImageAsset(assets, task, index, item.URL, item.B64JSON, item.RevisedPrompt)
	}
	if len(assets) == 0 {
		for index, item := range payload.Images {
			assets = appendOpenAICompatibleImageAsset(assets, task, index, item.URL, item.B64JSON, item.RevisedPrompt)
		}
	}
	if len(assets) == 0 {
		return errors.New("YuCore media upstream returned no image assets")
	}
	return settleYucoreMediaTaskWithAssets(task, assets)
}

func RunYucoreMediaTask(taskId int) {
	RunYucoreMediaTaskWithHeaders(taskId, nil)
}

func isYucoreMediaRunnableAdapter(adapter string) bool {
	switch adapter {
	case YucoreMediaAdapterOpenAICompatible, YucoreMediaAdapterYuAPIChannel, YucoreMediaAdapterUAGProxy:
		return true
	default:
		return false
	}
}

func RunYucoreMediaTaskWithHeaders(taskId int, upstreamHeaders YucoreMediaUAGProxyHeaders) {
	var task YucoreMediaTask
	if err := DB.Where("id = ?", taskId).First(&task).Error; err != nil {
		common.SysError("failed to load YuCore media task: " + err.Error())
		return
	}
	if task.Status != YucoreMediaTaskStatusProcessing {
		return
	}
	config := getYucoreMediaAdapterConfig()
	adapter := yucoreMediaTaskAdapter(&task)
	if !isYucoreMediaRunnableAdapter(adapter) {
		return
	}
	task.Progress = 24
	task.UpdatedTime = common.GetTimestamp()
	_ = DB.Model(&task).Select("progress", "updated_time").Updates(&task).Error
	var err error
	switch adapter {
	case YucoreMediaAdapterOpenAICompatible:
		err = runOpenAICompatibleYucoreTask(&task, config)
	case YucoreMediaAdapterYuAPIChannel:
		config, err = yucoreMediaOpenAIConfigForTask(&task, config)
		if err == nil {
			err = runOpenAICompatibleYucoreTask(&task, config)
		}
	case YucoreMediaAdapterUAGProxy:
		err = runUAGProxyYucoreTaskWithHeaders(&task, config, upstreamHeaders)
	}
	if err != nil {
		common.SysError("YuCore media task failed: " + err.Error())
		_ = failYucoreMediaTask(&task, err)
	}
}

func CreateYucoreMediaTask(task *YucoreMediaTask) error {
	return CreateYucoreMediaTaskWithHeaders(task, nil)
}

func CreateYucoreMediaTaskWithHeaders(task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	normalizeYucoreMediaTask(task)
	adapter, err := resolveYucoreMediaTaskAdapter(task)
	if err != nil {
		return err
	}
	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
		"adapter":             adapter,
		"adapter_configured":  adapter != YucoreMediaAdapterMock,
		"adapter_created_at":  common.GetTimestamp(),
		"real_assets_enabled": adapter != YucoreMediaAdapterMock,
	})
	now := common.GetTimestamp()
	task.TaskId = GenerateYucoreMediaTaskID()
	task.CreatedTime = now
	task.UpdatedTime = now
	task.Status = YucoreMediaTaskStatusProcessing
	task.Progress = 8
	if err := DB.Create(task).Error; err != nil {
		return err
	}
	if isYucoreMediaRunnableAdapter(adapter) {
		go RunYucoreMediaTaskWithHeaders(task.Id, cloneYucoreMediaUAGProxyHeaders(upstreamHeaders))
	}
	return nil
}

func CountYucoreMediaTasks(userId int, sessionId string, kind string, status string) (int64, error) {
	var total int64
	query := DB.Model(&YucoreMediaTask{}).Where("user_id = ?", userId)
	if sessionId != "" {
		query = query.Where("session_id = ?", sessionId)
	}
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&total).Error
	return total, err
}

func ListYucoreMediaTasks(userId int, sessionId string, kind string, status string, startIdx int, num int) ([]*YucoreMediaTask, error) {
	return ListYucoreMediaTasksWithHeaders(userId, sessionId, kind, status, startIdx, num, nil)
}

func ListYucoreMediaTasksWithHeaders(userId int, sessionId string, kind string, status string, startIdx int, num int, upstreamHeaders YucoreMediaUAGProxyHeaders) ([]*YucoreMediaTask, error) {
	var tasks []*YucoreMediaTask
	query := DB.Where("user_id = ?", userId)
	if sessionId != "" {
		query = query.Where("session_id = ?", sessionId)
	}
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("updated_time desc, id desc").
		Limit(num).
		Offset(startIdx).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if _, err := HydrateYucoreMediaTaskWithHeaders(task, upstreamHeaders); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func GetYucoreMediaTaskByTaskId(taskId string, userId int) (*YucoreMediaTask, error) {
	return GetYucoreMediaTaskByTaskIdWithHeaders(taskId, userId, nil)
}

func GetYucoreMediaTaskByTaskIdWithHeaders(taskId string, userId int, upstreamHeaders YucoreMediaUAGProxyHeaders) (*YucoreMediaTask, error) {
	var task YucoreMediaTask
	err := DB.Where("task_id = ? and user_id = ?", taskId, userId).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) && IsYucoreMediaUAGProxyConfigured() {
			return GetYucoreUAGProxyMediaTaskByTaskIdWithHeaders(taskId, userId, upstreamHeaders)
		}
		return nil, err
	}
	return HydrateYucoreMediaTaskWithHeaders(&task, upstreamHeaders)
}

func DeleteYucoreMediaTaskByTaskId(taskId string, userId int) error {
	return DB.Where("task_id = ? and user_id = ?", taskId, userId).Delete(&YucoreMediaTask{}).Error
}

func CancelYucoreMediaTask(task *YucoreMediaTask) error {
	return CancelYucoreMediaTaskWithHeaders(task, nil)
}

func CancelYucoreMediaTaskWithHeaders(task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	if task == nil {
		return nil
	}
	if task.Status == YucoreMediaTaskStatusCompleted {
		return nil
	}
	if yucoreMediaTaskAdapter(task) == YucoreMediaAdapterUAGProxy {
		if err := cancelUAGProxyYucoreTaskWithHeaders(task, upstreamHeaders); err != nil {
			return err
		}
		if task.Id == 0 {
			return nil
		}
		if task.Status == YucoreMediaTaskStatusCompleted || task.Status == YucoreMediaTaskStatusFailed {
			return nil
		}
	}
	task.Status = YucoreMediaTaskStatusCanceled
	task.Progress = 0
	task.UpdatedTime = common.GetTimestamp()
	return DB.Model(task).Select("status", "progress", "updated_time").Updates(task).Error
}
