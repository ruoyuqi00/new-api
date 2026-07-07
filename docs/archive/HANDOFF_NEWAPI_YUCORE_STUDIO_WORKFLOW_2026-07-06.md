# Archived: YuCore Studio / Canvas Handoff - 2026-07-06

Archive notice: this is historical YuCore Studio/Canvas context only. It is
not the current production baseline. Start from
`BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md` for current remote,
production, and project-role decisions.

# YuCore Studio / Canvas Handoff - 2026-07-06

This file is the continuation handoff for the work in:

```text
D:\wflogin\new-api
```

The thread context became too large, so this document is meant to let a new
window resume without losing the real goal, the current state, or the remaining
work.

## Core Goal

Keep the full objective intact:

- Turn YuCore user-side Studio and infinite canvas from a static demo into a
  real user workflow comparable in spirit to `https://ai.soulecho.cc/`.
- Keep the experience embedded in the authenticated user surface:
  - `/playground/studio`
  - `/playground/canvas`
- Keep super-admin controls in the admin/settings surface rather than turning
  Studio into a separate admin page.
- Make image task completion flow back into:
  - generated result cards
  - user asset lists
  - canvas nodes
  - Agent runs
- Final proof still requires:
  - local visual QA at `http://127.0.0.1:3015/playground/studio`
  - ordinary-user access verification
  - frontend type/build passing
  - backend tests/compile passing
  - real upstream image provider verification

Do not shrink the goal to "the UI exists" or "mock tasks complete".

## Do Not Misreport Completion

Do **not** mark this work complete while UAG is still mock-backed.

As of this handoff, the following still block true completion:

```text
KLEIN_PROVIDER_GPT=mock
KLEIN_PROVIDER_GROK=mock
KLEIN_PROVIDER_FLOW=mock
KLEIN_GPT_BASE_URL=
KLEIN_GROK_BASE_URL=
```

That means the current workflow is a strongly wired skeleton with ordinary-user
UI proof, backend backflow, admin control, and asset proxying, but not yet
proven against a real upstream image provider.

## Reference-Site Findings

Existing capture artifacts for `ai.soulecho.cc`:

```text
D:\wflogin\new-api\web\default\output\playwright\soulecho-live-20260705
```

Important file:

```text
D:\wflogin\new-api\web\default\output\playwright\soulecho-live-20260705\inspection.json
```

Useful findings from the capture:

- Public target: `https://ai.soulecho.cc/`
- Frontend appears to be React + TanStack style bundles:
  - `vendor-tanstack`
  - `vendor-ui-primitives`
  - `lib-react`
- Landing page includes a WebGL hero canvas:
  - `canvas#canvas-webgl`
  - class `p-canvas-webgl`
  - `sketch-threejs` assets
- Public positioning shows:
  - OpenAI / Codex access
  - model/API hub behavior
  - `gpt-image-2`
  - `/v1/responses`
  - request/response examples
  - latency / token / cost framing

This capture is useful as product reference, but it does not provide a
drop-in open-source Studio implementation to transplant.

## image-site-v2 Decision

Checked local reference directories:

```text
D:\wflogin\image-site-v2
D:\wflogin\sub2api-private\apps\image-site-v2
```

Important conclusion:

- `D:\wflogin\image-site-v2` explicitly says it is a deprecated standalone
  spike.
- The maintained line exists under
  `D:\wflogin\sub2api-private\apps\image-site-v2`.
- That maintained app is a standalone Next/React image site, useful as API/task
  reference, but not the right thing to directly migrate into `new-api`.

Decision:

- Do **not** migrate the deprecated standalone copy into this repo.
- Do **not** replace YuCore Studio with a separate image-site product.
- Continue completing the current embedded YuCore Studio / canvas workflow
  inside `new-api`.

## Current Real State

### 1. User-side embedding is in place

User routes:

```text
web/default/src/routes/_authenticated/playground/studio.tsx
web/default/src/routes/_authenticated/playground/canvas.tsx
```

