# Official Image Resolution Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one validated resolution-price policy per official image model, classify arbitrary image dimensions into 1K/2K/4K billing tiers, and freeze that price for pre-consume, settlement, pricing API, and audit logs without changing routing or upstream payloads.

**Architecture:** A new `operation_setting` module owns immutable, atomically swapped resolution policies and a pure resolver. `ModelPriceHelper` invokes the resolver before quota pre-consume for image requests and stores the complete quote in `types.PriceData`; existing fixed-price settlement then reuses that snapshot, so group ratio and image count are each applied exactly once. `/api/pricing` and consume logs expose only non-sensitive policy and quote metadata, while channel selection, model mapping, provider adapters, task billing, billing expressions, affinity, violation fees, retry, and actual-response-model handling remain unchanged.

**Tech Stack:** Go 1.22+, Gin, GORM v2, `sync/atomic`, `github.com/stretchr/testify`, existing `common` JSON wrappers and option/config infrastructure; SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+ compatible.

**Spec:** `docs/superpowers/specs/2026-08-31-official-image-resolution-pricing-design.md`

## Global Constraints

- This is the price phase only: do not remove legacy aliases, alter channel mappings, modify upstream request payloads, or change provider capability selection.
- Classify by the smallest square boundary: both edges `<=1024` is 1K, both `<=2048` is 2K, both `<=4096` is 4K, and either edge `>4096` is rejected.
- Accept `x`, `X`, `*`, and `×` as exact-dimension separators; accept explicit `1k`, `2k`, `4k`; empty or `auto` uses the configured default tier.
- Reject malformed, zero, negative, over-4K, incomplete-policy, non-finite-price, and non-monotonic-price inputs before any upstream request and before quota consumption.
- Initial base per-image prices are exactly: `gpt-image-2` = `0.01/0.04/0.045`; `nano-banana-pro` = `0.086666666667/0.108333333333/0.161416666667`; `nano-banana2` = `0.063916666667/0.086666666667/0.13` for 1K/2K/4K.
- Apply the existing group ratio exactly once and request image count `n` exactly once.
- Legacy alias suffixes are minimum tiers; actual tier is `max(alias minimum, request size tier)`.
- Models without a resolution policy retain their current pricing behavior.
- Pre-consume and settlement must use one frozen request snapshot; an option update during a request must not change its final charge.
- Do not change billing expressions, task billing, channel affinity, violation fees, actual response model, retry behavior, or media stream semantics.
- All JSON marshal/unmarshal in new business code must use `common.Marshal`, `common.Unmarshal`, or `common.UnmarshalJsonStr`.
- Do not add a database migration; store the complete map as one option value and replace it atomically.
- Do not make real paid completion or image-generation requests in this phase.
- Do not change production traffic, Caddy, the production container, production prices, or production database during implementation and local verification.

---

## File Map

- Create `setting/operation_setting/image_resolution_pricing.go`: policy types, defaults, validation, alias normalization, size parsing, immutable index, quote resolution, and API metadata projection.
- Create `setting/operation_setting/image_resolution_pricing_test.go`: resolver boundaries, separators, aliases, defaults, invalid values, policy validation, atomic replacement, and old-index preservation.
- Modify `model/option.go`: validate the one complete option value before persistence, rebuild the immutable index after config load/update, and invalidate pricing caches.
- Create `model/option_image_resolution_pricing_test.go`: option persistence and rollback behavior using the existing database fixture.
- Modify `types/price_data.go`: add the frozen non-sensitive image-resolution quote fields.
- Modify `relay/helper/price.go`: resolve official/alias image prices before pre-consume and skip legacy `ImagePriceRatio` only when a policy matched.
- Modify `relay/helper/price_test.go`: fixed-price quota, `n`, group ratio, alias floor, fallback, validation-before-preconsume, and snapshot tests.
- Modify `service/text_quota.go`: reuse the frozen unit price for final direct-image and Responses image-tool settlement and emit quote metadata.
- Modify `service/tool_billing.go`: remove the divergent `gpt-image-2` hardcoded price from the standalone calculation path by using the shared resolver.
- Modify `service/text_quota_test.go`: frozen settlement, group/count-once, tool surcharge, and zero-usage image semantics.
- Create `service/tool_billing_test.go`: shared-resolution-price behavior and legacy fallback.
- Modify `model/pricing.go`: add optional policy metadata and project canonical policy values onto configured aliases.
- Modify `controller/model_list_test.go`: `/api/pricing` data contract for canonical policy and alias minimum tiers.
- Modify `service/log_info_generate.go`: add non-sensitive frozen quote fields to `other_info`.
- Modify `service/log_info_generate_test.go`: audit metadata and upstream-secret non-exposure regression.
- Modify `setting/operation_setting/tools.go`: retain GPT Image 1 behavior but remove the conflicting GPT Image 2 tier table after all callers use the shared resolver.
- Modify `dto/openai_image_test.go`: retain legacy behavior only for unconfigured models and remove assertions that encode the replaced GPT Image 2 ratio table.

