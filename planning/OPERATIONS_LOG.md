# Operations Log

## 2026-06-07 NewAPI HH CPA/Codex reimport final verification

- Rechecked the four manually supplied HH CPA/Codex accounts after the NewAPI
  Codex `account_id` metadata repair.
- Final NewAPI channels:
  - tag: `cpa-codex-manual-hh-reimport-20260607`
  - channel IDs: `54`, `55`, `56`, `57`
  - group: `gpt`
  - channel type: Codex (`57`)
  - enabled status: all 4 enabled
  - model list includes `gpt-5.5`
- Metadata verification:
  - all 4 channels store `type: codex`;
  - all 4 channels store a real ChatGPT account id, not `pending-*`;
  - all 4 channels have access and refresh tokens present.
- Smoke verification:
  - created a temporary NewAPI token scoped to group `gpt` and model
    `gpt-5.5`;
  - called each channel through
    `POST https://newapi.vyywcw.cn/v1/responses`;
  - request used `stream: true` and list-form `input`;
  - channels `54`, `55`, `56`, and `57` all reached
    `response.completed`.
- Cleanup:
  - temporary smoke-test tokens were deleted after testing;
  - no `codex-hh-smoke-20260607` or `codex-hh-token-debug-20260607`
    tokens remained.
- Note: earlier `Invalid token` smoke output was caused by the local SSH/script
  invocation breaking curl headers before the request reached NewAPI. After
  switching to base64-transferred remote scripts, token creation and
  channel-specific calls verified correctly.

## 2026-06-07 NewAPI Codex account_id metadata repair

- Investigated why the same CPA/Codex OAuth accounts were usable when imported
  by local cockpit-tools but failed in NewAPI manual DB smoke tests.
- Root cause: the manual NewAPI path had stored placeholder
  `account_id` values such as `pending-*`. NewAPI's Codex adapter sends this
  value as the `chatgpt-account-id` header, while cockpit-tools and Sub2API
  both extract the real `chatgpt_account_id` from OAuth JWTs before use.
- Additional note: Codex refresh tokens are rotating credentials. If
  cockpit-tools refreshes an imported account first, the old pasted
  `refresh_token` may no longer be reusable on the server; use the latest
  exported credential chain when re-importing.
- Created a MySQL backup before repair:
  `/opt/newapi/backups/channels-before-accountid-fix-20260607182523.sql`.
- Repaired all existing NewAPI Codex channels by decoding the current
  `access_token` JWT and updating only key metadata:
  - channels scanned: 26
  - channels updated: 26
  - channels skipped: 0
  - fields repaired: `account_id`, `email`, and missing `type`
- Verification:
  - all NewAPI Codex channels now have non-placeholder `account_id` metadata
    and email metadata;
  - a temporary NewAPI token in group `gpt` was created and deleted during
    smoke testing;
  - `https://newapi.vyywcw.cn/v1/responses` with `model: gpt-5.5`,
    list-form `input`, and `stream: true` returned HTTP 200 SSE
    `response.created`;
  - the same call with `stream: false` returned `Stream must be set to true`,
    so clients should use streaming for these Codex/ChatGPT accounts;
  - `gpt-5-codex` is still rejected upstream for these ChatGPT accounts.
- Import rule going forward: never use a synthetic `pending-*` account id for
  NewAPI Codex channels. Extract `chatgpt_account_id` from the access token or
  import through a flow that refreshes/normalizes credentials like
  cockpit-tools.

## 2026-06-07 NewAPI manual HH account import attempt

- Attempted to import two manually supplied CPA/Codex OAuth accounts into the
  NewAPI sidecar with tag `cpa-codex-manual-hh-20260607`.
- Import target:
  - group `gpt`
  - models include `gpt-5.5`
  - NewAPI channel type `57`
- Created a pre-import MySQL backup under `/opt/newapi/backups/`.
- Per-channel isolated smoke used temporary groups/tokens and restarted NewAPI
  to refresh DB-backed token/group caches.
- Both accounts reached upstream but returned HTTP 401:
  - `Could not parse your authentication token`
  - upstream code `unauthorized_unknown`
- Per operator preference, both unusable channels were deleted rather than
  left disabled.
- Final state:
  - `cpa-codex-manual-hh-20260607`: 0 remaining channels.
  - Temporary `tmp-hh-smoke-*` tokens: 0 remaining.
  - NewAPI app, MySQL, and Redis containers were healthy after cleanup.

## 2026-06-07 NewAPI manual HH retry import attempt

- Retried with two newly supplied CPA/Codex OAuth accounts using tag
  `cpa-codex-manual-hh-retry-20260607`.
- Import target:
  - group `gpt`
  - models include `gpt-5.5`
  - NewAPI channel type `57`
- Created a pre-import MySQL backup under `/opt/newapi/backups/`.
- The accounts were inserted as temporary channels `52` and `53` for isolated
  smoke testing.
- Per-channel smoke restarted NewAPI to refresh token/group caches before
  calling `https://newapi.vyywcw.cn/v1/responses` with `gpt-5.5`.
- Both channels returned HTTP 401 `Invalid token`.
- Per operator preference, both unusable channels were deleted rather than
  left disabled.
- Final state:
  - `cpa-codex-manual-hh-retry-20260607`: 0 remaining channels.
  - Temporary `tmp-hh-retry-smoke-*` tokens: 0 remaining.
  - `gpt` group remained at 10 enabled channels.
  - NewAPI app, MySQL, and Redis containers were healthy after cleanup.

## 2026-06-07 Upstream merge upgrade and redeploy

- Merged `upstream/main` into the private `main` branch.
- Upstream changes included:
  - server version sync to `0.1.134`;
  - OpenAI Responses sticky-account handling;
  - OpenAI-compatible stream field validation;
  - scheduler snapshot sync;
  - usage cache token split;
  - user-visible error views and ops error log improvements;
  - `skills/sub2api-admin` helper scripts/docs.
- Preserved private provider adapter work:
  - Kiro import endpoint and frontend modal;
  - Windsurf import endpoint and frontend modal;
  - Provider Adapters admin page and routes;
  - `kiro-web-adapter` files and deployment wiring.
- Local verification:
  - `go test ./internal/handler/admin -run 'Kiro|Windsurf|Codex'` passed.
  - `go test ./internal/pkg/apicompat ./internal/service -run 'Responses|Anthropic|OpenAI|Codex|Scheduler|RateLimit|Account'`
    passed.
  - `npm run test:run -- src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts`
    passed.
  - `npm run build` passed and produced updated embedded frontend assets.
- Deployment verification is recorded below after the server image switch.
- Server deployment:
  - Built image `sub2api-provider-adapters:upstream-merge-20260607-c75c6b1a`
    from commit `c75c6b1a`.
  - The first two server builds failed because `/var/lib/docker` ran out of
    space during Go compilation/linking. Cleaned Docker build cache, temporary
    build artifacts, and old `sub2api-provider-adapters:*` image tags while
    retaining current/recent rollback images.
  - Updated `/opt/sub2api/docker-compose.yml` to use the new image and
    recreated `sub2api`.
  - `docker compose ps` showed `sub2api` healthy on the new image.
  - `https://api.vyywcw.cn/health` returned HTTP 200.
  - `/app/sub2api --version` reported `Sub2API 0.1.134`, commit `c75c6b1a`.
  - Windsurf internal health remained OK: version `2.0.97`, 11 active
    accounts, 0 error accounts.
  - Kiro internal admin probe through `kiro-web-adapter:8991` returned
    `kiro-web` account/model status with runtime availability.

## 2026-06-07 Sub2API upstream/provider adapter recheck

- Checked upstream `upstream/main`; upstream is ahead of the private branch.
  Notable upstream changes include:
  - OpenAI Responses sticky account handling.
  - Stream field validation for OpenAI-compatible gateways.
  - Chat Completions/Responses bridge tests.
  - Scheduler snapshot sync.
  - Usage cache token split.
  - A new `skills/sub2api-admin` helper.
- Did not merge upstream in this pass because upstream does not include the
  private Kiro/Windsurf adapter files and a direct merge would require a
  private-patch preservation pass.
- Rechecked local import parser tests:
  - `go test ./internal/handler/admin -run 'Kiro|Windsurf'` passed.
- Server validation:
  - `https://api.vyywcw.cn/health` returned OK.
  - `windsurf-api` internal health returned OK, version `2.0.97`, with 11
    active accounts and 0 error accounts.
  - Found Kiro adapter wiring drift: Sub2API was still defaulting Kiro internal
    admin calls to `http://kiro-rs:8990`, while `/api/admin/credentials` is
    implemented by `kiro-web-adapter`.
  - Updated `/opt/sub2api/docker-compose.yml` and `.env` so Kiro admin/import
    calls use `http://kiro-web-adapter:8991` and the same adapter admin key as
    `kiro-web-adapter`'s key file.
  - Recreated `sub2api`; the container returned healthy.
  - Verified from inside the Sub2API container that
    `http://kiro-web-adapter:8991/api/admin/credentials` returns Kiro web
    account/model status. The response showed the `kiro-web` engine, runtime
    availability, and model coverage including Claude Opus/Sonnet 4.x lines.
- Updated deployment templates:
  - `deploy/docker-compose.yml` now documents Provider Adapter env wiring and
    defaults Kiro internal admin URL to `http://kiro-web-adapter:8991`.
  - `deploy/.env.example` now documents `WINDSURF_API_KEY`,
    `KIRO_ADMIN_API_KEY`, and adapter base URL settings.
- Protocol/import status:
  - Public Kiro changelog recheck showed recent CLI/Web/model updates,
    including CLI 2.6.0, Kiro Web session improvements, and Opus/Sonnet 4.x
    model availability/context updates. No public note found that changes the
    refresh-token or Kiro web adapter import shape used by this deployment.
  - Kiro-Go reference remains on `a2e3971` after fetch. Its current useful
    differences are still multi-account/model refresh/SSO-local-token import
    UX and structured tool handling, not a new mandatory token format for our
    deployed web adapter path.
  - Public Windsurf API docs describe the Enterprise service-key API for usage
    and configuration management. That is separate from the deployed internal
    WindsurfAPI adapter path, which remains healthy on version `2.0.97`; no
    live import break was found in this check.

## 2026-06-07 NewAPI CPA GPT batch import

- Imported CPA/Codex OAuth credentials into the NewAPI sidecar from
  `C:\Users\Administrator\Downloads\cpa_649f355d0aa5402a` without printing
  access tokens or refresh tokens.
