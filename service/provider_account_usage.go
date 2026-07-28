package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type providerAccountCodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Email        string `json:"email,omitempty"`
	Type         string `json:"type,omitempty"`
	Expired      string `json:"expired,omitempty"`
}

type ProviderAccountUsageRefreshResult struct {
	AccountId      int    `json:"account_id"`
	AccountName    string `json:"account_name"`
	Success        bool   `json:"success"`
	Supported      bool   `json:"supported"`
	Message        string `json:"message,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	UpstreamStatus int    `json:"upstream_status"`
	TokenRefreshed bool   `json:"token_refreshed"`
	CheckedAt      int64  `json:"checked_at"`
	Data           any    `json:"data,omitempty"`
}

func RefreshProviderAccountUsage(ctx context.Context, account *model.ProviderAccountSummary) (ProviderAccountUsageRefreshResult, error) {
	if account == nil {
		return ProviderAccountUsageRefreshResult{}, fmt.Errorf("provider account is required")
	}
	result := ProviderAccountUsageRefreshResult{
		AccountId: account.Id, AccountName: account.Name, CheckedAt: common.GetTimestamp(),
	}
	if account.PoolAdapterType == constant.ChannelTypeXai && account.Type == "oauth_json" {
		return RefreshGrokProviderAccountUsage(ctx, account)
	}
	if account.PoolAdapterType != constant.ChannelTypeCodex || account.Type != "oauth_json" {
		result.Message = "provider account usage refresh is not supported for this account type"
		return result, nil
	}
	result.Supported = true

	var oauthKey providerAccountCodexOAuthKey
	err := common.Unmarshal([]byte(strings.TrimSpace(account.Credential)), &oauthKey)
	if err != nil || strings.TrimSpace(oauthKey.AccessToken) == "" || strings.TrimSpace(oauthKey.AccountID) == "" {
		result.ErrorCode = "invalid_credential"
		result.Message = "invalid Codex OAuth credential"
		return result, persistProviderAccountUsageResult(result, "")
	}
	client, err := GetHttpClientWithProxy("")
	if err != nil {
		result.ErrorCode = "http_client_error"
		result.Message = err.Error()
		return result, persistProviderAccountUsageResult(result, "")
	}
	baseURL := strings.TrimSpace(account.BaseURL)
	if baseURL == "" {
		baseURL = "https://chatgpt.com"
	}

	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	statusCode, body, fetchErr := FetchCodexWhamUsage(requestCtx, client, baseURL, oauthKey.AccessToken, oauthKey.AccountID)
	cancel()
	if fetchErr != nil {
		result.ErrorCode = "usage_request_failed"
		result.Message = "usage request failed: " + fetchErr.Error()
		return result, persistProviderAccountUsageResult(result, "")
	}

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(oauthKey.RefreshToken) != "" {
		refreshCtx, refreshCancel := context.WithTimeout(ctx, 10*time.Second)
		refreshed, refreshErr := RefreshCodexOAuthToken(refreshCtx, oauthKey.RefreshToken)
		refreshCancel()
		if refreshErr != nil {
			result.UpstreamStatus = statusCode
			result.ErrorCode = "oauth_refresh_failed"
			result.Message = fmt.Sprintf("OAuth refresh failed after HTTP %d: %v", statusCode, refreshErr)
			return result, persistProviderAccountUsageResult(result, "")
		}
		oauthKey.AccessToken = refreshed.AccessToken
		if strings.TrimSpace(refreshed.RefreshToken) != "" {
			oauthKey.RefreshToken = refreshed.RefreshToken
		}
		oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
		oauthKey.Expired = refreshed.ExpiresAt.Format(time.RFC3339)
		encoded, marshalErr := common.Marshal(oauthKey)
		if marshalErr != nil {
			return result, fmt.Errorf("encode refreshed provider account credential: %w", marshalErr)
		}
		if updateErr := model.UpdateProviderAccountCredential(account.Id, string(encoded), refreshed.ExpiresAt.Unix()); updateErr != nil {
			return result, fmt.Errorf("save refreshed provider account credential: %w", updateErr)
		}
		model.InitAccountPoolCache()
		result.TokenRefreshed = true

		retryCtx, retryCancel := context.WithTimeout(ctx, 15*time.Second)
		statusCode, body, fetchErr = FetchCodexWhamUsage(retryCtx, client, baseURL, oauthKey.AccessToken, oauthKey.AccountID)
		retryCancel()
		if fetchErr != nil {
			result.ErrorCode = "usage_request_failed"
			result.Message = "usage request failed after OAuth refresh: " + fetchErr.Error()
			return result, persistProviderAccountUsageResult(result, "")
		}
	}

	result.UpstreamStatus = statusCode
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		result.ErrorCode = "invalid_usage_response"
		result.Message = fmt.Sprintf("upstream returned invalid JSON (HTTP %d)", statusCode)
		return result, persistProviderAccountUsageResult(result, "")
	}
	result.Data = payload
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		result.ErrorCode, result.Message = providerAccountUpstreamError(payload, statusCode)
		return result, persistProviderAccountUsageResult(result, "")
	}

	result.Success = true
	return result, persistProviderAccountUsageResult(result, string(body))
}

func persistProviderAccountUsageResult(result ProviderAccountUsageRefreshResult, snapshot string) error {
	return model.UpdateProviderAccountUsageHealth(
		result.AccountId,
		snapshot,
		result.CheckedAt,
		result.UpstreamStatus,
		result.ErrorCode,
		result.Message,
	)
}

func providerAccountUpstreamError(payload any, statusCode int) (string, string) {
	errorCode := fmt.Sprintf("http_%d", statusCode)
	message := ""
	object, ok := payload.(map[string]interface{})
	if ok {
		if value, exists := object["error"]; exists {
			switch upstreamError := value.(type) {
			case string:
				message = upstreamError
			case map[string]interface{}:
				if value, exists := upstreamError["code"]; exists && fmt.Sprint(value) != "" {
					errorCode = fmt.Sprint(value)
				}
				if value, exists := upstreamError["message"]; exists {
					message = fmt.Sprint(value)
				}
			}
		}
		if message == "" {
			for _, key := range []string{"message", "detail", "error_description"} {
				if value, exists := object[key]; exists && fmt.Sprint(value) != "" {
					message = fmt.Sprint(value)
					break
				}
			}
		}
	}
	if message == "" {
		switch statusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			message = "credential expired, subscription unavailable, or access rejected"
		case http.StatusTooManyRequests:
			message = "account is rate limited by upstream"
		default:
			message = "upstream request failed"
		}
	}
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		message = message[:500]
	}
	return errorCode, fmt.Sprintf("HTTP %d: %s", statusCode, message)
}