---

### Task 1: Resolution Policy, Validation, and Resolver

**Files:**
- Create: `setting/operation_setting/image_resolution_pricing.go`
- Create: `setting/operation_setting/image_resolution_pricing_test.go`

**Interfaces:**
- Produces: `ImageResolutionTier`, `ImageResolutionPricePolicy`, `ImageResolutionPriceQuote`, and `ImageResolutionPricingMetadata`.
- Produces: `ResolveImageResolutionPrice(modelName, size string) (ImageResolutionPriceQuote, bool, error)`.
- Produces: `ValidateImageResolutionPriceJSONString(value string) error`.
- Produces: `RebuildImageResolutionPriceIndex()` and `ImageResolutionPriceSetting2JSONString() string`.
- Produces: `GetImageResolutionPricingMetadata(modelName string) (ImageResolutionPricingMetadata, bool)`.

- [ ] **Step 1: Write the failing resolver boundary and separator tests**

```go
func TestResolveImageResolutionPriceClassifiesDimensions(t *testing.T) {
	tests := []struct {
		name string
		size string
		want ImageResolutionTier
	}{
		{name: "1k lower rectangle", size: "650x1024", want: ImageResolutionTier1K},
		{name: "1k uppercase separator", size: "1024X650", want: ImageResolutionTier1K},
		{name: "2k star separator", size: "1024*1536", want: ImageResolutionTier2K},
		{name: "2k unicode separator", size: "2048×2048", want: ImageResolutionTier2K},
		{name: "4k rectangle", size: "2048x3072", want: ImageResolutionTier4K},
		{name: "4k narrow", size: "512x4096", want: ImageResolutionTier4K},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, configured, err := ResolveImageResolutionPrice("gpt-image-2", tt.size)
			require.NoError(t, err)
			require.True(t, configured)
			assert.Equal(t, tt.want, quote.Tier)
		})
	}
}
```

- [ ] **Step 2: Write failing tests for explicit/default tiers, alias floors, and provider prefixes**

```go
func TestResolveImageResolutionPriceHonorsDefaultAndAliasMinimum(t *testing.T) {
	tests := []struct {
		model string
		size string
		wantTier ImageResolutionTier
		wantPrice float64
	}{
		{model: "gpt-image-2", size: "", wantTier: ImageResolutionTier1K, wantPrice: 0.01},
		{model: "openai/gpt-image-2", size: "auto", wantTier: ImageResolutionTier1K, wantPrice: 0.01},
		{model: "gpt-image-2-4k", size: "1024x1024", wantTier: ImageResolutionTier4K, wantPrice: 0.045},
		{model: "gpt-image-2-1k", size: "1536x1024", wantTier: ImageResolutionTier2K, wantPrice: 0.04},
		{model: "nano-banana2-2k", size: "1k", wantTier: ImageResolutionTier2K, wantPrice: 0.086666666667},
	}
	for _, tt := range tests {
		quote, configured, err := ResolveImageResolutionPrice(tt.model, tt.size)
		require.NoError(t, err)
		require.True(t, configured)
		assert.Equal(t, tt.wantTier, quote.Tier)
		assert.InDelta(t, tt.wantPrice, quote.UnitPrice, 1e-12)
	}
}
```

- [ ] **Step 3: Write failing invalid-input and unconfigured-model tests**

```go
func TestResolveImageResolutionPriceRejectsInvalidConfiguredSizes(t *testing.T) {
	for _, size := range []string{"0x1024", "-1x1024", "1024", "1024x", "1024xabc", "4097x512", "512x4097", "8k"} {
		_, configured, err := ResolveImageResolutionPrice("gpt-image-2", size)
		require.True(t, configured, size)
		require.Error(t, err, size)
	}
	_, configured, err := ResolveImageResolutionPrice("unmanaged-image-model", "4097x512")
	require.NoError(t, err)
	assert.False(t, configured)
}
```

- [ ] **Step 4: Write failing policy validation and old-index-preservation tests**

```go
func TestRebuildImageResolutionPriceIndexRejectsWholeInvalidPolicy(t *testing.T) {
	original := ImageResolutionPriceSetting2JSONString()
	t.Cleanup(func() {
		require.NoError(t, replaceImageResolutionPriceSettingForTest(original))
	})

	before, configured, err := ResolveImageResolutionPrice("gpt-image-2", "2k")
	require.NoError(t, err)
	require.True(t, configured)

	invalid := `{"gpt-image-2":{"prices":{"1k":0.01,"2k":0.005,"4k":0.045},"default_tier":"1k"}}`
	require.Error(t, ValidateImageResolutionPriceJSONString(invalid))
	after, configured, err := ResolveImageResolutionPrice("gpt-image-2", "2k")
	require.NoError(t, err)
	require.True(t, configured)
	assert.Equal(t, before, after)
}
```

