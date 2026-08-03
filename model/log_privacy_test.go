package model

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMappedModelMetadataIsHiddenFromUserLogsButRetainedForAdmins(t *testing.T) {
	truncateTables(t)
	actualResponseModel := "provider-returned-model"
	originalOther := `{
		"is_model_mapped":true,
		"upstream_model_name":"internal-upstream-model",
		"model_price":0.5,
		"model_ratio":2,
		"group_ratio":1.25
	}`
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:              42,
		CreatedAt:           100,
		Type:                LogTypeConsume,
		ActualResponseModel: &actualResponseModel,
		Other:               originalOther,
	}).Error)

	userLogs, total, err := GetUserLogs(42, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, userLogs, 1)
	assert.Nil(t, userLogs[0].ActualResponseModel)
	var userOther map[string]json.RawMessage
	require.NoError(t, common.UnmarshalJsonStr(userLogs[0].Other, &userOther))
	assert.NotContains(t, userOther, "is_model_mapped")
	assert.NotContains(t, userOther, "upstream_model_name")
	assert.Equal(t, "0.5", string(userOther["model_price"]))
	assert.Equal(t, "2", string(userOther["model_ratio"]))
	assert.Equal(t, "1.25", string(userOther["group_ratio"]))

	adminLogs, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, adminLogs, 1)
	require.NotNil(t, adminLogs[0].ActualResponseModel)
	assert.Equal(t, actualResponseModel, *adminLogs[0].ActualResponseModel)
	var adminOther map[string]json.RawMessage
	require.NoError(t, common.UnmarshalJsonStr(adminLogs[0].Other, &adminOther))
	assert.Equal(t, "true", string(adminOther["is_model_mapped"]))
	assert.JSONEq(t, `"internal-upstream-model"`, string(adminOther["upstream_model_name"]))
	assert.Equal(t, "0.5", string(adminOther["model_price"]))
	assert.Equal(t, "2", string(adminOther["model_ratio"]))
	assert.Equal(t, "1.25", string(adminOther["group_ratio"]))
}

func TestLogActualResponseModelColumnIsNullable(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 1, CreatedAt: 1, Type: LogTypeConsume}).Error)

	var persisted Log
	require.NoError(t, LOG_DB.First(&persisted).Error)
	assert.Nil(t, persisted.ActualResponseModel)
}

func TestRecordConsumeLogPersistsActualResponseModel(t *testing.T) {
	truncateTables(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set("username", "audit-user")

	RecordConsumeLog(c, 7, RecordConsumeLogParams{
		ChannelId:           9,
		ModelName:           "request-model",
		ActualResponseModel: "response-model",
		Other:               map[string]interface{}{},
	})

	var persisted Log
	require.NoError(t, LOG_DB.Where("user_id = ?", 7).First(&persisted).Error)
	require.NotNil(t, persisted.ActualResponseModel)
	assert.Equal(t, "response-model", *persisted.ActualResponseModel)
}
