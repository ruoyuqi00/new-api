# Infinite Canvas Media Model Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make YuCore Studio and Infinite Canvas list and execute only the logged-in user's currently available image/video models through standard YuAPI balance billing.

**Architecture:** A service-layer catalog derives media models from user-usable groups, enabled channel abilities, pricing metadata, and endpoint types. Controllers validate every selected group/model before persisting a task, while the existing `yuapi-channel` adapter creates an internal per-user token for the selected group and sends the request through the normal relay. The frontend consumes one catalog, uses deterministic group/model fallback helpers, and exposes the same real selection in Studio and Canvas.

**Tech Stack:** Go 1.22+, Gin, GORM, testify, React 19, TypeScript, Bun, Base UI/Tailwind.

---

## File Map

- Create `service/yucore_media_catalog.go`: catalog construction, stable ordering, endpoint filtering, and selection validation.
- Create `service/yucore_media_catalog_test.go`: catalog and validation contracts against explicit database/cache fixtures.
- Modify `model/yucore_media.go`: expose a read-only copy of media catalog settings, then add task group persistence in Task 2.
- Modify `model/yucore_media.go`: additive `billing_group` persistence and group-aware cost estimate.
- Modify `model/yucore_media_openai_compatible.go`: selected-group managed token lookup with legacy fallback.
- Modify `model/yucore_media_openai_compatible_test.go`: managed-token routing regression tests.
- Modify `controller/yucore_media.go`: catalog endpoint, compatibility models endpoint, task group input/output, and validation.
- Modify `controller/yucore_canvas.go`: canvas agent group input and validation.
- Modify `router/api-router.go`: authenticated catalog route.
- Create `web/default/src/features/yucore-brand/lib/media-catalog.ts`: pure catalog selection and fallback utilities.
- Create `web/default/tests/yucore-media-catalog.test.ts`: frontend selection contracts.
- Modify `web/default/src/features/yucore-brand/api/studio.ts`: catalog/group/task API types and requests.
- Modify `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`: dynamic selectors, capability-aware controls, task metadata, and balance refresh.
- Modify `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: translations for new visible controls and errors through the project i18n workflow.

### Task 1: Dynamic User Media Catalog

**Files:**
- Create: `service/yucore_media_catalog.go`
- Create: `service/yucore_media_catalog_test.go`
- Modify: `model/yucore_media.go`

- [ ] **Step 1: Write failing catalog tests**

Create deterministic SQLite fixtures containing a user, enabled/disabled channels, abilities in two groups, an image model, a video model, and a text-only model. Assert the desired API:

```go
catalog, err := BuildYucoreMediaCatalog(userID)
require.NoError(t, err)
assert.Equal(t, "multimodal", catalog.DefaultGroup)
assert.Equal(t, "image-live", catalog.Groups[0].Models[0].Id)
assert.Equal(t, "image", catalog.Groups[0].Models[0].Kind)
assert.Equal(t, "video-live", catalog.Groups[0].Models[1].Id)
assert.Equal(t, "video", catalog.Groups[0].Models[1].Kind)
```

Add a second test for `auto` expansion and stable group/model ordering. Initialize user group settings, ratio settings, database rows, and pricing cache state explicitly in the fixture.

- [ ] **Step 2: Run tests and verify RED**

Run `go test ./service -run 'TestBuildYucoreMediaCatalog' -count=1`.

Expected: build failure because `BuildYucoreMediaCatalog` and catalog types do not exist.

- [ ] **Step 3: Implement the catalog builder**

Define focused exported response types and builder/validator APIs:

```go
type YucoreMediaCatalog struct {
    DefaultGroup string                     `json:"default_group"`
    Groups       []YucoreMediaCatalogGroup `json:"groups"`
}

type YucoreMediaCatalogGroup struct {
    Id          string                    `json:"id"`
    Description string                    `json:"description"`
    Ratio       float64                   `json:"ratio"`
    Models      []YucoreMediaCatalogModel `json:"models"`
}

type YucoreMediaCatalogModel struct {
    Id           string   `json:"id"`
    Name         string   `json:"name"`
    Description  string   `json:"description,omitempty"`
    Kind         string   `json:"kind"`
    Modes        []string `json:"modes"`
    Sizes        []string `json:"sizes,omitempty"`
    AspectRatios []string `json:"aspect_ratios,omitempty"`
    Durations    []int    `json:"durations,omitempty"`
    PriceDisplay string   `json:"price_display,omitempty"`
}

