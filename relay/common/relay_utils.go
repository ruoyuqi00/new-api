package common

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

// MaxTaskDurationSeconds caps user-supplied video duration before it can be
// used as a billing multiplier through task OtherRatios.
const MaxTaskDurationSeconds = 3600

func validateTaskDurationBounds(req TaskSubmitReq) *dto.TaskError {
	if taskErr := validateTaskDurationValue("duration", req.Duration, false); taskErr != nil {
		return taskErr
	}
	if taskErr := validateTaskDurationValue("seconds", req.Seconds, true); taskErr != nil {
		return taskErr
	}
	return validateMetadataTaskDurationBounds(req.Metadata)
}

func validateMetadataTaskDurationBounds(metadata map[string]interface{}) *dto.TaskError {
	if metadata == nil {
		return nil
	}
	for _, key := range []string{"duration", "durationSeconds"} {
		if value, ok := metadata[key]; ok {
			if taskErr := validateTaskDurationValue("metadata."+key, value, true); taskErr != nil {
				return taskErr
			}
		}
	}
	if rawParameters, ok := metadata["parameters"]; ok {
		if parameters, ok := rawParameters.(map[string]interface{}); ok {
			for _, key := range []string{"duration", "durationSeconds"} {
				if value, exists := parameters[key]; exists {
					if taskErr := validateTaskDurationValue("metadata.parameters."+key, value, true); taskErr != nil {
						return taskErr
					}
				}
			}
		}
	}
	return nil
}

func validateTaskDurationValue(field string, value interface{}, allowEmpty bool) *dto.TaskError {
	seconds, exists, err := parseTaskDurationValue(value, allowEmpty)
	if err != nil || !exists {
		if !exists {
			return nil
		}
		return invalidTaskDurationError(field)
	}
	if seconds < 0 || seconds > MaxTaskDurationSeconds {
		return invalidTaskDurationError(field)
	}
	return nil
}

func invalidTaskDurationError(field string) *dto.TaskError {
	return createTaskError(
		fmt.Errorf("%s must be an integer between 0 and %d", field, MaxTaskDurationSeconds),
		"invalid_seconds",
		http.StatusBadRequest,
		true,
	)
}

func parseTaskDurationValue(value interface{}, allowEmpty bool) (int, bool, error) {
	switch v := value.(type) {
	case nil:
		return 0, false, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" && allowEmpty {
			return 0, false, nil
		}
		seconds, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, true, err
		}
		return seconds, true, nil
	case int:
		return v, true, nil
	case int8:
		return int(v), true, nil
	case int16:
		return int(v), true, nil
	case int32:
		return int(v), true, nil
	case int64:
		if v > int64(MaxTaskDurationSeconds) {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		if v < 0 {
			return -1, true, nil
		}
		return int(v), true, nil
	case uint:
		if v > uint(MaxTaskDurationSeconds) {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		return int(v), true, nil
	case uint8:
		return int(v), true, nil
	case uint16:
		return int(v), true, nil
	case uint32:
		if v > uint32(MaxTaskDurationSeconds) {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		return int(v), true, nil
	case uint64:
		if v > uint64(MaxTaskDurationSeconds) {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		return int(v), true, nil
	case float32:
		return parseTaskDurationFloat(float64(v))
	case float64:
		return parseTaskDurationFloat(v)
	default:
		return 0, true, fmt.Errorf("unsupported duration value %T", value)
	}
}

func parseTaskDurationFloat(value float64) (int, bool, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, true, fmt.Errorf("duration must be an integer")
	}
	if value > float64(MaxTaskDurationSeconds) {
		return MaxTaskDurationSeconds + 1, true, nil
	}
	if value < 0 {
		return -1, true, nil
	}
	return int(value), true, nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:   formData.Get("prompt"),
		Model:    formData.Get("model"),
		Mode:     formData.Get("mode"),
		Image:    formData.Get("image"),
		Size:     formData.Get("size"),
		Metadata: make(map[string]interface{}),
	}

	if durationStr := formData.Get("seconds"); durationStr != "" {
		req.Seconds = durationStr
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	} else if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{strings.TrimSpace(req.Image)}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":          true,
		"model":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"duration":        true,
		"input_reference": true, // Sora 特有字段
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	}
	// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskDurationBounds(req); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}
