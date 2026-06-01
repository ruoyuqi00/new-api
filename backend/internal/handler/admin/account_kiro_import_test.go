package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseKiroImportCredentials(t *testing.T) {
	req := KiroImportRequest{
		RefreshToken:       " refresh-1 ",
		KiroAPIKey:         " ksk_top_level ",
		RefreshTokens:      []string{"refresh-1", " refresh-2 "},
		KiroAPIKeys:        []string{"ksk_top_level", " ksk-array-1 "},
		Raw:                "refresh-3\n user@example.com----refresh-4 ; api@example.com----ksk_raw_1 ",
		RefreshTokensCamel: []string{" refresh-5 "},
		Accounts: []KiroImportAccount{
			{
				RefreshTokenCamel: " refresh-6 ",
				AuthMethodCamel:   "builder-id",
				ClientIDCamel:     " client-id ",
				ClientSecretCamel: " client-secret ",
				Priority:          7,
				AuthRegionCamel:   " us-east-1 ",
				APIRegionCamel:    " us-east-1 ",
				ProxyURLCamel:     " direct ",
				Endpoint:          " ide ",
			},
			{KiroAPIKeyCamel: " ksk-account-1 ", Email: " key@example.com "},
		},
	}

	credentials, duplicateCount, err := parseKiroImportCredentials(req)
	require.NoError(t, err)
	require.Equal(t, 10, len(credentials))
	require.Equal(t, 2, duplicateCount)
	require.Equal(t, "refresh-1", credentials[0].RefreshToken)
	require.Equal(t, "social", credentials[0].AuthMethod)
	require.Equal(t, "ksk_top_level", credentials[1].KiroAPIKey)
	require.Equal(t, "api_key", credentials[1].AuthMethod)
	require.Equal(t, "refresh-2", credentials[2].RefreshToken)
	require.Equal(t, "refresh-5", credentials[3].RefreshToken)
	require.Equal(t, "ksk-array-1", credentials[4].KiroAPIKey)
	require.Equal(t, "refresh-6", credentials[5].RefreshToken)
	require.Equal(t, "idc", credentials[5].AuthMethod)
	require.Equal(t, "client-id", credentials[5].ClientID)
	require.Equal(t, "client-secret", credentials[5].ClientSecret)
	require.Equal(t, uint32(7), credentials[5].Priority)
	require.Equal(t, "direct", credentials[5].ProxyURL)
	require.Equal(t, "ide", credentials[5].Endpoint)
	require.Equal(t, "ksk-account-1", credentials[6].KiroAPIKey)
	require.Equal(t, "key@example.com", credentials[6].Email)
	require.Equal(t, "refresh-3", credentials[7].RefreshToken)
	require.Equal(t, "refresh-4", credentials[8].RefreshToken)
	require.Equal(t, "user@example.com", credentials[8].Email)
	require.Equal(t, "ksk_raw_1", credentials[9].KiroAPIKey)
	require.Equal(t, "api@example.com", credentials[9].Email)
}

func TestParseKiroImportCredentialsSupportsExportedCredentialJSON(t *testing.T) {
	var req KiroImportRequest
	err := json.Unmarshal([]byte(`{
		"accounts": [
			{
				"email": "user@example.com",
				"refresh_token": "fallback-refresh",
				"access_token": "fallback-access",
				"expires_at": 1778755870,
				"kiro_auth_token_raw": {
					"accessToken": "access-from-export",
					"refreshToken": "refresh-from-export",
					"profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/ABCDEF",
					"expiresAt": "2026-05-14T10:51:10.810569+00:00",
					"loginHint": "login@example.com"
				}
			}
		]
	}`), &req)
	require.NoError(t, err)

	credentials, duplicateCount, err := parseKiroImportCredentials(req)
	require.NoError(t, err)
	require.Zero(t, duplicateCount)
	require.Len(t, credentials, 1)
	require.Equal(t, "fallback-access", credentials[0].AccessToken)
	require.Equal(t, "fallback-refresh", credentials[0].RefreshToken)
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123456789012:profile/ABCDEF", credentials[0].ProfileARN)
	require.Equal(t, "2026-05-14T10:51:10Z", credentials[0].ExpiresAt)
	require.Equal(t, "user@example.com", credentials[0].Email)
	require.Equal(t, "social", credentials[0].AuthMethod)
}

func TestParseKiroImportCredentialsRejectsMixedSecretKinds(t *testing.T) {
	_, _, err := parseKiroImportCredentials(KiroImportRequest{
		Accounts: []KiroImportAccount{{RefreshToken: "refresh-1", KiroAPIKey: "ksk_1"}},
	})
	require.ErrorContains(t, err, "只能提供 refreshToken")
}

func TestParseKiroImportCredentialsRejectsIncompleteIDC(t *testing.T) {
	_, _, err := parseKiroImportCredentials(KiroImportRequest{
		Accounts: []KiroImportAccount{{RefreshToken: "refresh-1", AuthMethod: "idc", ClientID: "client-id"}},
	})
	require.ErrorContains(t, err, "clientId 和 clientSecret")
}

