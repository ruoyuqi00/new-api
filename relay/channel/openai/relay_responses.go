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

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	normalizedBody, _, err := helper.NormalizeClientResponseModelJSON(info, responseBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	responseBody = normalizedBody
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.CacheWriteTokens = responsesResponse.Usage.InputTokensDetails.CacheWriteTokens
		}
	}
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

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	terminalReceived := false
	var nextSequenceNumber *int64
	responseID := ""
	responseModel := ""

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
		if streamResponse.SequenceNumber != nil {
			next := *streamResponse.SequenceNumber + 1
			nextSequenceNumber = &next
		}
		if streamResponse.Response != nil {
			if streamResponse.Response.ID != "" {
				responseID = streamResponse.Response.ID
			}
			if streamResponse.Response.Model != "" {
				responseModel = streamResponse.Response.Model
			}
		}
		switch streamResponse.Type {
		case "response.completed", "response.done", "response.incomplete", "response.failed", "response.error":
			terminalReceived = true
		}
		if err := sendResponsesStreamData(c, streamResponse, data); err != nil {
			sr.Stop(err)
			return
		}
		switch streamResponse.Type {
		case "response.completed", "response.done":
			if streamResponse.Response != nil {
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
						usage.PromptTokensDetails.CacheWriteTokens = streamResponse.Response.Usage.InputTokensDetails.CacheWriteTokens
					}
				}
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

	if !terminalReceived && c.Request.Context().Err() == nil && info.StreamStatus != nil {
		switch info.StreamStatus.EndReason {
		case relaycommon.StreamEndReasonClientGone, relaycommon.StreamEndReasonHandlerStop:
		default:
			const incompleteMessage = "Upstream stream ended before completion."
			info.PreservePreConsumedQuota = true
			info.StreamStatus.RecordError(incompleteMessage)
			if responseID == "" {
				responseID = "resp_" + c.GetString(common.RequestIdKey)
				if responseID == "resp_" {
					responseID = "resp_incomplete"
				}
			}
			if responseModel == "" {
				responseModel = info.ClientResponseModelName()
			}
			failure := dto.ResponsesStreamResponse{
				Type:           "response.failed",
				SequenceNumber: nextSequenceNumber,
				Response: &dto.OpenAIResponsesResponse{
					ID:         responseID,
					Object:     "response",
					Status:     []byte(`"failed"`),
					Error:      map[string]any{"code": "server_error", "message": incompleteMessage},
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
			} else {
				if err := sendResponsesStreamData(c, failure, string(failureData)); err != nil {
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

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

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
