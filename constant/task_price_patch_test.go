package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskPricePatchAppliesKeepsGrokVideoDurationBased(t *testing.T) {
	original := TaskPricePatches
	TaskPricePatches = []string{"sora-2", "grok-imagine-video-1.5-preview"}
	t.Cleanup(func() {
		TaskPricePatches = original
	})

	assert.True(t, TaskPricePatchApplies("sora-2"))
	assert.False(t, TaskPricePatchApplies("grok-imagine-video-1.5-preview"))
	assert.False(t, TaskPricePatchApplies("unconfigured-video"))
}
