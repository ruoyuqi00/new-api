package controller

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	maxYucoreMediaPromptLength             = 6000
	maxYucoreMediaMetadataBytes            = 256 * 1024
	maxYucoreMediaInputsBytes              = 512 * 1024
	maxYucoreMediaSessionIdRunes           = 96
	maxYucoreMediaImageUploadBytes   int64 = 25 << 20
	maxYucoreMediaAudioUploadBytes   int64 = 25 << 20
	maxYucoreMediaVideoUploadBytes   int64 = 100 << 20
	yucoreMediaUploadRequestMaxBytes int64 = maxYucoreMediaVideoUploadBytes + (1 << 20)
	yucoreMediaUploadSniffBytes            = 1 << 20
	yucoreMediaUploadFTYPMaxBytes          = 512
)

var errYucoreMediaUploadTooLarge = errors.New("reference upload exceeds the allowed size")

type yucoreMediaUploadPolicy struct {
	Kind      string
	MIMEType  string
	Extension string
	MaxBytes  int64
}

type yucoreMediaMPEGFrame struct {
	Length      int
	VersionBits byte
	LayerBits   byte
	SampleRate  int
}

type yucoreMediaTaskRequest struct {
	Group          string          `json:"group"`
	Kind           string          `json:"kind"`
	Mode           string          `json:"mode"`
	ModelId        string          `json:"model_id"`
	Prompt         string          `json:"prompt"`
	NegativePrompt *string         `json:"negative_prompt,omitempty"`
	AspectRatio    string          `json:"aspect_ratio"`
	Size           string          `json:"size"`
	Quality        string          `json:"quality"`
	Format         string          `json:"format"`
	Count          int             `json:"count"`
	Duration       *int            `json:"duration,omitempty"`
	Resolution     *string         `json:"resolution,omitempty"`
	GenerateAudio  *bool           `json:"generate_audio,omitempty"`
	Seed           *int64          `json:"seed,omitempty"`
	ReferenceMode  *string         `json:"reference_mode,omitempty"`
	SessionId      string          `json:"session_id"`
	Inputs         json.RawMessage `json:"inputs"`
	Metadata       json.RawMessage `json:"metadata"`
	Action         string          `json:"action"`
}

type yucoreMediaTaskResponse struct {
	Id             int                      `json:"id"`
	TaskId         string                   `json:"task_id"`
	UserId         int                      `json:"user_id"`
	SessionId      string                   `json:"session_id"`
	Group          string                   `json:"group"`
	Kind           string                   `json:"kind"`
	Mode           string                   `json:"mode"`
	ModelId        string                   `json:"model_id"`
	Prompt         string                   `json:"prompt"`
	NegativePrompt string                   `json:"negative_prompt"`
	AspectRatio    string                   `json:"aspect_ratio"`
	Size           string                   `json:"size"`
	Quality        string                   `json:"quality"`
	Format         string                   `json:"format"`
	Count          int                      `json:"count"`
	Status         string                   `json:"status"`
	Progress       int                      `json:"progress"`
	Cost           int                      `json:"cost"`
	Assets         []model.YucoreMediaAsset `json:"assets"`
	Inputs         json.RawMessage          `json:"inputs"`
	Metadata       json.RawMessage          `json:"metadata"`
	Error          string                   `json:"error"`
	CreatedTime    int64                    `json:"created_time"`
	UpdatedTime    int64                    `json:"updated_time"`
}

func normalizeYucoreMediaRawJSON(raw json.RawMessage, fallback string, maxBytes int, label string) (string, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	if len(raw) > maxBytes {
		return "", fmt.Errorf("media %s is too large", label)
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("media %s must be valid JSON", label)
	}
	return string(raw), nil
}

func rawYucoreMediaJSON(value string, fallback string) json.RawMessage {
	if value == "" || !json.Valid([]byte(value)) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(value)
}

func decodeYucoreMediaReferences(value string) ([]model.YucoreMediaReferenceInput, error) {
	rawReferences := make([]json.RawMessage, 0)
	if err := common.Unmarshal([]byte(value), &rawReferences); err != nil {
		return nil, errors.New("media inputs must be a JSON array")
	}
	references := make([]model.YucoreMediaReferenceInput, 0, len(rawReferences))
	for _, rawReference := range rawReferences {
		trimmed := strings.TrimSpace(string(rawReference))
		if strings.HasPrefix(trimmed, `"`) {
			var referenceURL string
			if err := common.Unmarshal(rawReference, &referenceURL); err != nil {
				return nil, errors.New("media input string must contain a valid URL")
			}
			references = append(references, model.YucoreMediaReferenceInput{Role: "image", URL: strings.TrimSpace(referenceURL)})
			continue
		}
		if !strings.HasPrefix(trimmed, "{") {
			return nil, errors.New("media inputs must contain strings or objects")
		}
		fields := make(map[string]json.RawMessage)
		if err := common.Unmarshal(rawReference, &fields); err != nil {
			return nil, errors.New("media input object is invalid")
		}
		reference := model.YucoreMediaReferenceInput{Role: "image"}
		roleFound := false
		for _, key := range []string{"role", "kind"} {
			rawValue, ok := fields[key]
			if !ok {
				continue
			}
			if err := common.Unmarshal(rawValue, &reference.Role); err != nil {
				return nil, fmt.Errorf("media input %s must be a string", key)
			}
			roleFound = true
			break
		}
		if !roleFound {
			reference.Role = "image"
		}
		for _, key := range []string{"dataUrl", "data_url", "cachedUrl", "cached_url", "sourceUrl", "source_url", "url", "path", "id"} {
			rawValue, ok := fields[key]
			if !ok {
				continue
			}
			var candidate string
			if err := common.Unmarshal(rawValue, &candidate); err != nil {
				return nil, fmt.Errorf("media input %s must be a string", key)
			}
			if strings.TrimSpace(candidate) != "" {
				reference.URL = strings.TrimSpace(candidate)
				break
			}
		}
		for _, key := range []string{"mime_type", "mimeType"} {
			rawValue, ok := fields[key]
			if !ok {
				continue
			}
			if err := common.Unmarshal(rawValue, &reference.MimeType); err != nil {
				return nil, fmt.Errorf("media input %s must be a string", key)
			}
			break
		}
		for _, key := range []string{"duration_ms", "durationMs"} {
			rawValue, ok := fields[key]
			if !ok || strings.TrimSpace(string(rawValue)) == "null" {
				continue
			}
			var duration int
			if err := common.Unmarshal(rawValue, &duration); err != nil {
				return nil, fmt.Errorf("media input %s must be an integer", key)
			}
			reference.DurationMS = &duration
			break
		}
		references = append(references, reference)
	}
	return references, nil
}

func decodeYucoreMediaMetadata(value string) (map[string]json.RawMessage, error) {
	metadata := make(map[string]json.RawMessage)
	if err := common.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, errors.New("media metadata must be a JSON object")
	}
	if metadata == nil {
		metadata = make(map[string]json.RawMessage)
	}
	return metadata, nil
}

func yucoreMediaMetadataInt(metadata map[string]json.RawMessage, keys ...string) (*int, error) {
	for _, key := range keys {
		raw, ok := metadata[key]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			continue
		}
		var value int
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("media metadata %s must be an integer", key)
		}
		return &value, nil
	}
	return nil, nil
}

func yucoreMediaMetadataInt64(metadata map[string]json.RawMessage, keys ...string) (*int64, error) {
	for _, key := range keys {
		raw, ok := metadata[key]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			continue
		}
		var value int64
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("media metadata %s must be an integer", key)
		}
		return &value, nil
	}
	return nil, nil
}

func yucoreMediaMetadataBool(metadata map[string]json.RawMessage, keys ...string) (*bool, error) {
	for _, key := range keys {
		raw, ok := metadata[key]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			continue
		}
		var value bool
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("media metadata %s must be a boolean", key)
		}
		return &value, nil
	}
	return nil, nil
}

func yucoreMediaMetadataString(metadata map[string]json.RawMessage, keys ...string) (*string, error) {
	for _, key := range keys {
		raw, ok := metadata[key]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			continue
		}
		var value string
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("media metadata %s must be a string", key)
		}
		return &value, nil
	}
	return nil, nil
}

func setYucoreMediaMetadataValue(metadata map[string]json.RawMessage, key string, value any) error {
	raw, err := common.Marshal(value)
	if err != nil {
		return err
	}
	metadata[key] = raw
	return nil
}

