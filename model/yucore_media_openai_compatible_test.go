package model

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
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
		Inputs:      `[{"role":"image","url":"https://cdn.example.com/input.png"}]`,
		Metadata:    `{"duration":8}`,
	}

	durationPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy:    yucoreMediaDurationPolicyDuration,
		AllowedParameters: []string{"duration", "size", "aspect_ratio", "image"},
	})
	assert.Equal(t, 8, durationPayload["duration"])
	assert.NotContains(t, durationPayload, "seconds")
	assert.Equal(t, "https://cdn.example.com/input.png", durationPayload["image"])

	secondsPayload := buildOpenAICompatibleAsyncPayload(task, YucoreMediaModelCapability{
		DurationPolicy:    yucoreMediaDurationPolicySeconds,
		AllowedParameters: []string{"seconds"},
	})
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
		DurationPolicy:    yucoreMediaDurationPolicyNone,
		AllowedParameters: []string{"response_format"},
		ResponseFormat:    "b64_json",
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

func newCanonicalOpenAICompatiblePayloadTask(t *testing.T, modelID string, references []YucoreMediaReferenceInput, metadata map[string]any) *YucoreMediaTask {
	t.Helper()
	inputs, err := common.Marshal(references)
	require.NoError(t, err)
	rawMetadata, err := common.Marshal(metadata)
	require.NoError(t, err)
	return &YucoreMediaTask{
		Kind:           "video",
		ModelId:        modelID,
		Prompt:         "keep the subject consistent",
		NegativePrompt: "watermark",
		AspectRatio:    "16:9",
		Size:           "720p",
		Inputs:         string(inputs),
		Metadata:       string(rawMetadata),
	}
}

func TestBuildOpenAICompatibleAsyncPayloadOmni(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)

	t.Run("single image", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "omni-fast", []YucoreMediaReferenceInput{
			{Role: "image", URL: "https://cdn.example.com/main.png"},
		}, map[string]any{"duration": 10, "resolution": "720p", "unknown_upstream_parameter": "do-not-forward"})
		payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

		assert.Equal(t, []string{"https://cdn.example.com/main.png"}, payload["reference_image_urls"])
		assert.NotContains(t, payload, "image_url")
		assert.NotContains(t, payload, "image_urls")
		assert.NotContains(t, payload, "duration")
		assert.NotContains(t, payload, "seconds")
		assert.NotContains(t, payload, "resolution")
		assert.NotContains(t, payload, "size")
		assert.NotContains(t, payload, "unknown_upstream_parameter")
	})

	t.Run("first and last frames", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "omni-fast", []YucoreMediaReferenceInput{
			{Role: "first_frame", URL: "https://cdn.example.com/first.png"},
			{Role: "last_frame", URL: "https://cdn.example.com/last.png"},
		}, map[string]any{"reference_mode": "frames"})
		payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

		assert.Equal(t, "https://cdn.example.com/first.png", payload["first_image_url"])
		assert.Equal(t, "https://cdn.example.com/last.png", payload["last_image_url"])
		assert.NotContains(t, payload, "image_url")
		assert.NotContains(t, payload, "image_urls")
	})

	t.Run("video to video", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "omni-v2v", []YucoreMediaReferenceInput{
			{Role: "video", URL: "https://cdn.example.com/source.mp4"},
		}, map[string]any{"reference_mode": "media"})
		payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

		assert.Equal(t, []string{"https://cdn.example.com/source.mp4"}, payload["reference_videos"])
		assert.NotContains(t, payload, "video")
		assert.NotContains(t, payload, "video_url")
	})

	t.Run("frame presence excludes video without image authorization", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "omni-v2v", []YucoreMediaReferenceInput{
			{Role: "first_frame", URL: "https://cdn.example.com/first.png"},
			{Role: "video", URL: "https://cdn.example.com/source.mp4"},
		}, map[string]any{"reference_mode": "frames"})
		capability := catalog[task.ModelId]
		capability.AllowedParameters = []string{"video"}
		payload := buildOpenAICompatibleAsyncPayload(task, capability)

		for _, forbidden := range []string{"first_image_url", "last_image_url", "video_url", "reference_videos"} {
			assert.NotContains(t, payload, forbidden)
		}
	})
}

