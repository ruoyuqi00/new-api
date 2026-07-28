# Private Group Configuration Clarity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make production private groups understandable and secure in the existing group/pricing settings UI, while restoring private pricing visibility for authorized dashboard access tokens.

**Architecture:** Keep backend filtering authoritative. Add optional dashboard PAT recognition to `TryUserAuth`, expose an admin-only read-only group catalog assembled from ratio settings and enabled channel abilities, and feed that catalog into the existing group table and special-rule editor. Extract frontend serialization and coverage-state logic into a tested utility module; do not create a new page or touch the experimental UI.

**Tech Stack:** Go 1.22, Gin, GORM, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Query, shadcn/Base UI, Bun, i18next.

---

## File Map

- Create `middleware/auth_optional_test.go`: optional session/PAT authentication regression tests.
- Modify `middleware/auth.go`: recognize dashboard PATs in `TryUserAuth` while keeping anonymous access.
- Create `model/group_catalog.go`: cross-database active group routing coverage query.
- Create `model/group_catalog_test.go`: routing coverage aggregation contract.
- Modify `controller/group.go`: build and return the admin group catalog.
- Create `controller/group_catalog_test.go`: catalog response and public/private state tests.
- Modify `router/api-router.go`: register `GET /api/group/catalog` under existing `AdminAuth` group.
- Create `controller/pricing_test.go`: private pricing visibility regression tests.
- Modify `web/default/src/features/system-settings/types.ts`: catalog API types.
- Modify `web/default/src/features/system-settings/api.ts`: catalog request.
- Create `web/default/src/features/system-settings/models/group-config-utils.ts`: parse/serialize and coverage-state logic.
- Create `web/default/src/features/system-settings/models/group-config-utils.test.ts`: frontend behavior tests.
- Modify `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx`: public/private and routing coverage columns.
- Modify `web/default/src/features/system-settings/models/group-special-usable-editor.tsx`: authorization-specific labels and routing warnings.
- Modify `web/default/src/features/system-settings/models/group-ratio-form.tsx`: fetch/share catalog and clarify guide copy.
- Temporarily create `web/default/scripts/add-missing-keys.mjs`, then remove it after applying all six locales.
- Modify via script only `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`.

### Task 1: Optional Dashboard PAT Authentication

- [ ] **Step 1: Write failing middleware tests**

Create `middleware/auth_optional_test.go` with a private-group user fixture and routes using `TryUserAuth()`:

```go
func TestTryUserAuthUsesDashboardAccessToken(t *testing.T) {
    user := createOptionalAuthUser(t, "private", "dashboard-pat")
    recorder := performOptionalAuthRequest(t, "Bearer dashboard-pat")
    require.Equal(t, http.StatusOK, recorder.Code)
    require.JSONEq(t, fmt.Sprintf(`{"id":%d,"group":"private"}`, user.Id), recorder.Body.String())
}

func TestTryUserAuthRejectsInvalidSuppliedCredential(t *testing.T) {
    recorder := performOptionalAuthRequest(t, "Bearer invalid")
    require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTryUserAuthKeepsCredentialFreeRequestAnonymous(t *testing.T) {
    recorder := performOptionalAuthRequest(t, "")
    require.Equal(t, http.StatusOK, recorder.Code)
    require.JSONEq(t, `{"id":0,"group":""}`, recorder.Body.String())
}
```

- [ ] **Step 2: Run RED test**

Run: `go test ./middleware -run 'TestTryUserAuth' -count=1`

Expected: PAT context test fails because current `TryUserAuth` only reads the session; invalid credential test also fails because the request is treated as anonymous.

- [ ] **Step 3: Implement the minimum optional-auth path**

Update `TryUserAuth` so it:

```go
session := sessions.Default(c)
if id := session.Get("id"); id != nil {
    c.Set("id", id)
    c.Set("group", session.Get("group"))
    c.Set("user_group", session.Get("group"))
    c.Next()
    return
}

authorization := strings.TrimSpace(c.GetHeader("Authorization"))
if authorization == "" {
    c.Next()
    return
}

user, err := model.ValidateAccessToken(authorization)
// database errors -> 500, missing/invalid PAT -> 401, disabled/invalid user -> 401
// valid PAT -> set id, username, role, group, user_group, use_access_token and UserBase context
```

