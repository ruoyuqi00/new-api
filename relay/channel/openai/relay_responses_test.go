package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeCompletedImageGenerationStatus(t *testing.T) {
	input := []byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","status":"generating","result":"image-data"}}`)

	got, changed := normalizeCompletedImageGenerationStatus(input)

	require.True(t, changed)
	require.Equal(t, "completed", gjson.GetBytes(got, "item.status").String())
}

func TestNormalizeCompletedImageGenerationStatusLeavesPendingItemWithoutResult(t *testing.T) {
	input := []byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","status":"generating"}}`)

	got, changed := normalizeCompletedImageGenerationStatus(input)

	require.False(t, changed)
	require.Equal(t, input, got)
}
