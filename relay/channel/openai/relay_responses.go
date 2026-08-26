package openai

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	responsesIncompletePublicCode    = "upstream_stream_incomplete"
	responsesIncompletePublicMessage = "The stream ended before completion. Please retry later."
	responsesFailedPublicCode        = "upstream_response_failed"
	responsesFailedPublicMessage     = "The response failed before completion. Please retry later."
	codexRateLimitsEventType         = "codex.rate_limits"
	codexResponseMetadataEventType   = "codex.response.metadata"
	fixedCodexRateLimitsData         = `{"type":"codex.rate_limits","plan_type":"pro","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":null},"credits":null}`
)

func isGPTResponsesModel(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(info.OriginModelName))
	return strings.HasPrefix(model, "gpt-")
}

func shouldSendCodexRateLimitsPrelude(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil || c.Request.URL.Path != "/v1/responses" || !isGPTResponsesModel(info) {
		return false
	}
	if strings.Contains(strings.ToLower(c.GetHeader("Originator")), "codex") ||
		strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "codex") {
		return true
	}
	return strings.TrimSpace(c.GetHeader("X-Codex-Turn-Metadata")) != "" ||
		strings.TrimSpace(c.GetHeader("X-Codex-Beta-Features")) != ""
}

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	helper.CaptureActualResponseModelJSON(info, responseBody)
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	if info != nil && strings.TrimSpace(responsesResponse.ID) != "" {
		info.ChannelAffinityResponseID = strings.TrimSpace(responsesResponse.ID)
		info.ChannelAffinityResponseIDObserved = true
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// compute usage
	usage := dto.Usage{}
	usageConfirmed := false
	isImageGeneration := responsesResponse.HasImageGenerationCall()
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.CacheWriteTokens = responsesResponse.Usage.InputTokensDetails.CacheWriteTokens
		}
		if isImageGeneration {
			usageConfirmed = service.ValidUsage(&usage)
		} else {
			usageConfirmed = service.ValidGPTTextUsage(&usage)
		}
	}
	if !usageConfirmed && !isImageGeneration {
		info.PreservePreConsumedQuota = true
		usage = *service.ResponseText2Usage(c, service.ExtractOutputTextFromResponses(&responsesResponse), info.UpstreamModelName, info.GetEstimatePromptTokens())
		if sanitized, setErr := sjson.SetBytes(responseBody, "usage", map[string]int{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}); setErr == nil {
			responseBody = sanitized
		}
	} else if !isImageGeneration {
		usage.UsageSource = "upstream"
	}
	normalizedBody, _, err := helper.NormalizeClientResponseModelJSON(info, responseBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, normalizedBody)
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)
	info.StreamTerminalMarkersRequired = true
	info.PreservePreConsumedQuota = false

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	terminalReceived := false
	var nextSequenceNumber *int64
	responseID := ""
	publishedResponseID := ""
	responseModel := ""
	pendingEstimatedTerminal := false
	imageGenerationSeen := false
	isGPTModel := isGPTResponsesModel(info)

	if resp.StatusCode == http.StatusOK && shouldSendCodexRateLimitsPrelude(c, info) {
		helper.PrepareEventStreamHeaders(c, resp)
		if err := helper.ResponseChunkData(
			c,
			dto.ResponsesStreamResponse{Type: codexResponseMetadataEventType},
			fixedCodexRateLimitsData,
		); err != nil {
			info.PreservePreConsumedQuota = true
			return usage, nil
		}
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if normalized, changed := normalizeCompletedImageGenerationStatus([]byte(data)); changed {
			data = string(normalized)
		}
		normalizedData, _, err := helper.NormalizeClientResponseModelJSON(info, []byte(data))
		if err != nil {
			sr.Error(err)
			return
		}
		data = string(normalizedData)

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if isGPTModel && streamResponse.Type == codexRateLimitsEventType {
			return
		}
		if streamResponse.SequenceNumber != nil {
			next := *streamResponse.SequenceNumber + 1
			nextSequenceNumber = &next
		}
		if streamResponse.Response != nil {
			if streamResponse.Response.ID != "" {
				responseID = streamResponse.Response.ID
				info.ChannelAffinityResponseID = strings.TrimSpace(responseID)
				info.ChannelAffinityResponseIDObserved = true
			}
			if streamResponse.Response.Model != "" {
				responseModel = streamResponse.Response.Model
			}
			if streamResponse.Response.HasImageGenerationCall() {
				imageGenerationSeen = true
			}
			if streamResponse.Response.Usage != nil && !streamResponse.Response.HasImageGenerationCall() {
				candidate := &dto.Usage{
					PromptTokens:     streamResponse.Response.Usage.InputTokens,
					CompletionTokens: streamResponse.Response.Usage.OutputTokens,
					TotalTokens:      streamResponse.Response.Usage.TotalTokens,
				}
				if streamResponse.Response.Usage.InputTokensDetails != nil {
					candidate.PromptTokensDetails = *streamResponse.Response.Usage.InputTokensDetails
				}
				if !service.ValidGPTTextUsage(candidate) {
					streamResponse.Response.Usage = nil
					info.PreservePreConsumedQuota = true
					info.StreamTerminalUsageSeen = false
					if sanitized, marshalErr := common.Marshal(&streamResponse); marshalErr == nil {
						data = string(sanitized)
					}
				}
			}
		}
		originalEventType := streamResponse.Type
		switch originalEventType {
		case "response.completed", "response.done", "response.incomplete", "response.failed", "response.error", "error":
			if streamResponse.Response != nil && streamResponse.Response.Usage != nil {
				terminalUsage := &dto.Usage{
					PromptTokens:     streamResponse.Response.Usage.InputTokens,
					CompletionTokens: streamResponse.Response.Usage.OutputTokens,
					TotalTokens:      streamResponse.Response.Usage.TotalTokens,
				}
				if streamResponse.Response.Usage.InputTokensDetails != nil {
					terminalUsage.PromptTokensDetails = *streamResponse.Response.Usage.InputTokensDetails
				}
				usageValid := service.ValidGPTTextUsage(terminalUsage)
				if streamResponse.Response.HasImageGenerationCall() {
					usageValid = service.ValidUsage(terminalUsage)
				}
				if !usageValid {
					if !streamResponse.Response.HasImageGenerationCall() {
						info.PreservePreConsumedQuota = true
						usage = &dto.Usage{}
						info.StreamTerminalUsageSeen = false
						streamResponse.Response.Usage = nil
						if sanitized, marshalErr := common.Marshal(&streamResponse); marshalErr == nil {
							data = string(sanitized)
						}
					}
					break
				}
				info.StreamTerminalUsageSeen = true
				info.MarkStreamTerminalUsage()
				terminalUsage.UsageSource = "upstream"
				if terminalUsage.PromptTokens != 0 {
					usage.PromptTokens = terminalUsage.PromptTokens
				}
				if terminalUsage.CompletionTokens != 0 {
					usage.CompletionTokens = terminalUsage.CompletionTokens
				}
				if terminalUsage.TotalTokens != 0 {
					usage.TotalTokens = terminalUsage.TotalTokens
				}
				usage.PromptTokensDetails = terminalUsage.PromptTokensDetails
				usage.UsageSource = terminalUsage.UsageSource
			}
		}
		terminalFailure := false
		suppressTerminalEvent := false
		switch originalEventType {
		case "response.completed", "response.done":
			terminalReceived = true
			info.StreamTerminalSuccess = true
			if !info.StreamTerminalUsageSeen && !imageGenerationSeen {
				pendingEstimatedTerminal = true
				suppressTerminalEvent = true
			}
		case "response.incomplete", "response.failed", "response.error", "error":
			terminalReceived = true
			info.StreamTerminalSuccess = false
			terminalFailure = true
			publicResponseID := publishedResponseID
			if publicResponseID == "" {
				publicResponseID = "resp_" + c.GetString(common.RequestIdKey)
				if publicResponseID == "resp_" {
					publicResponseID = "resp_failed"
				}
			}
			publicResponseModel := responseModel
			if publicResponseModel == "" {
				publicResponseModel = info.ClientResponseModelName()
			}
			publicError := map[string]any{
				"code":    responsesFailedPublicCode,
				"message": responsesFailedPublicMessage,
			}
			streamResponse.Error = nil
			streamResponse.Code = ""
			streamResponse.Message = ""
			streamResponse.Param = nil
			if originalEventType == "error" {
				streamResponse.Code = responsesFailedPublicCode
				streamResponse.Message = responsesFailedPublicMessage
				streamResponse.Param = []byte(`null`)
				streamResponse.Response = nil
			} else {
				publicStatus := []byte(`"failed"`)
				if originalEventType == "response.incomplete" {
					publicStatus = []byte(`"incomplete"`)
				}
				streamResponse.Response = &dto.OpenAIResponsesResponse{
					ID:     publicResponseID,
					Object: "response",
					Model:  publicResponseModel,
					Status: publicStatus,
					Error:  publicError,
					Output: []dto.ResponsesOutput{},
				}
			}
			if !info.StreamTerminalUsageSeen && !imageGenerationSeen {
				pendingEstimatedTerminal = true
				suppressTerminalEvent = true
			}
			sanitized, err := common.Marshal(&streamResponse)
			if err != nil {
				sr.Error(err)
				return
			}
			data = string(sanitized)
		}
		if !suppressTerminalEvent {
			if err := sendResponsesStreamData(c, streamResponse, data); err != nil {
				sr.Stop(err)
				return
			}
		}
		if !terminalFailure && streamResponse.Response != nil && streamResponse.Response.ID != "" {
			publishedResponseID = streamResponse.Response.ID
		}
		switch streamResponse.Type {
		case "response.completed", "response.done":
			if streamResponse.Response != nil {
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		case dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
				if streamResponse.Item.Type == dto.ResponsesOutputTypeImageGenerationCall {
					imageGenerationSeen = true
				}
				switch streamResponse.Item.Type {
				case dto.BuildInCallWebSearchCall:
					if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
						if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
							webSearchTool.CallCount++
						}
					}
				case dto.ResponsesOutputTypeImageGenerationCall:
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Item.Quality)
					c.Set("image_generation_call_size", streamResponse.Item.Size)
				}
			}
		}
	})
	if !info.StreamTerminalSuccess || !info.StreamTerminalUsageSeen {
		info.PreservePreConsumedQuota = true
	}
	if !terminalReceived {
		info.PreservePreConsumedQuota = true
		if !imageGenerationSeen && c.Request.Context().Err() == nil {
			pendingEstimatedTerminal = true
		}
		if (imageGenerationSeen || c.Request.Context().Err() != nil) && info.StreamStatus != nil {
			switch info.StreamStatus.EndReason {
			case relaycommon.StreamEndReasonClientGone, relaycommon.StreamEndReasonHandlerStop:
			default:
				info.StreamStatus.RecordError(responsesIncompletePublicMessage)
				publicResponseID := publishedResponseID
				if publicResponseID == "" {
					publicResponseID = "resp_" + c.GetString(common.RequestIdKey)
					if publicResponseID == "resp_" {
						publicResponseID = "resp_incomplete"
					}
				}
				if responseModel == "" {
					responseModel = info.ClientResponseModelName()
				}
				failure := dto.ResponsesStreamResponse{
					Type:           "response.failed",
					SequenceNumber: nextSequenceNumber,
					Response: &dto.OpenAIResponsesResponse{
						ID:     publicResponseID,
						Object: "response",
						Status: []byte(`"failed"`),
						Error: map[string]any{
							"code":    responsesIncompletePublicCode,
							"message": responsesIncompletePublicMessage,
						},
						Model:      responseModel,
						Output:     []dto.ResponsesOutput{},
						ToolChoice: []byte(`"auto"`),
						Tools:      []map[string]any{},
						Truncation: []byte(`"disabled"`),
						Metadata:   []byte(`{}`),
					},
				}
				failureData, err := common.Marshal(&failure)
				if err != nil {
					logger.LogError(c, "failed to marshal incomplete responses stream event: "+err.Error())
					info.StreamStatus.RecordError(err.Error())
				} else if err := sendResponsesStreamData(c, failure, string(failureData)); err != nil {
					logger.LogError(c, "failed to send incomplete responses stream event: "+err.Error())
					info.StreamStatus.RecordError(err.Error())
				}
			}
		}
	}

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if !info.StreamTerminalUsageSeen {
		usage.UsageSource = "estimated"
	}
	if usage.PromptTokens == 0 && !info.StreamTerminalUsageSeen {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if pendingEstimatedTerminal && !imageGenerationSeen && c.Request.Context().Err() == nil && !info.StreamEstimatedTerminalSent {
		sequenceNumber := int64(0)
		if nextSequenceNumber != nil {
			sequenceNumber = *nextSequenceNumber
		}
		if err := EmitEstimatedGPTStreamTerminal(c, info, usage, publishedResponseID, 0, info.ClientResponseModelName(), "", sequenceNumber); err != nil {
			logger.LogError(c, "failed to emit estimated responses terminal: "+err.Error())
		}
	}

	return usage, nil
}

