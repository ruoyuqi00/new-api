package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type groupCatalogResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		Name               string   `json:"name"`
		Ratio              float64  `json:"ratio"`
		Public             bool     `json:"public"`
		Description        string   `json:"description"`
		ActiveChannelCount int      `json:"active_channel_count"`
		ActiveModelCount   int      `json:"active_model_count"`
		ActiveModels       []string `json:"active_models"`
	} `json:"data"`
}

func TestGetGroupCatalogCombinesRatiosVisibilityAndCoverage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":1,"private":0.2,"empty":0.4}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"public":"Public description"}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})

	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 7101, Name: "private-channel", Key: "private-key", Group: "private", Models: "private-a,private-b", Status: common.ChannelStatusEnabled},
		{Id: 7102, Name: "public-channel", Key: "public-key", Group: "public", Models: "public-a", Status: common.ChannelStatusEnabled},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "private", Model: "private-b", ChannelId: 7101, Enabled: true},
		{Group: "private", Model: "private-a", ChannelId: 7101, Enabled: true},
		{Group: "public", Model: "public-a", ChannelId: 7102, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/group/catalog", nil)

	GetGroupCatalog(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response groupCatalogResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 3)

	assert.Equal(t, "empty", response.Data[0].Name)
	assert.False(t, response.Data[0].Public)
	assert.Empty(t, response.Data[0].Description)
	assert.Zero(t, response.Data[0].ActiveChannelCount)
	assert.Zero(t, response.Data[0].ActiveModelCount)
	assert.Empty(t, response.Data[0].ActiveModels)

	assert.Equal(t, "private", response.Data[1].Name)
	assert.Equal(t, 0.2, response.Data[1].Ratio)
	assert.False(t, response.Data[1].Public)
	assert.Empty(t, response.Data[1].Description)
	assert.Equal(t, 1, response.Data[1].ActiveChannelCount)
	assert.Equal(t, 2, response.Data[1].ActiveModelCount)
	assert.Equal(t, []string{"private-a", "private-b"}, response.Data[1].ActiveModels)

	assert.Equal(t, "public", response.Data[2].Name)
	assert.True(t, response.Data[2].Public)
	assert.Equal(t, "Public description", response.Data[2].Description)
	assert.Equal(t, 1, response.Data[2].ActiveChannelCount)
	assert.Equal(t, 1, response.Data[2].ActiveModelCount)
	assert.Equal(t, []string{"public-a"}, response.Data[2].ActiveModels)
}
