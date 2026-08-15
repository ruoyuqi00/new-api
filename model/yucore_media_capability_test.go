package model

import (
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCangyuanCatalogMatchesAuditedVideoInventory(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)

	enabled := []string{
		"grok-video", "grok-video-1.5", "happyhouse-1.0", "happyhouse-1.1",
		"minimax-h3-2k", "omni-fast", "omni-fast-no-water", "omni-v2v",
		"omni-v2v-no-water", "sd7-seedance-2.0-1080p", "sd7-seedance-2.0-720p",
		"sd8-seedance-2.0", "seedance-2.0",
	}
	probes := []string{
		"sd4-seedance-2.0", "sd4-seedance-2.0-fast", "sd8-seedance-2.0-fast",
		"seedance-2.0-fast", "seedance-2.0-mini", "seedance-2.0-mini-8s", "veo-clean",
	}
	actualEnabled := make([]string, 0, len(enabled))
	actualProbes := make([]string, 0, len(probes))
	for id, capability := range catalog {
		assert.Equal(t, id, capability.Model)
		if capability.Kind != "video" {
			continue
		}
		if capability.Availability == YucoreMediaAvailabilityProbe {
			actualProbes = append(actualProbes, id)
		} else {
			actualEnabled = append(actualEnabled, id)
		}
	}
	sort.Strings(enabled)
	sort.Strings(probes)
	sort.Strings(actualEnabled)
	sort.Strings(actualProbes)
	assert.Equal(t, enabled, actualEnabled)
	assert.Equal(t, probes, actualProbes)
	for _, stale := range []string{"gemini-omni-flash", "sora-2", "veo-3.1", "sd5-seedance-2.0", "kling-3.0"} {
		assert.NotContains(t, catalog, stale)
	}
}

