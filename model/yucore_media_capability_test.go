package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCangyuanMediaCatalogInventory(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	require.Len(t, catalog, 40)

	enabled := 0
	probes := 0
	images := 0
	videos := 0
	for modelID, capability := range catalog {
		assert.Equal(t, modelID, capability.Model)
		switch capability.Availability {
		case YucoreMediaAvailabilityEnabled:
			enabled++
		case YucoreMediaAvailabilityProbe:
			probes++
		default:
			t.Errorf("model %s has unexpected availability %q", modelID, capability.Availability)
		}
		switch capability.Kind {
		case "image":
			images++
		case "video":
			videos++
		default:
			t.Errorf("model %s has unexpected kind %q", modelID, capability.Kind)
		}
	}
	assert.Equal(t, 38, enabled)
	assert.Equal(t, 2, probes)
	assert.Equal(t, 7, images)
	assert.Equal(t, 33, videos)

	assert.Equal(t, YucoreMediaAvailabilityProbe, catalog["seedance-2.0-mini-8s"].Availability)
	assert.Equal(t, YucoreMediaAvailabilityProbe, catalog["veo-clean"].Availability)
	for _, modelID := range []string{"sora-2", "sora-2-pro", "veo-3-1", "veo-3-1-fast", "veo-3-1-ref"} {
		assert.NotContains(t, catalog, modelID)
	}
	assert.Contains(t, catalog, "veo-3.1")
	assert.Contains(t, catalog, "veo-3.1-fast")
	assert.Zero(t, catalog["seedance-2.0-mini-8s"].UpstreamCost)
	assert.Zero(t, catalog["veo-clean"].UpstreamCost)

	expectedCosts := map[string]float64{
		"gpt-image-2-2k": 0.065, "nano-banana-pro-1k": 0.08, "nano-banana-pro-2k": 0.10,
		"nano-banana-pro-4k": 0.149, "nano-banana2-1k": 0.059, "nano-banana2-2k": 0.095, "nano-banana2-4k": 0.135,
		"gemini-omni-flash": 0.75, "grok-video": 0.69, "grok-video-1.5": 1.39, "happyhouse-1.0": 4.5,
		"happyhouse-1.1": 2.9, "kling-3.0": 1.3, "kling-3.0-omni": 1.3, "minimax-h3-2k": 2.5,
		"omni-fast": 0.6624, "omni-fast-no-water": 0.81, "omni-v2v": 0.8856, "omni-v2v-no-water": 1.035,
		"sd5-seedance-2.0": 3.35, "sd5-seedance-2.0-fast": 2.1, "sd6-seedance-2.0-1080p": 0.89,
		"sd6-seedance-2.0-720p": 4.6, "seedance-2.0": 3.9, "seedance-2.0-1080p": 2.25,
		"seedance-2.0-480p": 0.45, "seedance-2.0-4k": 4.5, "seedance-2.0-720p": 0.975,
		"seedance-2.0-fast": 2.9, "seedance-2.0-fast-480p": 0.25, "seedance-2.0-fast-720p": 0.75,
		"seedance-2.0-mini": 2.4, "seedance-2.0-mini-480p": 0.3, "seedance-2.0-mini-720p": 0.525,
		"seedance-2.5-480p": 0.25, "seedance-2.5-720p": 0.35, "veo-3.1": 0.99, "veo-3.1-fast": 0.5,
	}
	require.Len(t, expectedCosts, 38)
	for modelID, expectedCost := range expectedCosts {
		capability := catalog[modelID]
		assert.Equal(t, YucoreMediaAvailabilityEnabled, capability.Availability, modelID)
		assert.InDelta(t, expectedCost, capability.UpstreamCost, 0.0000001, modelID)
	}
}

