package ratio_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrokImagePriceUsesApprovedBaseline(t *testing.T) {
	price, err := GrokImagePrice(GrokImageOperationGenerate)
	require.NoError(t, err)
	assert.InDelta(t, 0.02619, price, 0.000000001)
}

func TestGrokImagePriceRejectsUnknownOperation(t *testing.T) {
	_, err := GrokImagePrice(GrokImageOperation("edit"))
	require.ErrorContains(t, err, "unsupported Grok image operation")
}

func TestGrokVideoSecondPriceUsesApprovedRates(t *testing.T) {
	tests := []struct {
		resolution string
		want       float64
	}{
		{resolution: "480p", want: 0.0414},
		{resolution: "720p", want: 0.0594},
		{resolution: "1080p", want: 0.0774},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			price, err := GrokVideoSecondPrice(tt.resolution)
			require.NoError(t, err)
			assert.InDelta(t, tt.want, price, 0.000000001)
			ratio, err := GrokVideoBillingRatio(tt.resolution, 7)
			require.NoError(t, err)
			assert.InDelta(t, tt.want*7/GrokVideoBasePrice, ratio, 0.000000001)
		})
	}
}

func TestGrokVideoSecondPriceNormalizesResolutionAliases(t *testing.T) {
	for _, resolution := range []string{"480", " 480P ", "480p", "1280x720", "1920x1080"} {
		price, err := GrokVideoSecondPrice(resolution)
		require.NoError(t, err)
		if strings.Contains(resolution, "720") {
			assert.InDelta(t, 0.0594, price, 0.000000001)
		} else if strings.Contains(resolution, "1080") {
			assert.InDelta(t, 0.0774, price, 0.000000001)
		} else {
			assert.InDelta(t, GrokVideoBasePrice, price, 0.000000001)
		}
	}
}

func TestGrokVideoBillingRejectsUnsupportedDimensions(t *testing.T) {
	_, err := GrokVideoSecondPrice("4k")
	require.ErrorContains(t, err, "unsupported Grok video resolution")

	for _, seconds := range []int{0, -1, 16} {
		_, err = GrokVideoBillingRatio("720p", seconds)
		require.ErrorContains(t, err, "unsupported Grok video duration")
	}
}

func TestGrokMediaModelIdentificationIsExact(t *testing.T) {
	for _, model := range []string{"grok-imagine-image", "grok-imagine-image-quality"} {
		assert.True(t, IsGrokImageGenerationModel(model), model)
		assert.False(t, IsGrokImagineVideoModel(model), model)
		assert.False(t, IsGrokImageEditModel(model), model)
	}

	assert.True(t, IsGrokImageEditModel("grok-imagine-edit"))
	assert.False(t, IsGrokImageGenerationModel("grok-imagine-edit"))

	for _, model := range []string{
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine-video-1.5-preview",
	} {
		assert.True(t, IsGrokImagineVideoModel(model), model)
		assert.False(t, IsGrokImageGenerationModel(model), model)
	}

	for _, model := range []string{
		"grok-video",
		"grok-video-1.5",
		"grok-4.6",
		"grok-imagine-video-experimental",
	} {
		assert.False(t, IsGrokImageGenerationModel(model), model)
		assert.False(t, IsGrokImageEditModel(model), model)
		assert.False(t, IsGrokImagineVideoModel(model), model)
	}
}

func TestDefaultGrokImaginePricesUseApprovedBase(t *testing.T) {
	prices := GetDefaultModelPriceMap()

	for _, model := range []string{"grok-imagine-image", "grok-imagine-image-quality"} {
		assert.InDelta(t, 0.02619, prices[model], 0.000000001, model)
	}
	for _, model := range []string{
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine-video-1.5-preview",
	} {
		assert.InDelta(t, GrokVideoBasePrice, prices[model], 0.000000001, model)
	}
	assert.NotContains(t, prices, "grok-imagine-edit")
}