func TestCangyuanCatalogMatchesAuditedCostsAndCapabilities(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)

	type expectedCapability struct {
		cost               float64
		policy             string
		fixedDuration      int
		durations          []int
		resolutions        []string
		aspectRatios       []string
		referenceModes     []string
		limits             YucoreMediaReferenceLimits
		supportsAudio      bool
		requiredKinds      []string
		disallowFrameAudio bool
		allowed            []string
	}
	range3To15 := []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	range4To15 := []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	range5To15 := []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	grokRatios := []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
	seedanceRatios := []string{"16:9", "9:16", "1:1", "21:9", "3:4", "4:3"}
	expected := map[string]expectedCapability{
		"grok-video":             {cost: 0.69, policy: yucoreMediaDurationPolicyDuration, durations: []int{4, 6, 8, 10, 12, 15}, resolutions: []string{"480p", "720p"}, aspectRatios: grokRatios, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 1, Total: 1}, allowed: []string{"duration", "resolution", "aspect_ratio", "reference_image_urls"}},
		"grok-video-1.5":         {cost: 1.39, policy: yucoreMediaDurationPolicyDuration, durations: []int{4, 6, 8, 10, 12, 15}, resolutions: []string{"480p", "720p"}, aspectRatios: grokRatios, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 7, Total: 7}, allowed: []string{"duration", "resolution", "aspect_ratio", "reference_image_urls"}},
		"happyhouse-1.0":         {cost: 4.5, policy: yucoreMediaDurationPolicyDuration, durations: range3To15, resolutions: []string{"720p", "1080p"}, aspectRatios: []string{"16:9", "9:16", "1:1", "3:4", "4:3"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 9, Videos: 1, Total: 9, MinVideoDurationMS: 3000, MaxVideoDurationMS: 10000, MaxTotalVideoDurationMS: 10000, MaxImagesWithVideo: 5}, supportsAudio: true, allowed: []string{"duration", "resolution", "aspect_ratio", "generate_audio", "reference_image_urls", "reference_videos"}},
		"happyhouse-1.1":         {cost: 2.9, policy: yucoreMediaDurationPolicyDuration, durations: range3To15, resolutions: []string{"720p", "1080p"}, aspectRatios: []string{"16:9", "9:16", "1:1", "3:4", "4:3"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 9, Total: 9}, supportsAudio: true, allowed: []string{"duration", "resolution", "aspect_ratio", "generate_audio", "reference_image_urls"}},
		"minimax-h3-2k":          {cost: 3.5, policy: yucoreMediaDurationPolicyDuration, durations: range5To15, resolutions: []string{"2k"}, aspectRatios: seedanceRatios, referenceModes: []string{"media", "frames"}, limits: YucoreMediaReferenceLimits{Images: 5, Audios: 3, Total: 8, MaxAudioDurationMS: 15000, MaxTotalAudioDurationMS: 15000}, supportsAudio: true, disallowFrameAudio: true, allowed: []string{"duration", "resolution", "aspect_ratio", "generate_audio", "reference_image_urls", "reference_audios", "first_image_url", "last_image_url"}},
		"omni-fast":              {cost: 0.6624, policy: yucoreMediaDurationPolicyFixed, fixedDuration: 10, durations: []int{10}, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16"}, referenceModes: []string{"media", "frames"}, limits: YucoreMediaReferenceLimits{Images: 5, Total: 5}, allowed: []string{"aspect_ratio", "reference_image_urls", "first_image_url", "last_image_url"}},
		"omni-fast-no-water":     {cost: 0.81, policy: yucoreMediaDurationPolicyFixed, fixedDuration: 10, durations: []int{10}, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16"}, referenceModes: []string{"media", "frames"}, limits: YucoreMediaReferenceLimits{Images: 5, Total: 5}, allowed: []string{"aspect_ratio", "reference_image_urls", "first_image_url", "last_image_url"}},
		"omni-v2v":               {cost: 0.8856, policy: yucoreMediaDurationPolicyFixed, fixedDuration: 10, durations: []int{10}, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Videos: 1, Total: 1}, requiredKinds: []string{"video"}, allowed: []string{"aspect_ratio", "reference_videos"}},
		"omni-v2v-no-water":      {cost: 1.035, policy: yucoreMediaDurationPolicyFixed, fixedDuration: 10, durations: []int{10}, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Videos: 1, Total: 1}, requiredKinds: []string{"video"}, allowed: []string{"aspect_ratio", "reference_videos"}},
		"sd7-seedance-2.0-1080p": {cost: 4.9, policy: yucoreMediaDurationPolicyDuration, durations: range4To15, resolutions: []string{"1080p"}, aspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 5, Videos: 3, Audios: 3, Total: 11}, supportsAudio: true, allowed: []string{"duration", "aspect_ratio", "generate_audio", "reference_image_urls", "reference_videos", "reference_audios"}},
		"sd7-seedance-2.0-720p":  {cost: 3.9, policy: yucoreMediaDurationPolicyDuration, durations: range4To15, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 5, Videos: 3, Audios: 3, Total: 11}, supportsAudio: true, allowed: []string{"duration", "aspect_ratio", "generate_audio", "reference_image_urls", "reference_videos", "reference_audios"}},
		"sd8-seedance-2.0":       {cost: 2.9, policy: yucoreMediaDurationPolicyDuration, durations: []int{5, 10, 15}, aspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 9, Videos: 3, Audios: 3, Total: 15}, allowed: []string{"duration", "aspect_ratio", "reference_image_urls", "reference_videos", "reference_audios"}},
		"seedance-2.0":           {cost: 3.9, policy: yucoreMediaDurationPolicyDuration, durations: range4To15, resolutions: []string{"720p"}, aspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, referenceModes: []string{"media"}, limits: YucoreMediaReferenceLimits{Images: 5, Videos: 3, Audios: 3, Total: 11}, supportsAudio: true, allowed: []string{"duration", "aspect_ratio", "generate_audio", "reference_image_urls", "reference_videos", "reference_audios"}},
	}
	totalCost := 0.0
	for modelID, want := range expected {
		capability := catalog[modelID]
		assert.Equal(t, YucoreMediaAvailabilityEnabled, capability.Availability, modelID)
		assert.Equal(t, YucoreMediaPricingPerCall, capability.PricingUnit, modelID)
		assert.InDelta(t, want.cost, capability.UpstreamCost, 0.0000001, modelID)
		assert.Equal(t, want.policy, capability.DurationPolicy, modelID)
		assert.Equal(t, want.fixedDuration, capability.FixedDurationSeconds, modelID)
		assert.Equal(t, want.durations, capability.Durations, modelID)
		assert.Equal(t, want.resolutions, capability.Resolutions, modelID)
		assert.Equal(t, want.aspectRatios, capability.AspectRatios, modelID)
		assert.Equal(t, want.referenceModes, capability.ReferenceModes, modelID)
		assert.Equal(t, want.limits, capability.ReferenceLimits, modelID)
		assert.Equal(t, want.supportsAudio, capability.SupportsAudio, modelID)
		assert.False(t, capability.SupportsSeed, modelID)
		assert.Equal(t, want.requiredKinds, capability.RequiredReferenceKinds, modelID)
		assert.Equal(t, want.disallowFrameAudio, capability.DisallowGeneratedAudioWithFrames, modelID)
		assert.Equal(t, want.allowed, capability.AllowedParameters, modelID)
		assert.Equal(t, "/v1/videos", capability.CreatePath, modelID)
		assert.Equal(t, "/v1/videos/{task_id}", capability.StatusPath, modelID)
		assert.Equal(t, "/v1/videos/{task_id}/content", capability.ContentPath, modelID)
		assert.Equal(t, 5, capability.PollIntervalSeconds, modelID)
		assert.Equal(t, 7200, capability.MaxPollDurationSeconds, modelID)
		totalCost += capability.UpstreamCost
	}
	assert.InDelta(t, 31.973, totalCost, 0.0000001)

	for _, modelID := range []string{
		"sd4-seedance-2.0", "sd4-seedance-2.0-fast", "sd8-seedance-2.0-fast",
		"seedance-2.0-fast", "seedance-2.0-mini", "seedance-2.0-mini-8s", "veo-clean",
	} {
		probe := catalog[modelID]
		assert.Equal(t, YucoreMediaAvailabilityProbe, probe.Availability, modelID)
		assert.Zero(t, probe.UpstreamCost, modelID)
		assert.Empty(t, probe.PricingUnit, modelID)
		assert.Empty(t, probe.Transport, modelID)
		assert.Empty(t, probe.CreatePath, modelID)
		assert.Empty(t, probe.StatusPath, modelID)
		assert.Empty(t, probe.ContentPath, modelID)
		assert.Empty(t, probe.AllowedParameters, modelID)
	}
	assert.Contains(t, catalog["sd8-seedance-2.0"].Notes, "Reference images containing people must mask the eyes.")
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
	arrayMerged, err := mergeYucoreMediaModelCapabilities(embedded, `[{"model":"seedance-2.0","supports_audio":false,"durations":[]}]`)
	require.NoError(t, err)
	assert.False(t, arrayMerged["seedance-2.0"].SupportsAudio)
	assert.Empty(t, arrayMerged["seedance-2.0"].Durations)
	assert.Equal(t, "video", arrayMerged["seedance-2.0"].Kind)
	legacyMerged, err := mergeYucoreMediaModelCapabilities(embedded, `{"seedance-2.0":{"max_reference_images":2}}`)
	require.NoError(t, err)
	assert.Equal(t, 2, legacyMerged["seedance-2.0"].ReferenceLimits.Images)
	assert.Equal(t, 3, legacyMerged["seedance-2.0"].ReferenceLimits.Videos)
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

