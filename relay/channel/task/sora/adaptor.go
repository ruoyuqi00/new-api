package sora

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // for image_url type
}

type ImageURL struct {
	URL string `json:"url"`
}

type responseTask struct {
	ID                 string `json:"id"`
	TaskID             string `json:"task_id,omitempty"` //兼容旧接口
	Object             string `json:"object"`
	Model              string `json:"model"`
	Status             string `json:"status"`
	Progress           int    `json:"progress"`
	CreatedAt          int64  `json:"created_at"`
	CompletedAt        int64  `json:"completed_at,omitempty"`
	ExpiresAt          int64  `json:"expires_at,omitempty"`
	Seconds            string `json:"seconds,omitempty"`
	Size               string `json:"size,omitempty"`
	RemixedFromVideoID string `json:"remixed_from_video_id,omitempty"`
	Error              *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

const (
	moonRequestContextKey        = "moon_video_request"
	moonRateRatioKey             = "moon_token_rate"
	moonTokenReservationRatioKey = "moon_token_reservation"
	moonResolutionRatioKey       = "moon_resolution"
)

type moonVideoRequest struct {
	Model           string                   `json:"model"`
	Prompt          string                   `json:"prompt"`
	Content         []map[string]interface{} `json:"content"`
	Duration        interface{}              `json:"duration"`
	Seconds         interface{}              `json:"seconds"`
	Resolution      string                   `json:"resolution"`
	Size            string                   `json:"size"`
	Videos          []moonVideoMedia         `json:"videos"`
	ReferenceVideos []moonVideoMedia         `json:"reference_videos"`
	Materials       []moonVideoMedia         `json:"materials"`
}

type moonVideoMedia struct {
	Type            string      `json:"type"`
	Role            string      `json:"role"`
	DurationSeconds interface{} `json:"durationSeconds"`
}

type moonStatusResponse struct {
	Status   string `json:"status"`
	Model    string `json:"model"`
	URL      string `json:"url"`
	Metadata struct {
		URL string `json:"url"`
	} `json:"metadata"`
	Output struct {
		ContentURL string `json:"content_url"`
	} `json:"output"`
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func validateRemixRequest(c *gin.Context) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	// 存储原始请求到 context，与 ValidateMultipartDirect 路径保持一致
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if a.isMoonGateway() {
		return validateMoonVideoRequest(c, info)
	}
	if info.Action == constant.TaskActionRemix {
		return validateRemixRequest(c)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

// EstimateBilling 根据用户请求的 seconds 和 size 计算 OtherRatios。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if a.isMoonGateway() {
		return estimateMoonVideoBilling(c, info)
	}
	// remix 路径的 OtherRatios 已在 ResolveOriginTask 中设置
	if info.Action == constant.TaskActionRemix {
		return nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 4
	}

	size := req.Size
	if size == "" {
		size = "720x1280"
	}

	ratios := map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
	if size == "1792x1024" || size == "1024x1792" {
		ratios["size"] = 1.666667
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.Action == constant.TaskActionRemix {
		return fmt.Sprintf("%s/v1/videos/%s/remix", a.baseURL, info.OriginTaskID), nil
	}
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	if a.isMoonGateway() {
		idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if idempotencyKey == "" && info.TaskRelayInfo != nil {
			idempotencyKey = info.PublicTaskID
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Moon-API/1.0")
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			bodyMap["model"] = info.UpstreamModelName
			upstreamModel := info.UpstreamModelName
			if upstreamModel == "" {
				upstreamModel = info.OriginModelName
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(upstreamModel)), "veo-3-1") {
				if resolution, ok := bodyMap["resolution"].(string); ok && strings.EqualFold(strings.TrimSpace(resolution), "auto") {
					bodyMap["resolution"] = "720p"
				}
			}
			if newBody, err := common.Marshal(bodyMap); err == nil {
				return bytes.NewReader(newBody), nil
			}
		}
		return bytes.NewReader(cachedBody), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("model", info.UpstreamModelName)
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, v := range values {
				writer.WriteField(key, v)
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					// Re-open after sniffing so the full content is copied below
					f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					f.Close()
					continue
				}
				io.Copy(part, f)
				f.Close()
			}
		}
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.NewReplayableBodyReader(storage), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Sora response
	var dResp responseTask
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.ID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 使用公开 task_xxxx ID 返回给客户端
	dResp.ID = info.PublicTaskID
	dResp.TaskID = info.PublicTaskID
	c.JSON(http.StatusOK, dResp)
	return upstreamID, common.SanitizeVideoTaskResponse(responseBody, info.PublicTaskID), nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	if a.isMoonGateway() {
		return parseMoonTaskResult(respBody)
	}
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch resTask.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		// Url intentionally left empty — the caller constructs the proxy URL using the public task ID
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
	}
	if resTask.Progress > 0 && resTask.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", resTask.Progress)
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if !a.isMoonGateway() || task == nil || taskResult == nil || taskResult.TotalTokens <= 0 {
		return 0
	}
	modelName := task.Properties.OriginModelName
	if task.PrivateData.BillingContext != nil && task.PrivateData.BillingContext.OriginModelName != "" {
		modelName = task.PrivateData.BillingContext.OriginModelName
	}
	if !isMoonSeedanceTokenModel(modelName) {
		return 0
	}
	billing := task.PrivateData.BillingContext
	if billing == nil || billing.ModelRatio <= 0 || billing.GroupRatio <= 0 {
		return 0
	}
	rateRatio := 1.0
	if billing.OtherRatios != nil && billing.OtherRatios[moonRateRatioKey] > 0 {
		rateRatio = billing.OtherRatios[moonRateRatioKey]
	}
	quota, _ := common.QuotaRoundChecked(float64(taskResult.TotalTokens) * billing.ModelRatio * billing.GroupRatio * rateRatio)
	return quota
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	if a.isMoonGateway() || isMoonVideoModel(task.Properties.OriginModelName) {
		video := task.ToOpenAIVideo()
		if task.Status == model.TaskStatusFailure {
			video.Error = &dto.OpenAIVideoError{Message: task.FailReason, Code: "moon_video_failed"}
		}
		return common.Marshal(video)
	}
	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	return data, nil
}

