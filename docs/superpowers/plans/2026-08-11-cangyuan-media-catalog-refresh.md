# Cangyuan Media Catalog Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refresh the Cangyuan image/video catalog, add capability-driven Infinite Canvas controls, preserve conservative task billing, verify every retained route with real generation, and produce six local quota-card images without changing production before explicit approval.

**Architecture:** Embed a non-secret, audited capability manifest and merge it with operator overrides; project that manifest into the existing YuCore catalog and validate a canonical media request before it reaches a provider adapter. Keep media creation at-most-once after an upstream write may have happened, persist ambiguous submissions, and render advanced controls in a focused component inside the unchanged production brand shell.

**Tech Stack:** Go 1.22+, Gin, GORM, testify, React 19, TypeScript, Base UI/shadcn components, Bun test runner, i18next, Playwright, Docker.

---

## Safety Boundary

- Work only on `codex/cangyuan-media-refresh-20260811`, derived from production source `0918868420218c7b45ef0ee02702efa5e8dc7aee`.
- Do not change Caddy, the production container, production traffic, or production channel records during Tasks 1-12.
- Do not commit or print credentials, cookies, request headers, database rows, signed asset URLs, or upstream account data.
- Keep current global model prices and group ratios unchanged. Add a price only when the model has no existing price.
- Remove `gpt-image-2-1k` and `gpt-image-2-4k` only from the Cangyuan channel plan; do not remove them globally.
- Keep `seedance-2.0-mini-8s` and `veo-clean` disabled until real generation and upstream debit are both known.
- Do not restore a production database snapshot. No schema migration is planned.

## File Map

**Create:**

- `model/yucore_media_capability.go` - capability schema, shared reference input, embedded Cangyuan manifest loader, merge rules, and configuration validation.
- `model/yucore_media_cangyuan_catalog.json` - 40 non-secret catalog entries: seven image routes, 31 priced video routes, and two disabled account-visible video routes.
- `model/yucore_media_capability_test.go` - inventory, parsing, override, and validation contracts.
- `service/yucore_media_request.go` - canonical request/reference normalization and server-side capability validation.
- `service/yucore_media_request_test.go` - duration, resolution, audio, reference mode, and reference-count contracts.
- `web/default/src/features/yucore-brand/lib/media-generation.ts` - pure draft normalization, validation, and API payload construction.
- `web/default/src/features/yucore-brand/lib/media-generation.test.ts` - frontend capability and payload contracts.
- `web/default/src/features/yucore-brand/components/yucore-media-controls.tsx` - focused capability-driven controls rendered inside the existing Studio shell.
- `web/default/src/features/yucore-brand/components/yucore-media-controls.test.tsx` - server-rendered control visibility and accessibility checks.
- `scripts/production/cangyuan-media-probe.mjs` - local-only, secret-redacting real-generation runner that never repeats an ambiguous POST.
- `scripts/production/cangyuan-media-probe.test.mjs` - probe state-machine and redaction tests.
- `web/default/playwright.config.ts` - local candidate browser-test configuration.
- `web/default/e2e/yucore-cangyuan-media.spec.ts` - Canvas generation, task restoration, and playback checks.
- `web/default/e2e/yucore-production-baseline.spec.ts` - production-versus-local brand/UI screenshot checks.

**Modify:**

- `model/yucore_media_openai_compatible.go` - consume the new capability schema, build richer payloads, and normalize task result metadata.
- `model/yucore_media_openai_compatible_test.go` - provider-family payload and same-task polling tests.
- `model/yucore_media.go` - merge embedded capabilities with options and support typed reference metadata without a migration.
- `service/yucore_media_catalog.go` - expose richer capabilities and pricing units to the browser.
- `service/yucore_media_catalog_test.go` - catalog projection and disabled-model tests.
- `controller/yucore_media.go` - pointer-based optional request fields and image/video/audio uploads.
- `controller/yucore_media_test.go` - omitted-versus-explicit scalar and upload-policy tests.
- `dto/task.go` - internal task submission acceptance state.
- `relay/common/relay_info.go` - upstream request-write marker for task submissions.
- `relay/channel/api_request.go` - `httptrace.WroteRequest` instrumentation for task requests.
- `relay/relay_task.go` - classify pre-send, rejected, ambiguous, and accepted failures.
- `relay/relay_task_test.go` - submission-state classification tests.
- `controller/relay.go` - skip retry/refund after an ambiguous write and persist an auditable unknown task.
- `controller/relay_task_retry_test.go` - no-retry/no-refund/persisted-task regression tests.
- `service/task_billing.go` and `service/task_billing_test.go` - settle conservative pre-consumption exactly once for ambiguous submissions.
- `web/default/src/features/yucore-brand/api/studio.ts` - catalog, upload, and task request types.
- `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx` - integrate the focused controls and canonical payload while preserving the brand shell.
- `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json` - updated only through the required script workflow.
- `web/default/package.json` and `web/bun.lock` - Playwright development dependency and test scripts.
- `web/default/public/developer-docs/yucore-api.md` - current models, parameters, billing, polling, and recovery guidance.

## Task 1: Embed and Validate the Audited Catalog

**Files:**

- Create: `model/yucore_media_capability.go`
- Create: `model/yucore_media_cangyuan_catalog.json`
- Create: `model/yucore_media_capability_test.go`
- Modify: `model/yucore_media_openai_compatible.go`
- Modify: `model/yucore_media.go`

- [ ] **Step 1: Write failing inventory and validation tests**

Add tests that demand 40 unique entries, 38 enabled entries, seven image entries,
33 video entries, and exactly two disabled account-visible entries:

