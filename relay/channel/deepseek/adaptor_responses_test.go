package deepseek

import (
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
