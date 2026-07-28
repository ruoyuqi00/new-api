package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const (
	grokUsageProbeModel   = "grok-4.5"
	grokUsageProbeTimeout = 20 * time.Second
	grokBillingTimeout    = 15 * time.Second
)

type grokOAuthUsageCredential struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	PlanType     string `json:"plan_type,omitempty"`
}

type grokUsageWindow struct {
	Limit     *int64 `json:"limit,omitempty"`
	Remaining *int64 `json:"remaining,omitempty"`
	ResetUnix *int64 `json:"reset_unix,omitempty"`
	ResetAt   string `json:"reset_at,omitempty"`
}

type grokUsageSnapshot struct {
	Requests          *grokUsageWindow `json:"requests,omitempty"`
	Tokens            *grokUsageWindow `json:"tokens,omitempty"`
	RetryAfterSeconds *int             `json:"retry_after_seconds,omitempty"`
	SubscriptionTier  string           `json:"subscription_tier,omitempty"`
	EntitlementStatus string           `json:"entitlement_status,omitempty"`
	StatusCode        int              `json:"status_code"`
	HeadersObserved   bool             `json:"headers_observed"`
	LastProbeAt       string           `json:"last_probe_at,omitempty"`
	UpdatedAt         string           `json:"updated_at"`
}

func RefreshGrokProviderAccountUsage(ctx context.Context, account *model.ProviderAccountSummary) (ProviderAccountUsageRefreshResult, error) {
	result := ProviderAccountUsageRefreshResult{
		AccountId: account.Id, AccountName: account.Name, Supported: true, CheckedAt: common.GetTimestamp(),
	}

	var credential grokOAuthUsageCredential
	if err := common.Unmarshal([]byte(strings.TrimSpace(account.Credential)), &credential); err != nil || strings.TrimSpace(credential.AccessToken) == "" {
		result.ErrorCode = "invalid_credential"
		result.Message = "invalid Grok OAuth credential"
		return result, persistProviderAccountUsageResult(result, "")
	}

	client, err := GetHttpClientWithProxy("")
	if err != nil {
		result.ErrorCode = "http_client_error"
		result.Message = err.Error()
		return result, persistProviderAccountUsageResult(result, "")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
	if baseURL == "" || strings.EqualFold(baseURL, "https://api.x.ai") || strings.EqualFold(baseURL, "https://api.x.ai/v1") {
		baseURL = constant.GrokOAuthBaseURL
	}

	probeCtx, cancel := context.WithTimeout(ctx, grokUsageProbeTimeout)
	statusCode, body, headers, fetchErr := fetchGrokUsageProbe(probeCtx, client, baseURL, credential.AccessToken)
	cancel()

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(credential.RefreshToken) != "" {
		refreshCtx, refreshCancel := context.WithTimeout(ctx, 10*time.Second)
		refreshed, refreshedExpiresAt, refreshErr := RefreshGrokOAuthCredential(refreshCtx, account.Credential)
		refreshCancel()
		if refreshErr == nil {
			var refreshedCredential grokOAuthUsageCredential
			if decodeErr := common.Unmarshal([]byte(refreshed), &refreshedCredential); decodeErr == nil && strings.TrimSpace(refreshedCredential.AccessToken) != "" {
				if updateErr := model.UpdateProviderAccountCredential(account.Id, refreshed, refreshedExpiresAt); updateErr == nil {
					model.InitAccountPoolCache()
					credential = refreshedCredential
					result.TokenRefreshed = true
					retryCtx, retryCancel := context.WithTimeout(ctx, grokUsageProbeTimeout)
					statusCode, body, headers, fetchErr = fetchGrokUsageProbe(retryCtx, client, baseURL, credential.AccessToken)
					retryCancel()
				}
			}
		}
	}

	result.UpstreamStatus = statusCode
	if fetchErr != nil {
		result.ErrorCode = "usage_request_failed"
		result.Message = "Grok probe request failed: " + fetchErr.Error()
		return result, persistProviderAccountUsageResult(result, "")
	}

	var probePayload any
	if len(body) > 0 && common.Unmarshal(body, &probePayload) != nil {
		probePayload = map[string]any{"raw": string(body)}
	}
	snapshot := buildGrokUsageSnapshot(headers, statusCode)
	data := map[string]any{
		"plan_type":   strings.TrimSpace(credential.PlanType),
		"probe_model": grokUsageProbeModel,
		"probe":       probePayload,
		"rate_limit":  snapshot,
	}
	billing, billingStatus := fetchGrokBilling(ctx, client, baseURL, credential.AccessToken)
	if billing != nil {
		billing["status_code"] = billingStatus
		data["billing"] = billing
		if planType := grokPlanTypeFromBilling(billing); planType != "" {
			data["plan_type"] = planType
		}
	} else if billingStatus != 0 {
		data["billing"] = map[string]any{"status_code": billingStatus}
	}
	encoded, encodeErr := common.Marshal(data)
	if encodeErr != nil {
		return result, encodeErr
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		result.ErrorCode, result.Message = providerAccountUpstreamError(probePayload, statusCode)
		return result, persistProviderUsageSnapshot(result, string(encoded))
	}
	result.Success = true
	result.Data = data
	return result, persistProviderUsageSnapshot(result, string(encoded))
}

// ProbeGrokOAuthCredential verifies a Grok OAuth credential with the same
// minimal Responses request used by account health and quota refresh.
func ProbeGrokOAuthCredential(ctx context.Context, baseURL string, rawCredential string) error {
	var credential grokOAuthUsageCredential
	if err := common.Unmarshal([]byte(strings.TrimSpace(rawCredential)), &credential); err != nil || strings.TrimSpace(credential.AccessToken) == "" {
		return fmt.Errorf("invalid Grok OAuth credential")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.EqualFold(baseURL, "https://api.x.ai") || strings.EqualFold(baseURL, "https://api.x.ai/v1") {
		baseURL = constant.GrokOAuthBaseURL
	}
	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, grokUsageProbeTimeout)
	defer cancel()
	statusCode, _, _, err := fetchGrokUsageProbe(probeCtx, client, baseURL, credential.AccessToken)
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Grok credential probe returned HTTP %d", statusCode)
	}
	return nil
}

