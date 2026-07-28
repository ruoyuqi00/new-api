package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type authFlowTestOAuthProvider struct {
	exchangeErr   error
	userInfoErr   error
	exchangeCalls int
	userInfoCalls int
	lookupCalls   int
	user          *model.User
	enabled       bool
}

func (*authFlowTestOAuthProvider) GetName() string          { return "Auth Flow Test" }
func (provider *authFlowTestOAuthProvider) IsEnabled() bool { return provider.enabled }
func (provider *authFlowTestOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	provider.exchangeCalls++
	if provider.exchangeErr != nil {
		return nil, provider.exchangeErr
	}
	return &oauth.OAuthToken{}, nil
}
func (provider *authFlowTestOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	provider.userInfoCalls++
	if provider.userInfoErr != nil {
		return nil, provider.userInfoErr
	}
	return &oauth.OAuthUser{ProviderUserID: "external-user"}, nil
}
func (provider *authFlowTestOAuthProvider) IsUserIDTaken(providerUserID string) bool {
	provider.lookupCalls++
	return provider.user != nil && providerUserID == "external-user"
}
func (provider *authFlowTestOAuthProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	if !provider.IsUserIDTaken(providerUserID) {
		return errors.New("test OAuth user not found")
	}
	*user = *provider.user
	return nil
}
func (*authFlowTestOAuthProvider) SetProviderUserID(*model.User, string) {}
func (*authFlowTestOAuthProvider) GetProviderPrefix() string             { return "flow_" }

const testOAuthLoginInitiatorCookiePrefix = "oauth_login_initiator_"

func setupAuthFlowControllerTest(t *testing.T) *authFlowTestOAuthProvider {
	t.Helper()
	require.NoError(t, i18n.Init())
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.AuthFlow{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	provider := &authFlowTestOAuthProvider{
		enabled: true,
		user: &model.User{
			Username: "auth-flow-test-user", Password: "unused", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
		},
	}
	require.NoError(t, db.Create(provider.user).Error)
	oauth.Register("auth-flow-test", provider)
	t.Cleanup(func() {
		oauth.Unregister("auth-flow-test")
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
	return provider
}

func createOAuthLoginState(t *testing.T, aff string) (string, *http.Cookie) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/oauth/state", strings.NewReader(`{"provider":"auth-flow-test","intent":"login","aff":"`+aff+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	GenerateOAuthCode(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			FlowToken string `json:"flow_token"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.FlowToken)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, testOAuthLoginInitiatorCookiePrefix+response.Data.FlowToken, cookies[0].Name)
	assert.Equal(t, response.Data.ExpiresAt, cookies[0].Expires.Unix())
	return response.Data.FlowToken, cookies[0]
}

func callbackOAuth(t *testing.T, state string, query string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.GET("/api/oauth/:provider", HandleOAuth)
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/auth-flow-test?state="+state+"&"+query, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestGenerateOAuthCodeBindsLoginFlowToInitiatorCookie(t *testing.T) {
	setupAuthFlowControllerTest(t)
	previousSecure := common.SessionCookieSecure
	common.SessionCookieSecure = true
	t.Cleanup(func() { common.SessionCookieSecure = previousSecure })

	state, cookie := createOAuthLoginState(t, "invite-code")

	assert.Equal(t, "/api/oauth", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Empty(t, cookie.Domain)
	assert.Greater(t, cookie.MaxAge, 0)

	flow, err := model.GetAuthFlow(state, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentLogin,
	})
	require.NoError(t, err)
	var payload map[string]string
	require.NoError(t, common.UnmarshalJsonStr(flow.Payload, &payload))
	assert.Equal(t, "invite-code", payload["affiliate_code"])
	assert.NotEmpty(t, payload["login_initiator_hash"])
	assert.NotEqual(t, cookie.Value, payload["login_initiator_hash"])
	assert.NotContains(t, flow.Payload, cookie.Value)
	assert.NotContains(t, payload, "login_initiator_secret")
}

