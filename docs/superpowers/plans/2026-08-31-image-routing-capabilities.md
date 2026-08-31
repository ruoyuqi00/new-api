# Official Image Routing Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route official image model names to a verified compatible channel while preserving requested resolution and aspect ratio.

**Architecture:** Normalize the request once into a canonical model, resolution tier, exact dimensions, and shape requirement. Extend the existing channel `settings` JSON with validated capability metadata, apply the same filter in memory-cache and database selection before priority/weight selection, then reuse the existing `model_mapping` and adapter conversion paths. Billing remains owned by the frozen image-resolution price quote and is not changed by this plan.

**Tech Stack:** Go 1.22+, Gin, GORM v2, existing channel cache/pool/affinity/retry services, existing OpenAI/Gemini/advanced-custom adapters, Testify, and the `web/default` API only where model metadata needs to reflect canonical names.

**Spec:** `docs/superpowers/specs/2026-08-31-image-routing-capabilities-design.md`

## Global Constraints

- The public canonical name is the official model name; existing resolution aliases remain accepted.
- A channel with unknown image capability is eligible for square requests only.
- No lower-resolution fallback is allowed for a higher-tier request.
- Apply capability filtering before existing priority, weight, affinity, pool lease, and retry ordering.
- `OriginModelName` remains the customer model and `UpstreamModelName` remains the mapped provider model.
- Explicit customer width, height, or ratio may not be replaced by a channel default or parameter override.
- Do not change billing expressions, task billing, violation fees, channel cooldown semantics, or actual-response-model recording.
- Never expose upstream URL, credentials, pool identity, or raw provider error content.
- Store capability data in the existing `channels.settings` JSON; do not add a database column.
- Use `common.Marshal` and `common.Unmarshal` wrappers in new business code.
- Test SQLite, MySQL, and PostgreSQL-compatible GORM expressions; do not add dialect-specific SQL.
- Implement and verify in the isolated worktree; do not modify production or Caddy.

---

### Task 1: Capability Schema and Validation

**Files:**
- Modify: `dto/channel_settings.go`
- Modify: `model/channel_image_selection.go`
- Test: `model/channel_image_selection_test.go`

**Interfaces:**
- Consumes: existing `ChannelOtherSettings` JSON and `operation_setting.ResolveImageResolutionPrice`.
- Produces: `ImageModelCapability`, `ImageCapabilityShape`, `ValidateImageCapabilitySettings`, and `ChannelImageCapabilityForModel`.

- [ ] **Step 1: Write failing validation tests**

Add table tests for `square`, `any`, `ratio`, `pending`, empty values, per-model `max_tier`, invalid tiers, invalid shapes, and a model cap above `4k`:

```go
func TestValidateImageCapabilitySettings(t *testing.T) {
	tests := []struct {
		name string
		settings dto.ChannelOtherSettings
		wantErr bool
	}{
		{name: "verified exact", settings: dto.ChannelOtherSettings{ImageDimensionSupport: "any"}, wantErr: false},
		{name: "verified ratio", settings: dto.ChannelOtherSettings{ImageDimensionSupport: "ratio"}, wantErr: false},
		{name: "unknown remains valid", settings: dto.ChannelOtherSettings{ImageDimensionSupport: "pending"}, wantErr: false},
		{name: "bad support", settings: dto.ChannelOtherSettings{ImageDimensionSupport: "diagonal"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageCapabilitySettings(tt.settings)
			if tt.wantErr { require.Error(t, err) } else { require.NoError(t, err) }
		})
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `go test ./model -run TestValidateImageCapabilitySettings -count=1`

Expected: FAIL because the capability type and validator do not exist.

- [ ] **Step 3: Add capability fields and validation**

Add these JSON-compatible types to `dto/channel_settings.go`:

```go
type ImageCapabilityShape string

const (
	ImageCapabilityShapeExact ImageCapabilityShape = "exact"
	ImageCapabilityShapeRatio ImageCapabilityShape = "ratio"
)

