package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupYucoreMediaCatalogTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}))
	model.DB = db
	model.LOG_DB = db

	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = make(map[string]string)
	}
	originalManagedGroup, hadManagedGroup := common.OptionMap["yucore_media.managed_token_group"]
	originalCapabilities, hadCapabilities := common.OptionMap["yucore_media.model_capabilities"]
	common.OptionMap["yucore_media.managed_token_group"] = "multimodal"
	common.OptionMap["yucore_media.model_capabilities"] = "{}"
	common.OptionMapRWMutex.Unlock()

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"auto":"Auto","default":"Default","multimodal":"Media"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["multimodal","default"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"auto":1,"default":1,"multimodal":1.5}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	model.InvalidatePricingCache()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if hadManagedGroup {
			common.OptionMap["yucore_media.managed_token_group"] = originalManagedGroup
		} else {
			delete(common.OptionMap, "yucore_media.managed_token_group")
		}
		if hadCapabilities {
			common.OptionMap["yucore_media.model_capabilities"] = originalCapabilities
		} else {
			delete(common.OptionMap, "yucore_media.model_capabilities")
		}
		if optionMapWasNil {
			common.OptionMap = nil
		}
		common.OptionMapRWMutex.Unlock()

		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(originalSpecialGroups)
		model.InvalidatePricingCache()
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

func createYucoreMediaCatalogUser(t *testing.T, db *gorm.DB, id int) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: fmt.Sprintf("media-user-%d", id),
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
}

func TestBuildYucoreMediaCatalogUsesOnlyActiveMediaRoutes(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9101)

	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 1, Type: constant.ChannelTypeOpenAI, Name: "images", Key: "key-1", Status: common.ChannelStatusEnabled},
		{Id: 2, Type: constant.ChannelTypeSora, Name: "videos", Key: "key-2", Status: common.ChannelStatusEnabled},
		{Id: 3, Type: constant.ChannelTypeOpenAI, Name: "disabled", Key: "key-3", Status: common.ChannelStatusManuallyDisabled},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "multimodal", Model: "gpt-image-1", ChannelId: 1, Enabled: true},
		{Group: "multimodal", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "multimodal", Model: "sora-2", ChannelId: 2, Enabled: true},
		{Group: "multimodal", Model: "gpt-image-2", ChannelId: 3, Enabled: true},
		{Group: "multimodal", Model: "gpt-image-disabled-ability", ChannelId: 1, Enabled: false},
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9101)
	require.NoError(t, err)
	require.Equal(t, "multimodal", catalog.DefaultGroup)
	require.NotEmpty(t, catalog.Groups)
	require.Equal(t, "multimodal", catalog.Groups[0].Id)
	require.Len(t, catalog.Groups[0].Models, 2)

	assert.Equal(t, "gpt-image-1", catalog.Groups[0].Models[0].Id)
	assert.Equal(t, YucoreMediaKindImage, catalog.Groups[0].Models[0].Kind)
	assert.Equal(t, "sora-2", catalog.Groups[0].Models[1].Id)
	assert.Equal(t, YucoreMediaKindVideo, catalog.Groups[0].Models[1].Kind)
}

func TestBuildYucoreMediaCatalogExpandsAutoGroupsInConfiguredOrder(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9102)

	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.managed_token_group"] = "missing"
	common.OptionMapRWMutex.Unlock()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"auto":"Auto","alpha":"Alpha","beta":"Beta","default":"Default"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["beta","alpha"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"auto":1,"alpha":1,"beta":2,"default":1}`))

	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 11, Type: constant.ChannelTypeOpenAI, Name: "alpha-images", Key: "key-11", Status: common.ChannelStatusEnabled},
		{Id: 12, Type: constant.ChannelTypeSora, Name: "beta-videos", Key: "key-12", Status: common.ChannelStatusEnabled},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "alpha", Model: "gpt-image-1", ChannelId: 11, Enabled: true},
		{Group: "beta", Model: "gpt-image-1", ChannelId: 11, Enabled: true},
		{Group: "beta", Model: "sora-2", ChannelId: 12, Enabled: true},
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9102)
	require.NoError(t, err)
	require.Equal(t, "auto", catalog.DefaultGroup)
	require.Len(t, catalog.Groups, 3)
	assert.Equal(t, []string{"auto", "alpha", "beta"}, []string{
		catalog.Groups[0].Id,
		catalog.Groups[1].Id,
		catalog.Groups[2].Id,
	})
	require.Len(t, catalog.Groups[0].Models, 2)
	assert.Equal(t, "gpt-image-1", catalog.Groups[0].Models[0].Id)
	assert.Equal(t, "sora-2", catalog.Groups[0].Models[1].Id)
}

func TestResolveYucoreMediaSelectionValidatesGroupModelAndKind(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9103)

	require.NoError(t, db.Create(&model.Channel{
		Id: 21, Type: constant.ChannelTypeOpenAI, Name: "images", Key: "key-21", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: "gpt-image-1", ChannelId: 21, Enabled: true,
	}).Error)

	group, selected, err := ResolveYucoreMediaSelection(9103, "", "gpt-image-1", YucoreMediaKindImage)
	require.NoError(t, err)
	assert.Equal(t, "multimodal", group)
	assert.Equal(t, "gpt-image-1", selected.Id)

	group, selected, err = ResolveYucoreMediaSelection(9103, "", "", YucoreMediaKindImage)
	require.NoError(t, err)
	assert.Equal(t, "multimodal", group)
	assert.Equal(t, "gpt-image-1", selected.Id)

	_, _, err = ResolveYucoreMediaSelection(9103, "unavailable", "gpt-image-1", YucoreMediaKindImage)
	require.ErrorContains(t, err, "group unavailable is not available")

	_, _, err = ResolveYucoreMediaSelection(9103, "multimodal", "gpt-image-1", YucoreMediaKindVideo)
	require.ErrorContains(t, err, "not available for video generation")
}
