package service

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeToolCallQuotaUsesSharedImageResolutionPolicy(t *testing.T) {
	result, err := ComputeToolCallQuota(ToolCallUsage{
		ModelName:           "gpt-image-2-1k",
		ImageGenerationCall: true,
		ImageGenerationSize: "1536x1024",
	}, 0.3)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.InDelta(t, 0.04, result.Items[0].TotalPrice, 1e-12)
	assert.Equal(t, int(math.Round(0.04*common.QuotaPerUnit*0.3)), result.TotalQuota)
}

func TestComputeToolCallQuotaRejectsInvalidConfiguredImageSize(t *testing.T) {
	result, err := ComputeToolCallQuota(ToolCallUsage{
		ModelName:           "gpt-image-2",
		ImageGenerationCall: true,
		ImageGenerationSize: "4097x512",
	}, 1)
	require.ErrorContains(t, err, "4097x512")
	assert.Zero(t, result.TotalQuota)
}

func TestComputeToolCallQuotaKeepsLegacyImagePriceForUnconfiguredModel(t *testing.T) {
	result, err := ComputeToolCallQuota(ToolCallUsage{
		ModelName:              "gpt-image-1.5",
		ImageGenerationCall:    true,
		ImageGenerationQuality: "high",
		ImageGenerationSize:    "1024x1024",
	}, 1)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.InDelta(t, 0.167, result.Items[0].TotalPrice, 1e-12)
	assert.Equal(t, int(math.Round(0.167*common.QuotaPerUnit)), result.TotalQuota)
}