type ImageModelCapability struct {
	MaxTier string `json:"max_tier,omitempty"`
	Shape   ImageCapabilityShape `json:"shape,omitempty"`
}
```

Add `ImageModelCapabilities map[string]ImageModelCapability` to
`ChannelOtherSettings`. Validate support values, canonicalize model keys by
trim/lowercase, require `max_tier` in `1k`, `2k`, `4k`, and require `shape` in
`exact`, `ratio` when a per-model entry is present. Keep `pending` and
`unknown` valid as square-only states.

- [ ] **Step 4: Implement inherited capability resolution**

In `model/channel_image_selection.go`, implement:

```go
func ChannelImageCapabilityForModel(channel *Channel, modelName string) ImageModelCapability
```

The function reads `GetOtherSettings`, defaults empty/unknown to
`MaxTier: 1k` and square-only, then overlays the canonical model entry when it
exists. Invalid settings return the same square-only capability and emit the
existing sanitized system error path.

- [ ] **Step 5: Run model tests**

Run: `go test ./model -run 'TestValidateImageCapabilitySettings|TestChannelImageCapabilityForModel' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add dto/channel_settings.go model/channel_image_selection.go model/channel_image_selection_test.go
git commit -m "feat: validate image channel capabilities"
```

### Task 2: Canonical Request Requirements

**Files:**
- Modify: `model/channel_image_selection.go`
- Modify: `middleware/distributor.go`
- Modify: `service/channel_select.go`
- Test: `model/channel_image_selection_test.go`
- Test: `service/channel_select_retry_test.go`

**Interfaces:**
- Consumes: `ImageRequest`, `operation_setting.ResolveImageResolutionPrice`, and existing retry parameters.
- Produces: an `ImageSelectionRequirements` containing canonical model, tier, exact dimensions, and shape requirement.

- [ ] **Step 1: Write failing requirement tests**

Cover `650x1024 -> 1k`, `1536x1024 -> 2k`, `4k + 3:2`, explicit aliases, canonical model extraction, and malformed dimensions returning an error before channel selection.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./model ./service -run 'Test.*Image.*Requirement|TestRetryParamPropagatesImageSelectionRequirements' -count=1`

Expected: FAIL for the missing canonical fields/constructor.

- [ ] **Step 3: Implement one constructor**

Add:

```go
func BuildImageSelectionRequirements(request *dto.ImageRequest) (*ImageSelectionRequirements, error)
```

Call the shared pricing resolver for canonical model and tier. Parse exact
dimensions with the same separators accepted by pricing. For an aspect-ratio
only request, retain the tier and mark `ShapeRequired` as ratio. Return a
validation error for malformed or over-limit sizes; do not silently return a
cheaper tier.

- [ ] **Step 4: Use the constructor in the distributor**

Replace the current string-only construction at
`middleware/distributor.go:imageSelectionRequirementsForRequest` with the
constructor. Preserve the existing `nil` result for non-image paths and pass
the returned pointer through `RetryParam` on every retry.

- [ ] **Step 5: Update retry tests**

Assert retries retain canonical model, tier, exact dimensions, and shape
requirement after a failed channel; a retry cannot clear these fields.

- [ ] **Step 6: Run tests**

Run: `go test ./model ./service ./middleware -run 'Test.*Image.*|TestRetryParam.*Image' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add model/channel_image_selection.go middleware/distributor.go service/channel_select.go model/channel_image_selection_test.go service/channel_select_retry_test.go
git commit -m "feat: normalize image routing requirements"
```

### Task 3: Capability-Aware Candidate Lookup

**Files:**
- Modify: `model/channel_pool_runtime.go`
- Modify: `model/channel_cache.go`
- Modify: `model/ability.go`
- Test: `model/channel_image_selection_test.go`
- Test: `model/channel_pool_runtime_test.go`

**Interfaces:**
- Consumes: `ImageSelectionRequirements` and existing `ChannelSelectionOptions`.
- Produces: identical priority/weight selection with capability filtering in both memory and database paths.

- [ ] **Step 1: Write failing lookup tests**

