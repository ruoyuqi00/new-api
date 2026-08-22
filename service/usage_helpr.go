package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

//func GetPromptTokens(textRequest dto.GeneralOpenAIRequest, relayMode int) (int, error) {
//	switch relayMode {
//	case constant.RelayModeChatCompletions:
//		return CountTokenMessages(textRequest.Messages, textRequest.Model)
//	case constant.RelayModeCompletions:
//		return CountTokenInput(textRequest.Prompt, textRequest.Model), nil
//	case constant.RelayModeModerations:
//		return CountTokenInput(textRequest.Input, textRequest.Model), nil
//	}
//	return 0, errors.New("unknown relay mode")
//}

func ResponseText2Usage(c *gin.Context, responseText string, modeName string, promptTokens int) *dto.Usage {
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	usage := &dto.Usage{UsageSource: "estimated"}
	usage.PromptTokens = promptTokens
	usage.CompletionTokens = EstimateTokenByModel(modeName, responseText)
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

func ValidUsage(usage *dto.Usage) bool {
	return usage != nil && (usage.PromptTokens != 0 || usage.CompletionTokens != 0)
}

const maxGPTTextUsageTokens = 10_000_000

// ValidGPTTextUsage accepts only a structurally complete, bounded usage report
// for GPT text settlement. It is intentionally separate from ValidUsage so
// media/provider-specific usage handling keeps its existing semantics.
func ValidGPTTextUsage(usage *dto.Usage) bool {
	if usage == nil {
		return false
	}
	values := []int{
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
		usage.PromptCacheHitTokens,
		usage.PromptTokensDetails.CachedTokens,
		usage.PromptTokensDetails.CachedCreationTokens,
		usage.PromptTokensDetails.CacheWriteTokens,
		usage.PromptTokensDetails.TextTokens,
		usage.PromptTokensDetails.AudioTokens,
		usage.PromptTokensDetails.ImageTokens,
		usage.CompletionTokenDetails.TextTokens,
		usage.CompletionTokenDetails.AudioTokens,
		usage.CompletionTokenDetails.ImageTokens,
		usage.CompletionTokenDetails.ReasoningTokens,
		usage.InputTokens,
		usage.OutputTokens,
		usage.ClaudeCacheCreation5mTokens,
		usage.ClaudeCacheCreation1hTokens,
	}
	for _, value := range values {
		if value < 0 || value > maxGPTTextUsageTokens {
			return false
		}
	}

	inputTokens := usage.PromptTokens
	if usage.InputTokens != 0 {
		if inputTokens != 0 && inputTokens != usage.InputTokens {
			return false
		}
		inputTokens = usage.InputTokens
	}
	outputTokens := usage.CompletionTokens
	if usage.OutputTokens != 0 {
		if outputTokens != 0 && outputTokens != usage.OutputTokens {
			return false
		}
		outputTokens = usage.OutputTokens
	}
	if inputTokens == 0 && outputTokens == 0 {
		return false
	}
	if inputTokens > maxGPTTextUsageTokens-outputTokens {
		return false
	}
	if usage.TotalTokens == 0 || usage.TotalTokens != inputTokens+outputTokens {
		return false
	}

	cacheTokens := usage.PromptCacheHitTokens
	if usage.PromptTokensDetails.CachedTokens > cacheTokens {
		cacheTokens = usage.PromptTokensDetails.CachedTokens
	}
	cacheWriteTokens := usage.PromptTokensDetails.CacheWriteTokens
	if usage.PromptTokensDetails.CachedCreationTokens > cacheWriteTokens {
		cacheWriteTokens = usage.PromptTokensDetails.CachedCreationTokens
	}
	if usage.InputTokensDetails != nil {
		inputDetails := []int{
			usage.InputTokensDetails.CachedTokens,
			usage.InputTokensDetails.CachedCreationTokens,
			usage.InputTokensDetails.CacheWriteTokens,
			usage.InputTokensDetails.TextTokens,
			usage.InputTokensDetails.AudioTokens,
			usage.InputTokensDetails.ImageTokens,
		}
		for _, value := range inputDetails {
			if value < 0 || value > maxGPTTextUsageTokens {
				return false
			}
		}
		if usage.InputTokensDetails.CachedTokens > cacheTokens {
			cacheTokens = usage.InputTokensDetails.CachedTokens
		}
		if usage.InputTokensDetails.CacheWriteTokens > cacheWriteTokens {
			cacheWriteTokens = usage.InputTokensDetails.CacheWriteTokens
		}
		if usage.InputTokensDetails.CachedCreationTokens > cacheWriteTokens {
			cacheWriteTokens = usage.InputTokensDetails.CachedCreationTokens
		}
	}
	if cacheTokens > inputTokens || cacheWriteTokens > inputTokens {
		return false
	}
	if usage.PromptTokensDetails.TextTokens > inputTokens ||
		usage.PromptTokensDetails.AudioTokens > inputTokens ||
		usage.PromptTokensDetails.ImageTokens > inputTokens ||
		usage.CompletionTokenDetails.TextTokens > outputTokens ||
		usage.CompletionTokenDetails.AudioTokens > outputTokens ||
		usage.CompletionTokenDetails.ImageTokens > outputTokens ||
		usage.CompletionTokenDetails.ReasoningTokens > outputTokens {
		return false
	}
	if usage.InputTokensDetails != nil &&
		(usage.InputTokensDetails.TextTokens > inputTokens ||
			usage.InputTokensDetails.AudioTokens > inputTokens ||
			usage.InputTokensDetails.ImageTokens > inputTokens) {
		return false
	}
	return true
}
