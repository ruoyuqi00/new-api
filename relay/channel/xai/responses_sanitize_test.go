package xai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestSanitizeXAIResponsesInput(t *testing.T) {
	input := []byte(`[
		{"type":"reasoning","content":null,"summary":[]},
		{"type":"additional_tools","tools":[{"type":"computer"}]},
		{"type":"message","content":[{"type":"input_text","text":"hello","external_web_access":true}]}
	]`)

	got, err := sanitizeXAIResponsesInput(input)

	require.NoError(t, err)
	var items []map[string]any
	require.NoError(t, common.Unmarshal(got, &items))
	require.Len(t, items, 2)
	require.NotContains(t, items[0], "content")
	content := items[1]["content"].([]any)
	require.NotContains(t, content[0].(map[string]any), "external_web_access")
}

func TestSanitizeXAIResponsesInputPreservesSupportedContent(t *testing.T) {
	input := []byte(`[{"type":"reasoning","content":[],"summary":[]}]`)

	got, err := sanitizeXAIResponsesInput(input)

	require.NoError(t, err)
	require.Equal(t, input, []byte(got))
}

func TestConvertOpenAIResponsesRequestSanitizesXAIUnsupportedFields(t *testing.T) {
	request := dto.OpenAIResponsesRequest{
		Model:                "xai/grok-composer-2.5-fast",
		Reasoning:            &dto.Reasoning{Effort: "high"},
		PromptCacheRetention: json.RawMessage(`"24h"`),
		SafetyIdentifier:     json.RawMessage(`"user-1"`),
		Tools: json.RawMessage(`[
			{"type":"function","name":"lookup","parameters":{"type":"object","external_web_access":true}},
			{"type":"computer"}
		]`),
		ToolChoice: json.RawMessage(`{"type":"computer"}`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	got := converted.(dto.OpenAIResponsesRequest)
	require.Nil(t, got.Reasoning)
	require.Empty(t, got.PromptCacheRetention)
	require.Empty(t, got.SafetyIdentifier)
	require.Empty(t, got.ToolChoice)
	var tools []map[string]any
	require.NoError(t, common.Unmarshal(got.Tools, &tools))
	require.Len(t, tools, 1)
	require.Equal(t, "function", tools[0]["type"])
	parameters := tools[0]["parameters"].(map[string]any)
	require.NotContains(t, parameters, "external_web_access")
}

func TestConvertOpenAIResponsesRequestPreservesReasoningForSupportedModel(t *testing.T) {
	reasoning := &dto.Reasoning{Effort: "high"}
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
		Model:     "grok-4.5",
		Reasoning: reasoning,
	})

	require.NoError(t, err)
	require.Equal(t, reasoning, converted.(dto.OpenAIResponsesRequest).Reasoning)
}