func TestBuildOpenAICompatibleAsyncPayloadCangyuan(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	tests := []struct {
		model      string
		references []YucoreMediaReferenceInput
		metadata   map[string]any
		want       map[string]any
		absent     []string
	}{
		{
			model:      "grok-video",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}},
			metadata:   map[string]any{"duration": 4, "resolution": "480p", "unknown_upstream_parameter": "do-not-forward"},
			want:       map[string]any{"duration": 4, "resolution": "480p", "reference_image_urls": []string{"https://cdn.example.com/ref.png"}},
			absent:     []string{"seconds", "size", "image", "image_url", "image_urls"},
		},
		{
			model:      "happyhouse-1.0",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}, {Role: "video", URL: "https://cdn.example.com/ref.mp4"}},
			metadata:   map[string]any{"duration": 3, "resolution": "720p", "generate_audio": false},
			want:       map[string]any{"duration": 3, "resolution": "720p", "generate_audio": false, "reference_image_urls": []string{"https://cdn.example.com/ref.png"}, "reference_videos": []string{"https://cdn.example.com/ref.mp4"}},
			absent:     []string{"seconds", "size", "audio", "image_urls", "video_url"},
		},
		{
			model:      "minimax-h3-2k",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}, {Role: "audio", URL: "https://cdn.example.com/ref.mp3"}},
			metadata:   map[string]any{"duration": 5, "resolution": "2k", "generate_audio": false},
			want:       map[string]any{"duration": 5, "resolution": "2k", "generate_audio": false, "reference_image_urls": []string{"https://cdn.example.com/ref.png"}, "reference_audios": []string{"https://cdn.example.com/ref.mp3"}},
			absent:     []string{"seconds", "size", "image", "image_url", "image_urls", "audio"},
		},
		{
			model:      "seedance-2.0",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}},
			metadata:   map[string]any{"duration": 4, "resolution": "720p", "generate_audio": false},
			want:       map[string]any{"duration": 4, "generate_audio": false, "reference_image_urls": []string{"https://cdn.example.com/ref.png"}},
			absent:     []string{"seconds", "resolution", "size", "audio", "image", "image_url", "image_urls"},
		},
		{
			model:      "sd7-seedance-2.0-720p",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}},
			metadata:   map[string]any{"duration": 4, "resolution": "720p", "generate_audio": false},
			want:       map[string]any{"duration": 4, "generate_audio": false, "reference_image_urls": []string{"https://cdn.example.com/ref.png"}},
			absent:     []string{"seconds", "resolution", "size", "audio", "image", "image_url", "image_urls"},
		},
		{
			model:      "sd8-seedance-2.0",
			references: []YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/ref.png"}},
			metadata:   map[string]any{"duration": 5, "resolution": "720p", "generate_audio": false},
			want:       map[string]any{"duration": 5, "reference_image_urls": []string{"https://cdn.example.com/ref.png"}},
			absent:     []string{"seconds", "resolution", "size", "generate_audio", "audio", "image", "image_url", "image_urls"},
		},
	}
	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			task := newCanonicalOpenAICompatiblePayloadTask(t, test.model, test.references, test.metadata)
			payload := buildOpenAICompatibleAsyncPayload(task, catalog[test.model])
			assert.Equal(t, test.model, payload["model"])
			assert.Equal(t, "keep the subject consistent", payload["prompt"])
			assert.Equal(t, "16:9", payload["aspect_ratio"])
			for field, value := range test.want {
				assert.Equal(t, value, payload[field], field)
			}
			for _, field := range append(test.absent, "negative_prompt", "seed", "unknown_upstream_parameter") {
				assert.NotContains(t, payload, field)
			}
		})
	}
}

func TestBuildOpenAICompatibleAsyncPayloadHappyhouseMultipleImages(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	task := newCanonicalOpenAICompatiblePayloadTask(t, "happyhouse-1.0", []YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example.com/main.png"},
		{Role: "image", URL: "https://cdn.example.com/style.png"},
	}, map[string]any{
		"duration": 10, "resolution": "1080p", "generate_audio": false, "seed": int64(0),
	})
	payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

	assert.Equal(t, 10, payload["duration"])
	assert.Equal(t, "1080p", payload["resolution"])
	assert.Equal(t, []string{"https://cdn.example.com/main.png", "https://cdn.example.com/style.png"}, payload["reference_image_urls"])
	assert.Equal(t, false, payload["generate_audio"])
	assert.NotContains(t, payload, "seconds")
	assert.NotContains(t, payload, "size")
	assert.NotContains(t, payload, "negative_prompt")
	assert.NotContains(t, payload, "audio")
	assert.NotContains(t, payload, "seed")
}

func TestBuildOpenAICompatibleAsyncPayloadMinimaxFrames(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	task := newCanonicalOpenAICompatiblePayloadTask(t, "minimax-h3-2k", []YucoreMediaReferenceInput{
		{Role: "first_frame", URL: "https://cdn.example.com/first.png"},
		{Role: "last_frame", URL: "https://cdn.example.com/last.png"},
	}, map[string]any{
		"duration": 5, "resolution": "2k", "generate_audio": false, "reference_mode": "frames",
	})
	payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

	assert.Equal(t, 5, payload["duration"])
	assert.Equal(t, "2k", payload["resolution"])
	assert.Equal(t, "https://cdn.example.com/first.png", payload["first_image_url"])
	assert.Equal(t, "https://cdn.example.com/last.png", payload["last_image_url"])
	assert.Equal(t, false, payload["generate_audio"])
	assert.NotContains(t, payload, "seconds")
	assert.NotContains(t, payload, "size")
	assert.NotContains(t, payload, "image")
	assert.NotContains(t, payload, "reference_image_urls")
	assert.NotContains(t, payload, "negative_prompt")
	assert.NotContains(t, payload, "seed")
}