func (a *TaskAdaptor) isMoonGateway() bool {
	parsed, err := url.Parse(a.baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "moon.sixai.cc")
}

func validateMoonVideoRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var request moonVideoRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if !isMoonVideoModel(request.Model) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model %s is not priced yet", request.Model), "model_not_priced", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Prompt) == "" && !moonContentHasText(request.Content) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("prompt or text content is required"), "invalid_request", http.StatusBadRequest)
	}
	duration, err := moonDuration(request)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if request.Model == "minimax-h3" {
		if request.Duration == nil && request.Seconds == nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("H3 duration is required"), "invalid_seconds", http.StatusBadRequest)
		}
		if duration < 4 || duration > 15 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("H3 duration must be between 4 and 15 seconds"), "invalid_seconds", http.StatusBadRequest)
		}
		if !moonH3TierSupported(request.Size, request.Resolution) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported H3 pricing tier"), "unsupported_pricing_tier", http.StatusBadRequest)
		}
	} else if isMoonSeedanceTokenModel(request.Model) {
		maxDuration := 15
		if request.Model == "seedance-2-5-official" {
			maxDuration = 30
		}
		if (request.Duration != nil || request.Seconds != nil) && duration != -1 && (duration < 4 || duration > maxDuration) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("Seedance duration must be -1 or between 4 and %d seconds", maxDuration), "invalid_seconds", http.StatusBadRequest)
		}
		resolution := strings.ToLower(strings.TrimSpace(request.Resolution))
		if resolution == "" {
			resolution = strings.ToLower(strings.TrimSpace(request.Size))
		}
		if _, _, ok := moonSeedanceTokenRate(request.Model, resolution, moonRequestHasVideo(request)); !ok {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported Seedance pricing tier"), "unsupported_pricing_tier", http.StatusBadRequest)
		}
	} else {
		if duration <= 0 {
			duration = moonDefaultDuration(request.Model)
		}
		if !moonDurationSupported(request.Model, duration) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported %s duration", request.Model), "invalid_seconds", http.StatusBadRequest)
		}
		if request.Model != "grok-v1.5-video" {
			if _, _, ok := moonPerSecondRate(request.Model, request.Size, request.Resolution); !ok {
				return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported %s pricing tier", request.Model), "unsupported_pricing_tier", http.StatusBadRequest)
			}
		} else if !moonGrokResolutionSupported(request.Size, request.Resolution) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported Grok pricing tier"), "unsupported_pricing_tier", http.StatusBadRequest)
		}
	}
	action := constant.TaskActionTextGenerate
	if moonRequestHasReference(request) {
		action = constant.TaskActionGenerate
	}
	info.Action = action
	c.Set(moonRequestContextKey, request)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:    request.Model,
		Prompt:   request.Prompt,
		Duration: duration,
	})
	return nil
}

