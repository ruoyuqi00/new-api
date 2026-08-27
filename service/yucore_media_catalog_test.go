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

func TestBuildYucoreMediaCatalogProjectsGrokImaginePricingAndLimits(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9110)
	ratio_setting.InitRatioSettings()

	require.NoError(t, db.Create(&model.Channel{
		Id: 40, Type: constant.ChannelTypeXai, Name: "grok-imagine", Key: "key-40", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "multimodal", Model: "grok-imagine-image", ChannelId: 40, Enabled: true},
		{Group: "multimodal", Model: "grok-imagine-video", ChannelId: 40, Enabled: true},
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9110)
	require.NoError(t, err)
	require.NotEmpty(t, catalog.Groups)
	require.Len(t, catalog.Groups[0].Models, 2)

	models := make(map[string]YucoreMediaCatalogModel)
	for _, item := range catalog.Groups[0].Models {
		models[item.Id] = item
	}
	assert.Equal(t, "USD", models["grok-imagine-image"].Pricing.Currency)
	assert.InDelta(t, 0.02619*1.5, models["grok-imagine-image"].Pricing.Amount, 0.000000001)
	assert.Equal(t, "$0.039285/image", models["grok-imagine-image"].Pricing.Display)
	assert.Equal(t, []string{"text-to-video", "image-to-video"}, models["grok-imagine-video"].Modes)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, models["grok-imagine-video"].Durations)
	assert.Equal(t, []string{"480p", "720p", "1080p"}, models["grok-imagine-video"].Resolutions)
	assert.Equal(t, "USD", models["grok-imagine-video"].Pricing.Currency)
	assert.InDelta(t, 0.0414*1.5, models["grok-imagine-video"].Pricing.Amount, 0.000000001)
	assert.Equal(t, "$0.0621/second", models["grok-imagine-video"].Pricing.Display)

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"auto":1,"default":1,"multimodal":0.15}`))
	model.InvalidatePricingCache()
	catalog, err = BuildYucoreMediaCatalog(9110)
	require.NoError(t, err)
	models = make(map[string]YucoreMediaCatalogModel)
	for _, item := range catalog.Groups[0].Models {
		models[item.Id] = item
	}
	assert.Equal(t, 0.0039285, models["grok-imagine-image"].Pricing.Amount)
	assert.Equal(t, "$0.0039285/image", models["grok-imagine-image"].Pricing.Display)
	assert.Equal(t, 0.00621, models["grok-imagine-video"].Pricing.Amount)
	assert.Equal(t, "$0.00621/second", models["grok-imagine-video"].Pricing.Display)
}

func TestBuildYucoreMediaCatalogProjectsConfiguredCapabilities(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9104)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.model_capabilities"] = `{"seedance-2.0":{"reference_limits":{"min_video_duration_ms":4000,"max_video_duration_ms":15000,"max_total_video_duration_ms":15000,"max_audio_duration_ms":6000,"max_total_audio_duration_ms":12000,"max_images_with_video":3},"required_reference_kinds":["image"],"disallow_generated_audio_with_frames":true,"require_primary_image_for_media":true}}`
	common.OptionMapRWMutex.Unlock()

	require.NoError(t, db.Create(&model.Channel{
		Id: 31, Type: constant.ChannelTypeSora, Name: "seedance", Key: "key-31", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: " seedance-2.0 ", ChannelId: 31, Enabled: true,
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9104)
	require.NoError(t, err)
	require.NotEmpty(t, catalog.Groups)
	require.Len(t, catalog.Groups[0].Models, 1)
	item := catalog.Groups[0].Models[0]

	assert.Equal(t, "seedance-2.0", item.Id)
	assert.Equal(t, []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, item.Durations)
	assert.Equal(t, []string{"720p"}, item.Resolutions)
	assert.Equal(t, []string{"720p"}, item.Sizes)
	assert.Equal(t, []string{"media"}, item.ReferenceModes)
	assert.True(t, item.SupportsAudio)
	assert.False(t, item.SupportsSeed)
	assert.Equal(t, 5, item.InputLimits.MaxReferenceImages)
	assert.Equal(t, 3, item.InputLimits.MaxReferenceVideos)
	assert.Equal(t, 3, item.InputLimits.MaxReferenceAudios)
	assert.Equal(t, 11, item.InputLimits.MaxReferences)
	assert.Equal(t, 15000, item.InputLimits.MaxReferenceVideoDurationMS)
	assert.Equal(t, 4000, item.InputLimits.MinReferenceVideoDurationMS)
	assert.Equal(t, 15000, item.InputLimits.MaxTotalReferenceVideoDurationMS)
	assert.Equal(t, 6000, item.InputLimits.MaxReferenceAudioDurationMS)
	assert.Equal(t, 12000, item.InputLimits.MaxTotalReferenceAudioDurationMS)
	assert.Equal(t, 3, item.InputLimits.MaxImagesWithVideo)
	assert.Equal(t, []string{"image"}, item.RequiredReferenceKinds)
	assert.True(t, item.DisallowGeneratedAudioWithFrames)
	assert.True(t, item.RequirePrimaryImageForMedia)
	assert.Equal(t, "per_call", item.Pricing.Unit)
}

func TestYucoreMediaImageResolutionOptionsUseCapabilityAsMaximum(t *testing.T) {
	assert.Equal(t, []string{"1k"}, yucoreMediaImageResolutionOptions([]string{"1k"}))
	assert.Equal(t, []string{"1k", "2k"}, yucoreMediaImageResolutionOptions([]string{"2k"}))
	assert.Equal(t, []string{"1k", "2k", "4k"}, yucoreMediaImageResolutionOptions([]string{"4k"}))
	assert.Equal(t, []string{"1024x1024", "1536x1024"}, yucoreMediaImageResolutionOptions([]string{"1024x1024", "1536x1024"}))
}

func TestBuildYucoreMediaCatalogImageSizesExposeLowerTiers(t *testing.T) {
	item := buildYucoreMediaCatalogModel("image-2k", YucoreMediaKindImage, model.YucoreMediaModelCapability{
		Kind:        YucoreMediaKindImage,
		Resolutions: []string{"2k"},
		AspectRatios: []string{
			"1:1", "16:9", "9:16",
		},
	}, true, 1)

	assert.Equal(t, []string{"1k", "2k"}, item.Resolutions)
	assert.Equal(t, []string{"1k", "2k"}, item.Sizes)
}

func TestBuildYucoreMediaCatalogHidesProbeModels(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9105)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.model_capabilities"] = `{"seedance-2.0-mini-8s":{"availability":" PrObE "}}`
	common.OptionMapRWMutex.Unlock()

	require.NoError(t, db.Create(&model.Channel{
		Id: 32, Type: constant.ChannelTypeSora, Name: "probe", Key: "key-32", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: " seedance-2.0-mini-8s ", ChannelId: 32, Enabled: true,
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9105)
	require.NoError(t, err)
	assert.Empty(t, catalog.Groups)
}

func TestBuildYucoreMediaCatalogUsesExplicitPerCallPricingUnit(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9107)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.model_capabilities"] = `{"catalog-per-call":{"kind":"video","pricing_unit":"per_call","transport":"async-task"}}`
	common.OptionMapRWMutex.Unlock()

	require.NoError(t, db.Create(&model.Channel{
		Id: 34, Type: constant.ChannelTypeSora, Name: "per-call", Key: "key-34", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: "catalog-per-call", ChannelId: 34, Enabled: true,
	}).Error)

	catalog, err := BuildYucoreMediaCatalog(9107)
	require.NoError(t, err)
	require.NotEmpty(t, catalog.Groups)
	require.Len(t, catalog.Groups[0].Models, 1)
	assert.Equal(t, "per_call", catalog.Groups[0].Models[0].Pricing.Unit)
}

func TestBuildYucoreMediaCatalogCapabilityProjectionIsolated(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9106)

	require.NoError(t, db.Create(&model.Channel{
		Id: 33, Type: constant.ChannelTypeSora, Name: "seedance", Key: "key-33", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: "seedance-2.0", ChannelId: 33, Enabled: true,
	}).Error)

	first, err := BuildYucoreMediaCatalog(9106)
	require.NoError(t, err)
	require.NotEmpty(t, first.Groups)
	item := &first.Groups[0].Models[0]
	require.NotNil(t, item.capability)
	item.Durations[0] = 999
	item.Resolutions[0] = "mutated"
	item.ReferenceModes[0] = "mutated"
	item.capability.AllowedParameters[0] = "mutated"

	second, err := BuildYucoreMediaCatalog(9106)
	require.NoError(t, err)
	require.NotEmpty(t, second.Groups)
	assert.Equal(t, []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, second.Groups[0].Models[0].Durations)
	assert.Equal(t, []string{"720p"}, second.Groups[0].Models[0].Resolutions)
	assert.Equal(t, []string{"media"}, second.Groups[0].Models[0].ReferenceModes)
	require.NotNil(t, second.Groups[0].Models[0].capability)
	assert.NotEqual(t, "mutated", second.Groups[0].Models[0].capability.AllowedParameters[0])
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

func TestResolveYucoreMediaSelectionKeepsCapabilitySnapshot(t *testing.T) {
	db := setupYucoreMediaCatalogTest(t)
	createYucoreMediaCatalogUser(t, db, 9108)
	require.NoError(t, db.Create(&model.Channel{
		Id: 35, Type: constant.ChannelTypeSora, Name: "snapshot", Key: "key-35", Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "multimodal", Model: "seedance-2.0", ChannelId: 35, Enabled: true,
	}).Error)

	_, selected, err := ResolveYucoreMediaSelection(9108, "multimodal", "seedance-2.0", YucoreMediaKindVideo)
	require.NoError(t, err)
	browserJSON, err := common.Marshal(selected)
	require.NoError(t, err)
	assert.NotContains(t, string(browserJSON), "capability")
	assert.NotContains(t, string(browserJSON), "create_path")
	assert.NotContains(t, string(browserJSON), "upstream_cost")

	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.model_capabilities"] = `{"seedance-2.0":{"allowed_parameters":[]}}`
	common.OptionMapRWMutex.Unlock()
	_, refreshed := model.GetYucoreMediaCatalogSettings()
	assert.Empty(t, refreshed["seedance-2.0"].AllowedParameters)
	require.NotNil(t, selected.capability)
	assert.Contains(t, selected.capability.AllowedParameters, "duration")

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Duration: intPointer(4)})
	require.NoError(t, err)
	require.NotNil(t, normalized.Duration)
	assert.Equal(t, 4, *normalized.Duration)
}

func TestValidateYucoreMediaRequestUsesReportedCapabilities(t *testing.T) {
	selected := YucoreMediaCatalogModel{
		Id:     "image-live",
		Kind:   YucoreMediaKindImage,
		Modes:  []string{"text-to-image", "image-to-image"},
		Counts: []int{1, 2},
		InputLimits: YucoreMediaCatalogInputLimits{
			MaxReferenceImages: 1,
		},
	}

	mode, count, err := ValidateYucoreMediaRequest(selected, "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, "text-to-image", mode)
	assert.Equal(t, 1, count)

	_, _, err = ValidateYucoreMediaRequest(selected, "text-to-video", 1, 0)
	require.ErrorContains(t, err, "does not support mode")

	_, _, err = ValidateYucoreMediaRequest(selected, "text-to-image", 3, 0)
	require.ErrorContains(t, err, "does not support count")

	_, _, err = ValidateYucoreMediaRequest(selected, "image-to-image", 1, 2)
	require.ErrorContains(t, err, "supports at most 1 reference image")
}
