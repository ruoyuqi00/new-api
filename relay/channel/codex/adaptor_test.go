package codex

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestNormalizesStringInput(t *testing.T) {
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		RelayMode:   constant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
		Input: []byte(`"hello"`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	var input []struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	require.NoError(t, common.Unmarshal(request.Input, &input))
	require.Len(t, input, 1)
	assert.Equal(t, "user", input[0].Role)
	require.Len(t, input[0].Content, 1)
	assert.Equal(t, "input_text", input[0].Content[0].Type)
	assert.Equal(t, "hello", input[0].Content[0].Text)
	assert.JSONEq(t, `false`, string(request.Store))
	assert.JSONEq(t, `""`, string(request.Instructions))
}