- [ ] **Step 5: Run the focused tests and verify they fail**

Run: `go test ./setting/operation_setting -run 'TestResolveImageResolutionPrice|TestRebuildImageResolutionPriceIndex' -count=1`

Expected: FAIL because the policy types and resolver do not exist.

- [ ] **Step 6: Implement the policy types, defaults, parser, validator, and atomic index**

```go
type ImageResolutionTier string

const (
	ImageResolutionTier1K ImageResolutionTier = "1k"
	ImageResolutionTier2K ImageResolutionTier = "2k"
	ImageResolutionTier4K ImageResolutionTier = "4k"
)

type ImageResolutionPricePolicy struct {
	Prices      map[ImageResolutionTier]float64 `json:"prices"`
	DefaultTier ImageResolutionTier            `json:"default_tier"`
}

type ImageResolutionPriceQuote struct {
	RequestedModel   string
	PricingModel     string
	RequestedSize    string
	NormalizedSize   string
	Tier             ImageResolutionTier
	AliasMinimumTier ImageResolutionTier
	UnitPrice        float64
}

type ImageResolutionPricingMetadata struct {
	PricingModel     string                              `json:"pricing_model"`
	DefaultTier      ImageResolutionTier                 `json:"default_tier"`
	Prices           map[ImageResolutionTier]float64     `json:"prices"`
	AliasMinimumTier ImageResolutionTier                 `json:"alias_minimum_tier,omitempty"`
}

type ImageResolutionPriceSetting struct {
	Models map[string]ImageResolutionPricePolicy `json:"models"`
}

type imageResolutionPriceIndex struct {
	models map[string]ImageResolutionPricePolicy
}

var imageResolutionPriceIndexValue atomic.Pointer[imageResolutionPriceIndex]
```

Implementation rules:

- Register `image_resolution_price_setting` with `config.GlobalConfig.Register`.
- Seed exactly the three initial model policies from Global Constraints.
- Normalize model names by trimming, lowercasing, and stripping only a provider prefix ending in `/`.
- Recognize `-1k`, `-2k`, and `-4k` as alias suffixes only when the remaining canonical name exists in the immutable index.
- Parse exact dimensions with `strconv.Atoi`; do not accept partial parses or floats.
- Validate every policy as a whole: exactly three positive finite prices, valid default, and `1K <= 2K <= 4K`.
- Deep-copy maps into the new index and call `Store` only after the complete copy validates.
- `ResolveImageResolutionPrice` returns `(zero, false, nil)` for unconfigured models and `(zero, true, error)` for invalid sizes on configured models.
- `replaceImageResolutionPriceSettingForTest` is test-only in `_test.go`; it calls `common.UnmarshalJsonStr`, validates, swaps the package config value, and rebuilds.

- [ ] **Step 7: Run the focused tests and verify they pass**

