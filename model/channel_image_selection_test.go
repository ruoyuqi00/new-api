package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelSupportsImageRequestRejectsFixedSquareOverride(t *testing.T) {
	channel := &Channel{Id: 2418, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"size":"1024x1024"}`)}
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "650x1024"}))
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "1k", AspectRatio: "1:1"}))
}

func TestChannelSupportsImageRequestKeepsOpenAIOverrideChannelEligible(t *testing.T) {
	channel := &Channel{Id: 2418, Type: constant.ChannelTypeOpenAI, ParamOverride: stringPtr(`{"size":"1024x1024"}`)}
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "650x1024"}))
}

func TestChannelSupportsImageRequestAppliesConditionalOverrideToMatchingModel(t *testing.T) {
	channel := &Channel{Id: 2487, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"operations":[{"mode":"set","path":"size","value":"2048x2048","conditions":[{"mode":"full","path":"original_model","value":"gpt-image-2-2k"}]}]}`)}
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2-2k", ImageSelectionRequirements{Size: "2k", AspectRatio: "3:2"}))
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2", ImageSelectionRequirements{Size: "2k", AspectRatio: "3:2"}))
}

func TestFilterChannelsBySelectionOptionsSkipsIncompatibleImageChannel(t *testing.T) {
	previous := channelsIDM
	channelsIDM = map[int]*Channel{
		2418: {Id: 2418, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"size":"1024x1024"}`)},
		2357: {Id: 2357},
	}
	t.Cleanup(func() { channelsIDM = previous })

	got := filterChannelsBySelectionOptions([]int{2418, 2357}, ChannelSelectionOptions{
		ImageRequirements: &ImageSelectionRequirements{Size: "650x1024"},
		ImageModelName:    "gpt-image-2-1k",
	})
	require.Equal(t, []int{2357}, got)
}

func stringPtr(value string) *string {
	return &value
}