```go
func TestCangyuanMediaCatalogInventory(t *testing.T) {
    catalog, err := loadEmbeddedYucoreMediaCapabilities()
    require.NoError(t, err)
    require.Len(t, catalog, 40)

    imageCount, videoCount, enabledCount := 0, 0, 0
    for modelID, capability := range catalog {
        assert.Equal(t, modelID, capability.Model)
        if capability.Kind == "image" {
            imageCount++
        }
        if capability.Kind == "video" {
            videoCount++
        }
        if capability.Availability == YucoreMediaAvailabilityEnabled {
            enabledCount++
        }
    }
    assert.Equal(t, 7, imageCount)
    assert.Equal(t, 33, videoCount)
    assert.Equal(t, 38, enabledCount)
    assert.Equal(t, YucoreMediaAvailabilityProbe, catalog["seedance-2.0-mini-8s"].Availability)
    assert.Equal(t, YucoreMediaAvailabilityProbe, catalog["veo-clean"].Availability)
}
```

Also assert that `sora-2`, `sora-2-pro`, `veo-3-1`, `veo-3-1-fast`, and
`veo-3-1-ref` are absent, while `veo-3.1` and `veo-3.1-fast` are present.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./model -run 'TestCangyuanMediaCatalogInventory|TestValidateYucoreMediaModelCapabilities' -count=1
```

Expected: FAIL because the embedded loader and richer capability fields do not exist.

- [ ] **Step 3: Define the capability schema and embedded loader**

Move the existing capability parsing/validation domain into the new focused file
and extend it with these concrete types:

```go
const (
    YucoreMediaAvailabilityEnabled = "enabled"
    YucoreMediaAvailabilityProbe   = "probe"
    YucoreMediaPricingPerCall      = "per_call"
    YucoreMediaPricingPerSecond    = "per_second"
)

type YucoreMediaReferenceLimits struct {
    Images             int `json:"images,omitempty"`
    Videos             int `json:"videos,omitempty"`
    Audios             int `json:"audios,omitempty"`
    Total              int `json:"total,omitempty"`
    MaxVideoDurationMS int `json:"max_video_duration_ms,omitempty"`
    MaxAudioDurationMS int `json:"max_audio_duration_ms,omitempty"`
}

type YucoreMediaReferenceInput struct {
    Role       string `json:"role"`
    URL        string `json:"url"`
    MimeType   string `json:"mime_type,omitempty"`
    DurationMS *int   `json:"duration_ms,omitempty"`
}

type YucoreMediaModelCapability struct {
    Model                string                     `json:"model,omitempty"`
    UpstreamModel        string                     `json:"upstream_model,omitempty"`
    Kind                 string                     `json:"kind,omitempty"`
    Family               string                     `json:"family,omitempty"`
    Availability         string                     `json:"availability,omitempty"`
    PricingUnit          string                     `json:"pricing_unit,omitempty"`
    UpstreamCost         float64                    `json:"upstream_cost,omitempty"`
    Transport            string                     `json:"transport,omitempty"`
    CreatePath           string                     `json:"create_path,omitempty"`
    EditPath             string                     `json:"edit_path,omitempty"`
    StatusPath           string                     `json:"status_path,omitempty"`
    ContentPath          string                     `json:"content_path,omitempty"`
    CancelPath           string                     `json:"cancel_path,omitempty"`
    DurationPolicy       string                     `json:"duration_policy,omitempty"`
    FixedDurationSeconds int                        `json:"fixed_duration_seconds,omitempty"`
    Durations            []int                      `json:"durations,omitempty"`
    Resolutions          []string                   `json:"resolutions,omitempty"`
    AspectRatios         []string                   `json:"aspect_ratios,omitempty"`
    ReferenceModes       []string                   `json:"reference_modes,omitempty"`
    ReferenceLimits      YucoreMediaReferenceLimits `json:"reference_limits,omitempty"`
    SupportsAudio        bool                       `json:"supports_audio,omitempty"`
    SupportsSeed         bool                       `json:"supports_seed,omitempty"`
    PollIntervalSeconds  int                        `json:"poll_interval_seconds,omitempty"`
    MaxPollDurationSeconds int                      `json:"max_poll_duration_seconds,omitempty"`
    MaxReferenceImages   int                        `json:"max_reference_images,omitempty"`
    AllowedParameters    []string                   `json:"allowed_parameters,omitempty"`
    TerminalSuccessStates []string                  `json:"terminal_success_states,omitempty"`
    TerminalFailureStates []string                  `json:"terminal_failure_states,omitempty"`
    ResponseFormat       string                     `json:"response_format,omitempty"`
    Notes                []string                   `json:"notes,omitempty"`
}
```

Use `//go:embed yucore_media_cangyuan_catalog.json`, decode through
`common.Unmarshal`, validate duplicate IDs, and return a fresh map on each call.
Decode embedded and operator documents into structured `map[string]map[string]any`
objects, overlay only keys that are present in the operator object, then encode
and decode the merged entry through `common.Marshal`/`common.Unmarshal`. This
presence-aware merge must preserve explicit operator `false`, `0`, and empty
slice values. An operator entry with `availability:"probe"` must keep the model
out of the public catalog without deleting its test metadata.

Normalize the legacy `max_reference_images` field into
`reference_limits.images` when the richer field is absent. If both are present
with different non-zero values, reject the configuration instead of silently
choosing one. Validate create/status/content/cancel path templates, pricing unit,
poll bounds, terminal-state lists, per-kind limits, and every copied slice.

- [ ] **Step 4: Populate the manifest with exact audited IDs and costs**

The seven enabled image entries are:

```text
gpt-image-2-2k=0.065/request
nano-banana-pro-1k=0.08/request
nano-banana-pro-2k=0.10/request
nano-banana-pro-4k=0.149/request
nano-banana2-1k=0.059/request
nano-banana2-2k=0.095/request
nano-banana2-4k=0.135/request
```

The 31 enabled video entries and audited costs are:

```text
gemini-omni-flash=0.75/request
grok-video=0.69/request
grok-video-1.5=1.39/request
happyhouse-1.0=4.5/request
happyhouse-1.1=2.9/request
kling-3.0=1.3/request
kling-3.0-omni=1.3/request
minimax-h3-2k=2.5/request
omni-fast=0.6624/request
omni-fast-no-water=0.81/request
omni-v2v=0.8856/request
omni-v2v-no-water=1.035/request
sd5-seedance-2.0=3.35/request
sd5-seedance-2.0-fast=2.1/request
sd6-seedance-2.0-1080p=0.89/second
sd6-seedance-2.0-720p=4.6/request
seedance-2.0=3.9/request
seedance-2.0-1080p=2.25/second
seedance-2.0-480p=0.45/second
seedance-2.0-4k=4.5/second
seedance-2.0-720p=0.975/second
seedance-2.0-fast=2.9/request
seedance-2.0-fast-480p=0.25/second
seedance-2.0-fast-720p=0.75/second
seedance-2.0-mini=2.4/request
seedance-2.0-mini-480p=0.3/second
seedance-2.0-mini-720p=0.525/second
seedance-2.5-480p=0.25/second
seedance-2.5-720p=0.35/second
veo-3.1=0.99/request
veo-3.1-fast=0.5/request
```

Add `seedance-2.0-mini-8s` and `veo-clean` with `availability:"probe"` and
without `upstream_cost`. Encode the audited duration, resolution, aspect-ratio,
audio, seed, negative-prompt support, reference limits, task paths, terminal
states, and poll bounds for each family rather than inventing shared defaults.

- [ ] **Step 5: Merge embedded capabilities in runtime configuration**

Change `getYucoreMediaAdapterConfig` so the embedded map is loaded first and
`YUCORE_MEDIA_MODEL_CAPABILITIES` / option-map entries override it. Preserve
copy-on-read behavior in `GetYucoreMediaCatalogSettings` for every slice.

- [ ] **Step 6: Run model tests and verify GREEN**

Run:

```powershell
go test ./model -run 'YucoreMedia.*Capabilit|CangyuanMediaCatalogInventory' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add model/yucore_media_capability.go model/yucore_media_cangyuan_catalog.json model/yucore_media_capability_test.go model/yucore_media_openai_compatible.go model/yucore_media.go
git commit -m "feat: add audited cangyuan media capabilities"
```

## Task 2: Project Capabilities into the YuCore Catalog

**Files:**

- Modify: `service/yucore_media_catalog.go`
- Modify: `service/yucore_media_catalog_test.go`

- [ ] **Step 1: Write failing catalog projection tests**

Add a configured Seedance ability and assert the catalog returns exact fields:

```go
assert.Equal(t, []int{4, 5, 6, 8, 10, 12, 15}, item.Durations)
assert.Equal(t, []string{"1080p"}, item.Resolutions)
assert.Equal(t, []string{"text", "media", "frames"}, item.ReferenceModes)
assert.Equal(t, 9, item.InputLimits.MaxReferenceImages)
assert.Equal(t, 3, item.InputLimits.MaxReferenceVideos)
assert.Equal(t, 1, item.InputLimits.MaxReferenceAudios)
assert.Equal(t, 12, item.InputLimits.MaxReferences)
assert.Equal(t, "per_second", item.Pricing.Unit)
```

Add a second test proving an active channel ability for a `probe` model is not
returned to users.

- [ ] **Step 2: Run the tests and verify RED**

```powershell
go test ./service -run 'TestBuildYucoreMediaCatalog.*Capabilit|TestBuildYucoreMediaCatalogHidesProbeModels' -count=1
```

Expected: FAIL because these catalog fields and availability filtering are absent.

- [ ] **Step 3: Extend catalog DTOs and projection**

Add `Resolutions`, `ReferenceModes`, `SupportsAudio`, `SupportsSeed`, and the
three reference limits plus total limit. Copy slices so API responses cannot
mutate shared configuration. Use `capability.PricingUnit` as the authoritative
unit; retain `TASK_PRICE_PATCH` only as compatibility fallback for capabilities
without an explicit pricing unit.

Do not expose `UpstreamCost`, upstream model IDs, channel IDs, or credentials in
the browser catalog.

- [ ] **Step 4: Run focused and existing catalog tests**

```powershell
go test ./service -run 'YucoreMediaCatalog|ValidateYucoreMediaRequest' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add service/yucore_media_catalog.go service/yucore_media_catalog_test.go
git commit -m "feat: expose media generation capabilities"
```

## Task 3: Normalize and Validate Canonical Media Requests

**Files:**

- Create: `service/yucore_media_request.go`
- Create: `service/yucore_media_request_test.go`
- Modify: `controller/yucore_media.go`
- Modify: `controller/yucore_media_test.go`

- [ ] **Step 1: Write failing request-contract tests**

Define tests for omitted versus explicit optional scalars and incompatible
reference modes. The shared `YucoreMediaReferenceInput` belongs to `model`, so
provider payload construction can use it without creating a model/service import
cycle:

```go
func TestNormalizeYucoreMediaRequestPreservesExplicitFalseAndZero(t *testing.T) {
    disabled := false
    zero := int64(0)
    selected := YucoreMediaCatalogModel{
        Id:            "seedance-test",
        Durations:     []int{8},
        SupportsAudio: true,
        SupportsSeed:  true,
    }
    got, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
        GenerateAudio: &disabled,
        Seed:          &zero,
    })
    require.NoError(t, err)
    require.NotNil(t, got.GenerateAudio)
    assert.False(t, *got.GenerateAudio)
    require.NotNil(t, got.Seed)
    assert.Zero(t, *got.Seed)
}
```

Cover: invalid duration, invalid resolution, unsupported audio, seed omitted,
first frame without last frame, frames mixed with media references, audio-only
reference where a primary image is required, and family total reference limit.