Run: `go test ./setting/operation_setting -run 'TestResolveImageResolutionPrice|TestRebuildImageResolutionPriceIndex' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the resolver**

```bash
git add setting/operation_setting/image_resolution_pricing.go setting/operation_setting/image_resolution_pricing_test.go
git commit -m "feat: add image resolution price policies"
```

---

### Task 2: Atomic Option Persistence and Pricing Cache Invalidation

**Files:**
- Modify: `model/option.go:228-313,645-679`
- Create: `model/option_image_resolution_pricing_test.go`

**Interfaces:**
- Consumes: `operation_setting.ValidateImageResolutionPriceJSONString` and `operation_setting.RebuildImageResolutionPriceIndex` from Task 1.
- Produces: atomic database-plus-runtime replacement for option key `image_resolution_price_setting.models`.

- [ ] **Step 1: Write the failing valid-update and invalid-rollback tests**

```go
func TestUpdateOptionImageResolutionPricingIsValidatedBeforePersistence(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:option_image_resolution?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	valid := `{"gpt-image-2":{"prices":{"1k":0.02,"2k":0.04,"4k":0.08},"default_tier":"1k"}}`
	require.NoError(t, UpdateOption("image_resolution_price_setting.models", valid))

	var stored Option
	require.NoError(t, DB.First(&stored, "key = ?", "image_resolution_price_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)

	invalid := `{"gpt-image-2":{"prices":{"1k":0.02,"2k":0.01,"4k":0.08},"default_tier":"1k"}}`
	require.Error(t, UpdateOption("image_resolution_price_setting.models", invalid))
	require.NoError(t, DB.First(&stored, "key = ?", "image_resolution_price_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)
}
```

- [ ] **Step 2: Write the failing bulk-update validation test**

```go
func TestUpdateOptionsBulkRejectsInvalidImageResolutionPricingBeforeTransaction(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:option_image_resolution_bulk?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	err := UpdateOptionsBulk(map[string]string{
		"SystemName": "must-not-commit",
		"image_resolution_price_setting.models": `{"gpt-image-2":{"prices":{"1k":0.01},"default_tier":"1k"}}`,
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key = ? AND value = ?", "SystemName", "must-not-commit").Count(&count).Error)
	assert.Zero(t, count)
}
```

- [ ] **Step 3: Run the focused tests and verify they fail**

Run: `go test ./model -run 'TestUpdateOptionImageResolutionPricing|TestUpdateOptionsBulkRejectsInvalidImageResolutionPricing' -count=1`

Expected: FAIL because the new option is persisted before it is validated and the immutable index is not rebuilt.

- [ ] **Step 4: Add one shared pre-persistence validator and the config post-update hook**

```go
func validateOptionValue(key, value string) error {
	switch key {
	case "yucore_media.model_capabilities":
		return validateYucoreMediaModelCapabilitiesForConfig(value)
	case "SensitiveInputRetentionDays":
		_, err := setting.ParseSensitiveInputRetentionDays(value)
		return err
	case "image_resolution_price_setting.models":
		return operation_setting.ValidateImageResolutionPriceJSONString(value)
	default:
		return nil
	}
}
```

Call this function before any write in `UpdateOption`, validate all supplied values before entering the GORM transaction in `UpdateOptionsBulk`, and call it at the start of `updateOptionMap` for database reload safety. Extend `handleConfigUpdate`:

```go
} else if configName == "image_resolution_price_setting" {
	operation_setting.RebuildImageResolutionPriceIndex()
	InvalidatePricingCache()
}
```

Do not add raw SQL or a migration.

- [ ] **Step 5: Run the focused tests and existing option/config regressions**

Run: `go test ./model -run 'TestUpdateOptionImageResolutionPricing|TestUpdateOptionsBulkRejectsInvalidImageResolutionPricing|TestUpdateOptionsBulk' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the option integration**

```bash
git add model/option.go model/option_image_resolution_pricing_test.go
git commit -m "feat: persist image resolution prices atomically"
```

---

### Task 3: Freeze Resolution Quotes in Pre-Consume Pricing

**Files:**
- Modify: `types/price_data.go:11-29`
- Modify: `relay/helper/price.go:92-205`
- Modify: `relay/helper/price_test.go`
- Modify: `dto/openai_image_test.go`

**Interfaces:**
- Consumes: `ResolveImageResolutionPrice` from Task 1.
- Produces: the frozen fields `ImageResolutionPricingModel`, `ImageResolutionRequestedSize`, `ImageResolutionTier`, `ImageResolutionUnitPrice`, and `ImageResolutionImageCount` on `types.PriceData`.

- [ ] **Step 1: Write the failing official-model quota and frozen-field test**

```go
func TestModelPriceHelperUsesResolutionPolicyAndFreezesQuote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"image-test":0.3}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	n := uint(2)
	request := &dto.ImageRequest{Model: "gpt-image-2", Size: "650x1024", N: &n}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		UserGroup: "image-test",
		UsingGroup: "image-test",
		Request: request,
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	assert.True(t, priceData.UsePrice)
	assert.Equal(t, "gpt-image-2", priceData.ImageResolutionPricingModel)
	assert.Equal(t, "1k", priceData.ImageResolutionTier)
	assert.InDelta(t, 0.01, priceData.ImageResolutionUnitPrice, 1e-12)
	assert.Equal(t, 2, priceData.ImageResolutionImageCount)
	assert.Equal(t, int(math.Round(0.01*common.QuotaPerUnit*0.3*2)), priceData.QuotaToPreConsume)
}
```

- [ ] **Step 2: Write failing alias-floor, invalid-before-preconsume, and fallback tests**

```go
func TestModelPriceHelperResolutionAliasesUseMinimumTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	n := uint(1)
	request := &dto.ImageRequest{Model: "gpt-image-2-4k", Size: "1024x1024", N: &n}
	info := &relaycommon.RelayInfo{OriginModelName: request.Model, UserGroup: "default", UsingGroup: "default", Request: request}
	priceData, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	assert.Equal(t, "4k", priceData.ImageResolutionTier)
	assert.InDelta(t, 0.045, priceData.ModelPrice, 1e-12)
}

func TestModelPriceHelperRejectsInvalidConfiguredImageSize(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	request := &dto.ImageRequest{Model: "gpt-image-2", Size: "4097x512"}
	info := &relaycommon.RelayInfo{OriginModelName: request.Model, UserGroup: "default", UsingGroup: "default", Request: request}
	_, err := ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.ErrorContains(t, err, "4097x512")
	assert.Zero(t, info.PriceData.QuotaToPreConsume)
}

func TestModelPriceHelperUnconfiguredImageModelKeepsLegacyRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"unmanaged-image-model":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"legacy-image":0.5}`))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "unmanaged-image-model", UserGroup: "legacy-image", UsingGroup: "legacy-image"}
	meta := &types.TokenCountMeta{ImagePriceRatio: 1.5, BillingRatios: map[string]float64{"n": 2}}
	priceData, err := ModelPriceHelper(ctx, info, 0, meta)
	require.NoError(t, err)
	want, err := common.QuotaFromFloatStrict(0.2 * 1.5 * common.QuotaPerUnit * 0.5 * 2)
	require.NoError(t, err)
	assert.Equal(t, want, priceData.QuotaToPreConsume)
	assert.Empty(t, priceData.ImageResolutionPricingModel)
}
```

- [ ] **Step 3: Run the focused tests and verify they fail**

Run: `go test ./relay/helper -run 'TestModelPriceHelper(UsesResolutionPolicy|ResolutionAliases|RejectsInvalidConfigured|UnconfiguredImageModel)' -count=1`

Expected: FAIL because `ModelPriceHelper` still reads only `ModelPrice` and multiplies the old size ratio.

- [ ] **Step 4: Add the immutable quote fields to `PriceData`**

```go
type PriceData struct {
	// existing fields remain unchanged
	ImageResolutionPricingModel  string
	ImageResolutionRequestedSize string
	ImageResolutionTier          string
	ImageResolutionUnitPrice     float64
	ImageResolutionImageCount    int
}
```

Extend `ToSetting` with tier and canonical pricing model only; do not include prompt text, upstream URL, channel key, or account-pool identity.

- [ ] **Step 5: Resolve configured image requests before legacy fixed-price logic**

```go
func resolveRequestImageResolutionPrice(info *relaycommon.RelayInfo) (operation_setting.ImageResolutionPriceQuote, bool, error) {
	var size string
	switch request := info.Request.(type) {
	case *dto.ImageRequest:
		size = request.Size
	case *dto.GeneralOpenAIRequest:
		size = request.Size
	default:
		return operation_setting.ImageResolutionPriceQuote{}, false, nil
	}
	return operation_setting.ResolveImageResolutionPrice(info.OriginModelName, size)
}
```

In `ModelPriceHelper`:

- Resolve before `ratio_setting.GetModelPrice` is used.
- If configured and valid, set `modelPrice = quote.UnitPrice` and `usePrice = true`.
- Skip `meta.ImagePriceRatio` for a configured quote; preserve it for unconfigured models.
- Continue copying `meta.BillingRatios`, so `n` remains the existing one-time multiplier.
- Read the validated `n` from `meta.BillingRatios["n"]`, default to 1, and copy it into `ImageResolutionImageCount` for audit only.
- Store the five frozen fields in `PriceData` before assigning `info.PriceData`.

- [ ] **Step 6: Update DTO tests so they no longer treat the old GPT Image 2 ratio table as authoritative**

Keep tests that ensure `n` remains separate and provider-prefixed models round-trip. Replace GPT Image 2 price-ratio assertions with a regression proving request metadata preserves `Size` on the request and defaults `n` to one; policy tier selection belongs only to `operation_setting` tests.

- [ ] **Step 7: Run focused and adjacent tests**

Run: `go test ./relay/helper ./dto -run 'Test(ModelPriceHelper|ImageRequest|GeneralOpenAIRequest)' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the frozen pre-consume quote**

```bash
git add types/price_data.go relay/helper/price.go relay/helper/price_test.go dto/openai_image_test.go
git commit -m "feat: freeze image resolution prices before billing"
```

---

### Task 4: Reuse the Frozen Quote During Settlement and Tool Billing

**Files:**
- Modify: `service/text_quota.go:90-145,174-180,182-344,686-717`
- Modify: `service/text_quota_test.go`
- Modify: `service/tool_billing.go:10-90`
- Create: `service/tool_billing_test.go`
- Modify: `setting/operation_setting/tools.go:191-216`

**Interfaces:**
- Consumes: frozen `types.PriceData` fields from Task 3.
- Produces: final direct-image and image-generation-tool charges that cannot drift after an option update.

- [ ] **Step 1: Write the failing fixed settlement snapshot test**

```go
func TestCalculateTextQuotaSummaryUsesFrozenImageResolutionPrice(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		PriceData: types.PriceData{
			UsePrice: true,
			ModelPrice: 0.04,
			OtherRatios: map[string]float64{"n": 2},
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3},
			ImageResolutionTier: "2k",
			ImageResolutionUnitPrice: 0.04,
			ImageResolutionImageCount: 2,
		},
	}
	usage := &dto.Usage{PromptTokens: 1, TotalTokens: 1}
	summary := calculateTextQuotaSummary(ctx, info, usage)
	assert.Equal(t, int(math.Round(0.04*common.QuotaPerUnit*0.3*2)), summary.Quota)
}
```

- [ ] **Step 2: Write the failing Responses image-tool frozen-price test**

```go
func TestImageGenerationToolUsesFrozenResolutionUnitPrice(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("image_generation_call", true)
	ctx.Set("image_generation_call_size", "4096x4096")
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		PriceData: types.PriceData{
			UsePrice: true,
			ModelPrice: 0.01,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
			ImageResolutionPricingModel: "gpt-image-2",
			ImageResolutionTier: "1k",
			ImageResolutionUnitPrice: 0.01,
			ImageResolutionImageCount: 1,
		},
	}
	summary := calculateTextQuotaSummary(ctx, info, &dto.Usage{})
	assert.InDelta(t, 0.01, summary.ImageGenerationCallPrice, 1e-12)
}
```

The context deliberately says 4K while the snapshot says 1K. The expected result proves settlement does not re-read or re-resolve a mutable price after pre-consume.

- [ ] **Step 3: Write the failing standalone tool-billing shared-policy test**

```go
func TestComputeToolCallQuotaUsesSharedImageResolutionPolicy(t *testing.T) {
	result, err := ComputeToolCallQuota(ToolCallUsage{
		ModelName: "gpt-image-2-1k",
		ImageGenerationCall: true,
		ImageGenerationSize: "1536x1024",
	}, 0.3)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.InDelta(t, 0.04, result.Items[0].TotalPrice, 1e-12)
	assert.Equal(t, int(math.Round(0.04*common.QuotaPerUnit*0.3)), result.TotalQuota)
}
```

- [ ] **Step 4: Run the focused tests and verify they fail**

Run: `go test ./service -run 'Test(CalculateTextQuotaSummaryUsesFrozenImageResolutionPrice|ImageGenerationToolUsesFrozenResolutionUnitPrice|ComputeToolCallQuotaUsesSharedImageResolutionPolicy)' -count=1`

Expected: FAIL because the tool paths still call the hardcoded `0.05/0.08/0.12` table and `ComputeToolCallQuota` cannot return validation errors.

- [ ] **Step 5: Make settlement use the frozen quote and make standalone tool billing return errors**

Change the standalone signature to:

```go
func ComputeToolCallQuota(usage ToolCallUsage, groupRatio float64) (ToolCallResult, error)
```

For image-generation-only models:

- In `calculateTextToolCallSurcharge`, prefer `relayInfo.PriceData.ImageResolutionUnitPrice`; use the legacy GPT Image 1 table only when no resolution quote exists.
- In `ComputeToolCallQuota`, call `ResolveImageResolutionPrice(usage.ModelName, usage.ImageGenerationSize)` and propagate configured-model validation errors.
- If the resolver reports unconfigured, keep the existing GPT Image 1 behavior.
- Do not multiply the unit price by `n` here; this tool usage represents one recorded call.
- Delete `GetGPTImage2PriceOnceCall` and `gptImage2PriceTier` from `tools.go` only after `rg` confirms no callers remain.

- [ ] **Step 6: Run settlement and tool tests**

Run: `go test ./service ./setting/operation_setting -run 'Test(CalculateTextQuotaSummary|ImageGeneration|ComputeToolCallQuota|ResolveImageResolutionPrice)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit settlement consistency**

```bash
git add service/text_quota.go service/text_quota_test.go service/tool_billing.go service/tool_billing_test.go setting/operation_setting/tools.go
git commit -m "fix: settle image billing from frozen resolution prices"
```

---

### Task 5: Pricing API Metadata and Consume-Log Audit Fields

**Files:**
- Modify: `model/pricing.go:18-39,288-349`
- Modify: `controller/model_list_test.go`
- Modify: `service/log_info_generate.go:36-91`
- Modify: `service/log_info_generate_test.go`

**Interfaces:**
- Consumes: `GetImageResolutionPricingMetadata` from Task 1 and frozen `PriceData` fields from Task 3.
- Produces: optional backward-compatible JSON fields on `model.Pricing` and non-sensitive `other_info` audit fields.

- [ ] **Step 1: Write the failing pricing projection test**

```go
func TestPricingIncludesCanonicalImageResolutionPolicyForAliases(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-image-2-1k", ChannelId: 1, Enabled: true}).Error)
	model.InvalidatePricingCache()

	pricing, ok := pricingByModelName(model.GetPricing())["gpt-image-2-1k"]
	require.True(t, ok)
	require.NotNil(t, pricing.ImageResolutionPricing)
	assert.Equal(t, "gpt-image-2", pricing.ImageResolutionPricing.PricingModel)
	assert.Equal(t, operation_setting.ImageResolutionTier1K, pricing.ImageResolutionPricing.AliasMinimumTier)
	assert.InDelta(t, 0.045, pricing.ImageResolutionPricing.Prices[operation_setting.ImageResolutionTier4K], 1e-12)
}
```

- [ ] **Step 2: Write the failing log audit and privacy test**

```go
func TestGenerateTextOtherInfoIncludesImageResolutionAuditWithoutUpstreamSecrets(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{
		ImageResolutionPricingModel: "gpt-image-2",
		ImageResolutionRequestedSize: "650x1024",
		ImageResolutionTier: "1k",
		ImageResolutionUnitPrice: 0.01,
		ImageResolutionImageCount: 2,
	}}
	other := GenerateTextOtherInfo(ctx, info, 0, 0.3, 0, 0, 0, 0.01, -1)
	assert.Equal(t, "gpt-image-2", other["image_pricing_model"])
	assert.Equal(t, "650x1024", other["image_requested_size"])
	assert.Equal(t, "1k", other["image_resolution_tier"])
	assert.Equal(t, 0.01, other["image_unit_price"])
	assert.Equal(t, 2, other["image_count"])
	assert.NotContains(t, other, "upstream_url")
	assert.NotContains(t, other, "api_key")
}
```

- [ ] **Step 3: Run the focused tests and verify they fail**

Run: `go test ./model ./controller ./service -run 'Test(PricingIncludesCanonicalImageResolutionPolicyForAliases|GenerateTextOtherInfoIncludesImageResolutionAuditWithoutUpstreamSecrets)' -count=1`

Expected: FAIL because the optional API and log fields do not exist.

- [ ] **Step 4: Add optional pricing metadata without changing existing fields**

```go
type Pricing struct {
	// existing fields remain unchanged
	ImageResolutionPricing *operation_setting.ImageResolutionPricingMetadata `json:"image_resolution_pricing,omitempty"`
}
```

In `updatePricing`, after existing fixed-price/ratio fields are populated:

```go
if metadata, ok := operation_setting.GetImageResolutionPricingMetadata(model); ok {
	pricing.ImageResolutionPricing = &metadata
}
```

Do not replace `ModelPrice`, `QuotaType`, `EnableGroup`, endpoint metadata, or group filtering. Alias rows may keep their current legacy `ModelPrice` during this compatibility phase; the new nested policy is authoritative for resolution-aware clients.

- [ ] **Step 5: Append frozen quote fields to logs only when a quote exists**

Add a focused `appendImageResolutionBillingInfo(relayInfo, other)` call from `GenerateTextOtherInfo`. It adds only:

```go
other["image_pricing_model"] = priceData.ImageResolutionPricingModel
other["image_requested_size"] = priceData.ImageResolutionRequestedSize
other["image_resolution_tier"] = priceData.ImageResolutionTier
other["image_unit_price"] = priceData.ImageResolutionUnitPrice
other["image_count"] = priceData.ImageResolutionImageCount
```

Return immediately when `ImageResolutionPricingModel` is empty. Preserve existing ordinary-user masking of `ActualResponseModel` and admin-only upstream audit fields.

- [ ] **Step 6: Run API/log tests and pricing visibility regressions**

Run: `go test ./model ./controller ./service -run 'Test(Pricing|GetPricing|FilterPricing|GenerateTextOtherInfo)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit API and audit metadata**

