package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageRequestGPTImage2PriceRatio(t *testing.T) {
	tests := []struct {
		name    string
		size    string
		quality string
		want    float64
	}{
		{name: "default is 1k medium", want: 1},
		{name: "1k square", size: "1024x1024", quality: "medium", want: 1},
		{name: "2k square shorthand", size: "2k", quality: "medium", want: 1.6},
		{name: "2k square explicit", size: "2048x2048", quality: "medium", want: 1.6},
		{name: "4k square shorthand", size: "4k", quality: "medium", want: 2.4},
		{name: "4k square explicit", size: "4096x4096", quality: "medium", want: 2.4},
		{name: "portrait 2k size", size: "1024x1536", quality: "medium", want: 1.6},
		{name: "landscape 2k size", size: "1536x1024", quality: "medium", want: 1.6},
		{name: "low quality keeps fixed tier", size: "1024x1024", quality: "low", want: 1},
		{name: "high quality keeps fixed tier", size: "1024x1024", quality: "high", want: 1},
		{name: "hd alias keeps fixed tier", size: "1024x1024", quality: "hd", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ImageRequest{
				Model:   "gpt-image-2",
				Prompt:  "draw a billing chart",
				Size:    tt.size,
				Quality: tt.quality,
			}

			require.InDelta(t, tt.want, req.GetTokenCountMeta().ImagePriceRatio, 0.000001)
		})
	}
}

func TestImageRequestGPTImage2KeepsCountSeparateFromSizeRatio(t *testing.T) {
	n := uint(3)
	req := ImageRequest{
		Model:   "gpt-image-2",
		Prompt:  "draw a billing chart",
		N:       &n,
		Size:    "1024x1024",
		Quality: "medium",
	}

	meta := req.GetTokenCountMeta()
	require.Equal(t, 1.0, meta.ImagePriceRatio)
	require.Equal(t, 3.0, meta.BillingRatios["n"])
}

func TestImageRequestDefaultsBillingCountToOne(t *testing.T) {
	req := ImageRequest{Model: "gpt-image-2", Prompt: "draw a billing chart"}
	require.Equal(t, 1.0, req.GetTokenCountMeta().BillingRatios["n"])
}

func TestImageRequestGPTImage2ProviderPrefixedModelPriceRatio(t *testing.T) {
	req := ImageRequest{
		Model:   "openai/gpt-image-2",
		Prompt:  "draw a billing chart",
		Size:    "2048x2048",
		Quality: "high",
	}

	require.Equal(t, 1.6, req.GetTokenCountMeta().ImagePriceRatio)
}

func TestGeneralOpenAIRequestGPTImage2FixedTierDoesNotRepeatSizePriceRatio(t *testing.T) {
	req := GeneralOpenAIRequest{
		Model: "gpt-image-2-4k",
		Size:  "4096x4096",
	}

	require.Zero(t, req.GetTokenCountMeta().ImagePriceRatio)
}

func TestImageRequestGPTImage2FixedTierDoesNotRepeatSizePriceRatio(t *testing.T) {
	req := ImageRequest{
		Model:  "gpt-image-2-2k",
		Prompt: "draw a billing chart",
		Size:   "2048x2048",
	}

	require.Equal(t, 1.0, req.GetTokenCountMeta().ImagePriceRatio)
}
