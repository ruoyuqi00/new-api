package operation_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func completeH3VideoTierPrices(standard ...float64) map[string]VideoTierPricePoint {
	return map[string]VideoTierPricePoint{
		"480p":  {Standard: standard[0]},
		"768p":  {Standard: standard[1]},
		"1080p": {Standard: standard[2]},
		"2k":    {Standard: standard[3]},
		"4k":    {Standard: standard[4]},
	}
}

func replaceVideoTierPriceSettingForTest(t *testing.T, models map[string]map[string]VideoTierPricePoint) {
	t.Helper()
	require.NoError(t, validateVideoTierPriceModels(models))
	videoTierPriceSetting.Models = models
	RebuildVideoTierPriceIndex()
}

func preserveVideoTierPriceSetting(t *testing.T) {
	t.Helper()
	original := VideoTierPriceSetting2JSONString()
	t.Cleanup(func() {
		var models map[string]map[string]VideoTierPricePoint
		require.NoError(t, common.UnmarshalJsonStr(original, &models))
		replaceVideoTierPriceSettingForTest(t, models)
	})
}

func TestResolveVideoTierPriceUsesCompleteOverrideOrInheritedRates(t *testing.T) {
	preserveVideoTierPriceSetting(t)
	replaceVideoTierPriceSettingForTest(t, map[string]map[string]VideoTierPricePoint{
		"minimax-h3": completeH3VideoTierPrices(0.11, 0.17, 0.21, 0.31, 0.47),
	})

	explicit, configured, err := ResolveVideoTierPrice("minimax-h3", "1920x1088", false, 0.10)
	require.NoError(t, err)
	require.True(t, configured)
	assert.Equal(t, "1080p", explicit.Tier)
	assert.InDelta(t, 0.21, explicit.UnitPrice, 1e-12)
	assert.False(t, explicit.Inherited)

	inherited, configured, err := ResolveVideoTierPrice("wan3.0-video", "1080p", false, 0.30)
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.30*(0.72/0.27), inherited.UnitPrice, 1e-12)
	assert.True(t, inherited.Inherited)
}

func TestResolveVideoTierPriceSelectsReferenceProfileAndAliases(t *testing.T) {
	preserveVideoTierPriceSetting(t)
	withReference480 := 7.5
	withReference720 := 8.5
	replaceVideoTierPriceSettingForTest(t, map[string]map[string]VideoTierPricePoint{
		"seedance-2-0-mini-official": {
			"480p": {Standard: 12.5, WithReferenceVideo: &withReference480},
			"720p": {Standard: 13.5, WithReferenceVideo: &withReference720},
		},
	})

	standard, configured, err := ResolveVideoTierPrice("seedance-2-0-mini-official", "480P", false, 11.5)
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 12.5, standard.UnitPrice, 1e-12)
	assert.False(t, standard.HasReferenceVideo)

	reference, configured, err := ResolveVideoTierPrice("seedance-2-0-mini-official", "720p", true, 11.5)
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 8.5, reference.UnitPrice, 1e-12)
	assert.True(t, reference.HasReferenceVideo)

	h3, configured, err := ResolveVideoTierPrice("minimax-h3", "1088x1920", false, 0.10)
	require.NoError(t, err)
	require.True(t, configured)
	assert.Equal(t, "1080p", h3.Tier)
}

func TestGetVideoTierPricingMetadataReturnsEffectiveDefensiveCopy(t *testing.T) {
	preserveVideoTierPriceSetting(t)
	replaceVideoTierPriceSettingForTest(t, map[string]map[string]VideoTierPricePoint{})

	metadata, configured := GetVideoTierPricingMetadata("minimax-h3", 0.20)
	require.True(t, configured)
	assert.Equal(t, VideoBillingUnitPerSecond, metadata.BillingUnit)
	assert.True(t, metadata.Inherited)
	assert.Equal(t, []string{"480p", "768p", "1080p", "2k", "4k"}, metadata.Tiers)
	assert.InDelta(t, 0.20, metadata.Prices["480p"].Standard, 1e-12)
	assert.InDelta(t, 0.72, metadata.Prices["4k"].Standard, 1e-12)

	metadata.Prices["480p"] = VideoTierPricePoint{Standard: 99}
	fresh, configured := GetVideoTierPricingMetadata("minimax-h3", 0.20)
	require.True(t, configured)
	assert.InDelta(t, 0.20, fresh.Prices["480p"].Standard, 1e-12)
}

func TestValidateVideoTierPriceModelsRejectsIncompleteAndUnsafeOverrides(t *testing.T) {
	withReference := 7.0
	tests := []struct {
		name   string
		models map[string]map[string]VideoTierPricePoint
	}{
		{name: "unknown model", models: map[string]map[string]VideoTierPricePoint{"unknown-video": {"720p": {Standard: 1}}}},
		{name: "missing tier", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": {"480p": {Standard: 0.1}}}},
		{name: "unknown tier", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": {"480p": {Standard: 0.1}, "768p": {Standard: 0.16}, "1080p": {Standard: 0.18}, "2k": {Standard: 0.26}, "4k": {Standard: 0.36}, "8k": {Standard: 0.5}}}},
		{name: "zero", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": completeH3VideoTierPrices(0, 0.16, 0.18, 0.26, 0.36)}},
		{name: "negative", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": completeH3VideoTierPrices(0.1, -0.16, 0.18, 0.26, 0.36)}},
		{name: "nan", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": completeH3VideoTierPrices(0.1, 0.16, math.NaN(), 0.26, 0.36)}},
		{name: "infinity", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": completeH3VideoTierPrices(0.1, 0.16, 0.18, math.Inf(1), 0.36)}},
		{name: "unexpected reference price", models: map[string]map[string]VideoTierPricePoint{"minimax-h3": {"480p": {Standard: 0.1, WithReferenceVideo: &withReference}, "768p": {Standard: 0.16}, "1080p": {Standard: 0.18}, "2k": {Standard: 0.26}, "4k": {Standard: 0.36}}}},
		{name: "missing required reference price", models: map[string]map[string]VideoTierPricePoint{"seedance-2-0-mini-official": {"480p": {Standard: 11.5}, "720p": {Standard: 11.5}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, validateVideoTierPriceModels(test.models))
		})
	}
}

func TestValidateVideoTierPriceJSONStringAcceptsEmptyAndCompleteOverrides(t *testing.T) {
	require.NoError(t, ValidateVideoTierPriceJSONString(`{}`))
	require.NoError(t, ValidateVideoTierPriceJSONString(`{
		"minimax-h3": {
			"480p":{"standard":0.11},
			"768p":{"standard":0.17},
			"1080p":{"standard":0.21},
			"2k":{"standard":0.31},
			"4k":{"standard":0.47}
		}
	}`))
}

func TestVideoTierPriceSettingPreservesCanonicalPublicModelNames(t *testing.T) {
	preserveVideoTierPriceSetting(t)
	replaceVideoTierPriceSettingForTest(t, map[string]map[string]VideoTierPricePoint{
		"seedance2.0-fast-PT": {
			"480p": {Standard: 0.31},
			"720p": {Standard: 0.39},
		},
	})

	serialized := VideoTierPriceSetting2JSONString()
	assert.Contains(t, serialized, `"seedance2.0-fast-PT"`)
	assert.NotContains(t, serialized, `"seedance2.0-fast-pt"`)
}
