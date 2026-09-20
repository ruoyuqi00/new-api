package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildGroupProtocolMetadataUsesActiveChannelCapabilities(t *testing.T) {
	advancedSettings, err := common.Marshal(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{
				{IncomingPath: "/v1/messages"},
				{IncomingPath: "/v1/chat/completions"},
			},
		},
	})
	require.NoError(t, err)

	metadata := BuildGroupProtocolMetadata([]model.AbilityWithChannel{
		{
			Ability:         model.Ability{Group: "multi", Model: "claude-sonnet-4", ChannelId: 101},
			ChannelType:     constant.ChannelTypeAdvancedCustom,
			ChannelSettings: string(advancedSettings),
		},
		{
			Ability:     model.Ability{Group: "multi", Model: "gemini-2.5-pro", ChannelId: 102},
			ChannelType: constant.ChannelTypeGemini,
		},
		{
			Ability:     model.Ability{Group: "multi", Model: "gpt-image-1", ChannelId: 103},
			ChannelType: constant.ChannelTypeOpenAI,
		},
	})

	require.Equal(t,
		[]string{"openai", "claude", "gemini", "image"},
		metadata["multi"].Protocols,
	)
	require.Equal(t,
		[]string{
			"/v1/chat/completions",
			"/v1/messages",
			"/v1beta/models/{model}:generateContent",
			"/v1/images/generations",
		},
		metadata["multi"].EndpointPaths,
	)
}

func TestBuildGroupProtocolMetadataClassifiesVideoModels(t *testing.T) {
	metadata := BuildGroupProtocolMetadata([]model.AbilityWithChannel{
		{
			Ability:     model.Ability{Group: "video", Model: "wan2.7-t2v"},
			ChannelType: constant.ChannelTypeAli,
		},
		{
			Ability:     model.Ability{Group: "video", Model: "sora-2"},
			ChannelType: constant.ChannelTypeSora,
		},
	})

	require.Equal(t, []string{"video"}, metadata["video"].Protocols)
	require.Equal(t, []string{"/v1/videos"}, metadata["video"].EndpointPaths)
}

func TestBuildGroupProtocolMetadataClassifiesImageModelsWithoutChangingGlobalPricing(t *testing.T) {
	metadata := BuildGroupProtocolMetadata([]model.AbilityWithChannel{
		{
			Ability:     model.Ability{Group: "image", Model: "grok-imagine-image"},
			ChannelType: constant.ChannelTypeXai,
		},
		{
			Ability:     model.Ability{Group: "image", Model: "jimeng_high_aes_general_v21_L"},
			ChannelType: constant.ChannelTypeJimeng,
		},
		{
			Ability:     model.Ability{Group: "image", Model: "doubao-seedream-4-5-251128"},
			ChannelType: constant.ChannelTypeVolcEngine,
		},
	})

	require.Contains(t, metadata["image"].Protocols, "image")
	require.Contains(t, metadata["image"].EndpointPaths, "/v1/images/generations")
}

func TestMergeGroupProtocolMetadataKeepsStableProtocolOrder(t *testing.T) {
	metadata := map[string]GroupProtocolMetadata{
		"claude": {
			Protocols:     []string{"claude", "openai"},
			EndpointPaths: []string{"/v1/messages", "/v1/chat/completions"},
		},
		"media": {
			Protocols:     []string{"image", "video"},
			EndpointPaths: []string{"/v1/images/generations", "/v1/videos"},
		},
	}

	merged := MergeGroupProtocolMetadata([]string{"media", "missing", "claude"}, metadata)

	require.Equal(t, []string{"openai", "claude", "image", "video"}, merged.Protocols)
	require.Equal(t,
		[]string{
			"/v1/chat/completions",
			"/v1/messages",
			"/v1/images/generations",
			"/v1/videos",
		},
		merged.EndpointPaths,
	)
}
