package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type userPhase1Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserPhase1TestDB(t *testing.T) {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalRedisEnabled := common.RedisEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaForInviter := common.QuotaForInviter
	originalQuotaForInvitee := common.QuotaForInvitee

	t.Cleanup(func() {
		if model.DB != nil {
			sqlDB, err := model.DB.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		common.RedisEnabled = originalRedisEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaForInviter = originalQuotaForInviter
		common.QuotaForInvitee = originalQuotaForInvitee
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gormOpenSQLiteForUserPhase1(dsn)
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
}

func gormOpenSQLiteForUserPhase1(dsn string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
}

func performUserJSONRequest(t *testing.T, handler gin.HandlerFunc, method string, target string, body string, role int) (*httptest.ResponseRecorder, userPhase1Response) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", role)

	handler(ctx)

	var response userPhase1Response
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func TestRegisterTrimsUsernameBeforePersisting(t *testing.T) {
	setupUserPhase1TestDB(t)

	recorder, response := performUserJSONRequest(t, Register, http.MethodPost, "/api/user/register", `{
		"username": "  alice  ",
		"password": "password123"
	}`, common.RoleCommonUser)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "alice").First(&user).Error)
	require.Equal(t, "alice", user.Username)

	var whitespaceCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "  alice  ").Count(&whitespaceCount).Error)
	require.Zero(t, whitespaceCount)
}

func TestRegisterRejectsBlankUsernameAfterTrim(t *testing.T) {
	setupUserPhase1TestDB(t)

	recorder, response := performUserJSONRequest(t, Register, http.MethodPost, "/api/user/register", `{
		"username": "   ",
		"password": "password123"
	}`, common.RoleCommonUser)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, response.Success)

	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateUserTrimsUsernameBeforePersisting(t *testing.T) {
	setupUserPhase1TestDB(t)

	user := model.User{
		Username:    "bob",
		DisplayName: "Bob",
		Password:    "hashed-password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
	}
	require.NoError(t, model.DB.Create(&user).Error)

	recorder, response := performUserJSONRequest(t, UpdateUser, http.MethodPut, "/api/user/", fmt.Sprintf(`{
		"id": %d,
		"username": "  bob-new  ",
		"password": "",
		"display_name": "Bob",
		"role": %d,
		"group": "default"
	}`, user.Id, common.RoleCommonUser), common.RoleAdminUser)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	require.Equal(t, "bob-new", updated.Username)
}