- Local precheck:
  - 10 JSON files found.
  - 10 unique refresh tokens.
  - Access-token JWTs were not expired at import time.
- NewAPI changes:
  - Added channels with tag `cpa-codex-649f355d0aa5402a`.
  - Imported channels were assigned to group `gpt` and model list including
    `gpt-5.5`.
  - Created temporary per-channel smoke groups/tokens for verification, then
    removed the temporary tokens.
- Verification and cleanup:
  - Per-channel smoke against `https://newapi.vyywcw.cn/v1/responses` found
    9 usable channels and 1 upstream `token_invalidated` channel.
  - Per operator preference, the unusable channel was deleted rather than left
    disabled.
  - Final NewAPI state for `cpa-codex-649f355d0aa5402a`: 9 total, 9 enabled.
  - Final `gpt` group smoke with model `gpt-5.5` returned HTTP 200 and SSE
    `response.created`.

## 2026-06-06 NewAPI manual Codex account import

- Imported one manually supplied CPA/Codex OAuth account into the NewAPI
  sidecar without printing token values.
- NewAPI channel:
  - channel id `37`
  - tag `cpa-codex-manual-20260606`
  - group `gpt`
  - model list includes `gpt-5.5`
- Verification:
  - Isolated channel smoke through a temporary test group returned HTTP 200
    from `https://newapi.vyywcw.cn/v1/responses`.
  - Final `gpt` group smoke with model `gpt-5.5` returned HTTP 200 and SSE
    `response.created`.
- Cleanup:
  - The older `cpa-codex-18129d47107c4f90` batch repeatedly returned upstream
    `token_invalidated` errors during `gpt` group smoke.
  - Per operator preference, unusable channels were deleted instead of left
    disabled. The old bad batch now has 0 channels left in group `gpt`.

## 2026-06-05 NewAPI CPA GPT group import

- Imported CPA/Codex OAuth credentials into the NewAPI sidecar from
  `C:\Users\Administrator\Downloads\cpa_18129d47107c4f90` without printing
  access tokens or refresh tokens.
- Local precheck:
  - 20 JSON files found.
  - 20 unique refresh tokens.
  - Access-token JWTs were not expired at import time.
- NewAPI changes:
  - Added 20 enabled channels with tag `cpa-codex-18129d47107c4f90`.
  - Set all imported channels to group `gpt`.
  - Added `gpt` to `GroupRatio` and `UserUsableGroups`.
  - Created a NewAPI token named `gpt`, limited to `gpt-5.5`, with
    cross-group retry disabled. The token value is intentionally not stored in
    Git.
- Server safety:
  - Created a pre-import NewAPI MySQL backup under `/opt/newapi/backups/`.
  - Temporary SQL/import files were removed after execution.
- Verification:
  - NewAPI channel summary for the new tag: 20 total, 20 enabled.
  - Public smoke against `https://newapi.vyywcw.cn/v1/responses` with the
    `gpt` token, `stream=true`, and `model: gpt-5.5` returned HTTP 200 and
    SSE `response.created`.

## 2026-06-04 Kiro adapter admin visibility

- Reviewed the existing Sub2API Kiro/Windsurf adapter admin area without
  inspecting `D:\wflogin\注册机相关项目`.
- Found the main Kiro display gap:
  - Sub2API already probes Kiro runtime account data at
    `/api/admin/credentials`.
  - The deployed `kiro-web` adapter only exposed runtime/model endpoints and
    `/admin/usage`, so the Kiro Runtime panel could show sparse or empty
    account data even when the adapter itself could serve model requests.
- Compared against the newer Kiro-Go local gateway pattern and kept only the
  useful management-surface ideas: account pool summary, sanitized account
  fields, model counts, runtime status, and stable refresh behavior.
- Code changes:
  - Added redacted `kiro-web` management endpoints:
    `/api/admin/credentials`, `/api/admin/accounts`, and `/api/admin/status`.
  - The admin response now includes account id/label/email where available,
    auth method, provider, region, disabled state/reason, token status, Profile
    ARN presence, supported model count, and default model metadata, without
    returning raw access or refresh tokens.
  - Improved the Sub2API Adapter Admin Kiro account table to show auth,
    Profile ARN, model count, disabled reason, and error/cooldown fields.
  - Refresh now keeps the previous response visible if a later refresh fails,
    reducing the blank/abnormal component state reported in the admin UI.
- Local verification:
  - `python -m py_compile adapters\kiro-web\kiro_web_adapter.py` passed.
  - `go test ./internal/handler/admin -run Kiro` passed.
  - `npm run build` from `frontend` passed.
- Follow-up fix:
  - Kiro runtime admin fetches now treat `/api/admin/*` like `/v1/*` and use
    the internal adapter key first. This matches `kiro-web-adapter`, whose
    redacted management endpoints share the same internal key as model calls.
- Server deployment:
  - Deployed Sub2API image
    `sub2api-provider-adapters:kiro-admin-20260604b`.
  - Deployed Kiro web adapter image
    `sub2api-kiro-web-adapter:admin-20260604`.
  - Backed up compose before image and key-alignment changes.
  - Aligned the live Sub2API Kiro internal-key environment with the
    `kiro-web-adapter` key file without printing the secret.
- Post-deploy verification:
  - `https://api.vyywcw.cn/health` returned `{"status":"ok"}`.
  - `docker compose ps` showed `sub2api` and `sub2api-kiro-web-adapter`
    running on the new images.
  - From inside the Sub2API container, the redacted Kiro management endpoint
    `http://kiro-web-adapter:8991/api/admin/credentials` returned account
    metadata through `x-api-key`.
  - The returned account payload did not include raw access-token or
    refresh-token fields in the sampled admin response.

## 2026-06-02 CPA Codex passthrough failover fix

- Investigated why CPA/Codex accounts imported through Sub2API looked broadly
  rate-limited while native NewAPI-style CPA routing worked better.
- Root cause found:
  - Codex/CPA accounts must use the OpenAI Responses/Codex passthrough path.
  - Sub2API passthrough previously only failed over on `429` and `529`, so
    bad CPA tokens returning `401` could leak directly to the client.
  - The OpenAI passthrough handler also stopped after the configured 10 account
    switches, which is too small for a 200-account CPA pool containing a mix of
    revoked and upstream-limited accounts.
- Code changes:
  - CPA Codex imports now default `extra.openai_passthrough=true`.
  - CPA import identity matching now prefers refresh token/user identity over
    shared `chatgpt_account_id`, avoiding accidental duplicate merges.
  - OpenAI passthrough failover now covers `401`, `402`, `403`, `429`, `529`,
    and `5xx`, while preserving direct passthrough for client-side `400`.
  - OpenAI passthrough accounts get a 50-switch minimum failover budget, and are
    not prematurely stopped by the OpenAI OAuth 429 storm shortcut.
- Local verification:
  - `go test ./internal/handler/admin ./internal/handler ./internal/service`
    passed.
- Server deployment:
  - Deployed image `sub2api-provider-adapters:cpa-import-20260602c`.
  - Public health: `https://api.vyywcw.cn/health` returned `{"status":"ok"}`.
- Live GPT5.5 group smoke:
  - `/v1/messages` with `claude-opus-4-8` returned HTTP 200 and routed upstream
    to `/v1/responses` with `gpt-5.5`.
  - `/v1/responses` with `gpt-5.5` returned HTTP 200 after skipping revoked and
    upstream-limited CPA accounts; logs showed `max_switches=50`.
  - Successful accounts were from import batch `cpa_c9fc1ffeabda4f2b`.
- Post-smoke live DB summary for group `GPT5.5`:
  - OpenAI active accounts: 230.
  - OpenAI schedulable accounts: 230.
  - New CPA batch `cpa_c9fc1ffeabda4f2b`: 179 active/schedulable, 21 error.
  - The errors observed during smoke were upstream token revocations, not leaked
    Sub2API client responses.

## 2026-06-02 CPA Codex batch import

- Inspected local CPA export folder
  `C:\Users\Administrator\Desktop\cpa_c9fc1ffeabda4f2b` without printing token
  values.
- Confirmed all 200 files are Codex session JSON records with:
  - `type=codex`
  - `access_token`
  - `refresh_token`
- Local JWT metadata check showed all 200 records have refresh tokens, OpenAI
  Codex auth claims, no duplicate refresh tokens within this batch, and
  unexpired access tokens.
- Imported all 200 records through
  `POST /api/v1/admin/accounts/import/codex-session`, binding them to group
  `GPT5.5` (group id 8), with `skip_default_group_bind=true`.
- Post-import live DB summary:
  - `GPT5.5` bound accounts: 270.
  - `GPT5.5` active accounts: 253.
  - `GPT5.5` currently schedulable accounts: 240.
  - This CPA batch contributes 200 active accounts.
- Public smoke through the `GPT5.5` group:
  - `/v1/models` returned HTTP 200.
  - `/v1/chat/completions` with `gpt-5.5` returned HTTP 200 with `ok`.

## 2026-06-01 CPA Codex batch import

- Inspected local CPA export folder
  `C:\Users\Administrator\Desktop\cpa_7a87050f07484b49` without printing token
  values.
- Confirmed all 52 files are Codex session JSON records with:
  - `type=codex`
  - `access_token`
  - `refresh_token`
- Local JWT metadata check showed all 52 records have refresh tokens, OpenAI
  Codex auth claims, plan `team`, and unexpired access tokens.
- Imported all 52 records through
  `POST /api/v1/admin/accounts/import/codex-session`, binding them to group
  `GPT5.5` (group id 8).
- Import result: 52 created, 0 updated, 0 skipped, 0 failed.
- Post-import live DB summary:
  - OpenAI OAuth active accounts: 52.
  - Existing OpenAI OAuth error accounts remain: 100.
  - `GPT5.5` group now has 52 active accounts from this CPA batch.
- Public smoke through the `GPT5.5` group:
  - `/v1/models` returned the GPT model list including `gpt-5.5`.
  - `/v1/chat/completions` with `gpt-5.5` returned HTTP 200 with `ok`.

## 2026-06-01 Kiro Opus group split and GPT 5.5 verification

- Rechecked live groups after the Windsurf-only correction and confirmed there
  were no separate Kiro-named user groups online; Kiro accounts were only bound
  inside `provider-mixed`.
- Added internal account `kiro-web-internal-anthropic` by reusing the existing
  Kiro Web adapter credentials with `platform=anthropic`.
