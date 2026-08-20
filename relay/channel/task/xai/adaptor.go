package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
)

type imageInput struct {
	URL string `json:"url"`
}

type requestPayload struct {
	Model       string      `json:"model"`
	Prompt      string      `json:"prompt"`
	Image       *imageInput `json:"image,omitempty"`
	Duration    int         `json:"duration,omitempty"`
	AspectRatio string      `json:"aspect_ratio,omitempty"`
	Resolution  string      `json:"resolution,omitempty"`
}

type startResponse struct {
	RequestID string `json:"request_id"`
}

type videoResult struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration,omitempty"`
}

type errorResult struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type statusResponse struct {
	Status string       `json:"status"`
	Model  string       `json:"model,omitempty"`
	Video  *videoResult `json:"video,omitempty"`
	Error  *errorResult `json:"error,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return map[string]float64{"seconds": 5}
	}
	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds <= 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 5
	}
	if seconds > 15 {
		seconds = 15
	}
	return map[string]float64{"seconds": float64(seconds)}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/videos/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}
	payload, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()
	var started startResponse
	if err := common.Unmarshal(responseBody, &started); err != nil {
		return "", responseBody, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	if started.RequestID == "" {
		return "", responseBody, service.TaskErrorWrapperLocal(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusBadGateway)
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return started.RequestID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var result statusResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal xai video response failed")
	}
	taskResult := &relaycommon.TaskInfo{Code: 0}
	switch strings.ToLower(result.Status) {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "done", "completed", "success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if result.Video != nil {
			videoURL := strings.TrimSpace(result.Video.URL)
			if strings.HasPrefix(videoURL, "/") {
				videoURL = strings.TrimRight(a.baseURL, "/") + videoURL
			}
			taskResult.Url = videoURL
		}
	case "failed", "expired", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "xAI video task " + strings.ToLower(result.Status)
		if result.Error != nil && result.Error.Message != "" {
			taskResult.Reason = result.Error.Message
		}
	default:
		return nil, fmt.Errorf("unknown xai video status: %s", result.Status)
	}
	return taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := task.ToOpenAIVideo()
	if task.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Message: task.FailReason, Code: "xai_video_failed"}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }

func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	duration := req.Duration
	if duration <= 0 {
		duration, _ = strconv.Atoi(req.Seconds)
	}
	if duration <= 0 {
		duration = 5
	}
	if duration > 15 {
		duration = 15
	}
	payload := &requestPayload{
		Model:       info.UpstreamModelName,
		Prompt:      req.Prompt,
		Duration:    duration,
		AspectRatio: aspectRatioFromSize(req.Size),
		Resolution:  resolutionFromSize(req.Size),
	}
	imageURL := strings.TrimSpace(req.Image)
	if imageURL == "" && len(req.Images) > 0 {
		imageURL = strings.TrimSpace(req.Images[0])
	}
	if imageURL != "" {
		payload.Image = &imageInput{URL: imageURL}
	}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, payload); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata to xai video request failed")
	}
	if payload.Model == "" {
		payload.Model = info.UpstreamModelName
	}
	return payload, nil
}

func aspectRatioFromSize(size string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return "16:9"
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return "16:9"
	}
	if w == h {
		return "1:1"
	}
	if w*9 == h*16 {
		return "16:9"
	}
	if w*16 == h*9 {
		return "9:16"
	}
	if w*3 == h*4 {
		return "4:3"
	}
	if w*4 == h*3 {
		return "3:4"
	}
	return "16:9"
}

func resolutionFromSize(size string) string {
	size = strings.ToLower(size)
	switch {
	case strings.Contains(size, "1080"):
		return "1080p"
	case strings.Contains(size, "720"):
		return "720p"
	case strings.Contains(size, "480"):
		return "480p"
	default:
		return "480p"
	}
}