func buildYucoreMediaTaskFromRequest(req yucoreMediaTaskRequest, userId int) (*model.YucoreMediaTask, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if utf8.RuneCountInString(prompt) > maxYucoreMediaPromptLength {
		return nil, errors.New("prompt is too long")
	}
	sessionId := strings.TrimSpace(req.SessionId)
	if utf8.RuneCountInString(sessionId) > maxYucoreMediaSessionIdRunes {
		return nil, errors.New("session id is too long")
	}
	inputs, err := normalizeYucoreMediaRawJSON(req.Inputs, "[]", maxYucoreMediaInputsBytes, "inputs")
	if err != nil {
		return nil, err
	}
	references, err := decodeYucoreMediaReferences(inputs)
	if err != nil {
		return nil, err
	}
	canonicalInputs, err := common.Marshal(references)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := normalizeYucoreMediaRawJSON(req.Metadata, "{}", maxYucoreMediaMetadataBytes, "metadata")
	if err != nil {
		return nil, err
	}
	metadata, err := decodeYucoreMediaMetadata(metadataJSON)
	if err != nil {
		return nil, err
	}

	duration := req.Duration
	if duration == nil {
		duration, err = yucoreMediaMetadataInt(metadata, "duration", "durationSeconds", "duration_seconds")
		if err != nil {
			return nil, err
		}
	}
	resolution := req.Resolution
	if resolution == nil && strings.TrimSpace(req.Size) != "" {
		value := req.Size
		resolution = &value
	}
	if resolution == nil {
		resolution, err = yucoreMediaMetadataString(metadata, "resolution", "size")
		if err != nil {
			return nil, err
		}
	}
	generateAudio := req.GenerateAudio
	if generateAudio == nil {
		generateAudio, err = yucoreMediaMetadataBool(metadata, "generate_audio", "generateAudio")
		if err != nil {
			return nil, err
		}
	}
	seed := req.Seed
	if seed == nil {
		seed, err = yucoreMediaMetadataInt64(metadata, "seed")
		if err != nil {
			return nil, err
		}
	}
	referenceMode := req.ReferenceMode
	if referenceMode == nil {
		referenceMode, err = yucoreMediaMetadataString(metadata, "reference_mode", "referenceMode")
		if err != nil {
			return nil, err
		}
	}
	negativePrompt := req.NegativePrompt
	if negativePrompt == nil {
		negativePrompt, err = yucoreMediaMetadataString(metadata, "negative_prompt", "negativePrompt")
		if err != nil {
			return nil, err
		}
	}
	if duration != nil {
		if err := setYucoreMediaMetadataValue(metadata, "duration", duration); err != nil {
			return nil, err
		}
	}
	if resolution != nil {
		if err := setYucoreMediaMetadataValue(metadata, "resolution", resolution); err != nil {
			return nil, err
		}
	}
	if generateAudio != nil {
		if err := setYucoreMediaMetadataValue(metadata, "generate_audio", generateAudio); err != nil {
			return nil, err
		}
	}
	if seed != nil {
		if err := setYucoreMediaMetadataValue(metadata, "seed", seed); err != nil {
			return nil, err
		}
	}
	if referenceMode != nil {
		if err := setYucoreMediaMetadataValue(metadata, "reference_mode", referenceMode); err != nil {
			return nil, err
		}
	}
	if negativePrompt != nil {
		if err := setYucoreMediaMetadataValue(metadata, "negative_prompt", negativePrompt); err != nil {
			return nil, err
		}
	}
	canonicalMetadata, err := common.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	negativePromptValue := ""
	if negativePrompt != nil {
		negativePromptValue = *negativePrompt
	}
	resolutionValue := req.Size
	if resolution != nil {
		resolutionValue = *resolution
	}
	return &model.YucoreMediaTask{
		UserId:         userId,
		SessionId:      sessionId,
		BillingGroup:   strings.TrimSpace(req.Group),
		Kind:           req.Kind,
		Mode:           req.Mode,
		ModelId:        req.ModelId,
		Prompt:         prompt,
		NegativePrompt: negativePromptValue,
		AspectRatio:    req.AspectRatio,
		Size:           resolutionValue,
		Quality:        req.Quality,
		Format:         req.Format,
		Count:          req.Count,
		Inputs:         string(canonicalInputs),
		Metadata:       string(canonicalMetadata),
	}, nil
}

func buildYucoreMediaTaskResponse(task *model.YucoreMediaTask) yucoreMediaTaskResponse {
	return yucoreMediaTaskResponse{
		Id:             task.Id,
		TaskId:         task.TaskId,
		UserId:         task.UserId,
		SessionId:      task.SessionId,
		Group:          task.BillingGroup,
		Kind:           task.Kind,
		Mode:           task.Mode,
		ModelId:        task.ModelId,
		Prompt:         task.Prompt,
		NegativePrompt: task.NegativePrompt,
		AspectRatio:    task.AspectRatio,
		Size:           task.Size,
		Quality:        task.Quality,
		Format:         task.Format,
		Count:          task.Count,
		Status:         task.Status,
		Progress:       task.Progress,
		Cost:           task.Cost,
		Assets:         model.YucoreMediaTaskAssets(task),
		Inputs:         rawYucoreMediaJSON(task.Inputs, "[]"),
		Metadata:       rawYucoreMediaJSON(task.Metadata, "{}"),
		Error:          task.Error,
		CreatedTime:    task.CreatedTime,
		UpdatedTime:    task.UpdatedTime,
	}
}

func buildYucoreMediaTaskResponses(tasks []*model.YucoreMediaTask) []yucoreMediaTaskResponse {
	responses := make([]yucoreMediaTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, buildYucoreMediaTaskResponse(task))
	}
	return responses
}

func normalizeYucoreMediaTaskWithSelection(task *model.YucoreMediaTask, selectedModel service.YucoreMediaCatalogModel) error {
	if limit := selectedModel.InputLimits.MaxPromptChars; limit > 0 && utf8.RuneCountInString(task.Prompt) > limit {
		return fmt.Errorf("prompt is too long for model %s (maximum %d characters)", selectedModel.Id, limit)
	}
	var references []model.YucoreMediaReferenceInput
	if err := common.Unmarshal([]byte(task.Inputs), &references); err != nil {
		return errors.New("media inputs must be a JSON array")
	}
	metadata, err := decodeYucoreMediaMetadata(task.Metadata)
	if err != nil {
		return err
	}
	duration, err := yucoreMediaMetadataInt(metadata, "duration")
	if err != nil {
		return err
	}
	resolution, err := yucoreMediaMetadataString(metadata, "resolution")
	if err != nil {
		return err
	}
	generateAudio, err := yucoreMediaMetadataBool(metadata, "generate_audio")
	if err != nil {
		return err
	}
	seed, err := yucoreMediaMetadataInt64(metadata, "seed")
	if err != nil {
		return err
	}
	referenceMode, err := yucoreMediaMetadataString(metadata, "reference_mode")
	if err != nil {
		return err
	}
	negativePrompt, err := yucoreMediaMetadataString(metadata, "negative_prompt")
	if err != nil {
		return err
	}
	resolutionValue := ""
	if resolution != nil {
		resolutionValue = *resolution
	}
	referenceModeValue := ""
	if referenceMode != nil {
		referenceModeValue = *referenceMode
	}

	normalized, err := service.NormalizeYucoreMediaRequest(selectedModel, service.YucoreMediaRequestOptions{
		Mode:           task.Mode,
		Count:          task.Count,
		Duration:       duration,
		Resolution:     resolutionValue,
		AspectRatio:    task.AspectRatio,
		GenerateAudio:  generateAudio,
		Seed:           seed,
		NegativePrompt: negativePrompt,
		ReferenceMode:  referenceModeValue,
		References:     references,
	})
	if err != nil {
		return err
	}

	canonicalInputs, err := common.Marshal(normalized.References)
	if err != nil {
		return err
	}
	for _, alias := range []string{"durationSeconds", "duration_seconds", "size", "generateAudio", "referenceMode", "negativePrompt"} {
		delete(metadata, alias)
	}
	for _, key := range []string{"duration", "resolution", "generate_audio", "seed", "reference_mode", "negative_prompt"} {
		delete(metadata, key)
	}
	if normalized.Duration != nil {
		if err := setYucoreMediaMetadataValue(metadata, "duration", normalized.Duration); err != nil {
			return err
		}
	}
	if normalized.Resolution != "" {
		if err := setYucoreMediaMetadataValue(metadata, "resolution", normalized.Resolution); err != nil {
			return err
		}
	}
	if normalized.GenerateAudio != nil {
		if err := setYucoreMediaMetadataValue(metadata, "generate_audio", normalized.GenerateAudio); err != nil {
			return err
		}
	}
	if normalized.Seed != nil {
		if err := setYucoreMediaMetadataValue(metadata, "seed", normalized.Seed); err != nil {
			return err
		}
	}
	if err := setYucoreMediaMetadataValue(metadata, "reference_mode", normalized.ReferenceMode); err != nil {
		return err
	}
	if normalized.NegativePrompt != nil {
		if err := setYucoreMediaMetadataValue(metadata, "negative_prompt", normalized.NegativePrompt); err != nil {
			return err
		}
	}
	canonicalMetadata, err := common.Marshal(metadata)
	if err != nil {
		return err
	}

	task.Mode = normalized.Mode
	task.Count = normalized.Count
	task.Size = normalized.Resolution
	task.AspectRatio = normalized.AspectRatio
	task.NegativePrompt = ""
	if normalized.NegativePrompt != nil {
		task.NegativePrompt = *normalized.NegativePrompt
	}
	task.Inputs = string(canonicalInputs)
	task.Metadata = string(canonicalMetadata)
	return nil
}

