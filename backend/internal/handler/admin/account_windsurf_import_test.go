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

func TestParseWindsurfImportAccounts(t *testing.T) {
	req := WindsurfImportRequest{
		Token:  " token-1 ",
		Tokens: []string{"token-1", " token-2 "},
		Raw:    "token-3\n token-2 ; token-4 ; user@example.com----secret-pass ",
		Accounts: []WindsurfImportAccount{
			{APIKey: " api-key-1 ", Label: " main ", Proxy: " http://127.0.0.1:9000 "},
			{Email: " login@example.com ", Password: " pass-1 "},
		},
	}

	accounts, duplicateCount, err := parseWindsurfImportAccounts(req)
	require.NoError(t, err)
	require.Equal(t, 7, len(accounts))
	require.Equal(t, 2, duplicateCount)
	require.Equal(t, "token-1", accounts[0].Token)
	require.Equal(t, "token-2", accounts[1].Token)
	require.Equal(t, "api-key-1", accounts[2].APIKey)
	require.Equal(t, "main", accounts[2].Label)
	require.Equal(t, "http://127.0.0.1:9000", accounts[2].Proxy)
	require.Equal(t, "login@example.com", accounts[3].Email)
	require.Equal(t, "pass-1", accounts[3].Password)
	require.Equal(t, "token-3", accounts[4].Token)
	require.Equal(t, "token-4", accounts[5].Token)
	require.Equal(t, "user@example.com", accounts[6].Email)
	require.Equal(t, "secret-pass", accounts[6].Password)
}

func TestParseWindsurfImportAccountsRejectsMixedSecretKinds(t *testing.T) {
	_, _, err := parseWindsurfImportAccounts(WindsurfImportRequest{
		Accounts: []WindsurfImportAccount{{Token: "token-1", APIKey: "api-key-1"}},
	})
	require.ErrorContains(t, err, "只能提供 token")
}

func TestParseWindsurfImportAccountsRejectsPartialEmailPassword(t *testing.T) {
	_, _, err := parseWindsurfImportAccounts(WindsurfImportRequest{
		Accounts: []WindsurfImportAccount{{Email: "login@example.com"}},
	})
	require.ErrorContains(t, err, "同时提供 email 和 password")
}

func TestWindsurfImportIdempotencyPayloadDoesNotContainRawSecrets(t *testing.T) {
	accounts, duplicateCount, err := parseWindsurfImportAccounts(WindsurfImportRequest{
		Tokens: []string{"raw-token-secret"},
		Accounts: []WindsurfImportAccount{
			{APIKey: "raw-api-key-secret", Label: "label"},
			{Email: "private@example.com", Password: "raw-password-secret"},
		},
	})
	require.NoError(t, err)

	payload := buildWindsurfImportIdempotencyPayload(accounts, duplicateCount)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "raw-token-secret")
	require.NotContains(t, string(raw), "raw-api-key-secret")
	require.NotContains(t, string(raw), "raw-password-secret")
	require.NotContains(t, string(raw), "private@example.com")
	require.Contains(t, string(raw), hashWindsurfSecret("raw-token-secret"))
	require.Contains(t, string(raw), hashWindsurfSecret("raw-api-key-secret"))
	require.Contains(t, string(raw), hashWindsurfSecret("raw-password-secret"))
	require.Contains(t, string(raw), hashWindsurfSecret("private@example.com"))
}

func TestImportWindsurfAccountsForwardsToAdapterAndRedactsResponse(t *testing.T) {
	var gotAuth string
	var gotXAPIKey string
	var gotPath string
	var gotBody struct {
		Accounts []windsurfForwardAccount `json:"accounts"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotXAPIKey = r.Header.Get("x-api-key")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"token":"should-not-return","nested":{"api_key":"secret","ok":true},"results":[{"id":"1","email":"a@example.com","status":"active"}]}`))
	}))
	defer server.Close()

	result, err := importWindsurfAccounts(context.Background(), windsurfAdapterConfig{
		InternalBaseURL: server.URL + "/",
		InternalAPIKey:  "internal-key",
		Timeout:         time.Second,
	}, []windsurfForwardAccount{
		{Token: "token-1", Label: "one"},
		{APIKey: "api-key-1", Proxy: "http://127.0.0.1:9000"},
		{Email: "login@example.com", Password: "pass-1"},
	}, 0)
	require.NoError(t, err)

	require.Equal(t, "/auth/login", gotPath)
	require.Equal(t, "Bearer internal-key", gotAuth)
	require.Equal(t, "internal-key", gotXAPIKey)
	require.Len(t, gotBody.Accounts, 3)
	require.Equal(t, "token-1", gotBody.Accounts[0].Token)
	require.Equal(t, "one", gotBody.Accounts[0].Label)
	require.Equal(t, "api-key-1", gotBody.Accounts[1].APIKey)
	require.Equal(t, "login@example.com", gotBody.Accounts[2].Email)
	require.Equal(t, "pass-1", gotBody.Accounts[2].Password)
	require.Equal(t, 3, result.Total)
	require.Equal(t, http.StatusOK, result.UpstreamStatus)

	upstream, ok := result.Upstream.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", upstream["token"])
	nested, ok := upstream["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", nested["api_key"])
	require.Equal(t, true, nested["ok"])
}

func TestImportWindsurfAccountsReturnsSafeUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad","token":"should-not-return"}`))
	}))
	defer server.Close()

	result, err := importWindsurfAccounts(context.Background(), windsurfAdapterConfig{
		InternalBaseURL: server.URL,
		InternalAPIKey:  "internal-key",
		Timeout:         time.Second,
	}, []windsurfForwardAccount{{Token: "token-1"}}, 0)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, result.UpstreamStatus)
	require.NotContains(t, err.Error(), "token-1")
	upstream, ok := result.Upstream.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "***", upstream["token"])
}

func TestImportWindsurfTokensRequiresAdapterConfig(t *testing.T) {
	old := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(nil)
	defer service.SetDefaultIdempotencyCoordinator(old)

	t.Setenv(envWindsurfAdapterBaseURL, "")
	t.Setenv(envWindsurfAdapterAPIKey, "")
	t.Setenv(envProviderAdaptersWindsurfBaseURL, "")
	t.Setenv(envProviderAdaptersWindsurfAPIKey, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &AccountHandler{}
	router.POST("/api/v1/admin/accounts/import/windsurf", handler.ImportWindsurfTokens)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/import/windsurf", strings.NewReader(`{"token":"token-1"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "token-1")
	require.Contains(t, recorder.Body.String(), "WINDSURF_ADAPTER_NOT_CONFIGURED")
}

func TestSanitizeWindsurfAdapterResponseHandlesNonJSON(t *testing.T) {
	got := sanitizeWindsurfAdapterResponse([]byte(`token=abc api_key=def plain`))
	asMap, ok := got.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "token=*** api_key=*** plain", asMap["body"])
}