Do not call relay `ValidateUserToken`; relay API keys remain a separate credential type.

- [ ] **Step 4: Run GREEN test**

Run: `go test ./middleware -run 'TestTryUserAuth' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```text
git add middleware/auth.go middleware/auth_optional_test.go
git commit -m "fix: recognize dashboard tokens in optional auth"
```

### Task 2: Active Routing Coverage and Admin Group Catalog

- [ ] **Step 1: Write failing model coverage test**

Create `model/group_catalog_test.go` with enabled/disabled channels and duplicate model abilities:

```go
func TestGetActiveGroupRoutingCoverageDeduplicatesChannelsAndModels(t *testing.T) {
    coverage, err := GetActiveGroupRoutingCoverage()
    require.NoError(t, err)
    require.Equal(t, 2, coverage["private"].ActiveChannelCount)
    assert.Equal(t, []string{"image-model", "video-model"}, coverage["private"].ActiveModels)
    assert.Equal(t, 2, coverage["private"].ActiveModelCount)
}
```

The fixture must verify disabled abilities and abilities attached to disabled channels are excluded.

- [ ] **Step 2: Run RED model test**

Run: `go test ./model -run TestGetActiveGroupRoutingCoverage -count=1`

Expected: compile failure because the coverage API does not exist.

- [ ] **Step 3: Implement cross-database coverage aggregation**

Create `model/group_catalog.go`:

```go
type GroupRoutingCoverage struct {
    ActiveChannelCount int      `json:"active_channel_count"`
    ActiveModelCount   int      `json:"active_model_count"`
    ActiveModels       []string `json:"active_models"`
}

func GetActiveGroupRoutingCoverage() (map[string]GroupRoutingCoverage, error) {
    // GORM query joins abilities to channels, requires both enabled states,
    // selects distinct group/model/channel rows, then deduplicates and sorts in Go.
}
```

Use `abilities.` plus the dialect-aware `commonGroupCol`; do not use database-specific aggregation functions.

- [ ] **Step 4: Run GREEN model test**

Run: `go test ./model -run TestGetActiveGroupRoutingCoverage -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing controller catalog test**

Create `controller/group_catalog_test.go`:

```go
func TestGetGroupCatalogCombinesRatiosVisibilityAndCoverage(t *testing.T) {
    // ratio map contains public + private; UserUsableGroups contains public only.
    // assert sorted rows, ratio, public flag, description, channel count and models.
}
```

- [ ] **Step 6: Run RED controller test**

Run: `go test ./controller -run TestGetGroupCatalog -count=1`

Expected: compile failure because `GetGroupCatalog` does not exist.

- [ ] **Step 7: Implement controller and protected route**

Add to `controller/group.go`:

```go
type groupCatalogItem struct {
    Name               string   `json:"name"`
    Ratio              float64  `json:"ratio"`
    Public             bool     `json:"public"`
    Description        string   `json:"description"`
    ActiveChannelCount int      `json:"active_channel_count"`
    ActiveModelCount   int      `json:"active_model_count"`
    ActiveModels       []string `json:"active_models"`
}
```

Build rows from `ratio_setting.GetGroupRatioCopy()`, public descriptions from `setting.GetUserUsableGroupsCopy()`, coverage from the model method, sort by name, and return `{success,message,data}`. Register `groupRoute.GET("/catalog", controller.GetGroupCatalog)` before the existing `GET /` route; the route group already uses `AdminAuth`.

- [ ] **Step 8: Run GREEN controller test**

Run: `go test ./controller -run TestGetGroupCatalog -count=1`

Expected: PASS.

- [ ] **Step 9: Commit**

```text
git add model/group_catalog.go model/group_catalog_test.go controller/group.go controller/group_catalog_test.go router/api-router.go
git commit -m "feat: expose private group routing coverage"
```

### Task 3: Private Pricing Visibility Contract

- [ ] **Step 1: Write pricing regression tests**

Create `controller/pricing_test.go` around `GetPricing` and `filterPricingByUsableGroups`:

