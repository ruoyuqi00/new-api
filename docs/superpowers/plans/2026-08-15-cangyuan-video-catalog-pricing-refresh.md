# Cangyuan Video Catalog and Pricing Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the stale production Cangyuan video inventory with the 14 currently visible and priced VIDEO-group models, enforce their observed request contracts, publish the exact 20%-markup prices, and prepare a verified no-stop production release.

**Architecture:** Keep the existing YuCore capability catalog and asynchronous task state machine. Extend only the reusable reference-constraint vocabulary that the current upstream contract needs, generate provider payloads from explicit allowed fields, and let normal YuAPI channels continue to own routing and billing. Production channel/price changes remain runtime configuration and are applied only after local and private-candidate verification; the old container and legacy channel rows remain available for rollback.

**Tech Stack:** Go 1.22+, Gin, GORM, testify, embedded JSON capability catalog, React 19, TypeScript, Bun/Rsbuild, i18next, Docker, Caddy graceful reload, MySQL/Redis production runtime.

---

## File map

- `model/yucore_media_capability.go`: reusable capability and reference-limit schema, cloning, normalization, and validation.
- `model/yucore_media_cangyuan_catalog.json`: current Cangyuan image rows plus the exact 18 visible video rows (14 enabled, 4 probe).
- `model/yucore_media_capability_test.go`: embedded inventory, pricing-unit, cost, and capability regression contracts.
- `model/yucore_media_openai_compatible.go`: exact JSON field mapping for references, duration, resolution, audio, and upstream model IDs.
- `model/yucore_media_openai_compatible_test.go`: public-to-upstream payload and async same-ID task behavior.
- `service/yucore_media_catalog.go`: project reusable constraints and video resolutions to the authenticated Studio catalog.
- `service/yucore_media_catalog_test.go`: catalog visibility, pricing unit, price multiplication, and projection contracts.
- `service/yucore_media_request.go`: reject invalid conditional reference/audio combinations before any upstream write.
- `service/yucore_media_request_test.go`: reference duration totals, required references, frame/audio exclusion, and conditional image limits.
- `web/default/src/features/yucore-brand/api/studio.ts`: type the capability fields already returned by the backend.
- `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`: expose video resolution and generated-audio controls and submit only supported values.
- `web/default/src/features/yucore-brand/i18n/use-yucore-translation.ts`: translations for the narrowly added Studio labels.
- `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: synchronized user-facing translations required by project policy.
- `web/default/public/developer-docs/yucore-api.md`: replace the stale video model, price, parameter, and example sections.
- `docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md`: non-secret target/rollback configuration and hot-switch checklist.
- `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`: redacted automated, paid-task, candidate, cutover, and rollback evidence.

### Task 1: Extend reusable media reference constraints

**Files:**
- Modify: `model/yucore_media_capability.go`
- Modify: `model/yucore_media_capability_test.go`
- Modify: `service/yucore_media_catalog.go`
- Modify: `service/yucore_media_catalog_test.go`
- Modify: `service/yucore_media_request.go`
- Modify: `service/yucore_media_request_test.go`

- [ ] **Step 1: Write failing schema validation tests**

Add table cases that reject negative/min-max-inverted reference durations,
invalid required reference kinds, a conditional image maximum greater than the
normal image maximum, and negative total-duration limits:

```go
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
```

- [ ] **Step 2: Run the schema test and verify it fails**

Run:

```powershell
go test ./model -run TestValidateYucoreMediaCapabilitiesRejectsConditionalReferenceConstraints -count=1
```

Expected: FAIL because the new JSON fields are not validated.

- [ ] **Step 3: Add the capability fields and validation**

Extend the existing structures without changing current JSON names. Add the
duration and mixed-reference fields to the existing
`YucoreMediaReferenceLimits` type:

```go
MinVideoDurationMS      int `json:"min_video_duration_ms,omitempty"`
MaxTotalVideoDurationMS int `json:"max_total_video_duration_ms,omitempty"`
MaxTotalAudioDurationMS int `json:"max_total_audio_duration_ms,omitempty"`
MaxImagesWithVideo      int `json:"max_images_with_video,omitempty"`
```

Add these fields to the existing `YucoreMediaModelCapability` type:

```go
RequiredReferenceKinds           []string `json:"required_reference_kinds,omitempty"`
DisallowGeneratedAudioWithFrames bool     `json:"disallow_generated_audio_with_frames,omitempty"`
RequirePrimaryImageForMedia      bool     `json:"require_primary_image_for_media,omitempty"`
```

Clone `RequiredReferenceKinds`, accept only `image`, `video`, or `audio`, require
all duration limits to be nonnegative, require minimum video duration not to
exceed the per-video maximum, and require `MaxImagesWithVideo <= Images` when
both are set. Preserve all existing validation limits and error behavior.

- [ ] **Step 4: Run schema tests and verify they pass**

Run:

```powershell
go test ./model -run 'TestValidateYucoreMediaCapabilities|TestLoadCangyuan' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing request-normalization tests**

