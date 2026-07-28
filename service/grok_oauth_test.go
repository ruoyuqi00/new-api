package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeGrokOAuthCodeUsesPKCEContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, grokOAuthClientID, r.Form.Get("client_id"))
		assert.Equal(t, "auth-code", r.Form.Get("code"))
		assert.Equal(t, grokOAuthRedirectURI, r.Form.Get("redirect_uri"))
		assert.Equal(t, "verifier", r.Form.Get("code_verifier"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	token, err := exchangeGrokOAuthCode(context.Background(), server.Client(), server.URL, "auth-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, "access", token.AccessToken)
	assert.Equal(t, "refresh", token.RefreshToken)
	assert.Equal(t, int64(3600), token.ExpiresIn)
	assert.WithinDuration(t, time.Now().Add(time.Hour), time.Unix(token.ExpiresAt, 0), 2*time.Second)
}

func TestRefreshGrokOAuthTokenPreservesRefreshAtCredentialLayer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "original-refresh", r.Form.Get("refresh_token"))
		assert.Equal(t, "custom-client", r.Form.Get("client_id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"new-access","expires_in":7200}`)
	}))
	defer server.Close()

	token, err := refreshGrokOAuthToken(context.Background(), server.Client(), server.URL, "original-refresh", "custom-client")
	require.NoError(t, err)
	assert.Equal(t, "new-access", token.AccessToken)
	assert.Empty(t, token.RefreshToken)
}

func TestGrokOAuthTokenUsesAccessTokenIdentityWhenIDTokenIsMissing(t *testing.T) {
	accessToken := grokOAuthTestJWT(t, map[string]interface{}{
		"sub": "grok-user", "email": "grok@example.com",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"refresh","expires_in":3600}`, accessToken)
	}))
	defer server.Close()

	token, err := exchangeGrokOAuthCode(context.Background(), server.Client(), server.URL, "auth-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, "grok-user", token.Subject)
	assert.Equal(t, "grok@example.com", token.Email)
}

func TestGenerateGrokOAuthAuthorizationBuildsOfficialPKCEURL(t *testing.T) {
	authorization, err := GenerateGrokOAuthAuthorization()
	require.NoError(t, err)
	parsed, err := url.Parse(authorization.AuthURL)
	require.NoError(t, err)
	assert.Equal(t, "auth.x.ai", parsed.Host)
	assert.Equal(t, grokOAuthClientID, parsed.Query().Get("client_id"))
	assert.Equal(t, grokOAuthRedirectURI, parsed.Query().Get("redirect_uri"))
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	assert.NotEmpty(t, parsed.Query().Get("code_challenge"))
	assert.NotEmpty(t, parsed.Query().Get("state"))
	assert.NotEmpty(t, authorization.SessionID)
}

func TestParseGrokOAuthCallback(t *testing.T) {
	code, state, required := parseGrokOAuthCallback("http://127.0.0.1:56121/callback?code=abc&state=xyz")
	assert.Equal(t, "abc", code)
	assert.Equal(t, "xyz", state)
	assert.True(t, required)

	code, state, required = parseGrokOAuthCallback("bare-code")
	assert.Equal(t, "bare-code", code)
	assert.Empty(t, state)
	assert.False(t, required)
}

func grokOAuthTestJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	payload, err := common.Marshal(claims)
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