```bash
git add model/pricing.go controller/model_list_test.go service/log_info_generate.go service/log_info_generate_test.go
git commit -m "feat: expose image resolution billing metadata"
```

---

### Task 6: Full Billing and Routing Regression Gate

**Files:**
- Modify only files from Tasks 1-5 if a regression reveals a defect.

**Interfaces:**
- Consumes: all implementation tasks.
- Produces: evidence that unrelated YuAPI billing and routing behavior remains intact.

- [ ] **Step 1: Format all changed Go files**

Run: `gofmt -w setting/operation_setting/image_resolution_pricing.go setting/operation_setting/image_resolution_pricing_test.go model/option.go model/option_image_resolution_pricing_test.go types/price_data.go relay/helper/price.go relay/helper/price_test.go dto/openai_image_test.go service/text_quota.go service/text_quota_test.go service/tool_billing.go service/tool_billing_test.go setting/operation_setting/tools.go model/pricing.go controller/model_list_test.go service/log_info_generate.go service/log_info_generate_test.go`

Expected: command exits 0.

- [ ] **Step 2: Run the focused affected-package suite**

Run: `go test ./setting/operation_setting ./model ./dto ./relay/helper ./service ./controller -count=1`

Expected: PASS.

- [ ] **Step 3: Run invariant regressions explicitly**