func BuildYucoreMediaCatalog(userID int) (*YucoreMediaCatalog, error)
func ResolveYucoreMediaSelection(userID int, group, modelID, kind string) (string, YucoreMediaCatalogModel, error)
```

Add `model.GetYucoreMediaCatalogSettings() (string, map[string]YucoreMediaModelCapability)` that returns the preferred managed group and a defensive copy of explicit capabilities without exposing any API key or base URL. Use `GetUserUsableGroups`, `GetUserAutoGroup`, `GetGroupsEnabledModels`, and `model.GetPricing()`. Include a model only when `SupportedEndpointTypes` contains `constant.EndpointTypeImageGeneration` or `constant.EndpointTypeOpenAIVideo`. Prefer the configured managed media group when usable and non-empty, then `auto`, then remaining groups sorted by ID. Build conservative capability arrays and calculate price display with `GetUserGroupRatio`.

- [ ] **Step 4: Run tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit catalog service**

```powershell
git add service/yucore_media_catalog.go service/yucore_media_catalog_test.go model/yucore_media.go
git commit -m "feat: derive canvas media catalog from user routing"
```

### Task 2: Persist and Validate the Selected Billing Group

**Files:**
- Modify: `model/yucore_media.go`
- Modify: `controller/yucore_media.go`
- Modify: `controller/yucore_canvas.go`
- Modify: `router/api-router.go`
- Test: `model/yucore_media_test.go`
- Create: `controller/yucore_media_catalog_test.go`

- [ ] **Step 1: Write failing persistence and controller tests**

Add a model test that migrates `YucoreMediaTask`, creates a task with `BillingGroup: "multimodal"`, reloads it, and asserts the value survives. Add controller tests for an authenticated user and assert the catalog default plus persisted task group. Add rejection cases for an unauthorized group, a model absent from the group, and a text-only model requested as image/video. Assert no task row is created on rejection.

- [ ] **Step 2: Run tests and verify RED**

Run `go test ./model ./controller -run 'TestYucoreMediaTaskPersistsBillingGroup|TestYucoreMediaCatalog|TestCreateYucoreMediaTaskValidatesSelection' -count=1`.

Expected: compile/assertion failures for missing group fields, route/controller, and validation.

- [ ] **Step 3: Add the group field and catalog controller**

Add the cross-database-safe field:

```go
BillingGroup string `json:"group" gorm:"column:billing_group;type:varchar(64);index;default:''"`
```

Add `Group string` to task and canvas execute request DTOs and the task response. Register `GET /api/yucore/media/catalog`. Replace the hard-coded `ListYucoreMediaModels` with a compatibility response derived from the dynamic catalog's default group. Before normal or canvas task creation, resolve and validate the selection and store the resolved group before any database write.

- [ ] **Step 4: Run tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Format and commit**

```powershell
gofmt -w service/yucore_media_catalog.go service/yucore_media_catalog_test.go model/yucore_media.go model/yucore_media_test.go controller/yucore_media.go controller/yucore_canvas.go controller/yucore_media_catalog_test.go router/api-router.go
git add service model controller router
git commit -m "feat: validate canvas media group and model"
```

### Task 3: Route and Bill Through the Selected Group

**Files:**
- Modify: `model/yucore_media.go`
- Modify: `model/yucore_media_openai_compatible.go`
- Modify: `model/yucore_media_openai_compatible_test.go`

- [ ] **Step 1: Write failing managed-token tests**

Create two tasks for one user with different billing groups and assert `yucoreMediaOpenAIConfigForTask` creates/uses distinct `yucore-studio-managed` tokens with those exact groups. Add a legacy task with an empty group and assert it uses `config.ManagedTokenGroup`. Assert the token belongs to the task user and media task creation does not directly change user quota.

- [ ] **Step 2: Run tests and verify RED**

Run `go test ./model -run 'TestYucoreMediaManagedTokenUsesTaskBillingGroup|TestYucoreMediaManagedTokenLegacyGroupFallback' -count=1`.

Expected: selected-group test fails because the current implementation always uses `ManagedTokenGroup`.

- [ ] **Step 3: Implement selected-group routing**

Resolve the managed token group from `task.BillingGroup`, falling back to `config.ManagedTokenGroup` only for legacy tasks. Use the same resolved group for cost estimates. Do not add a media-layer quota mutation; the normal internal relay remains the only billing authority.

- [ ] **Step 4: Run tests and verify GREEN**

Run the Step 2 command, followed by `go test ./model -run 'YucoreMedia' -count=1`. Expected: PASS.

- [ ] **Step 5: Commit billing routing**

```powershell
git add model/yucore_media.go model/yucore_media_openai_compatible.go model/yucore_media_openai_compatible_test.go
git commit -m "fix: bill canvas media through selected group"
```

### Task 4: Frontend Catalog Contract and Selection State

**Files:**
- Create: `web/default/src/features/yucore-brand/lib/media-catalog.ts`
- Create: `web/default/tests/yucore-media-catalog.test.ts`
- Modify: `web/default/src/features/yucore-brand/api/studio.ts`

- [ ] **Step 1: Write failing Bun tests**

Test a pure helper API that preserves a valid selection, replaces a missing model, keeps image/video selection independent, and returns an empty model for a group with no matching kind:

```ts
expect(resolveMediaSelection(catalog, 'multimodal', 'missing', 'image')).toEqual({
  group: 'multimodal',
  modelId: 'image-live',
})
expect(modelsForKind(catalog, 'video', 'multimodal').map((m) => m.id)).toEqual([
  'video-live',
])
```

- [ ] **Step 2: Run tests and verify RED**

From `web/default`, run `bun test tests/yucore-media-catalog.test.ts`.

Expected: import failure because `media-catalog.ts` does not exist.

- [ ] **Step 3: Implement types, API call, and helpers**

Add `YucoreMediaCatalog` and group types, include `group` on tasks, and implement `getYucoreMediaCatalog()`. Update task and agent payloads to require `group`. Implement stable side-effect-free helpers without embedded model ID fallbacks.

- [ ] **Step 4: Run tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit frontend contract**

```powershell
git add web/default/src/features/yucore-brand/api/studio.ts web/default/src/features/yucore-brand/lib/media-catalog.ts web/default/tests/yucore-media-catalog.test.ts
git commit -m "feat: add canvas media catalog client"
```

### Task 5: Expose Real Group and Model Selection

**Files:**
- Modify: `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Load the i18n skill and add translations**