func TestCangyuanMediaCatalogOperatorOverridesPreserveExplicitZeroValues(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{
			"seedance-2.0": {
				"supports_audio": false,
				"poll_interval_seconds": 0,
				"durations": [],
				"allowed_parameters": []
			}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	capability := getYucoreMediaAdapterConfig().ModelCapabilities["seedance-2.0"]
	assert.False(t, capability.SupportsAudio)
	assert.Zero(t, capability.PollIntervalSeconds)
	assert.Empty(t, capability.Durations)
	assert.Empty(t, capability.AllowedParameters)
	assert.Equal(t, "video", capability.Kind)
	assert.NotEmpty(t, capability.CreatePath)

	embedded, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	arrayMerged := mergeYucoreMediaModelCapabilities(embedded, `[{"model":"seedance-2.0","supports_audio":false,"durations":[]}]`)
	assert.False(t, arrayMerged["seedance-2.0"].SupportsAudio)
	assert.Empty(t, arrayMerged["seedance-2.0"].Durations)
	assert.Equal(t, "video", arrayMerged["seedance-2.0"].Kind)
	legacyMerged := mergeYucoreMediaModelCapabilities(embedded, `{"seedance-2.0":{"max_reference_images":2}}`)
	assert.Equal(t, 2, legacyMerged["seedance-2.0"].ReferenceLimits.Images)
	assert.Equal(t, 1, legacyMerged["seedance-2.0"].ReferenceLimits.Videos)
}

func TestCangyuanMediaCatalogMergesEnvironmentBeforeOptions(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_MODEL_CAPABILITIES", `{"seedance-2.0":{"supports_audio":false}}`)
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{"seedance-2.0":{"durations":[]}}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	capability := getYucoreMediaAdapterConfig().ModelCapabilities["seedance-2.0"]
	assert.False(t, capability.SupportsAudio)
	assert.Empty(t, capability.Durations)
	assert.Equal(t, "video", capability.Kind)
}

func TestCangyuanMediaCatalogReturnsIndependentCopies(t *testing.T) {
	first, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	firstCapability := first["seedance-2.0"]
	require.NotEmpty(t, firstCapability.Durations)
	require.NotEmpty(t, firstCapability.ReferenceModes)
	firstCapability.Durations[0] = 999
	firstCapability.ReferenceModes[0] = "mutated"
	firstCapability.ReferenceLimits.Images = 999
	first["seedance-2.0"] = firstCapability
	probeCapability := first["veo-clean"]
	probeCapability.Notes[0] = "mutated"
	first["veo-clean"] = probeCapability
	delete(first, "veo-3.1")

	second, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	assert.Contains(t, second, "veo-3.1")
	assert.NotEqual(t, 999, second["seedance-2.0"].Durations[0])
	assert.NotEqual(t, "mutated", second["seedance-2.0"].ReferenceModes[0])
	assert.NotEqual(t, 999, second["seedance-2.0"].ReferenceLimits.Images)
	assert.NotEqual(t, "mutated", second["veo-clean"].Notes[0])

	_, settings := GetYucoreMediaCatalogSettings()
	settingsCapability := settings["seedance-2.0"]
	settingsCapability.Resolutions[0] = "mutated"
	settingsCapability.TerminalFailureStates[0] = "mutated"
	settings["seedance-2.0"] = settingsCapability
	_, freshSettings := GetYucoreMediaCatalogSettings()
	assert.NotEqual(t, "mutated", freshSettings["seedance-2.0"].Resolutions[0])
	assert.NotEqual(t, "mutated", freshSettings["seedance-2.0"].TerminalFailureStates[0])
}

func TestValidateYucoreMediaModelCapabilitiesRichSchema(t *testing.T) {
	valid := `[{"model":"video","kind":"video","availability":"enabled","pricing_unit":"per_second","transport":"async-task","create_path":"/v1/videos","status_path":"/v1/videos/{task_id}","content_path":"/v1/videos/{task_id}/content","cancel_path":"/v1/videos/{task_id}/cancel","poll_interval_seconds":5,"max_poll_duration_seconds":900,"reference_limits":{"images":2,"videos":1,"audios":1,"total":4,"max_video_duration_ms":30000,"max_audio_duration_ms":30000},"terminal_success_states":["completed"],"terminal_failure_states":["failed","canceled"]}]`
	require.NoError(t, validateYucoreMediaModelCapabilities(valid))

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "invalid create path", raw: `{"video":{"create_path":"v1/videos"}}`, wantErr: "create path"},
		{name: "invalid content template", raw: `{"video":{"content_path":"/v1/videos/content"}}`, wantErr: "content path must contain {task_id}"},
		{name: "invalid cancel template", raw: `{"video":{"cancel_path":"/v1/videos/cancel"}}`, wantErr: "cancel path must contain {task_id}"},
		{name: "invalid pricing unit", raw: `{"video":{"pricing_unit":"per_minute"}}`, wantErr: "invalid pricing unit"},
		{name: "negative poll interval", raw: `{"video":{"poll_interval_seconds":-1}}`, wantErr: "invalid poll interval"},
		{name: "poll interval too large", raw: `{"video":{"poll_interval_seconds":3601}}`, wantErr: "invalid poll interval"},
		{name: "poll duration shorter than interval", raw: `{"video":{"poll_interval_seconds":30,"max_poll_duration_seconds":10}}`, wantErr: "maximum poll duration"},
		{name: "blank terminal success state", raw: `{"video":{"terminal_success_states":[""]}}`, wantErr: "terminal success states"},
		{name: "duplicate terminal success state", raw: `{"video":{"terminal_success_states":["done","DONE"]}}`, wantErr: "duplicate terminal success state"},
		{name: "overlapping terminal states", raw: `{"video":{"terminal_success_states":["done"],"terminal_failure_states":["DONE"]}}`, wantErr: "terminal states overlap"},
		{name: "negative reference limit", raw: `{"video":{"reference_limits":{"images":-1}}}`, wantErr: "reference image limit"},
		{name: "reference total below parts", raw: `{"video":{"reference_limits":{"images":2,"videos":1,"total":2}}}`, wantErr: "reference total limit"},
		{name: "image accepts video references", raw: `{"image":{"kind":"image","reference_limits":{"videos":1}}}`, wantErr: "image model cannot accept video references"},
		{name: "legacy richer conflict", raw: `{"video":{"max_reference_images":2,"reference_limits":{"images":3}}}`, wantErr: "conflicting reference image limits"},
		{name: "duplicate model", raw: `[{"model":"video"},{"model":" video "}]`, wantErr: "duplicate model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYucoreMediaModelCapabilities(tt.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseYucoreMediaModelCapabilitiesNormalizesLegacyReferenceLimit(t *testing.T) {
	capabilities := parseYucoreMediaModelCapabilities(`{"video":{"max_reference_images":5}}`)
	require.Contains(t, capabilities, "video")
	assert.Equal(t, 5, capabilities["video"].ReferenceLimits.Images)
	assert.Equal(t, 5, capabilities["video"].MaxReferenceImages)
}
