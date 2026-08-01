package model

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYucoreMediaModelCapabilities(t *testing.T) {
	objectConfig := `{
		"grok-image":{"transport":"sync-image","edit_path":"/v1/images/edits"},
		"fixed-video":{"transport":"async-video-task","duration_policy":"fixed","fixed_duration_seconds":10}
	}`
	capabilities := parseYucoreMediaModelCapabilities(objectConfig)
	require.Len(t, capabilities, 2)
	assert.Equal(t, yucoreMediaTransportSyncImage, capabilities["grok-image"].Transport)
	assert.Equal(t, 10, capabilities["fixed-video"].FixedDurationSeconds)

	arrayConfig := `[{"model":"veo","transport":"async-task","duration_policy":"duration"}]`
	capabilities = parseYucoreMediaModelCapabilities(arrayConfig)
	require.Len(t, capabilities, 1)
	assert.Equal(t, yucoreMediaDurationPolicyDuration, capabilities["veo"].DurationPolicy)

	assert.Nil(t, parseYucoreMediaModelCapabilities("not-json"))
}

func TestValidateYucoreMediaModelCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: ""},
		{name: "valid async", raw: `{"video":{"transport":"async-task","status_path":"/v1/videos/{task_id}","duration_policy":"seconds"}}`},
		{name: "invalid JSON", raw: "not-json", wantErr: "must be a JSON object or array"},
		{name: "invalid transport", raw: `{"video":{"transport":"websocket"}}`, wantErr: "invalid transport"},
		{name: "missing task placeholder", raw: `{"video":{"transport":"async-task","status_path":"/v1/videos/status"}}`, wantErr: "must contain {task_id}"},
		{name: "invalid fixed duration", raw: `{"video":{"duration_policy":"fixed"}}`, wantErr: "requires a positive fixed duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYucoreMediaModelCapabilities(tt.raw)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestYucoreMediaCapabilityForTask(t *testing.T) {
	videoTask := &YucoreMediaTask{Kind: "video", ModelId: "GROK-VIDEO"}
	config := yucoreMediaAdapterConfig{ModelCapabilities: map[string]YucoreMediaModelCapability{
		"grok-video": {
			Transport:           "async-video-task",
			UpstreamModel:       "upstream-grok-video",
			DurationPolicy:      yucoreMediaDurationPolicyFixed,
			PollIntervalSeconds: 3,
		},
	}}
	capability := yucoreMediaCapabilityForTask(videoTask, config)
	assert.Equal(t, yucoreMediaTransportAsyncTask, capability.Transport)
	assert.Equal(t, "/v1/videos", capability.CreatePath)
	assert.Equal(t, "/v1/videos/{task_id}", capability.StatusPath)
	assert.Equal(t, yucoreMediaDurationPolicyFixed, capability.DurationPolicy)
	assert.Equal(t, 3, capability.PollIntervalSeconds)
	assert.Equal(t, "upstream-grok-video", yucoreMediaCapabilityModel(videoTask, capability))

	imageCapability := yucoreMediaCapabilityForTask(&YucoreMediaTask{Kind: "image", ModelId: "image"}, yucoreMediaAdapterConfig{})
	assert.Equal(t, yucoreMediaTransportSyncImage, imageCapability.Transport)
	assert.Equal(t, "/v1/images/generations", imageCapability.CreatePath)
	assert.Equal(t, yucoreMediaDurationPolicyNone, imageCapability.DurationPolicy)
}

