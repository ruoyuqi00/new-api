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

	payload := buildKiroImportIdempotencyPayload(credentials, duplicateCount)
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
		{RefreshToken: "refresh-1", AuthMethod: "social", Email: "one@example.com"},
		{KiroAPIKey: "ksk_1", AuthMethod: "api_key"},
	}, 0)
	require.NoError(t, err)

	require.Equal(t, []string{"/api/admin/credentials", "/api/admin/credentials"}, gotPaths)
	require.Equal(t, "Bearer admin-key", gotAuth)
	require.Equal(t, "admin-key", gotXAPIKey)
	require.Len(t, gotBodies, 2)
	require.Equal(t, "refresh-1", gotBodies[0]["refreshToken"])
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

func TestSanitizeKiroAdapterResponseHandlesNonJSON(t *testing.T) {
	got := sanitizeKiroAdapterResponse([]byte(`refreshToken=abc kiroApiKey=def plain`))
	asMap, ok := got.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "refreshToken=*** kiroApiKey=*** plain", asMap["body"])
}
