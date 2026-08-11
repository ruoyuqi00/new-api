package model

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

func yucoreMediaCapabilityForTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig) YucoreMediaModelCapability {
	capability, ok := config.ModelCapabilities[task.ModelId]
	if !ok {
		for modelID, candidate := range config.ModelCapabilities {
			if strings.EqualFold(strings.TrimSpace(modelID), task.ModelId) {
				capability = candidate
				break
			}
		}
	}

	capability.Transport = strings.ToLower(strings.TrimSpace(capability.Transport))
	switch capability.Transport {
	case "async-image-task", "async-video-task":
		capability.Transport = yucoreMediaTransportAsyncTask
	case yucoreMediaTransportSyncImage, yucoreMediaTransportAsyncTask:
	default:
		if task.Kind == "video" {
			capability.Transport = yucoreMediaTransportAsyncTask
		} else {
			capability.Transport = yucoreMediaTransportSyncImage
		}
	}

	if strings.TrimSpace(capability.CreatePath) == "" {
		if capability.Transport == yucoreMediaTransportAsyncTask {
			capability.CreatePath = "/v1/videos"
		} else {
			capability.CreatePath = "/v1/images/generations"
		}
	}
	if strings.TrimSpace(capability.EditPath) == "" {
		capability.EditPath = "/v1/images/edits"
	}
	if strings.TrimSpace(capability.StatusPath) == "" {
		capability.StatusPath = "/v1/videos/{task_id}"
	}

	capability.DurationPolicy = strings.ToLower(strings.TrimSpace(capability.DurationPolicy))
	switch capability.DurationPolicy {
	case yucoreMediaDurationPolicyDuration, yucoreMediaDurationPolicySeconds, yucoreMediaDurationPolicyFixed, yucoreMediaDurationPolicyNone:
	default:
		if task.Kind == "video" {
			capability.DurationPolicy = yucoreMediaDurationPolicySeconds
		} else {
			capability.DurationPolicy = yucoreMediaDurationPolicyNone
		}
	}
	if capability.PollIntervalSeconds <= 0 {
		capability.PollIntervalSeconds = 5
	}
	if capability.MaxReferenceImages <= 0 {
		if capability.ReferenceLimits.Images > 0 {
			capability.MaxReferenceImages = capability.ReferenceLimits.Images
		} else if task.Kind == "video" {
			capability.MaxReferenceImages = 8
		} else {
			capability.MaxReferenceImages = 16
		}
	}
	return capability
}

func yucoreMediaCapabilityModel(task *YucoreMediaTask, capability YucoreMediaModelCapability) string {
	if upstreamModel := strings.TrimSpace(capability.UpstreamModel); upstreamModel != "" {
		return upstreamModel
	}
	return task.ModelId
}

func yucoreMediaCapabilityAllowsParameter(capability YucoreMediaModelCapability, parameter string) bool {
	if len(capability.AllowedParameters) == 0 {
		return true
	}
	for _, allowed := range capability.AllowedParameters {
		if strings.EqualFold(strings.TrimSpace(allowed), parameter) {
			return true
		}
	}
	return false
}