func TestBuildOpenAICompatibleAsyncPayloadDurationPolicies(t *testing.T) {
	task := &YucoreMediaTask{
		Kind:        "video",
		ModelId:     "video-model",
		Prompt:      "animate this frame",
		Size:        "1080p",
		AspectRatio: "16:9",
		Inputs:      `[{"sourceUrl":"https://cdn.example.com/input.png"}]`,
		Metadata:    `{"duration":8}`,
	}

	durationPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{DurationPolicy: yucoreMediaDurationPolicyDuration})
	assert.Equal(t, 8, durationPayload["duration"])
	assert.NotContains(t, durationPayload, "seconds")
	assert.Equal(t, "https://cdn.example.com/input.png", durationPayload["image"])

	secondsPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{DurationPolicy: yucoreMediaDurationPolicySeconds})
	assert.Equal(t, "8", secondsPayload["seconds"])
	assert.NotContains(t, secondsPayload, "duration")

	fixedPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy:       yucoreMediaDurationPolicyFixed,
		FixedDurationSeconds: 10,
	})
	assert.NotContains(t, fixedPayload, "duration")
	assert.NotContains(t, fixedPayload, "seconds")

	nonePayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{DurationPolicy: yucoreMediaDurationPolicyNone})
	assert.NotContains(t, nonePayload, "duration")
	assert.NotContains(t, nonePayload, "seconds")

	filteredPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		UpstreamModel:     "provider-video-model",
		DurationPolicy:    yucoreMediaDurationPolicySeconds,
		AllowedParameters: []string{"seconds"},
		ResponseFormat:    "b64_json",
	})
	assert.Equal(t, "provider-video-model", filteredPayload["model"])
	assert.Equal(t, "8", filteredPayload["seconds"])
	assert.NotContains(t, filteredPayload, "size")
	assert.NotContains(t, filteredPayload, "aspect_ratio")
	assert.NotContains(t, filteredPayload, "image")
	assert.NotContains(t, filteredPayload, "response_format")

	formattedPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy: yucoreMediaDurationPolicyNone,
		ResponseFormat: "b64_json",
	})
	assert.Equal(t, "b64_json", formattedPayload["response_format"])

	videoAliasPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy:    yucoreMediaDurationPolicySeconds,
		AllowedParameters: []string{"seconds", "size", "ratio", "image_url", "response_format"},
		ResponseFormat:    "url",
	})
	assert.Equal(t, "8", videoAliasPayload["seconds"])
	assert.Equal(t, "1080p", videoAliasPayload["size"])
	assert.Equal(t, "16:9", videoAliasPayload["ratio"])
	assert.Equal(t, "https://cdn.example.com/input.png", videoAliasPayload["image_url"])
	assert.Equal(t, "url", videoAliasPayload["response_format"])
	assert.NotContains(t, videoAliasPayload, "aspect_ratio")
	assert.NotContains(t, videoAliasPayload, "image")

	grokPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy:    yucoreMediaDurationPolicySeconds,
		AllowedParameters: []string{"seconds", "size", "aspect_ratio", "image_urls", "response_format"},
		ResponseFormat:    "url",
	})
	assert.Equal(t, "8", grokPayload["seconds"])
	assert.Equal(t, "1080p", grokPayload["size"])
	assert.Equal(t, "16:9", grokPayload["aspect_ratio"])
	assert.Equal(t, []string{"https://cdn.example.com/input.png"}, grokPayload["image_urls"])
	assert.Equal(t, "url", grokPayload["response_format"])
	assert.NotContains(t, grokPayload, "ratio")
	assert.NotContains(t, grokPayload, "image_url")
}

func TestOpenAICompatibleTaskResponseNormalization(t *testing.T) {
	tests := []struct {
		name       string
		payload    map[string]any
		taskID     string
		status     string
		resultURLs []string
	}{
		{
			name:       "top-level task id and video URL",
			payload:    map[string]any{"id": "task-1", "status": "succeeded", "video_url": "https://cdn.example.com/a.mp4"},
			taskID:     "task-1",
			status:     YucoreMediaTaskStatusCompleted,
			resultURLs: []string{"https://cdn.example.com/a.mp4"},
		},
		{
			name: "nested data output",
			payload: map[string]any{"data": map[string]any{
				"task_id": "task-2",
				"status":  "running",
				"output":  []any{map[string]any{"url": "https://cdn.example.com/b.mp4"}},
			}},
			taskID:     "task-2",
			status:     YucoreMediaTaskStatusProcessing,
			resultURLs: []string{"https://cdn.example.com/b.mp4"},
		},
		{
			name: "metadata result URLs",
			payload: map[string]any{"data": map[string]any{"task": map[string]any{
				"id":       "task-3",
				"status":   "success",
				"metadata": map[string]any{"result_urls": []any{"https://cdn.example.com/c.png"}},
			}}},
			taskID:     "task-3",
			status:     YucoreMediaTaskStatusCompleted,
			resultURLs: []string{"https://cdn.example.com/c.png"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.taskID, openAICompatibleTaskID(tt.payload))
			assert.Equal(t, tt.status, openAICompatibleTaskStatus(tt.payload))
			assert.Equal(t, tt.resultURLs, openAICompatibleResultURLs(tt.payload))
		})
	}
}