func moonContentHasText(content []map[string]interface{}) bool {
	for _, item := range content {
		if item["type"] == "text" {
			if text, ok := item["text"].(string); ok && strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}

func moonRequestHasReference(request moonVideoRequest) bool {
	if len(request.Videos) > 0 || len(request.ReferenceVideos) > 0 || len(request.Materials) > 0 {
		return true
	}
	for _, item := range request.Content {
		itemType, _ := item["type"].(string)
		if itemType != "" && itemType != "text" {
			return true
		}
	}
	return false
}

func estimateMoonVideoBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	value, ok := c.Get(moonRequestContextKey)
	if !ok {
		return nil
	}
	request, ok := value.(moonVideoRequest)
	if !ok {
		return nil
	}
	modelName := info.OriginModelName
	if modelName == "" {
		modelName = request.Model
	}
	duration, _ := moonDuration(request)
	if modelName == "minimax-h3" {
		if duration <= 0 {
			duration = 4
		}
		resolutionRatio := moonH3ResolutionRate(request.Size, request.Resolution) / 0.10
		return map[string]float64{
			"seconds":              float64(duration),
			moonResolutionRatioKey: resolutionRatio,
		}
	}
	if modelName == "grok-v1.5-video" {
		return nil
	}
	if isMoonPerSecondModel(modelName) {
		if duration <= 0 {
			duration = moonDefaultDuration(modelName)
		}
		rate, baseRate, ok := moonPerSecondRate(modelName, request.Size, request.Resolution)
		if !ok {
			return nil
		}
		billableSeconds := float64(duration)
		if isMoonWanModel(modelName) {
			billableSeconds += moonReferenceVideoSeconds(request)
		}
		return map[string]float64{
			"seconds":              billableSeconds,
			moonResolutionRatioKey: rate / baseRate,
		}
	}
	if !isMoonSeedanceTokenModel(modelName) {
		return nil
	}
	resolution := strings.ToLower(strings.TrimSpace(request.Resolution))
	if resolution == "" {
		resolution = strings.ToLower(strings.TrimSpace(request.Size))
	}
	hasVideo := moonRequestHasVideo(request)
	rate, baseRate, ok := moonSeedanceTokenRate(modelName, resolution, hasVideo)
	if !ok {
		return nil
	}
	if duration == -1 {
		if modelName == "seedance-2-5-official" {
			duration = 30
		} else {
			duration = 15
		}
	} else if duration <= 0 {
		duration = 5
	}
	totalSeconds := float64(duration) + moonReferenceVideoSeconds(request)
	estimatedTokens := totalSeconds * moonResolutionPixels(resolution) * 24 / 1024
	reservationRatio := estimatedTokens * 2 / common.QuotaPerUnit
	if reservationRatio <= 0 {
		reservationRatio = 1
	}
	return map[string]float64{
		moonRateRatioKey:             rate / baseRate,
		moonTokenReservationRatioKey: reservationRatio,
	}
}

func isMoonSeedanceTokenModel(modelName string) bool {
	switch modelName {
	case "seedance-2-0-mini-official", "seedance-2-0-fast-official", "seedance-2-0-official", "seedance-2-5-official":
		return true
	default:
		return false
	}
}

func isMoonVideoModel(modelName string) bool {
	switch modelName {
	case "minimax-h3",
		"wan3.0-video",
		"wan3.0-video-prime",
		"grok-v1.5-video",
		"seedance2.0-9-3-3-PT",
		"seedance2.5-30-10-10-PT",
		"seedance2.0-fast-PT":
		return true
	default:
		return isMoonSeedanceTokenModel(modelName)
	}
}

func isMoonWanModel(modelName string) bool {
	return modelName == "wan3.0-video" || modelName == "wan3.0-video-prime"
}

func isMoonPerSecondModel(modelName string) bool {
	return isMoonWanModel(modelName) ||
		modelName == "seedance2.0-9-3-3-PT" ||
		modelName == "seedance2.5-30-10-10-PT" ||
		modelName == "seedance2.0-fast-PT"
}

func moonDefaultDuration(modelName string) int {
	if modelName == "grok-v1.5-video" {
		return 6
	}
	return 5
}

func moonDurationSupported(modelName string, duration int) bool {
	switch modelName {
	case "wan3.0-video", "wan3.0-video-prime":
		return duration >= 2 && duration <= 30
	case "seedance2.0-9-3-3-PT", "seedance2.0-fast-PT":
		return duration >= 5 && duration <= 15
	case "seedance2.5-30-10-10-PT":
		return duration >= 5 && duration <= 30
	case "grok-v1.5-video":
		return duration >= 4 && duration <= 15
	default:
		return false
	}
}

func moonPerSecondRate(modelName, size, resolution string) (float64, float64, bool) {
	tier := moonPerSecondResolutionTier(size, resolution)
	prices := map[string]map[string]float64{
		"wan3.0-video":            {"480p": 0.27, "720p": 0.36, "1080p": 0.72},
		"wan3.0-video-prime":      {"480p": 0.40, "720p": 0.54, "1080p": 1.08},
		"seedance2.0-9-3-3-PT":    {"480p": 0.34, "720p": 0.42},
		"seedance2.5-30-10-10-PT": {"480p": 0.45, "720p": 0.67},
		"seedance2.0-fast-PT":     {"480p": 0.30, "720p": 0.36},
	}
	modelPrices, ok := prices[modelName]
	if !ok {
		return 0, 0, false
	}
	rate, ok := modelPrices[tier]
	if !ok {
		return 0, 0, false
	}
	return rate, modelPrices["480p"], true
}

func moonPerSecondResolutionTier(size, resolution string) string {
	tier := strings.ToLower(strings.TrimSpace(resolution))
	if tier == "" {
		tier = strings.ToLower(strings.TrimSpace(size))
	}
	if tier == "" {
		return "720p"
	}
	switch tier {
	case "854x480", "832x480", "864x480", "480x854":
		return "480p"
	case "1280x720", "720x1280":
		return "720p"
	case "1920x1080", "1080x1920":
		return "1080p"
	default:
		return tier
	}
}

func moonGrokResolutionSupported(size, resolution string) bool {
	switch moonPerSecondResolutionTier(size, resolution) {
	case "720p", "1080p":
		return true
	default:
		return false
	}
}

func moonSeedanceTokenRate(modelName, resolution string, hasVideo bool) (float64, float64, bool) {
	type ratePair struct{ withoutVideo, withVideo float64 }
	prices := map[string]map[string]ratePair{
		"seedance-2-0-mini-official": {"480p": {11.5, 7}, "720p": {11.5, 7}},
		"seedance-2-0-fast-official": {"480p": {24.05, 14.3}, "720p": {24.05, 14.3}},
		"seedance-2-0-official": {
			"480p": {36.8, 22.4}, "720p": {36.8, 22.4}, "1080p": {40.8, 24.8}, "4k": {20.8, 12.8},
		},
		"seedance-2-5-official": {"720p": {59.5, 35.7}, "1080p": {65.45, 39.1}},
	}
	modelPrices, ok := prices[modelName]
	if !ok {
		return 0, 0, false
	}
	if resolution == "" {
		resolution = "720p"
	}
	pair, ok := modelPrices[resolution]
	if !ok {
		return 0, 0, false
	}
	base := modelPrices["720p"].withoutVideo
	if hasVideo {
		return pair.withVideo, base, true
	}
	return pair.withoutVideo, base, true
}

func moonDuration(request moonVideoRequest) (int, error) {
	duration, hasDuration, err := moonDurationValue(request.Duration)
	if err != nil {
		return 0, err
	}
	seconds, hasSeconds, err := moonDurationValue(request.Seconds)
	if err != nil {
		return 0, err
	}
	if hasDuration && hasSeconds && duration != seconds {
		return 0, fmt.Errorf("duration and seconds must match when both are provided")
	}
	if hasDuration {
		return duration, nil
	}
	if hasSeconds {
		return seconds, nil
	}
	return 0, nil
}

func moonDurationValue(value interface{}) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	switch duration := value.(type) {
	case float64:
		if duration != float64(int(duration)) {
			return 0, true, fmt.Errorf("duration must be an integer")
		}
		return int(duration), true, nil
	case string:
		seconds, err := strconv.Atoi(strings.TrimSpace(duration))
		if err != nil {
			return 0, true, fmt.Errorf("duration must be an integer")
		}
		return seconds, true, nil
	default:
		return 0, true, fmt.Errorf("duration must be an integer")
	}
}

