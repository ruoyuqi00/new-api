package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	cfg := loadKiroAdapterConfigFromEnv()
	if err := validateKiroAdapterConfig(cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := fetchProviderAdapterAdminJSON(c.Request.Context(), providerAdapterAdminFetchConfig{
		Provider:      "kiro",
		Endpoint:      "credentials",
		BaseURL:       cfg.InternalBaseURL,
		APIKey:        cfg.AdminAPIKey,
		Path:          "/api/admin/credentials",
		Timeout:       cfg.Timeout,
		Sanitize:      sanitizeKiroAdapterResponse,
		SensitiveKeys: kiroSensitiveKeys,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
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