func fetchGrokUsageProbe(ctx context.Context, client *http.Client, baseURL string, accessToken string) (int, []byte, http.Header, error) {
	body, err := common.Marshal(map[string]any{
		"model":             grokUsageProbeModel,
		"input":             ".",
		"max_output_tokens": 1,
		"store":             false,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/responses", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, nil, err
	}
	applyGrokCLIHeaders(&req.Header, accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, responseBody, resp.Header.Clone(), readErr
}

func fetchGrokBilling(ctx context.Context, client *http.Client, baseURL string, accessToken string) (map[string]any, int) {
	baseURL = strings.TrimRight(baseURL, "/")
	lastStatus := 0
	// Billing is exposed by the CLI proxy under the same /v1 base path.
	for _, suffix := range []string{"/billing?format=credits", "/billing"} {
		requestCtx, cancel := context.WithTimeout(ctx, grokBillingTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+suffix, nil)
		if err != nil {
			cancel()
			continue
		}
		applyGrokCLIHeaders(&req.Header, accessToken)
		resp, err := client.Do(req)
		if err == nil {
			responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			status := resp.StatusCode
			_ = resp.Body.Close()
			cancel()
			if readErr == nil && status >= 200 && status < 300 {
				var payload map[string]any
				if common.Unmarshal(responseBody, &payload) == nil {
					return payload, status
				}
			}
			if status == http.StatusForbidden || lastStatus == 0 {
				lastStatus = status
			}
			continue
		}
		cancel()
	}
	return nil, lastStatus
}

func applyGrokCLIHeaders(header *http.Header, accessToken string) {
	if header == nil {
		return
	}
	header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	header.Set("Accept", "application/json")
	header.Set("Content-Type", "application/json")
	header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	header.Set("x-grok-client-version", constant.GrokCLIClientVersion)
	header.Set("User-Agent", constant.GrokCLIUserAgent)
}

func buildGrokUsageSnapshot(headers http.Header, statusCode int) grokUsageSnapshot {
	now := time.Now().UTC().Format(time.RFC3339)
	snapshot := grokUsageSnapshot{StatusCode: statusCode, UpdatedAt: now, LastProbeAt: now}
	snapshot.Requests = buildGrokUsageWindow(headers, "requests")
	snapshot.Tokens = buildGrokUsageWindow(headers, "tokens")
	if raw := strings.TrimSpace(headers.Get("retry-after")); raw != "" {
		var value int
		if _, err := fmt.Sscan(raw, &value); err == nil {
			snapshot.RetryAfterSeconds = &value
		}
	}
	snapshot.SubscriptionTier = firstGrokHeader(headers, "xai-subscription-tier", "x-subscription-tier")
	snapshot.EntitlementStatus = firstGrokHeader(headers, "xai-entitlement-status", "x-entitlement-status")
	snapshot.HeadersObserved = snapshot.Requests != nil || snapshot.Tokens != nil || snapshot.RetryAfterSeconds != nil || snapshot.SubscriptionTier != "" || snapshot.EntitlementStatus != ""
	return snapshot
}

func buildGrokUsageWindow(headers http.Header, dimension string) *grokUsageWindow {
	window := &grokUsageWindow{}
	if value := strings.TrimSpace(headers.Get("x-ratelimit-limit-" + dimension)); value != "" {
		var parsed int64
		if _, err := fmt.Sscan(value, &parsed); err == nil {
			window.Limit = &parsed
		}
	}
	if value := strings.TrimSpace(headers.Get("x-ratelimit-remaining-" + dimension)); value != "" {
		var parsed int64
		if _, err := fmt.Sscan(value, &parsed); err == nil {
			window.Remaining = &parsed
		}
	}
	if window.Limit == nil && window.Remaining == nil {
		return nil
	}
	return window
}

func firstGrokHeader(headers http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(headers.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func grokPlanTypeFromBilling(payload map[string]any) string {
	config, _ := payload["config"].(map[string]any)
	if config == nil {
		return ""
	}
	if value, ok := config["plan"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value, ok := config["planType"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func persistProviderUsageSnapshot(result ProviderAccountUsageRefreshResult, snapshot string) error {
	return model.UpdateProviderAccountUsageHealth(result.AccountId, snapshot, result.CheckedAt, result.UpstreamStatus, result.ErrorCode, result.Message)
}