- [ ] **Step 2: Run the tests and verify RED**

```powershell
go test ./service ./controller -run 'NormalizeYucoreMediaRequest|BuildYucoreMediaTaskPreservesOptional' -count=1
```

Expected: FAIL because canonical option/reference types do not exist.

- [ ] **Step 3: Add canonical request types and validation**

Use pointer fields for optional scalars and reference the shared model type:

```go
type YucoreMediaRequestOptions struct {
    Mode          string
    Count         int
    Duration      *int
    Resolution    string
    AspectRatio   string
    GenerateAudio *bool
    Seed          *int64
    NegativePrompt *string
    ReferenceMode string
    References    []model.YucoreMediaReferenceInput
}
```

`NormalizeYucoreMediaRequest` returns a normalized copy, never mutates the
caller, selects capability defaults only for omitted fields, and rejects invalid
combinations before the task row or upstream request is created.

- [ ] **Step 4: Extend the controller DTO without breaking old clients**

Add `duration`, `resolution`, `generate_audio`, `seed`, `negative_prompt`, and
`reference_mode` to `yucoreMediaTaskRequest`. Use pointer types for every new
optional scalar in the parsed request DTO, including optional strings. Decode
legacy `inputs` and `metadata` through
`common.Unmarshal`, normalize them into canonical `inputs`/`metadata`, and keep
the existing response shape. Explicit `false` and `0` must survive the round trip.

- [ ] **Step 5: Run focused controller/service tests**

```powershell
go test ./service ./controller -run 'YucoreMediaRequest|YucoreMediaTaskPreserves' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add service/yucore_media_request.go service/yucore_media_request_test.go controller/yucore_media.go controller/yucore_media_test.go
git commit -m "feat: validate canonical media requests"
```

## Task 4: Accept Image, Video, and Audio Reference Uploads

**Files:**

- Modify: `controller/yucore_media.go`
- Modify: `controller/yucore_media_test.go`
- Modify: `web/default/src/features/yucore-brand/api/studio.ts`

- [ ] **Step 1: Write failing upload policy tests**

Use a deterministic table test for MIME type, extension, media kind, and global
hard limit:

```go
tests := []struct {
    mime string
    name string
    kind string
    ext  string
}{
    {"image/png", "frame.png", "image", ".png"},
    {"video/mp4", "motion.mp4", "video", ".mp4"},
    {"video/quicktime", "motion.mov", "video", ".mov"},
    {"audio/mpeg", "music.mp3", "audio", ".mp3"},
    {"audio/wav", "voice.wav", "audio", ".wav"},
}
```

Also assert executable, HTML, SVG-with-script, and unknown binary uploads are
rejected. Keep signed serving authorization behavior unchanged.

- [ ] **Step 2: Run and verify RED**

```powershell
go test ./controller -run 'YucoreMediaUpload' -count=1
```

Expected: FAIL because only images are accepted.

- [ ] **Step 3: Implement the upload policy**

Use an initial 101 MiB multipart request envelope so a valid 100 MiB video is not
rejected before its MIME type is known, then enforce per-file caps of 25 MiB for
images, 100 MiB for video, and 25 MiB for audio while streaming the selected
part. Return `kind` in the upload response. Inspect only the bounded MIME-sniff
prefix and never log file bodies. Continue writing owner-only files with `0700`
directories, `0600` files, signed read URLs, `nosniff`, and private cache control.

- [ ] **Step 4: Update frontend upload types**

Extend `YucoreMediaReferenceUpload` with `kind: 'image' | 'video' | 'audio'` and
optional `duration_ms`. Keep the same multipart endpoint.

- [ ] **Step 5: Run tests**

```powershell
go test ./controller -run 'YucoreMediaUpload|ServeYucoreMediaUploadedReference' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add controller/yucore_media.go controller/yucore_media_test.go web/default/src/features/yucore-brand/api/studio.ts
git commit -m "feat: support media reference uploads"
```

## Task 5: Build Exact Provider Payloads

**Files:**

- Modify: `model/yucore_media_openai_compatible.go`
- Modify: `model/yucore_media_openai_compatible_test.go`

- [ ] **Step 1: Write failing family payload tests**

Add one table case for each distinct binding contract:

- Omni single image, first/last frame, and video-to-video.
- Grok `image_urls` plus `seconds`.
- Happyhouse and Kling duration/resolution/audio/seed.
- Seedance primary image plus remaining references.
- Seedance multimodal image/video/audio arrays.
- Seedance first/last frame mutual exclusion.
- Veo duration/resolution and image references.
- Explicit `audio:false`, `seed:0`, and supported negative-prompt preservation.

Example assertion:

```go
assert.Equal(t, "https://cdn.example.com/main.png", payload["image_url"])
assert.Equal(t, []string{"https://cdn.example.com/style.png"}, payload["reference_image_urls"])
assert.Equal(t, []string{"https://cdn.example.com/motion.mp4"}, payload["reference_videos"])
assert.Equal(t, []string{"https://cdn.example.com/music.mp3"}, payload["reference_audios"])
assert.Equal(t, false, payload["audio"])
assert.Equal(t, int64(0), payload["seed"])
```

- [ ] **Step 2: Run and verify RED**

```powershell
go test ./model -run 'TestBuildOpenAICompatibleAsyncPayload.*(Omni|Grok|Seedance|Veo|Kling|Happyhouse)' -count=1
```

Expected: FAIL on unsupported canonical fields.

- [ ] **Step 3: Extend payload construction directly**

Group canonical references by role. Select provider field names only from the
capability allowlist:

```text
image -> image_url / image_urls / images / reference_image_urls
video -> video_url / reference_videos
audio -> reference_audios
first_frame -> first_image_url
last_frame -> last_image_url
resolution -> resolution, falling back to size only when configured
generated audio -> audio or generate_audio, never both
negative prompt -> negative_prompt only when capability-allowed
```