Read `.agents/skills/i18n-translate/SKILL.md`. Add literal `t('...')` keys for billing group, image model, video model, empty states, and catalog refresh failures, then use the sanctioned locale sync command.

- [ ] **Step 2: Replace independent model loading with catalog state**

Load `getYucoreMediaCatalog()` during initialization. Store `selectedGroup`, `imageModelId`, and `videoModelId`; derive arrays with the tested helpers. Remove hard-coded `gpt-image-2` and `veo-3.1-generate-preview` fallbacks.

- [ ] **Step 3: Add Canvas selectors**

Add compact group and image-model selectors above the canvas agent prompt using existing controls. Keep them visible on mobile, show exact model IDs, and disable submission when no image model is available.

- [ ] **Step 4: Make Studio controls capability-aware**

Use selected-group models on image/video screens. Render modes, sizes, formats, qualities, durations, counts, and reference limits only when reported. Replace the loading placeholder with an explicit empty state after loading completes.

- [ ] **Step 5: Submit and display exact routing**

Include `group: selectedGroup` in normal and canvas requests. Put group/model in task nodes and history labels. Refresh billing after acceptance and terminal completion without client-side deduction.

- [ ] **Step 6: Run frontend verification**

From `web/default`, run:

```powershell
bun test tests/yucore-media-catalog.test.ts
bun run typecheck
bun run lint
bun run format:check
bun run build
```

Expected: all commands exit 0 without TypeScript or lint errors.

- [ ] **Step 7: Commit UI**

```powershell
git add web/default/src/features/yucore-brand web/default/src/i18n/locales web/default/tests/yucore-media-catalog.test.ts
git commit -m "feat: select live media models in infinite canvas"
```

### Task 6: Verify, Push, and Deploy Seamlessly

**Files:**
- Verify all changed files and deployment configuration without modifying unrelated containers.

- [ ] **Step 1: Run backend and repository checks**

Run `go test ./service ./model ./controller -count=1`, `go test ./... -count=1`, `git diff --check`, and `git status --short`. Expected: tests pass and only intentional changes remain.

- [ ] **Step 2: Review final diff**

Confirm there are no secrets, fixed production model lists, direct quota mutations, or unrelated changes. Any correction must start with a failing regression test.

- [ ] **Step 3: Push branch**

Run `git push ruoyu codex/sub2api-account-pool-integration-20260731`. Expected: remote advances to the verified commit.

- [ ] **Step 4: Build and start candidate**

Build a production image tagged with the commit SHA and start a candidate on the existing Docker network with production configuration but no Caddy traffic. Wait for Docker health conditions.

- [ ] **Step 5: Verify candidate**

Check status, authentication protection, migration success, and an authenticated catalog without exposing credentials. Confirm the catalog contains current production media abilities and excludes text-only models while production stays healthy.

- [ ] **Step 6: Switch and observe**

Change only YuAPI Caddy upstream targets to the healthy candidate, reload Caddy, verify five production domains, and inspect health/restart counts. Replace the old production container only after checks pass and retain the prior image/database state for rollback.

- [ ] **Step 7: Report evidence**

Report commit/image IDs, catalog image/video counts, health/restart state, domain status codes, and test results without exposing tokens, passwords, request headers, `.env`, prompts, or database contents.