- Added Kiro-specific groups:
  - `kiro-opus4.6`: `platform=anthropic`, model list `claude-opus-4.6`, bound
    to `kiro-web-internal-anthropic`.
  - `kiro-opus4.7`: `platform=anthropic`, model list `claude-opus-4.7`, bound
    to `kiro-web-internal-anthropic`.
  - `kiro-gpt5.5`: created as an OpenAI placeholder, then disabled after live
    smoke showed Kiro upstream returns `Invalid model ID` for `gpt-5.5`.
- Validation:
  - Public `/v1/models` for `kiro-opus4.6` returned `claude-opus-4.6`.
  - Public `/v1/messages` for `kiro-opus4.6` returned HTTP 200 with `ok`.
  - Public `/v1/models` for `kiro-opus4.7` returned `claude-opus-4.7`.
  - Public `/v1/messages` for `kiro-opus4.7` returned HTTP 200 with `ok`.
- Temporary smoke-test API keys were created and soft-deleted after validation.
- The Kiro Web adapter model list and official Kiro model docs do not expose
  GPT 5.5 at this time, so do not enable `kiro-gpt5.5` unless upstream support
  is verified again.

## 2026-06-01 User key Claude/CCS model config deployment

- Fixed user-facing API key group DTOs to include `models_list_config`, allowing
  the “使用密钥” modal and CC-Switch import link to generate client model
  mappings from the selected group.
- Claude Code setup now includes model environment variables for Sonnet, Opus,
  Haiku, and the small/fast model when the group has custom model-list config.
- CC-Switch import links for Claude clients now include `sonnetModel`,
  `opusModel`, and `haikuModel` when resolvable from the group model list.
- Validation before deploy:
  - `go test ./...` from `backend`.
  - `npm run build` from `frontend`.
  - Targeted Vitest coverage for `ccswitchImport` and `UseKeyModal`.
- Deployed Sub2API image
  `sub2api-provider-adapters:ccs-model-config-20260601`.
- Backed up compose before switching:
  `/opt/sub2api-backups/docker-compose-20260601-084626-pre-ccs-model-config-20260601.yml`.
- Post-deploy checks:
  - Container `sub2api` reported running/healthy on the new image.
  - Public health through the real server IP returned HTTP 200.
  - Deployed frontend `KeysView` asset contains the new Claude env/model import
    fields (`ANTHROPIC_DEFAULT_OPUS_MODEL`, `sonnetModel`, `opusModel`,
    `haikuModel`, and `models_list_config`).
- Follow-up validation:
  - Found live group `provider-mixed` still had empty `models_list_config`, so
    user-side exports would not pin Claude models for that group.
  - Updated `provider-mixed` to expose
    `claude-sonnet-4.6` and `claude-opus-4.7`, then restarted `sub2api` to clear
    runtime caches.
  - Server-side smoke through `provider-mixed` returned HTTP 200 with content for
    both `claude-sonnet-4.6` and `claude-opus-4.7`.

## 2026-06-01 Windsurf custom model-list correction

- Corrected live `models_list_config` entries for Windsurf custom groups from
  short aliases to the model IDs exposed by the internal Windsurf adapter:
  - `windsurf-opus4.6`: `claude-opus-4.6`,
    `claude-opus-4.6-thinking`.
  - `windsurf-opus4.7`: `claude-opus-4-7-*`.
  - `windsurf-gpt5.5`: `gpt-5.5*`.
  - `windsurf-gpt5.4`: `gpt-5.4-*`.
- Updated Sub2API so Anthropic-compatible groups with explicit
  `models_list_config` return that configured list from `/v1/models` instead of
  filtering non-default custom IDs to an empty list.
- Deployed Sub2API image
  `sub2api-provider-adapters:custom-models-list-v2-20260601`.
- Validation:
  - `go test ./internal/handler -run TestGatewayModels`.
  - `go test ./...` from `backend`.
  - `/v1/models` through active keys now returns the corrected model lists for
    `provider-mixed`, `windsurf-opus4.6`, `windsurf-opus4.7`,
    `windsurf-gpt5.5`, and `windsurf-gpt5.4`.
  - `provider-mixed` smoke returned HTTP 200 with content for
    `claude-sonnet-4.6` and `claude-opus-4.7`.
  - Dedicated Windsurf groups still returned HTTP 503 through Sub2API because
    their only bound upstream account, `windsurf-internal-anthropic`, is in
    `error` state. Direct internal Windsurf adapter checks returned
    `model_not_entitled` for the corrected Opus/GPT model IDs, so this requires
    a usable/entitled Windsurf upstream account rather than another export
    config change.

## 2026-06-01 Kiro import group binding fix deployed

- Added Kiro import support for `group_ids` so the admin import flow can bind
  the internal Kiro Gateway upstream account to the selected Anthropic groups.
- The import flow now creates or updates Sub2API account
  `kiro-gateway-internal-anthropic` as `platform=anthropic,type=apikey`,
  using the internal Kiro runtime API key from server environment and
  `base_url=http://kiro-gateway:8000`.
- Deployed Sub2API image
  `sub2api-provider-adapters:kiro-import-groups-20260601`.
- Public health returned `{"status":"ok"}` after deploy.
- Verified `windsurf-api`, `kiro-rs`, and `kiro-gateway` remain internal-only
  with no host-published ports.

## 2026-05-21 KAM/Kiro-Go routing research

- Rechecked `chaogei/Kiro-account-manager`: latest observed HEAD
  `7ad57fd26e67b3ea91b780b2ca983c78737ed88a`, tag/package version `v1.6.6`.
- Added `Quorinex/Kiro-Go` as a new Kiro reference: latest observed HEAD
  `68110f30e01b4789b1baf7c40b992e4d71c68619`, version `1.0.8`.
- `Kiro-Go` passed `go test ./...` locally.
- Current conclusion: KAM is stronger than old `kiro.rs` for account-pool
  strategy, route fallback, token refresh locking, model discovery, and request
  lifecycle handling. Kiro-Go is easier to borrow from for a server-side Go
  implementation.
- Added `planning/KIRO_KAM_ROUTE_RESEARCH_2026-05-21.md`.
- Added `planning/KIRO_RUNTIME_ADMIN_CONSOLE_PLAN_2026-05-21.md`.
- Updated the reference policy: future Kiro runtime work should prefer
  Sub2API private adapter + `kiro-web`, Kiro-Go, and KAM. `kiro.rs` is now
  legacy/reference unless a task specifically requires its old IDE/API-key
  behavior.

## 2026-05-20 Kiro/Windsurf recheck after user-provided account context

- Rechecked upstream HEADs with `git ls-remote`; `dwgx/WindsurfAPI`,
  `Jwadow/kiro-gateway`, `hank9999/kiro.rs`, and
  `chaogei/Kiro-account-manager` remain at the previously documented heads.
- Re-tested the live server Kiro credential without printing secrets. The
  credential has refresh/access token material and a social profile ARN, but no
  persisted `machineId`, `provider`, `clientId`, or `clientSecret`.
- Official Kiro `ListAvailableModels` from the US server still returned only:
  `deepseek-3.2`, `minimax-m2.5`, `minimax-m2.1`, `glm-5`, and
  `qwen3-coder-next`.
- Direct generate smoke returned HTTP 200 for `qwen3-coder-next` on
  `codewhisperer`, `q`, and `runtime` endpoints; `claude-sonnet-4.6` and
  `claude-opus-4.6` returned `INVALID_MODEL_ID` on those endpoints.
- Added `tools/kiro_server_probe.py`, a dependency-free sanitized probe script
  for future server-side Kiro model discovery and generate smoke.
- Added `planning/KIRO_WINDSURF_RECHECK_2026-05-20.md` with current conclusions,
  references, and the acceptance gate before exposing Kiro Claude-family models.

## 2026-05-20 Provider adapter protocol research

- Mirrored or refreshed additional reference projects:
  - `jlcodes99/cockpit-tools` at `f6c92cbbdd86357405a0589ee0a09bae855d26e2`.
  - `chaogei/Kiro-account-manager` at
    `7ad57fd26e67b3ea91b780b2ca983c78737ed88a` / `v1.6.6`.
  - `pfcoperez/windsurfinabox` at
    `86a7da7821413497756bc53f85508eeeb8b18945`.
  - `dwgx/WindsurfAPI` at `c028576a56b9fa19f84810643610cae4af824238` /
    `v2.0.96`.
- Confirmed `dwgx/WindsurfAPI v2.0.96` already includes the important
  `auth1_ -> WindsurfPostAuth -> devin-session-token$ as apiKey` workaround
  after the old Windsurf one-time-token path became unreliable.
- Pulled `windsurf-api` and `kiro-gateway` images on the server and recreated
  if needed; both containers remained running/healthy with no public host ports.
- Re-tested Kiro official model discovery from the server with the current
  Kiro social credential and profile ARN. `ListAvailableModels` returned only:
  `deepseek-3.2`, `minimax-m2.5`, `minimax-m2.1`, `glm-5`, and
  `qwen3-coder-next`.
- Direct server calls to Kiro `q.<region>.amazonaws.com/generateAssistantResponse`
  for `claude-opus-4.7`, `claude-opus-4.6`, and `claude-sonnet-4.6` still
  returned `INVALID_MODEL_ID`; `qwen3-coder-next` returned HTTP 200.
- Changing `x-amzn-kiro-agent-mode` between `spec` and `vibe`, and testing
  `AI_EDITOR`/`MD_IDE` origins, did not make Claude-family models appear in the
  official model list.
- Added `planning/PROVIDER_ADAPTER_PROTOCOL_RESEARCH_2026-05-20.md`.

## 2026-05-20 Provider mixed group and Kiro model expansion

Changes:

- Confirmed group `windsurf-smoke` was already exclusive (`is_exclusive=true`),
  so it was not public to all users.
- Renamed group `windsurf-smoke` to `provider-mixed`.
- Renamed public API key label `server-windsurf-smoke` to
  `server-provider-mixed`.
- Enabled group model routing for Kiro Gateway models:
  - `qwen3-coder-next`
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.1`
  - `minimax-m2.5`
- Added `minimax-m2.1` to the Sub2API Kiro Gateway upstream account
  `model_mapping`.

Validation:

- Direct Kiro Gateway smoke returned HTTP 200 for:
  - `qwen3-coder-next`
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.1`
  - `minimax-m2.5`
- Direct Kiro Gateway smoke returned HTTP 400
  `Invalid model ID or insufficient subscription level to use it` for the
  Kiro Claude-family model IDs, so those were not opened in Sub2API.
