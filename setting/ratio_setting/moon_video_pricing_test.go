package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoonVideoDefaultPricingEnablesApprovedModels(t *testing.T) {
	assert.Equal(t, 5.75, defaultModelRatio["seedance-2-0-mini-official"])
	assert.Equal(t, 12.025, defaultModelRatio["seedance-2-0-fast-official"])
	assert.Equal(t, 18.4, defaultModelRatio["seedance-2-0-official"])
	assert.Equal(t, 29.75, defaultModelRatio["seedance-2-5-official"])
	assert.Equal(t, 0.10, defaultModelPrice["minimax-h3"])
	assert.Equal(t, 0.27, defaultModelPrice["wan3.0-video"])
	assert.Equal(t, 0.40, defaultModelPrice["wan3.0-video-prime"])
	assert.Equal(t, 0.60, defaultModelPrice["grok-v1.5-video"])
	assert.Equal(t, 0.34, defaultModelPrice["seedance2.0-9-3-3-PT"])
	assert.Equal(t, 0.45, defaultModelPrice["seedance2.5-30-10-10-PT"])
	assert.Equal(t, 0.30, defaultModelPrice["seedance2.0-fast-PT"])
}
