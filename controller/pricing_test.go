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

type privatePricingResponse struct {
	Success     bool               `json:"success"`
	Data        []model.Pricing    `json:"data"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string  `json:"usable_group"`
}

func withPrivatePricingSettings(t *testing.T, special map[string]map[string]string) {
	t.Helper()

	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	originalSpecial := specialGroups.ReadAll()

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":1,"private":0.2,"partner":0.8}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"public":"Public group"}`))
	specialGroups.Clear()
	specialGroups.AddAll(special)

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		specialGroups.Clear()
		specialGroups.AddAll(originalSpecial)
	})
}

func performPrivatePricingRequest(t *testing.T, userID int) privatePricingResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	if userID > 0 {
		ctx.Set("id", userID)
	}

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response privatePricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestGetPricingHidesPrivateGroupFromAnonymousRequest(t *testing.T) {
	setupModelListControllerTestDB(t)
	withPrivatePricingSettings(t, nil)

	response := performPrivatePricingRequest(t, 0)

	assert.Contains(t, response.GroupRatio, "public")
	assert.NotContains(t, response.GroupRatio, "private")
	assert.Contains(t, response.UsableGroup, "public")
	assert.NotContains(t, response.UsableGroup, "private")
	for _, pricing := range response.Data {
		assert.False(t, len(pricing.EnableGroup) == 1 && pricing.EnableGroup[0] == "private")
	}
}

func TestGetPricingIncludesPrivateGroupForAuthorizedUser(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	withPrivatePricingSettings(t, nil)
	require.NoError(t, db.Create(&model.User{
		Id:       7201,
		Username: "private-pricing-user",
		Password: "password",
		Group:    "private",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)

	response := performPrivatePricingRequest(t, 7201)

	assert.Equal(t, 0.2, response.GroupRatio["private"])
	assert.Contains(t, response.UsableGroup, "private")
}

func TestGetPricingIncludesSpecialRulePrivateGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	withPrivatePricingSettings(t, map[string]map[string]string{
		"partner": {
			"+:private": "Authorized private group",
		},
	})
	require.NoError(t, db.Create(&model.User{
		Id:       7202,
		Username: "partner-pricing-user",
		Password: "password",
		Group:    "partner",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)

	response := performPrivatePricingRequest(t, 7202)

	assert.Equal(t, 0.2, response.GroupRatio["private"])
	assert.Equal(t, "Authorized private group", response.UsableGroup["private"])
}

func TestFilterPricingByUsableGroupsExcludesPrivateOnlyModels(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "public-model", EnableGroup: []string{"public"}},
		{ModelName: "private-model", EnableGroup: []string{"private"}},
		{ModelName: "shared-model", EnableGroup: []string{"all"}},
	}

	filtered := filterPricingByUsableGroups(pricing, map[string]string{"public": "Public group"})

	require.Len(t, filtered, 2)
	assert.Equal(t, "public-model", filtered[0].ModelName)
	assert.Equal(t, "shared-model", filtered[1].ModelName)
}