func resolveYucoreMediaTaskRequest(task *model.YucoreMediaTask) error {
	if strings.EqualFold(strings.TrimSpace(task.Kind), service.YucoreMediaKindVideo) {
		task.Kind = service.YucoreMediaKindVideo
	} else {
		task.Kind = service.YucoreMediaKindImage
	}
	resolvedGroup, selectedModel, err := service.ResolveYucoreMediaSelection(
		task.UserId,
		task.BillingGroup,
		task.ModelId,
		task.Kind,
	)
	if err != nil {
		return err
	}
	if err := normalizeYucoreMediaTaskWithSelection(task, selectedModel); err != nil {
		return err
	}
	task.BillingGroup = resolvedGroup
	task.ModelId = selectedModel.Id
	return nil
}

func normalizeYucoreMediaUAGAuthorization(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, " ") {
		return value
	}
	return "Bearer " + value
}

func yucoreMediaUAGProxyHeadersFromRequest(c *gin.Context) model.YucoreMediaUAGProxyHeaders {
	headers := model.YucoreMediaUAGProxyHeaders{}
	if authorization := normalizeYucoreMediaUAGAuthorization(c.GetHeader("X-YuCore-UAG-Authorization")); authorization != "" {
		headers["Authorization"] = authorization
	} else if model.YucoreMediaUAGProxyAuthorizationHeader() == "" && common.GetEnvOrDefaultBool("YUCORE_MEDIA_FORWARD_BROWSER_AUTHORIZATION", true) {
		if authorization := normalizeYucoreMediaUAGAuthorization(c.GetHeader("Authorization")); authorization != "" {
			headers["Authorization"] = authorization
		}
	}
	if requestId := strings.TrimSpace(c.GetHeader("X-Request-Id")); requestId != "" {
		headers["X-Request-Id"] = requestId
	}
	if demoUser := strings.TrimSpace(c.GetHeader("X-Demo-User")); demoUser != "" {
		headers["X-Demo-User"] = demoUser
	}
	if canvasIdentity := strings.TrimSpace(c.GetHeader("X-YuCore-Canvas-Identity")); canvasIdentity != "" {
		headers["X-YuCore-Canvas-Identity"] = canvasIdentity
	}
	if canvasSession := strings.TrimSpace(c.GetHeader("X-YuCore-Canvas-Session")); canvasSession != "" {
		headers["X-YuCore-Canvas-Session"] = canvasSession
	}
	if idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key")); idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	if cookie := strings.TrimSpace(c.GetHeader("X-YuCore-UAG-Cookie")); cookie != "" {
		headers["Cookie"] = cookie
	} else if common.GetEnvOrDefaultBool("YUCORE_MEDIA_FORWARD_BROWSER_COOKIE", false) {
		if cookie := strings.TrimSpace(c.Request.Header.Get("Cookie")); cookie != "" {
			headers["Cookie"] = cookie
		}
	}
	return headers
}

func yucoreMediaUAGProxyHeadersWithConfiguredAuth(headers model.YucoreMediaUAGProxyHeaders) model.YucoreMediaUAGProxyHeaders {
	next := model.YucoreMediaUAGProxyHeaders{}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			next[key] = value
		}
	}
	if strings.TrimSpace(next["Authorization"]) == "" {
		if authorization := model.YucoreMediaUAGProxyAuthorizationHeader(); authorization != "" {
			next["Authorization"] = authorization
		}
	}
	return next
}