func TestBuildOpenAICompatibleAsyncPayloadSeedance20(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)

	t.Run("multimodal canonical references", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "seedance-2.0", []YucoreMediaReferenceInput{
			{Role: "image", URL: "https://cdn.example.com/main.png"},
			{Role: "image", URL: "https://cdn.example.com/style.png"},
			{Role: "video", URL: "https://cdn.example.com/motion.mp4"},
			{Role: "audio", URL: "https://cdn.example.com/music.mp3"},
		}, map[string]any{
			"duration": 6, "resolution": "720p", "generate_audio": false, "seed": int64(0),
			"reference_mode": "media", "unknown_upstream_parameter": "do-not-forward",
		})
		payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

		assert.Equal(t, []string{"https://cdn.example.com/main.png", "https://cdn.example.com/style.png"}, payload["reference_image_urls"])
		assert.Equal(t, []string{"https://cdn.example.com/motion.mp4"}, payload["reference_videos"])
		assert.Equal(t, []string{"https://cdn.example.com/music.mp3"}, payload["reference_audios"])
		assert.Equal(t, false, payload["generate_audio"])
		assert.Equal(t, 6, payload["duration"])
		assert.NotContains(t, payload, "resolution")
		assert.NotContains(t, payload, "image_url")
		assert.NotContains(t, payload, "image_urls")
		assert.NotContains(t, payload, "images")
		assert.NotContains(t, payload, "video")
		assert.NotContains(t, payload, "audio")
		assert.NotContains(t, payload, "seed")
		assert.NotContains(t, payload, "negative_prompt")
		assert.NotContains(t, payload, "seconds")
		assert.NotContains(t, payload, "size")
		assert.NotContains(t, payload, "unknown_upstream_parameter")
	})

	t.Run("undocumented seed values are not forwarded", func(t *testing.T) {
		for _, seed := range []int64{0, 9007199254740993, math.MaxInt64, -1} {
			t.Run(yucoreMediaStringValue(seed), func(t *testing.T) {
				task := newCanonicalOpenAICompatiblePayloadTask(t, "seedance-2.0", nil, map[string]any{"seed": seed})
				payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

				assert.NotContains(t, payload, "seed")
			})
		}
	})

	t.Run("unsupported frame references are not forwarded", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "seedance-2.0", []YucoreMediaReferenceInput{
			{Role: "first_frame", URL: "https://cdn.example.com/first.png"},
		}, map[string]any{"reference_mode": "frames"})
		payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

		for _, forbidden := range []string{"first_image_url", "last_image_url", "image_url", "image_urls", "reference_image_urls"} {
			assert.NotContains(t, payload, forbidden)
		}
	})

	t.Run("metadata reference aliases are never a payload source", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			inputs   string
			metadata string
		}{
			{name: "empty inputs", inputs: "", metadata: `{"ref_assets":["https://cdn.example.com/metadata.png"]}`},
			{name: "malformed inputs", inputs: "{", metadata: `{"refAssets":["https://cdn.example.com/metadata.png"]}`},
		} {
			t.Run(test.name, func(t *testing.T) {
				task := newCanonicalOpenAICompatiblePayloadTask(t, "seedance-2.0", nil, nil)
				task.Inputs = test.inputs
				task.Metadata = test.metadata
				payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

				for _, forbidden := range []string{"image", "image_url", "image_urls", "images", "reference_image_urls", "video_url", "reference_videos", "reference_audios"} {
					assert.NotContains(t, payload, forbidden)
				}
			})
		}
	})

	t.Run("partial typed reference decoding is discarded", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			inputs string
		}{
			{
				name:   "valid image before scalar",
				inputs: `[{"role":"image","url":"https://cdn.example.com/partial.png"},1]`,
			},
			{
				name:   "valid video before invalid role type",
				inputs: `[{"role":"video","url":"https://cdn.example.com/partial.mp4"},{"role":1,"url":"https://cdn.example.com/invalid.mp4"}]`,
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				task := newCanonicalOpenAICompatiblePayloadTask(t, "seedance-2.0", nil, nil)
				task.Inputs = test.inputs
				payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

				for _, forbidden := range []string{"image", "image_url", "image_urls", "images", "reference_image_urls", "video_url", "reference_videos", "reference_audios"} {
					assert.NotContains(t, payload, forbidden)
				}
			})
		}
	})

	t.Run("semantic gates suppress every derived alias", func(t *testing.T) {
		task := newCanonicalOpenAICompatiblePayloadTask(t, "sd4-seedance-2.0", []YucoreMediaReferenceInput{
			{Role: "image", URL: "https://cdn.example.com/main.png"},
			{Role: "video", URL: "https://cdn.example.com/motion.mp4"},
			{Role: "audio", URL: "https://cdn.example.com/music.mp3"},
		}, map[string]any{"generate_audio": false})
		capability := catalog[task.ModelId]
		capability.AllowedParameters = []string{}
		payload := buildOpenAICompatibleAsyncPayload(task, capability)

		for _, forbidden := range []string{"size", "aspect_ratio", "image_url", "image_urls", "images", "reference_image_urls", "video_url", "reference_videos", "reference_audios", "audio", "generate_audio"} {
			assert.NotContains(t, payload, forbidden)
		}
	})
}

