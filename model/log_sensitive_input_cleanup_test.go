package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeExpiredSensitiveInputAuditBatchPreservesBillingMetadata(t *testing.T) {
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
	t.Cleanup(func() { _ = LOG_DB.Exec("DELETE FROM logs").Error })

	const (
		cutoff   = int64(1_000)
		purgedAt = int64(2_000)
	)
	violationOther := func(words []string) string {
		return common.MapToJsonStr(map[string]any{
			"violation_fee_reason": SensitiveInputViolationReason,
			"sensitive_words":      words,
			"charge_succeeded":     true,
			"status_code":          400,
		})
	}
	logs := []Log{
		{UserId: 11, CreatedAt: cutoff - 1, Type: LogTypeConsume, Content: "expired input", ModelName: "gpt-test", PromptTokens: 123, Quota: 456, Other: violationOther([]string{"expired"})},
		{UserId: 12, CreatedAt: cutoff, Type: LogTypeConsume, Content: "current input", ModelName: "gpt-test", PromptTokens: 124, Quota: 457, Other: violationOther([]string{"current"})},
		{UserId: 13, CreatedAt: cutoff - 1, Type: LogTypeConsume, Content: "normal content", ModelName: "gpt-test", PromptTokens: 125, Quota: 458, Other: common.MapToJsonStr(map[string]any{"charge_succeeded": true})},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := PurgeExpiredSensitiveInputAuditBatch(context.Background(), cutoff, 200, purgedAt)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Purged)

	var saved []Log
	require.NoError(t, LOG_DB.Order("user_id asc").Find(&saved).Error)
	require.Len(t, saved, 3)
	assert.Equal(t, SensitiveInputBlockedLogContent, saved[0].Content)
	assert.Equal(t, 456, saved[0].Quota)
	assert.Equal(t, 123, saved[0].PromptTokens)
	assert.Equal(t, "gpt-test", saved[0].ModelName)
	assert.Equal(t, 11, saved[0].UserId)

	expiredOther, err := common.StrToMap(saved[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, expiredOther, "sensitive_words")
	assert.Equal(t, SensitiveInputViolationReason, expiredOther["violation_fee_reason"])
	assert.Equal(t, true, expiredOther["charge_succeeded"])
	assert.Equal(t, float64(400), expiredOther["status_code"])
	assert.Equal(t, true, expiredOther["sensitive_input_purged"])
	assert.Equal(t, float64(purgedAt), expiredOther["sensitive_input_purged_at"])

	assert.Equal(t, "current input", saved[1].Content)
	assert.Contains(t, saved[1].Other, "sensitive_words")
	assert.Equal(t, "normal content", saved[2].Content)
}
