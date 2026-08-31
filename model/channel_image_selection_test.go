package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelSupportsImageRequestRejectsFixedSquareOverride(t *testing.T) {
	channel := &Channel{Id: 2418, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"size":"1024x1024"}`)}
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "650x1024"}))
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "1k", AspectRatio: "1:1"}))
}

func TestChannelSupportsImageRequestRejectsFixedSquareOverrideEvenWhenVerified(t *testing.T) {
	channel := &Channel{Id: 2418, Type: constant.ChannelTypeOpenAI, ParamOverride: stringPtr(`{"size":"1024x1024"}`), OtherSettings: `{"image_dimension_support":"any"}`}
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "650x1024"}))
}

func TestChannelSupportsImageRequestAllowsVerifiedOpenAIChannel(t *testing.T) {
	channel := &Channel{Id: 2418, Type: constant.ChannelTypeOpenAI, OtherSettings: `{"image_dimension_support":"any"}`}
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2-1k", ImageSelectionRequirements{Size: "650x1024"}))
}

func TestChannelSupportsImageRequestAppliesConditionalOverrideToMatchingModel(t *testing.T) {
	channel := &Channel{Id: 2487, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"operations":[{"mode":"set","path":"size","value":"2048x2048","conditions":[{"mode":"full","path":"original_model","value":"gpt-image-2-2k"}]}]}`), OtherSettings: `{"image_dimension_support":"any"}`}
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2-2k", ImageSelectionRequirements{Size: "2k", AspectRatio: "3:2"}))
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2", ImageSelectionRequirements{Size: "2k", AspectRatio: "3:2"}))
}

func TestFilterChannelsBySelectionOptionsSkipsIncompatibleImageChannel(t *testing.T) {
	previous := channelsIDM
	channelsIDM = map[int]*Channel{
		2418: {Id: 2418, Type: constant.ChannelTypeAdvancedCustom, ParamOverride: stringPtr(`{"size":"1024x1024"}`)},
		2357: {Id: 2357, OtherSettings: `{"image_dimension_support":"any"}`},
	}
	t.Cleanup(func() { channelsIDM = previous })

	got := filterChannelsBySelectionOptions([]int{2418, 2357}, ChannelSelectionOptions{
		ImageRequirements: &ImageSelectionRequirements{Size: "650x1024"},
		ImageModelName:    "gpt-image-2-1k",
	})
	require.Equal(t, []int{2357}, got)
}

func TestChannelSupportsImageRequestHonorsModelTierAndShape(t *testing.T) {
	channel := &Channel{Id: 2500, OtherSettings: `{"image_dimension_support":"any","image_model_capabilities":{"gpt-image-2":{"max_tier":"2k","shape":"exact"}}}`}
	require.True(t, ChannelSupportsImageRequest(channel, "gpt-image-2", ImageSelectionRequirements{Size: "1536x1024", Tier: "2k"}))
	require.False(t, ChannelSupportsImageRequest(channel, "gpt-image-2", ImageSelectionRequirements{Size: "2048x3072", Tier: "4k"}))
	ratioOnly := &Channel{Id: 2501, OtherSettings: `{"image_dimension_support":"ratio","image_model_capabilities":{"gpt-image-2":{"max_tier":"2k","shape":"ratio"}}}`}
	require.False(t, ChannelSupportsImageRequest(ratioOnly, "gpt-image-2", ImageSelectionRequirements{Size: "1536x1024", Tier: "2k"}))
}

func TestValidateImageCapabilitySettings(t *testing.T) {
	require.NoError(t, ValidateImageCapabilitySettings(dto.ChannelOtherSettings{ImageDimensionSupport: "any"}))
	require.NoError(t, ValidateImageCapabilitySettings(dto.ChannelOtherSettings{ImageDimensionSupport: "pending"}))
	require.Error(t, ValidateImageCapabilitySettings(dto.ChannelOtherSettings{ImageDimensionSupport: "diagonal"}))
	require.NoError(t, ValidateImageCapabilitySettings(dto.ChannelOtherSettings{
		ImageDimensionSupport: "any",
		ImageModelCapabilities: map[string]dto.ImageModelCapability{
			"gpt-image-2": {MaxTier: "2k", Shape: dto.ImageCapabilityShapeExact},
		},
	}))
	err := ValidateImageCapabilitySettings(dto.ChannelOtherSettings{
		ImageModelCapabilities: map[string]dto.ImageModelCapability{
			"gpt-image-2": {MaxTier: "8k", Shape: dto.ImageCapabilityShapeExact},
		},
	})
	assert.Error(t, err)
}

func TestBuildImageSelectionRequirementsNormalizesModelAndTier(t *testing.T) {
	ratio := "3:2"
	request := &dto.ImageRequest{Model: "openai/gpt-image-2-1k", Size: "1536x1024", AspectRatio: &ratio}
	requirements, err := BuildImageSelectionRequirements(request)
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", requirements.CanonicalModel)
	require.Equal(t, "1536x1024", requirements.Size)
	require.Equal(t, "2k", string(requirements.Tier))
	require.Equal(t, 1536, requirements.Width)
	require.Equal(t, 1024, requirements.Height)
	require.True(t, requirements.ExactDimensions)
}

func TestBuildImageSelectionRequirementsRejectsInvalidShape(t *testing.T) {
	ratio := "wide"
	_, err := BuildImageSelectionRequirements(&dto.ImageRequest{Model: "gpt-image-2", Size: "1k", AspectRatio: &ratio})
	require.Error(t, err)
}

func TestImageModelSelectionNamesIncludeHigherTierAliases(t *testing.T) {
	require.Equal(t, []string{"gpt-image-2", "gpt-image-2-1k", "gpt-image-2-2k", "gpt-image-2-4k"}, ImageModelSelectionNames("gpt-image-2", "1k"))
	require.Equal(t, []string{"gpt-image-2", "gpt-image-2-2k", "gpt-image-2-4k"}, ImageModelSelectionNames("gpt-image-2", "2k"))
}

func TestFilterChannelsBySelectionOptionsRejectsUnknownChannelForNonSquareRequest(t *testing.T) {
	previous := channelsIDM
	channelsIDM = map[int]*Channel{}
	t.Cleanup(func() { channelsIDM = previous })

	got := filterChannelsBySelectionOptions([]int{9999}, ChannelSelectionOptions{
		ImageRequirements: &ImageSelectionRequirements{Size: "650x1024"},
		ImageModelName:    "gpt-image-2-1k",
	})
	require.Empty(t, got)
}

func TestFilterAbilitiesBySelectionOptionsFailsClosedWhenImageCapabilitiesCannotBeLoaded(t *testing.T) {
	previousDB := DB
	db, err := gorm.Open(sqlite.Open("file:image_selection_fail_closed?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })

	got := filterAbilitiesBySelectionOptions([]Ability{{ChannelId: 9999}}, ChannelSelectionOptions{
		ImageRequirements: &ImageSelectionRequirements{Size: "650x1024"},
		ImageModelName:    "gpt-image-2-1k",
	})
	require.Empty(t, got)
}

func stringPtr(value string) *string {
	return &value
}
