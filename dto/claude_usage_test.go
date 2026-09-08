package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestClaudeUsageParsesThinkingTokensAsOutputBreakdown(t *testing.T) {
	var usage ClaudeUsage
	require.NoError(t, common.Unmarshal([]byte(`{
		"input_tokens":25,
		"output_tokens":348,
		"output_tokens_details":{"thinking_tokens":312}
	}`), &usage))

	require.Equal(t, 348, usage.OutputTokens)
	require.NotNil(t, usage.OutputTokensDetails)
	require.Equal(t, 312, usage.OutputTokensDetails.ThinkingTokens)
}
