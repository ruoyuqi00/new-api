package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupReadonlyAuthTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	originalRedisEnabled := common.RedisEnabled

	t.Cleanup(func() {
		if model.DB != nil {
			sqlDB, err := model.DB.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		common.RedisEnabled = originalRedisEnabled
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}))
}

func TestTokenAuthReadOnlyRejectsDisabledToken(t *testing.T) {
	setupReadonlyAuthTestDB(t)

	user := model.User{
		Username: "readonly-user",
		Password: "hashed-password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(&user).Error)

	token := model.Token{
		UserId:         user.Id,
		Name:           "disabled",
		Key:            "disabled",
		Status:         common.TokenStatusDisabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, model.DB.Create(&token).Error)

	router := gin.New()
	router.Use(TokenAuthReadOnly())
	router.GET("/readonly", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readonly", nil)
	request.Header.Set("Authorization", "Bearer sk-disabled")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
}

func TestTokenAuthReadOnlyAllowsExpiredNonDisabledToken(t *testing.T) {
	setupReadonlyAuthTestDB(t)

	user := model.User{
		Username: "readonly-compatible-user",
		Password: "hashed-password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, model.DB.Create(&user).Error)

	token := model.Token{
		UserId:         user.Id,
		Name:           "expired",
		Key:            "expired",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    1,
		RemainQuota:    0,
		UnlimitedQuota: false,
		Group:          "default",
	}
	require.NoError(t, model.DB.Create(&token).Error)

	router := gin.New()
	router.Use(TokenAuthReadOnly())
	router.GET("/readonly", func(c *gin.Context) {
		require.Equal(t, user.Id, c.GetInt("id"))
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readonly", nil)
	request.Header.Set("Authorization", "Bearer sk-expired")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}
