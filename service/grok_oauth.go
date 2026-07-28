package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	grokOAuthAuthorizeURL = "https://auth.x.ai/oauth2/authorize"
	grokOAuthTokenURL     = "https://auth.x.ai/oauth2/token"
	grokOAuthClientID     = "b1a00492-073a-47ea-816f-4c329264a828"
	grokOAuthScope        = "openid profile email offline_access grok-cli:access api:access"
	grokOAuthRedirectURI  = "http://127.0.0.1:56121/callback"
	grokOAuthSessionTTL   = 30 * time.Minute
	grokOAuthDefaultTTL   = 6 * time.Hour
)

type grokOAuthSession struct {
	State        string
	CodeVerifier string
	CreatedAt    time.Time
}

var grokOAuthSessions = struct {
	sync.Mutex
	items map[string]grokOAuthSession
}{items: make(map[string]grokOAuthSession)}

type GrokOAuthAuthorization struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
}

type GrokOAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	ClientID     string `json:"client_id,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Email        string `json:"email,omitempty"`
	Subject      string `json:"subject,omitempty"`
}

type GrokOAuthCredential struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Email        string `json:"email,omitempty"`
	Subject      string `json:"subject,omitempty"`
	ExpiresAt    string `json:"expires_at"`
	Type         string `json:"type"`
}

type grokOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func GenerateGrokOAuthAuthorization() (*GrokOAuthAuthorization, error) {
	state, err := randomGrokOAuthValue(32, false)
	if err != nil {
		return nil, fmt.Errorf("generate Grok OAuth state: %w", err)
	}
	nonce, err := randomGrokOAuthValue(16, false)
	if err != nil {
		return nil, fmt.Errorf("generate Grok OAuth nonce: %w", err)
	}
	sessionID, err := randomGrokOAuthValue(16, false)
	if err != nil {
		return nil, fmt.Errorf("generate Grok OAuth session: %w", err)
	}
	codeVerifier, err := randomGrokOAuthValue(32, true)
	if err != nil {
		return nil, fmt.Errorf("generate Grok OAuth verifier: %w", err)
	}
	challengeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])

	authorizeURL, err := url.Parse(grokOAuthAuthorizeURL)
	if err != nil {
		return nil, err
	}
	query := authorizeURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", grokOAuthClientID)
	query.Set("redirect_uri", grokOAuthRedirectURI)
	query.Set("scope", grokOAuthScope)
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	query.Set("plan", "generic")
	query.Set("referrer", "new-api")
	authorizeURL.RawQuery = query.Encode()

	now := time.Now()
	grokOAuthSessions.Lock()
	for id, session := range grokOAuthSessions.items {
		if now.Sub(session.CreatedAt) > grokOAuthSessionTTL {
			delete(grokOAuthSessions.items, id)
		}
	}
	grokOAuthSessions.items[sessionID] = grokOAuthSession{
		State: state, CodeVerifier: codeVerifier, CreatedAt: now,
	}
	grokOAuthSessions.Unlock()

	return &GrokOAuthAuthorization{AuthURL: authorizeURL.String(), SessionID: sessionID}, nil
}

func ExchangeGrokOAuthAuthorization(ctx context.Context, sessionID string, callbackInput string) (*GrokOAuthToken, error) {
	sessionID = strings.TrimSpace(sessionID)
	grokOAuthSessions.Lock()
	session, ok := grokOAuthSessions.items[sessionID]
	delete(grokOAuthSessions.items, sessionID)
	grokOAuthSessions.Unlock()
	if !ok || time.Since(session.CreatedAt) > grokOAuthSessionTTL {
		return nil, errors.New("Grok OAuth session not found or expired")
	}

	code, state, stateRequired := parseGrokOAuthCallback(callbackInput)
	if code == "" {
		return nil, errors.New("Grok OAuth callback is missing the authorization code")
	}
	if stateRequired && state == "" {
		return nil, errors.New("Grok OAuth callback is missing state")
	}
	if state != "" && subtle.ConstantTimeCompare([]byte(state), []byte(session.State)) != 1 {
		return nil, errors.New("Grok OAuth callback state does not match the authorization session")
	}

	client, err := getCodexOAuthHTTPClient("")
	if err != nil {
		return nil, err
	}
	return exchangeGrokOAuthCode(ctx, client, grokOAuthTokenURL, code, session.CodeVerifier)
}

func RefreshGrokOAuthCredential(ctx context.Context, rawCredential string) (string, int64, error) {
	var credential GrokOAuthCredential
	if err := common.UnmarshalJsonStr(strings.TrimSpace(rawCredential), &credential); err != nil {
		return "", 0, errors.New("invalid Grok OAuth credential")
	}
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return "", 0, errors.New("Grok OAuth credential has no refresh_token")
	}
	clientID := strings.TrimSpace(credential.ClientID)
	if clientID == "" {
		clientID = grokOAuthClientID
	}
	client, err := getCodexOAuthHTTPClient("")
	if err != nil {
		return "", 0, err
	}
	token, err := refreshGrokOAuthToken(ctx, client, grokOAuthTokenURL, credential.RefreshToken, clientID)
	if err != nil {
		return "", 0, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = credential.RefreshToken
	}
	if token.IDToken == "" {
		token.IDToken = credential.IDToken
	}
	if token.Email == "" {
		token.Email = credential.Email
	}
	if token.Subject == "" {
		token.Subject = credential.Subject
	}
	encoded, err := BuildGrokOAuthCredential(token)
	if err != nil {
		return "", 0, err
	}
	return encoded, token.ExpiresAt, nil
}

func BuildGrokOAuthCredential(token *GrokOAuthToken) (string, error) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return "", errors.New("Grok OAuth token is missing access_token")
	}
	credential := GrokOAuthCredential{
		AccessToken: strings.TrimSpace(token.AccessToken), RefreshToken: strings.TrimSpace(token.RefreshToken),
		IDToken: strings.TrimSpace(token.IDToken), TokenType: strings.TrimSpace(token.TokenType),
		ClientID: strings.TrimSpace(token.ClientID), Scope: strings.TrimSpace(token.Scope),
		Email: strings.TrimSpace(token.Email), Subject: strings.TrimSpace(token.Subject),
		ExpiresAt: time.Unix(token.ExpiresAt, 0).UTC().Format(time.RFC3339),
		Type:      "grok",
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		return "", fmt.Errorf("encode Grok OAuth credential: %w", err)
	}
	return string(encoded), nil
}

func exchangeGrokOAuthCode(ctx context.Context, client *http.Client, tokenURL string, code string, codeVerifier string) (*GrokOAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", grokOAuthClientID)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", grokOAuthRedirectURI)
	form.Set("code_verifier", codeVerifier)
	return requestGrokOAuthToken(ctx, client, tokenURL, form, "exchange")
}

func refreshGrokOAuthToken(ctx context.Context, client *http.Client, tokenURL string, refreshToken string, clientID string) (*GrokOAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	return requestGrokOAuthToken(ctx, client, tokenURL, form, "refresh")
}

func requestGrokOAuthToken(ctx context.Context, client *http.Client, tokenURL string, form url.Values, operation string) (*GrokOAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-grok-oauth/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Grok OAuth %s request failed: %w", operation, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Grok OAuth %s failed: status=%d", operation, resp.StatusCode)
	}
	var payload grokOAuthTokenResponse
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("decode Grok OAuth %s response: %w", operation, err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, fmt.Errorf("Grok OAuth %s response is missing access_token", operation)
	}
	if operation == "exchange" && strings.TrimSpace(payload.RefreshToken) == "" {
		return nil, errors.New("Grok OAuth response is missing refresh_token; authorize offline access again")
	}
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int64(grokOAuthDefaultTTL.Seconds())
	}
	token := &GrokOAuthToken{
		AccessToken: strings.TrimSpace(payload.AccessToken), RefreshToken: strings.TrimSpace(payload.RefreshToken),
		IDToken: strings.TrimSpace(payload.IDToken), TokenType: strings.TrimSpace(payload.TokenType),
		ExpiresIn: expiresIn, ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
		ClientID: grokOAuthClientID, Scope: strings.TrimSpace(payload.Scope),
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	for _, rawToken := range []string{token.IDToken, token.AccessToken} {
		claims, ok := decodeJWTClaims(rawToken)
		if !ok {
			continue
		}
		if token.Subject == "" {
			token.Subject, _ = claims["sub"].(string)
			token.Subject = strings.TrimSpace(token.Subject)
		}
		if token.Email == "" {
			token.Email, _ = claims["email"].(string)
			token.Email = strings.TrimSpace(token.Email)
		}
	}
	return token, nil
}

func parseGrokOAuthCallback(raw string) (code string, state string, stateRequired bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}
	if parsed, err := url.Parse(trimmed); err == nil {
		if value := strings.TrimSpace(parsed.Query().Get("code")); value != "" {
			return value, strings.TrimSpace(parsed.Query().Get("state")), true
		}
	}
	queryCandidate := strings.TrimPrefix(trimmed, "?")
	if strings.Contains(queryCandidate, "=") {
		if values, err := url.ParseQuery(queryCandidate); err == nil {
			if value := strings.TrimSpace(values.Get("code")); value != "" {
				return value, strings.TrimSpace(values.Get("state")), true
			}
		}
	}
	return trimmed, "", false
}

func randomGrokOAuthValue(size int, base64URL bool) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	if base64URL {
		return base64.RawURLEncoding.EncodeToString(value), nil
	}
	return hex.EncodeToString(value), nil
}