func ListYucoreMediaModels(c *gin.Context) {
	if model.IsYucoreMediaUAGProxyConfigured() {
		models, err := model.ListYucoreUAGProxyMediaModelsWithHeaders(yucoreMediaUAGProxyHeadersFromRequest(c))
		if err == nil && len(models) > 0 {
			common.ApiSuccess(c, models)
			return
		}
		if err != nil {
			common.SysError("YuCore UAG model list fallback: " + err.Error())
		}
	}
	catalog, err := service.BuildYucoreMediaCatalog(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, group := range catalog.Groups {
		if group.Id == catalog.DefaultGroup {
			common.ApiSuccess(c, group.Models)
			return
		}
	}
	common.ApiSuccess(c, []service.YucoreMediaCatalogModel{})
	return

	models := []gin.H{
		{
			"id":             "gpt-image-2",
			"name":           "GPT Image 2",
			"family":         "gpt",
			"badge":          "OpenAI 官方生图",
			"description":    "适合产品图、海报、品牌视觉和高保真图生图工作流。支持灵活像素尺寸和可选流式预览。",
			"kind":           "image",
			"modes":          []string{"text-to-image", "image-to-image"},
			"sizes":          []string{"auto", "1024x1024", "1536x1024", "1024x1536", "2048x2048", "2048x1152", "3840x2160", "2160x3840"},
			"size_label":     "尺寸",
			"aspect_ratios":  []string{"auto", "1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"},
			"qualities":      []string{"auto", "low", "medium", "high"},
			"output_formats": []string{"png", "jpeg", "webp"},
			"formats":        []string{"png", "jpeg", "webp"},
			"backgrounds":    []string{"auto", "opaque"},
			"moderations":    []string{"auto", "low"},
			"stream_modes":   []string{"final", "partial"},
			"partial_images": []int{0, 1, 2, 3},
			"style_presets":  []string{"auto", "commercial", "product", "editorial", "realistic"},
			"counts":         []int{1, 2, 4},
			"input_limits": gin.H{
				"max_prompt_chars":     4000,
				"max_reference_images": 16,
				"max_file_size_mb":     50,
			},
			"pricing": gin.H{
				"unit": "per_asset",
			},
		},
		{
			"id":             "gpt-image-2-adobe",
			"name":           "Image2 Arbitrary Ratio",
			"family":         "image2",
			"badge":          "Image2 任意比例",
			"description":    "已验证的 Image2 任意比例模型，适合产品图、海报和自定义宽高构图。1K、2K 与 4K 统一按张计费。",
			"kind":           "image",
			"modes":          []string{"text-to-image", "image-to-image"},
			"sizes":          []string{"1024x1024", "1024x768", "768x1024", "1536x1024", "1024x1536", "2048x2048", "2048x1152", "3840x2160", "2160x3840"},
			"size_label":     "尺寸",
			"aspect_ratios":  []string{"auto"},
			"qualities":      []string{"high"},
			"output_formats": []string{"url", "b64_json"},
			"formats":        []string{"url", "b64_json"},
			"stream_modes":   []string{"final"},
			"style_presets":  []string{"auto", "commercial", "product", "editorial", "realistic"},
			"counts":         []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     4000,
				"max_reference_images": 16,
			},
			"pricing": gin.H{
				"unit":     "per_asset",
				"amount":   0.10,
				"currency": "CNY",
				"display":  "¥0.10/张",
			},
		},
		{
			"id":             "grok-imagine-image",
			"name":           "Grok Imagine Image",
			"family":         "grok",
			"badge":          "xAI 标准生图",
			"description":    "已验证的 Grok 标准图片模型，适合快速概念图、社媒素材和批量创意探索。",
			"kind":           "image",
			"modes":          []string{"text-to-image", "image-to-image"},
			"sizes":          []string{"1k", "2k"},
			"size_label":     "分辨率",
			"aspect_ratios":  []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2"},
			"output_formats": []string{"url", "b64_json"},
			"formats":        []string{"url", "b64_json"},
			"stream_modes":   []string{"final"},
			"style_presets":  []string{"auto", "cinematic", "anime", "realistic"},
			"counts":         []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     3000,
				"max_reference_images": 3,
			},
			"pricing": gin.H{
				"unit":     "per_asset",
				"amount":   0.025,
				"currency": "CNY",
				"display":  "¥0.025/次",
			},
		},
		{
			"id":             "grok-imagine-image-quality",
			"name":           "Grok Imagine Image Quality",
			"family":         "grok",
			"badge":          "xAI 官方生图",
			"description":    "适合创意概念图、社媒素材和快速批量出图。走 OpenAI-compatible images endpoint，支持比例和分辨率参数。",
			"kind":           "image",
			"modes":          []string{"text-to-image", "image-to-image"},
			"sizes":          []string{"1k", "2k"},
			"size_label":     "分辨率",
			"aspect_ratios":  []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20"},
			"output_formats": []string{"url", "b64_json"},
			"formats":        []string{"url", "b64_json"},
			"stream_modes":   []string{"final"},
			"style_presets":  []string{"auto", "cinematic", "anime", "realistic"},
			"counts":         []int{1, 2, 4, 8, 10},
			"input_limits": gin.H{
				"max_prompt_chars":     3000,
				"max_reference_images": 3,
			},
			"pricing": gin.H{
				"unit":     "per_asset",
				"amount":   0.032,
				"currency": "CNY",
				"display":  "¥0.032/次",
			},
		},
		{
			"id":             "gemini-3.1-flash-image",
			"name":           "Gemini 3.1 Flash Image",
			"family":         "gemini",
			"badge":          "Gemini 交互式生图",
			"description":    "适合多轮编辑、参考图融合和需要 Gemini reasoning 的生图。通过 response_format 控制比例和清晰度。",
			"kind":           "image",
			"modes":          []string{"text-to-image", "image-to-image"},
			"sizes":          []string{"0.5K", "1K", "2K", "4K"},
			"size_label":     "image_size",
			"aspect_ratios":  []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "4:5", "5:4", "21:9", "1:4", "4:1", "1:8", "8:1"},
			"output_formats": []string{"image/png", "image/jpeg"},
			"formats":        []string{"image/png", "image/jpeg"},
			"stream_modes":   []string{"final"},
			"style_presets":  []string{"auto", "commercial", "editorial", "realistic"},
			"counts":         []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     4000,
				"max_reference_images": 14,
			},
			"pricing": gin.H{
				"unit": "per_asset",
			},
		},
		{
			"id":             "imagen-4.0-generate-001",
			"name":           "Imagen 4",
			"family":         "gemini",
			"badge":          "Gemini Imagen",
			"description":    "适合稳定文生图。通过 Imagen generateImages 配置 numberOfImages、imageSize 和 aspectRatio。",
			"kind":           "image",
			"modes":          []string{"text-to-image"},
			"sizes":          []string{"1K", "2K"},
			"size_label":     "imageSize",
			"aspect_ratios":  []string{"1:1", "3:4", "4:3", "9:16", "16:9"},
			"output_formats": []string{"image/png"},
			"formats":        []string{"image/png"},
			"stream_modes":   []string{"final"},
			"style_presets":  []string{"auto", "commercial", "product", "realistic"},
			"counts":         []int{1, 2, 4},
			"input_limits": gin.H{
				"max_prompt_chars":     2000,
				"max_reference_images": 0,
			},
			"pricing": gin.H{
				"unit": "per_asset",
			},
		},
		{
			"id":            "veo-3.1-generate-preview",
			"name":          "Veo 3.1",
			"family":        "gemini",
			"badge":         "Gemini 视频任务",
			"description":   "适合带音频的短视频生成。通过长任务创建后轮询 operation，支持文生视频和图生视频。",
			"kind":          "video",
			"modes":         []string{"text-to-video", "image-to-video"},
			"sizes":         []string{"720p", "1080p", "4k"},
			"size_label":    "resolution",
			"aspect_ratios": []string{"16:9", "9:16"},
			"durations":     []int{4, 6, 8},
			"stream_modes":  []string{"poll"},
			"style_presets": []string{"auto", "cinematic", "commercial", "realistic"},
			"counts":        []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     2000,
				"max_reference_images": 3,
			},
			"pricing": gin.H{
				"unit": "per_base_seconds",
			},
		},
		{
			"id":            "grok-imagine-video-1.5-preview",
			"name":          "Grok Imagine Video 1.5 Preview",
			"family":        "grok",
			"badge":         "xAI 图生视频",
			"description":   "Grok 图生视频模型，使用一张真实首帧图片创建最长 15 秒的视频任务，按实际生成秒数计费。",
			"kind":          "video",
			"modes":         []string{"image-to-video"},
			"sizes":         []string{"1280x720", "720x1280"},
			"size_label":    "resolution",
			"aspect_ratios": []string{"16:9", "9:16", "1:1"},
			"durations":     []int{4, 5, 6, 8, 10, 12, 15},
			"stream_modes":  []string{"poll"},
			"style_presets": []string{"auto", "cinematic", "commercial", "realistic"},
			"counts":        []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     1800,
				"max_reference_images": 1,
			},
			"pricing": gin.H{
				"unit":     "per_second",
				"amount":   0.65,
				"currency": "CNY",
				"display":  "¥0.65/秒",
			},
		},
		{
			"id":            "grok-imagine-video",
			"name":          "Grok Imagine Video",
			"family":        "grok",
			"badge":         "xAI 视频任务",
			"description":   "适合文生视频和图片动态化。REST 先返回 request_id，再轮询视频状态。",
			"kind":          "video",
			"modes":         []string{"text-to-video", "image-to-video", "reference-to-video"},
			"sizes":         []string{"480p", "720p", "1080p"},
			"size_label":    "resolution",
			"aspect_ratios": []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"},
			"durations":     []int{5, 8, 10, 12, 15},
			"stream_modes":  []string{"poll"},
			"style_presets": []string{"auto", "cinematic", "anime", "realistic"},
			"counts":        []int{1},
			"input_limits": gin.H{
				"max_prompt_chars":     1800,
				"max_reference_images": 7,
			},
			"pricing": gin.H{
				"unit": "per_second",
			},
		},
	}
	configured := model.YucoreMediaConfiguredModelIDs()
	if len(configured) > 0 {
		filtered := make([]gin.H, 0, len(models))
		for _, row := range models {
			modelId, _ := row["id"].(string)
			if _, ok := configured[strings.ToLower(strings.TrimSpace(modelId))]; ok {
				filtered = append(filtered, row)
			}
		}
		models = filtered
	}
	for _, row := range models {
		modelId, _ := row["id"].(string)
		unitPrice, ok := model.YucoreMediaModelUnitPrice(modelId)
		if !ok {
			continue
		}
		pricing, ok := row["pricing"].(gin.H)
		if !ok {
			continue
		}
		if model.YucoreMediaModelUsesPerCallPricing(modelId) {
			pricing["unit"] = "per_call"
		}
		unitLabel := "/次"
		switch pricing["unit"] {
		case "per_asset":
			unitLabel = "/张"
		case "per_second", "per_base_seconds":
			unitLabel = "/秒"
		}
		pricing["amount"] = unitPrice
		pricing["currency"] = "CNY"
		pricing["display"] = "¥" + strconv.FormatFloat(unitPrice, 'f', -1, 64) + unitLabel
	}
	common.ApiSuccess(c, models)
}

func GetYucoreMediaCatalog(c *gin.Context) {
	catalog, err := service.BuildYucoreMediaCatalog(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, catalog)
}

