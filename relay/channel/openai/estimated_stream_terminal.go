package openai

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// EmitEstimatedGPTStreamTerminal emits a gateway-owned terminal event for an
// accepted GPT text stream whose provider did not return authoritative usage.
// The caller retains the same usage snapshot for billing; this function only
// serializes the public protocol event and never includes provider diagnostics.
func EmitEstimatedGPTStreamTerminal(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage,
	responseID string, createdAt int64, model string, systemFingerprint string, sequenceNumber int64) error {
	if c == nil || c.Request == nil || c.Request.Context().Err() != nil || info == nil || usage == nil {
		return nil
	}
	if info.IsStreamDetached() || info.RelayFormat != types.RelayFormatOpenAI {
		return nil
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeResponses && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		return nil
	}

	publicModel := strings.TrimSpace(model)
	if publicModel == "" {
		publicModel = info.ClientResponseModelName()
	}
	if publicModel == "" {
		publicModel = info.UpstreamModelName
	}
	publicID := strings.TrimSpace(responseID)
	if publicID == "" {
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			publicID = "resp_" + c.GetString(common.RequestIdKey)
			if publicID == "resp_" {
				publicID = "resp_incomplete"
			}
		} else {
			publicID = helper.GetResponseID(c)
		}
	}
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}

	publicUsage := *usage
	publicUsage.UsageSource = "estimated"
	if publicUsage.PromptTokens < 0 {
		publicUsage.PromptTokens = 0
	}
	if publicUsage.CompletionTokens < 0 {
		publicUsage.CompletionTokens = 0
	}
	publicUsage.TotalTokens = publicUsage.PromptTokens + publicUsage.CompletionTokens

	if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
		if err := emitEstimatedResponsesTerminal(c, publicID, publicModel, createdAt, publicUsage, sequenceNumber); err != nil {
			return err
		}
		info.StreamEstimatedTerminalSent = true
		return nil
	}

	stop := helper.GenerateStopResponse(publicID, createdAt, publicModel, "length")
	if systemFingerprint != "" {
		stop.SetSystemFingerprint(systemFingerprint)
	}
	if err := helper.ObjectData(c, stop); err != nil {
		return err
	}
	finalUsage := helper.GenerateFinalUsageResponse(publicID, createdAt, publicModel, publicUsage)
	if systemFingerprint != "" {
		finalUsage.SetSystemFingerprint(systemFingerprint)
	}
	if err := helper.ObjectData(c, finalUsage); err != nil {
		return err
	}
	if err := helper.StringData(c, "[DONE]"); err != nil {
		return err
	}
	info.StreamEstimatedTerminalSent = true
	return nil
}

func emitEstimatedResponsesTerminal(c *gin.Context, responseID, model string, createdAt int64, usage dto.Usage, sequenceNumber int64) error {
	type inputTokenDetails struct {
		CachedTokens int `json:"cached_tokens"`
	}
	type responseUsage struct {
		InputTokens        int                `json:"input_tokens"`
		OutputTokens       int                `json:"output_tokens"`
		TotalTokens        int                `json:"total_tokens"`
		InputTokensDetails *inputTokenDetails `json:"input_tokens_details,omitempty"`
	}
	type responsePayload struct {
		ID                string         `json:"id"`
		Object            string         `json:"object"`
		CreatedAt         int            `json:"created_at,omitempty"`
		Status            string         `json:"status"`
		Model             string         `json:"model"`
		Output            []any          `json:"output"`
		Usage             *responseUsage `json:"usage"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	}
	type eventPayload struct {
		Type           string          `json:"type"`
		Code           string          `json:"code,omitempty"`
		Message        string          `json:"message,omitempty"`
		SequenceNumber *int64          `json:"sequence_number,omitempty"`
		Response       responsePayload `json:"response"`
	}
	details := &inputTokenDetails{CachedTokens: usage.PromptTokensDetails.CachedTokens}
	if details.CachedTokens < 0 {
		details.CachedTokens = 0
	}
	event := eventPayload{
		Type:    "response.incomplete",
		Code:    "upstream_stream_incomplete",
		Message: "The stream ended before completion. Please retry later.",
		Response: responsePayload{
			ID:        responseID,
			Object:    "response",
			CreatedAt: int(createdAt),
			Status:    "incomplete",
			Model:     model,
			Output:    []any{},
			Usage: &responseUsage{
				InputTokens:        usage.PromptTokens,
				OutputTokens:       usage.CompletionTokens,
				TotalTokens:        usage.TotalTokens,
				InputTokensDetails: details,
			},
			IncompleteDetails: struct {
				Reason string `json:"reason"`
			}{Reason: "max_output_tokens"},
		},
	}
	if sequenceNumber > 0 {
		event.SequenceNumber = &sequenceNumber
	}
	data, err := common.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal estimated responses terminal: %w", err)
	}
	return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data))
}