func normalizeCompletedImageGenerationStatus(data []byte) ([]byte, bool) {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return data, false
	}
	shouldNormalize := func(item gjson.Result) bool {
		if !item.Exists() || !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != dto.ResponsesOutputTypeImageGenerationCall {
			return false
		}
		switch strings.TrimSpace(item.Get("status").String()) {
		case "generating", "in_progress":
			return strings.TrimSpace(item.Get("result").String()) != ""
		default:
			return false
		}
	}

	switch strings.TrimSpace(gjson.GetBytes(data, "type").String()) {
	case dto.ResponsesOutputTypeItemDone:
		if !shouldNormalize(gjson.GetBytes(data, "item")) {
			return data, false
		}
		updated, err := sjson.SetBytes(data, "item.status", "completed")
		return updated, err == nil
	case "response.completed", "response.done":
		output := gjson.GetBytes(data, "response.output")
		if !output.Exists() || !output.IsArray() {
			return data, false
		}
		updated := data
		changed := false
		for i, item := range output.Array() {
			if !shouldNormalize(item) {
				continue
			}
			next, err := sjson.SetBytes(updated, "response.output."+strconv.Itoa(i)+".status", "completed")
			if err != nil {
				return data, false
			}
			updated = next
			changed = true
		}
		return updated, changed
	default:
		return data, false
	}
}