func TestBuildOpenAICompatibleAsyncPayloadSD8OmitsUnsupportedFields(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	task := newCanonicalOpenAICompatiblePayloadTask(t, "sd8-seedance-2.0", []YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example.com/first.png"},
		{Role: "image", URL: "https://cdn.example.com/last.png"},
	}, map[string]any{
		"duration": 5, "resolution": "1080p", "generate_audio": false, "seed": int64(0),
		"unknown_upstream_parameter": "do-not-forward",
	})
	payload := buildOpenAICompatibleAsyncPayload(task, catalog[task.ModelId])

	assert.Equal(t, 5, payload["duration"])
	assert.Equal(t, []string{"https://cdn.example.com/first.png", "https://cdn.example.com/last.png"}, payload["reference_image_urls"])
	for _, forbidden := range []string{"seconds", "resolution", "size", "image", "image_url", "image_urls", "audio", "generate_audio", "seed", "negative_prompt", "unknown_upstream_parameter"} {
		assert.NotContains(t, payload, forbidden)
	}
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
	assert.Equal(t, "image-group", config.ManagedTokenGroup)

	selected, err := getOrCreateYucoreMediaManagedToken(42, "selected-group")
	require.NoError(t, err)
	task.BillingGroup = "selected-group"
	config, err = yucoreMediaOpenAIConfigForTask(task, yucoreMediaAdapterConfig{ManagedTokenGroup: "image-group"})
	require.NoError(t, err)
	assert.Equal(t, selected.Key, config.APIKey)
	assert.Equal(t, "selected-group", config.ManagedTokenGroup)
}

func TestYucoreMediaAssetProxyHeadersBindToNormalizedOriginAndBasePath(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":  YucoreMediaAdapterOpenAICompatible,
		"yucore_media.api_key":  "origin-scoped-key",
		"yucore_media.base_url": "https://Example.COM:443/v1",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})
	task := &YucoreMediaTask{Metadata: `{"adapter":"openai-compatible"}`}

	for _, target := range []string{
		"https://example.com/v1/content",
		"https://EXAMPLE.com:443/v1/content?signature=one",
	} {
		headers, err := YucoreMediaAssetProxyHeaders(task, target)
		require.NoError(t, err)
		assert.Equal(t, "Bearer origin-scoped-key", headers["Authorization"])
	}
	for _, target := range []string{
		"http://example.com/v1/content",
		"https://example.com:444/v1/content",
		"https://example.com/v10/content",
		"https://cdn.example.com/v1/content",
	} {
		headers, err := YucoreMediaAssetProxyHeaders(task, target)
		require.NoError(t, err)
		assert.Empty(t, headers)
	}

	_, err := YucoreMediaAssetProxyHeaders(task, "https://user:password@example.com/v1/content")
	assert.EqualError(t, err, "YuCore media asset source is invalid")
}