func TestYucoreMediaConfiguredModelIDsKeepsEmbeddedCatalogForPartialOverride(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":            YucoreMediaAdapterYuAPIChannel,
		"yucore_media.model_capabilities": `{"seedance-2.0":{"poll_interval_seconds":0},"operator-video":{"kind":"video"},"veo-clean":{"availability":" PROBE "}}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	configured := YucoreMediaConfiguredModelIDs()
	assert.Contains(t, configured, "seedance-2.0")
	assert.Contains(t, configured, "gpt-image-2-2k")
	assert.Contains(t, configured, "grok-video")
	assert.Contains(t, configured, "operator-video")
	assert.NotContains(t, configured, "seedance-2.0-mini-8s")
	assert.NotContains(t, configured, "veo-clean")
}

func TestYucoreMediaModelPricingUnitPrefersConfiguredCapability(t *testing.T) {
	originalPatches := constant.TaskPricePatches
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{
			"explicit-per-call":{"kind":"video","pricing_unit":"per_call"},
			"explicit-per-second":{"kind":"video","pricing_unit":"per_second"}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	constant.TaskPricePatches = []string{"explicit-per-second", "fallback-per-call"}
	t.Cleanup(func() {
		constant.TaskPricePatches = originalPatches
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	unit, explicit := YucoreMediaModelPricingUnit(" EXPLICIT-PER-CALL ")
	assert.True(t, explicit)
	assert.Equal(t, YucoreMediaPricingPerCall, unit)
	assert.True(t, YucoreMediaModelUsesPerCallPricing("explicit-per-call"))

	unit, explicit = YucoreMediaModelPricingUnit(" explicit-per-second ")
	assert.True(t, explicit)
	assert.Equal(t, YucoreMediaPricingPerSecond, unit)
	assert.False(t, YucoreMediaModelUsesPerCallPricing("explicit-per-second"))
	assert.True(t, YucoreMediaModelUsesPerCallPricing("fallback-per-call"))
	assert.False(t, YucoreMediaModelUsesPerCallPricing("unconfigured-video"))
}

func TestCangyuanMediaCatalogReturnsIndependentCopies(t *testing.T) {
	first, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	firstCapability := first["seedance-2.0"]
	require.NotEmpty(t, firstCapability.Durations)
	require.NotEmpty(t, firstCapability.ReferenceModes)
	firstCapability.Durations[0] = 999
	firstCapability.ReferenceModes[0] = "mutated"
	firstCapability.RequiredReferenceKinds = []string{"video"}
	firstCapability.ReferenceLimits.Images = 999
	first["seedance-2.0"] = firstCapability
	probeCapability := first["veo-clean"]
	probeCapability.Notes[0] = "mutated"
	first["veo-clean"] = probeCapability
	delete(first, "grok-video")

	second, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)
	assert.Contains(t, second, "grok-video")
	assert.NotEqual(t, 999, second["seedance-2.0"].Durations[0])
	assert.NotEqual(t, "mutated", second["seedance-2.0"].ReferenceModes[0])
	assert.Empty(t, second["seedance-2.0"].RequiredReferenceKinds)
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
	require.NoError(t, validateYucoreMediaModelCapabilities(`{"video":{"kind":"video","reference_limits":{"images":9,"videos":3,"audios":1,"total":12}}}`))
	require.NoError(t, validateYucoreMediaModelCapabilities(`{"video":{"kind":"video","reference_limits":{"images":30,"videos":10,"audios":10}}}`))

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
		{name: "reference video limit too large", raw: `{"video":{"reference_limits":{"videos":33}}}`, wantErr: "reference video limit"},
		{name: "reference audio limit too large", raw: `{"video":{"reference_limits":{"audios":33}}}`, wantErr: "reference audio limit"},
		{name: "negative reference total", raw: `{"video":{"reference_limits":{"total":-1}}}`, wantErr: "reference total limit"},
		{name: "reference total too large", raw: `{"video":{"reference_limits":{"total":33}}}`, wantErr: "reference total limit"},
		{name: "image accepts video references", raw: `{"image":{"kind":"image","reference_limits":{"videos":1}}}`, wantErr: "image model cannot accept video references"},
		{name: "legacy richer conflict", raw: `{"video":{"max_reference_images":2,"reference_limits":{"images":3}}}`, wantErr: "conflicting reference image limits"},
		{name: "duplicate model", raw: `[{"model":"video"},{"model":" video "}]`, wantErr: "duplicate model"},
		{name: "duplicate object model", raw: `{"video":{"kind":"video"},"video":{"kind":"image"}}`, wantErr: "duplicate model"},
		{name: "invalid kind", raw: `{"video":{"kind":"audio"}}`, wantErr: "invalid kind"},
		{name: "unknown reference mode", raw: `{"video":{"reference_modes":["storyboard"]}}`, wantErr: "invalid reference mode"},
		{name: "nonpositive duration", raw: `{"video":{"durations":[0]}}`, wantErr: "invalid duration"},
		{name: "duplicate duration", raw: `{"video":{"durations":[4,4]}}`, wantErr: "duplicate duration"},
		{name: "blank resolution", raw: `{"video":{"resolutions":[" "]}}`, wantErr: "invalid resolution"},
		{name: "duplicate aspect ratio", raw: `{"video":{"aspect_ratios":["16:9"," 16:9 "]}}`, wantErr: "duplicate aspect ratio"},
		{name: "blank allowed parameter", raw: `{"video":{"allowed_parameters":[""]}}`, wantErr: "invalid allowed parameter"},
		{name: "duplicate note", raw: `{"video":{"notes":["Needs audio"," needs audio "]}}`, wantErr: "duplicate note"},
		{name: "reference duration too large", raw: `{"video":{"reference_limits":{"max_video_duration_ms":86400001}}}`, wantErr: "reference duration limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYucoreMediaModelCapabilities(tt.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestYucoreMediaCapabilitiesUseCaseInsensitiveModelIdentity(t *testing.T) {
	tests := []string{
		`[{"model":"Video"},{"model":" video "}]`,
		`{"Video":{}," video ":{}}`,
	}
	for _, raw := range tests {
		_, err := decodeYucoreMediaModelCapabilities([]byte(raw))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate model")
	}

	base := map[string]YucoreMediaModelCapability{
		"seedance-2.0": {Model: "seedance-2.0", Kind: "video", PollIntervalSeconds: 5},
	}
	merged, err := mergeYucoreMediaModelCapabilities(base, `{"SEEDANCE-2.0":{"poll_interval_seconds":7}}`)
	require.NoError(t, err)
	require.Len(t, merged, 1)
	assert.Equal(t, 7, merged["seedance-2.0"].PollIntervalSeconds)
	assert.NotContains(t, merged, "SEEDANCE-2.0")

	merged, err = mergeYucoreMediaModelCapabilities(merged, `{"Custom-Video":{"kind":"video","poll_interval_seconds":3}}`)
	require.NoError(t, err)
	merged, err = mergeYucoreMediaModelCapabilities(merged, `{" custom-video ":{"poll_interval_seconds":9}}`)
	require.NoError(t, err)
	assert.Equal(t, 9, merged["Custom-Video"].PollIntervalSeconds)
	assert.NotContains(t, merged, "custom-video")

	unchanged, err := mergeYucoreMediaModelCapabilities(merged, `{"broken":`)
	require.Error(t, err)
	assert.Equal(t, merged, unchanged)
}

func TestYucoreMediaCheckedConfigRejectsCrossLayerConflict(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_MODEL_CAPABILITIES", `{"Layered-Video":{"kind":"video","max_reference_images":2}}`)
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{"layered-video":{"reference_limits":{"images":3}}}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	config, err := getYucoreMediaAdapterConfigChecked()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting reference image limits")
	assert.Equal(t, 2, config.ModelCapabilities["Layered-Video"].ReferenceLimits.Images)
}

func TestYucoreMediaCapabilityValidationIsDeterministic(t *testing.T) {
	capabilities := map[string]YucoreMediaModelCapability{
		"z-model": {Model: "z-model", Kind: "audio", CreatePath: "also-invalid"},
		"a-model": {Model: "a-model", Kind: "video", CreatePath: "invalid", EditPath: "invalid"},
	}
	for range 50 {
		err := validateYucoreMediaCapabilities(capabilities)
		require.EqualError(t, err, "YuCore media model a-model create path must start with /")
	}
}

func TestYucoreMediaReferenceModeSchema(t *testing.T) {
	require.NoError(t, validateYucoreMediaModelCapabilities(
		`{"video":{"kind":"video","reference_modes":["text","media","frames"],"durations":[]}}`,
	))
	for _, mode := range []string{"frame", " Media "} {
		err := validateYucoreMediaCapabilities(map[string]YucoreMediaModelCapability{
			"video": {Model: "video", Kind: "video", ReferenceModes: []string{mode}},
		})
		require.ErrorContains(t, err, "invalid reference mode")
	}
}

func TestYucoreMediaReferenceModesNormalizeLegacyAliases(t *testing.T) {
	tests := []struct {
		name     string
		modes    string
		expected []string
	}{
		{name: "singular frame", modes: `["frame"]`, expected: []string{"frames"}},
		{name: "image alias", modes: `["image"]`, expected: []string{"media"}},
		{name: "media type aliases", modes: `["image","video","audio"]`, expected: []string{"media"}},
		{name: "mixed canonical and aliases", modes: `["frames","video","text","frame","media","image"]`, expected: []string{"text", "media", "frames"}},
		{name: "whitespace and case", modes: `[" FRAME "," Media ","TEXT","AuDiO"]`, expected: []string{"text", "media", "frames"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"video":{"kind":"video","reference_modes":` + tt.modes + `}}`
			capabilities, err := decodeYucoreMediaModelCapabilities([]byte(raw))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, capabilities["video"].ReferenceModes)
			require.NoError(t, validateYucoreMediaCapabilities(capabilities))
		})
	}
}