func ListYucoreMediaTemplates(c *gin.Context) {
	common.ApiSuccess(c, []gin.H{
		{
			"id":                "direct-flash-editorial",
			"preview_image_url": "/yucore/prompt-library/direct-flash-editorial.webp",
			"title":             "直闪胶片人像",
			"tag":               "人像",
			"kind":              "image",
			"model_id":          "gpt-image-2-adobe",
			"mode":              "text-to-image",
			"style":             "CCD / direct flash",
			"prompt":            "真实直闪胶片质感，室内生活快照，人物自然动作，硬边阴影，轻微颗粒和暗角，保留真实皮肤纹理。",
			"negative_prompt":   "塑料皮肤、畸形手指、多余肢体、过度 HDR、商业棚拍感",
			"aspect_ratio":      "3:4",
		},
		{
			"id":                "premium-product-core",
			"preview_image_url": "/yucore/prompt-library/premium-product-core.webp",
			"title":             "高端产品核心图",
			"tag":               "商业",
			"kind":              "image",
			"model_id":          "gpt-image-2-adobe",
			"mode":              "text-to-image",
			"style":             "product / cinematic",
			"prompt":            "高级黑色产品主视觉，精确边缘光，玻璃与金属细节，干净背景，商业级构图。",
			"negative_prompt":   "低清晰度、文字错误、杂乱背景、廉价塑料质感",
			"aspect_ratio":      "1:1",
		},
		{
			"id":                "social-campaign-poster",
			"preview_image_url": "/yucore/prompt-library/social-campaign-poster.webp",
			"title":             "社媒活动海报",
			"tag":               "营销",
			"kind":              "image",
			"model_id":          "grok-imagine-image-quality",
			"mode":              "text-to-image",
			"style":             "commercial",
			"prompt":            "生成一张适合社媒传播的活动海报，构图有冲击力，画面层次丰富，留出标题和按钮空间，整体高级且易于转化。",
			"negative_prompt":   "水印、品牌错字、低质量排版、过度拥挤",
			"aspect_ratio":      "4:5",
		},
		{
			"id":                "reference-polish",
			"preview_image_url": "/yucore/prompt-library/reference-polish.webp",
			"title":             "参考图精修",
			"tag":               "图生图",
			"kind":              "image",
			"model_id":          "gpt-image-2-adobe",
			"mode":              "image-to-image",
			"style":             "commercial",
			"prompt":            "基于参考图做商业级精修，保留主体结构和核心识别特征，提升光影、材质、清晰度和整体质感，适合对外发布。",
			"negative_prompt":   "主体变化过大、脸部变形、多余肢体、文字错误",
			"aspect_ratio":      "auto",
		},
		{
			"id":                "short-product-video",
			"preview_image_url": "/yucore/prompt-library/short-product-video.webp",
			"title":             "产品短视频",
			"tag":               "视频",
			"kind":              "video",
			"model_id":          "grok-imagine-video-1.5-preview",
			"mode":              "image-to-video",
			"style":             "slow cinematic push",
			"prompt":            "生成一段产品展示短视频，镜头缓慢推进，光影自然，主体稳定，节奏适合广告素材，画面干净且有高级感。",
			"negative_prompt":   "闪烁、断帧、主体漂移、低清晰度、文字错误",
			"aspect_ratio":      "16:9",
			"duration":          10,
		},
		{
			"id":                "image-to-video-motion",
			"preview_image_url": "/yucore/prompt-library/image-to-video-motion.webp",
			"title":             "图片动态化",
			"tag":               "图生视频",
			"kind":              "video",
			"model_id":          "grok-imagine-video-1.5-preview",
			"mode":              "image-to-video",
			"style":             "cinematic",
			"prompt":            "把参考图延展为自然短视频，保持主体一致，添加轻微镜头运动和真实环境氛围，避免夸张变形。",
			"negative_prompt":   "主体变形、跳切、闪烁、画面撕裂、低质量运动",
			"aspect_ratio":      "16:9",
			"duration":          10,
		},
	})
}

func GetYucoreMediaBilling(c *gin.Context) {
	userId := c.GetInt("id")
	remainQuota, err := model.GetUserQuota(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usedQuota, err := model.GetUserUsedQuota(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"active_mode":      "native_wallet",
		"available_points": remainQuota,
		"used_points":      usedQuota,
		"estimated_unit":   "points",
		"settlement":       "YuCore Studio 已与站内钱包和模型渠道统一结算，无需使用外部生图站额度同步码。",
	})
}

func GetYucoreMediaHealth(c *gin.Context) {
	common.ApiSuccess(c, model.GetYucoreMediaAdapterInfo())
}

func yucoreMediaUploadRoot() string {
	if configured := strings.TrimSpace(common.GetEnvOrDefaultString("YUCORE_MEDIA_UPLOAD_DIR", "")); configured != "" {
		return configured
	}
	return filepath.Join("data", "yucore-media", "uploads")
}

