package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAuthLogoutRejectsRefreshCookieSessionMismatch(t *testing.T) {
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	model.DB = db
	common.RedisEnabled = false
	common.SessionSecret = "auth-logout-mismatch-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
	})

	user := &model.User{
		Username: "logout-mismatch-user", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	sessionA, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "agent-a")
	require.NoError(t, err)
	sessionB, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "agent-b")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/auth/logout", nil)
	c.Request.Header.Set("Authorization", "Bearer "+sessionA.AccessToken)
	c.Request.Header.Set("X-Auth-Session", sessionA.Session.SID)
	c.Request.AddCookie(&http.Cookie{Name: service.RefreshCookieName, Value: sessionB.RefreshToken})

	AuthLogout(c)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Equal(t, "AUTH_SESSION_MISMATCH", response.Code)
	for _, sid := range []string{sessionA.Session.SID, sessionB.Session.SID} {
		stored, err := model.GetUserSessionBySID(sid)
		require.NoError(t, err)
		assert.Equal(t, model.UserSessionStatusActive, stored.Status)
	}
}

func TestWriteAuthSessionErrorMapsSessionGrowthLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "active session limit",
			err:            model.ErrUserSessionLimit,
			expectedStatus: http.StatusConflict,
			expectedCode:   "AUTH_SESSION_LIMIT",
		},
		{
			name:           "issuance limit",
			err:            model.ErrUserSessionIssuanceLimit,
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "AUTH_SESSION_ISSUANCE_LIMIT",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			writeAuthSessionError(c, test.err)

			assert.Equal(t, test.expectedStatus, recorder.Code)
			var response struct {
				Success bool   `json:"success"`
				Code    string `json:"code"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.False(t, response.Success)
			assert.Equal(t, test.expectedCode, response.Code)
		})
	}
}

func TestSessionLimitReplacementReturnsSuccessfulLogin(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	previousPasswordLoginEnabled := common.PasswordLoginEnabled
	previousActiveLimit := common.UserSessionActiveLimit
	previousIssuanceLimit := common.UserSessionIssuanceLimit
	previousIssuanceWindow := common.UserSessionIssuanceWindowSeconds
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.TwoFA{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "session-limit-replacement-test-secret"
	common.PasswordLoginEnabled = true
	common.UserSessionActiveLimit = 1
	common.UserSessionIssuanceLimit = 100
	common.UserSessionIssuanceWindowSeconds = int64(common.DefaultUserSessionIssuanceWindowSeconds)
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		common.PasswordLoginEnabled = previousPasswordLoginEnabled
		common.UserSessionActiveLimit = previousActiveLimit
		common.UserSessionIssuanceLimit = previousIssuanceLimit
		common.UserSessionIssuanceWindowSeconds = previousIssuanceWindow
	})

	const previousLastLoginAt = int64(123)
	const loginPassword = "ValidPassword123"
	hashedPassword, err := common.Password2Hash(loginPassword)
	require.NoError(t, err)
	user := &model.User{
		Username: "session-limit-recovery-user", Password: hashedPassword, Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, LastLoginAt: previousLastLoginAt,
	}
	require.NoError(t, db.Create(user).Error)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.UserSession{
		SID: "existing-active-session", UserID: user.Id, Version: 1, UserAuthVersion: user.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: "hash", LoginMethod: "password",
		CreatedAt: now, LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/user/login", Login)
	requestBody, err := common.Marshal(LoginRequest{Username: user.Username, Password: loginPassword})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken string                   `json:"access_token"`
			TokenType   string                   `json:"token_type"`
			Session     service.LoginSessionView `json:"session"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Data.AccessToken)
	assert.Equal(t, "Bearer", response.Data.TokenType)
	assert.NotEmpty(t, response.Data.Session.SID)

	var refreshCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == service.RefreshCookieName {
			refreshCookie = cookie
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.NotEmpty(t, refreshCookie.Value)

	previousSession, err := model.GetUserSessionBySID("existing-active-session")
	require.NoError(t, err)
	assert.Equal(t, model.UserSessionStatusRevoked, previousSession.Status)
	assert.Equal(t, "session_limit_replaced", previousSession.RevokedReason)
	activeCount, err := model.CountActiveUserSessions(user.Id, time.Now().Unix())
	require.NoError(t, err)
	assert.Equal(t, int64(1), activeCount)
	identity, err := service.ParseAccessToken(response.Data.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, response.Data.Session.SID, identity.SessionID)
	_, _, err = service.ValidateLoginSession(identity)
	require.NoError(t, err)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Greater(t, stored.LastLoginAt, previousLastLoginAt)
	var loginLog model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeLogin).First(&loginLog).Error)
	var auditDetails struct {
		LoginMethod string `json:"login_method"`
		Operation   struct {
			Action string `json:"action"`
			Params struct {
				Method string `json:"method"`
			} `json:"params"`
		} `json:"op"`
	}
	require.NoError(t, common.UnmarshalJsonStr(loginLog.Other, &auditDetails))
	assert.Equal(t, "password", auditDetails.LoginMethod)
	assert.Equal(t, "login", auditDetails.Operation.Action)
	assert.Equal(t, "password", auditDetails.Operation.Params.Method)
}

func TestUpdateSelfPasswordChangeAdvancesCurrentSession(t *testing.T) {
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	model.DB = db
	common.RedisEnabled = false
	common.SessionSecret = "update-self-password-session-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
	})

	hashedPassword, err := common.Password2Hash("CurrentPassword123")
	require.NoError(t, err)
	user := &model.User{
		Username: "password-change-user", Password: hashedPassword, DisplayName: "Before",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "password-change-agent")
	require.NoError(t, err)
	issuedSession, err := model.GetUserSessionBySID(bundle.Session.SID)
	require.NoError(t, err)

	body := []byte(`{"username":"password-change-user","display_name":"After","original_password":"CurrentPassword123","password":"NewPassword123"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/user/self", bytes.NewReader(body))
	c.Set("id", user.Id)
	c.Set("session_id", bundle.Session.SID)
	c.Set("auth_version", int64(1))
	c.Set("session_version", issuedSession.Version)

	UpdateSelf(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken string                   `json:"access_token"`
			Session     service.LoginSessionView `json:"session"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Data.AccessToken)
	assert.Equal(t, bundle.Session.SID, response.Data.Session.SID)

	storedSession, err := model.GetUserSessionBySID(bundle.Session.SID)
	require.NoError(t, err)
	assert.Equal(t, model.UserSessionStatusActive, storedSession.Status)
	assert.EqualValues(t, 2, storedSession.UserAuthVersion)
	assert.EqualValues(t, 2, storedSession.Version)
}
