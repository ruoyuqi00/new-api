package deepseek

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLUsesResponsesEndpoint(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: constant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.deepseek.com",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://api.deepseek.com/responses", url)
}

func TestConvertOpenAIResponsesRequestAppliesDeepSeekV4ThinkingSuffix(t *testing.T) {
	adaptor := &Adaptor{}

	for _, test := range []struct {
		name       string
		model      string
		wantModel  string
		wantEffort string
	}{
		{name: "maximum reasoning", model: "deepseek-v4-test-max", wantModel: "deepseek-v4-test", wantEffort: "max"},
		{name: "reasoning disabled", model: "deepseek-v4-test-none", wantModel: "deepseek-v4-test", wantEffort: "none"},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{}
			converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{Model: test.model})

			require.NoError(t, err)
			request, ok := converted.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			assert.Equal(t, test.wantModel, request.Model)
			require.NotNil(t, request.Reasoning)
			assert.Equal(t, test.wantEffort, request.Reasoning.Effort)
			assert.Equal(t, test.wantEffort, info.ReasoningEffort)
		})
	}
}

func TestConvertOpenAIResponsesRequestUsesMappedUpstreamModel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	info.UpstreamModelName = "deepseek-v4-pro-max"

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{Model: "customer-alias"})

	require.NoError(t, err)
	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, "deepseek-v4-pro", request.Model)
	assert.Equal(t, "deepseek-v4-pro", info.UpstreamModelName)
	require.NotNil(t, request.Reasoning)
	assert.Equal(t, "max", request.Reasoning.Effort)
	assert.Equal(t, "max", info.ReasoningEffort)
}

func TestConvertOpenAIResponsesRequestPreservesUnsuffixedRequest(t *testing.T) {
	stream := false
	temperature := 0.0
	reasoning := &dto.Reasoning{Effort: "high", Summary: "detailed"}
	request := dto.OpenAIResponsesRequest{
		Model:       "deepseek-v4-flash",
		Input:       json.RawMessage(`"hello"`),
		Reasoning:   reasoning,
		Stream:      &stream,
		Temperature: &temperature,
	}
	info := &relaycommon.RelayInfo{}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)

	require.NoError(t, err)
	got, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Equal(t, request.Model, got.Model)
	assert.Equal(t, request.Input, got.Input)
	assert.Same(t, reasoning, got.Reasoning)
	assert.Equal(t, "detailed", got.Reasoning.Summary)
	require.NotNil(t, got.Stream)
	assert.False(t, *got.Stream)
	require.NotNil(t, got.Temperature)
	assert.Zero(t, *got.Temperature)
	assert.Equal(t, "high", info.ReasoningEffort)
}