func moonRequestHasVideo(request moonVideoRequest) bool {
	if len(request.Videos) > 0 || len(request.ReferenceVideos) > 0 {
		return true
	}
	for _, media := range request.Materials {
		if strings.EqualFold(media.Type, "video") || strings.EqualFold(media.Role, "reference_video") {
			return true
		}
	}
	for _, item := range request.Content {
		itemType, _ := item["type"].(string)
		if itemType == "video_url" {
			return true
		}
		if _, ok := item["video_url"]; ok {
			return true
		}
	}
	return false
}

func moonReferenceVideoSeconds(request moonVideoRequest) float64 {
	total := 0.0
	for _, media := range append(append([]moonVideoMedia{}, request.Videos...), request.ReferenceVideos...) {
		total += moonNumericValue(media.DurationSeconds)
	}
	for _, media := range request.Materials {
		if strings.EqualFold(media.Type, "video") || strings.EqualFold(media.Role, "reference_video") {
			total += moonNumericValue(media.DurationSeconds)
		}
	}
	for _, item := range request.Content {
		itemType, _ := item["type"].(string)
		if itemType == "video_url" {
			total += moonNumericValue(item["durationSeconds"])
		}
	}
	return total
}

func moonNumericValue(value interface{}) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed
	default:
		return 0
	}
}

