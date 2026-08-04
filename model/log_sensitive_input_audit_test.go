package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsRedactsSensitiveInputAuditEvidence(t *testing.T) {
	logs := []*Log{
		{
			Content: "full blocked input",
			Other: common.MapToJsonStr(map[string]any{
				"violation_fee_reason": SensitiveInputViolationReason,
				"sensitive_words":      []string{"blocked"},
				"charged_quota":        12,
			}),
		},
		{
			Content: "normal content",
			Other:   common.MapToJsonStr(map[string]any{"charged_quota": 3}),
		},
	}

	formatUserLogs(logs, 0)

	assert.Equal(t, SensitiveInputBlockedLogContent, logs[0].Content)
	redactedOther, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, redactedOther, "sensitive_words")
	assert.Equal(t, float64(12), redactedOther["charged_quota"])
	assert.Equal(t, "normal content", logs[1].Content)
}