Add cases where a group contains a square-only high-priority channel and an
`any` lower-priority channel. Assert a non-square request selects the compatible
channel; a square request keeps the high-priority channel. Add a case where an
official model must find a legacy `-1k/-2k/-4k` ability.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./model -run 'Test.*Selection.*Image|Test.*Capability.*Channel' -count=1`

Expected: FAIL because candidate lookup only uses the exact public model key.

- [ ] **Step 3: Add canonical/alias candidate keys**

Implement:

```go
func ImageModelSelectionNames(modelName string, tier ImageResolutionTier) []string
```

Return the canonical name first, followed by the requested tier alias, with
deduplication. Update `GetRandomSatisfiedChannelWithOptions` and
`GetChannelWithOptions` to search each key in order, apply path filtering,
`filterChannelsBySelectionOptions`, pool availability, then existing priority
and weight logic. Never merge incompatible candidate sets across model names.

- [ ] **Step 4: Make unknown capabilities square-only**

Change `ChannelSupportsImageRequest` so a non-square/ratio requirement returns
false for empty, `pending`, `unknown`, or `square` capability. For a verified
capability, require the requested tier to be no greater than `MaxTier`; require
`exact` for exact dimensions and permit `ratio` for ratio-only requests.

- [ ] **Step 5: Preserve affinity semantics**

Keep the existing affinity channel when it is compatible. When it is
incompatible only for the current image shape, skip it for this request without
clearing the stored affinity record. Add a regression test in the distributor
selection tests.

- [ ] **Step 6: Run memory/database selection tests**

Run: `go test ./model ./middleware -run 'Test.*Selection.*|Test.*Affinity.*Image' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add model/channel_pool_runtime.go model/channel_cache.go model/ability.go model/channel_image_selection.go model/channel_image_selection_test.go model/channel_pool_runtime_test.go middleware/distributor.go middleware/*test.go
git commit -m "feat: route image requests by verified capability"
```

### Task 4: Official-to-Upstream Mapping

**Files:**
- Modify: `relay/helper/model_mapped.go`
- Modify: `relay/common/relay_info.go`
- Test: `relay/helper/model_mapped_test.go`

**Interfaces:**
- Consumes: selected channel `model_mapping`, canonical request model, and legacy aliases.
- Produces: `OriginModelName` as customer model and `UpstreamModelName` as mapped provider model.

- [ ] **Step 1: Write failing mapping tests**

Test canonical mapping (`gpt-image-2 -> provider-image-v3`), legacy alias fallback,
provider prefix normalization, mapping cycle rejection, and no mapping keeping
the canonical upstream name.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./relay/helper -run 'TestModelMapped.*Image' -count=1`

Expected: FAIL for canonical-to-legacy fallback behavior.

- [ ] **Step 3: Implement mapping precedence**

Replace direct JSON decoding in the modified path with `common.Unmarshal`.
Resolve mapping in this order: canonical model key, requested legacy alias key,
then existing chain resolution. Keep cycle detection and set
`info.IsModelMapped` only when the final name differs. Never overwrite
`OriginModelName` with the upstream name.

- [ ] **Step 4: Add customer-model audit assertions**

Assert `ClientResponseModelName()` returns the customer model while logs and
actual-response-model handling remain unchanged.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./relay/helper ./relay/common -run 'TestModelMapped.*Image|Test.*ClientResponseModelName' -count=1`

```bash
git add relay/helper/model_mapped.go relay/common/relay_info.go relay/helper/model_mapped_test.go
git commit -m "feat: map canonical image models per channel"
```

### Task 5: Provider-Native Dimension Conversion

**Files:**
- Modify: `relay/image_handler.go`
- Modify: `relay/channel/openai/adaptor.go`
- Modify: `relay/channel/advancedcustom/adaptor.go`
- Modify: `relay/channel/gemini/adaptor.go`
- Test: `relay/image_handler_test.go`
- Test: `relay/channel/openai/adaptor_test.go`
- Test: `relay/channel/advancedcustom/adaptor_test.go`
- Test: `relay/channel/gemini/adaptor_test.go`

**Interfaces:**
- Consumes: normalized request requirements, `UpstreamModelName`, and selected channel capability.
- Produces: provider payloads with exact dimensions or native tier/ratio fields.

- [ ] **Step 1: Write failing payload tests**

Add OpenAI-compatible tests for `650x1024`, `2k + 3:2`, and a mapped upstream
model. Add Gemini/Imagen tests that assert `imageSize` is `1K`/`2K` and the
native `aspectRatio` is preserved. Add a test that unsupported `4k` is rejected
before adapter invocation.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./relay ./relay/channel/openai ./relay/channel/gemini ./relay/channel/advancedcustom -run 'Test.*Image.*(Dimension|Payload|Ratio)' -count=1`

Expected: FAIL where adapters currently overwrite or infer only fixed square sizes.

- [ ] **Step 3: Centralize exact-size normalization**

Keep `preserveRequestedImageDimensions` as the final guard after parameter
overrides. Extend it to use normalized requirements and only restore fields the
selected capability declares. Do not restore `size` for a ratio-only adapter;
restore the native ratio/tier fields instead.

- [ ] **Step 4: Update OpenAI-compatible adapters**

Send normalized `WxH` for exact capabilities and retain the request's model
field as `UpstreamModelName`. Parameter overrides can fill missing values but
cannot replace explicit dimensions or ratios.

- [ ] **Step 5: Update Imagen/Gemini conversion**

Map the selected tier to `imageSize` and pass the original ratio through
`aspectRatio`. Reject a tier or ratio not declared by the model capability.

- [ ] **Step 6: Run adaptor tests**

Run: `go test ./relay ./relay/channel/openai ./relay/channel/gemini ./relay/channel/advancedcustom -run 'Test.*Image' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add relay/image_handler.go relay/image_handler_test.go relay/channel/openai/adaptor.go relay/channel/advancedcustom/adaptor.go relay/channel/gemini/adaptor.go relay/channel/*/*image*test.go
git commit -m "fix: preserve image dimensions in provider payloads"
```

### Task 6: Public Metadata, Errors, and Regression Coverage

**Files:**
- Modify: `model/pricing.go`
- Modify: `controller/model_list.go` or the existing model-list controller file
- Modify: `service/log_info_generate.go`
- Modify: `web/default/src/features/yucore-brand/lib/media-catalog.ts` only if API metadata requires a type update
- Test: `controller/pricing_test.go`
- Test: `service/log_info_generate_test.go`
- Test: `web/default/src/features/yucore-brand/lib/media-catalog.test.ts` when touched

**Interfaces:**
- Consumes: canonical model metadata and selection decisions.
- Produces: official names in pricing/model metadata, non-sensitive audit fields, and stable public errors.

- [ ] **Step 1: Write failing API/log tests**

Assert `/api/pricing` includes canonical image names and no upstream mapping;
legacy aliases, if returned, point at the same policy. Assert a no-compatible
channel error contains no channel name, URL, credential, or provider body.

- [ ] **Step 2: Implement metadata projection**

Use the existing pricing/model list projection to expose canonical names and
capability metadata only. Keep aliases backward-compatible without publishing
provider mappings.

- [ ] **Step 3: Add internal audit metadata**

Add normalized size, tier, and capability decision to the existing non-sensitive
log `other_info`. Do not add raw request bodies or channel credentials.

- [ ] **Step 4: Run regression suites**

Run: `go test ./controller ./service ./model ./relay/... -run 'Test.*(Pricing|Billing|Affinity|Violation|ActualResponse|Image)' -count=1`

Expected: PASS with existing billing, affinity, violation-fee, and actual-response-model behavior unchanged.

- [ ] **Step 5: Build frontend and run full backend tests**

Run from `web/default`: `bun run build`

Run from repository root: `go test ./... -count=1`

Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add model/pricing.go controller service/log_info_generate.go web/default/src/features/yucore-brand/lib/media-catalog.ts web/default/src/features/yucore-brand/lib/media-catalog.test.ts
git commit -m "feat: expose canonical image routing metadata"
```