```go
func TestGetPricingHidesPrivateGroupFromAnonymousRequest(t *testing.T) { /* no id; no private ratio/group/model */ }
func TestGetPricingIncludesPrivateGroupForAuthorizedUser(t *testing.T) { /* context id for private user; private ratio/group/model present */ }
func TestGetPricingIncludesSpecialRulePrivateGroup(t *testing.T) { /* user group has +:private rule */ }
```

Save and restore global ratio/user-usable/special settings in test cleanup and invalidate pricing caches.

- [ ] **Step 2: Run tests and confirm contract**

Run: `go test ./controller -run 'TestGetPricing.*Private' -count=1`

Expected: PASS after Task 1 because `GetPricing` already filters through `service.GetUserUsableGroups`; any failure indicates a real filtering defect and must be fixed minimally before continuing.

- [ ] **Step 3: Commit**

```text
git add controller/pricing_test.go
git commit -m "test: protect private group pricing visibility"
```

### Task 4: Existing Group UI Clarity

- [ ] **Step 1: Write failing frontend logic tests**

Create `group-config-utils.test.ts`:

```ts
test('serializes private groups out of UserUsableGroups', () => {
  const result = serializeGroupPricingRows([{ name: 'private', ratio: 0.2, public: false, description: 'ignored' }])
  expect(JSON.parse(result.UserUsableGroups)).toEqual({})
})

test('reports saved groups without active models as blocked', () => {
  expect(getGroupCoverageState('private', catalog)).toBe('missing')
})

test('reports new unsaved groups separately', () => {
  expect(getGroupCoverageState('new_group', catalog)).toBe('unsaved')
})
```

- [ ] **Step 2: Run RED frontend test**

Run from `web/default`: `bun test src/features/system-settings/models/group-config-utils.test.ts`

Expected: module/function-not-found failure.

- [ ] **Step 3: Extract minimal tested utilities and catalog types/API**

Add API types:

```ts
export type GroupCatalogItem = {
  name: string
  ratio: number
  public: boolean
  description: string
  active_channel_count: number
  active_model_count: number
  active_models: string[]
}
export type GroupCatalogResponse = { success: boolean; message: string; data: GroupCatalogItem[] }
```

Add `getGroupCatalog()` to system-settings `api.ts`, and move `buildGroupPricingRows`, `serializeGroupPricingRows`, signatures, and coverage-state logic into `group-config-utils.ts`. Rename the row flag from `selectable` to `public` while preserving the exact `GroupRatio` and `UserUsableGroups` storage format.

- [ ] **Step 4: Run GREEN frontend test**

Run: `bun test src/features/system-settings/models/group-config-utils.test.ts`

Expected: PASS.

- [ ] **Step 5: Enhance the existing visual editor**

Use `useQuery({ queryKey: ['group-catalog'], queryFn: getGroupCatalog })` in `GroupRatioForm` and pass catalog data to both editors.

In `GroupPricingTable`:

- Replace “User selectable” with a Public/Private status badge and a checkbox/switch labelled as public visibility.
- Show active channel and active model counts from the saved catalog.
- Show a destructive warning for saved groups with zero active models.
- Show “Check routing after saving” for unsaved group names.
- Keep descriptions editable only for public groups; private rows show the fixed authorization note.
- Keep duplicate and empty-name validation visible and disable/guard save when invalid.
- Before deleting a row, display dependency context when the saved catalog has active routing or a special rule targets it; never delete channels or rules automatically.

In `GroupSpecialUsableRulesEditor`:

- Rename the section to user-group authorization.
- Clarify source user group vs target private group.
- Explain that `+:` grants visibility/selectability only and does not create routing, copy models, or alter ratios.
- Warn when the target group is absent from the ratio catalog or has zero active models.

- [ ] **Step 6: Update the usage guide**

Replace ambiguous “non-selectable” language with public/private semantics and explicitly state that private groups require same-group membership or a special rule, plus existing channel/model routing coverage.

- [ ] **Step 7: Format and run targeted frontend verification**

Run from `web/default`:

```text
bun run format -- src/features/system-settings/api.ts src/features/system-settings/types.ts src/features/system-settings/models/group-config-utils.ts src/features/system-settings/models/group-config-utils.test.ts src/features/system-settings/models/group-ratio-form.tsx src/features/system-settings/models/group-ratio-visual-editor.tsx src/features/system-settings/models/group-special-usable-editor.tsx
bun test src/features/system-settings/models/group-config-utils.test.ts
bun run typecheck
bun run lint
```

Expected: zero test, type, or lint failures.

- [ ] **Step 8: Commit**

```text
git add web/default/src/features/system-settings
git commit -m "feat: clarify private group configuration"
```

### Task 5: Six-Locale UI Copy

- [ ] **Step 1: Run the current i18n report**

Run from `web/default`: `bun run i18n:sync`

Read `src/i18n/locales/_reports/_sync-report.json` and distinguish pre-existing report items from the new group copy.

- [ ] **Step 2: Add all new keys through the mandated script**

Create `scripts/add-missing-keys.mjs` with the skill-prescribed atomic writer and populate every new key for `en`, `zh`, `fr`, `ja`, `ru`, and `vi`. Do not edit locale JSON directly.

- [ ] **Step 3: Apply, normalize, and remove the temporary script**

Run:

```text
node scripts/add-missing-keys.mjs
bun run i18n:sync
Remove-Item scripts/add-missing-keys.mjs
```

Expected: all new literal `t(...)` keys exist in every locale and locale JSON remains sorted/valid.

- [ ] **Step 4: Commit**

```text
git add web/default/src/i18n/locales
git commit -m "i18n: translate private group configuration"
```

### Task 6: Full Verification and Production-Safe Delivery

- [ ] **Step 1: Run backend verification**

```text
gofmt -w middleware/auth.go middleware/auth_optional_test.go model/group_catalog.go model/group_catalog_test.go controller/group.go controller/group_catalog_test.go controller/pricing_test.go router/api-router.go
go test ./middleware ./model ./controller -count=1
go test ./... -count=1
```

Expected: all packages PASS.

- [ ] **Step 2: Run frontend verification**

From `web/default`:

```text
bun test src/features/system-settings/models/group-config-utils.test.ts
bun run i18n:sync
bun run format:check
bun run typecheck
bun run lint
bun run build
```

Expected: all commands exit 0.

- [ ] **Step 3: Audit scope**

Run:

```text
git status --short
git diff --stat e9665378a..HEAD
git diff --name-only e9665378a..HEAD
```

Expected: only the production backend, `web/default`, tests, locale files, and the approved docs appear. `.local-preview/`, `production-b5514ebe1.tar`, `test-results/`, and the experimental UI remain untracked and untouched.

- [ ] **Step 4: Build and deploy only the production service**

Use the repository's existing production deployment workflow. Back up the production database and Compose configuration first, build the exact verified commit, replace only the `newapi` service, and wait for its health check; do not upload the experimental UI or restart unrelated services.

- [ ] **Step 5: Verify production contracts**

Check:

- Anonymous and ordinary-user `/api/pricing` responses do not contain `下游`.
- An authorized dashboard PAT receives `下游` ratio and supported models.
- Admin `GET /api/group/catalog` shows `下游` as private with one active channel and four active models.
- Existing token 141 still receives exactly `gpt-5.5`, `gpt-5.6-luna`, `gpt-5.6-sol`, and `gpt-5.6-terra` from `/v1/models`.
- Existing API and frontend health endpoints return HTTP 200.

- [ ] **Step 6: Final commit/push only after verification**

Commit any verification-only formatting changes, push the current production branch, and report the exact commit and deployment health evidence. Never add the three preserved untracked paths.

---

## Self-Review

- Spec coverage: optional PAT auth, backend privacy, admin catalog, routing coverage, existing UI clarification, special-rule warnings, six locales, verification, and production-only deployment are each mapped to tasks.
- Scope exclusions: no new settings page, no direct-domain deployment, no channel/user/ratio data mutation, and no experimental UI changes.
- Type consistency: backend and frontend catalog fields use the same snake_case JSON contract; frontend row state uses `public` consistently and serializes public groups only into `UserUsableGroups`.
- Placeholder scan: all implementation steps name concrete files, functions, commands, and expected outcomes.
