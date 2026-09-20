package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestIsVideoGenerationModel(t *testing.T) {
	testCases := []struct {
		name        string
		channelType int
		model       string
		want        bool
	}{
		{name: "Sora", channelType: constant.ChannelTypeOpenAI, model: "sora-2-pro", want: true},
		{name: "Moon Seedance Mini", channelType: constant.ChannelTypeOpenAI, model: "seedance-2-0-mini-official", want: true},
		{name: "Moon Seedance Fast", channelType: constant.ChannelTypeOpenAI, model: "seedance-2-0-fast-official", want: true},
		{name: "Moon Seedance 2.0", channelType: constant.ChannelTypeOpenAI, model: "seedance-2-0-official", want: true},
		{name: "Moon Seedance 2.5", channelType: constant.ChannelTypeOpenAI, model: "seedance-2-5-official", want: true},
		{name: "Moon H3", channelType: constant.ChannelTypeOpenAI, model: "minimax-h3", want: true},
		{name: "Moon Wan 3.0", channelType: constant.ChannelTypeOpenAI, model: "wan3.0-video", want: true},
		{name: "Moon Wan 3.0 Prime", channelType: constant.ChannelTypeOpenAI, model: "wan3.0-video-prime", want: true},
		{name: "Moon Grok 1.5", channelType: constant.ChannelTypeOpenAI, model: "grok-v1.5-video", want: true},
		{name: "Moon Seedance 2.0 PT", channelType: constant.ChannelTypeOpenAI, model: "seedance2.0-9-3-3-PT", want: true},
		{name: "Moon Seedance 2.5 PT", channelType: constant.ChannelTypeOpenAI, model: "seedance2.5-30-10-10-PT", want: true},
		{name: "Moon Seedance Fast PT", channelType: constant.ChannelTypeOpenAI, model: "seedance2.0-fast-PT", want: true},
		{name: "Veo", channelType: constant.ChannelTypeGemini, model: "veo-3.1-generate-preview", want: true},
		{name: "xAI video", channelType: constant.ChannelTypeXai, model: "grok-imagine-video-1.5", want: true},
		{name: "xAI image", channelType: constant.ChannelTypeXai, model: "grok-imagine-image", want: false},
		{name: "Jimeng video", channelType: constant.ChannelTypeJimeng, model: "jimeng_vgfm_t2v_l20", want: true},
		{name: "Wan video", channelType: constant.ChannelTypeAli, model: "wan2.7-i2v", want: true},
		{name: "Wan image", channelType: constant.ChannelTypeAli, model: "wan2.6-image", want: false},
		{name: "Gemini text", channelType: constant.ChannelTypeGemini, model: "gemini-2.5-pro", want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, IsVideoGenerationModel(testCase.channelType, testCase.model))
		})
	}
}
