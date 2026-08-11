package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func yucoreMediaControllerTestModel(id string) service.YucoreMediaCatalogModel {
	return service.YucoreMediaCatalogModel{
		Id:             id,
		Kind:           service.YucoreMediaKindVideo,
		Modes:          []string{"text-to-video", "image-to-video"},
		Counts:         []int{1},
		Durations:      []int{4, 5, 6, 8, 10, 12, 15},
		Resolutions:    []string{"480p", "720p"},
		AspectRatios:   []string{"16:9", "9:16", "1:1"},
		ReferenceModes: []string{"media", "frames"},
		SupportsAudio:  true,
		SupportsSeed:   true,
		InputLimits: service.YucoreMediaCatalogInputLimits{
			MaxReferenceImages: 4,
			MaxReferenceVideos: 3,
			MaxReferenceAudios: 1,
			MaxReferences:      8,
		},
	}
}

func controllerIntPointer(value int) *int       { return &value }
func controllerInt64Pointer(value int64) *int64 { return &value }
func controllerBoolPointer(value bool) *bool    { return &value }
func controllerStringPointer(value string) *string {
	return &value
}

func mustYucoreMediaJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestBuildYucoreMediaTaskPreservesBillingGroup(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Group:   "multimodal",
		Kind:    "image",
		ModelId: "gpt-image-1",
		Prompt:  "test prompt",
	}, 42)
	require.NoError(t, err)
	assert.Equal(t, "multimodal", task.BillingGroup)

	response := buildYucoreMediaTaskResponse(&model.YucoreMediaTask{
		BillingGroup: "multimodal",
	})
	assert.Equal(t, "multimodal", response.Group)
}

func TestBuildYucoreMediaTaskPreservesOptionalZeroValues(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:          "video",
		ModelId:       "seedance-2.0",
		Prompt:        "test prompt",
		GenerateAudio: controllerBoolPointer(false),
		Seed:          controllerInt64Pointer(0),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))

	omitted, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(omitted, yucoreMediaControllerTestModel("seedance-2.0")))
	metadata = nil
	require.NoError(t, common.Unmarshal([]byte(omitted.Metadata), &metadata))
	_, hasAudio := metadata["generate_audio"]
	_, hasSeed := metadata["seed"]
	assert.False(t, hasAudio)
	assert.False(t, hasSeed)
}

func TestYucoreMediaCanonicalLegacyInputs(t *testing.T) {
	rawInputs := mustYucoreMediaJSON(t, []any{
		" https://cdn.example/string.png ",
		map[string]any{"url": "url", "source_url": "source", "cachedUrl": "cached", "dataUrl": " data "},
		map[string]any{"data_url": "data_snake", "cached_url": "cached_snake"},
		map[string]any{"cached_url": "cached_snake", "sourceUrl": "source_camel"},
		map[string]any{"source_url": "source_snake", "url": "url"},
		map[string]any{"url": "url", "path": "path"},
		map[string]any{"path": "path", "id": "id"},
		map[string]any{"id": "id"},
		map[string]any{"kind": " VIDEO ", "url": "video", "mimeType": "video/mp4", "durationMs": 1200},
		map[string]any{"role": "audio", "kind": "video", "url": "audio", "mime_type": "audio/mpeg", "duration_ms": 2300},
	})
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Inputs:  rawInputs,
	}, 42)
	require.NoError(t, err)

	var references []model.YucoreMediaReferenceInput
	require.NoError(t, common.Unmarshal([]byte(task.Inputs), &references))
	require.Len(t, references, 10)
	assert.Equal(t, []string{"https://cdn.example/string.png", "data", "data_snake", "cached_snake", "source_snake", "url", "path", "id", "video", "audio"}, []string{
		references[0].URL, references[1].URL, references[2].URL, references[3].URL, references[4].URL,
		references[5].URL, references[6].URL, references[7].URL, references[8].URL, references[9].URL,
	})
	assert.Equal(t, "image", references[0].Role)
	assert.Equal(t, "image", references[1].Role)
	assert.Equal(t, "video", strings.ToLower(strings.TrimSpace(references[8].Role)))
	assert.Equal(t, "audio", references[9].Role)
	assert.Equal(t, "video/mp4", references[8].MimeType)
	require.NotNil(t, references[8].DurationMS)
	assert.Equal(t, 1200, *references[8].DurationMS)
	assert.Equal(t, "audio/mpeg", references[9].MimeType)
	require.NotNil(t, references[9].DurationMS)
	assert.Equal(t, 2300, *references[9].DurationMS)
}

func TestYucoreMediaCanonicalTopLevelFieldsOverrideLegacyMetadata(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:           "video",
		ModelId:        "seedance-2.0",
		Prompt:         "test prompt",
		Duration:       controllerIntPointer(5),
		Resolution:     controllerStringPointer("720P"),
		GenerateAudio:  controllerBoolPointer(false),
		Seed:           controllerInt64Pointer(0),
		ReferenceMode:  controllerStringPointer("text"),
		NegativePrompt: controllerStringPointer("top level"),
		Metadata: mustYucoreMediaJSON(t, map[string]any{
			"durationSeconds": 10,
			"resolution":      "480p",
			"generateAudio":   true,
			"seed":            99,
			"referenceMode":   "media",
			"negativePrompt":  "metadata",
			"ui_state":        "keep",
		}),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "5", string(metadata["duration"]))
	assert.JSONEq(t, `"720p"`, string(metadata["resolution"]))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))
	assert.JSONEq(t, `"text"`, string(metadata["reference_mode"]))
	assert.JSONEq(t, `"top level"`, string(metadata["negative_prompt"]))
	assert.JSONEq(t, `"keep"`, string(metadata["ui_state"]))
	assert.Equal(t, "720p", task.Size)
	assert.Equal(t, "top level", task.NegativePrompt)
}

func TestYucoreMediaCanonicalReadsLegacyMetadataWhenTopLevelIsAbsent(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Metadata: mustYucoreMediaJSON(t, map[string]any{
			"duration_seconds": 8,
			"resolution":       "720P",
			"generateAudio":    false,
			"seed":             0,
			"referenceMode":    "text",
			"negativePrompt":   "metadata value",
		}),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "8", string(metadata["duration"]))
	assert.JSONEq(t, `"720p"`, string(metadata["resolution"]))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))
	assert.Equal(t, "metadata value", task.NegativePrompt)
}

func TestYucoreMediaCanonicalRejectsFractionalReferenceDuration(t *testing.T) {
	_, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Inputs:  json.RawMessage(`[{"url":"video","kind":"video","duration_ms":1.5}]`),
	}, 42)
	require.ErrorContains(t, err, "duration_ms")
}

func TestYucoreMediaCanonicalPreservesExistingRequestContracts(t *testing.T) {
	_, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Prompt: "  "}, 42)
	require.ErrorContains(t, err, "prompt is required")

	_, err = buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Prompt: "ok", SessionId: strings.Repeat("x", maxYucoreMediaSessionIdRunes+1)}, 42)
	require.ErrorContains(t, err, "session id is too long")

	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Kind: "video", Prompt: "ok", Count: 2}, 42)
	require.NoError(t, err)
	err = normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0"))
	require.ErrorContains(t, err, "count")

	task.Count = 1
	task.Mode = "text-to-image"
	err = normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0"))
	require.ErrorContains(t, err, "mode")
}
