package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestInputTokenDetailsCacheCreationTokensTotal(t *testing.T) {
	tests := []struct {
		name    string
		details InputTokenDetails
		want    int
	}{
		{name: "legacy creation", details: InputTokenDetails{CachedCreationTokens: 100}, want: 100},
		{name: "native write wins", details: InputTokenDetails{CachedCreationTokens: 100, CacheWriteTokens: 120}, want: 120},
		{name: "overlap is not added", details: InputTokenDetails{CachedCreationTokens: 120, CacheWriteTokens: 100}, want: 120},
		{name: "negative clamps", details: InputTokenDetails{CacheWriteTokens: -1}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.details.CacheCreationTokensTotal())
		})
	}
}

func TestOpenAIUsageParsesNativeCacheWriteTokens(t *testing.T) {
	var usage Usage
	require.NoError(t, common.Unmarshal([]byte(`{"prompt_tokens":100,"prompt_tokens_details":{"cache_write_tokens":80}}`), &usage))
	require.Equal(t, 80, usage.PromptTokensDetails.CacheWriteTokens)
}

func TestResponsesCompactionRequestPreservesCodexFields(t *testing.T) {
	var request OpenAIResponsesCompactionRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-test","tools":[{"type":"function"}],"parallel_tool_calls":true,"reasoning":{"effort":"high","mode":"enabled","context":{"turn":1}},"service_tier":"priority","prompt_cache_key":"stable","prompt_cache_options":{"retention":"24h"},"prompt_cache_retention":"24h","text":{"verbosity":"low"}}`), &request))
	require.JSONEq(t, `[{"type":"function"}]`, string(request.Tools))
	require.JSONEq(t, `true`, string(request.ParallelToolCalls))
	require.NotNil(t, request.Reasoning)
	require.JSONEq(t, `"enabled"`, string(request.Reasoning.Mode))
	require.JSONEq(t, `{"turn":1}`, string(request.Reasoning.Context))
	require.Equal(t, "priority", request.ServiceTier)
	require.JSONEq(t, `"stable"`, string(request.PromptCacheKey))
	require.JSONEq(t, `{"retention":"24h"}`, string(request.PromptCacheOptions))
	require.JSONEq(t, `"24h"`, string(request.PromptCacheRetention))
	require.JSONEq(t, `{"verbosity":"low"}`, string(request.Text))
}

func TestResponsesRequestPreservesClientMetadata(t *testing.T) {
	var request OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-test","client_metadata":{"client":"codex"}}`), &request))
	require.JSONEq(t, `{"client":"codex"}`, string(request.ClientMetadata))
}