### Task 7: Local Candidate Verification and Handoff

**Files:**
- Create: `docs/superpowers/handoffs/2026-08-31-image-routing-capabilities-local-verification.md`

- [ ] **Step 1: Start one isolated local instance**

Use an independent database and loopback-only port; do not reuse production
cookies, database files, or Caddy configuration.

- [ ] **Step 2: Verify API behavior**

Check official model names, legacy alias compatibility, exact dimensions,
1K/2K/4K rejection boundaries, and generic no-compatible-channel errors.

- [ ] **Step 3: Verify mocked upstream payloads**

Use deterministic mocked upstreams to confirm OpenAI exact `WxH`, Gemini native
`imageSize`/`aspectRatio`, mapped upstream model names, and unchanged customer
model in responses/logs. Do not issue paid production requests.

- [ ] **Step 4: Verify branded UI**

Use Playwright to inspect sign-in, home, studio, canvas, pricing, usage logs,
and admin channel pages. Confirm the existing production-derived branding and
no new upstream details in the UI.

- [ ] **Step 5: Record results and stop for approval**

Write the image hashes, test commands, payload assertions, and rollback commit
to the handoff file. Ask for explicit production approval; do not deploy from
this plan automatically.

- [ ] **Step 6: Commit handoff**

```bash
git add docs/superpowers/handoffs/2026-08-31-image-routing-capabilities-local-verification.md
git commit -m "docs: record local image routing verification"
```