func TestKiroImportIdempotencyPayloadDoesNotContainRawSecrets(t *testing.T) {
	credentials, duplicateCount, err := parseKiroImportCredentials(KiroImportRequest{
		RefreshTokens: []string{"raw-refresh-token-secret"},
		Accounts: []KiroImportAccount{
			{KiroAPIKey: "raw-kiro-api-key-secret", Email: "private@example.com"},
			{RefreshToken: "raw-idc-token-secret", AuthMethod: "idc", ClientID: "client-id", ClientSecret: "raw-client-secret"},
		},
	})
	require.NoError(t, err)

	payload := buildKiroImportIdempotencyPayload(credentials, duplicateCount, []int64{12, 10, 12})
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "raw-refresh-token-secret")
	require.NotContains(t, string(raw), "raw-kiro-api-key-secret")
	require.NotContains(t, string(raw), "raw-idc-token-secret")
	require.NotContains(t, string(raw), "raw-client-secret")
	require.NotContains(t, string(raw), "private@example.com")
	require.Contains(t, string(raw), hashKiroSecret("raw-refresh-token-secret"))
	require.Contains(t, string(raw), hashKiroSecret("raw-kiro-api-key-secret"))
	require.Contains(t, string(raw), hashKiroSecret("raw-idc-token-secret"))
	require.Contains(t, string(raw), hashKiroSecret("private@example.com"))
	require.Contains(t, string(raw), `"group_ids":[12,10,12]`)
}

func TestImportKiroCredentialsForwardsToAdapterAndRedactsResponse(t *testing.T) {
	var gotAuth string
	var gotXAPIKey string
	var gotPaths []string
	var gotBodies []map[string]any
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		gotPaths = append(gotPaths, r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		gotXAPIKey = r.Header.Get("x-api-key")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotBodies = append(gotBodies, body)
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 2 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"duplicate","refreshToken":"should-not-return"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"credentialId":12,"kiroApiKey":"should-not-return","nested":{"clientSecret":"secret","ok":true}}`))
	}))
	defer server.Close()

	result, err := importKiroCredentials(context.Background(), kiroAdapterConfig{
		InternalBaseURL: server.URL + "/",
		AdminAPIKey:     "admin-key",
		Timeout:         time.Second,
	}, []kiroForwardCredential{
		{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ProfileARN:   "arn:aws:codewhisperer:us-east-1:123456789012:profile/ABCDEF",
			ExpiresAt:    "2026-05-14T10:51:10.810569+00:00",
			AuthMethod:   "social",
			Email:        "one@example.com",
		},
		{KiroAPIKey: "ksk_1", AuthMethod: "api_key"},
	}, 0)
	require.NoError(t, err)

	require.Equal(t, []string{"/api/admin/credentials", "/api/admin/credentials"}, gotPaths)
	require.Equal(t, "Bearer admin-key", gotAuth)
	require.Equal(t, "admin-key", gotXAPIKey)
	require.Len(t, gotBodies, 2)
	require.Equal(t, "refresh-1", gotBodies[0]["refreshToken"])
	require.Equal(t, "access-1", gotBodies[0]["accessToken"])
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123456789012:profile/ABCDEF", gotBodies[0]["profileArn"])
	require.Equal(t, "2026-05-14T10:51:10.810569Z", gotBodies[0]["expiresAt"])
	require.Equal(t, "social", gotBodies[0]["authMethod"])
	require.Equal(t, "one@example.com", gotBodies[0]["email"])
	require.Equal(t, "ksk_1", gotBodies[1]["kiroApiKey"])
	require.Equal(t, "api_key", gotBodies[1]["authMethod"])
	require.Equal(t, 2, result.Total)
	require.Equal(t, 1, result.Succeeded)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.Items[0].Success)
	require.False(t, result.Items[1].Success)

	upstream, ok := result.Items[0].Upstream.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", upstream["kiroApiKey"])
	nested, ok := upstream["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", nested["clientSecret"])
	require.Equal(t, true, nested["ok"])
	failedUpstream, ok := result.Items[1].Upstream.(map[string]any)
	require.True(t, ok)
	errObj, ok := failedUpstream["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", errObj["refreshToken"])
}

func TestImportKiroCredentialsRequiresAdapterConfig(t *testing.T) {
	old := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	defer service.SetDefaultIdempotencyCoordinator(old)

	t.Setenv(envKiroAdapterBaseURL, "")
	t.Setenv(envKiroAdapterAdminAPIKey, "")
	t.Setenv(envKiroAdapterInternalAPIKey, "")
	t.Setenv(envProviderAdaptersKiroBaseURL, "")
	t.Setenv(envProviderAdaptersKiroAPIKey, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &AccountHandler{}
	router.POST("/api/v1/admin/accounts/import/kiro", handler.ImportKiroCredentials)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/import/kiro", strings.NewReader(`{"refresh_token":"refresh-1"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "refresh-1")
	require.Contains(t, recorder.Body.String(), "KIRO_ADAPTER_NOT_CONFIGURED")
}