- Public Sub2API `/v1/messages` smoke through `https://api.vyywcw.cn/` returned
  HTTP 200 for all five Kiro Gateway models above.
- Public Sub2API `/v1/models` includes `minimax-m2.1`, `minimax-m2.5`,
  `qwen3-coder-next`, `deepseek-3.2`, `glm-5`, and Windsurf
  `claude-sonnet-4.6`.

## 2026-05-20 Windsurf model exposure sync

Findings:

- The Windsurf dashboard reported one active Trial/Pro account.
- Dashboard capability summary: 110 total model entries, 81 marked `ok`, and
  29 not usable.
- The Sub2API Windsurf account previously exposed a broader 128-entry
  `model_mapping`, including models that the current Windsurf account did not
  have entitlement for.

Changes:

- Synced `windsurf-internal-anthropic` `model_mapping` to the dashboard `ok`
  capabilities plus usable aliases.
- Removed deprecated direct-smoke failures:
  - `gpt-4o-mini`
  - `grok-3-mini`

Validation:

- Public Sub2API `/v1/messages` returned HTTP 200 for sampled Windsurf models:
  - `claude-sonnet-4.6`
  - `claude-opus-4.6`
  - `gpt-5.2`
  - `gemini-2.5-pro`
  - `o3`
  - `swe-1.5-fast`
- Direct Windsurf calls for `gpt-4o-mini` and `grok-3-mini` returned HTTP 410
  `model_deprecated`, so they are intentionally not exposed through
  `/v1/models`.
- Public Sub2API `/v1/models` now reports 86 total models for the
  `provider-mixed` key and no longer includes `gpt-4o-mini` or `grok-3-mini`.

## 2026-05-19 Sub2API image a147caa0 deployment

Deployment:

- Built local fork image on the US server from git archive:
  `sub2api-provider-adapters:a147caa0`.
- Previous production image:
  `sub2api-provider-adapters:c4cefc76`.
- Server data backup before image build:
  `/opt/sub2api-backups/sub2api-20260519-123646.tar.gz`.
- Compose backup before switching the image tag:
  `/opt/sub2api-backups/docker-compose-20260519-044206-pre-a147caa0.yml`.

Post-deploy validation:

- `sub2api` healthy on `127.0.0.1:8080->8080`.
- Public `https://api.vyywcw.cn/` returned HTTP 200.
- Public `https://www.vyywcw.cn/` returned HTTP 200.
- Internal adapter ports remained unpublished:
  - `windsurf-api` `3003/tcp`: `null`
  - `kiro-rs` `8990/tcp`: `null`
  - `kiro-gateway` `8000/tcp`: `null`
- Internal `kiro-gateway /v1/models` returned 13 model IDs.
- Public Sub2API smoke with `qwen3-coder-next` returned HTTP 200 and text
  `ok`.

## 2026-05-19 Kiro.rs Claude correction

Correction:

- Local `D:\wflogin\kiro.rs-master` can access Claude through its local runtime
  configuration.
- Local smoke against `claude-sonnet-4-6` returned HTTP 200 and text `ok`.
- The local config includes a proxy URL, while the server `kiro-rs` service
  uses direct egress.

Server recheck:

- Copied the local full `credentials.json` to
  `/opt/sub2api/kiro-rs/config/credentials.json` after backing up the previous
  server credential file.
- Restarted `kiro-rs`.
- Forced token refresh through the server `kiro-rs` admin API.
- Server direct `kiro-rs` smoke with `claude-sonnet-4-6` still returned
  `INVALID_MODEL_ID`.

Conclusion:

- Do not describe this as “kiro.rs cannot access Claude.”
- The more accurate statement is that local `kiro.rs` with the local proxy
  egress can access Claude, while the current US server direct egress cannot.
- A separate `kiro.rs` fix is still needed because its admin add-credential
  path drops `accessToken`, `profileArn`, and `expiresAt`; direct file import
  preserves those fields.

## 2026-05-19 Kiro Gateway runtime validation

Reference refresh:

- Official Sub2API upstream advanced to `14f54be0`; merged into the private
  fork after committing the Kiro credential metadata import support.
- `hank9999/kiro.rs` remained at observed HEAD `f1bbe9f`, latest observed tag
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remained at observed HEAD `a5292ca`, latest observed
  tag `v2.3`.
- `dwgx/WindsurfAPI` remained at observed HEAD `c028576`, tag `v2.0.96`.

Server changes:

- Added `kiro-gateway` to `/opt/sub2api/docker-compose.yml` as an
  internal-only service.
- Compose backup before the change:
  `/opt/sub2api-backups/docker-compose-20260519-042547-pre-kiro-gateway.yml`.
- Removed the previous ad-hoc `sub2api-kiro-gateway` container and recreated it
  through Docker Compose.
- Kept `kiro-gateway` unexposed publicly: no host port and no Caddy route.
- Persisted runtime paths:
  - `/opt/sub2api/kiro-gateway/creds`
  - `/opt/sub2api/kiro-gateway/debug_logs`
  - `/opt/sub2api/kiro-gateway/state`

Kiro account handling:

- The user-provided Kiro credential was treated as a secret and stored only on
  the server runtime path, not in Git.
- The credential initialized successfully in `kiro-gateway`.
- Claude-family Kiro models remain disabled in Sub2API mapping because direct
  runtime smoke still returns model/subscription errors.

Sub2API upstream account:

- Added database account `kiro-gateway-internal-anthropic`.
- Platform/type: `anthropic` / `apikey`.
- Base URL: `http://kiro-gateway:8000`.
- Added to group `windsurf-smoke`.
- Enabled only these smoke-passed Kiro models:
  - `deepseek-3.2`
  - `glm-5`
  - `minimax-m2.5`
  - `qwen3-coder-next`

Validation:

- `kiro-gateway` health became healthy after compose start.
- Internal `GET http://kiro-gateway:8000/v1/models` returned 13 model IDs.
- Internal Sub2API smoke with `qwen3-coder-next` returned HTTP 200 and text
  `ok`.
- Public Sub2API smoke through `https://api.vyywcw.cn/v1/messages` with
  `deepseek-3.2` returned HTTP 200 and text `ok`.

Operational note:

- Do not widen Kiro model mapping until a model-specific smoke passes.
- If the Kiro runtime account fails, disable or remove only
  `kiro-gateway-internal-anthropic`; Windsurf uses a separate internal account.

This log records local upstream checks, server maintenance, and deployment notes
for the private fork. Do not store secrets here.

## 2026-05-18 Upstream and Server Maintenance

Operator context:

- Local workspace: `D:\wflogin\sub2api-private`
- Production server path: `/opt/sub2api`
- Public URL checked: `https://api.vyywcw.cn/`

### Upstream checks

Sub2API official upstream:

- Remote: `https://github.com/Wei-Shaw/sub2api.git`
- Previous observed base in private fork: `6e66edb chore: update sponsors`
- New upstream head: `f5bd25be Merge pull request #2530 from lyen1688/fix/openai-responses-sse-terminal`
- Included fix commit: `cc5328c4 修复 OpenAI Responses SSE 终止事件识别`
- Action: merged `upstream/main` into private `main`
- Private fork merge commit: `b41778ef Merge remote-tracking branch 'upstream/main'`
- Push status: pushed to `origin/main`

WindsurfAPI:

- Remote: `https://github.com/dwgx/WindsurfAPI`
- Local path: `D:\wflogin\_github_research\WindsurfAPI`
- Current observed head: `c028576 release: 2.0.96`
- Current observed latest tag: `v2.0.96`
- Action: fetched successfully after one transient GitHub connection timeout
- Result: no upstream changes to apply

WindsurfPoolAPI:

- Remote: `https://github.com/guanxiaol/WindsurfPoolAPI`
- Local path: `D:\wflogin\_github_research\WindsurfPoolAPI`
- Current observed latest tag: `v2.0.7`
- Result: no upstream changes observed

Kiro:

- Remote: `https://github.com/hank9999/kiro.rs`
- New local git checkout: `D:\wflogin\_github_research\kiro.rs`
- Current observed head: `f1bbe9f chore(build): 更新 Node.js 至 22，pnpm 至 11`
- Current observed latest tag: `v2026.3.1`
- Action: cloned as a real git checkout for future tracking

kiro-gateway:

- Remote: `https://github.com/jwadow/kiro-gateway`
- Observed branch head before timeout: `b370ec5de14d3cc48cd575f9b71cab1ff9e3832f refs/heads/main`
- Action: tag fetch hit a transient GitHub connection timeout
- Follow-up: retry before using as a source of truth

### Local validation

Command:

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/service 42.458s
```

Note:

- Initial attempt with the default Go proxy failed while downloading Go
  toolchain `go1.26.3`.
- Retrying with `GOPROXY=https://goproxy.cn,direct` succeeded.

### Server check

Current server deployment:

- Compose path: `/opt/sub2api`
- Running image: `weishaw/sub2api:latest`
- Current image ID after update: `abd6d88929c4`
- Current companion images after update:
  - `caddy:2-alpine` image ID `86deaf5e3d34`
  - `postgres:18-alpine` image ID `96d56f7f57c6`
  - `redis:8-alpine` image ID `d146f83b1e0f`

Backup created before update:

```text
/opt/sub2api-backups/sub2api-20260518-142850.tar.gz
```

Update command:

```bash
cd /opt/sub2api
./update.sh
```

Result:

- `docker compose pull` completed.
- `docker compose up -d --remove-orphans` completed.
- Internal health check passed: `GET http://127.0.0.1:8080/health`.
- Public HEAD request to `https://api.vyywcw.cn/` returned HTTP 200.
- Containers remained healthy.
- No Windsurf/Kiro internal adapter services are deployed yet.

Admin account note:

- `.env` contains initialization values only.
- Actual admin user in database is:
  - email: `adminyu@vyywcw.cn`
  - username: `yu`
  - role: `admin`
  - 2FA: disabled
- The admin password is stored as a bcrypt hash and cannot be read back in
  plaintext.
- No password reset was performed because the password was remembered.

### Follow-ups

- [ ] Retry `kiro-gateway` tag fetch when GitHub connectivity is stable.
- [ ] Decide whether to update server `.env` comments/values to avoid confusion
      between initial admin email and actual admin user. Do not change secrets
      without a maintenance window.
- [ ] Add WindsurfAPI as an internal service only after collecting account
      tokens and confirming language server binary handling.