Keep `duration` numeric and `seconds` string only where the existing upstream
contract requires it. Never forward unknown `metadata` keys. Build the payload
once and pass the same normalized duration used by billing.

- [ ] **Step 4: Run payload and existing adapter tests**

```powershell
go test ./model -run 'OpenAICompatible|YucoreMediaCapability' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add model/yucore_media_openai_compatible.go model/yucore_media_openai_compatible_test.go
git commit -m "feat: relay advanced media parameters"
```

## Task 6: Normalize Results and Poll Only the Accepted Task

**Files:**

- Modify: `model/yucore_media_openai_compatible.go`
- Modify: `model/yucore_media_openai_compatible_test.go`
- Modify: `model/yucore_media.go`

- [ ] **Step 1: Write failing task-result tests**

Use an `httptest.Server` with counters. The creation endpoint returns one task
ID; subsequent hydration must issue GET only for that ID and must never issue a
second POST. Assert completed assets include playable URL, thumbnail, MIME type,
duration, width, height, and upstream status metadata when present.

- [ ] **Step 2: Run and verify RED**

```powershell
go test ./model -run 'TestOpenAICompatibleTaskPollsAcceptedIDOnly|TestOpenAICompatibleTaskResultMetadata' -count=1
```

Expected: FAIL because complete metadata extraction is missing.

- [ ] **Step 3: Extend result normalization**

Read task IDs and result fields only through the existing normalized response
helpers. Persist provider task ID once. Merge `video_url`, content URL,
`thumbnail_url`, `duration_ms`, `width`, `height`, and `mime_type` into
`YucoreMediaAsset`; preserve relative content URLs so the existing proxy can
resolve them. Polling errors update `last_status_error` but do not create a new
task or mark the accepted task failed prematurely.

- [ ] **Step 4: Run model task tests**

```powershell
go test ./model -run 'OpenAICompatibleTask|YucoreMediaTask' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add model/yucore_media_openai_compatible.go model/yucore_media_openai_compatible_test.go model/yucore_media.go
git commit -m "feat: preserve media task results and polling"
```

## Task 7: Make Task Creation At-Most-Once After an Upstream Write

**Files:**

- Modify: `dto/task.go`
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/api_request.go`
- Modify: `relay/relay_task.go`
- Modify: `relay/relay_task_test.go`
- Modify: `controller/relay.go`
- Modify: `controller/relay_task_retry_test.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`

- [ ] **Step 1: Write failing retry and refund tests**

Add an internal-only submission state to `TaskError`. Test refundability for all
states, and test retry behavior separately because a `not_sent` transport error
may remain retryable while local validation/pricing errors are not:

```go
tests := []struct {
    name       string
    state      dto.TaskSubmissionState
    refundable bool
}{
    {"not sent", dto.TaskSubmissionNotSent, true},
    {"explicitly rejected", dto.TaskSubmissionRejected, true},
    {"write ambiguous", dto.TaskSubmissionAmbiguous, false},
    {"accepted response invalid", dto.TaskSubmissionAccepted, false},
}
```

Add retry-policy cases proving `ambiguous` and `accepted` are never retried,
while `not_sent` still follows the existing status/local-error rules.

Add a billing-session test proving ambiguous submission calls `Settle` with the
frozen pre-consumed quota and never calls `Refund`. Add a controller test proving
an ambiguous submission persists a `TaskStatusUnknown` row with the public task
ID and returns that ID in `TaskError.Data`.

Add exact billing invariants: a `0.5` per-call model with group ratio `1.2`
charges `0.6` regardless of duration metadata; a `0.35` per-second model at five
validated seconds with the same group ratio charges `2.1`; polling and result
reads do not charge again; no retained route can produce a zero or below-cost
charge.

- [ ] **Step 2: Run and verify RED**

```powershell
go test ./relay ./controller ./service -run 'TaskSubmission|RetryTaskRelay|AmbiguousTask' -count=1
```

Expected: FAIL because submission state and conservative settlement are absent.

- [ ] **Step 3: Instrument task request writes**

Attach `httptrace.ClientTrace{WroteRequest: ...}` only inside
`DoTaskApiRequest`. Record the marker on `RelayInfo.TaskRelayInfo`; do not change
stream/text request behavior.

- [ ] **Step 4: Classify errors at the relay boundary**

Use these rules:

- request construction, mapping, pricing, or pre-consumption failure: `not_sent`;
- explicit 400, 401, 403, 404, 409, 422, or 429 response: `rejected`;
- network error after `WroteRequest`, HTTP 408, or any 5xx after a write: `ambiguous`;
- any 2xx response that cannot be parsed or has no task ID: `accepted`.

`shouldRetryTaskRelay` must return false for `ambiguous` and `accepted` before
examining status codes.

- [ ] **Step 5: Preserve charge and persist ambiguity**

Replace the unconditional deferred refund with a pure predicate. For ambiguous
or accepted errors:

1. settle the billing session at the frozen pre-consumed quota;
2. log consumption once;
3. insert an unknown task using the pre-generated public task ID and frozen
   billing context;
4. return `data: {"task_id": "task_...", "submission_state": "unknown"}`;
5. never auto-submit another POST.

Explicitly rejected and not-sent failures retain the existing refund behavior.

- [ ] **Step 6: Run focused tests and the complete task billing suite**

```powershell
go test ./relay ./controller ./service -run 'Task|Billing|Submission|Retry' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add dto/task.go relay/common/relay_info.go relay/channel/api_request.go relay/relay_task.go relay/relay_task_test.go controller/relay.go controller/relay_task_retry_test.go service/task_billing.go service/task_billing_test.go
git commit -m "fix: preserve billing after ambiguous task submission"
```

## Task 8: Add Pure Frontend Capability and Payload Logic

**Files:**

- Modify: `web/default/src/features/yucore-brand/api/studio.ts`
- Create: `web/default/src/features/yucore-brand/lib/media-generation.ts`
- Create: `web/default/src/features/yucore-brand/lib/media-generation.test.ts`
- Modify: `web/default/src/features/yucore-brand/lib/media-catalog.ts`

- [ ] **Step 1: Install the existing frontend dependencies**

```powershell
Set-Location web/default
bun install --frozen-lockfile
```

Expected: dependencies install without modifying the lockfile.

- [ ] **Step 2: Write failing pure-logic tests**

Use Bun's compatible test runner and strict assertions like existing YuCore tests. Assert that
model changes reset only unsupported fields, explicit `false` and `0` remain,
frame references cannot mix with media references, and the payload contains only
capability-supported keys.

```ts
assert.deepEqual(
  buildMediaTaskPayload(model, {
    duration: 8,
    generateAudio: false,
    seed: 0,
    referenceMode: 'media',
    references,
  }),
  {
    duration: 8,
    generate_audio: false,
    seed: 0,
    reference_mode: 'media',
    inputs: references,
  }
)
```

- [ ] **Step 3: Run and verify RED**

```powershell
bun test src/features/yucore-brand/lib/media-generation.test.ts
```

Expected: FAIL because the module does not exist.

- [ ] **Step 4: Extend API types and implement pure helpers**

Add typed durations, resolutions, reference modes/limits, audio/seed support,
and typed reference roles to `YucoreMediaModel`. Keep control values derived
during render; do not mirror capability arrays into effect-managed state.

Implement:

```ts
export function normalizeMediaDraft(
  model: YucoreMediaModel,
  draft: YucoreMediaDraft
): YucoreMediaDraft

