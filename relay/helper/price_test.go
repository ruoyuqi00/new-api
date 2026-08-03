package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