func TestEnsureKiroRuntimeUpstreamAccountCreatesAndBindsGroups(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = nil
	handler := &AccountHandler{adminService: adminSvc}

	result, err := handler.ensureKiroRuntimeUpstreamAccount(context.Background(), kiroAdapterConfig{
		InternalBaseURL: "http://kiro-rs:8990",
		InternalAPIKey:  "runtime-key",
	}, []int64{20, 10, 20, 0})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "created", result.Action)
	require.Equal(t, kiroRuntimeDefaultBaseURL, result.BaseURL)
	require.Equal(t, []int64{10, 20}, result.GroupIDs)

	require.Len(t, adminSvc.createdAccounts, 1)
	input := adminSvc.createdAccounts[0]
	require.Equal(t, kiroRuntimeUpstreamAccountName, input.Name)
	require.Equal(t, service.PlatformAnthropic, input.Platform)
	require.Equal(t, service.AccountTypeAPIKey, input.Type)
	require.Equal(t, []int64{10, 20}, input.GroupIDs)
	require.True(t, input.SkipMixedChannelCheck)
	require.Equal(t, "runtime-key", input.Credentials["api_key"])
	require.Equal(t, kiroRuntimeDefaultBaseURL, input.Credentials["base_url"])
	mapping, ok := input.Credentials["model_mapping"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "qwen3-coder-next", mapping["qwen3-coder-next"])
	require.Equal(t, "kiro", input.Extra["provider_adapter"])
	require.Equal(t, "kiro-gateway", input.Extra["provider_adapter_runtime"])
}

func TestEnsureKiroRuntimeUpstreamAccountUpdatesExistingWithoutDroppingMapping(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:       77,
			Name:     "custom-kiro-account",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":       "old-key",
				"base_url":      "http://old",
				"model_mapping": map[string]any{"custom-model": "custom-upstream"},
			},
			Extra: map[string]any{"provider_adapter": "kiro", "keep": "yes"},
		},
	}
	handler := &AccountHandler{adminService: adminSvc}

	result, err := handler.ensureKiroRuntimeUpstreamAccount(context.Background(), kiroAdapterConfig{
		InternalBaseURL: "http://kiro-gateway:8000/",
		InternalAPIKey:  "new-key",
	}, []int64{3})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "updated", result.Action)
	require.Equal(t, int64(77), result.AccountID)
	require.Equal(t, "http://kiro-gateway:8000", result.BaseURL)

	require.Empty(t, adminSvc.createdAccounts)
	require.Equal(t, []int64{77}, adminSvc.updatedAccountIDs)
	require.Len(t, adminSvc.updatedAccounts, 1)
	input := adminSvc.updatedAccounts[0]
	require.Equal(t, []int64{3}, *input.GroupIDs)
	require.True(t, input.SkipMixedChannelCheck)
	require.Equal(t, "new-key", input.Credentials["api_key"])
	require.Equal(t, "http://kiro-gateway:8000", input.Credentials["base_url"])
	mapping, ok := input.Credentials["model_mapping"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "custom-upstream", mapping["custom-model"])
	require.Equal(t, "deepseek-3.2", mapping["deepseek-3.2"])
	require.Equal(t, "yes", input.Extra["keep"])
	require.Equal(t, "kiro-gateway", input.Extra["provider_adapter_runtime"])
}

func TestKiroImportHandlerEnsuresUpstreamAfterSuccessfulImport(t *testing.T) {
	old := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	defer service.SetDefaultIdempotencyCoordinator(old)

	adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/admin/credentials", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer adapter.Close()

	t.Setenv(envKiroAdapterBaseURL, adapter.URL)
	t.Setenv(envKiroAdapterRuntimeBaseURL, "http://kiro-gateway:8000")
	t.Setenv(envKiroAdapterAdminAPIKey, "admin-key")
	t.Setenv(envKiroAdapterInternalAPIKey, "runtime-key")
	t.Setenv(envProviderAdaptersKiroBaseURL, "")
	t.Setenv(envProviderAdaptersKiroRuntimeURL, "")
	t.Setenv(envProviderAdaptersKiroAPIKey, "")

	adminSvc := newStubAdminService()
	adminSvc.accounts = nil

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &AccountHandler{adminService: adminSvc}
	router.POST("/api/v1/admin/accounts/import/kiro", handler.ImportKiroCredentials)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/import/kiro", strings.NewReader(`{"refresh_token":"refresh-1","group_ids":[8,7,8]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "runtime-key")
	require.NotContains(t, recorder.Body.String(), "admin-key")
	require.NotContains(t, recorder.Body.String(), "refresh-1")
	require.Contains(t, recorder.Body.String(), `"action":"created"`)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, []int64{7, 8}, adminSvc.createdAccounts[0].GroupIDs)
}

func TestSanitizeKiroAdapterResponseHandlesNonJSON(t *testing.T) {
	got := sanitizeKiroAdapterResponse([]byte(`refreshToken=abc kiroApiKey=def plain`))
	asMap, ok := got.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "refreshToken=*** kiroApiKey=*** plain", asMap["body"])
}
