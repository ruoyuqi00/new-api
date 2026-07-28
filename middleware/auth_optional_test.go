package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionalAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newOptionalAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/optional", TryUserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":               c.GetInt("id"),
			"group":            c.GetString("group"),
			"use_access_token": c.GetBool("use_access_token"),
		})
	})
	return router
}

func TestTryUserAuthUsesDashboardAccessToken(t *testing.T) {
	db := setupOptionalAuthTestDB(t)
	accessToken := "dashboard-pat-private"
	require.NoError(t, db.Create(&model.User{
		Id:          2001,
		Username:    "private-user",
		Password:    "password",
		Group:       "private",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AccessToken: &accessToken,
	}).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	newOptionalAuthRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":2001,"group":"private","use_access_token":true}`, recorder.Body.String())
}

func TestTryUserAuthKeepsUnmatchedOpaqueCredentialAnonymous(t *testing.T) {
	setupOptionalAuthTestDB(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	request.Header.Set("Authorization", "Bearer invalid-dashboard-pat")
	newOptionalAuthRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":0,"group":"","use_access_token":false}`, recorder.Body.String())
}

func TestTryUserAuthKeepsCredentialFreeRequestAnonymous(t *testing.T) {
	setupOptionalAuthTestDB(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	newOptionalAuthRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":0,"group":"","use_access_token":false}`, recorder.Body.String())
}

func TestTryUserAuthKeepsSessionAuthentication(t *testing.T) {
	db := setupOptionalAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:          2002,
		Username:    "session-user",
		Password:    "password",
		Group:       "session-group",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AuthVersion: 1,
	}).Error)
	bundle, err := service.CreateLoginSession(2002, "password", "127.0.0.1", "optional-auth-test")
	require.NoError(t, err)
	router := newOptionalAuthRouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":2002,"group":"session-group","use_access_token":false}`, recorder.Body.String())
}