Run: `go test ./service ./relay/helper ./controller -run 'Test.*(BillingExpr|Tiered|TaskBilling|ChannelAffinity|ViolationFee|ActualResponseModel|Image|Pricing)' -count=1`

Expected: PASS. This gate specifically protects billing expressions, task billing, affinity, violation fees, actual response model, existing image handling, and pricing visibility.

- [ ] **Step 4: Run the complete Go suite**

Run: `go test ./... -count=1`

Expected: PASS. If an environment-dependent integration test is skipped by its existing guard, record the skip; do not weaken or delete the test.

- [ ] **Step 5: Verify no forbidden routing or payload files changed**

Run: `git diff --name-only $(git merge-base HEAD origin/main)..HEAD`

Expected: no changes under `model/channel_image_selection.go`, `relay/channel/`, Caddy/deployment files, database migrations, or frontend theme source. The only `relay/` changes are `relay/helper/price.go` and its tests.

- [ ] **Step 6: Verify the old conflicting GPT Image 2 table has no callers or definition**

Run: `rg -n 'GetGPTImage2PriceOnceCall|gptImage2PriceTier' --glob '!tmp/**'`

Expected: no output.

- [ ] **Step 7: Commit any regression-only fixes**

If Steps 2-6 required code corrections:

```bash
git status --short
git add setting/operation_setting/image_resolution_pricing.go setting/operation_setting/image_resolution_pricing_test.go model/option.go model/option_image_resolution_pricing_test.go types/price_data.go relay/helper/price.go relay/helper/price_test.go dto/openai_image_test.go service/text_quota.go service/text_quota_test.go service/tool_billing.go service/tool_billing_test.go setting/operation_setting/tools.go model/pricing.go controller/model_list_test.go service/log_info_generate.go service/log_info_generate_test.go
git commit -m "test: harden image resolution billing regressions"
```

