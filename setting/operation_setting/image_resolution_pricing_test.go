package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func replaceImageResolutionPriceSettingForTest(t *testing.T, value string) {
	t.Helper()
	var models map[string]ImageResolutionPricePolicy
	require.NoError(t, common.UnmarshalJsonStr(value, &models))
	require.NoError(t, validateImageResolutionPricePolicies(models))
	imageResolutionPriceSetting.Models = models
	RebuildImageResolutionPriceIndex()
}

func preserveImageResolutionPriceSetting(t *testing.T) {
	t.Helper()
	original := ImageResolutionPriceSetting2JSONString()
	t.Cleanup(func() {
		replaceImageResolutionPriceSettingForTest(t, original)
	})
}

func TestResolveImageResolutionPriceClassifiesDimensions(t *testing.T) {
	tests := []struct {
		name string
		size string
		want ImageResolutionTier
	}{
		{name: "1k lower rectangle", size: "650x1024", want: ImageResolutionTier1K},
		{name: "1k uppercase separator", size: "1024X650", want: ImageResolutionTier1K},
		{name: "2k star separator", size: "1024*1536", want: ImageResolutionTier2K},
		{name: "2k unicode separator", size: "2048\u00d72048", want: ImageResolutionTier2K},
		{name: "4k rectangle", size: "2048x3072", want: ImageResolutionTier4K},
		{name: "4k narrow", size: "512x4096", want: ImageResolutionTier4K},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, configured, err := ResolveImageResolutionPrice("gpt-image-2", tt.size)
			require.NoError(t, err)
			require.True(t, configured)
			assert.Equal(t, tt.want, quote.Tier)
		})
	}
}

func TestResolveImageResolutionPriceHonorsDefaultAndAliasMinimum(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		size      string
		wantModel string
		wantTier  ImageResolutionTier
		wantPrice float64
	}{
		{name: "empty uses default", model: "gpt-image-2", wantModel: "gpt-image-2", wantTier: ImageResolutionTier1K, wantPrice: 0.01},
		{name: "provider prefix and auto", model: "openai/gpt-image-2", size: "auto", wantModel: "gpt-image-2", wantTier: ImageResolutionTier1K, wantPrice: 0.01},
		{name: "4k alias is floor", model: "gpt-image-2-4k", size: "1024x1024", wantModel: "gpt-image-2", wantTier: ImageResolutionTier4K, wantPrice: 0.045},
		{name: "larger request raises 1k alias", model: "gpt-image-2-1k", size: "1536x1024", wantModel: "gpt-image-2", wantTier: ImageResolutionTier2K, wantPrice: 0.04},
		{name: "banana alias floor", model: "nano-banana2-2k", size: "1k", wantModel: "nano-banana2", wantTier: ImageResolutionTier2K, wantPrice: 0.086666666667},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, configured, err := ResolveImageResolutionPrice(tt.model, tt.size)
			require.NoError(t, err)
			require.True(t, configured)
			assert.Equal(t, tt.wantModel, quote.PricingModel)
			assert.Equal(t, tt.wantTier, quote.Tier)
			assert.InDelta(t, tt.wantPrice, quote.UnitPrice, 1e-12)
		})
	}
}

func TestResolveImageResolutionPriceUsesConfiguredInitialPrices(t *testing.T) {
	tests := []struct {
		model string
		size  string
		want  float64
	}{
		{model: "nano-banana-pro", size: "1k", want: 0.086666666667},
		{model: "nano-banana-pro", size: "2k", want: 0.108333333333},
		{model: "nano-banana-pro", size: "4k", want: 0.161416666667},
		{model: "nano-banana2", size: "1k", want: 0.063916666667},
		{model: "nano-banana2", size: "4k", want: 0.13},
	}
	for _, tt := range tests {
		quote, configured, err := ResolveImageResolutionPrice(tt.model, tt.size)
		require.NoError(t, err)
		require.True(t, configured)
		assert.InDelta(t, tt.want, quote.UnitPrice, 1e-12)
	}
}

func TestResolveImageResolutionPriceRejectsInvalidConfiguredSizes(t *testing.T) {
	for _, size := range []string{"0x1024", "-1x1024", "1024", "1024x", "1024xabc", "4097x512", "512x4097", "8k"} {
		t.Run(size, func(t *testing.T) {
			_, configured, err := ResolveImageResolutionPrice("gpt-image-2", size)
			require.True(t, configured)
			require.Error(t, err)
		})
	}

	_, configured, err := ResolveImageResolutionPrice("unmanaged-image-model", "4097x512")
	require.NoError(t, err)
	assert.False(t, configured)
}

func TestValidateImageResolutionPriceJSONStringRejectsIncompleteAndUnsafePolicies(t *testing.T) {
	tests := []string{
		`{"gpt-image-2":{"prices":{"1k":0.01},"default_tier":"1k"}}`,
		`{"gpt-image-2":{"prices":{"1k":0.01,"2k":0.005,"4k":0.045},"default_tier":"1k"}}`,
		`{"gpt-image-2":{"prices":{"1k":0,"2k":0.04,"4k":0.045},"default_tier":"1k"}}`,
		`{"gpt-image-2":{"prices":{"1k":0.01,"2k":0.04,"4k":0.045},"default_tier":"8k"}}`,
		`{}`,
	}
	for _, value := range tests {
		require.Error(t, ValidateImageResolutionPriceJSONString(value), value)
	}
}

func TestInvalidPolicyDoesNotReplaceCurrentPriceIndex(t *testing.T) {
	before, configured, err := ResolveImageResolutionPrice("gpt-image-2", "2k")
	require.NoError(t, err)
	require.True(t, configured)

	invalid := `{"gpt-image-2":{"prices":{"1k":0.01,"2k":0.005,"4k":0.045},"default_tier":"1k"}}`
	require.Error(t, ValidateImageResolutionPriceJSONString(invalid))

	after, configured, err := ResolveImageResolutionPrice("gpt-image-2", "2k")
	require.NoError(t, err)
	require.True(t, configured)
	assert.Equal(t, before, after)
}

func TestGetImageResolutionPricingMetadataReturnsDefensiveCopy(t *testing.T) {
	metadata, configured := GetImageResolutionPricingMetadata("gpt-image-2-2k")
	require.True(t, configured)
	assert.Equal(t, "gpt-image-2", metadata.PricingModel)
	assert.Equal(t, ImageResolutionTier2K, metadata.AliasMinimumTier)
	metadata.Prices[ImageResolutionTier1K] = 99

	quote, configured, err := ResolveImageResolutionPrice("gpt-image-2", "1k")
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.01, quote.UnitPrice, 1e-12)
}