- [ ] Add Kiro proxy as an internal service only after deciding whether to use
      `kiro.rs` image/build or build from source.
- [ ] Add explicit backup paths for future Windsurf/Kiro data directories once
      those services exist.

## 2026-05-18 Windsurf Stage A implementation

Code changes:

- Added `POST /api/v1/admin/accounts/import/windsurf`.
- Added safe parser for `token`, `tokens`, `raw`, and `accounts`.
- Extended parser to support Windsurf `email/password` and raw `email----password` lines.
- Added duplicate detection inside one request.
- Added forwarding to internal `WindsurfAPI /auth/login`.
- Added recursive upstream response redaction.
- Added secret-hash idempotency payload so raw Windsurf tokens/passwords/emails are not stored in Sub2API idempotency records.

Reference checked:

- `dwgx/WindsurfAPI` stayed at `c028576 release: 2.0.96`, tag `v2.0.96`.
- Confirmed `/auth/login` accepts `token`, `api_key`, and `accounts`.
- Confirmed source path also accepts account items containing `email` and `password`.
- Confirmed accepted auth headers include `Authorization: Bearer <key>` and `x-api-key`.

Validation:

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run Windsurf
go test ./internal/handler/admin
go test ./internal/server
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin
?  github.com/Wei-Shaw/sub2api/internal/server [no test files]
```

Not deployed yet:

- Server still needs a custom fork image.
- Server still needs internal `windsurf-api` compose service.
- Server still needs Sub2API env `WINDSURF_ADAPTER_INTERNAL_BASE_URL` and `WINDSURF_ADAPTER_INTERNAL_API_KEY`.

## 2026-05-18 Windsurf Stage A server deployment

Server status after deployment:

- Compose path: `/opt/sub2api`
- Public Sub2API domains:
  - `https://api.vyywcw.cn/`
  - `https://www.vyywcw.cn/`
- Deployed Sub2API fork image: `sub2api-provider-adapters:544f553b`
- Internal Windsurf adapter image: `ghcr.io/dwgx/windsurf-api:latest`
- WindsurfAPI upstream version observed in reference repo and container logs:
  `v2.0.96` / commit `c028576`
- Sub2API upstream check: local fork is ahead of official upstream; upstream
  had no new commits to merge at the time of the check.

Services:

- `sub2api` healthy after restart.
- `windsurf-api` healthy and reachable only inside the Docker network.
- `postgres`, `redis`, and `caddy` remained healthy.
- No Caddy public route and no host port were added for `windsurf-api`.

Server configuration added:

- `WINDSURF_API_KEY`
- `WINDSURF_DASHBOARD_PASSWORD`
- `WINDSURF_ADAPTER_INTERNAL_BASE_URL=http://windsurf-api:3003`
- `WINDSURF_ADAPTER_INTERNAL_API_KEY=<WINDSURF_API_KEY>`
- `WINDSURF_ADAPTER_TIMEOUT_SECONDS=30`
- `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true`
- `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`

Important: the security URL allowlist was loosened because Sub2API needed to
call an internal Docker service over plain HTTP. Keep `windsurf-api` internal
only; do not add a public route for it.

Import endpoint verified on server:

- `POST /api/v1/admin/accounts/import/windsurf`
- Email/password import succeeded.
- Top-level `api_key` import succeeded after commit `544f553b`.
- `accounts[].api_key` batch-shaped import succeeded after commit `544f553b`.

Model smoke after deployment:

- Internal WindsurfAPI direct `/v1/messages`:
  - `gemini-2.5-flash` returned HTTP 200.
  - `claude-sonnet-4.6` returned HTTP 200.
  - `claude-4.5-haiku` returned HTTP 200.
- Public Sub2API `/v1/messages` through the `windsurf-smoke` group:
  - `gemini-2.5-flash` returned HTTP 200.
  - `claude-sonnet-4.6` returned HTTP 200.

Incident found and fixed during smoke:

- Symptom: Sub2API public route returned 403/503 after WindsurfAPI rejected
  `gemini-2.5-flash` with `model_not_entitled`.
- Direct WindsurfAPI account state showed the account capability probe had
  `gemini-2.5-flash` as successful, but `availableModels` did not include it.
- Cause: WindsurfAPI `getAvailableModelsForAccount` only includes enum-keyed
  models when capability reason is `user_status`; this account had a canary
  `success` capability for `gemini-2.5-flash`, so preflight excluded it.
- Server-side short-term fix: set the Windsurf trial account tier to `pro` via
  the internal dashboard API, then clear the Sub2API upstream account error
  state and restart `sub2api`.
- Long-term fix candidate: in our adapter notes or future WindsurfAPI fork,
  treat `capabilities[model].ok === true` as available even when the reason is
  `success`, unless an explicit blocklist or `not_entitled` result exists.

Operational note:

- A single bad upstream 403 can mark the Sub2API upstream account `error`.
  Before retesting after a known adapter-side fix, clear only transient/error
  state for the internal upstream account and restart `sub2api` to refresh the
  scheduler snapshot.

## 2026-05-19 Upstream sync and Windsurf UI import

Reference check:

- Official Sub2API upstream advanced by two commits:
  - `164e2f61 fix: add keepalive for Anthropic passthrough streams`
  - `1d78dde8 Merge pull request #2552 from lyen1688/fix/anthropic-passthrough-keepalive`
- Merged `upstream/main` into the private fork without conflicts.
- `dwgx/WindsurfAPI` remained at `c028576 release: 2.0.96`, tag `v2.0.96`.

Code changes:

- Added the admin UI entry `Import Windsurf` under account management more actions.
- Added a Windsurf import modal that calls `POST /api/v1/admin/accounts/import/windsurf`.
- UI import modes:
  - `token`: one or more Windsurf tokens.
  - `api_key`: one or more Codeium/Windsurf API keys, submitted as `api_key`.
  - `email_password`: one `email----password` pair per line.
  - `json`: full backend-supported request object or account array.
- Added frontend request/response types and an API client wrapper with an
  idempotency key header.
- Added pure parser tests for the UI payload builder.

Validation:

```powershell
$env:GOCACHE='C:\Users\Administrator\.codex\memories\gocache'
go test ./internal/handler/admin -run Windsurf
go test ./internal/service -run 'AnthropicAPIKeyPassthrough'

$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm exec vitest run src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin
ok github.com/Wei-Shaw/sub2api/internal/service
1 frontend test file passed, 6 tests passed
vue-tsc passed
```

Operational note:

- `pnpm` v11 may require build-script approval for `esbuild` and `vue-demi`
  on a fresh local checkout. For local verification, using
  `npm_config_dangerously_allow_all_builds=true` avoids committing a generated
  `pnpm-workspace.yaml` approval file.

## 2026-05-19 Kiro adapter import bridge

Reference check:

- `hank9999/kiro.rs` latest observed HEAD: `f1bbe9f`.
- `Jwadow/kiro-gateway` latest observed HEAD: `a5292ca`; latest observed tag
  line includes `v2.3`.
- `dwgx/WindsurfAPI` remained at `c028576`, tag `v2.0.96`.
- Official Sub2API upstream had advanced again; merged `upstream/main` and
  resolved the only conflict in `Dockerfile` by keeping the fork's pinned
  `PNPM_VERSION=9.15.9` while absorbing upstream's pnpm-v9 build fix.

Code changes:

- Added `POST /api/v1/admin/accounts/import/kiro`.
- Added admin UI entry `Import Kiro`.
- Added Kiro import modal with modes:
  - refresh token lines.
  - Kiro API key lines.
  - full JSON object or account array.
- Backend forwards each credential to internal `kiro.rs`
  `POST /api/admin/credentials`.
- Backend accepts snake_case and kiro.rs camelCase fields.
- Backend returns per-item import results and redacts nested secrets.
- Added local mirrors:
  - `D:\wflogin\_github_research\kiro.rs-latest`
  - `D:\wflogin\_github_research\kiro-gateway`
- Added `planning/KIRO_STAGE_B_IMPORT_ENDPOINT.md`.

Validation:

```powershell
go test ./internal/handler/admin -run Kiro
go test ./internal/handler/admin
go test ./internal/server

$env:npm_config_dangerously_allow_all_builds='true'
corepack pnpm exec vitest run src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts
corepack pnpm exec vue-tsc --noEmit
corepack pnpm exec vite build
```

Result:

```text
Kiro backend tests passed.
Admin handler tests passed.
Server route package passed.
Frontend parser tests passed: 12 tests across Kiro and Windsurf.
vue-tsc passed.
vite build passed with existing chunk/dynamic import warnings.
```

Deployment note:

- This code path requires a private internal Kiro adapter service and these
  Sub2API env vars:
  - `KIRO_ADAPTER_INTERNAL_BASE_URL=http://kiro-rs:8990`
  - `KIRO_ADAPTER_ADMIN_API_KEY=<kiro-rs adminApiKey>`
  - `KIRO_ADAPTER_TIMEOUT_SECONDS=30`
- Do not expose `kiro-rs` through Caddy or Docker host ports.

## 2026-05-19 provider adapter fusion round 2

Reference refresh:

- Official Sub2API upstream advanced from `11870cf8` to `8584b8f7`.
- Merged upstream updates into the private fork. The upstream changes cover
  OpenAI-compatible usage parsing, Codex tool-call ID test alignment, and admin
  settings dark-mode readability.
- `dwgx/WindsurfAPI` remains at `c028576`, tag `v2.0.96`.
- `hank9999/kiro.rs` remains at `f1bbe9f`; local tags were refreshed through
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remains at `a5292ca`, tag line `v2.3`.

Code changes:

- Normalized Windsurf import responses into per-item `items[]`, matching the
  Kiro import shape more closely.
- `POST /api/v1/admin/accounts/import/windsurf` now returns:
  - `succeeded`
  - `failed`
  - `items[].index`
  - `items[].kind`
  - `items[].success`
  - `items[].error`
  - redacted `items[].upstream`
- The admin Windsurf import modal now shows success/failure counts and a
  per-item preview instead of relying only on the raw upstream body.

Operational impact:

- Existing clients that only read `total`, `forwarded`, `duplicate_count`,
  `upstream_status`, or `upstream` remain compatible.
- Operators can now identify the exact failed row in a batch import without
  printing tokens, API keys, or passwords.

Deployment:

- Built and deployed server image `sub2api-provider-adapters:c4cefc76`.
- Previous server image was `sub2api-provider-adapters:f95c2073`.
- Server backup before switch:
  `/opt/sub2api-backups/sub2api-20260519-102757-pre-c4cefc76.tar.gz`
  (about 40 MB).
- `docker compose ps` showed:
  - `sub2api` healthy on `127.0.0.1:8080->8080`.
  - `windsurf-api` internal only on `3003/tcp`.
  - `kiro-rs` internal only on `8990/tcp`.
  - Postgres, Redis, and Caddy running.

Smoke results:

- Local Sub2API health: `GET http://127.0.0.1:8080/health` returned
  `{"status":"ok"}`.
- Public `https://api.vyywcw.cn/` returned HTTP 200.
- Public `https://www.vyywcw.cn/` returned HTTP 200.
- Internal Windsurf health from Sub2API container returned status ok and
  reported one active account.
- Internal Kiro models endpoint from Sub2API container returned a model list.
- Internal Kiro admin credentials endpoint returned `total: 0`, which is
  expected before a real Kiro credential is imported.
- Docker inspect confirmed adapter ports are not published:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`

Note:

- The deploy helper's first health curl ran too early while the container was
  still starting and returned a transient connection reset. A follow-up retry
  verification passed after the container became healthy.

## 2026-05-19 kiro.rs admin metadata patch and public smoke

Reference refresh:

- `hank9999/kiro.rs` remains at `f1bbe9f`, latest observed tag
  `v2026.3.1`.
- `Jwadow/kiro-gateway` remains at `a5292ca`, latest observed tag `v2.3`.
- Private Sub2API fork remains synced with `origin/main` at `1709e676` before
  this documentation update.

kiro.rs patch:

- Patched local `D:\wflogin\kiro.rs-master` so admin
  `POST /api/admin/credentials` accepts full exported metadata:
  - `accessToken` / `access_token`
  - `profileArn` / `profile_arn`
  - `expiresAt` / `expires_at`
- Added snake_case aliases for common admin import fields so direct API import
  and Sub2API bridge import use the same shape.
- Changed `AdminService::add_credential` to pass `access_token`,
  `profile_arn`, and `expires_at` into `KiroCredentials` instead of setting
  them to `None`.
- Built server image `kiro-rs-admin-metadata:20260519-1425`.
- Switched only the internal `kiro-rs` service to that image.

Verification:

- Targeted Rust test passed:
  `cargo test admin::types::tests::add_credential_request_accepts_full_export_metadata`.
- Full local `cargo test` still has 8 pre-existing Anthropic converter failures
  around old `claude-sonnet-4` / `claude-opus` model assertions. Those failures
  are unrelated to the admin import metadata patch.
- Server `kiro-rs` restarted successfully and loaded 1 credential.
- Internal Kiro admin credential status returned:
  - `total: 1`
  - `available: 1`
  - `with_profile_arn: 1`
  - `auth_methods: ["social"]`
- Internal Windsurf account status returned:
  - `total: 1`
  - `active: 1`
  - tier: `pro`
- Public Sub2API smoke through `https://api.vyywcw.cn/v1/messages` passed:
  - Windsurf `claude-sonnet-4.6`: HTTP 200, text `ok`
  - Kiro `qwen3-coder-next`: HTTP 200, text `ok`
  - Kiro `deepseek-3.2`: HTTP 200, text `ok`

Exposure check:

- Adapter ports remain internal only:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`
  - `sub2api-kiro-gateway`: `{"8000/tcp":null}`
- Caddy has no `windsurf-api`, `kiro-rs`, `kiro-gateway`, `3003`, `8990`, or
  `8000` routes.

## 2026-05-19 post-push verification

- Pushed commit `76633539` to
  `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`.
- Confirmed local fork is clean and aligned with `origin/main`.
- Server compose status:
  - `sub2api` healthy on `127.0.0.1:8080->8080`.
  - `windsurf-api`, `kiro-rs`, and `kiro-gateway` are running without public
    port mappings.
- Internal adapter status:
  - Windsurf: `total: 1`, `active: 1`, tier `pro`.
  - Kiro: `total: 1`, `available: 1`, `with_profile_arn: 1`,
    `auth_methods: ["social"]`.
- External public-domain smoke from local machine through
  `https://api.vyywcw.cn/v1/messages` returned HTTP 200 for:
  - `claude-sonnet-4.6`
  - `qwen3-coder-next`
  - `deepseek-3.2`

Operational note:

- Browser admin login should use the current remembered password.

## 2026-05-20 upstream main merge and server rollout

Reference refresh:

- `Wei-Shaw/sub2api` upstream was merged through `3d22dd34`.
- `dwgx/WindsurfAPI` remained at `c028576`, tag `v2.0.96`.
- `guanxiaol/WindsurfPoolAPI` remained at `a8d2f4c`, tag `v2.0.7`.
- `hank9999/kiro.rs` remained at `f1bbe9f`, tag `v2026.3.1`.
- `Jwadow/kiro-gateway` remained at `a5292ca`, tag `v2.3`.

Validation before deploy:

- `go test ./internal/handler/admin -run "Windsurf|Kiro|AvailableModels|SyncUpstream"` passed.
- `go test ./internal/server ./internal/pkg/apicompat ./internal/pkg/openai_compat` passed.
- `go test ./internal/handler/admin` passed.
- `go test ./internal/service -run "Upstream|OpenAI|Gateway|AccountCredentials|AdminService|Windsurf|Kiro|Pricing|Channel|Gemini"` passed.
- `go test ./...` passed after using a temporary `GOPROXY=https://goproxy.cn,direct`
  because local access to `proxy.golang.org` timed out over IPv6.
- `corepack pnpm exec vitest run src/components/admin/account/__tests__/kiroImport.spec.ts src/components/admin/account/__tests__/windsurfImport.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` passed.
- `corepack pnpm build` passed with only Vite chunk/dynamic-import warnings.

Server rollout:

- Built initial server image `sub2api-provider-adapters:66940db0`, then rebuilt
  final image `sub2api-provider-adapters:da09f930` after adding the Opus 4.7
  alias migration.
- Backed up compose and database before switching:
  - `/opt/sub2api-backups/docker-compose-20260520-094723-pre-66940db0.yml`
  - `/opt/sub2api-backups/sub2api-db-20260520-094723-pre-66940db0.dump`
- Updated `/opt/sub2api/docker-compose.yml` to use
  `sub2api-provider-adapters:da09f930`.
- Restarted only the `sub2api` service. Adapter services were not rebuilt.

Post-deploy verification:

- `sub2api` is healthy on `127.0.0.1:8080->8080`.
- Internal adapter ports remain un published:
  - `sub2api-windsurf-api`: `{"3003/tcp":null}`
  - `sub2api-kiro-rs`: `{"8990/tcp":null}`
  - `sub2api-kiro-gateway`: `{"8000/tcp":null}`
- Internal adapter status:
  - Windsurf: `total: 1`, `active: 1`, tier `pro`.
  - Kiro: `total: 1`, `available: 1`, `with_profile_arn: 1`,
    `auth_methods: ["social"]`.
- Public smoke through `https://api.vyywcw.cn/v1/messages` returned HTTP 200
  for:
  - `claude-sonnet-4.6`
  - `claude-opus-4.6`
  - `qwen3-coder-next`
  - `deepseek-3.2`

Opus 4.7 alias note:

- Windsurf already exposed effort-specific Opus 4.7 model keys such as
  `claude-opus-4-7-low` and `claude-opus-4-7-medium`.
- Added migration `140_windsurf_opus47_aliases.sql` so user-facing dotted
  aliases map to the Windsurf keys.
- Manually applied the same aliases on the current server before the migration
  image was rebuilt. The final image then applied
  `140_windsurf_opus47_aliases.sql` idempotently.
- Public smoke returned HTTP 200 for:
  - `claude-opus-4.7`
  - `claude-opus-4-7`
  - `claude-opus-4.7-medium`
- High/max Opus 4.7 effort keys can still hit upstream rate limits depending
  on current account quota and Windsurf upstream state.
- The server `.env` `ADMIN_PASSWORD` no longer matches the live admin password,
  so CLI admin import smoke through `POST /api/v1/admin/accounts/import/kiro`
  was skipped after a 401 login response. Do not store the current admin
  password in Git or docs.

## 2026-05-20 Kiro Opus clarification

User corrected that the target path is still the old IDE/refresh-token method,
not Kiro `ksk_...` API key import.

Current evidence:

- Local historical validation is still important: `D:\wflogin\kiro.rs-master`
  used `endpoint=ide`, a fixed `machineId`, and local proxy egress
  `http://127.0.0.1:7897`; on 2026-05-19 that local runtime returned HTTP 200
  for `claude-sonnet-4-6`.
- Server `kiro-rs` now has the fixed local `machineId` and `endpoint=ide` in
  `/opt/sub2api/kiro-rs/config/credentials.json`.
- Re-running the sanitized server probe with the fixed local `machineId` still
  returns only five official Kiro models:
  `deepseek-3.2`, `minimax-m2.5`, `minimax-m2.1`, `glm-5`,
  `qwen3-coder-next`.
- Direct server generate smoke through `q.<region>.amazonaws.com` returns HTTP
  200 for `qwen3-coder-next`, but HTTP 400 `INVALID_MODEL_ID` for
  `claude-opus-4.5`, `claude-opus-4.6`, `claude-opus-4.7`,
  `claude-sonnet-4.5`, `claude-sonnet-4.6`, and `claude-haiku-4.5`.
- Direct internal `kiro-rs /v1/messages` with `claude-opus-4-6` returns 502 at
  the proxy layer; the `kiro-rs` logs show the upstream reason is the same Kiro
  `INVALID_MODEL_ID` response.

Conclusion:

- Do not treat this as "Kiro/kiro.rs no longer supports Opus in general".
- The narrower, verified statement is: with the current imported Kiro Pro
  credential on the US server, the old IDE/refresh-token server path is not
  currently accepted for Claude-family models, even after restoring the local
  fixed `machineId`.
- The remaining material difference from the previously working local runtime is
  local proxy/runtime egress context and possibly the exact fresh IDE export
  source. Do not expose Kiro Claude/Opus through public Sub2API until a direct
  server smoke returns 200.

## 2026-05-20 GitHub workaround scan

Checked and tested the newest practical GitHub workaround ideas:

- `pi-kiro` `v0.1.3` / commit `438327370eb86a35ba7ac89aa1249e3c2a18ec85`
  uses `origin=KIRO_CLI` and AmazonQ-For-CLI style request headers.