func TestOAuthLoginRejectsMissingOrMismatchedInitiatorCookieBeforeProviderExchange(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)

	for _, test := range []struct {
		name           string
		callbackCookie *http.Cookie
	}{
		{name: "missing cookie"},
		{name: "mismatched cookie", callbackCookie: &http.Cookie{Name: testOAuthLoginInitiatorCookiePrefix + "other-state", Value: "other-secret", Path: "/api/oauth"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, initiatorCookie := createOAuthLoginState(t, "")
			callbackCookie := test.callbackCookie
			if callbackCookie != nil {
				callbackCookie = &http.Cookie{Name: initiatorCookie.Name, Value: callbackCookie.Value, Path: "/api/oauth"}
			}
			response := callbackOAuth(t, state, "code=test", callbackCookie)

			require.Equal(t, http.StatusForbidden, response.Code)
			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			assert.False(t, body.Success)
			assert.Equal(t, i18n.Translate(i18n.DefaultLang, i18n.MsgOAuthStateInvalid), body.Message)
			assert.Zero(t, provider.exchangeCalls)
			assert.Zero(t, provider.userInfoCalls)
			flow, err := model.GetAuthFlow(state, model.AuthFlowMatch{
				Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentLogin,
			})
			require.NoError(t, err)
			assert.Nil(t, flow.ConsumedAt)
		})
	}
}

func TestOAuthLoginAcceptsMatchingInitiatorCookieAndConsumesFlow(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	state, cookie := createOAuthLoginState(t, "")

	response := callbackOAuth(t, state, "code=test", cookie)

	assert.Equal(t, http.StatusOK, response.Code)
	_, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Equal(t, 1, provider.exchangeCalls)
	assert.Equal(t, 1, provider.userInfoCalls)
	var cleared *http.Cookie
	for _, responseCookie := range response.Result().Cookies() {
		if responseCookie.Name == cookie.Name {
			cleared = responseCookie
			break
		}
	}
	require.NotNil(t, cleared)
	assert.Equal(t, -1, cleared.MaxAge)
}

func TestOAuthLoginProviderDenialConsumesFlowAndClearsInitiatorCookie(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	state, cookie := createOAuthLoginState(t, "")

	response := callbackOAuth(t, state, "error=access_denied&error_description=cancelled", cookie)

	require.Equal(t, http.StatusOK, response.Code)
	_, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Zero(t, provider.exchangeCalls)
	assert.Zero(t, provider.userInfoCalls)
	var cleared *http.Cookie
	for _, responseCookie := range response.Result().Cookies() {
		if responseCookie.Name == cookie.Name {
			cleared = responseCookie
			break
		}
	}
	require.NotNil(t, cleared)
	assert.Empty(t, cleared.Value)
	assert.Equal(t, "/api/oauth", cleared.Path)
	assert.Equal(t, -1, cleared.MaxAge)
}

func TestOAuthLoginProviderDenialConsumesFlowWhenProviderBecomesDisabled(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	state, cookie := createOAuthLoginState(t, "")
	provider.enabled = false

	response := callbackOAuth(t, state, "error=access_denied&error_description=cancelled", cookie)

	require.Equal(t, http.StatusOK, response.Code)
	_, err := model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Zero(t, provider.exchangeCalls)
	assert.Zero(t, provider.userInfoCalls)
	assert.Zero(t, provider.lookupCalls)
	responseCookies := response.Result().Cookies()
	require.Len(t, responseCookies, 1)
	assert.Equal(t, cookie.Name, responseCookies[0].Name)
	assert.Empty(t, responseCookies[0].Value)
	assert.Equal(t, -1, responseCookies[0].MaxAge)
}