func TestResolveYucoreMediaAssetSourceURLRejectsUserinfo(t *testing.T) {
	_, err := ResolveYucoreMediaAssetSourceURL("https://user:password@example.com/content")
	assert.EqualError(t, err, "YuCore media asset source URL must not contain userinfo")
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

func TestOpenAICompatibleTaskPollsAcceptedIDOnly(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&YucoreMediaTask{}))

	const acceptedTaskID = "accepted/id ?"
	postCount := 0
	getCount := 0
	getPaths := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			postCount++
			assert.Equal(t, "/v1/videos", r.URL.Path)
			_, _ = w.Write([]byte(`{"data":{"task":{"task_id":"accepted/id ?","status":"queued"}}}`))
		case http.MethodGet:
			getCount++
			getPaths = append(getPaths, r.URL.EscapedPath())
			switch getCount {
			case 1:
				_, _ = w.Write([]byte(`{"data":{"task":{"task_id":"noisy-poll-id","status":"running","progress":35}},"id":"other-noisy-id"}`))
			case 2:
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":{"message":"do not persist this upstream body"}}`))
			default:
				_, _ = w.Write([]byte(`{"data":{"task":{"task_id":"noisy-final-id","status":"succeeded","result":{"content":{"url":"media/results/final clip.mp4","thumbnail_url":"media/thumbs/final.jpg","mime_type":"video/webm","duration_ms":4321,"width":1280,"height":720}}}}}`))
			}
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":         YucoreMediaAdapterOpenAICompatible,
		"yucore_media.base_url":        server.URL,
		"yucore_media.api_key":         "test-key",
		"yucore_media.timeout_seconds": "5",
		"yucore_media.model_capabilities": `{
			"video-model":{"transport":"async-task","create_path":"/v1/videos","status_path":"/v1/videos/{task_id}","poll_interval_seconds":1}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	task := &YucoreMediaTask{
		TaskId:      fmt.Sprintf("yu_poll_%d", time.Now().UnixNano()),
		UserId:      42,
		Kind:        "video",
		ModelId:     "video-model",
		Prompt:      "animate",
		Status:      YucoreMediaTaskStatusProcessing,
		Metadata:    `{"adapter":"openai-compatible"}`,
		CreatedTime: common.GetTimestamp(),
		UpdatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(task).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(task) })

	config := getYucoreMediaAdapterConfig()
	capability := yucoreMediaCapabilityForTask(task, config)
	require.NoError(t, runOpenAICompatibleAsyncTask(task, config, capability))
	assert.Equal(t, 1, postCount)
	assert.Equal(t, acceptedTaskID, yucoreMediaMetadataMap(task.Metadata)["upstream_task_id"])
	var stored YucoreMediaTask
	require.NoError(t, DB.Where("id = ?", task.Id).First(&stored).Error)
	assert.Equal(t, acceptedTaskID, yucoreMediaMetadataMap(stored.Metadata)["upstream_task_id"])

	forcePoll := func() {
		task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{"last_status_at": 0})
		require.NoError(t, DB.Model(task).Select("metadata").Updates(task).Error)
	}
	forcePoll()
	_, err := HydrateYucoreMediaTask(task)
	require.NoError(t, err)
	assert.Equal(t, YucoreMediaTaskStatusProcessing, task.Status)
	assert.Equal(t, acceptedTaskID, yucoreMediaMetadataMap(task.Metadata)["upstream_task_id"])

	forcePoll()
	_, err = HydrateYucoreMediaTask(task)
	require.NoError(t, err)
	metadata := yucoreMediaMetadataMap(task.Metadata)
	assert.Equal(t, YucoreMediaTaskStatusProcessing, task.Status)
	assert.Equal(t, acceptedTaskID, metadata["upstream_task_id"])
	assert.Equal(t, "YuCore media upstream returned 502", metadata["last_status_error"])
	assert.NotContains(t, task.Metadata, "do not persist this upstream body")

	forcePoll()
	_, err = HydrateYucoreMediaTask(task)
	require.NoError(t, err)
	assert.Equal(t, YucoreMediaTaskStatusCompleted, task.Status)
	metadata = yucoreMediaMetadataMap(task.Metadata)
	assert.Equal(t, acceptedTaskID, metadata["upstream_task_id"])
	assert.Equal(t, "succeeded", metadata["upstream_status"])
	assert.NotContains(t, metadata, "last_status_error")
	assets := YucoreMediaTaskAssets(task)
	require.Len(t, assets, 1)
	assert.Equal(t, "media/results/final clip.mp4", assets[0].SourceUrl)
	assert.Equal(t, "media/thumbs/final.jpg", assets[0].SourceThumbUrl)
	assert.Equal(t, "/api/yucore/media/tasks/"+task.TaskId+"/assets/0?variant=thumbnail", assets[0].ThumbUrl)
	assert.Equal(t, "video/webm", assets[0].MimeType)
	assert.Equal(t, 4321, assets[0].DurationMs)
	assert.Equal(t, 1280, assets[0].Width)
	assert.Equal(t, 720, assets[0].Height)
	require.NoError(t, DB.Where("id = ?", task.Id).First(&stored).Error)
	assert.Equal(t, task.Metadata, stored.Metadata)
	assert.Equal(t, task.Assets, stored.Assets)
	assert.Equal(t, 1, postCount)
	assert.Equal(t, 3, getCount)
	assert.Equal(t, []string{
		"/v1/videos/accepted%2Fid%20%3F",
		"/v1/videos/accepted%2Fid%20%3F",
		"/v1/videos/accepted%2Fid%20%3F",
	}, getPaths)
	for _, requestedPath := range getPaths {
		assert.False(t, strings.Contains(requestedPath, "noisy"))
	}
}

func TestApplyOpenAICompatibleTaskPayloadClearsStaleErrorFromMalformedMetadata(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&YucoreMediaTask{}))
	task := &YucoreMediaTask{
		TaskId: fmt.Sprintf("yu_malformed_metadata_%d", time.Now().UnixNano()), UserId: 42,
		Kind: "video", ModelId: "video-model", Status: YucoreMediaTaskStatusProcessing,
		Metadata: "not-json", CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(task).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(task) })

	require.NoError(t, applyOpenAICompatibleTaskPayload(task, map[string]any{
		"status": "processing", "progress": 50,
	}, YucoreMediaModelCapability{Transport: yucoreMediaTransportAsyncTask}))
	metadata := yucoreMediaMetadataMap(task.Metadata)
	assert.NotContains(t, metadata, "last_status_error")
	assert.Equal(t, "processing", metadata["upstream_status"])
	assert.Equal(t, 50, task.Progress)
}

func TestOpenAICompatibleTaskPollsNestedRoutedAcceptedID(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Task{}, &YucoreMediaTask{}))
	routedTask := &Task{
		TaskID:    "accepted_public_id",
		UserId:    84,
		CreatedAt: common.GetTimestamp(),
		Properties: Properties{
			OriginModelName: "routed-video-model",
		},
		PrivateData: TaskPrivateData{UpstreamTaskID: "provider_private_id"},
	}
	require.NoError(t, DB.Create(routedTask).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(routedTask) })

	postCount := 0
	getPaths := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			postCount++
			_, _ = w.Write([]byte(`{"data":{"task":{"task_id":"provider_private_id","status":"queued"}},"task_id":"noisy_top_level_id"}`))
			return
		}
		getPaths = append(getPaths, r.URL.EscapedPath())
		_, _ = w.Write([]byte(`{"data":{"task":{"task_id":"noisy_poll_id","status":"running","progress":40}}}`))
	}))
	t.Cleanup(server.Close)

	task := &YucoreMediaTask{
		TaskId:      fmt.Sprintf("yu_routed_poll_%d", time.Now().UnixNano()),
		UserId:      84,
		Kind:        "video",
		ModelId:     "routed-video-model",
		Prompt:      "animate",
		Status:      YucoreMediaTaskStatusProcessing,
		Metadata:    `{"adapter":"yuapi-channel"}`,
		CreatedTime: common.GetTimestamp(),
		UpdatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(task).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(task) })
	config := yucoreMediaAdapterConfig{
		BaseURL: server.URL, APIKey: "test-key", TimeoutSeconds: 5,
		ModelCapabilities: map[string]YucoreMediaModelCapability{
			"routed-video-model": {
				Transport: yucoreMediaTransportAsyncTask, CreatePath: "/v1/videos",
				StatusPath: "/v1/videos/{task_id}", PollIntervalSeconds: 1,
			},
		},
	}
	capability := yucoreMediaCapabilityForTask(task, config)
	require.NoError(t, runOpenAICompatibleAsyncTask(task, config, capability))
	metadata := yucoreMediaMetadataMap(task.Metadata)
	assert.Equal(t, "accepted_public_id", metadata["upstream_task_id"])
	assert.Equal(t, "provider_private_id", metadata["provider_task_id"])

	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{"last_status_at": 0})
	require.NoError(t, refreshOpenAICompatibleYucoreTask(task, config))
	metadata = yucoreMediaMetadataMap(task.Metadata)
	assert.Equal(t, "accepted_public_id", metadata["upstream_task_id"])
	assert.Equal(t, "provider_private_id", metadata["provider_task_id"])
	assert.Equal(t, 1, postCount)
	assert.Equal(t, []string{"/v1/videos/accepted_public_id"}, getPaths)
}

func TestOpenAICompatibleTaskResultMetadata(t *testing.T) {
	tests := []struct {
		name       string
		payload    map[string]any
		want       YucoreMediaAsset
		wantStatus string
	}{
		{
			name: "nested video result aliases",
			payload: map[string]any{"data": map[string]any{"task": map[string]any{
				"status": "succeeded",
				"results": []any{map[string]any{
					"video_url": "relative/videos/final.mp4",
					"metadata": map[string]any{
						"thumbnail_url": "relative/thumbs/final.jpg",
						"duration_ms":   "6100",
						"width":         float64(1920),
						"height":        float64(1080),
						"mime_type":     "video/mp4",
					},
				}},
			}}},
			want:       YucoreMediaAsset{SourceUrl: "relative/videos/final.mp4", ThumbUrl: "/api/yucore/media/tasks/yu_result/assets/0?variant=thumbnail", DurationMs: 6100, Width: 1920, Height: 1080, MimeType: "video/mp4"},
			wantStatus: "succeeded",
		},
		{
			name: "nested content URL and camel case metadata",
			payload: map[string]any{"data": map[string]any{"task": map[string]any{
				"state": "completed",
				"result": map[string]any{
					"thumbnailUrl": "/v1/videos/provider-task/thumbnail",
					"durationMs":   float64(4321),
					"width":        "1280",
					"height":       "720",
					"contentType":  "video/webm",
					"content": map[string]any{
						"url": "/v1/videos/provider-task/content",
					},
				},
			}}},
			want:       YucoreMediaAsset{SourceUrl: "/v1/videos/provider-task/content", ThumbUrl: "/api/yucore/media/tasks/yu_result/assets/0?variant=thumbnail", DurationMs: 4321, Width: 1280, Height: 720, MimeType: "video/webm"},
			wantStatus: "completed",
		},
		{
			name: "multiple nested content results keep enclosing metadata",
			payload: map[string]any{"data": map[string]any{"task": map[string]any{
				"status": "succeeded",
				"results": []any{
					map[string]any{"thumbnail_url": "signed/thumb-one", "mime_type": "video/mp4", "duration_ms": 1001, "width": 640, "height": 360, "content": map[string]any{"url": "signed/video-one"}},
					map[string]any{"thumbnail_url": "signed/thumb-two", "mime_type": "video/webm", "duration_ms": 2002, "width": 1280, "height": 720, "content": map[string]any{"url": "signed/video-two"}},
					map[string]any{"thumbnail_url": "signed/duplicate-thumb", "mime_type": "video/duplicate", "duration_ms": 9999, "content": map[string]any{"url": "signed/video-one"}},
				},
			}}},
			want:       YucoreMediaAsset{SourceUrl: "signed/video-one", ThumbUrl: "/api/yucore/media/tasks/yu_result/assets/0?variant=thumbnail", DurationMs: 1001, Width: 640, Height: 360, MimeType: "video/mp4"},
			wantStatus: "succeeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &YucoreMediaTask{TaskId: "yu_result", Kind: "video", ModelId: "video-model"}
			before, err := common.Marshal(tt.payload)
			require.NoError(t, err)
			assets := buildOpenAICompatibleTaskAssets(task, tt.payload)
			wantLen := 1
			if tt.name == "multiple nested content results keep enclosing metadata" {
				wantLen = 2
			}
			require.Len(t, assets, wantLen)
			assert.Equal(t, "/api/yucore/media/tasks/yu_result/assets/0", assets[0].Url)
			assert.Equal(t, tt.want.SourceUrl, assets[0].SourceUrl)
			if tt.name == "nested video result aliases" {
				assert.Equal(t, "relative/thumbs/final.jpg", assets[0].SourceThumbUrl)
			}
			if tt.name == "nested content URL and camel case metadata" {
				assert.Equal(t, "/v1/videos/provider-task/thumbnail", assets[0].SourceThumbUrl)
			}
			assert.Equal(t, tt.want.ThumbUrl, assets[0].ThumbUrl)
			assert.Equal(t, tt.want.DurationMs, assets[0].DurationMs)
			assert.Equal(t, tt.want.Width, assets[0].Width)
			assert.Equal(t, tt.want.Height, assets[0].Height)
			assert.Equal(t, tt.want.MimeType, assets[0].MimeType)
			assert.Equal(t, tt.wantStatus, assets[0].Metadata["upstream_status"])
			after, err := common.Marshal(tt.payload)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
			if wantLen == 2 {
				assert.Equal(t, "signed/thumb-one", assets[0].SourceThumbUrl)
				assert.Equal(t, "signed/video-two", assets[1].SourceUrl)
				assert.Equal(t, "signed/thumb-two", assets[1].SourceThumbUrl)
				assert.Equal(t, "/api/yucore/media/tasks/yu_result/assets/1?variant=thumbnail", assets[1].ThumbUrl)
				assert.Equal(t, 2002, assets[1].DurationMs)
				assert.Equal(t, 1280, assets[1].Width)
				assert.Equal(t, 720, assets[1].Height)
				assert.Equal(t, "video/webm", assets[1].MimeType)
			}
		})
	}
}

func TestOpenAICompatibleTaskSerializationKeepsSourcesInternal(t *testing.T) {
	task := &YucoreMediaTask{TaskId: "yu_private_sources", Kind: "video", ModelId: "video-model"}
	payload := map[string]any{"status": "succeeded", "result": map[string]any{
		"thumbnail_url": "https://signed.example/private-thumb.jpg?token=secret-thumb",
		"url":           "https://signed.example/private-video.mp4?token=secret-content",
	}}
	assets := buildOpenAICompatibleTaskAssets(task, payload)
	require.Len(t, assets, 1)
	assets[0].CachedUrl = "https://signed.example/private-cache.mp4?token=secret-cache"
	responseJSON, err := common.Marshal(map[string]any{"assets": assets})
	require.NoError(t, err)
	serialized := string(responseJSON)
	assert.NotContains(t, serialized, "signed.example")
	assert.NotContains(t, serialized, "secret-content")
	assert.NotContains(t, serialized, "secret-thumb")
	assert.NotContains(t, serialized, "secret-cache")
	assert.NotContains(t, serialized, "source_url")
	assert.Contains(t, serialized, `"url":"/api/yucore/media/tasks/yu_private_sources/assets/0"`)
	assert.Contains(t, serialized, `"thumb_url":"/api/yucore/media/tasks/yu_private_sources/assets/0?variant=thumbnail"`)
}

func TestYucoreMediaAssetPrivateSourcesRoundTrip(t *testing.T) {
	assets := []YucoreMediaAsset{{
		Id:             "asset_private",
		Kind:           "video",
		Url:            "/api/yucore/media/tasks/yu_private/assets/0",
		ThumbUrl:       "/api/yucore/media/tasks/yu_private/assets/0?variant=thumbnail",
		SourceUrl:      "https://signed.example/content.mp4?token=content-secret",
		SourceThumbUrl: "https://signed.example/thumb.jpg?token=thumb-secret",
		CachedUrl:      "https://signed.example/cache.mp4?token=cache-secret",
		Label:          "private result",
	}}
	rawAssets, err := marshalYucoreMediaAssets(assets)
	require.NoError(t, err)
	task := &YucoreMediaTask{Assets: YucoreMediaAssets(rawAssets)}
	roundTripped := YucoreMediaTaskAssets(task)
	require.Len(t, roundTripped, 1)
	assert.Equal(t, assets[0], roundTripped[0])
	assert.Equal(t, assets[0].SourceUrl, YucoreMediaAssetSource(roundTripped[0]))
	assert.Equal(t, assets[0].SourceThumbUrl, YucoreMediaAssetThumbnailSource(roundTripped[0]))
	roundTripped[0].SourceThumbUrl = ""
	assert.Equal(t, assets[0].SourceUrl, YucoreMediaAssetThumbnailSource(roundTripped[0]))

	publicJSON, err := common.Marshal(roundTripped)
	require.NoError(t, err)
	assert.NotContains(t, string(publicJSON), "signed.example")
	assert.NotContains(t, string(publicJSON), "secret")
	assert.Contains(t, string(publicJSON), `"thumb_url":"/api/yucore/media/tasks/yu_private/assets/0?variant=thumbnail"`)
}

func TestOpenAICompatibleTaskResultMetadataKeepsNestedURLDistinct(t *testing.T) {
	task := &YucoreMediaTask{TaskId: "yu_nested_urls", Kind: "video", ModelId: "video-model"}
	payload := map[string]any{"status": "succeeded", "result": map[string]any{
		"video_url":   "relative/parent.mp4",
		"duration_ms": 1000,
		"content": map[string]any{
			"url":         "relative/nested.mp4",
			"duration_ms": 2000,
		},
	}}
	assets := buildOpenAICompatibleTaskAssets(task, payload)
	require.Len(t, assets, 2)
	assert.Equal(t, "relative/parent.mp4", assets[0].SourceUrl)
	assert.Equal(t, 1000, assets[0].DurationMs)
	assert.Equal(t, "relative/nested.mp4", assets[1].SourceUrl)
	assert.Equal(t, 2000, assets[1].DurationMs)
}

func TestYucoreMediaRunnableAdapters(t *testing.T) {
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterOpenAICompatible))
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterYuAPIChannel))
	assert.True(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterUAGProxy))
	assert.False(t, isYucoreMediaRunnableAdapter(YucoreMediaAdapterMock))
	assert.False(t, isYucoreMediaRunnableAdapter("unknown"))
}

func TestYucoreMediaConfiguredModelIDs(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_MODEL_CAPABILITIES", "")
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
	require.Len(t, configured, 26)
	assert.Contains(t, configured, "grok-imagine-image")
	assert.Contains(t, configured, "grok-imagine-image-quality")
	assert.Contains(t, configured, "grok-imagine-video")
	assert.Contains(t, configured, "grok-imagine-video-1.5")
	assert.Contains(t, configured, "grok-imagine-video-1.5-preview")
	assert.Contains(t, configured, "gpt-image-2-adobe")
	assert.Contains(t, configured, "gpt-image-2-2k")
	assert.Contains(t, configured, "grok-video")
	assert.NotContains(t, configured, "seedance-2.0-mini-8s")
	assert.NotContains(t, configured, "veo-clean")
	assert.NotContains(t, configured, "grok-imagine-edit")

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

func TestEstimateYucoreMediaTaskCostUsesSelectedGroupRatio(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"managed-image":0.032}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"image-group":1,"premium-media":2}`))

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
		BillingGroup: "premium-media",
		Kind:         "image",
		ModelId:      "managed-image",
		Count:        1,
	})
	assert.Equal(t, 32000, cost)
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