func TestYucoreMediaOpenAIURL(t *testing.T) {
	endpoint, err := yucoreMediaOpenAIURL("https://provider.example/v1", "/v1/videos")
	require.NoError(t, err)
	assert.Equal(t, "https://provider.example/v1/videos", endpoint)

	endpoint, err = yucoreMediaOpenAIURL("https://provider.example/gateway", "v1/images/edits")
	require.NoError(t, err)
	assert.Equal(t, "https://provider.example/gateway/v1/images/edits", endpoint)

	_, err = yucoreMediaOpenAIURL("provider.example", "/v1/videos")
	assert.Error(t, err)
}

func TestRequestOpenAICompatibleJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/videos", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"task_id":"task-4","status":"queued"}}`))
	}))
	t.Cleanup(server.Close)

	payload, err := requestOpenAICompatibleJSON(yucoreMediaAdapterConfig{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 5,
	}, http.MethodPost, "/v1/videos", map[string]any{"model": "video"})
	require.NoError(t, err)
	assert.Equal(t, "task-4", openAICompatibleTaskID(payload))
}

func TestReadOpenAICompatibleReferenceDataURL(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	decoded, mimeType, err := readOpenAICompatibleReference(source, 5)
	require.NoError(t, err)
	assert.Equal(t, data, decoded)
	assert.Equal(t, "image/png", mimeType)

	_, _, err = readOpenAICompatibleReference("http://example.com/image.png", 5)
	assert.EqualError(t, err, "reference image must use a public HTTPS URL")
}

func TestYucoreMediaManagedTokenIsStablePerUserAndGroup(t *testing.T) {
	truncateTables(t)

	first, err := getOrCreateYucoreMediaManagedToken(42, "image-group")
	require.NoError(t, err)
	require.NotEmpty(t, first.Key)
	assert.True(t, first.UnlimitedQuota)
	assert.Equal(t, "image-group", first.Group)
	assert.Equal(t, int64(-1), first.ExpiredTime)

	second, err := getOrCreateYucoreMediaManagedToken(42, "image-group")
	require.NoError(t, err)
	assert.Equal(t, first.Id, second.Id)
	assert.Equal(t, first.Key, second.Key)

	task := &YucoreMediaTask{
		UserId:   42,
		Metadata: `{"adapter":"yuapi-channel"}`,
	}
	config, err := yucoreMediaOpenAIConfigForTask(task, yucoreMediaAdapterConfig{ManagedTokenGroup: "image-group"})
	require.NoError(t, err)
	assert.Equal(t, first.Key, config.APIKey)
}

func TestYucoreMediaRoutedTaskIDUsesPublicTaskID(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Task{}))
	require.NoError(t, DB.Exec("DELETE FROM tasks").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM tasks")
	})

	routedTask := &Task{
		TaskID:    "task_public",
		UserId:    42,
		CreatedAt: common.GetTimestamp(),
		Properties: Properties{
			OriginModelName: "video-model",
		},
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "task_provider",
		},
	}
	require.NoError(t, DB.Create(routedTask).Error)

	mediaTask := &YucoreMediaTask{
		UserId:   42,
		ModelId:  "video-model",
		Metadata: `{"adapter":"yuapi-channel"}`,
	}
	assert.Equal(t, "task_public", yucoreMediaRoutedTaskID(mediaTask, "task_provider"))
	assert.Equal(t, "task_unknown", yucoreMediaRoutedTaskID(mediaTask, "task_unknown"))
}

func TestApplyOpenAICompatibleTaskPayloadPreservesPollingTaskID(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&YucoreMediaTask{}))
	task := &YucoreMediaTask{
		TaskId:   "yu_polling_id",
		UserId:   42,
		Kind:     "video",
		ModelId:  "video-model",
		Status:   YucoreMediaTaskStatusProcessing,
		Progress: 8,
		Metadata: `{"upstream_task_id":"task_public","provider_task_id":"task_provider"}`,
	}
	require.NoError(t, DB.Create(task).Error)
	t.Cleanup(func() {
		DB.Delete(task)
	})

	err := applyOpenAICompatibleTaskPayload(task, map[string]any{
		"task_id":  "task_provider",
		"status":   "processing",
		"progress": 64,
	}, YucoreMediaModelCapability{Transport: yucoreMediaTransportAsyncTask})
	require.NoError(t, err)

	metadata := yucoreMediaMetadataMap(task.Metadata)
	assert.Equal(t, "task_public", metadata["upstream_task_id"])
	assert.Equal(t, "task_provider", metadata["provider_task_id"])
	assert.Equal(t, 64, task.Progress)
}