These render the YuCore Studio workspace in the authenticated user area. This
is the correct direction and should not be replaced by a separate admin-only
screen.

### 2. Super-admin control plane is in place

Settings entry:

```text
web/default/src/features/system-settings/models/yucore-media-settings-card.tsx
web/default/src/features/system-settings/models/section-registry.tsx
web/default/src/features/system-settings/models/index.tsx
web/default/src/features/system-settings/types.ts
```

Admin-controllable settings now include:

- adapter:
  - `mock`
  - `openai-compatible`
  - `uag-proxy`
- gateway base URL
- gateway API key write-only update path
- timeout seconds
- require-real-assets flag
- UAG allowed providers
- UAG allowed user-visible models
- user-facing-to-UAG model mapping
- upstream verified marker

This is important because the user specifically wanted Studio embedded on the
user side, but controlled by the super-admin side.

### 3. Backend config binding is in place

Relevant backend files:

```text
model/option.go
model/yucore_media.go
model/yucore_media_test.go
```

Important option keys:

```text
yucore_media.adapter
yucore_media.base_url
yucore_media.api_key
yucore_media.timeout_seconds
yucore_media.require_real_assets
yucore_media.uag_model_map
yucore_media.uag_allowed_providers
yucore_media.uag_allowed_models
yucore_media.upstream_verified
```

Behavior:

- YuCore media config reads admin options first.
- Environment variables remain as deployment defaults / fallback.
- Ordinary-user `/api/yucore/media/models` reflects admin control.

### 4. Backend task backflow is in place

Relevant files:

```text
model/yucore_canvas.go
model/yucore_media.go
controller/yucore_canvas.go
controller/yucore_media.go
```

Completed terminal media tasks now flow back into:

- `YucoreCanvasAgentRun`
- canvas prompt/task/result node data
- asset/result metadata
- run summary and run actions

This is the key bridge from "task exists" to "user canvas / Agent view reflects
the task result".

## Frontend Fixes Already Landed

The following user-visible fixes are already part of the current worktree /
runtime direction:

### Studio workbench exists and is usable

- ordinary users can open the image workbench
- prompt and negative prompt textareas are usable
- task submission from the UI works
- result cards and task history update

### Generated result preview was improved

File:

```text
web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx
```

Change:

- generated result / asset previews and canvas media-node previews were changed
  from `object-cover` to `object-contain`

Reason:

- some successful images had large bright sky areas
- crop-fill previews could make them look blank
- full containment makes result inspection much more trustworthy in the Studio

### Infinite canvas route loads persisted user state

- ordinary users can open `/playground/canvas`
- the grid/canvas surface loads
- the Agent assistant panel loads
- persisted canvas state is visible

## Runtime / QA Environment

Current QA backend image:

```text
newapi:yucore-qa-admin-control-20260706
```

Current QA backend container:

```text
yucore-qa-backend
```

SQLite backup created before admin-control rebuild:

```text
output/yucore-qa-before-admin-control-20260706-153256.sqlite
```

Current ordinary-user API state after QA backend replacement:

```json
{
  "login_success": true,
  "adapter": "uag-proxy",
  "configured": true,
  "require_real_assets": true,
  "upstream_status": "unverified",
  "model_ids": "img-v3,gpt-image-2",
  "model_count": 2
}
```

Admin control was explicitly verified by temporarily changing:

```text
yucore_media.uag_allowed_models=img-v3
```

and confirming ordinary-user `/api/yucore/media/models` shrank to:

```json
{
  "model_ids": "img-v3",
  "model_count": 1
}
```

Then QA state was restored to:

```text
yucore_media.uag_allowed_models=img-v3,gpt-image-2
```

## Ordinary-user Visual QA Evidence

### Studio entry + workbench

Artifacts:

```text
output/playwright/yucore-studio-user-qa-20260706/result.json
output/playwright/yucore-studio-user-qa-20260706/studio-user-visual.png
output/playwright/yucore-studio-user-qa-20260706/studio-image-workbench.png
output/playwright-runner/yucore-studio-user-qa-20260706.cjs
```

What this proves:

- user stays signed in
- user-side Studio is visible
- workbench is visible
- `img-v3` and `gpt-image-2` are the user-visible models
- UAG gateway state is surfaced
- image-generation controls are present

### Ordinary-user UI task submission

Artifacts:

```text
output/playwright/yucore-studio-ui-generate-qa-20260706/result.json
output/playwright/yucore-studio-ui-generate-qa-20260706/ui-generate-result.png
output/playwright-runner/yucore-studio-ui-generate-qa-20260706.cjs
```

Important QA task:

```text
task_id = yu_yrgtODv0zmFogLsvD4
model   = img-v3
status  = completed
progress = 100
asset_count = 1
```

This proves:

- a normal user can submit an image task from the Studio UI
- the task completes
- result state appears in the workbench
- the asset is attached to the completed task

### Asset proxy + image inspection

Artifacts:

```text
output/playwright/yucore-studio-ui-generate-qa-20260706/yu_yrgtODv0zmFogLsvD4-asset0.jpg
```

Verification:

- backend asset endpoint served a real image file
- the file is not a broken proxy response
- the image is visually valid

### Result-preview containment verification

Artifacts:

```text
output/playwright/yucore-studio-ui-generate-qa-20260706/ui-generate-selected-result.png
output/playwright/yucore-studio-ui-generate-qa-20260706/selected-result.json
output/playwright-runner/yucore-studio-select-ui-result-qa-20260706.cjs
```

What changed:

- the same high-key image that used to look almost blank in a cropped card now
  remains inspectable in the result panel

### Infinite canvas ordinary-user QA

Artifacts:

```text
output/playwright/yucore-canvas-user-qa-20260706/result.json
output/playwright/yucore-canvas-user-qa-20260706/canvas-user-visual.png
output/playwright-runner/yucore-canvas-user-qa-20260706.cjs
```

Canvas API spot check:

```json
{
  "success": true,
  "total": 2,
  "item_count": 2,
  "first_title": "Backend backflow QA 1783319280",
  "first_revision": 3
}
```

What this proves:

- `/playground/canvas` is usable as an ordinary user
- it is not an empty placeholder
- it loads persisted canvas state and Agent panel state

## Checks That Passed

Frontend:

```powershell
D:\wflogin\new-api\web> .\node_modules\.bin\tsgo -b --pretty false
D:\wflogin\new-api\web\default> .\node_modules\.bin\rsbuild.exe build
```

Backend:

```powershell
docker run --rm `
  -v 'D:\wflogin\new-api:/src' `
  -v newapi-go-cache:/go/pkg/mod `
  -v newapi-go-build-cache:/root/.cache/go-build `
  -w /src golang:1.25 `
  go test -count=1 ./model ./controller ./router