func TestYucoreMediaReferenceModeAliasesKeepEnvironmentAndOptionLayersActive(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_MODEL_CAPABILITIES", `{
		"SEEDANCE-2.0": {
			"supports_audio": false,
			"reference_modes": ["image", "video", "audio"]
		}
	}`)
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{
			"seedance-2.0": {
				"durations": [],
				"reference_modes": [" FRAME ", "TeXT", "image", "frames", "MEDIA"]
			}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	config, err := getYucoreMediaAdapterConfigChecked()
	require.NoError(t, err)
	capability := config.ModelCapabilities["seedance-2.0"]
	assert.False(t, capability.SupportsAudio)
	assert.Empty(t, capability.Durations)
	assert.Equal(t, []string{"text", "media", "frames"}, capability.ReferenceModes)
	assert.Equal(t, YucoreMediaReferenceLimits{Images: 5, Videos: 3, Audios: 3, Total: 11}, capability.ReferenceLimits)
}

func TestYucoreMediaReferenceInputRoleIsAlwaysSerialized(t *testing.T) {
	raw, err := common.Marshal(YucoreMediaReferenceInput{URL: "https://cdn.example.com/reference.png"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"role":"","url":"https://cdn.example.com/reference.png"}`, string(raw))
}

func TestParseYucoreMediaModelCapabilitiesNormalizesLegacyReferenceLimit(t *testing.T) {
	capabilities := parseYucoreMediaModelCapabilities(`{"video":{"max_reference_images":5}}`)
	require.Contains(t, capabilities, "video")
	assert.Equal(t, 5, capabilities["video"].ReferenceLimits.Images)
	assert.Equal(t, 5, capabilities["video"].MaxReferenceImages)
}

func TestValidateYucoreMediaCapabilitiesRejectsConditionalReferenceConstraints(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "negative minimum video duration", raw: `{"video":{"reference_limits":{"min_video_duration_ms":-1}}}`, wantErr: "minimum reference video duration"},
		{name: "video minimum exceeds maximum", raw: `{"video":{"reference_limits":{"min_video_duration_ms":10001,"max_video_duration_ms":10000}}}`, wantErr: "reference video duration range"},
		{name: "negative total audio duration", raw: `{"video":{"reference_limits":{"max_total_audio_duration_ms":-1}}}`, wantErr: "total reference audio duration"},
		{name: "conditional images exceed normal maximum", raw: `{"video":{"reference_limits":{"images":5,"max_images_with_video":6}}}`, wantErr: "images with video"},
		{name: "invalid required reference kind", raw: `{"video":{"required_reference_kinds":["document"]}}`, wantErr: "required reference kind"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateYucoreMediaModelCapabilities(test.raw)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