func yucoreMediaUploadPolicyFor(prefix []byte, declaredMimeType string) (yucoreMediaUploadPolicy, error) {
	var policy yucoreMediaUploadPolicy
	switch {
	case bytes.HasPrefix(prefix, []byte("\x89PNG\r\n\x1a\n")):
		policy = yucoreMediaUploadPolicy{Kind: "image", MIMEType: "image/png", Extension: ".png", MaxBytes: maxYucoreMediaImageUploadBytes}
	case len(prefix) >= 3 && prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff:
		policy = yucoreMediaUploadPolicy{Kind: "image", MIMEType: "image/jpeg", Extension: ".jpg", MaxBytes: maxYucoreMediaImageUploadBytes}
	case len(prefix) >= 12 && bytes.Equal(prefix[:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WEBP")):
		policy = yucoreMediaUploadPolicy{Kind: "image", MIMEType: "image/webp", Extension: ".webp", MaxBytes: maxYucoreMediaImageUploadBytes}
	case bytes.HasPrefix(prefix, []byte("GIF87a")) || bytes.HasPrefix(prefix, []byte("GIF89a")):
		policy = yucoreMediaUploadPolicy{Kind: "image", MIMEType: "image/gif", Extension: ".gif", MaxBytes: maxYucoreMediaImageUploadBytes}
	case len(prefix) >= 8 && bytes.Equal(prefix[4:8], []byte("ftyp")):
		policy, _ = yucoreMediaBMFFUploadPolicy(prefix)
	case bytes.HasPrefix(prefix, []byte("ID3")) || yucoreMediaHasMPEGFrameSync(prefix):
		if yucoreMediaHasMP3Structure(prefix) {
			policy = yucoreMediaUploadPolicy{Kind: "audio", MIMEType: "audio/mpeg", Extension: ".mp3", MaxBytes: maxYucoreMediaAudioUploadBytes}
		}
	case len(prefix) >= 12 && bytes.Equal(prefix[:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WAVE")):
		policy = yucoreMediaUploadPolicy{Kind: "audio", MIMEType: "audio/wav", Extension: ".wav", MaxBytes: maxYucoreMediaAudioUploadBytes}
	}
	if policy.MIMEType == "" {
		return yucoreMediaUploadPolicy{}, errors.New("reference upload uses an unsupported media format")
	}

	declaredMimeType = strings.ToLower(strings.TrimSpace(strings.Split(declaredMimeType, ";")[0]))
	switch declaredMimeType {
	case "", "application/octet-stream":
		return policy, nil
	case "image/jpg":
		declaredMimeType = "image/jpeg"
	case "audio/x-wav":
		declaredMimeType = "audio/wav"
	}
	if declaredMimeType != policy.MIMEType {
		return yucoreMediaUploadPolicy{}, errors.New("reference upload content does not match its declared media type")
	}
	return policy, nil
}

func yucoreMediaHasMPEGFrameSync(prefix []byte) bool {
	return len(prefix) >= 2 && prefix[0] == 0xff && prefix[1]&0xe0 == 0xe0
}

func yucoreMediaBMFFUploadPolicy(prefix []byte) (yucoreMediaUploadPolicy, bool) {
	if len(prefix) < 16 || !bytes.Equal(prefix[4:8], []byte("ftyp")) {
		return yucoreMediaUploadPolicy{}, false
	}
	boxSize := int(binary.BigEndian.Uint32(prefix[:4]))
	if boxSize < 16 || boxSize > yucoreMediaUploadFTYPMaxBytes || boxSize > len(prefix) || (boxSize-16)%4 != 0 {
		return yucoreMediaUploadPolicy{}, false
	}

	mimeType := yucoreMediaBMFFBrandMIMEType(string(prefix[8:12]))
	for offset := 16; offset < boxSize; offset += 4 {
		compatibleMimeType := yucoreMediaBMFFBrandMIMEType(string(prefix[offset : offset+4]))
		if compatibleMimeType == "" {
			continue
		}
		if mimeType != "" && mimeType != compatibleMimeType {
			return yucoreMediaUploadPolicy{}, false
		}
		mimeType = compatibleMimeType
	}
	switch mimeType {
	case "video/mp4":
		return yucoreMediaUploadPolicy{Kind: "video", MIMEType: mimeType, Extension: ".mp4", MaxBytes: maxYucoreMediaVideoUploadBytes}, true
	case "video/quicktime":
		return yucoreMediaUploadPolicy{Kind: "video", MIMEType: mimeType, Extension: ".mov", MaxBytes: maxYucoreMediaVideoUploadBytes}, true
	default:
		return yucoreMediaUploadPolicy{}, false
	}
}

func yucoreMediaBMFFBrandMIMEType(brand string) string {
	if brand == "qt  " {
		return "video/quicktime"
	}
	switch brand {
	case "avc1", "iso2", "iso3", "iso4", "iso5", "iso6", "isom", "mp41", "mp42", "M4V ", "MSNV", "dash", "cmfc", "cmfs":
		return "video/mp4"
	default:
		return ""
	}
}

func yucoreMediaHasMP3Structure(prefix []byte) bool {
	audioOffset := 0
	if bytes.HasPrefix(prefix, []byte("ID3")) {
		if len(prefix) < 10 {
			return false
		}
		version := prefix[3]
		if version < 2 || version > 4 || prefix[4] == 0xff {
			return false
		}
		allowedFlags := byte(0xc0)
		if version == 3 {
			allowedFlags = 0xe0
		} else if version == 4 {
			allowedFlags = 0xf0
		}
		if prefix[5]&^allowedFlags != 0 {
			return false
		}
		for _, sizeByte := range prefix[6:10] {
			if sizeByte&0x80 != 0 {
				return false
			}
		}
		tagSize := int(prefix[6])<<21 | int(prefix[7])<<14 | int(prefix[8])<<7 | int(prefix[9])
		audioOffset = 10 + tagSize
		if version == 4 && prefix[5]&0x10 != 0 {
			footerEnd := audioOffset + 10
			if footerEnd > len(prefix) || !bytes.Equal(prefix[audioOffset:audioOffset+3], []byte("3DI")) ||
				!bytes.Equal(prefix[3:10], prefix[audioOffset+3:footerEnd]) {
				return false
			}
			audioOffset = footerEnd
		}
		if audioOffset > len(prefix) {
			return false
		}
	}

	firstFrame, ok := yucoreMediaParseMPEGFrame(prefix[audioOffset:])
	if !ok || audioOffset+firstFrame.Length >= len(prefix) {
		return false
	}
	// A lone header or truncated tiny file is not enough to classify arbitrary bytes as MP3.
	secondFrame, ok := yucoreMediaParseMPEGFrame(prefix[audioOffset+firstFrame.Length:])
	return ok && firstFrame.VersionBits == secondFrame.VersionBits && firstFrame.LayerBits == secondFrame.LayerBits && firstFrame.SampleRate == secondFrame.SampleRate
}

func yucoreMediaParseMPEGFrame(data []byte) (yucoreMediaMPEGFrame, bool) {
	if !yucoreMediaHasMPEGFrameSync(data) || len(data) < 4 {
		return yucoreMediaMPEGFrame{}, false
	}
	versionBits := (data[1] >> 3) & 0x03
	layerBits := (data[1] >> 1) & 0x03
	bitrateIndex := (data[2] >> 4) & 0x0f
	sampleRateIndex := (data[2] >> 2) & 0x03
	if versionBits == 1 || layerBits != 1 || bitrateIndex == 0 || bitrateIndex == 15 || sampleRateIndex == 3 || data[3]&0x03 == 2 {
		return yucoreMediaMPEGFrame{}, false
	}

	mpeg1Bitrates := [...]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	mpeg2Bitrates := [...]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
	sampleRates := [...]int{44100, 48000, 32000}
	bitrate := mpeg2Bitrates[bitrateIndex]
	sampleRate := sampleRates[sampleRateIndex]
	coefficient := 72
	if versionBits == 3 {
		bitrate = mpeg1Bitrates[bitrateIndex]
		coefficient = 144
	} else if versionBits == 2 {
		sampleRate /= 2
	} else {
		sampleRate /= 4
	}
	frameLength := coefficient*bitrate*1000/sampleRate + int((data[2]>>1)&0x01)
	if frameLength < 4 {
		return yucoreMediaMPEGFrame{}, false
	}
	return yucoreMediaMPEGFrame{Length: frameLength, VersionBits: versionBits, LayerBits: layerBits, SampleRate: sampleRate}, true
}

func storeYucoreMediaUpload(reader io.Reader, finalPath string, maxBytes int64) (written int64, returnErr error) {
	return storeYucoreMediaUploadValidated(reader, finalPath, maxBytes, nil)
}

func storeYucoreMediaUploadValidated(reader io.Reader, finalPath string, maxBytes int64, validate func(written int64) error) (written int64, returnErr error) {
	ownerDir := filepath.Dir(finalPath)
	if err := os.MkdirAll(ownerDir, 0o700); err != nil {
		return 0, err
	}
	if err := os.Chmod(ownerDir, 0o700); err != nil {
		return 0, err
	}
	tempFile, err := os.CreateTemp(ownerDir, ".yucore-media-*.part")
	if err != nil {
		return 0, err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		if returnErr != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err := tempFile.Chmod(0o600); err != nil {
		return 0, err
	}

	written, err = io.Copy(tempFile, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, errYucoreMediaUploadTooLarge
	}
	if err := tempFile.Close(); err != nil {
		return written, err
	}
	if validate != nil {
		if err := validate(written); err != nil {
			return written, err
		}
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return written, err
	}
	return written, nil
}

func yucoreMediaUploadSignature(userId int, fileName string) string {
	return common.GenerateHMAC(fmt.Sprintf("yucore-media-upload:%d:%s", userId, fileName))
}

func yucoreMediaUploadURL(userId int, fileName string) string {
	return fmt.Sprintf("/api/yucore/media/uploads/%d/%s", userId, url.PathEscape(fileName))
}

func yucoreMediaRequestBaseURL(c *gin.Context) string {
	if configured := strings.TrimRight(strings.TrimSpace(common.GetEnvOrDefaultString("YUCORE_MEDIA_PUBLIC_BASE_URL", "")), "/"); configured != "" {
		return configured
	}
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = strings.TrimSpace(c.GetHeader("X-Forwarded-Scheme"))
	}
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func yucoreMediaSafeUploadPath(userId int, fileName string) (string, error) {
	cleanName := path.Base(strings.ReplaceAll(fileName, "\\", "/"))
	if cleanName == "." || cleanName == "/" || cleanName == "" || cleanName != fileName {
		return "", errors.New("invalid upload file name")
	}
	userDir := filepath.Join(yucoreMediaUploadRoot(), strconv.Itoa(userId))
	fullPath := filepath.Join(userDir, cleanName)
	absUserDir, err := filepath.Abs(userDir)
	if err != nil {
		return "", err
	}
	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absFile != absUserDir && !strings.HasPrefix(absFile, absUserDir+string(os.PathSeparator)) {
		return "", errors.New("invalid upload path")
	}
	return fullPath, nil
}

func UploadYucoreMediaReference(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, yucoreMediaUploadRequestMaxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if fileHeader.Size <= 0 {
		common.ApiErrorMsg(c, "reference upload is empty")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()

	prefix := make([]byte, yucoreMediaUploadSniffBytes)
	prefixLength, err := io.ReadFull(file, prefix)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		common.ApiError(c, err)
		return
	}
	prefix = prefix[:prefixLength]
	policy, err := yucoreMediaUploadPolicyFor(prefix, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if fileHeader.Size > policy.MaxBytes {
		common.ApiErrorMsg(c, fmt.Sprintf("reference %s must be %dMB or smaller", policy.Kind, policy.MaxBytes>>20))
		return
	}

	name := strings.TrimSpace(path.Base(strings.ReplaceAll(fileHeader.Filename, "\\", "/")))
	if name == "." || name == "/" || name == "" {
		name = "reference-image"
	}
	key, _ := common.GenerateRandomCharsKey(10)
	if key == "" {
		key = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	id := fmt.Sprintf("ref_%d_%s", common.GetTimestamp(), key)
	fileName := id + policy.Extension
	fullPath, err := yucoreMediaSafeUploadPath(c.GetInt("id"), fileName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	size, err := storeYucoreMediaUpload(io.MultiReader(bytes.NewReader(prefix), file), fullPath, policy.MaxBytes)
	if errors.Is(err, errYucoreMediaUploadTooLarge) {
		common.ApiErrorMsg(c, fmt.Sprintf("reference %s must be %dMB or smaller", policy.Kind, policy.MaxBytes>>20))
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cachedURL := yucoreMediaUploadURL(c.GetInt("id"), fileName)
	sourceURL := cachedURL
	if baseURL := yucoreMediaRequestBaseURL(c); baseURL != "" {
		sourceURL = baseURL + cachedURL
	}
	sourceURL += "?sig=" + url.QueryEscape(yucoreMediaUploadSignature(c.GetInt("id"), fileName))

	common.ApiSuccess(c, gin.H{
		"id":         id,
		"name":       name,
		"fileName":   name,
		"kind":       policy.Kind,
		"size":       size,
		"mime_type":  policy.MIMEType,
		"mimeType":   policy.MIMEType,
		"cached_url": cachedURL,
		"cachedUrl":  cachedURL,
		"source_url": sourceURL,
		"sourceUrl":  sourceURL,
		"url":        cachedURL,
		"createdAt":  time.Now().UTC().Format(time.RFC3339),
	})
}

func ServeYucoreMediaUploadedReference(c *gin.Context) {
	ownerId, err := strconv.Atoi(strings.TrimSpace(c.Param("user_id")))
	if err != nil || ownerId <= 0 {
		c.String(http.StatusBadRequest, "invalid upload owner")
		return
	}
	fileName := strings.TrimSpace(c.Param("file"))
	if fileName == "" {
		c.String(http.StatusBadRequest, "invalid upload file")
		return
	}
	signature := strings.TrimSpace(c.Query("sig"))
	expectedSignature := yucoreMediaUploadSignature(ownerId, fileName)
	authorized := c.GetInt("id") == ownerId
	if !authorized && signature != "" {
		authorized = subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) == 1
	}
	if !authorized {
		c.String(http.StatusUnauthorized, "unauthorized")
		return
	}
	fullPath, err := yucoreMediaSafeUploadPath(ownerId, fileName)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid upload file")
		return
	}
	if _, err := os.Stat(fullPath); err != nil {
		c.String(http.StatusNotFound, "upload not found")
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(fullPath)
}

func ListYucoreMediaTasks(c *gin.Context) {
	userId := c.GetInt("id")
	includeAdminSamples := c.GetInt("role") >= common.RoleAdminUser
	pageInfo := common.GetPageQuery(c)
	sessionId := strings.TrimSpace(c.Query("session_id"))
	kind := strings.TrimSpace(c.Query("kind"))
	status := strings.TrimSpace(c.Query("status"))
	if model.IsYucoreMediaUAGProxyConfigured() {
		tasks, total, err := model.ListYucoreMergedUAGProxyMediaTasks(userId, sessionId, kind, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), yucoreMediaUAGProxyHeadersFromRequest(c), includeAdminSamples)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(buildYucoreMediaTaskResponses(tasks))
		common.ApiSuccess(c, pageInfo)
		return
	}
	tasks, err := model.ListYucoreMediaTasks(userId, sessionId, kind, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), includeAdminSamples)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountYucoreMediaTasks(userId, sessionId, kind, status, includeAdminSamples)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildYucoreMediaTaskResponses(tasks))
	common.ApiSuccess(c, pageInfo)
}

func CreateYucoreMediaTask(c *gin.Context) {
	var req yucoreMediaTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := buildYucoreMediaTaskFromRequest(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := resolveYucoreMediaTaskRequest(task); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateYucoreMediaTaskWithHeaders(task, yucoreMediaUAGProxyHeadersFromRequest(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildYucoreMediaTaskResponse(task))
}

func GetYucoreMediaTask(c *gin.Context) {
	taskId := strings.TrimSpace(c.Param("task_id"))
	if taskId == "" {
		common.ApiErrorMsg(c, "invalid task id")
		return
	}
	task, err := model.GetYucoreMediaTaskByTaskIdWithHeaders(taskId, c.GetInt("id"), yucoreMediaUAGProxyHeadersFromRequest(c))
	if err != nil {
		writeYucoreMediaTaskNotFound(c)
		return
	}
	if denyYucoreMediaSampleAccess(c, task) {
		return
	}
	common.ApiSuccess(c, buildYucoreMediaTaskResponse(task))
}

func UpdateYucoreMediaTask(c *gin.Context) {
	taskId := strings.TrimSpace(c.Param("task_id"))
	if taskId == "" {
		common.ApiErrorMsg(c, "invalid task id")
		return
	}
	var req yucoreMediaTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := model.GetYucoreMediaTaskByTaskIdWithHeaders(taskId, c.GetInt("id"), yucoreMediaUAGProxyHeadersFromRequest(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if rejectYucoreMediaSampleMutation(c, task) {
		return
	}
	if strings.EqualFold(req.Action, "cancel") {
		if err := model.CancelYucoreMediaTaskWithHeaders(task, yucoreMediaUAGProxyHeadersFromRequest(c)); err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, buildYucoreMediaTaskResponse(task))
		return
	}
	common.ApiErrorMsg(c, "unsupported media task action")
}

func DeleteYucoreMediaTask(c *gin.Context) {
	taskId := strings.TrimSpace(c.Param("task_id"))
	if taskId == "" {
		common.ApiErrorMsg(c, "invalid task id")
		return
	}
	task, err := model.GetYucoreMediaTaskByTaskIdWithHeaders(taskId, c.GetInt("id"), yucoreMediaUAGProxyHeadersFromRequest(c))
	if err != nil {
		writeYucoreMediaTaskNotFound(c)
		return
	}
	if rejectYucoreMediaSampleMutation(c, task) {
		return
	}
	if err := model.DeleteYucoreMediaTaskByTaskId(taskId, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListYucoreMediaGallery(c *gin.Context) {
	userId := c.GetInt("id")
	includeAdminSamples := c.GetInt("role") >= common.RoleAdminUser
	pageInfo := common.GetPageQuery(c)
	kind := strings.TrimSpace(c.Query("kind"))
	if model.IsYucoreMediaUAGProxyConfigured() {
		tasks, total, err := model.ListYucoreMergedUAGProxyMediaTasks(userId, "", kind, model.YucoreMediaTaskStatusCompleted, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), yucoreMediaUAGProxyHeadersFromRequest(c), includeAdminSamples)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		pageInfo.SetTotal(int(total))
		pageInfo.SetItems(buildYucoreMediaTaskResponses(tasks))
		common.ApiSuccess(c, pageInfo)
		return
	}
	tasks, err := model.ListYucoreMediaTasks(userId, "", kind, model.YucoreMediaTaskStatusCompleted, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), includeAdminSamples)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountYucoreMediaTasks(userId, "", kind, model.YucoreMediaTaskStatusCompleted, includeAdminSamples)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildYucoreMediaTaskResponses(tasks))
	common.ApiSuccess(c, pageInfo)
}

func ServeYucoreMediaUpstreamAsset(c *gin.Context) {
	if c.GetInt("id") <= 0 {
		c.String(http.StatusUnauthorized, "unauthorized")
		return
	}
	source := strings.TrimSpace(c.Param("path"))
	if source == "" || source == "/" {
		c.String(http.StatusBadRequest, "invalid upstream asset path")
		return
	}
	if rawQuery := strings.TrimSpace(c.Request.URL.RawQuery); rawQuery != "" {
		source += "?" + rawQuery
	}
	resolvedSource, err := model.ResolveYucoreMediaAssetSourceURL(source)
	if err != nil {
		c.String(http.StatusBadGateway, "invalid upstream asset source")
		return
	}
	if strings.HasPrefix(resolvedSource, "data:") {
		serveYucoreMediaDataURL(c, resolvedSource, "")
		return
	}
	serveYucoreMediaRemoteAsset(c, resolvedSource, "", yucoreMediaUAGProxyHeadersWithConfiguredAuth(yucoreMediaUAGProxyHeadersFromRequest(c)), nil)
}

func ServeYucoreMediaTaskAsset(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.String(http.StatusUnauthorized, "unauthorized")
		return
	}
	taskId := strings.TrimSpace(c.Param("task_id"))
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		c.String(http.StatusBadRequest, "invalid asset index")
		return
	}
	upstreamHeaders := yucoreMediaUAGProxyHeadersFromRequest(c)
	task, err := model.GetYucoreMediaTaskByTaskIdWithHeaders(taskId, userId, upstreamHeaders)
	if err != nil {
		c.String(http.StatusNotFound, "asset not found")
		return
	}
	if denyYucoreMediaSampleAccess(c, task) {
		return
	}
	assets := model.YucoreMediaTaskAssets(task)
	if index >= len(assets) {
		c.String(http.StatusNotFound, "asset not found")
		return
	}
	asset := assets[index]
	if asset.ManagedFileName != "" {
		if !model.IsYucoreMediaSampleTask(task) {
			c.String(http.StatusNotFound, "asset not found")
			return
		}
		managedPath, err := yucoreMediaSafeUploadPath(task.UserId, asset.ManagedFileName)
		if err != nil {
			c.String(http.StatusNotFound, "asset not found")
			return
		}
		info, err := os.Lstat(managedPath)
		if err != nil || !info.Mode().IsRegular() {
			c.String(http.StatusNotFound, "asset not found")
			return
		}
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "private, max-age=86400")
		c.Header("X-Content-Type-Options", "nosniff")
		c.File(managedPath)
		return
	}
	if model.IsYucoreMediaMockTask(task) {
		if model.GetYucoreMediaAdapterInfo().RequireRealAssets {
			c.String(http.StatusBadGateway, "mock media assets are disabled")
			return
		}
		c.Header("Content-Type", "image/svg+xml; charset=utf-8")
		c.Header("Cache-Control", "no-store")
		c.String(http.StatusOK, buildYucoreMediaAssetSVG(task, index))
		return
	}
	source := model.YucoreMediaAssetSource(asset)
	isThumbnail := c.Query("variant") == model.YucoreMediaAssetVariantThumbnail
	if isThumbnail {
		source = model.YucoreMediaAssetThumbnailSource(asset)
	}
	if source == "" {
		c.String(http.StatusBadGateway, "provider media asset source is missing")
		return
	}
	resolvedSource, err := model.ResolveYucoreMediaAssetSourceURL(source)
	if err != nil {
		c.String(http.StatusBadGateway, "invalid provider media asset source")
		return
	}
	fallbackMimeType := asset.MimeType
	if isThumbnail {
		fallbackMimeType = ""
	}
	if strings.HasPrefix(resolvedSource, "data:") {
		serveYucoreMediaDataURL(c, resolvedSource, fallbackMimeType)
		return
	}
	assetHeaders := model.YucoreMediaUAGProxyHeaders{}
	var redirectHeaders func(string) (model.YucoreMediaUAGProxyHeaders, error)
	if model.IsYucoreMediaUAGProxyTask(task) {
		assetHeaders = yucoreMediaUAGProxyHeadersWithConfiguredAuth(upstreamHeaders)
	} else {
		assetHeaders, err = model.YucoreMediaAssetProxyHeaders(task, resolvedSource)
		if err != nil {
			c.String(http.StatusBadGateway, "provider media asset authorization unavailable")
			return
		}
		if len(assetHeaders) > 0 {
			redirectHeaders = func(target string) (model.YucoreMediaUAGProxyHeaders, error) {
				return model.YucoreMediaAssetProxyHeaders(task, target)
			}
		}
	}
	serveYucoreMediaRemoteAsset(c, resolvedSource, fallbackMimeType, assetHeaders, redirectHeaders)
}

func serveYucoreMediaDataURL(c *gin.Context, dataURL string, fallbackMimeType string) {
	header, encoded, ok := strings.Cut(dataURL, ",")
	if !ok {
		c.String(http.StatusBadGateway, "invalid provider media asset data URL")
		return
	}
	mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	if mimeType == "" {
		mimeType = fallbackMimeType
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	var data []byte
	var err error
	if strings.Contains(header, ";base64") {
		data, err = base64.StdEncoding.DecodeString(encoded)
	} else {
		var decoded string
		decoded, err = url.QueryUnescape(encoded)
		data = []byte(decoded)
	}
	if err != nil {
		c.String(http.StatusBadGateway, "invalid provider media asset data")
		return
	}
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, mimeType, data)
}

func serveYucoreMediaRemoteAsset(c *gin.Context, source string, fallbackMimeType string, upstreamHeaders model.YucoreMediaUAGProxyHeaders, redirectHeaders func(string) (model.YucoreMediaUAGProxyHeaders, error)) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, source, nil)
	if err != nil {
		c.String(http.StatusBadGateway, "invalid provider media asset source")
		return
	}
	req.Header.Set("Accept", "image/*,video/*,*/*;q=0.8")
	for key, value := range upstreamHeaders {
		value = strings.TrimSpace(value)
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	if redirectHeaders != nil {
		client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
			for key := range upstreamHeaders {
				req.Header.Del(key)
			}
			nextHeaders, err := redirectHeaders(req.URL.String())
			if err != nil {
				return err
			}
			for key, value := range nextHeaders {
				if value = strings.TrimSpace(value); value != "" {
					req.Header.Set(key, value)
				}
			}
			return nil
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "failed to fetch provider media asset")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.String(http.StatusBadGateway, "provider media asset returned %d", resp.StatusCode)
		return
	}
	const maxAssetBytes int64 = 256 << 20
	limitedBody := &io.LimitedReader{R: resp.Body, N: maxAssetBytes}
	var responseBody io.Reader = limitedBody
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		prefix := make([]byte, 512)
		prefixLength, readErr := io.ReadFull(limitedBody, prefix)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			c.String(http.StatusBadGateway, "failed to read provider media asset")
			return
		}
		prefix = prefix[:prefixLength]
		mimeType = http.DetectContentType(prefix)
		if mimeType == "application/octet-stream" && fallbackMimeType != "" {
			mimeType = fallbackMimeType
		}
		responseBody = io.MultiReader(bytes.NewReader(prefix), limitedBody)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	if length := strings.TrimSpace(resp.Header.Get("Content-Length")); length != "" {
		if parsedLength, parseErr := strconv.ParseInt(length, 10, 64); parseErr == nil && parsedLength >= 0 && parsedLength <= maxAssetBytes {
			c.Header("Content-Length", length)
		}
	}
	if contentRange := strings.TrimSpace(resp.Header.Get("Content-Range")); contentRange != "" {
		c.Header("Content-Range", contentRange)
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, responseBody)
}

func buildYucoreMediaAssetSVG(task *model.YucoreMediaTask, index int) string {
	title := html.EscapeString(task.ModelId)
	prompt := strings.TrimSpace(task.Prompt)
	if utf8.RuneCountInString(prompt) > 92 {
		runes := []rune(prompt)
		prompt = string(runes[:92]) + "..."
	}
	prompt = html.EscapeString(prompt)
	seed := 0
	for _, r := range task.TaskId {
		seed += int(r)
	}
	hue := (seed + index*38) % 360
	accent := fmt.Sprintf("hsl(%d 92%% 70%%)", hue)
	accentB := fmt.Sprintf("hsl(%d 88%% 64%%)", (hue+64)%360)
	label := "YuCore image"
	if task.Kind == "video" {
		label = "YuCore video preview"
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 900" width="1200" height="900" role="img" aria-label="%s">
  <defs>
    <radialGradient id="core" cx="48%%" cy="38%%" r="58%%">
      <stop offset="0%%" stop-color="%s" stop-opacity=".96"/>
      <stop offset="42%%" stop-color="%s" stop-opacity=".32"/>
      <stop offset="100%%" stop-color="#02040a"/>
    </radialGradient>
    <linearGradient id="grid" x1="0" x2="1" y1="0" y2="1">
      <stop offset="0%%" stop-color="%s" stop-opacity=".52"/>
      <stop offset="100%%" stop-color="#ffffff" stop-opacity=".08"/>
    </linearGradient>
    <filter id="grain">
      <feTurbulence type="fractalNoise" baseFrequency=".9" numOctaves="3" stitchTiles="stitch"/>
      <feColorMatrix type="saturate" values="0"/>
      <feComponentTransfer>
        <feFuncA type="table" tableValues="0 .16"/>
      </feComponentTransfer>
    </filter>
  </defs>
  <rect width="1200" height="900" fill="#02040a"/>
  <rect width="1200" height="900" fill="url(#core)"/>
  <path d="M-80 720 C 190 620 390 760 610 640 S 970 520 1280 650" fill="none" stroke="url(#grid)" stroke-width="2"/>
  <path d="M-60 790 C 220 700 405 810 650 705 S 1010 610 1260 730" fill="none" stroke="url(#grid)" stroke-width="1.4" opacity=".7"/>
  <g opacity=".23">
    <path d="M0 120H1200M0 220H1200M0 320H1200M0 420H1200M0 520H1200M0 620H1200M0 720H1200" stroke="#fff"/>
    <path d="M120 0V900M240 0V900M360 0V900M480 0V900M600 0V900M720 0V900M840 0V900M960 0V900M1080 0V900" stroke="#fff"/>
  </g>
  <circle cx="590" cy="365" r="164" fill="none" stroke="%s" stroke-opacity=".36" stroke-width="2"/>
  <circle cx="590" cy="365" r="102" fill="#05070c" fill-opacity=".54" stroke="#fff" stroke-opacity=".16"/>
  <text x="72" y="102" fill="#fff" font-family="Inter,Arial,sans-serif" font-size="34" font-weight="700">%s</text>
  <text x="72" y="150" fill="%s" font-family="Inter,Arial,sans-serif" font-size="19" letter-spacing="4">%s</text>
  <foreignObject x="72" y="700" width="930" height="120">
    <div xmlns="http://www.w3.org/1999/xhtml" style="font-family:Inter,Arial,sans-serif;color:white;font-size:30px;line-height:1.35;font-weight:650">%s</div>
  </foreignObject>
  <rect width="1200" height="900" filter="url(#grain)"/>
</svg>`, html.EscapeString(label), accent, accentB, accent, accent, title, accent, strings.ToUpper(task.Kind)+" / "+strings.ToUpper(task.Size), prompt)
}
