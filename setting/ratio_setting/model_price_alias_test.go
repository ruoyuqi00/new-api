package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetModelPriceFallsBackToGPTImage2BaseForResolutionAliases(t *testing.T) {
	previous := modelPriceMap.ReadAll()
	t.Cleanup(func() {
		modelPriceMap.Clear()
		modelPriceMap.AddAll(previous)
	})
	modelPriceMap.Set("gpt-image-2", 0.05)

	for _, modelName := range []string{"gpt-image-2-1k", "gpt-image-2-2k", "gpt-image-2-4k", "openai/gpt-image-2-4k"} {
		price, found := GetModelPrice(modelName, false)
		require.True(t, found, modelName)
		require.Equal(t, 0.05, price, modelName)
	}

	modelPriceMap.Set("gpt-image-2-4k", 0.12)
	price, found := GetModelPrice("gpt-image-2-4k", false)
	require.True(t, found)
	require.Equal(t, 0.12, price)
}