Add deterministic cases for the new behavior:

```go
func TestNormalizeYucoreMediaRequestEnforcesObservedReferenceConstraints(t *testing.T) {
	selected := yucoreMediaRequestTestModel("conditional-video")
	selected.InputLimits.MinReferenceVideoDurationMS = 3000
	selected.InputLimits.MaxReferenceVideoDurationMS = 10000
	selected.InputLimits.MaxTotalVideoDurationMS = 10000
	selected.InputLimits.MaxReferenceAudioDurationMS = 15000
	selected.InputLimits.MaxTotalAudioDurationMS = 15000
	selected.InputLimits.MaxImagesWithVideo = 1
	selected.RequiredReferenceKinds = []string{"video"}

	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
	require.ErrorContains(t, err, "requires a video reference")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example/a.png"},
		{Role: "image", URL: "https://cdn.example/b.png"},
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(5000)},
	}})
	require.ErrorContains(t, err, "at most 1 reference image")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(2000)},
	}})
	require.ErrorContains(t, err, "at least 3000 ms")
}

func TestNormalizeYucoreMediaRequestRejectsGeneratedAudioWithFrames(t *testing.T) {
	selected := yucoreMediaRequestTestModel("frame-video")
	selected.DisallowGeneratedAudioWithFrames = true
	generateAudio := true
	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
		GenerateAudio: &generateAudio,
		References: []model.YucoreMediaReferenceInput{
			{Role: "first_frame", URL: "https://cdn.example/first.png"},
			{Role: "last_frame", URL: "https://cdn.example/last.png"},
		},
	})
	require.ErrorContains(t, err, "generated audio with frame references")
}
```

- [ ] **Step 6: Run request tests and verify they fail**

Run:

```powershell
go test ./service -run 'TestNormalizeYucoreMediaRequestEnforcesObservedReferenceConstraints|TestNormalizeYucoreMediaRequestRejectsGeneratedAudioWithFrames' -count=1
```

Expected: compile or assertion failure because catalog projection and request
validation do not expose the new constraints.

- [ ] **Step 7: Project and enforce the constraints**

Add these matching fields to the existing `YucoreMediaCatalogInputLimits`:

```go
MinReferenceVideoDurationMS      int `json:"min_reference_video_duration_ms,omitempty"`
MaxTotalReferenceVideoDurationMS int `json:"max_total_reference_video_duration_ms,omitempty"`
MaxTotalReferenceAudioDurationMS int `json:"max_total_reference_audio_duration_ms,omitempty"`
MaxImagesWithVideo               int `json:"max_images_with_video,omitempty"`
```

Add these fields to the existing `YucoreMediaCatalogModel`:

```go
RequiredReferenceKinds           []string `json:"required_reference_kinds,omitempty"`
DisallowGeneratedAudioWithFrames bool     `json:"disallow_generated_audio_with_frames,omitempty"`
RequirePrimaryImageForMedia      bool     `json:"require_primary_image_for_media,omitempty"`
```

Project values from the capability record. In `NormalizeYucoreMediaRequest`,
track video/audio duration totals, apply per-item minimum/maximum and total
maximums when duration metadata is present, apply `MaxImagesWithVideo` when a
video reference exists, enforce `RequiredReferenceKinds`, and gate the old
primary-image rule on `RequirePrimaryImageForMedia` instead of inferring it from
allowed parameter names.

- [ ] **Step 8: Run focused model/service tests**

Run:

```powershell
go test ./model ./service -run 'YucoreMedia(Capabilit|Catalog|Request)' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit the reusable constraint change**

```powershell
git add model/yucore_media_capability.go model/yucore_media_capability_test.go service/yucore_media_catalog.go service/yucore_media_catalog_test.go service/yucore_media_request.go service/yucore_media_request_test.go
git commit -m "feat: validate media reference constraints"
```

### Task 2: Replace the embedded Cangyuan video inventory

**Files:**
- Modify: `model/yucore_media_cangyuan_catalog.json`
- Modify: `model/yucore_media_capability_test.go`
- Modify: `service/yucore_media_catalog_test.go`

- [ ] **Step 1: Write the failing exact-inventory test**

Replace stale 33-model assumptions with an exact enabled/probe contract:

```go
func TestCangyuanCatalogMatchesAuditedVideoInventory(t *testing.T) {
	catalog, err := loadCangyuanMediaCatalog()
	require.NoError(t, err)

	enabled := []string{
		"grok-video", "grok-video-1.5", "happyhouse-1.0", "happyhouse-1.1",
		"minimax-h3-2k", "omni-fast", "omni-fast-no-water", "omni-v2v",
		"omni-v2v-no-water", "sd4-seedance-2.0", "sd4-seedance-2.0-fast",
		"sd7-seedance-2.0-1080p", "sd7-seedance-2.0-720p", "sd8-seedance-2.0",
	}
	probes := []string{
		"sd8-seedance-2.0-fast", "seedance-2.0-mini",
		"seedance-2.0-mini-8s", "veo-clean",
	}

	actualEnabled := make([]string, 0)
	actualProbes := make([]string, 0)
	for id, capability := range catalog {
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
	for _, stale := range []string{"sora-2", "veo-3-1", "sd5-seedance-2.0", "seedance-2.0", "kling-3.0"} {
		assert.NotContains(t, catalog, stale)
	}
}
```

- [ ] **Step 2: Run the inventory test and verify it fails**

Run:

```powershell
go test ./model -run TestCangyuanCatalogMatchesAuditedVideoInventory -count=1
```

Expected: FAIL with stale and missing model differences.

- [ ] **Step 3: Replace only the video rows in the embedded catalog**

Keep the seven current image rows byte-for-byte unless formatting requires a
trailing comma change. Encode the video rows from this exact source table:

```text
model|availability|cost|duration|resolution|ratios|images|videos|audios|total|audio|frames
grok-video|enabled|0.69|4,6,8,10,12,15|480p,720p|1:1,16:9,9:16,4:3,3:4,3:2,2:3|1|0|0|1|false|false
grok-video-1.5|enabled|1.39|4,6,8,10,12,15|480p,720p|1:1,16:9,9:16,4:3,3:4,3:2,2:3|7|0|0|7|false|false
happyhouse-1.0|enabled|4.5|3,4,5,6,7,8,9,10,11,12,13,14,15|720p,1080p|16:9,9:16,1:1,3:4,4:3|9|1|0|9|true|false
happyhouse-1.1|enabled|2.9|3,4,5,6,7,8,9,10,11,12,13,14,15|720p,1080p|16:9,9:16,1:1,3:4,4:3|9|0|0|9|true|false
minimax-h3-2k|enabled|3.5|5,6,7,8,9,10,11,12,13,14,15|2k|16:9,9:16,1:1,21:9,3:4,4:3|5|0|3|8|true|true
omni-fast|enabled|0.6624|fixed:10|720p|16:9,9:16|5|0|0|5|false|true
omni-fast-no-water|enabled|0.81|fixed:10|720p|16:9,9:16|5|0|0|5|false|true
omni-v2v|enabled|0.8856|fixed:10|720p|16:9,9:16|0|1|0|1|false|false
omni-v2v-no-water|enabled|1.035|fixed:10|720p|16:9,9:16|0|1|0|1|false|false
sd4-seedance-2.0|enabled|3.9|4,5,6,7,8,9,10,11,12,13,14,15|480p,720p|16:9,9:16,1:1,21:9,3:4,4:3|4|3|1|8|true|true
sd4-seedance-2.0-fast|enabled|2.9|4,5,6,7,8,9,10,11,12,13,14,15|480p,720p|16:9,9:16,1:1,21:9,3:4,4:3|4|3|1|8|true|true
sd7-seedance-2.0-1080p|enabled|4.9|4,5,6,7,8,9,10,11,12,13,14,15|1080p|16:9,9:16,1:1,4:3,3:4,21:9|5|3|3|11|true|false
sd7-seedance-2.0-720p|enabled|3.9|4,5,6,7,8,9,10,11,12,13,14,15|720p|16:9,9:16,1:1,4:3,3:4,21:9|5|3|3|11|true|false
sd8-seedance-2.0|enabled|2.9|5,10,15|none|16:9,9:16,1:1,4:3,3:4|9|3|3|15|false|false
sd8-seedance-2.0-fast|probe|unknown|none|none|none|0|0|0|0|false|false
seedance-2.0-mini|probe|unknown|none|none|none|0|0|0|0|false|false
seedance-2.0-mini-8s|probe|unknown|none|none|none|0|0|0|0|false|false
veo-clean|probe|unknown|none|none|none|0|0|0|0|false|false
```

All enabled rows use `pricing_unit: per_call`, async `/v1/videos` create/status/
content paths, a five-second poll interval, a two-hour maximum poll duration,
`duration_policy: duration` except fixed Omni, and only documented allowed
parameters. Add `required_reference_kinds:["video"]` to both Omni V2V rows,
`max_images_with_video:5` plus 3-10 second video limits to Happyhouse 1.0,
15-second total audio plus frame/audio exclusion to Minimax, and the documented
video/audio totals to SD4. Record the SD8 eye-mask requirement in `notes`; do not
pretend it can be inferred from pixels by the gateway.

- [ ] **Step 4: Add exact cost and capability assertions**

Use a table test with exact expected `UpstreamCost`, duration policy, reference
limits, and allowed parameters for all 14 enabled rows. Assert every enabled row
uses `per_call`, every probe has no cost/transport fields, and all target video
rows use `/v1/videos` lifecycle paths.

- [ ] **Step 5: Run catalog tests**

Run:

```powershell
go test ./model ./service -run 'CangyuanCatalog|BuildYucoreMediaCatalog' -count=1
```

Expected: PASS with 14 enabled and 4 hidden probes.

- [ ] **Step 6: Commit the catalog refresh**

```powershell
git add model/yucore_media_cangyuan_catalog.json model/yucore_media_capability_test.go service/yucore_media_catalog_test.go
git commit -m "feat: refresh cangyuan video catalog"
```

### Task 3: Map canonical requests to the observed upstream JSON contract

**Files:**
- Modify: `model/yucore_media_openai_compatible.go`
- Modify: `model/yucore_media_openai_compatible_test.go`

- [ ] **Step 1: Replace stale payload tests with failing target-family tests**

Add table-driven cases that assert:

```go
tests := []struct {
	model      string
	wantModel  string
	wantFields map[string]any
	absent     []string
}{
	{model: "grok-video", wantModel: "grok-video", wantFields: map[string]any{"duration": 4, "resolution": "480p", "reference_image_urls": []string{"https://cdn.example/ref.png"}}, absent: []string{"seconds", "size", "image_urls"}},
	{model: "happyhouse-1.0", wantModel: "happyhouse-1.0", wantFields: map[string]any{"duration": 3, "resolution": "720p", "generate_audio": false, "reference_image_urls": []string{"https://cdn.example/ref.png"}, "reference_videos": []string{"https://cdn.example/ref.mp4"}}, absent: []string{"seconds", "audio", "image_urls"}},
	{model: "minimax-h3-2k", wantModel: "minimax-h3-2k", wantFields: map[string]any{"duration": 5, "resolution": "2k", "reference_image_urls": []string{"https://cdn.example/ref.png"}, "reference_audios": []string{"https://cdn.example/ref.mp3"}}, absent: []string{"seconds", "image_url"}},
	{model: "sd4-seedance-2.0", wantModel: "sd4-seedance-2.0", wantFields: map[string]any{"duration": 4, "resolution": "480p", "generate_audio": false, "reference_image_urls": []string{"https://cdn.example/ref.png"}}, absent: []string{"seconds", "audio", "image_url"}},
	{model: "sd7-seedance-2.0-720p", wantModel: "sd7-seedance-2.0-720p", wantFields: map[string]any{"duration": 4, "reference_image_urls": []string{"https://cdn.example/ref.png"}}, absent: []string{"seconds", "resolution", "size"}},
	{model: "sd8-seedance-2.0", wantModel: "sd8-seedance-2.0", wantFields: map[string]any{"duration": 5, "reference_image_urls": []string{"https://cdn.example/ref.png"}}, absent: []string{"seconds", "resolution", "generate_audio", "audio"}},
}
```

Keep separate tests proving Omni omits duration/resolution and uses
`reference_image_urls`, frame fields, or `reference_videos` exactly as documented.
Continue asserting explicit `false` survives for generated-audio models and
unknown metadata never reaches the provider.

- [ ] **Step 2: Run the payload tests and verify they fail**

Run:

```powershell
go test ./model -run 'TestBuildOpenAICompatibleAsyncPayload(Cangyuan|Omni)' -count=1
```

Expected: FAIL because the old builder emits `seconds`, `size`, `image_url`, or
`image_urls` for these families.

- [ ] **Step 3: Make reference-field selection explicit**

Within `buildOpenAICompatibleAsyncPayload`, select a field only when it appears
in `AllowedParameters`. Use this deterministic order:

```go
if hasFrameReferences {
	if len(firstFrames) > 0 && allowsParameter("first_image_url") {
		payload["first_image_url"] = firstFrames[0]
	}
	if len(lastFrames) > 0 && allowsParameter("last_image_url") {
		payload["last_image_url"] = lastFrames[0]
	}
} else {
	switch {
	case len(images) > 0 && allowsParameter("reference_image_urls"):
		payload["reference_image_urls"] = images
	case len(images) > 0 && allowsParameter("image_urls"):
		payload["image_urls"] = images
	case len(images) == 1 && allowsParameter("image_url"):
		payload["image_url"] = images[0]
	case len(images) == 1 && allowsParameter("image"):
		payload["image"] = images[0]
	case len(images) > 1 && allowsParameter("images"):
		payload["images"] = images
	}
	if len(videos) > 0 && allowsParameter("reference_videos") {
		payload["reference_videos"] = videos
	} else if len(videos) > 0 && allowsParameter("video_url") {
		payload["video_url"] = videos[0]
	}
	if len(audios) > 0 && allowsParameter("reference_audios") {
		payload["reference_audios"] = audios
	}
}
```

Use `DurationPolicy=duration` to emit an integer `duration`; retain fixed Omni
omission. Continue using `resolution` when allowed and `size` only when the
capability explicitly requests it. Do not hard-code provider aliases in source;
real paid probes decide runtime channel `model_mapping`.

- [ ] **Step 4: Run the full media adapter tests**

Run:

```powershell
go test ./model -run 'OpenAICompatible|YucoreMedia' -count=1
```

Expected: PASS, including one-POST/same-ID polling and billing tests.

- [ ] **Step 5: Commit the request mapper**

```powershell
git add model/yucore_media_openai_compatible.go model/yucore_media_openai_compatible_test.go
git commit -m "fix: map current cangyuan video payloads"
```

### Task 4: Project video controls into YuCore Studio

**Files:**
- Modify: `service/yucore_media_catalog.go`
- Modify: `service/yucore_media_catalog_test.go`
- Modify: `web/default/src/features/yucore-brand/api/studio.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`
- Modify: `web/default/src/features/yucore-brand/i18n/use-yucore-translation.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Write a failing catalog projection test**

Assert that a configured video capability with `Resolutions:["480p","720p"]`
produces both `resolutions` and `sizes` with the same ordered values, exposes
`supports_audio`, and preserves reference constraints. This protects the
existing Studio control contract, which reads `sizes`.

- [ ] **Step 2: Run the projection test and verify it fails**

Run:

```powershell
go test ./service -run TestBuildYucoreMediaCatalogProjectsCurrentVideoControls -count=1
```

Expected: FAIL because video `Sizes` is currently empty.

- [ ] **Step 3: Project video resolutions to the existing size control**

In `buildYucoreMediaCatalogModel`, keep `Resolutions` and add:

```go
if kind == YucoreMediaKindVideo {
	item.Sizes = append([]string(nil), capability.Resolutions...)
}
```

Do not set a resolution for SD7/SD8 payloads when `resolution` is absent from
their allowlist; the selected value is display metadata for fixed-resolution
model IDs and the adapter still omits it upstream.

- [ ] **Step 4: Type and submit generated-audio support**

Extend `YucoreMediaModel` with the already-returned fields:

```ts
reference_modes?: string[]
supports_audio?: boolean
supports_seed?: boolean
resolutions?: string[]
```

Add `const [generateAudio, setGenerateAudio] = useState(true)`, reset it only
when switching to a model that does not support audio, render an accessible
checkbox/toggle only for video models with `supports_audio`, and submit:

```ts
...(currentModel.supports_audio ? { generate_audio: generateAudio } : {})
```

inside task metadata. Use `t('Generate native audio')` and
`t('Include model-generated audio in the video.')`; add these translations
through the project i18n workflow:

| locale | `Generate native audio` | `Include model-generated audio in the video.` |
|---|---|---|
| en | Generate native audio | Include model-generated audio in the video. |
| zh | 生成原生音频 | 在视频中包含模型生成的音频。 |
| fr | Generer l'audio natif | Inclure l'audio genere par le modele dans la video. |
| ja | ネイティブ音声を生成 | モデルが生成した音声を動画に含めます。 |
| ru | Создавать встроенный звук | Включать в видео звук, созданный моделью. |
| vi | Tao am thanh goc | Bao gom am thanh do mo hinh tao trong video. |

- [ ] **Step 5: Run focused backend and frontend checks**

Run:

```powershell
go test ./service -run 'BuildYucoreMediaCatalog|NormalizeYucoreMediaRequest' -count=1
Set-Location web/default
bun run i18n:sync
bun run typecheck
bun run lint
Set-Location ../..
```

Expected: all commands exit 0; locale files contain no missing new keys.

- [ ] **Step 6: Commit Studio control projection**

```powershell
git add service/yucore_media_catalog.go service/yucore_media_catalog_test.go web/default/src/features/yucore-brand/api/studio.ts web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx web/default/src/features/yucore-brand/i18n/use-yucore-translation.ts web/default/src/i18n/locales
git commit -m "feat: expose current video controls in studio"
```

### Task 5: Replace stale video developer documentation

**Files:**
- Modify: `web/default/public/developer-docs/yucore-api.md`
- Create: `docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md`

- [ ] **Step 1: Replace the video price table exactly**

Publish the 14 `多模态创作` final prices from the approved design:

```text
grok-video=0.9936
grok-video-1.5=2.0016
happyhouse-1.0=6.48
happyhouse-1.1=4.176
minimax-h3-2k=5.04
omni-fast=0.95388
omni-fast-no-water=1.1664
omni-v2v=1.27536
omni-v2v-no-water=1.4904
sd4-seedance-2.0=5.616
sd4-seedance-2.0-fast=4.176
sd7-seedance-2.0-1080p=7.056
sd7-seedance-2.0-720p=5.616
sd8-seedance-2.0=4.176
```

State that every listed model is per generation, the `下游多模态` amount is the
base price shown by the pricing API for that group, and poll/content/download do
not charge again.

- [ ] **Step 2: Replace examples and parameter tables**

Remove Sora, old Veo, SD5, old Seedance, Mini, and duration-based Grok examples.
Add valid JSON examples for Grok, Happyhouse, Minimax, Omni image/video inputs,
SD4 frames/multimodal references, fixed-resolution SD7, and SD8. Every example
must use the public model ID, `duration` rather than `seconds`, documented
`reference_*` fields, environment placeholders, and non-sensitive example URLs.

- [ ] **Step 3: Write the non-secret production runbook**

Include the exact five target family channel lists, 14 base prices, the exact
comma-separated `TASK_PRICE_PATCH` list, expected group ratios, alias-preservation
rule, two-reference Caddy validation/reload, scoped configuration backup/readback,
and reverse-order rollback. Use internal aliases and omit provider/server/account
identifiers and credentials.

The exact fixed-price environment value is:

```text
grok-video,grok-video-1.5,happyhouse-1.0,happyhouse-1.1,minimax-h3-2k,omni-fast,omni-fast-no-water,omni-v2v,omni-v2v-no-water,sd4-seedance-2.0,sd4-seedance-2.0-fast,sd7-seedance-2.0-1080p,sd7-seedance-2.0-720p,sd8-seedance-2.0
```

The staged family channels and base-price values are:

```text
cangyuan-video-refresh-20260815-omni=omni-fast,omni-fast-no-water,omni-v2v,omni-v2v-no-water
cangyuan-video-refresh-20260815-grok=grok-video,grok-video-1.5
cangyuan-video-refresh-20260815-happyhouse=happyhouse-1.0,happyhouse-1.1
cangyuan-video-refresh-20260815-minimax=minimax-h3-2k
cangyuan-video-refresh-20260815-seedance=sd4-seedance-2.0,sd4-seedance-2.0-fast,sd7-seedance-2.0-1080p,sd7-seedance-2.0-720p,sd8-seedance-2.0

grok-video=0.828
grok-video-1.5=1.668
happyhouse-1.0=5.4
happyhouse-1.1=3.48
minimax-h3-2k=4.2
omni-fast=0.7949
omni-fast-no-water=0.972
omni-v2v=1.0628
omni-v2v-no-water=1.242
sd4-seedance-2.0=4.68
sd4-seedance-2.0-fast=3.48
sd7-seedance-2.0-1080p=5.88
sd7-seedance-2.0-720p=4.68
sd8-seedance-2.0=3.48
```

- [ ] **Step 4: Scan documentation for stale models and secrets**

Run:

```powershell
rg -n 'sora-2|veo-3-1|sd5-seedance|seedance-2\.0-fast|grok-imagine-video-1\.5-preview' web/default/public/developer-docs/yucore-api.md docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md
$privatePatterns = Get-Content -LiteralPath $env:YUAPI_PRIVATE_PATTERN_FILE
Select-String -Path web/default/public/developer-docs/yucore-api.md,docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md -Pattern $privatePatterns
```

Expected: no matches. The operator-local pattern file is outside Git and
contains the private provider/server/account markers that must not be committed.

- [ ] **Step 5: Commit documentation and runbook**

```powershell
git add web/default/public/developer-docs/yucore-api.md docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md
git commit -m "docs: publish current video models and prices"
```

### Task 6: Full local verification and visual acceptance

**Files:**
- Create: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Run formatting and focused tests**

```powershell
gofmt -w model/yucore_media_capability.go model/yucore_media_capability_test.go model/yucore_media_openai_compatible.go model/yucore_media_openai_compatible_test.go service/yucore_media_catalog.go service/yucore_media_catalog_test.go service/yucore_media_request.go service/yucore_media_request_test.go
go test ./model ./service ./controller ./relay ./constant -count=1
```

Expected: PASS.

- [ ] **Step 2: Run all backend tests**

Ensure both ignored frontend `dist` directories exist, then run:

```powershell
go test ./...
```

Expected: PASS with no failing package.

- [ ] **Step 3: Run both production frontend builds**

```powershell
Set-Location web/default
bun run typecheck
bun run lint
bun run format:check
bun run build
Set-Location ../classic
bun run lint
bun run build
Set-Location ../..
```

Expected: all commands exit 0 and both `dist/index.html` files exist.

- [ ] **Step 4: Start a local private candidate and inspect it**

Start the app on an unused localhost port with an isolated test database and
non-production settings. Use Playwright desktop/mobile screenshots and functional
checks for home, sign-in, console, pricing, Studio video controls, Canvas, and
developer docs. Confirm no visual change outside the approved video controls and
documentation.

- [ ] **Step 5: Record redacted local evidence**

Create the validation handoff with commit, commands, pass/fail counts, local
ports expressed only as `localhost`, screenshot filenames, exact enabled/probe
inventory, price calculations, and remaining real-task/server gates. Do not
record secrets, real user IDs, provider task IDs, balances, cookies, or private
asset URLs.

- [ ] **Step 6: Commit verification evidence**

```powershell
git add docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md
git commit -m "docs: record local video refresh verification"
```

### Task 7: Run bounded real VIDEO-group probes

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`
- Modify if evidence requires it: `model/yucore_media_cangyuan_catalog.json`
- Modify if evidence requires it: `model/yucore_media_openai_compatible_test.go`

- [ ] **Step 1: Capture non-secret pre-probe facts**

Read the authenticated model list, pricing version, token group, and balance
without displaying or saving credentials. Confirm the 14 target models remain
visible/priced and the four probes remain unpriced. Stop if inventory or price
differs from the approved table.

- [ ] **Step 2: Probe mapping ambiguities safely**

For Grok, Omni, and SD4, configure the isolated candidate with the public model
ID first and send the smallest valid YuAPI request. Only an explicit validation
rejection with no accepted ID and no debit permits one alternate runtime channel
mapping attempt. Once accepted or ambiguous, never send a replacement POST.
Record only a redacted case ID and the proven public-to-upstream mapping.

- [ ] **Step 3: Run one smallest valid accepted task per enabled model**

Use these non-secret minima:

```text
grok*=4s,480p,16:9
happyhouse*=3s,720p,16:9
minimax-h3-2k=5s,2k,16:9
omni-fast*=fixed 10s,720p,16:9,one image
omni-v2v*=fixed 10s,720p,16:9,one video
sd4*=4s,480p,16:9
sd7-720p=4s,16:9
sd7-1080p=4s,16:9
sd8=5s,16:9
```

The known accepted-task provider subtotal is `34.873`. Accepted mapping probes
from Step 2 are the model's one paid task and must not be repeated in Step 3.
Poll only each accepted ID, download one completed asset through the
authenticated content path, and compare provider debit and YuAPI quota with the
expected prices. Do not probe the four unknown-price models.

- [ ] **Step 4: Encode only proven mappings and rerun tests**

If runtime `model_mapping` is sufficient, leave the embedded public IDs
unchanged and record the mapping in the runbook's runtime table. If payload
behavior differs from the public contract, first add a failing fixture that
matches the observed non-secret request/response envelope, then make the
smallest adapter/catalog correction and rerun:

```powershell
go test ./model ./service -run 'Cangyuan|OpenAICompatible|YucoreMedia' -count=1
```

Expected: PASS.

- [ ] **Step 5: Append redacted paid-task evidence and commit**

Record model, parameters, accepted attempt count, status progression, MIME,
dimensions/duration, numeric debit, expected base/group charge, and result only.

```powershell
git add model/yucore_media_cangyuan_catalog.json model/yucore_media_openai_compatible_test.go docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md
git commit -m "test: verify current cangyuan video models"
```

### Task 8: Prepare private server candidate and stop at traffic approval

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Push the reviewed branch**

```powershell
git push -u origin codex/cangyuan-video-refresh-20260815
```

Expected: the remote branch points to the fully verified commit.

- [ ] **Step 2: Re-audit production read-only**

Confirm running image/commit, current and rollback containers, health/restart
counts, networks/aliases, private ports, exact Caddy runtime/file config, exact
two active YuAPI upstream references, database/Redis health, disk/memory, current
video channels, `ModelPrice`, group ratios, `TASK_PRICE_PATCH`, and aggregate
error counts. Runtime observations override older handoff names.

- [ ] **Step 3: Capture scoped rollback artifacts**

Write root-readable server-local artifacts containing only the current Cangyuan
video channel rows, affected `ModelPrice` entries, media capability override,
candidate environment, and exact two Caddy references. Validate that the files
can restore those values; do not dump the full database or user data.

- [ ] **Step 4: Build and start the blue-green candidate**

Build a unique image from the pushed commit, record the digest, and start a
uniquely named container on a new localhost port and unique release-network
alias. Keep the current app and every existing alias attached and reachable.
Do not restart Caddy, MySQL, Redis, the current app, or rollback containers.

- [ ] **Step 5: Verify candidate privately**

Verify health, restart count, source commit, asset fingerprints, Caddy-to-
candidate connectivity, protected UI pages, authenticated catalog, fixed prices,
one cheapest bounded target task, same-ID poll/content, quota, database/Redis
errors, and no secret-bearing logs without changing public Caddy routing.

- [ ] **Step 6: Record candidate evidence and request explicit switch approval**

Append redacted candidate facts to the validation handoff. Stop here. Present
the exact old/candidate container names, image digest, private health result,
the two Caddy references that would change, scoped configuration delta, and
rollback commands to the user. Do not reload Caddy or enable replacement
production channels until the user explicitly approves the traffic switch.

### Task 9: Hot cutover and observation after explicit approval

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Recheck the approval facts immediately before mutation**

Ensure container IDs, candidate health, exact two Caddy source references,
database values, and rollback artifacts still match the approved evidence. Stop
on drift.

- [ ] **Step 2: Validate and gracefully reload only the two Caddy references**

Generate a timestamped Caddy rollback copy, validate the candidate config, and
reload Caddy gracefully. Keep the current container running and its old aliases
reachable for connection draining. Do not restart Caddy.

- [ ] **Step 3: Apply the scoped video configuration transaction**

Enable only the five replacement family channels, disable the three legacy
Cangyuan video channels, set only the 14 approved `ModelPrice` values, apply the
exact 14-model `TASK_PRICE_PATCH`, refresh caches, and read back every value.
Do not modify image channels, group ratios, GPT settings, users, balances, or
existing task rows.

- [ ] **Step 4: Verify public production**

Check public health/assets, login/console/pricing/Studio/docs, 14 enabled and four
hidden probe models, one cheapest target generation, accepted-ID polling,
content/download, exact base/group charge, aggregate 4xx/5xx/502 counts,
container restarts, DB/Redis errors, existing image route, and a bounded GPT text
request. Poll/read must not charge.

- [ ] **Step 5: Roll back immediately on a trigger**

Restore the scoped video settings first, refresh/read back, validate the old
two-reference Caddy config, and gracefully reload it. Keep old and candidate
containers running until public rollback checks pass. Never restore a database
snapshot or stop the current app as part of immediate rollback.

- [ ] **Step 6: Record final evidence and retain rollback state**

Commit only redacted status, image/commit IDs, aggregate checks, pricing/model
facts, and rollback readiness. Keep legacy channels, old/candidate containers,
old aliases, Caddy copies, and scoped rollback artifacts until a separate user
cleanup approval.
