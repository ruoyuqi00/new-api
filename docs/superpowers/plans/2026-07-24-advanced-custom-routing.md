# Advanced-Custom Routing Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add explicit, safe upstream model discovery for advanced-custom channels while preserving existing private-group routing, model aliases, downstream model lists, and rollback behavior.

**Architecture:** Discovery is opt-in: an advanced-custom channel must explicitly configure an exact `/v1/models` route with no converter before it can be fetched or scheduled for model updates. The relay adaptor builds that route with the same base URL and authentication semantics as a normal request; the controller uses it only for an administrator's discovery request and never as a customer routing candidate. Existing channel `Group`, `Models`, and `ModelMapping` remain the authority for downstream eligibility.

**Tech Stack:** Go 1.22+, Gin, GORM, testify, standard `net/http`.

---

## File Map

- Modify: `dto/channel_settings.go` - Define and validate the exact optional model-discovery route.
- Modify: `dto/channel_settings_test.go` - Lock discovery-route validation behavior.
- Modify: `model/channel.go` - Require a discovery route only when an advanced-custom channel explicitly enables scheduled upstream model checks.
- Modify: `model/channel_settings_test.go` - Cover the opt-in scheduled-check boundary.
- Modify: `relay/channel/advancedcustom/adaptor.go` - Build an authenticated discovery URL without resolving a customer request route.
- Modify: `relay/channel/advancedcustom/adaptor_test.go` - Cover header, query, bearer, no-auth, and missing-route cases.
- Modify: `controller/channel.go` - Reuse one non-passthrough header-override helper for ordinary and advanced-custom discovery.
- Modify: `controller/channel_upstream_update.go` - Fetch and strictly parse advanced-custom model lists, preserve proxy/Host handling, and redact discovery credentials from errors.
- Modify: `controller/channel_upstream_update_test.go` - Cover transport, selected multikey, error redaction, and no destructive model removal after invalid discovery.
- Modify: `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md` - Record the compatibility boundary and deployment evidence after verification.

### Task 1: Make The Discovery Route Explicit And Opt-In

**Files:** `dto/channel_settings.go`; `dto/channel_settings_test.go`; `model/channel.go`; `model/channel_settings_test.go`

- [x] **Step 1: Write failing DTO and channel-setting tests**

Add `TestAdvancedCustomValidateModelListRouteConstraints` and `TestAdvancedCustomChannelRequiresModelListRouteOnlyWhenUpdateChecksEnabled`. The valid configuration contains an exact `/v1/models` incoming path, a non-template upstream path, and converter `none`. Assert that a converter, `{model}` in the upstream path, or scheduled checks without that route fails validation; assert legacy advanced-custom routes remain valid when scheduled checks are off.

- [x] **Step 2: Verify RED**

Run `go test ./dto ./model -run 'AdvancedCustom.*ModelList|AdvancedCustomChannelRequiresModelList' -count=1`. The test must fail because the current DTO has no discovery-route contract and `ValidateSettings` does not require one for scheduled checks.

- [x] **Step 3: Implement the narrow contract**

Add `dto.AdvancedCustomModelListPath = "/v1/models"` and `(*AdvancedCustomConfig).ModelListRoute()`. In `Validate`, permit at most one exact route, require `converter == AdvancedCustomConverterNone`, and reject an upstream `{model}` template. In `(*model.Channel).ValidateSettings`, require `ModelListRoute` only when `Type == ChannelTypeAdvancedCustom` and `UpstreamModelUpdateCheckEnabled` is true.

- [x] **Step 4: Verify GREEN and commit**

Run `go test ./dto ./model -run 'AdvancedCustom.*ModelList|AdvancedCustomChannelRequiresModelList' -count=1`, then `go test ./dto ./model -count=1`. Commit with `git commit -m "feat: define advanced custom model discovery route"`.

### Task 2: Build Discovery Requests Without Affecting Customer Routes

**Files:** `relay/channel/advancedcustom/adaptor.go`; `relay/channel/advancedcustom/adaptor_test.go`

- [x] **Step 1: Write failing adaptor tests**

Add tests for `BuildModelListRequest(info)` proving it uses only the exact discovery route, joins relative paths to `ChannelBaseUrl`, applies default bearer/header/query/no-auth authentication, preserves existing query parameters, and returns an error when the route is absent. The test must call this method with an unrelated `RequestURLPath` so it cannot accidentally reuse the relay request route.

- [x] **Step 2: Verify RED**

Run `go test ./relay/channel/advancedcustom -run '^TestAdaptorBuildModelListRequest' -count=1`. The suite must fail because the method does not exist.

- [x] **Step 3: Implement only discovery URL construction**

