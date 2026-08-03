package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildYucoreMediaTaskPreservesBillingGroup(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Group:   "multimodal",
		Kind:    "image",
		ModelId: "gpt-image-1",
		Prompt:  "test prompt",
	}, 42)
	require.NoError(t, err)
	assert.Equal(t, "multimodal", task.BillingGroup)

	response := buildYucoreMediaTaskResponse(&model.YucoreMediaTask{
		BillingGroup: "multimodal",
	})
	assert.Equal(t, "multimodal", response.Group)
}
