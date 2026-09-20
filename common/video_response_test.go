package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeVideoTaskResponseHidesPrivateUpstreamFields(t *testing.T) {
	body := []byte(`{
		"id":"upstream_id",
		"task_id":"upstream_task_id",
		"status":"completed",
		"billing":{"charged_amount":9.9},
		"response":{"video":"abcdefghijklmnopqrstuvwxyz"}
	}`)

	got := SanitizeVideoTaskResponse(body, "task_public_123")

	assert.JSONEq(t, `{
		"id":"task_public_123",
		"task_id":"task_public_123",
		"status":"completed",
		"response":{"video":"abcdefghijklmnopqrstuvwxyz"}
	}`, string(got))
	assert.NotContains(t, string(got), "upstream_id")
	assert.NotContains(t, string(got), "charged_amount")
}