func TestYucoreMediaRunnableAdapters(t *testing.T) {
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterOpenAICompatible))
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterYuAPIChannel))
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterUAGProxy))
	assert.False(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterMock))
	assert.False(t, isYucoreMediaRunnableAdapter("unknown"))
}

func TestYucoreMediaConfiguredModelIDs(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter": YucoreMediaAdapterYuAPIChannel,
		"yucore_media.model_capabilities": `{
			"Grok-Imagine-Image":{"transport":"sync-image"},
			"gpt-image-2-adobe":{"transport":"sync-image"}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	configured := YucoreMediaConfiguredModelIDs()
	require.Len(t, configured, 2)
	assert.Contains(t, configured, "grok-imagine-image")
	assert.Contains(t, configured, "gpt-image-2-adobe")

	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.adapter"] = YucoreMediaAdapterUAGProxy
	common.OptionMapRWMutex.Unlock()
	assert.Nil(t, YucoreMediaConfiguredModelIDs())
}

func TestCreateYucoreCanvasAgentExecutionRunsYuAPIChannelTask(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&YucoreCanvasAgentRun{}, &YucoreMediaTask{}))
	require.NoError(t, DB.Exec("DELETE FROM yucore_canvas_agent_runs").Error)
	require.NoError(t, DB.Exec("DELETE FROM yucore_media_tasks").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM yucore_canvas_agent_runs")
		DB.Exec("DELETE FROM yucore_media_tasks")
	})
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		requested <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}]}`))
	}))
	t.Cleanup(server.Close)

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.base_url":            server.URL,
		"yucore_media.managed_token_group": "image-group",
		"yucore_media.timeout_seconds":     "5",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	run := &YucoreCanvasAgentRun{CanvasId: 1, UserId: 42, Prompt: "draw", Status: YucoreCanvasAgentRunStatusRunning}
	task := &YucoreMediaTask{UserId: 42, Kind: "image", ModelId: "managed-image", Prompt: "draw"}
	require.NoError(t, CreateYucoreCanvasAgentExecution(run, task, nil))

	select {
	case <-requested:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "YuAPI channel canvas task was not dispatched")
	}
	require.Eventually(t, func() bool {
		var stored YucoreMediaTask
		return DB.Where("id = ?", task.Id).First(&stored).Error == nil && stored.Status == YucoreMediaTaskStatusCompleted
	}, 2*time.Second, 10*time.Millisecond)
}

func TestEstimateYucoreMediaTaskCostUsesYuAPIChannelPrice(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"managed-image":0.032}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"image-group":1}`))

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.managed_token_group": "image-group",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	cost := estimateYucoreMediaTaskCost(&YucoreMediaTask{
		Kind:    "image",
		ModelId: "managed-image",
		Count:   1,
	})
	assert.Equal(t, 16000, cost)
}

func TestEstimateYucoreMediaTaskCostUsesVideoDuration(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"managed-video":0.04}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"media-group":1}`))

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.managed_token_group": "media-group",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	cost := estimateYucoreMediaTaskCost(&YucoreMediaTask{
		Kind:     "video",
		ModelId:  "managed-video",
		Metadata: `{"duration":5}`,
	})
	assert.Equal(t, 100000, cost)
}

func TestEstimateYucoreMediaTaskCostKeepsPatchedVideoPricePerCall(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalPatches := constant.TaskPricePatches
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"managed-video":0.04}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"media-group":1}`))
	constant.TaskPricePatches = []string{"managed-video"}

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.managed_token_group": "media-group",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		constant.TaskPricePatches = originalPatches
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	cost := estimateYucoreMediaTaskCost(&YucoreMediaTask{
		Kind:     "video",
		ModelId:  "managed-video",
		Metadata: `{"duration":15}`,
	})
	assert.Equal(t, 20000, cost)
}

func TestYucoreMediaModelUnitPriceUsesManagedGroupRatio(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"managed-image":0.025}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"media-group":1.2}`))

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.managed_token_group": "media-group",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	price, ok := YucoreMediaModelUnitPrice("managed-image")
	require.True(t, ok)
	assert.InDelta(t, 0.03, price, 0.0000001)
}
