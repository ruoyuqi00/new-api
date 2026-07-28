package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeGrokOAuthCredentialUsesCLIResponsesHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		require.Equal(t, "xai-grok-cli", r.Header.Get("X-XAI-Token-Auth"))
		require.Equal(t, "0.2.93", r.Header.Get("x-grok-client-version"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"resp_1","output":[]}`)
	}))
	t.Cleanup(server.Close)

	err := ProbeGrokOAuthCredential(t.Context(), server.URL, `{"access_token":"access-token"}`)
	require.NoError(t, err)
}

func TestFetchGrokBillingPreservesForbiddenEligibilityObservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Contains(t, []string{"/billing", "/billing?format=credits"}, r.URL.RequestURI())
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":"media entitlement required"}`)
	}))
	t.Cleanup(server.Close)

	billing, status := fetchGrokBilling(t.Context(), server.Client(), server.URL, "access-token")
	require.Nil(t, billing)
	require.Equal(t, http.StatusForbidden, status)
}
