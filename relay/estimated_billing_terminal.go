package relay

import (
	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// EmitEstimatedBillingTerminal mirrors an already-settled local estimate to an
// active OpenAI-compatible downstream stream. It never changes billing state.
func EmitEstimatedBillingTerminal(c *gin.Context, info *relaycommon.RelayInfo) (bool, error) {
	if c == nil || c.Request == nil || c.Request.Context().Err() != nil || info == nil || !info.IsStream {
		return false, nil
	}
	if !info.HasAmbiguousUpstreamSubmission() || !info.PreservePreConsumedQuota || info.StreamTerminalUsageSeen || info.StreamEstimatedTerminalSent {
		return false, nil
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
	default:
		return false, nil
	}
	if c.GetBool("image_generation_call") {
		return false, nil
	}
	if info.RelayMode == relayconstant.RelayModeChatCompletions && info.RelayFormat != types.RelayFormatOpenAI {
		return false, nil
	}
	if info.RelayMode == relayconstant.RelayModeResponses && info.RelayFormat != types.RelayFormatOpenAIResponses {
		return false, nil
	}
	if info.RelayMode == relayconstant.RelayModeResponsesCompact && info.RelayFormat != types.RelayFormatOpenAIResponsesCompaction {
		return false, nil
	}
	if responsesRequest, ok := info.Request.(*dto.OpenAIResponsesRequest); ok {
		for _, tool := range responsesRequest.GetToolsMap() {
			if toolType, _ := tool["type"].(string); toolType == "image_generation" {
				return false, nil
			}
		}
	}

	usage := service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
	usage.UsageSource = "estimated"
	helper.PrepareEventStreamHeaders(c, nil)
	if err := openaichannel.EmitEstimatedGPTStreamTerminal(c, info, usage, "", 0, info.ClientResponseModelName(), "", 0); err != nil {
		return false, err
	}
	return info.StreamEstimatedTerminalSent, nil
}
