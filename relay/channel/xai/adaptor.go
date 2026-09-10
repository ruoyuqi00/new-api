package xai

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	projectconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	//panic("implement me")
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//not available
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	aspectRatio, resolution, err := xAIImageParameters(request)
	if err != nil {
		return nil, err
	}
	xaiRequest := ImageRequest{
		Model:          request.Model,
		Prompt:         request.Prompt,
		N:              int(lo.FromPtrOr(request.N, uint(1))),
		AspectRatio:    aspectRatio,
		Resolution:     resolution,
		ResponseFormat: request.ResponseFormat,
	}
	return xaiRequest, nil
}

func xAIImageParameters(request dto.ImageRequest) (string, string, error) {
	aspectRatio := strings.TrimSpace(lo.FromPtrOr(request.AspectRatio, ""))
	if strings.EqualFold(aspectRatio, "auto") {
		aspectRatio = ""
	} else if aspectRatio != "" {
		parts := strings.Split(aspectRatio, ":")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid xAI image aspect ratio %q", aspectRatio)
		}
		width, widthErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		height, heightErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
			return "", "", fmt.Errorf("invalid xAI image aspect ratio %q", aspectRatio)
		}
		aspectRatio = closestXAIImageAspectRatio(request.Model, width/height)
	}
	size := strings.ToLower(strings.TrimSpace(request.Size))
	switch size {
	case "", "auto":
		return aspectRatio, "", nil
	case "1k", "2k":
		return aspectRatio, size, nil
	case "4k":
		return "", "", errors.New("xAI image generation supports resolutions up to 2k")
	}

	canonical := strings.NewReplacer("*", "x", "×", "x").Replace(size)
	parts := strings.Split(canonical, "x")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid xAI image size %q", request.Size)
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", "", fmt.Errorf("invalid xAI image size %q", request.Size)
	}
	if width > 2048 || height > 2048 {
		return "", "", fmt.Errorf("xAI image size %q exceeds the 2048x2048 limit", request.Size)
	}

	resolution := "1k"
	if width > 1024 || height > 1024 {
		resolution = "2k"
	}
	if aspectRatio != "" {
		return aspectRatio, resolution, nil
	}

	return closestXAIImageAspectRatio(request.Model, float64(width)/float64(height)), resolution, nil
}

func closestXAIImageAspectRatio(modelName string, requested float64) string {
	type supportedRatio struct {
		name  string
		value float64
	}
	supported := []supportedRatio{
		{name: "1:1", value: 1},
		{name: "16:9", value: 16.0 / 9.0},
		{name: "9:16", value: 9.0 / 16.0},
		{name: "4:3", value: 4.0 / 3.0},
		{name: "3:4", value: 3.0 / 4.0},
		{name: "3:2", value: 3.0 / 2.0},
		{name: "2:3", value: 2.0 / 3.0},
		{name: "2:1", value: 2},
		{name: "1:2", value: 0.5},
		{name: "19.5:9", value: 19.5 / 9.0},
		{name: "9:19.5", value: 9.0 / 19.5},
		{name: "20:9", value: 20.0 / 9.0},
		{name: "9:20", value: 9.0 / 20.0},
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "grok-imagine-image-2") {
		supported = append(supported,
			supportedRatio{name: "21:9", value: 21.0 / 9.0},
			supportedRatio{name: "5:2", value: 2.5},
		)
	}
	closest := supported[0]
	closestDistance := math.Abs(math.Log(requested / closest.value))
	for _, candidate := range supported[1:] {
		distance := math.Abs(math.Log(requested / candidate.value))
		if distance < closestDistance {
			closest = candidate
			closestDistance = distance
		}
	}
	return closest.name
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	apiKey := strings.TrimSpace(info.ApiKey)
	if strings.HasPrefix(apiKey, "{") {
		var oauthCredential struct {
			AccessToken string `json:"access_token"`
		}
		if err := common.UnmarshalJsonStr(apiKey, &oauthCredential); err != nil {
			return errors.New("xAI OAuth credential is not valid JSON")
		}
		apiKey = strings.TrimSpace(oauthCredential.AccessToken)
		if apiKey == "" {
			return errors.New("xAI OAuth credential requires access_token")
		}
	}
	req.Set("Authorization", "Bearer "+apiKey)
	if strings.HasPrefix(strings.TrimSpace(info.ApiKey), "{") {
		req.Set("X-XAI-Token-Auth", "xai-grok-cli")
		req.Set("x-grok-client-version", projectconstant.GrokCLIClientVersion)
		req.Set("User-Agent", projectconstant.GrokCLIUserAgent)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if strings.HasSuffix(info.UpstreamModelName, "-search") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-search")
		request.Model = info.UpstreamModelName
		toMap := request.ToMap()
		toMap["search_parameters"] = map[string]any{
			"mode": "on",
		}
		return toMap, nil
	}
	if strings.HasPrefix(request.Model, "grok-3-mini") {
		if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = nil
		}
		if strings.HasSuffix(request.Model, "-high") {
			request.ReasoningEffort = "high"
			request.Model = strings.TrimSuffix(request.Model, "-high")
		} else if strings.HasSuffix(request.Model, "-low") {
			request.ReasoningEffort = "low"
			request.Model = strings.TrimSuffix(request.Model, "-low")
		}
		info.ReasoningEffort = request.ReasoningEffort
		info.UpstreamModelName = request.Model
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//not available
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if request.Model == "" && info != nil {
		request.Model = info.UpstreamModelName
	}
	if xAIModelRejectsReasoning(request.Model) {
		request.Reasoning = nil
	}
	request.PromptCacheRetention = nil
	request.SafetyIdentifier = nil
	input, err := sanitizeXAIResponsesInput(request.Input)
	if err != nil {
		return nil, err
	}
	request.Input = input
	request.Tools, request.ToolChoice, err = sanitizeXAIResponsesTools(request.Tools, request.ToolChoice)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
		usage, err = openai.OpenaiImageHandler(c, info, resp)
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	default:
		if info.IsStream {
			usage, err = xAIStreamHandler(c, info, resp)
		} else {
			usage, err = xAIHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