func getOrCreateYucoreMediaManagedToken(userID int, group string) (*Token, error) {
	group = strings.TrimSpace(group)
	if userID <= 0 || group == "" {
		return nil, errors.New("invalid YuCore managed token identity")
	}
	var token Token
	now := common.GetTimestamp()
	err := DB.Where("user_id = ? AND name = ? AND "+commonGroupCol+" = ? AND status = ? AND (expired_time = -1 OR expired_time > ?)", userID, "yucore-studio-managed", group, common.TokenStatusEnabled, now).
		Order("id DESC").First(&token).Error
	if err == nil {
		return &token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	token = Token{
		UserId:         userID,
		Name:           "yucore-studio-managed",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          group,
	}
	if err := token.Insert(); err != nil {
		var existing Token
		lookupErr := DB.Where("user_id = ? AND name = ? AND "+commonGroupCol+" = ? AND status = ?", userID, "yucore-studio-managed", group, common.TokenStatusEnabled).
			Order("id DESC").First(&existing).Error
		if lookupErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &token, nil
}

func yucoreMediaOpenAIConfigForTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig) (yucoreMediaAdapterConfig, error) {
	if yucoreMediaTaskAdapter(task) != YucoreMediaAdapterYuAPIChannel {
		return config, nil
	}
	config.ManagedTokenGroup = yucoreMediaTaskBillingGroup(task, config)
	token, err := getOrCreateYucoreMediaManagedToken(task.UserId, config.ManagedTokenGroup)
	if err != nil {
		return config, err
	}
	config.APIKey = token.GetFullKey()
	return config, nil
}

func yucoreMediaRoutedTaskID(task *YucoreMediaTask, providerTaskID string) string {
	providerTaskID = strings.TrimSpace(providerTaskID)
	if providerTaskID == "" || yucoreMediaTaskAdapter(task) != YucoreMediaAdapterYuAPIChannel {
		return providerTaskID
	}

	var routedTasks []Task
	if err := DB.Where("user_id = ? AND created_at >= ?", task.UserId, common.GetTimestamp()-60).
		Order("id DESC").Limit(10).Find(&routedTasks).Error; err != nil {
		return providerTaskID
	}
	for _, routedTask := range routedTasks {
		if routedTask.PrivateData.UpstreamTaskID != providerTaskID {
			continue
		}
		originModel := strings.TrimSpace(routedTask.Properties.OriginModelName)
		upstreamModel := strings.TrimSpace(routedTask.Properties.UpstreamModelName)
		if originModel == task.ModelId || upstreamModel == task.ModelId {
			return routedTask.TaskID
		}
	}
	return providerTaskID
}

func yucoreMediaOpenAIURL(baseURL string, endpointPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid YuCore media base URL")
	}
	endpointPath = "/" + strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	basePath := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(endpointPath, "/v1/") {
		parsed.Path = basePath + strings.TrimPrefix(endpointPath, "/v1")
	} else {
		parsed.Path = basePath + endpointPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func yucoreMediaTaskDuration(task *YucoreMediaTask) int {
	metadata := yucoreMediaMetadataMap(task.Metadata)
	for _, key := range []string{"duration", "durationSeconds", "duration_seconds"} {
		if value := yucoreMediaIntValue(metadata[key]); value > 0 {
			return value
		}
	}
	return 0
}

func buildOpenAICompatibleAsyncPayload(task *YucoreMediaTask, capability YucoreMediaModelCapability) map[string]any {
	payload := map[string]any{
		"model":  yucoreMediaCapabilityModel(task, capability),
		"prompt": task.Prompt,
	}
	family := strings.ToLower(strings.TrimSpace(capability.Family))
	allowsParameter := func(parameter string) bool {
		for _, allowed := range capability.AllowedParameters {
			if strings.EqualFold(strings.TrimSpace(allowed), parameter) {
				return true
			}
		}
		return false
	}
	if task.NegativePrompt != "" && allowsParameter("negative_prompt") {
		payload["negative_prompt"] = task.NegativePrompt
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	resolution := yucoreMediaStringValue(metadata["resolution"])
	if resolution == "" {
		resolution = strings.TrimSpace(task.Size)
	}
	if resolution != "" && !strings.EqualFold(resolution, "auto") {
		if allowsParameter("resolution") {
			payload["resolution"] = resolution
		} else if allowsParameter("size") {
			payload["size"] = resolution
		}
	}
	if task.AspectRatio != "" && !strings.EqualFold(task.AspectRatio, "auto") && allowsParameter("aspect_ratio") {
		payload["aspect_ratio"] = task.AspectRatio
	} else if task.AspectRatio != "" && !strings.EqualFold(task.AspectRatio, "auto") && allowsParameter("ratio") {
		payload["ratio"] = task.AspectRatio
	}
	if capability.ResponseFormat != "" && allowsParameter("response_format") {
		payload["response_format"] = capability.ResponseFormat
	}

	images := make([]string, 0)
	videos := make([]string, 0)
	audios := make([]string, 0)
	firstFrames := make([]string, 0, 1)
	lastFrames := make([]string, 0, 1)
	var references []YucoreMediaReferenceInput
	if task.Inputs != "" {
		if err := common.Unmarshal([]byte(task.Inputs), &references); err != nil {
			references = nil
		}
	}
	for _, reference := range references {
		referenceURL := strings.TrimSpace(reference.URL)
		if referenceURL == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(reference.Role)) {
		case "", "image":
			images = append(images, referenceURL)
		case "video":
			videos = append(videos, referenceURL)
		case "audio":
			audios = append(audios, referenceURL)
		case "first_frame":
			firstFrames = append(firstFrames, referenceURL)
		case "last_frame":
			lastFrames = append(lastFrames, referenceURL)
		}
	}

	imageAllowed := allowsParameter("image") || allowsParameter("image_url") || allowsParameter("image_urls") || allowsParameter("images") || allowsParameter("reference_image_urls")
	videoAllowed := allowsParameter("video") || allowsParameter("video_url") || allowsParameter("reference_videos")
	audioAllowed := allowsParameter("audio") || allowsParameter("reference_audios")
	hasFrameReferences := len(firstFrames) > 0 || len(lastFrames) > 0
	switch family {
	case "omni":
		if hasFrameReferences {
			if imageAllowed {
				if len(firstFrames) > 0 {
					payload["first_image_url"] = firstFrames[0]
				}
				if len(lastFrames) > 0 {
					payload["last_image_url"] = lastFrames[0]
				}
			}
			break
		} else if len(videos) > 0 && videoAllowed {
			payload["video_url"] = videos[0]
		} else if len(images) == 1 && imageAllowed {
			payload["image_url"] = images[0]
		} else if len(images) > 1 && imageAllowed {
			payload["image_urls"] = images
		}
	case "grok", "happyhouse", "veo":
		if len(images) > 0 && imageAllowed {
			payload["image_urls"] = images
		}
	case "kling":
		if len(images) > 0 && imageAllowed {
			payload["image_urls"] = images
		}
		if len(videos) > 0 && videoAllowed {
			payload["reference_videos"] = videos
		}
		if len(audios) > 0 && audioAllowed {
			payload["reference_audios"] = audios
		}
	case "seedance":
		if hasFrameReferences {
			if imageAllowed {
				if len(firstFrames) > 0 {
					payload["first_image_url"] = firstFrames[0]
				}
				if len(lastFrames) > 0 {
					payload["last_image_url"] = lastFrames[0]
				}
			}
			break
		}
		if len(images) > 0 && imageAllowed {
			payload["image_url"] = images[0]
			if len(images) > 1 {
				payload["reference_image_urls"] = images[1:]
			}
		}
		if len(videos) > 0 && videoAllowed {
			payload["reference_videos"] = videos
		}
		if len(audios) > 0 && audioAllowed {
			payload["reference_audios"] = audios
		}
	default:
		if len(images) > 0 && len(capability.AllowedParameters) > 0 && allowsParameter("image_urls") {
			payload["image_urls"] = images
		} else if len(images) == 1 && len(capability.AllowedParameters) > 0 && allowsParameter("image_url") {
			payload["image_url"] = images[0]
		} else if len(images) == 1 && allowsParameter("image") {
			payload["image"] = images[0]
		} else if len(images) > 1 && allowsParameter("images") {
			payload["images"] = images
		}
		if len(videos) > 0 && allowsParameter("video_url") {
			payload["video_url"] = videos[0]
		} else if len(videos) > 0 && allowsParameter("reference_videos") {
			payload["reference_videos"] = videos
		}
		if len(audios) > 0 && allowsParameter("reference_audios") {
			payload["reference_audios"] = audios
		}
	}

	if generateAudio, ok := metadata["generate_audio"].(bool); ok {
		if allowsParameter("generate_audio") {
			payload["generate_audio"] = generateAudio
		} else if allowsParameter("audio") {
			payload["audio"] = generateAudio
		}
	}
	if rawSeed, ok := metadata["seed"]; ok && allowsParameter("seed") {
		seed, err := strconv.ParseInt(yucoreMediaStringValue(rawSeed), 10, 64)
		if err == nil {
			payload["seed"] = seed
		}
	}

	duration := yucoreMediaTaskDuration(task)
	if duration <= 0 && capability.DurationPolicy == yucoreMediaDurationPolicyFixed {
		duration = capability.FixedDurationSeconds
	}
	switch capability.DurationPolicy {
	case yucoreMediaDurationPolicyDuration:
		if duration > 0 && allowsParameter("duration") {
			payload["duration"] = duration
		}
	case yucoreMediaDurationPolicySeconds:
		if duration > 0 && allowsParameter("seconds") {
			payload["seconds"] = strconv.Itoa(duration)
		}
	}
	return payload
}

func requestOpenAICompatibleJSON(config yucoreMediaAdapterConfig, method string, endpointPath string, body any) (map[string]any, error) {
	endpoint, err := yucoreMediaOpenAIURL(config.BaseURL, endpointPath)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		rawBody, err := common.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("YuCore media upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	payload := map[string]any{}
	if len(responseBody) > 0 {
		if err := common.Unmarshal(responseBody, &payload); err != nil {
			return nil, err
		}
	}
	return payload, nil
}

func openAICompatibleTaskRows(payload map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, 3)
	if payload == nil {
		return rows
	}
	if data := yucoreMediaMapValue(payload["data"]); data != nil {
		if task := yucoreMediaMapValue(data["task"]); task != nil {
			rows = append(rows, task)
		}
		rows = append(rows, data)
	}
	return append(rows, payload)
}

func openAICompatibleTaskID(payload map[string]any) string {
	for _, row := range openAICompatibleTaskRows(payload) {
		if taskID := yucoreMediaFirstString(row, "task_id", "taskId", "id"); taskID != "" {
			return taskID
		}
	}
	return ""
}

func openAICompatibleTaskStatus(payload map[string]any) string {
	for _, row := range openAICompatibleTaskRows(payload) {
		if rawStatus := yucoreMediaFirstString(row, "status", "state"); rawStatus != "" {
			return yucoreMediaUAGStatus(rawStatus)
		}
	}
	return YucoreMediaTaskStatusProcessing
}

func openAICompatibleTaskProgress(payload map[string]any) int {
	for _, row := range openAICompatibleTaskRows(payload) {
		for _, key := range []string{"progress", "progress_percent", "progressPercent"} {
			if progress := yucoreMediaIntValue(row[key]); progress > 0 {
				return min(progress, 100)
			}
		}
	}
	return 0
}

func appendOpenAICompatibleResultURL(urls []string, seen map[string]struct{}, value any) []string {
	resultURL := yucoreMediaStringValue(value)
	if resultURL == "" {
		return urls
	}
	if _, ok := seen[resultURL]; ok {
		return urls
	}
	seen[resultURL] = struct{}{}
	return append(urls, resultURL)
}

func openAICompatibleResultURLs(payload map[string]any) []string {
	urls := make([]string, 0, 2)
	seen := map[string]struct{}{}
	rows := openAICompatibleTaskRows(payload)
	if dataRows := yucoreMediaSliceValue(payload["data"]); len(dataRows) > 0 {
		for _, item := range dataRows {
			if row := yucoreMediaMapValue(item); row != nil {
				rows = append(rows, row)
			}
		}
	}
	for _, row := range rows {
		for _, key := range []string{"video_url", "videoUrl", "image_url", "imageUrl", "url"} {
			urls = appendOpenAICompatibleResultURL(urls, seen, row[key])
		}
		for _, key := range []string{"output", "outputs", "results", "images", "videos"} {
			for _, item := range yucoreMediaSliceValue(row[key]) {
				if itemRow := yucoreMediaMapValue(item); itemRow != nil {
					for _, itemKey := range []string{"video_url", "videoUrl", "image_url", "imageUrl", "url"} {
						urls = appendOpenAICompatibleResultURL(urls, seen, itemRow[itemKey])
					}
				} else {
					urls = appendOpenAICompatibleResultURL(urls, seen, item)
				}
			}
		}
		if metadata := yucoreMediaMapValue(row["metadata"]); metadata != nil {
			for _, item := range yucoreMediaSliceValue(metadata["result_urls"]) {
				urls = appendOpenAICompatibleResultURL(urls, seen, item)
			}
		}
	}
	return urls
}

func openAICompatibleTaskError(payload map[string]any) string {
	for _, row := range openAICompatibleTaskRows(payload) {
		if errRow := yucoreMediaMapValue(row["error"]); errRow != nil {
			if message := yucoreMediaFirstString(errRow, "message", "detail", "code"); message != "" {
				return message
			}
		}
		if message := yucoreMediaFirstString(row, "error_message", "errorMessage", "message", "detail"); message != "" {
			return message
		}
	}
	return "YuCore media upstream task failed"
}

func buildOpenAICompatibleTaskAssets(task *YucoreMediaTask, payload map[string]any) []YucoreMediaAsset {
	resultURLs := openAICompatibleResultURLs(payload)
	assets := make([]YucoreMediaAsset, 0, len(resultURLs))
	for index, resultURL := range resultURLs {
		mimeType := "image/png"
		if task.Kind == "video" {
			mimeType = "video/mp4"
		} else {
			ext := strings.ToLower(path.Ext(strings.Split(resultURL, "?")[0]))
			if ext == ".jpg" || ext == ".jpeg" {
				mimeType = "image/jpeg"
			} else if ext == ".webp" {
				mimeType = "image/webp"
			}
		}
		assets = append(assets, YucoreMediaAsset{
			Id:        fmt.Sprintf("%s_asset_%d", task.TaskId, index),
			Kind:      task.Kind,
			Url:       fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
			ThumbUrl:  fmt.Sprintf("/api/yucore/media/tasks/%s/assets/%d", task.TaskId, index),
			SourceUrl: resultURL,
			Label:     fmt.Sprintf("%s result %d", task.ModelId, index+1),
			MimeType:  mimeType,
			Metadata: map[string]any{
				"adapter": YucoreMediaAdapterOpenAICompatible,
			},
		})
	}
	return assets
}

func applyOpenAICompatibleTaskPayload(task *YucoreMediaTask, payload map[string]any, capability YucoreMediaModelCapability) error {
	status := openAICompatibleTaskStatus(payload)
	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
		"last_status_at": common.GetTimestamp(),
		"transport":      capability.Transport,
		"upstream_model": yucoreMediaCapabilityModel(task, capability),
	})
	if upstreamTaskID := openAICompatibleTaskID(payload); upstreamTaskID != "" {
		metadata := yucoreMediaMetadataMap(task.Metadata)
		if yucoreMediaStringValue(metadata["upstream_task_id"]) == "" {
			task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{"upstream_task_id": upstreamTaskID})
		}
	}
	if status == YucoreMediaTaskStatusCompleted {
		assets := buildOpenAICompatibleTaskAssets(task, payload)
		if len(assets) == 0 {
			return errors.New("YuCore media upstream completed without result assets")
		}
		return settleYucoreMediaTaskWithAssets(task, assets)
	}
	if status == YucoreMediaTaskStatusFailed {
		return failYucoreMediaTask(task, errors.New(openAICompatibleTaskError(payload)))
	}
	if status == YucoreMediaTaskStatusCanceled {
		task.Status = status
		task.Progress = 0
		task.UpdatedTime = common.GetTimestamp()
		if err := DB.Model(task).Select("status", "progress", "metadata", "updated_time").Updates(task).Error; err != nil {
			return err
		}
		notifyYucoreMediaTaskTerminalBackflow(task)
		return nil
	}
	task.Status = status
	if progress := openAICompatibleTaskProgress(payload); progress > task.Progress {
		task.Progress = progress
	}
	task.UpdatedTime = common.GetTimestamp()
	return DB.Model(task).Select("status", "progress", "metadata", "updated_time").Updates(task).Error
}

func runOpenAICompatibleAsyncTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig, capability YucoreMediaModelCapability) error {
	payload, err := requestOpenAICompatibleJSON(config, http.MethodPost, capability.CreatePath, buildOpenAICompatibleAsyncPayload(task, capability))
	if err != nil {
		return err
	}
	providerTaskID := openAICompatibleTaskID(payload)
	if providerTaskID == "" {
		return errors.New("YuCore media upstream returned no task ID")
	}
	if routedTaskID := yucoreMediaRoutedTaskID(task, providerTaskID); routedTaskID != providerTaskID {
		task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{"provider_task_id": providerTaskID})
		payload["id"] = routedTaskID
		payload["task_id"] = routedTaskID
		if data := yucoreMediaMapValue(payload["data"]); data != nil {
			data["id"] = routedTaskID
			data["task_id"] = routedTaskID
		}
	}
	return applyOpenAICompatibleTaskPayload(task, payload, capability)
}

func refreshOpenAICompatibleYucoreTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig) error {
	capability := yucoreMediaCapabilityForTask(task, config)
	if capability.Transport != yucoreMediaTransportAsyncTask {
		return nil
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	upstreamTaskID := yucoreMediaStringValue(metadata["upstream_task_id"])
	if upstreamTaskID == "" {
		return nil
	}
	lastStatusAt := int64(yucoreMediaIntValue(metadata["last_status_at"]))
	if lastStatusAt > 0 && common.GetTimestamp()-lastStatusAt < int64(capability.PollIntervalSeconds) {
		return nil
	}
	statusPath := strings.ReplaceAll(capability.StatusPath, "{task_id}", url.PathEscape(upstreamTaskID))
	payload, err := requestOpenAICompatibleJSON(config, http.MethodGet, statusPath, nil)
	if err != nil {
		return err
	}
	return applyOpenAICompatibleTaskPayload(task, payload, capability)
}

func readOpenAICompatibleReference(source string, timeoutSeconds int) ([]byte, string, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "data:") {
		header, encoded, ok := strings.Cut(source, ",")
		if !ok || !strings.Contains(header, ";base64") || !strings.HasPrefix(header, "data:image/") {
			return nil, "", errors.New("reference image data URL is invalid")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, "", errors.New("reference image data URL is invalid")
		}
		if len(data) == 0 || len(data) > 25<<20 {
			return nil, "", errors.New("reference image must be between 1 byte and 25MB")
		}
		mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
		if detected := strings.Split(http.DetectContentType(data), ";")[0]; !strings.HasPrefix(detected, "image/") {
			return nil, "", errors.New("reference image data URL is invalid")
		}
		return data, mimeType, nil
	}

	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", errors.New("reference image must use a public HTTPS URL")
	}
	fetchSetting := system_setting.GetFetchSetting()
	validateURL := func(rawURL string) error {
		parsedURL, err := url.Parse(rawURL)
		if err != nil || parsedURL.Scheme != "https" {
			return errors.New("reference image redirect must use HTTPS")
		}
		return common.ValidateURLWithFetchSetting(rawURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
	}
	if err := validateURL(source); err != nil {
		return nil, "", fmt.Errorf("reference image URL rejected: %w", err)
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many reference image redirects")
			}
			return validateURL(req.URL.String())
		},
	}
	resp, err := client.Get(source)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("reference image returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (25<<20)+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > 25<<20 {
		return nil, "", errors.New("reference image must be between 1 byte and 25MB")
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0])
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, "", errors.New("reference URL did not return an image")
	}
	return data, mimeType, nil
}

func runOpenAICompatibleYucoreImageEditTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig, capability YucoreMediaModelCapability) error {
	refs := yucoreMediaReferenceAssets(task)
	if len(refs) == 0 {
		return errors.New("image-to-image requires at least one reference image")
	}
	if len(refs) > capability.MaxReferenceImages {
		return fmt.Errorf("image-to-image accepts at most %d reference images", capability.MaxReferenceImages)
	}
	endpoint, err := yucoreMediaOpenAIURL(config.BaseURL, capability.EditPath)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"model":  yucoreMediaCapabilityModel(task, capability),
		"prompt": task.Prompt,
	}
	if yucoreMediaCapabilityAllowsParameter(capability, "n") {
		fields["n"] = strconv.Itoa(task.Count)
	}
	if task.Size != "" && !strings.EqualFold(task.Size, "auto") && yucoreMediaCapabilityAllowsParameter(capability, "size") {
		fields["size"] = normalizeYucoreMediaImageSize(task.Size)
	}
	if task.Quality != "" && !strings.EqualFold(task.Quality, "auto") && yucoreMediaCapabilityAllowsParameter(capability, "quality") {
		fields["quality"] = task.Quality
	}
	if task.Format != "" && yucoreMediaCapabilityAllowsParameter(capability, "output_format") {
		fields["output_format"] = strings.TrimPrefix(task.Format, "image/")
	}
	if capability.ResponseFormat != "" && yucoreMediaCapabilityAllowsParameter(capability, "response_format") {
		fields["response_format"] = capability.ResponseFormat
	}
	if task.AspectRatio != "" && !strings.EqualFold(task.AspectRatio, "auto") && yucoreMediaCapabilityAllowsParameter(capability, "aspect_ratio") {
		fields["aspect_ratio"] = task.AspectRatio
	}
	for key, value := range fields {
		if value != "" {
			if err := writer.WriteField(key, value); err != nil {
				return err
			}
		}
	}
	for index, source := range refs {
		data, mimeType, err := readOpenAICompatibleReference(source, config.TimeoutSeconds)
		if err != nil {
			return fmt.Errorf("reference image %d: %w", index+1, err)
		}
		ext := strings.TrimPrefix(mimeType, "image/")
		if ext == "jpeg" {
			ext = "jpg"
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="reference-%d.%s"`, index+1, ext))
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return err
		}
		if _, err := part.Write(data); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("YuCore media upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var payload openAICompatibleImageResponse
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return err
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return errors.New(payload.Error.Message)
	}
	assets := make([]YucoreMediaAsset, 0, max(task.Count, 1))
	for index, item := range payload.Data {
		assets = appendOpenAICompatibleImageAsset(assets, task, index, item.URL, item.B64JSON, item.RevisedPrompt)
	}
	if len(assets) == 0 {
		for index, item := range payload.Images {
			assets = appendOpenAICompatibleImageAsset(assets, task, index, item.URL, item.B64JSON, item.RevisedPrompt)
		}
	}
	if len(assets) == 0 {
		return errors.New("YuCore media upstream returned no image assets")
	}
	return settleYucoreMediaTaskWithAssets(task, assets)
}

func runOpenAICompatibleYucoreTask(task *YucoreMediaTask, config yucoreMediaAdapterConfig) error {
	capability := yucoreMediaCapabilityForTask(task, config)
	if referenceCount := len(yucoreMediaReferenceAssets(task)); referenceCount > capability.MaxReferenceImages {
		return fmt.Errorf("model accepts at most %d reference images", capability.MaxReferenceImages)
	}
	if capability.Transport == yucoreMediaTransportAsyncTask {
		return runOpenAICompatibleAsyncTask(task, config, capability)
	}
	if task.Kind != "image" {
		return errors.New("sync-image transport only supports image tasks")
	}
	if task.Mode == "image-to-image" {
		return runOpenAICompatibleYucoreImageEditTask(task, config, capability)
	}
	return runOpenAICompatibleYucoreImageTask(task, config, capability)
}