- `open-kiro` Go module `v0.0.0-20260324032827-cf5db84025da` uses Kiro CLI
  style `ListAvailableModels` and POSTs to the Q service root.
- Kiro-account-manager `v1.6.6` remains the latest checked reference and keeps
  the relevant identity/machine metadata preservation ideas.

Live server probes were added to `tools/kiro_server_probe.py` for these styles:

- `--client-style open-kiro`
- `--client-style pi-cli`
- `--client-style kam-amazonq`

Results:

- `open-kiro` style: same five models; `qwen3-coder-next` succeeds; Claude,
  Opus, and Haiku still return `INVALID_MODEL_ID`.
- `pi-cli` style: same five models; `qwen3-coder-next` succeeds; Claude, Opus,
  and Haiku still return `INVALID_MODEL_ID`.
- Hidden/alias tests for `claude-opus-4.7`, `claude-opus-4.6-1m`,
  `claude-sonnet-4.6-1m`, `claude-opus-4-20250918`,
  `MODEL_PLACEHOLDER_M26`, and `auto` also return `INVALID_MODEL_ID`.
- Kiro-account-manager's AmazonQCLI `SendMessageStreaming` shape is not a
  direct drop-in replacement for the current Anthropic/Sub2API path; the simple
  generated payload returns `Improperly formed request`.

Conclusion:

- No GitHub project currently provides a header-only or model-name-only fix for
  this imported server credential.
- The next meaningful Kiro test is a fresh Builder ID or IdC OIDC credential
  export from a runtime that can currently use Claude/Opus.
- See `planning/KIRO_GITHUB_WORKAROUND_SCAN_2026-05-20.md`.

## 2026-05-21 reference protocol scan

Rechecked upstream reference projects for protocol-sensitive updates that could
affect official account import, token refresh, or model routing:

- `dwgx/WindsurfAPI` remains `c028576a56b9fa19f84810643610cae4af824238` /
  `v2.0.96`.
- `guanxiaol/WindsurfPoolAPI` remains
  `a8d2f4cf0c4c36d021debfe0428ec497660c55e6` / `v2.0.7`.
- `hank9999/kiro.rs` remains
  `f1bbe9f1d14b962211592c661792d48a9855e32e` / `v2026.3.1`.
- `Jwadow/kiro-gateway` remains
  `a5292ca04c7c6231e0b47673ac3f981f5a706e1e` / `v2.3`.
- `tickernelz/opencode-kiro-auth` remains
  `d0d9b18c8031abe29fe27c91d505a4f2ff24e9a0` / `v1.10.1`.
- `hongyilyu/pi-kiro` remains
  `438327370eb86a35ba7ac89aa1249e3c2a18ec85` / `v0.1.3`.
- `chaogei/Kiro-account-manager` remains
  `7ad57fd26e67b3ea91b780b2ca983c78737ed88a` / `v1.6.6`.
- `jlcodes99/cockpit-tools` advanced to
  `2b148437ef19812ffbea50d62ccc5f52a47caaf2`, latest tag `v0.24.3`.

Impact:

- No deployed Windsurf/Kiro adapter update is required from this scan.
- `cockpit-tools` changes are Antigravity/Codex local routing and path
  detection, not Windsurf/Kiro provider protocol.
- `kiro.rs` non-default refactor branches were fetched and inspected; they
  improve IDE endpoint layering and pool behavior but do not add the Kiro Web
  Portal `CreateSpace` / `StreamSendMessage` path.
- Keep the deployed Kiro Web adapter as the Claude/Opus source of truth.

## 2026-05-21 Sub2API upstream scan

Fetched official `Wei-Shaw/sub2api` without merging.

- Previous private-fork merge base:
  `3d22dd34d3de9076804858f979f60fccdf9f2de1`.
- Latest official upstream `main`:
  `35901a174b281367ca4c9655dfc14e2c2347c7ae`.
- Latest observed official tag: `v0.1.129`.
- Official `backend/cmd/server/VERSION`: `0.1.129`.
- Private fork version before merge: `0.1.127`.
- Private branch is `40` commits ahead and `64` commits behind official
  upstream.

No merge or server deployment was performed.

Notable upstream areas:

- OpenAI Responses / Chat Completions bridge fixes.
- OpenAI image `n` pass-through and moderation error surfacing.
- Codex OAuth user-agent rewrite and reused refresh-token handling.
- Scheduler cache cleanup on account deletion and errored-account unscheduling.
- Bedrock Claude Code compatibility transformations.
- API Key daily usage detail.
- Redeem-code batch update.
- Email template editor and notification-email services.
- Content-audit keyword blocking.
- OIDC verified-email fast path.

Potential merge touch points with private adapter work:

- `backend/internal/server/routes/admin.go`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/types/index.ts`
- Migration numbering collision: private `140_windsurf_opus47_aliases.sql`
  versus official `140_extend_user_provider_default_grants_check.sql` and
  `141_subscription_expiry_notify_enabled.sql`.
  Resolved during merge by renaming the private migration to
  `142_windsurf_opus47_aliases.sql`.

## 2026-05-27 Windsurf cache and Sub2API route update

Updated the live internal Windsurf adapter from `dwgx/WindsurfAPI v2.0.96`
to `v2.0.97` (`41a36b9176633a9e67eb7ca87d725b5eb98564b8`) because upstream
added Cascade reuse optimization that directly addresses the observed
`reuse MISS` / 0% cache hit behavior.

Server changes:

- Backed up compose to `/opt/sub2api/docker-compose.yml.bak.20260527-154627`.
- Added `CASCADE_REUSE_BY_CALLER=1`, `CASCADE_POOL_MAX=5`, and
  `CASCADE_REUSE_HASH_SYSTEM=0` to the `windsurf-api` service.
- Recreated `sub2api-windsurf-api`; health confirmed `version=2.0.97`,
  `conversationPool.maxSize=5`, and `reuseByCaller=true`.
- Re-enabled the Sub2API upstream account `windsurf-internal-anthropic`.
- Added forced Windsurf public aliases under `provider-mixed`:
  `ws-claude-sonnet-4.6`, `ws-claude-sonnet-4.6-thinking`,
  `ws-claude-opus-4.6`, `ws-claude-opus-4.6-thinking`,
  `ws-gemini-2.5-flash`, `ws-gpt-5.1`, and `ws-gpt-5.2`.
- Restarted Sub2API so the scheduler snapshot picked up the DB route changes.

Verification:

- Direct internal WindsurfAPI `/v1/messages` with `claude-sonnet-4.6`
  returned HTTP 200.
- Public Sub2API `/v1/messages` with `ws-claude-sonnet-4.6` returned HTTP 200.
- A second public turn with the same conversation returned
  `cache_read_input_tokens=1935`, and Windsurf health showed
  `conversationPool.hits=1`.
- Public Sub2API `/v1/messages` with `ws-claude-opus-4.6` returned HTTP 200.

## 2026-05-28 Sub2API mail relay activated with Resend

Sub2API production email delivery is now routed through the internal
SMTP-to-HTTPS relay:

- Sub2API sends SMTP to `mail-relay:1025` inside the Docker network.
- `mail-relay` sends through Resend over HTTPS.
- The Resend API key is stored only in `/opt/sub2api/.env`.
- Saved SMTP config uses empty username/password and TLS disabled because auth
  is handled by the relay's HTTPS provider key.

Verification:

- `https://api.vyywcw.cn/health` returned HTTP 200.
- `mail-relay` health returned `{"ok": true, "provider": "resend", ...}`.
- Sub2API saved SMTP settings are `mail-relay:1025`,
  `no-reply@vyywcw.cn`, from name `vyywcw`.
- Real saved-config test emails returned HTTP 200 and relay logs showed
  `sent mail provider=resend ... subject='[yuapi] Test Email'`.

Operational notes:

- Do not expose `mail-relay` ports publicly.
- Rotate the Resend key from Resend dashboard, then update
  `/opt/sub2api/.env` and restart only `mail-relay`.

## 2026-05-28 Windsurf model-family groups and runtime hotfix

Created five public Sub2API Windsurf groups with one API key per model family:

- `windsurf-opus4.6`
- `windsurf-opus4.7`
- `windsurf-gpt5.5`
- `windsurf-gpt5.4`
- `windsurf-grok`

Each key exposes only its own aliases through `/v1/models`. Cross-family calls
are blocked by channel model restrictions, not only by soft `model_routing`.

During smoke testing, `WindsurfAPI v2.0.97` generated upstream text but failed
the non-stream success path with `acct is not defined`. The live server now uses
a local patched image:

- `sub2api-windsurf-api-acctfix:20260528`
- server build context: `/opt/sub2api/windsurf-api-acctfix`
- repository patch copy:
  `provider-patches/windsurf-api/acct-scope-hotfix-20260528/`

Public smoke after the fix:

- `opus4.6`: HTTP 200
- `gpt5.5-low`: HTTP 200
- `gpt5.4-low`: HTTP 200
- `grok`: HTTP 200
- `opus4.7-low`: HTTP 429 from upstream rate limit during this run

Also rechecked upstream versions:

- `dwgx/WindsurfAPI` `master` remains
  `41a36b9176633a9e67eb7ca87d725b5eb98564b8` / `v2.0.97`.
- `Wei-Shaw/sub2api` `main` is
  `89d96f4b25c6e5ead427f67b54a8aae5fdf993e4` / `v0.1.132`.
- This private fork currently contains `upstream/main`; `rev-list
  HEAD...upstream/main` is `68 0`, so the left-top update prompt in the admin
  UI is probably an online-update/image-label detection issue rather than a
  missing official merge.

## 2026-06-01 CCS model config and live group verification

Updated the user key frontend config so CC Switch/Codex imports no longer pin
the old `gpt-5.4` model. OpenAI imports now prefer the key group's model list
and fall back to `gpt-5.5`, matching the Codex config shown in "Use Key".

Deployment:

- Built and deployed image
  `sub2api-provider-adapters:ccs-codex-model-20260601`.
- Updated `/opt/sub2api/docker-compose.yml` after creating an automatic backup.
- Container health returned OK.

Verification:

- Frontend targeted tests passed:
  `npm run test:run -- src/utils/__tests__/ccswitchImport.spec.ts`
  and
  `npm run test:run -- src/components/keys/__tests__/UseKeyModal.spec.ts`.
