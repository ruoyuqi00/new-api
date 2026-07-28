package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestGetEndpointTypesByChannelTypeCompactModel(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIResponseCompact},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-5-openai-compact"),
	)
}

func TestGetEndpointTypesByChannelTypeCodex(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIResponse},
		GetEndpointTypesByChannelType(constant.ChannelTypeCodex, "gpt-5-codex"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIResponseCompact},
		GetEndpointTypesByChannelType(constant.ChannelTypeCodex, "gpt-5-codex-openai-compact"),
	)
}

func TestGetEndpointTypesByChannelTypeExistingDefaults(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-4o-mini"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIResponse},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "o3-pro"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse},
		GetEndpointTypesByChannelType(constant.ChannelTypeXai, "grok-4"),
	)
}

func TestGetEndpointTypesByChannelTypeEmbeddingModels(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeEmbeddings, constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "text-embedding-3-large"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeEmbeddings, constant.EndpointTypeGemini, constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeGemini, "gemini-embedding-001"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeEmbeddings, constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "bge-large-zh-v1.5"),
	)
}

func TestGetEndpointTypesByChannelTypeFixedImageModels(t *testing.T) {
	for _, modelName := range []string{
		"gpt-image-2-1k",
		"gpt-image-2-2k",
		"gpt-image-2-4k",
		"nano-banana-pro-1k",
		"nano-banana2-4k",
	} {
		require.Equal(t,
			[]constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI},
			GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, modelName),
		)
	}
}

func TestOpenAIVideoDefaultEndpoint(t *testing.T) {
	info, ok := GetDefaultEndpointInfo(constant.EndpointTypeOpenAIVideo)
	require.True(t, ok)
	require.Equal(t, EndpointInfo{Path: "/v1/videos", Method: "POST"}, info)
}

func TestOpenAIChannelSoraModelsUseVideoEndpoint(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "sora-2"),
	)
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-5.6"),
	)
}