```

Diff hygiene:

```powershell
D:\wflogin\new-api> git diff --check
```

These checks passed after the current Studio / canvas / admin-control work.

## What Is Still Incomplete

The biggest remaining gap is still external reality, not local UI plumbing:

1. UAG still uses mock providers internally.
2. Therefore current successful image tasks are not proof of real upstream
   production wiring.
3. `upstream_verified` should remain false until a real provider path is proven
   end to end by an ordinary user.

## Next Best Work

The next window should prioritize the following, in this order:

1. Replace mock UAG provider config with a real image provider/account.
2. Keep `yucore_media.require_real_assets=true`.
3. Re-run ordinary-user Studio generation through the real provider.
4. Verify all of the following against current state:
   - task completes through real upstream
   - generated image is proxied and visibly rendered in Studio
   - result can be sent to canvas and persisted
   - Agent run state reflects the completed result task
   - admin allowlist still governs user-visible models
5. Re-run:
   - `tsgo -b --pretty false`
   - `rsbuild build`
   - `go test -count=1 ./model ./controller ./router`
   - `git diff --check`

## Useful Commands

Ordinary-user model and health check:

```powershell
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$loginBody = @{username='qa3304660569';password='yuapiqa26'} | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:3000/api/user/login' -ContentType 'application/json' -Body $loginBody -WebSession $session | Out-Null
$headers=@{'New-Api-User'='4'}
Invoke-RestMethod -Method Get -Uri 'http://127.0.0.1:3000/api/yucore/media/models' -WebSession $session -Headers $headers | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Get -Uri 'http://127.0.0.1:3000/api/yucore/media/health' -WebSession $session -Headers $headers | ConvertTo-Json -Depth 8
```

Check UAG provider reality:

```powershell
docker inspect uag-api --format '{{range .Config.Env}}{{println .}}{{end}}' | Select-String -Pattern 'KLEIN_PROVIDER|KLEIN_GPT_BASE|KLEIN_GROK_BASE|KLEIN_FLOW'
```

Rebuild QA backend image:

```powershell
docker build -t newapi:yucore-qa-admin-control-20260706 .
```

Restart QA backend with current QA SQLite:

```powershell
$loginBody = @{account='uagqa20260706@example.com';password='YuCoreUagQa2026'} | ConvertTo-Json -Compress
$login = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:17080/api/v1/auth/login' -ContentType 'application/json' -Body $loginBody
$token = $login.data.token.access_token

docker stop yucore-qa-backend
docker rm yucore-qa-backend
docker run -d --name yucore-qa-backend -p 3000:3000 `
  -v 'D:\wflogin\new-api\output\yucore-qa-standard-current-20260706.sqlite:/data/yucore-qa.sqlite' `
  -e SQL_DSN=local `
  -e SQLITE_PATH=/data/yucore-qa.sqlite `
  -e DEBUG=true `
  -e YUCORE_MEDIA_ADAPTER=uag-proxy `
  -e YUCORE_MEDIA_BASE_URL=http://host.docker.internal:17080 `
  -e YUCORE_MEDIA_API_KEY=$token `
  -e YUCORE_MEDIA_REQUIRE_REAL_ASSETS=true `
  -e YUCORE_MEDIA_FORWARD_BROWSER_AUTHORIZATION=false `
  -e YUCORE_MEDIA_UAG_MODEL_MAP=gpt-image-2=img-v3 `
  -e YUCORE_MEDIA_UAG_ALLOWED_PROVIDERS=gpt `
  -e YUCORE_MEDIA_UAG_ALLOWED_MODELS=img-v3,gpt-image-2 `
  newapi:yucore-qa-admin-control-20260706
```

## Archived Resume Prompt For A New Window

The prompt below belongs to the older YuCore Studio/Canvas window. For current
production feature work, create a clean worktree from `ruoyu/main` as described
in `BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`.

Use this directly in the next window:

```text
Continue the YuCore Studio / canvas workflow handoff in D:\wflogin\new-api.

Read and follow:
D:\wflogin\new-api\HANDOFF_NEWAPI_YUCORE_STUDIO_WORKFLOW_2026-07-06.md

Important constraints:
- Keep the real goal intact: embedded ordinary-user Studio + canvas + admin control + backend backflow + real upstream verification.
- Do not mark complete while UAG providers are still mock-backed.
- Prefer continuing the current embedded YuCore implementation instead of migrating the deprecated D:\wflogin\image-site-v2 spike.

Current verified state:
- ordinary-user /playground/studio visual QA exists
- ordinary-user UI image-task submission exists
- asset proxy works
- result preview now uses object-contain for inspectable images
- ordinary-user /playground/canvas visual QA exists
- super-admin model allowlist controls ordinary-user visible models
- frontend checks and backend tests pass

Top next task:
- replace UAG mock image provider with a real upstream provider/account and rerun the ordinary-user Studio workflow end to end
```
