package common

import "strings"

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
