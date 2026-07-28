package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetModelRequestOpenAIVideoSubmitUsesMultipartModel(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, shouldSelectChannel, err := getModelRequest(c)

	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.Equal(t, "sora-2", request.Model)
	require.Equal(t, relayconstant.RelayModeVideoSubmit, c.GetInt("relay_mode"))
}

func TestGetModelRequestOpenAIVideoFetchUsesStoredOriginModel(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open("file:distributor-video-fetch?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task_video_fetch",
		UserId: 42,
		Properties: model.Properties{
			OriginModelName: "sora-2",
		},
	}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_video_fetch", nil)
	c.Params = gin.Params{{Key: "task_id", Value: "task_video_fetch"}}
	c.Set("id", 42)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)

	request, shouldSelectChannel, err := getModelRequest(c)

	require.NoError(t, err)
	require.False(t, shouldSelectChannel)
	require.Equal(t, "sora-2", request.Model)
	require.Equal(t, relayconstant.RelayModeVideoFetchByID, c.GetInt("relay_mode"))
}