- Frontend production build passed with existing Vite chunk warnings.
- Public `/v1/models` returned HTTP 200 for `provider-mixed`,
  `windsurf-opus4.6`, `windsurf-opus4.7`, `windsurf-gpt5.5`,
  `windsurf-gpt5.4`, `windsurf-grok`, `GPT5.5`, `kiro-opus4.6`, and
  `kiro-opus4.7`.
- Public `GPT5.5` smoke with `gpt-5.5` returned HTTP 200 and usable text.
- Temporary Kiro smoke keys were created for testing and then removed.

Current caveats:

- Kiro Web adapter direct smoke returns usable text for `auto`,
  `claude-sonnet-4`, `qwen3-coder-next`, `glm-5`, `deepseek-3.2`, and
  `minimax-m2.1`.
- The deployed Kiro account is `KIRO FREE`; direct adapter smoke for
  `claude-sonnet-4.6`, `claude-opus-4.6`, and `claude-opus-4.7` returned
  upstream text saying `Invalid model ID`, so Kiro Opus/Sonnet 4.6 groups
  should not be treated as callable until a Kiro account/adapter path with
  those model entitlements is available.
- Windsurf family groups still expose their configured model lists, but their
  bound Windsurf upstream account is currently not schedulable.

Upstream check:

- Fetched official `Wei-Shaw/sub2api` `main`.
- New upstream head is `aa69e394`.
- This private branch is behind official upstream by 5 commits focused on the
  Codex Responses/Chat Completions bridge and Antigravity scheduling/rate-limit
  fixes.
- No upstream merge was performed because the diff intersects private
  Kiro/Windsurf adapter files and should be handled in a separate merge window.

## 2026-06-03 upstream protocol merge and OpenAI OAuth recovery guard

Merged official `Wei-Shaw/sub2api` `upstream/main` at `aa69e394` into the
private branch. The upstream delta includes the Responses/Chat Completions
bridge redesign, Anthropic Messages API compatibility work, Antigravity fixes,
WS Codex image bridge updates, and apicompat streaming tests.

Deployment:

- Built and deployed
  `sub2api-provider-adapters:upstream-merge-20260603b`.
- Server compose now runs that image for `sub2api`.
- Local and public health checks returned `{"status":"ok"}`.
- Cleared GPT5.5 group OpenAI OAuth Redis token cache entries and scheduler
  cache entries after deployment, then restarted `sub2api`.

Local fixes added after the upstream merge:

- OpenAI OAuth `401` with a non-empty `refresh_token` no longer permanently
  marks the account as `error` for `token_invalidated`, `token_revoked`, or
  `{"detail":"Unauthorized"}`.
- The handler now invalidates the token cache, marks only token-related
  credential fields as needing refresh, and temporarily removes the account
  from scheduling.
- Added a repository partial JSONB credential update helper so this path does
  not overwrite the whole credentials document.

Verification:

- `go test ./internal/service -run TestOpenAIGatewayService_OpenAIPassthrough_AccountPoolErrorsTriggerFailover`
- `go test ./internal/repository ./internal/pkg/apicompat`
- `go test ./internal/service`
- `go test ./internal/handler/admin ./internal/handler`

Known limitation:

- `go test -tags unit ./internal/service -run TestRateLimitService_HandleUpstreamError_OpenAITokenInvalidatedWithRefreshTokenRecovers`
  is blocked by an existing unrelated compile issue in
  `openai_account_runtime_block_fastpath_test.go`.
- Live GPT5.5 smoke still returns upstream auth failure. Logs show many imported
  CPA OpenAI OAuth accounts fail refresh with OpenAI `refresh_token_reused`,
  which means those refresh tokens have already been consumed/rotated elsewhere
  or the CPA import source contains stale refresh tokens. Re-importing current
  CPA credentials or disabling the stale accounts is required before those
  accounts can become callable. After smoke/background refresh, the GPT5.5
  OpenAI OAuth pool was down to 48 active accounts, 203 error accounts, and 41
  DB-schedulable accounts.

## 2026-06-04 NewAPI sidecar deployment

Deployed an independent NewAPI instance on the same server as Sub2API.

Runtime layout:

- NewAPI path: `/opt/newapi`
- Public URL: `https://newapi.vyywcw.cn/`
- Image: `calciumion/new-api:latest`
- Running version from logs: `New API v1.0.0-rc.10`
- Local host port: `127.0.0.1:3001 -> newapi:3000`
- Data is isolated from Sub2API:
  - `newapi-mysql` with `/opt/newapi/mysql_data`
  - `newapi-redis` with `/opt/newapi/redis_data`
  - app data under `/opt/newapi/data`

Caddy now serves `api.vyywcw.cn`, `www.vyywcw.cn`, and
`newapi.vyywcw.cn` from one site block. A `@newapi` host matcher routes
`newapi.vyywcw.cn` to `newapi:3000`; existing Sub2API routing remains the
fallback for `api.vyywcw.cn`.

Verification:

- `newapi`, `newapi-mysql`, and `newapi-redis` containers are healthy.
- `http://127.0.0.1:3001/` returned HTTP 200.
- Caddy HTTPS routing for `newapi.vyywcw.cn` returned HTTP 200.
- `https://newapi.vyywcw.cn/` returned HTTP 200 from the local machine.
- `https://api.vyywcw.cn/health` still returned `{"status":"ok"}`.

Operational notes:

- NewAPI is intentionally uninitialized; create the root admin user through
  the web UI so the admin password is not handled by automation.
- NewAPI is documented in this same repository on `main` as a sidecar
  deployment record, not as a branch replacing Sub2API.
- Do not import the same CPA/OpenAI OAuth accounts into both NewAPI and Sub2API
  with auto-refresh enabled. Refresh tokens rotate, and whichever system uses a
  refresh token first owns the next token; the other system will hit
  `refresh_token_reused`.
- Recommended split: use NewAPI for CPA/OpenAI OAuth pools and standard
  OpenAI-compatible aggregation; keep Sub2API for private Kiro/Windsurf and
  custom protocol adapter work.

## 2026-06-04 NewAPI CPA Codex channel imports

Imported CPA/Codex OAuth credentials into the NewAPI sidecar. Secrets were used
only during import and were not written to Git.

Imported channel tags:

- `cpa-codex-20260604`: 6 channels imported from manually supplied refresh
  tokens.
- `cpa-codex-c72b8eef485f4865`: 10 channels imported from
  `C:\Users\Administrator\Downloads\cpa_c72b8eef485f4865`.

Verification:

- Both tags are fully enabled and refreshed in NewAPI:
  - `cpa-codex-20260604`: 6 total, 6 enabled, 6 refreshed.
  - `cpa-codex-c72b8eef485f4865`: 10 total, 10 enabled, 10 refreshed.
- Enabled NewAPI self-use mode (`SelfUseModeEnabled=true`) so self-hosted
  channel testing and calls are not blocked by missing model pricing.
- `/v1/chat/completions` is not supported by the Codex channel path.
- `/v1/responses` with `stream=true` and list-form `input` returned HTTP 200
  SSE responses for sampled channels from both tags using `gpt-5.4`.

Operational notes:

- Codex/ChatGPT accounts rejected `gpt-5` and `gpt-5-codex` during channel
  tests, while `gpt-5.4` reached the upstream successfully when using the
  Responses API format.
- NewAPI requires `input` to be a list for this Codex Responses path, for
  example a user message with `input_text` content.

## 2026-06-04 NewAPI GPT-5.5 CPA group split

Configured two dedicated NewAPI token/channel groups for the imported
CPA/Codex pools:

- `cpa-gpt55-a`: channels tagged `cpa-codex-20260604` (6 channels).
- `cpa-gpt55-b`: channels tagged `cpa-codex-c72b8eef485f4865` (10 channels).

Changes:

- Added both groups to NewAPI group ratio and user usable group settings.
- Created one NewAPI API token per group with unlimited quota, model limit
  `gpt-5.5`, and cross-group retry disabled.
- Appended `gpt-5.5` to all 16 CPA/Codex channel model lists so the dispatcher
  can route the model inside the split groups.

Verification:

- NewAPI enabled model list includes `gpt-5.5`.
- `https://newapi.vyywcw.cn/v1/responses` returned HTTP 200 SSE
  `response.created` for both `cpa-gpt55-a` and `cpa-gpt55-b` keys using
  `model: gpt-5.5`.

Usage note:

- Use the Responses API shape with `stream=true` and list-form `input`.
- The usable model name is `gpt-5.5`; `gpt-5.5-codex` is rejected upstream for
  these ChatGPT/Codex accounts.

## 2026-06-18 Sub2API GPT account-pool cleanup

After the user cleaned the Sub2API account list manually, production was
audited again from the database and through live smoke probes. No secrets or
full account payloads were recorded.

Pre-cleanup state:

- Active account rows: 684.
- `gpt-team`, `gpt-plus`, and `gpt-pro` each had the same 684 linked accounts.
- Only 3 accounts were initially schedulable.

Cleanup actions:

- Soft-deleted 77 OpenAI OAuth accounts already marked with revoked,
  invalidated, authentication-failed, or 401 errors.
- Sampled access-token-only OpenAI OAuth accounts that lacked refresh tokens;
  samples returned 401 expired, and their stored `expires_at` values were
  already in the past.
- Soft-deleted 604 expired access-token-only OpenAI OAuth accounts with no
  refresh token.
- Soft-deleted 1 OpenAI OAuth account that failed live probing with workspace
  deactivated.
- Removed 2 internal Kiro adapter accounts from the GPT pool by soft-deleting
  them, because they were the only remaining active rows and did not provide
  usable GPT supply.
- Added missing Sub2API allowed-group rows for user `1` so the dedicated
  NewAPI bridge keys are permitted to bind `gpt-team`, `gpt-plus`, and
  `gpt-pro`.
- Published API-key auth cache invalidation messages for the bridge keys.

Final state:

- Active Sub2API account rows: 0.
- `gpt-team`, `gpt-plus`, and `gpt-pro`: 0 linked accounts and 0 schedulable
  accounts.
- `https://api.vyywcw.cn/health` returned `{"status":"ok"}`.
- `https://newapi.vyywcw.cn/api/status` returned success.

Operational implication:

- NewAPI and Sub2API are healthy, and the bridge group permission issue was
  fixed, but GPT traffic currently has no usable Sub2API supply behind it.
- Before opening GPT traffic again, import fresh upstream API-key accounts or
  CPA/OAuth accounts with refresh tokens into the intended Sub2API tier:
  `gpt-team`, `gpt-plus`, or `gpt-pro`.
