package common

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const OpenAIResponseCompactModelSuffix = "-openai-compact"

var (
	// OpenAIResponseOnlyModels is a list of models that are only available for OpenAI responses.
	OpenAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
	}
	ImageGenerationModels = []string{
		"dall-e-3",
		"dall-e-2",
		"gpt-image-1",
		"gpt-image-2",
		"nano-banana-pro-",
		"nano-banana2-",
		"prefix:imagen-",
		"flux-",
		"flux.1-",
	}
	EmbeddingModels = []string{
		"embedding",
		"embed",
		"prefix:m3e",
		"bge-",
	}
	OpenAIVideoModels = []string{
		"sora-2",
		"sora-2-pro",
		"seedance-2-0-mini-official",
		"seedance-2-0-fast-official",
		"seedance-2-0-official",
		"seedance-2-5-official",
		"minimax-h3",
		"wan3.0-video",
		"wan3.0-video-prime",
		"grok-v1.5-video",
		"seedance2.0-9-3-3-pt",
		"seedance2.5-30-10-10-pt",
		"seedance2.0-fast-pt",
	}
	OpenAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
)

func IsOpenAIResponseOnlyModel(modelName string) bool {
	for _, m := range OpenAIResponseOnlyModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func IsOpenAIResponseCompactModel(modelName string) bool {
	return strings.HasSuffix(strings.TrimSpace(modelName), OpenAIResponseCompactModelSuffix)
}

func IsImageGenerationModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range ImageGenerationModels {
		if strings.Contains(modelName, m) {
			return true
		}
		if strings.HasPrefix(m, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(m, "prefix:")) {
			return true
		}
	}
	return false
}

func IsVideoGenerationModel(channelType int, modelName string) bool {
	switch channelType {
	case constant.ChannelTypeKling,
		constant.ChannelTypeVidu,
		constant.ChannelTypeDoubaoVideo,
		constant.ChannelTypeSora:
		return true
	}

	modelName = strings.ToLower(strings.TrimSpace(modelName))
	switch channelType {
	case constant.ChannelTypeOpenAI:
		if strings.HasPrefix(modelName, "sora-") {
			return true
		}
		switch modelName {
		case "seedance-2-0-mini-official",
			"seedance-2-0-fast-official",
			"seedance-2-0-official",
			"seedance-2-5-official",
			"minimax-h3",
			"wan3.0-video",
			"wan3.0-video-prime",
			"grok-v1.5-video",
			"seedance2.0-9-3-3-pt",
			"seedance2.5-30-10-10-pt",
			"seedance2.0-fast-pt":
			return true
		default:
			return false
		}
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi:
		return strings.HasPrefix(modelName, "veo-")
	case constant.ChannelTypeXai:
		return strings.HasPrefix(modelName, "grok-imagine-video")
	case constant.ChannelTypeJimeng:
		return strings.HasPrefix(modelName, "jimeng_vgfm_t2v") ||
			strings.HasPrefix(modelName, "jimeng_v30")
	case constant.ChannelTypeVolcEngine:
		return strings.HasPrefix(modelName, "doubao-seedance-")
	case constant.ChannelTypeAli:
		return strings.HasPrefix(modelName, "wan") &&
			(strings.Contains(modelName, "-i2v") ||
				strings.Contains(modelName, "-t2v") ||
				strings.Contains(modelName, "-kf2v") ||
				strings.Contains(modelName, "-s2v"))
	case constant.ChannelTypeMiniMax:
		for _, prefix := range []string{"minimax-hailuo-", "t2v-", "i2v-", "s2v-"} {
			if strings.HasPrefix(modelName, prefix) {
				return true
			}
		}
	}
	return false
}

func IsEmbeddingModel(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	for _, m := range EmbeddingModels {
		if strings.HasPrefix(m, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(m, "prefix:")) {
			return true
		}
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func IsOpenAIVideoModel(modelName string) bool {
	return StringsContains(OpenAIVideoModels, strings.ToLower(strings.TrimSpace(modelName)))
}

func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range OpenAITextModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}