func TestOAuthLoginConcurrentStatesUseIndependentInitiatorCookies(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	firstState, firstCookie := createOAuthLoginState(t, "")
	secondState, secondCookie := createOAuthLoginState(t, "")

	assert.NotEqual(t, firstState, secondState)
	assert.NotEqual(t, firstCookie.Name, secondCookie.Name)
	assert.NotEqual(t, firstCookie.Value, secondCookie.Value)

	response := callbackOAuth(t, firstState, "code=test", secondCookie)
	require.Equal(t, http.StatusForbidden, response.Code)
	assert.Zero(t, provider.exchangeCalls)
	flow, err := model.GetAuthFlow(firstState, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	require.NoError(t, err)
	assert.Nil(t, flow.ConsumedAt)

	callbackOAuth(t, firstState, "code=test", firstCookie)
	callbackOAuth(t, secondState, "code=test", secondCookie)
	_, err = model.GetAuthFlow(firstState, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	_, err = model.GetAuthFlow(secondState, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Equal(t, 2, provider.exchangeCalls)
	assert.Equal(t, 2, provider.userInfoCalls)
}

func TestGenerateOAuthCodeCarriesAffiliateInLoginFlow(t *testing.T) {
	setupAuthFlowControllerTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/oauth/state", strings.NewReader(`{"provider":"auth-flow-test","intent":"login","aff":"invite-code"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	GenerateOAuthCode(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			FlowToken string `json:"flow_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	flow, err := model.GetAuthFlow(response.Data.FlowToken, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentLogin,
	})
	require.NoError(t, err)
	var payload oauthFlowPayload
	require.NoError(t, common.UnmarshalJsonStr(flow.Payload, &payload))
	assert.Equal(t, "invite-code", payload.AffiliateCode)
	assert.Zero(t, flow.UserId)
	assert.Empty(t, flow.SessionId)
}

func TestGenerateOAuthCodeBindsFlowToAuthenticatedSession(t *testing.T) {
	setupAuthFlowControllerTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/oauth/state", strings.NewReader(`{"provider":"auth-flow-test","intent":"bind"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 42)
	c.Set("session_id", "session-42")
	c.Set("auth_version", int64(3))
	c.Set("session_version", int64(2))

	GenerateOAuthCode(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			FlowToken string `json:"flow_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	flow, err := model.GetAuthFlow(response.Data.FlowToken, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentBind,
		UserId: 42, SessionId: "session-42",
	})
	require.NoError(t, err)
	assert.Equal(t, 42, flow.UserId)
	assert.Equal(t, "session-42", flow.SessionId)
}

func TestOAuthLoginConsumesFlowOnlyAfterProviderIdentity(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)

	tests := []struct {
		name        string
		exchangeErr error
		userInfoErr error
	}{
		{name: "exchange failure", exchangeErr: errors.New("exchange failed")},
		{name: "user info failure", userInfoErr: errors.New("user info failed")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider.exchangeErr = test.exchangeErr
			provider.userInfoErr = test.userInfoErr
			token, cookie := createOAuthLoginState(t, "")
			response := callbackOAuth(t, token, "code=test", cookie)

			flow, err := model.GetAuthFlow(token, model.AuthFlowMatch{
				Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentLogin,
			})
			require.NoError(t, err)
			assert.Nil(t, flow.ConsumedAt)
			for _, responseCookie := range response.Result().Cookies() {
				assert.NotEqual(t, cookie.Name, responseCookie.Name)
			}
		})
	}
}

func TestOAuthLoginConsumesFlowAfterProviderIdentityAndOnProviderError(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)

	provider.exchangeErr = nil
	provider.userInfoErr = nil
	successToken, successCookie := createOAuthLoginState(t, "")
	callbackOAuth(t, successToken, "code=test", successCookie)
	var err error
	_, err = model.GetAuthFlow(successToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Equal(t, 1, provider.exchangeCalls)
	assert.Equal(t, 1, provider.userInfoCalls)

	providerErrorToken, providerErrorCookie := createOAuthLoginState(t, "")
	callbackOAuth(t, providerErrorToken, "error=access_denied", providerErrorCookie)
	_, err = model.GetAuthFlow(providerErrorToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Equal(t, 1, provider.exchangeCalls)
	assert.Equal(t, 1, provider.userInfoCalls)
}

func TestOAuthBindProviderErrorConsumesSessionBoundFlow(t *testing.T) {
	provider := setupAuthFlowControllerTest(t)
	flowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeOAuth, Provider: "auth-flow-test", Intent: model.AuthFlowIntentBind,
		UserId: 42, SessionId: "session-42", Payload: `{}`, ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 42)
		c.Set("session_id", "session-42")
		c.Set("auth_version", int64(1))
		c.Set("session_version", int64(1))
		c.Next()
	})
	router.GET("/api/oauth/:provider", HandleOAuth)
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/auth-flow-test?state="+flowToken+"&error=access_denied&error_description=cancelled", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	_, err = model.GetAuthFlow(flowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
	assert.Zero(t, provider.exchangeCalls)
	assert.Zero(t, provider.userInfoCalls)
	assert.Empty(t, response.Result().Cookies())
}
