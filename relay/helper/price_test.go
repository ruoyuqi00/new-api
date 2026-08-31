package helper

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureImageResolutionPriceTestRatios(t *testing.T) {
	t.Helper()
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":9,"gpt-image-2-4k":9,"unmanaged-image-model":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"image-test":0.3,"legacy-image":0.5}`))
}

func TestResolveEffectiveGroupPrefersAutoGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("auto_group", "final")
	info := &relaycommon.RelayInfo{UsingGroup: "initial"}

	assert.Equal(t, "final", ResolveEffectiveGroup(ctx, info))
	assert.Equal(t, "initial", info.UsingGroup)
}

func TestResolveEffectiveGroupFallsBackToUsingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UsingGroup: "initial"}

	assert.Equal(t, "initial", ResolveEffectiveGroup(ctx, info))
}

func TestHandleGroupRatioReconcilesTieredSnapshotAfterAutoGroupSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"initial":0.2,"final":0.045}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("auto_group", "final")
	info := &relaycommon.RelayInfo{
		UserGroup:  "initial",
		UsingGroup: "initial",
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			GroupRatio:                0.2,
			EstimatedQuotaBeforeGroup: 1000,
			EstimatedQuotaAfterGroup:  200,
		},
	}

	groupRatioInfo := HandleGroupRatio(ctx, info)

	require.Equal(t, "final", info.UsingGroup)
	require.Equal(t, 0.045, groupRatioInfo.GroupRatio)
	require.Equal(t, 0.045, info.TieredBillingSnapshot.GroupRatio)
	require.Equal(t, 45, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
}

func TestModelPriceHelperFixedPriceAppliesRequestBillingRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ratio_setting.InitRatioSettings()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	meta := &types.TokenCountMeta{
		ImagePriceRatio: 1.6,
		BillingRatios:   map[string]float64{"n": 3},
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, meta)

	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 3.0, priceData.OtherRatios["n"])
	want, err := common.QuotaFromFloatStrict(priceData.ModelPrice * common.QuotaPerUnit * 3)
	require.NoError(t, err)
	require.Equal(t, want, priceData.QuotaToPreConsume)
}

func TestModelPriceHelperUsesResolutionPolicyAndFreezesQuote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureImageResolutionPriceTestRatios(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	n := uint(2)
	request := &dto.ImageRequest{Model: "gpt-image-2", Size: "650x1024", N: &n}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		UserGroup:       "image-test",
		UsingGroup:      "image-test",
		Request:         request,
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	assert.True(t, priceData.UsePrice)
	assert.InDelta(t, 0.01, priceData.ModelPrice, 1e-12)
	assert.Equal(t, "gpt-image-2", priceData.ImageResolutionPricingModel)
	assert.Equal(t, "650x1024", priceData.ImageResolutionRequestedSize)
	assert.Equal(t, "1k", priceData.ImageResolutionTier)
	assert.InDelta(t, 0.01, priceData.ImageResolutionUnitPrice, 1e-12)
	assert.Equal(t, 2, priceData.ImageResolutionImageCount)
	assert.Equal(t, 2.0, priceData.OtherRatios["n"])
	assert.Equal(t, int(math.Round(0.01*common.QuotaPerUnit*0.3*2)), priceData.QuotaToPreConsume)
}