func moonResolutionPixels(resolution string) float64 {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480p":
		return 854 * 480
	case "1080p":
		return 1920 * 1080
	case "4k":
		return 3840 * 2160
	default:
		return 1280 * 720
	}
}

func moonH3ResolutionRate(size, resolution string) float64 {
	tier := strings.ToLower(strings.TrimSpace(size))
	if tier == "" {
		tier = strings.ToLower(strings.TrimSpace(resolution))
	}
	switch tier {
	case "4k":
		return 0.36
	case "2k":
		return 0.26
	case "1080p", "1920x1088", "1088x1920", "1440x1440", "1184x1760", "1760x1184", "1248x1664", "1664x1248", "2208x960":
		return 0.18
	case "768p", "1376x768", "768x1376", "1024x1024", "832x1248", "1248x832", "896x1184", "1184x896", "1568x672":
		return 0.16
	default:
		return 0.10
	}
}

func moonH3TierSupported(size, resolution string) bool {
	tier := strings.ToLower(strings.TrimSpace(size))
	if tier == "" {
		tier = strings.ToLower(strings.TrimSpace(resolution))
	}
	switch tier {
	case "480p", "864x480", "480x864", "640x640", "544x800", "800x544", "576x736", "736x576", "992x416",
		"768p", "1376x768", "768x1376", "1024x1024", "832x1248", "1248x832", "896x1184", "1184x896", "1568x672",
		"1080p", "1920x1088", "1088x1920", "1440x1440", "1184x1760", "1760x1184", "1248x1664", "1664x1248", "2208x960",
		"2k", "4k":
		return true
	default:
		return false
	}
}

func parseMoonTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var response moonStatusResponse
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal Moon video response failed")
	}
	result := &relaycommon.TaskInfo{Code: 0, TotalTokens: response.Usage.TotalTokens}
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "pending", "queued", "reserving", "submitted":
		result.Status = model.TaskStatusQueued
		result.Progress = taskcommon.ProgressQueued
	case "processing", "running", "in_progress", "usage_pending", "funding_pending", "settlement_balance_required", "commit_pending", "refund_pending", "submitting_unknown", "reservation_unknown":
		result.Status = model.TaskStatusInProgress
		result.Progress = taskcommon.ProgressInProgress
	case "done", "completed", "success", "succeeded":
		if isMoonSeedanceTokenModel(response.Model) && response.Usage.TotalTokens <= 0 {
			result.Status = model.TaskStatusInProgress
			result.Progress = taskcommon.ProgressInProgress
			return result, nil
		}
		result.Status = model.TaskStatusSuccess
		result.Progress = taskcommon.ProgressComplete
		result.Url = strings.TrimSpace(response.URL)
		if result.Url == "" {
			result.Url = strings.TrimSpace(response.Metadata.URL)
		}
		if result.Url == "" {
			result.Url = strings.TrimSpace(response.Output.ContentURL)
		}
		if result.Url == "" && len(response.Data) > 0 {
			result.Url = strings.TrimSpace(response.Data[0].URL)
		}
	case "failed", "cancelled", "canceled", "expired":
		result.Status = model.TaskStatusFailure
		result.Progress = taskcommon.ProgressComplete
		result.Reason = "Moon video task " + strings.ToLower(strings.TrimSpace(response.Status))
		if response.Error != nil && strings.TrimSpace(response.Error.Message) != "" {
			result.Reason = response.Error.Message
		}
	default:
		return nil, fmt.Errorf("unknown Moon video status: %s", response.Status)
	}
	return result, nil
}