If no corrections were needed, do not create an empty commit.

---

### Task 7: Local Candidate and Pricing Simulation (No Production Changes)

**Files:**
- Create: `tmp/image-resolution-pricing/` runtime artifacts only; do not commit.
- Modify: `docs/superpowers/handoffs/2026-08-31-stream-image-production-hot-cutover.md` only after local evidence exists.

**Interfaces:**
- Consumes: a fully passing Task 6 branch.
- Produces: loopback-only candidate evidence, UI asset-graph evidence, and a simulated charge matrix for user approval.

- [ ] **Step 1: Capture the current local/production-derived baseline metadata without exposing secrets**

Record only image/container IDs, creation times, UI asset fingerprints, and current `/api/pricing` image rows. Redact headers, tokens, environment values, credentials, database content, and access logs.

- [ ] **Step 2: Build a candidate from this exact worktree and recovered complete production UI asset graph**

Use the existing production-derived build procedure documented in `docs/superpowers/handoffs/2026-08-31-stream-image-production-hot-cutover.md`. Bind the candidate only to an unused `127.0.0.1` port and an independent local database. Do not point Caddy at it.

- [ ] **Step 3: Verify complete UI asset closure before opening the candidate**

For every script and stylesheet reachable from the entry HTML and route chunks, require HTTP 200 and the correct JavaScript/CSS content type. Explicitly verify the previously missing chunks are not served as SPA HTML:

```text
/static/js/async/3395.7bb002d7bb.js
/static/js/async/531.c823517b31.js
```

Expected: both return JavaScript, not `text/html`. If hashes changed in the new build, verify the equivalent login/wallet/log lazy chunks from the new manifest instead.

- [ ] **Step 4: Simulate the exact quota matrix without paid requests**

Use resolver/helper tests or a local authenticated dry-run harness to record:

```text
gpt-image-2 650x1024 n=1 group=1.0 -> base 0.01, 1K
gpt-image-2 1024x1536 n=1 group=0.3 -> base 0.04, 2K, final factor 0.3 once
gpt-image-2-4k 1024x1024 n=2 group=0.3 -> base 0.045, 4K floor, n=2 once
nano-banana-pro 2048x3072 n=1 group=1.0 -> base 0.161416666667, 4K
nano-banana2 auto n=1 group=1.0 -> base 0.063916666667, default 1K
gpt-image-2 4097x512 -> HTTP 400 before channel selection and zero pre-consume
unmanaged-image-model -> unchanged legacy pricing path
```

- [ ] **Step 5: Verify `/api/pricing` compatibility locally**

Expected:

- existing aliases still appear where their channel/group abilities permit;
- each configured alias exposes canonical policy metadata and its minimum tier;
- all legacy fields remain present and group filtering still applies;
- no upstream URL, key, provider account, or channel credential appears.

- [ ] **Step 6: Run local UI checks and open the candidate for user review**

Check homepage, sign-in/register, console, API keys, logs, wallet, system settings, infinite canvas, docs, custom brand, animations, and model/pricing views. Open the loopback candidate in Codex only after all checks return without 500s.

- [ ] **Step 7: Update the handoff with local evidence and rollback boundaries**

Document the branch commit, candidate image/container, loopback port, asset fingerprints, test commands, simulated matrix, and the explicit statement: no production Caddy/container/database/price change occurred.

- [ ] **Step 8: Commit the handoff update**

```bash
git add docs/superpowers/handoffs/2026-08-31-stream-image-production-hot-cutover.md
git commit -m "docs: record image resolution pricing candidate"
```

Stop here for user UI and pricing approval. A later production rollout requires a separate approved runbook that snapshots the old option and `/api/pricing`, updates the complete map atomically, health-checks before traffic, preserves the old production container/image, and rolls back immediately on UI, billing, database, or availability regression.