func TestModelPriceHelperResolutionAliasUsesMinimumTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureImageResolutionPriceTestRatios(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	request := &dto.ImageRequest{Model: "gpt-image-2-4k", Size: "1024x1024"}
	info := &relaycommon.RelayInfo{
		OriginModelName: request.Model,
		UserGroup:       "default",
		UsingGroup:      "default",
		Request:         request,
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	assert.Equal(t, "4k", priceData.ImageResolutionTier)
	assert.InDelta(t, 0.045, priceData.ModelPrice, 1e-12)
}

func TestModelPriceHelperRejectsInvalidConfiguredImageSizeBeforePreConsume(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureImageResolutionPriceTestRatios(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	request := &dto.ImageRequest{Model: "gpt-image-2", Size: "4097x512"}
	info := &relaycommon.RelayInfo{
		OriginModelName: request.Model,
		UserGroup:       "default",
		UsingGroup:      "default",
		Request:         request,
	}

	_, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.ErrorContains(t, err, "4097x512")
	assert.Zero(t, info.PriceData.QuotaToPreConsume)
}

func TestModelPriceHelperUnconfiguredImageModelKeepsLegacyRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureImageResolutionPriceTestRatios(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	request := &dto.ImageRequest{Model: "unmanaged-image-model", Size: "1536x1024"}
	info := &relaycommon.RelayInfo{
		OriginModelName: request.Model,
		UserGroup:       "legacy-image",
		UsingGroup:      "legacy-image",
		Request:         request,
	}
	meta := &types.TokenCountMeta{ImagePriceRatio: 1.5, BillingRatios: map[string]float64{"n": 2}}

	priceData, err := ModelPriceHelper(ctx, info, 0, meta)
	require.NoError(t, err)
	want, err := common.QuotaFromFloatStrict(0.2 * 1.5 * common.QuotaPerUnit * 0.5 * 2)
	require.NoError(t, err)
	assert.Equal(t, want, priceData.QuotaToPreConsume)
	assert.Empty(t, priceData.ImageResolutionPricingModel)
}

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperTieredPreConsumeMaxTokensFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"tiered-fallback-model":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"tiered-fallback-model":"tier(\"base\", p * 3 + c * 15)"}`,
		"group_ratio_setting.group_ratio": `{"default":1,"free":0}`,
	}))

	const promptTokens = 1000

	cases := []struct {
		name      string
		group     string
		maxTokens int
		expected  int
	}{
		{
			name:      "non-free group falls back to default completion tokens",
			group:     "default",
			maxTokens: 0,
			expected:  62940,
		},
		{
			name:      "explicit max tokens are used verbatim",
			group:     "default",
			maxTokens: 100,
			expected:  2250,
		},
		{
			name:      "free group stays zero without fallback",
			group:     "free",
			maxTokens: 0,
			expected:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("Content-Type", "application/json")
			ctx.Request = req
			ctx.Set("group", tc.group)

			info := &relaycommon.RelayInfo{
				OriginModelName: "tiered-fallback-model",
				UserGroup:       tc.group,
				UsingGroup:      tc.group,
				RequestHeaders:  map[string]string{"Content-Type": "application/json"},
				BillingRequestInput: &billingexpr.RequestInput{
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    []byte(`{}`),
				},
			}

			priceData, err := ModelPriceHelper(ctx, info, promptTokens, &types.TokenCountMeta{MaxTokens: tc.maxTokens})
			require.NoError(t, err)
			require.Equal(t, tc.expected, priceData.QuotaToPreConsume)
		})
	}
}

func TestMappedModelBillingUsesPublicModelAcrossBillingModes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedConfig := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		savedConfig[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{
		"public-fixed":1.3,
		"public-per-call":2.6,
		"private-upstream":0.05
	}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"public-tiered":"tiered_expr","private-upstream":"tiered_expr"}`,
		"billing_setting.billing_expr": `{
			"public-tiered":"tier(\"public\", p * 10)",
			"private-upstream":"tier(\"upstream\", p * 0.1)"
		}`,
	}))

	newContext := func(path string) *gin.Context {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("group", "default")
		return ctx
	}
	newInfo := func(originModel string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			OriginModelName: originModel,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "private-upstream",
				IsModelMapped:     true,
			},
			UserGroup:      "default",
			UsingGroup:     "default",
			RequestHeaders: map[string]string{"Content-Type": "application/json"},
			BillingRequestInput: &billingexpr.RequestInput{
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    []byte(`{}`),
			},
		}
	}

	fixedPrice, err := ModelPriceHelper(
		newContext("/v1/chat/completions"),
		newInfo("public-fixed"),
		0,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.Equal(t, 1.3, fixedPrice.ModelPrice)
	wantFixedQuota, err := common.QuotaFromFloatStrict(1.3 * common.QuotaPerUnit)
	require.NoError(t, err)
	require.Equal(t, wantFixedQuota, fixedPrice.QuotaToPreConsume)

	perCallPrice, err := ModelPriceHelperPerCall(
		newContext("/v1/videos"),
		newInfo("public-per-call"),
	)
	require.NoError(t, err)
	require.Equal(t, 2.6, perCallPrice.ModelPrice)
	wantPerCallQuota, err := common.QuotaFromFloatStrict(2.6 * common.QuotaPerUnit)
	require.NoError(t, err)
	require.Equal(t, wantPerCallQuota, perCallPrice.Quota)

	tieredInfo := newInfo("public-tiered")
	tieredPrice, err := ModelPriceHelper(
		newContext("/v1/responses"),
		tieredInfo,
		1000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	wantTieredQuota, err := billingexpr.QuotaRoundStrict(
		float64(1000) * 10 / 1_000_000 * common.QuotaPerUnit,
	)
	require.NoError(t, err)
	require.Equal(t, wantTieredQuota, tieredPrice.QuotaToPreConsume)
	require.NotNil(t, tieredInfo.TieredBillingSnapshot)
	require.Equal(t, "public-tiered", tieredInfo.TieredBillingSnapshot.ModelName)
	require.Equal(t, "public", tieredInfo.TieredBillingSnapshot.EstimatedTier)
}

func TestDomesticTieredPricingUsesGroupRatioOnceAndCallTierIgnoresOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedConfig := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		savedConfig[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"国模按量":0.3,"国模按次":0.3}`))
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"MiniMax-M2.7":"tiered_expr","MiniMax-M2.7-call":"per_call_expr"}`,
		"billing_setting.billing_expr": `{"MiniMax-M2.7":"tier(\"base\", p * 2.1 + c * 8.4 + cr * 0.42 + cc * 2.625)","MiniMax-M2.7-call":"len <= 128000 ? tier(\"short\", 0.05) : tier(\"long\", 0.1)"}`,
	}))

	newContext := func(path string) *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
		ctx.Request.Header.Set("Content-Type", "application/json")
		return ctx
	}

	usageInfo := &relaycommon.RelayInfo{
		OriginModelName: "MiniMax-M2.7",
		UserGroup:       "default",
		UsingGroup:      "国模按量",
	}
	usagePrice, err := ModelPriceHelper(newContext("/v1/chat/completions"), usageInfo, 1_000_000, &types.TokenCountMeta{MaxTokens: 1_000_000})
	require.NoError(t, err)
	expectedUsageQuota, err := common.QuotaFromFloatStrict((2.1 + 8.4) * common.QuotaPerUnit * 0.3)
	require.NoError(t, err)
	require.Equal(t, expectedUsageQuota, usagePrice.QuotaToPreConsume)

	callInfo := &relaycommon.RelayInfo{
		OriginModelName: "MiniMax-M2.7-call",
		UserGroup:       "default",
		UsingGroup:      "国模按次",
	}
	shortCall, err := ModelPriceHelper(newContext("/v1/chat/completions"), callInfo, 64_000, &types.TokenCountMeta{MaxTokens: 1_000_000})
	require.NoError(t, err)
	expectedShortQuota, err := common.QuotaFromFloatStrict(0.05 * common.QuotaPerUnit * 0.3)
	require.NoError(t, err)
	require.Equal(t, expectedShortQuota, shortCall.QuotaToPreConsume)
	require.NotNil(t, callInfo.TieredBillingSnapshot)
	require.Equal(t, "per_call_expr", callInfo.TieredBillingSnapshot.BillingMode)

	longCall, err := ModelPriceHelper(newContext("/v1/chat/completions"), &relaycommon.RelayInfo{
		OriginModelName: "MiniMax-M2.7-call",
		UserGroup:       "default",
		UsingGroup:      "国模按次",
	}, 64_000, &types.TokenCountMeta{MaxTokens: 1})
	require.NoError(t, err)
	require.Equal(t, shortCall.QuotaToPreConsume, longCall.QuotaToPreConsume)

	longContext, err := ModelPriceHelper(newContext("/v1/chat/completions"), &relaycommon.RelayInfo{
		OriginModelName: "MiniMax-M2.7-call",
		UserGroup:       "default",
		UsingGroup:      "国模按次",
	}, 128_001, &types.TokenCountMeta{})
	require.NoError(t, err)
	expectedLongQuota, err := common.QuotaFromFloatStrict(0.1 * common.QuotaPerUnit * 0.3)
	require.NoError(t, err)
	require.Equal(t, expectedLongQuota, longContext.QuotaToPreConsume)
}