func TestEstimateYucoreMediaTaskCostHonorsExplicitPricingUnit(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalPatches := constant.TaskPricePatches
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"explicit-per-call":0.04,"explicit-per-second":0.04,"fallback-per-call":0.04}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"media-group":1}`))
	constant.TaskPricePatches = []string{"explicit-per-second", "fallback-per-call"}

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":             YucoreMediaAdapterYuAPIChannel,
		"yucore_media.managed_token_group": "media-group",
		"yucore_media.model_capabilities": `{
			"explicit-per-call":{"kind":"video","pricing_unit":"per_call"},
			"explicit-per-second":{"kind":"video","pricing_unit":"per_second"}
		}`,
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

	tests := []struct {
		name  string
		model string
		cost  int
	}{
		{name: "explicit per call overrides duration", model: "explicit-per-call", cost: 20000},
		{name: "explicit per second overrides patch", model: "explicit-per-second", cost: 300000},
		{name: "no explicit unit keeps patch fallback", model: "fallback-per-call", cost: 20000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := estimateYucoreMediaTaskCost(&YucoreMediaTask{
				Kind:     "video",
				ModelId:  tt.model,
				Metadata: `{"duration":15}`,
			})
			assert.Equal(t, tt.cost, cost)
		})
	}
}

func TestEstimateYucoreMediaTaskCostChargesGrokVideoByDuration(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalPatches := constant.TaskPricePatches
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video-1.5-preview":0.65}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"media-group":1.2}`))
	constant.TaskPricePatches = []string{"grok-imagine-video-1.5-preview"}

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
		ModelId:  "grok-imagine-video-1.5-preview",
		Metadata: `{"duration":15}`,
	})
	assert.Equal(t, 5_850_000, cost)
}

func TestEstimateYucoreMediaTaskCostChargesGrokImagineVideoResolution(t *testing.T) {
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video":0.0414}`))
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

	cost := estimateYucoreMediaTaskCost(&YucoreMediaTask{
		Kind:     "video",
		ModelId:  "grok-imagine-video",
		Metadata: `{"duration":10,"resolution":"720p"}`,
	})
	assert.Equal(t, 356400, cost)
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