Extract the URL body of `routeURL` into `buildRouteURL(route, converter, info)`. Add `BuildModelListRequest` to validate `ChannelOtherSettings.AdvancedCustom`, retrieve `ModelListRoute`, reject any non-`none` converter, call `buildRouteURL`, and return headers from the configured route auth. Do not add the discovery path to normal `resolve`, `MatchPath`, ability selection, or customer relay handling.

- [x] **Step 4: Verify GREEN and commit**

Run `go test ./relay/channel/advancedcustom -count=1`. Commit with `git commit -m "feat: build advanced custom model discovery requests"`.

### Task 3: Fetch, Sanitize, And Preserve Existing Models On Failure

**Files:** `controller/channel.go`; `controller/channel_upstream_update.go`; `controller/channel_upstream_update_test.go`

- [x] **Step 1: Write failing discovery tests**

Add a test server and advanced-custom channel fixture. Cover route-auth followed by channel header overrides, `Host` override, selected enabled multikey, non-200 response, malformed/missing/null/empty `data`, query-auth transport failure with no credential in the returned error, and an enabled scheduled update that receives an empty list without staging removals or changing persisted `Models`.

- [x] **Step 2: Verify RED**

Run `go test ./controller -run 'AdvancedCustom|ParseOpenAIModelIDs|FailedAdvancedCustomDetection' -count=1`. The suite must fail because advanced-custom discovery currently treats base URL `/v1/models` as a generic OpenAI request and does not use the configured route/auth contract.

- [x] **Step 3: Implement strict advanced-custom discovery**

Extract `applyFetchModelsHeaderOverrides(channel, key, headers)` from `buildFetchModelsHeaders`. Add `getFetchModelsResponseBody` that sets every header and treats `Host` as `request.Host`; use the channel proxy and return only non-secret status errors. Add `sanitizeFetchModelsError` and `parseOpenAIModelIDs`. In `fetchCredentialUpstreamModelIDs`, branch on `ChannelTypeAdvancedCustom` before generic URL construction, call `BuildModelListRequest`, apply overrides after route auth, fetch with the proxy, sanitize errors, and reject an empty or invalid response. Do not write `Models`, `Group`, `ModelMapping`, or account-pool bindings in this fetch path.

- [x] **Step 4: Verify GREEN and commit**

Run `go test ./controller -run 'AdvancedCustom|ParseOpenAIModelIDs|FailedAdvancedCustomDetection' -count=1`, then `go test ./controller -count=1`. Commit with `git commit -m "feat: fetch advanced custom upstream models safely"`.

### Task 4: Verify Private-Group And Downstream Compatibility

**Files:** `controller/model_list_test.go`; `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`

- [x] **Step 1: Run existing group-boundary regressions**

Run `go test ./controller -run '^(TestGetUserModelsFiltersByRequestedGroup|TestGetUserModelsExpandsAutoGroupsInConfiguredOrder|TestListModels.*)$' -count=1`. These tests establish the downstream group-filter and auto-group contract that the discovery code must not change.

- [x] **Step 2: Review the complete diff against the boundary**

Run `git diff --name-only <base>..HEAD` and `git diff --check <base>..HEAD`. Confirm no change exists in distributor selection, ability group selection, `Group`, `ModelMapping`, account credentials, live channel priorities, or experimental UI. The discovery implementation must be read-only until an administrator explicitly applies detected model changes through the existing channel workflow.

- [x] **Step 3: Run broad verification**

Run `go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model ./middleware -count=1` and `git diff --check`. Build `newapi:advanced-custom-routing-20260724` only after these checks pass.

### Task 5: Deploy With Per-Channel Rollback

**Files:** `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`

- [x] **Step 1: Inspect live advanced-custom usage before deployment**

Run a read-only production query that returns only channel ID, status, group, name, and models for type `ChannelTypeAdvancedCustom`; never print keys, headers, or channel settings. If active channels exist, snapshot their model lists and keep scheduled checks disabled until their exact model-list route is manually reviewed.

- [x] **Step 2: Deploy only after a candidate build and backup**

Transfer the candidate image, validate the archive, copy `/opt/newapi/docker-compose.yml` to `/opt/newapi/backups/`, replace only the `newapi` image, and run `docker compose up -d newapi`. Do not modify MySQL, Redis, volumes, group grants, account pools, pricing, or local experimental UI.

- [x] **Step 3: Verify and record rollback**

Require a healthy `newapi`, local `/api/status` success, public API `200`, and unauthenticated API/VIP `/v1/models` `401`. Record that scheduled discovery is disabled per channel by setting `UpstreamModelUpdateCheckEnabled` false; compose rollback restores the backed-up image line and runs `docker compose up -d newapi`.