export function validateMediaDraft(
  model: YucoreMediaModel,
  draft: YucoreMediaDraft
): string[]

export function buildMediaTaskPayload(
  model: YucoreMediaModel,
  draft: YucoreMediaDraft
): YucoreMediaGenerationPayload
```

Use functional updates for reference arrays and stable module-level empty arrays.

- [ ] **Step 5: Run tests and typecheck**

```powershell
bun test src/features/yucore-brand/lib/media-generation.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add web/default/src/features/yucore-brand/api/studio.ts web/default/src/features/yucore-brand/lib/media-generation.ts web/default/src/features/yucore-brand/lib/media-generation.test.ts web/default/src/features/yucore-brand/lib/media-catalog.ts
git commit -m "feat: add media generation form model"
```

## Task 9: Render Capability-Driven Controls in the Existing Brand Shell

**Files:**

- Create: `web/default/src/features/yucore-brand/components/yucore-media-controls.tsx`
- Create: `web/default/src/features/yucore-brand/components/yucore-media-controls.test.tsx`
- Modify: `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`
- Modify: `web/default/src/features/yucore-brand/components/index.ts`

- [ ] **Step 1: Write a failing server-rendered component test**

Render a Seedance capability and assert accessible labels for resolution,
duration, reference mode, generated audio, negative prompt, seed, first frame,
last frame, image, video, and audio references. Render an Omni V2V capability
and assert unsupported controls are absent.

- [ ] **Step 2: Run and verify RED**

```powershell
Set-Location web/default
bun test src/features/yucore-brand/components/yucore-media-controls.test.tsx
```

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the focused controls component**

Use existing shadcn/Base UI primitives:

- `FieldGroup`/`Field`/`FieldLabel` for form structure;
- `Select` for model/reference mode option sets;
- `ToggleGroup` for duration, aspect ratio, and resolution choices;
- `Switch` for generated audio;
- `Input` for seed;
- icon buttons using the configured Hugeicons library for add/remove actions;
- stable dimensions and the existing YuCore dark surface tokens.

Do not add cards inside the existing panel. Do not alter the page shell,
navigation, brand mark, background, motion, or current canvas dimensions.

- [ ] **Step 4: Integrate with the Studio workspace**

Replace only the existing inline media-option block with the focused component.
Keep submission inside the click handler, call `validateMediaDraft`, upload each
reference once, then call `createYucoreMediaTask` once. Poll by returned task ID.
On page reload, existing task/canvas restoration must recreate a playable video
node from persisted assets.

- [ ] **Step 5: Run component tests, typecheck, and scoped lint**

```powershell
bun test src/features/yucore-brand/components/yucore-media-controls.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/yucore-brand/components/yucore-media-controls.tsx src/features/yucore-brand/components/yucore-studio-workspace.tsx
```

Expected: PASS with no lint errors in touched files.

- [ ] **Step 6: Commit**

```powershell
git add web/default/src/features/yucore-brand/components/yucore-media-controls.tsx web/default/src/features/yucore-brand/components/yucore-media-controls.test.tsx web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx web/default/src/features/yucore-brand/components/index.ts
git commit -m "feat: add advanced infinite canvas media controls"
```

## Task 10: Add All UI Copy Through the Required i18n Workflow

**Files:**

- Temporarily create and delete: `web/default/scripts/add-missing-keys.mjs`
- Modify through script only: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Run the current sync report**

```powershell
Set-Location web/default
bun run i18n:sync
```

Record the pre-existing report; do not fix unrelated locale debt.

- [ ] **Step 2: Create the sanctioned locale update script**

Use the exact structure required by `.agents/skills/i18n-translate/SKILL.md` and
populate all six locales for the new keys. The minimum new key set is:

```text
Reference mode
Reference images
Reference videos
Reference audio
First frame
Last frame
Generate audio
Negative prompt
Resolution
Seed
Media references
Frames cannot be combined with other references.
This model requires both a first frame and a last frame.
This model accepts at most {{count}} {{type}} references.
This model accepts at most {{count}} references in total.
The selected model does not support this option.
```

Use compact, natural translations; preserve `{{count}}` and `{{type}}` exactly.
Do not edit any locale JSON manually.

- [ ] **Step 3: Apply, sync, and verify**

```powershell
bun scripts/add-missing-keys.mjs
bun run i18n:sync
rg -n 'Reference mode|Frames cannot be combined' src/i18n/locales -g '*.json'
Remove-Item -LiteralPath scripts/add-missing-keys.mjs
```

Expected: every new key exists in all six locales and the sync report adds no
new missing keys.

- [ ] **Step 4: Run frontend checks**

```powershell
bun run typecheck
bun run format:check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "i18n: translate media generation controls"
```

## Task 11: Add a Secret-Redacting Real-Generation Probe

**Files:**

- Create: `scripts/production/cangyuan-media-probe.mjs`
- Create: `scripts/production/cangyuan-media-probe.test.mjs`

- [ ] **Step 1: Write failing state-machine tests**

Test with a local fake HTTP server:

- image success downloads one file;
- video success submits one POST and polls the returned ID;
- POST timeout records `ambiguous` and performs zero retries;
- polling timeout continues only GET requests for the same ID;
- output never includes authorization values, cookies, signed query strings, or
  raw headers;
- final filenames contain only sanitized model and quota identifiers.

- [ ] **Step 2: Run and verify RED**

```powershell
bun test scripts/production/cangyuan-media-probe.test.mjs
```

Expected: FAIL because the probe does not exist.

- [ ] **Step 3: Implement the probe**

Read `YUAPI_TEST_BASE_URL` and `YUAPI_TEST_API_KEY` from environment variables.
Reject any base URL that is not `127.0.0.1` or `localhost` unless
`YUAPI_ALLOW_REMOTE_PROBE=1` is explicitly set for the later approved paid test.
Never print the key. Use one POST per case, persist the task ID immediately, poll
with GET only, and write a redacted JSONL audit file.

Add an explicit `direct-upstream-audit` mode that reads a separate base URL and
credential from environment variables, accepts only a caller-supplied allowlist
of model IDs, records debit deltas without account details, and never applies
YuAPI pricing. This mode is reserved for the two `probe` models whose upstream
cost must be known before they can be enabled locally.

Add quota-card cases for `5`, `20`, `50`, `100`, `200`, and `500` using the
verbatim Chinese hierarchy from the supplied reference. Exercise all seven
image models; use the seventh route as a second candidate for one amount and
keep the better of the two only after visual inspection. Save selected files as:

```text
output/cangyuan-media-20260811/quota-5-<model>.png
output/cangyuan-media-20260811/quota-20-<model>.png
output/cangyuan-media-20260811/quota-50-<model>.png
output/cangyuan-media-20260811/quota-100-<model>.png
output/cangyuan-media-20260811/quota-200-<model>.png
output/cangyuan-media-20260811/quota-500-<model>.png
```

The script must not commit generated media or audit output.

- [ ] **Step 4: Run probe unit tests**

```powershell
bun test scripts/production/cangyuan-media-probe.test.mjs
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add scripts/production/cangyuan-media-probe.mjs scripts/production/cangyuan-media-probe.test.mjs
git commit -m "test: add cangyuan media generation probe"
```

## Task 12: Update the Developer Documentation

**Files:**

- Modify: `web/default/public/developer-docs/yucore-api.md`

- [ ] **Step 1: Write a documentation verification command that fails when stale IDs remain active**

```powershell
$doc = Get-Content -Raw -Encoding utf8 web/default/public/developer-docs/yucore-api.md
$active = [regex]::Match($doc, '(?s)<!-- active-media-catalog:start -->(.*?)<!-- active-media-catalog:end -->').Groups[1].Value
if (-not $active) { throw 'active media catalog markers are missing' }
$stale = @('sora-2','sora-2-pro','veo-3-1','veo-3-1-fast','veo-3-1-ref')
foreach ($id in $stale) { if ($active -match [regex]::Escape($id)) { throw "active stale model: $id" } }
```

Expected: FAIL on the current document.

- [ ] **Step 2: Replace the stale media sections**

Document the seven retained Cangyuan image models, 31 enabled priced video
models, and two pending validation models. Wrap only the current catalog in
`<!-- active-media-catalog:start -->` / `<!-- active-media-catalog:end -->` so
removed IDs can remain in the dated history without being mistaken for active
routes. Include:

- dated added/removed/replaced table;
- per-request versus per-second billing with the unchanged `1.2` group ratio;
- exact duration, resolution, aspect-ratio, audio, seed, and reference limits;
- one POST followed by same-ID GET polling;
- `unknown` submission state and the rule not to repeat ambiguous creation;
- image/video/audio upload and Infinite Canvas behavior;
- examples with clearly redacted example keys and public-safe URLs only.

Correct the prior statement that Cangyuan still serves GPT Image 1K and 4K.

- [ ] **Step 3: Run documentation scans**

Run the stale-ID command again, then:

```powershell
rg -n 'gpt-image-2-2k|veo-3\.1|seedance-2\.5-720p|per_second|submission_state' web/default/public/developer-docs/yucore-api.md
git diff --check
```

Expected: no stale IDs in the active catalog, all required current concepts
present, no whitespace errors.

- [ ] **Step 4: Commit**

```powershell
git add web/default/public/developer-docs/yucore-api.md
git commit -m "docs: refresh cangyuan media API catalog"
```

## Task 13: Build, Run Locally, Compare UI, and Perform Paid Validation

**Files:**

- Create: `web/default/playwright.config.ts`
- Create: `web/default/e2e/yucore-cangyuan-media.spec.ts`
- Create: `web/default/e2e/yucore-production-baseline.spec.ts`
- Modify: `web/default/package.json`
- Modify: `web/bun.lock`

- [ ] **Step 1: Add Playwright and write browser acceptance tests**

```powershell
Set-Location web/default
bun add -d @playwright/test
bunx playwright install chromium
```

The media test must select a model, observe only supported controls, submit once,
poll the returned task, reload, and verify the video/image node remains playable
or visible. The baseline test captures home, sign-in/sign-up, console, API keys,
system settings, Infinite Canvas, docs, brand mark, and animation-ready state at
desktop and mobile viewports.

Use isolated local test accounts and authenticated fixtures for protected local
pages. For production comparison, reuse only the browser's already-authorized
session in memory, redact dynamic account data, disable motion during screenshot
capture, and assert animation elements separately. Never save storage state,
cookies, tokens, headers, or account values as test artifacts.

- [ ] **Step 2: Run browser acceptance tests against the implemented local candidate**

Run the current local candidate on a private port and execute:

```powershell
bunx playwright test e2e/yucore-cangyuan-media.spec.ts --project=chromium
```

Expected: PASS. Production behavior has already been driven by the failing pure
logic and component tests in Tasks 8-9; these browser tests add end-to-end
acceptance coverage without introducing additional production code.

- [ ] **Step 3: Run all automated verification**

From repository root:

```powershell
go test -p 2 ./...
Set-Location web/default
bun test src/features/yucore-brand/lib/media-generation.test.ts src/features/yucore-brand/components/yucore-media-controls.test.tsx
bun run i18n:sync
bun run typecheck
bun run lint
bun run format:check
bun run build
bun test ../../scripts/production/cangyuan-media-probe.test.mjs
```

Expected: every command exits 0. Use `-p 2` to avoid the previously observed
Windows paging-file failure under unrestricted Go package parallelism.

- [ ] **Step 4: Build and start the isolated local candidate**

Build from the feature branch, use a new SQLite database and local-only media
configuration, and bind to a private port such as `127.0.0.1:3112`. Confirm the
process does not use the production database or production Caddy network.

- [ ] **Step 5: Configure local channels only**

Create a separate upstream `IMAGE` credential and use it only for the two local
image channels. Keep the upstream `VIDEO` credential on local video channels.
Configure family channels and groups exactly as the design specifies. Merge
missing model prices from the manifest; do not overwrite an existing global
price. Merge per-request model IDs into `TASK_PRICE_PATCH`; exclude every
`per_second` entry.

Put image routes in `生图按次`, `多模态创作`, and `下游多模态`; put video routes in
`多模态创作` and `下游多模态`. Remove `gpt-image-2-1k` and
`gpt-image-2-4k` only from the two isolated Cangyuan image channels. Do not
remove their global prices, other providers, or abilities.

Before any paid request, calculate and record the expected local charge:

```text
per request: base price * 1.2
per second: base price * validated duration * 1.2
```

- [ ] **Step 6: Run Playwright UI and brand comparison**

```powershell
Set-Location web/default
$env:YUAPI_LOCAL_URL='http://127.0.0.1:3112'
$env:YUAPI_PRODUCTION_URL='https://yuaiapi.com'
bunx playwright test e2e/yucore-production-baseline.spec.ts e2e/yucore-cangyuan-media.spec.ts --project=chromium
```

Expected: no unexplained brand/layout diff; changes are limited to approved media
controls and documentation. Inspect screenshots at both desktop and mobile sizes.

- [ ] **Step 7: Run paid image and video validation**

Run the probe first against local YuAPI. Validate all seven retained image routes
and all 33 account-visible video routes. Use minimum valid duration/resolution
for video tests, but cover reference image, reference video, reference audio,
first/last frames, generated audio, and video-to-video at least once.

For each task compare:

- exactly one creation POST;
- same-ID polling only;
- upstream completion and downloadable content;
- expected upstream debit;
- expected YuAPI quota using the frozen base price and `1.2` ratio;
- no refund after accepted/ambiguous submission;
- playable/restorable Infinite Canvas node.

For each pending model, first use the probe's one-case direct-upstream audit mode
to establish completion and debit without involving YuAPI billing. If it succeeds
and its debit is known, add the verified cost, change availability to `enabled`,
then repeat the automated and paid test through local YuAPI and commit that
isolated manifest change. Otherwise keep it `probe` and unavailable.

- [ ] **Step 8: Inspect and retain the six quota-card deliverables**

Open all candidates, select the best exact-text result for each amount, and keep
the six final files under `output/cangyuan-media-20260811/`. Check that the amount
is correct, Chinese text is legible, the supplied reference is not overwritten,
and no watermark or credential-bearing URL appears.

- [ ] **Step 9: Commit browser-test infrastructure only**

```powershell
git add web/default/playwright.config.ts web/default/e2e/yucore-cangyuan-media.spec.ts web/default/e2e/yucore-production-baseline.spec.ts web/default/package.json web/bun.lock
git commit -m "test: cover cangyuan media workflows"
```

Do not add `output/`, screenshots containing account data, local databases, local
environment files, or probe audit logs.

- [ ] **Step 10: Stop for user review**

Provide the local URL, the redacted model pass/fail and billing comparison, the
six local image paths, and the Playwright screenshot directory. Do not prepare
or switch production until the user explicitly confirms the local candidate.

## Production Gate After Separate Approval

This is not part of the implementation execution until the user approves the
local candidate.

1. Build a production candidate from the reviewed commit and bind it only to
   `127.0.0.1` on the server.
2. Preserve the running image, containers, Caddy config, channel export, option
   export, and rollback metadata.
3. Verify candidate health, UI, DB compatibility, channel model lists, task
   creation, polling, billing, and one low-cost real asset without traffic.
4. Ask for explicit traffic-switch confirmation.
5. Switch with the shortest possible Caddy reload, run immediate smoke checks,
   and watch errors/billing/task completion.
6. On any UI, database, billing, task, or provider regression, restore the prior
   traffic target and channel/options configuration immediately. Do not restore
   an old database snapshot and do not delete retained images or containers.
