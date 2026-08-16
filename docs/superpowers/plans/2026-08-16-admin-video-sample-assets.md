# Admin Video Sample Assets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import the ten retained probe videos byte-for-byte into an owner-and-role-protected “视频模型示例” Studio collection, with idempotent admin APIs, safe rollback, checksum evidence, and a no-stop production release.

**Architecture:** Reuse `YucoreMediaTask` and its owner-scoped gallery instead of adding a table. Persist a hidden managed-file name inside the existing asset JSON, identify samples with a deterministic owner-plus-checksum task ID and fixed metadata, and enforce sample access in list/detail/content/delete paths on the backend. A sequential operator script computes SHA-256, calls audited admin import/rollback endpoints without logging auth, and writes a mode-0600 result manifest outside Git.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite/MySQL/PostgreSQL-compatible queries, testify, React 19, TypeScript, Bun tests, i18next, Node ESM, Docker, Caddy.

---

## File Map

- Create `model/yucore_media_sample.go`: constants, checksum/task/file derivation, sample detection, and direct zero-cost task persistence.
- Create `model/yucore_media_sample_test.go`: deterministic identity, hidden managed field, metadata, and cross-database-safe query behavior.
- Modify `model/yucore_media.go`: hidden `ManagedFileName`, persisted JSON mapping, and include/exclude-sample list/count parameters.
- Create `controller/yucore_media_sample.go`: admin import, idempotent conflict handling, rollback, and access guard.
- Create `controller/yucore_media_sample_test.go`: multipart validation, idempotency, cleanup, role/owner access, Range, and rollback protection.
- Modify `controller/yucore_media.go`: sample-aware list/detail/update/delete/gallery/content behavior and verified-upload callback support.
- Modify `router/api-router.go`: audited admin sample import/rollback routes.
- Modify `middleware/audit.go`: stable audit action names for sample import and rollback.
- Create `web/default/src/features/yucore-brand/lib/media-gallery.ts`: pure sample/personal grouping.
- Create `web/default/src/features/yucore-brand/lib/media-gallery.test.ts`: grouping and anti-spoof contract.
- Modify `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`: separate private sample collection while retaining drag/download actions.
- Temporarily modify `web/default/scripts/add-missing-keys.mjs`: apply collection labels to all six locales, then restore before commit.
- Modify through script `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: new collection copy.
- Create `scripts/production/import-yucore-video-samples.mjs`: fixed ten-file sequential import and manifest-driven rollback.
- Create `scripts/production/import-yucore-video-samples.test.mjs`: manifest, redaction, stop-on-error, and rollback request tests.
- Modify `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`: redacted local/candidate/production evidence.

### Task 1: Define sample identity and hidden managed-file persistence

**Files:**
- Create: `model/yucore_media_sample_test.go`
- Create: `model/yucore_media_sample.go`
- Modify: `model/yucore_media.go`

- [ ] **Step 1: Write failing model tests**

Use deterministic input and assert observable contracts:

```go
func TestYucoreMediaSampleIdentityIsOwnerScopedAndDeterministic(t *testing.T) {
	sha := strings.Repeat("a", 64)
	firstID, err := YucoreMediaSampleTaskID(42, sha)
	require.NoError(t, err)
	secondID, err := YucoreMediaSampleTaskID(42, strings.ToUpper(sha))
	require.NoError(t, err)
	otherOwnerID, err := YucoreMediaSampleTaskID(7, sha)
	require.NoError(t, err)

	assert.Equal(t, firstID, secondID)
	assert.NotEqual(t, firstID, otherOwnerID)
	assert.LessOrEqual(t, len(firstID), 64)
	assert.Equal(t, "sample_"+sha+".mp4", YucoreMediaSampleFileName(sha))
}

func TestYucoreMediaSampleManagedFileStaysOutOfPublicJSON(t *testing.T) {
	assets := []YucoreMediaAsset{{
		Id: "sample_asset", Kind: "video", Url: "/api/yucore/media/tasks/id/assets/0",
		ManagedFileName: "sample_" + strings.Repeat("b", 64) + ".mp4",
	}}
	raw, err := marshalYucoreMediaAssets(assets)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "managed_file_name")

	task := &YucoreMediaTask{Assets: YucoreMediaAssets(raw)}
	publicAssets := YucoreMediaTaskAssets(task)
	require.Len(t, publicAssets, 1)
	encoded, err := common.Marshal(publicAssets)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "managed_file")
	assert.NotContains(t, string(encoded), "sample_"+strings.Repeat("b", 64)+".mp4")
}
```

Add table cases rejecting non-positive owner IDs, non-hex checksums, wrong checksum length, missing metadata, and ordinary tasks whose prompt happens to mention the collection.

- [ ] **Step 2: Run the tests and verify missing-symbol failures**

```powershell
go test ./model -run 'TestYucoreMediaSample' -count=1
```

Expected: FAIL because sample helpers and `ManagedFileName` do not exist.

- [ ] **Step 3: Implement the sample domain constants and identity helpers**

Create `model/yucore_media_sample.go` with these public constants:

```go
const (
	YucoreMediaSampleCollectionID   = "video-model-examples"
	YucoreMediaSampleCollectionName = "视频模型示例"
	YucoreMediaSampleTaskPrefix     = "yu_sample_"
	YucoreMediaSampleMode           = "admin-sample-import"
)
```

Normalize SHA-256 to lowercase, require exactly 64 hex characters, and derive:

```go
func YucoreMediaSampleTaskID(userID int, checksum string) (string, error) {
	checksum, err := normalizeYucoreMediaSampleSHA256(checksum)
	if err != nil || userID <= 0 {
		return "", errors.New("invalid YuCore media sample identity")
	}
	prefix := fmt.Sprintf("%s%d_", YucoreMediaSampleTaskPrefix, userID)
	return prefix + checksum[:64-len(prefix)], nil
}

func YucoreMediaSampleFileName(checksum string) string {
	checksum, err := normalizeYucoreMediaSampleSHA256(checksum)
	if err != nil {
		return ""
	}
	return "sample_" + checksum + ".mp4"
}
```

`IsYucoreMediaSampleTask` must require all of: deterministic prefix, `Mode`, `Metadata.imported_sample == true`, and fixed `Metadata.collection_id`. Do not classify a task from user-controlled metadata alone.

- [ ] **Step 4: Persist the hidden managed-file field**

Add to `YucoreMediaAsset`:

```go
ManagedFileName string `json:"-"`
```

Add to `yucoreMediaPersistedAsset`:

```go
ManagedFileName string `json:"managed_file_name,omitempty"`
```

Copy the field in both `YucoreMediaTaskAssets` and `marshalYucoreMediaAssets`. Public JSON remains unchanged because the public field has `json:"-"`.

- [ ] **Step 5: Add direct sample task persistence**

Implement `CreateYucoreMediaSampleTask` without calling the normal generation path. It validates sample identity and creates an already-completed task with `Cost=0`, `Progress=100`, current timestamps, one video asset, and no runnable adapter goroutine. It uses `DB.Create` and lets GORM preserve all database dialects.

- [ ] **Step 6: Run tests, format, and commit**

```powershell
gofmt -w model/yucore_media.go model/yucore_media_sample.go model/yucore_media_sample_test.go
go test ./model -run 'TestYucoreMediaSample|TestYucoreMediaTaskAssets' -count=1
git add model/yucore_media.go model/yucore_media_sample.go model/yucore_media_sample_test.go
git commit -m "feat: add managed video sample tasks"
```

Expected: focused tests PASS.

### Task 2: Add idempotent admin import and rollback APIs

**Files:**
- Create: `controller/yucore_media_sample_test.go`
- Create: `controller/yucore_media_sample.go`
- Modify: `controller/yucore_media.go`
- Modify: `router/api-router.go`
- Modify: `middleware/audit.go`

- [ ] **Step 1: Write failing import behavior tests**

Build a multipart request with `yucoreMediaTestFTYP("isom", "mp41")` and test:

```go
func TestImportYucoreMediaSampleCreatesZeroCostCompletedTask(t *testing.T) {
	// Initialize a temporary SQLite DB and upload root, set context id=42 and admin role.
	// POST file, model_id=seedance-2.0, exact sha256, collection_id=video-model-examples.
	// Assert success, created=true, status record completed, progress=100, cost=0,
	// one managed video asset, exact checksum metadata, and byte-identical stored file.
}

func TestImportYucoreMediaSampleIsIdempotent(t *testing.T) {
	// Execute the same request twice.
	// Assert the same task_id, first created=true, second created=false,
	// one DB row, and one deterministic final file.
}
```

Add cases for non-admin 403, wrong collection, unknown/non-video/probe model, malformed checksum, checksum mismatch, spoofed MIME, oversized body, DB failure cleanup, existing-row/missing-file conflict, and two concurrent identical imports.

- [ ] **Step 2: Run the controller tests and verify failures**

```powershell
go test ./controller -run 'TestImportYucoreMediaSample' -count=1
```

Expected: FAIL because handlers and routes do not exist.

- [ ] **Step 3: Generalize verified upload finalization**

Refactor the current `storeYucoreMediaUpload` internals into a validator-capable helper:

```go
func storeYucoreMediaUploadValidated(
	reader io.Reader,
	finalPath string,
	maxBytes int64,
	validate func(written int64) error,
) (written int64, returnErr error)
```

Call `validate` after the temp file is closed and before `os.Rename`. Keep `storeYucoreMediaUpload` as a one-line wrapper passing `nil` so ordinary uploads preserve behavior. On validation failure the deferred cleanup removes only the temp file.

- [ ] **Step 4: Implement the import handler**

`ImportYucoreMediaSample` must:

1. Require current role `>= common.RoleAdminUser` through route middleware.
2. Limit the multipart body to the existing video-upload maximum plus form overhead.
3. Accept only `file`, `model_id`, `sha256`, and fixed `collection_id`.
4. Confirm the configured capability exists, is enabled, and has `Kind == video` using `model.GetYucoreMediaCatalogSettings()`.
5. Sniff and require canonical `video/mp4`.
6. Stream through `sha256.New()` into `storeYucoreMediaUploadValidated`.
7. Compare the actual hash before rename.
8. Create the deterministic zero-cost task.
9. On unique conflict, fetch and validate the existing owner/sample/hash/file and return `created:false`.
10. On any other DB error, remove the deterministic final file only when no valid existing task owns it.

Return only:

```go
gin.H{
	"created":   created,
	"task_id":  task.TaskId,
	"asset_url": fmt.Sprintf("/api/yucore/media/tasks/%s/assets/0", task.TaskId),
	"sha256":   checksum,
	"size":     written,
}
```

- [ ] **Step 5: Implement exact rollback**

`DeleteYucoreMediaSample` fetches by owner and task ID, requires `IsYucoreMediaSampleTask`, and validates the managed file under that owner directory. It then performs an exact two-phase rollback:

1. Atomically rename the final file to a unique quarantine name in the same owner directory.
2. Soft-delete the task with GORM.
3. If the database delete fails, atomically rename the quarantined file back.
4. If database delete succeeds, remove only the quarantined file.
5. If quarantine removal fails, restore `deleted_at` with `Unscoped()` and rename the file back before returning an error.

Never glob, remove an owner directory, or delete the source file.

- [ ] **Step 6: Register audited admin routes**

Inside the authenticated media route add:

```go
mediaAdminRoute := mediaRoute.Group("/admin")
mediaAdminRoute.Use(middleware.AdminAuth())
{
	mediaAdminRoute.POST("/sample-assets", controller.ImportYucoreMediaSample)
	mediaAdminRoute.DELETE("/sample-assets/:task_id", controller.DeleteYucoreMediaSample)
}
```

Add audit names in `middleware/audit.go`:

```go
"POST /api/yucore/media/admin/sample-assets":            "yucore.media_sample_import",
"DELETE /api/yucore/media/admin/sample-assets/:task_id": "yucore.media_sample_delete",
```

- [ ] **Step 7: Run tests and commit**

```powershell
gofmt -w controller/yucore_media.go controller/yucore_media_sample.go controller/yucore_media_sample_test.go router/api-router.go middleware/audit.go
go test ./controller ./model ./middleware -run 'YucoreMediaSample|YucoreMediaUpload|AdminAudit' -count=1
git add controller/yucore_media.go controller/yucore_media_sample.go controller/yucore_media_sample_test.go router/api-router.go middleware/audit.go
git commit -m "feat: add admin video sample import"
```

Expected: all focused tests PASS.

### Task 3: Enforce sample visibility and serve managed videos privately

**Files:**
- Modify: `model/yucore_media.go`
- Modify: `controller/yucore_media.go`
- Modify: `controller/yucore_media_sample.go`
- Modify: `controller/yucore_media_sample_test.go`

- [ ] **Step 1: Write failing access-control tests**

Create one sample owned by admin 42 and one ordinary task. Assert:

- Admin 42 sees both in task list/gallery and can read sample detail.
- User 7 sees only their ordinary tasks and receives 404 for admin sample detail/content.
- Demoting owner 42 below `RoleAdminUser` hides the sample from list/gallery/detail/content.
- The generic task DELETE and PATCH endpoints reject sample tasks.
- The dedicated rollback endpoint still requires a current admin owner.
- A valid owner Range request `bytes=0-15` returns 206, exact bytes, `Content-Range`, `video/mp4`, `Cache-Control: private`, and `X-Content-Type-Options: nosniff`.

- [ ] **Step 2: Run and verify the access tests fail**

```powershell
go test ./controller ./model -run 'TestYucoreMediaSample.*(Access|Gallery|Range|Delete|Demoted)' -count=1
```

Expected: FAIL because existing list/detail/content/delete paths do not recognize samples.

- [ ] **Step 3: Add query-level sample exclusion**

Add `includeAdminSamples bool` to local list/count functions and apply a portable predicate when false:

```go
if !includeAdminSamples {
	query = query.Where("task_id NOT LIKE ?", YucoreMediaSampleTaskPrefix+"%")
}
```

Thread the boolean through `ListYucoreMediaTasks`, `ListYucoreMediaTasksWithHeaders`, `CountYucoreMediaTasks`, and `ListYucoreMergedUAGProxyMediaTasks`. Controller list/gallery calls set it from `c.GetInt("role") >= common.RoleAdminUser`. This keeps totals and pagination from leaking hidden sample counts.

- [ ] **Step 4: Centralize owner-role sample access**

Add a controller helper that returns 404 for a sample when the current role is below admin. Use it after owner-scoped lookup in task GET, PATCH, DELETE, and asset serving. Generic DELETE/PATCH returns a stable error for any sample even when the owner is still admin, directing deletion through the dedicated rollback path.

- [ ] **Step 5: Serve the hidden managed file before remote-source resolution**

After owner and role checks in `ServeYucoreMediaTaskAsset`, branch on `asset.ManagedFileName`:

```go
managedPath, err := yucoreMediaSafeUploadPath(task.UserId, asset.ManagedFileName)
if err != nil {
	c.String(http.StatusNotFound, "asset not found")
	return
}
c.Header("Content-Type", "video/mp4")
c.Header("Cache-Control", "private, max-age=86400")
c.Header("X-Content-Type-Options", "nosniff")
c.File(managedPath)
return
```

Do not resolve a provider URL, attach provider headers, or expose the file path. Gin/`http.ServeFile` handles byte ranges.

- [ ] **Step 6: Run focused and full media tests, then commit**

```powershell
gofmt -w model/yucore_media.go controller/yucore_media.go controller/yucore_media_sample.go controller/yucore_media_sample_test.go
go test ./controller ./model -run 'YucoreMediaSample|YucoreMediaTask|YucoreMediaUpload|ServeYucoreMedia' -count=1
git add model/yucore_media.go controller/yucore_media.go controller/yucore_media_sample.go controller/yucore_media_sample_test.go
git commit -m "fix: protect managed video sample assets"
```

Expected: focused tests PASS.

### Task 4: Present a private sample collection in Studio

**Files:**
- Create: `web/default/src/features/yucore-brand/lib/media-gallery.test.ts`
- Create: `web/default/src/features/yucore-brand/lib/media-gallery.ts`
- Modify: `web/default/src/features/yucore-brand/components/yucore-studio-workspace.tsx`
- Temporarily modify: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Write the failing grouping test**

```ts
import { describe, expect, test } from 'bun:test'
import { splitMediaGalleryTasks } from './media-gallery'

test('separates only server-identified sample tasks', () => {
  const sample = task({
    task_id: 'yu_sample_42_abc',
    mode: 'admin-sample-import',
    metadata: { imported_sample: true, collection_id: 'video-model-examples' },
  })
  const spoof = task({
    task_id: 'yu_regular',
    mode: 'text-to-video',
    metadata: { imported_sample: true, collection_id: 'video-model-examples' },
  })
  const result = splitMediaGalleryTasks([sample, spoof])
  expect(result.samples.map((item) => item.task_id)).toEqual([sample.task_id])
  expect(result.personal.map((item) => item.task_id)).toEqual([spoof.task_id])
})
```

- [ ] **Step 2: Run and verify missing-module failure**

```powershell
Set-Location web/default
bun test src/features/yucore-brand/lib/media-gallery.test.ts
```

Expected: FAIL because `media-gallery.ts` does not exist.

- [ ] **Step 3: Implement the pure grouping helper**

Require task ID prefix, mode, boolean marker, and fixed collection ID. Preserve input order and return `{ samples, personal }` without mutating input.

- [ ] **Step 4: Render two un-nested page sections**

Derive the split with `useMemo` from `completedTasks`. In the existing assets view:

- If `samples.length > 0`, render an unframed section headed with `t('Video model examples')` and an explanatory line, then `AssetGrid` for samples.
- Render personal assets below under `t('Your generated assets')` only when the sample section exists; otherwise preserve the current single-grid presentation.
- Pass the existing `addAssetToCanvas` callback to both grids so videos can be placed on Canvas.
- Keep the existing download button and do not add public/share controls.
- Use stable `gap-*`, existing semantic colors, and no nested cards.

- [ ] **Step 5: Apply new translations through the required script**

Use `add-missing-keys.mjs` for these exact keys in all six locales:

```js
const newKeys = {
  en: {
    'Video model examples': 'Video model examples',
    'Verified private examples available only to administrators.': 'Verified private examples available only to administrators.',
    'Your generated assets': 'Your generated assets',
  },
  zh: {
    'Video model examples': '视频模型示例',
    'Verified private examples available only to administrators.': '仅管理员可见的已验证私有示例。',
    'Your generated assets': '你的生成素材',
  },
  fr: {
    'Video model examples': 'Exemples de modèles vidéo',
    'Verified private examples available only to administrators.': 'Exemples privés vérifiés, accessibles uniquement aux administrateurs.',
    'Your generated assets': 'Vos médias générés',
  },
  ja: {
    'Video model examples': '動画モデルのサンプル',
    'Verified private examples available only to administrators.': '管理者のみが利用できる検証済みの非公開サンプルです。',
    'Your generated assets': '生成した素材',
  },
  ru: {
    'Video model examples': 'Примеры видеомоделей',
    'Verified private examples available only to administrators.': 'Проверенные приватные примеры, доступные только администраторам.',
    'Your generated assets': 'Ваши созданные материалы',
  },
  vi: {
    'Video model examples': 'Ví dụ mô hình video',
    'Verified private examples available only to administrators.': 'Các ví dụ riêng tư đã xác minh, chỉ dành cho quản trị viên.',
    'Your generated assets': 'Tài nguyên bạn đã tạo',
  },
}
```

Run the script, then `bun run i18n:sync`, then restore the script before staging.

- [ ] **Step 6: Verify and commit the Studio grouping**

```powershell
bun test src/features/yucore-brand/lib/media-gallery.test.ts
bun run typecheck
bunx oxlint src/features/yucore-brand/lib/media-gallery.ts src/features/yucore-brand/lib/media-gallery.test.ts src/features/yucore-brand/components/yucore-studio-workspace.tsx
bun run i18n:sync
git add src/features/yucore-brand/lib/media-gallery.ts src/features/yucore-brand/lib/media-gallery.test.ts src/features/yucore-brand/components/yucore-studio-workspace.tsx src/i18n/locales
git commit -m "feat: group admin video samples in Studio"
```

Expected: all commands exit 0.

### Task 5: Add a redacted sequential production import tool

**Files:**
- Create: `scripts/production/import-yucore-video-samples.test.mjs`
- Create: `scripts/production/import-yucore-video-samples.mjs`

- [ ] **Step 1: Write failing script tests**

Test that the tool:

- Contains exactly the ten approved file-to-model mappings.
- Computes lowercase SHA-256 and file size.
- Sends one file at a time in fixed order.
- Stops after the first non-success response.
- Never includes `YUAPI_ADMIN_AUTH_HEADER` in logs or result JSON.
- Writes an import result with `file_name`, deterministic `managed_file_name`, `model_id`, `sha256`, `size`, `task_id`, and `created`.
- In rollback mode, deletes only task IDs from the provided result manifest in reverse order.

- [ ] **Step 2: Run and verify missing-module failure**

```powershell
bun test scripts/production/import-yucore-video-samples.test.mjs
```

Expected: FAIL because the import module does not exist.

- [ ] **Step 3: Implement the fixed manifest and CLI**

Use exactly:

```js
export const VIDEO_SAMPLES = [
  ['happyhouse-1.0.mp4', 'happyhouse-1.0'],
  ['happyhouse-1.1.mp4', 'happyhouse-1.1'],
  ['minimax-h3-2k.mp4', 'minimax-h3-2k'],
  ['omni-fast-no-water.mp4', 'omni-fast-no-water'],
  ['omni-fast.mp4', 'omni-fast'],
  ['omni-v2v-no-water.mp4', 'omni-v2v-no-water'],
  ['omni-v2v.mp4', 'omni-v2v'],
  ['sd7-seedance-2.0-1080p.mp4', 'sd7-seedance-2.0-1080p'],
  ['sd7-seedance-2.0-720p.mp4', 'sd7-seedance-2.0-720p'],
  ['seedance-2.0.mp4', 'seedance-2.0'],
]
```

Required import arguments: `--base-url`, `--source-dir`, `--result-file`. Read auth only from `YUAPI_ADMIN_AUTH_HEADER`, require HTTPS unless base URL is loopback, and never print the header. Write the result file atomically with mode `0600`; reject a result path inside the Git worktree.

Required rollback arguments: `--base-url`, `--rollback-manifest`. Read task IDs only from that manifest and call the dedicated DELETE endpoint. Do not delete source files locally.

- [ ] **Step 4: Run tests and a mock-server dry run**

```powershell
bun test scripts/production/import-yucore-video-samples.test.mjs
```

Expected: all tests PASS, including stop-on-error and auth redaction.

- [ ] **Step 5: Commit the operator tool**

```powershell
git add scripts/production/import-yucore-video-samples.mjs scripts/production/import-yucore-video-samples.test.mjs
git commit -m "ops: add private video sample importer"
```

### Task 6: Verify all behavior locally with the ten retained files

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Verify the source set without changing it**

Scope amendment (2026-08-16): canceled by the user. The original provider
files were unavailable, and the user explicitly declined regeneration and
production import. Steps 2-7 remain valid local verification evidence; this
unchecked source-file step is no longer a release gate.

Resolve the user-approved source directory outside Git. Assert exactly ten `.mp4` files and no other import candidates. Compute names, sizes, MIME/container data, durations, and SHA-256 into an operator-local manifest. Do not move, rename, rewrite, or delete any source file.

- [x] **Step 2: Run backend verification**

```powershell
go test ./model ./controller ./middleware -run 'YucoreMediaSample|YucoreMediaUpload|ServeYucoreMedia|AdminAudit' -count=1
go test ./model ./controller ./middleware -count=1
go test -p 1 ./... -count=1
```

Expected: all packages PASS.

- [x] **Step 3: Run frontend and operator-tool verification**

```powershell
Set-Location web/default
bun test src/features/yucore-brand/lib/media-gallery.test.ts src/features/docs/document-locale.test.ts scripts/check-video-api-docs.test.mjs
bun run docs:check
bun run i18n:sync
bun run typecheck
bun run build
Set-Location ../..
bun test scripts/production/import-yucore-video-samples.test.mjs
git diff --check
```

Expected: all commands exit 0.

- [x] **Step 4: Import all ten into a disposable local admin account**

Start the application with a temporary SQLite database and temporary `YUCORE_MEDIA_UPLOAD_DIR`. Use a synthetic admin authorization value and the operator tool. Verify import count 10, `created=true` for the first run, `created=false` for the second run, zero quota change, ten completed zero-cost tasks, and byte-for-byte checksum equality.

- [x] **Step 5: Verify browser and permission behavior**

Using Playwright desktop and mobile:

- The synthetic admin sees “视频模型示例” with exactly ten playable videos.
- Each video downloads and can be sent to Canvas.
- Removing a Canvas node does not remove the gallery item or source file.
- A second ordinary user cannot list, fetch detail, play, or download any sample.
- After demoting the owner below admin in the disposable DB, the former owner cannot list or read samples.
- Restoring the role restores access without rewriting files.

- [x] **Step 6: Verify exact rollback locally**

Run manifest rollback. Assert the ten imported task rows and ten managed copies are gone, while all ten original source files and their SHA-256 values remain unchanged. Assert ordinary test tasks/uploads remain.

- [x] **Step 7: Record redacted evidence and commit**

Append test counts, local import/idempotency/rollback counts, checksum equality count, browser pass counts, and source-preserved confirmation. Do not record auth, user IDs, absolute source paths, production paths, or private URLs.

```powershell
git add docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md
git commit -m "docs: record private sample asset verification"
```

### Task 7: Prepare a no-stop production candidate and request switch approval

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [x] **Step 1: Push the fully verified branch**

```powershell
git push -u origin codex/cangyuan-video-refresh-20260815
```

Expected: remote branch HEAD equals local HEAD.

- [x] **Step 2: Re-audit production read-only**

Over an interactive SSH session, resolve the current live container, retained rollback container, image IDs, restart counts, health, Caddy container and effective config, stable alias `yuapi-production-live`, attached release network, private bindings, database/Redis health, disk space, and current public static fingerprints. Runtime readback overrides older handoff names. Do not print or save environment secrets.

- [x] **Step 3: Capture scoped rollback artifacts**

Create a new root-only server directory with mode `0700`. Save checksummed copies of only the active Caddy config, current live container inspect output with environment values redacted, image ID, network alias mapping, and the exact sample-task IDs/files that would be created. Use file mode `0600`. Do not dump the database or user tables.

- [x] **Step 4: Build and start the parallel candidate**

Build a unique image tagged with the reviewed short commit. Start a uniquely named container on a new loopback-only port and attach it to the currently audited release network under a new candidate alias. Keep the live and rollback containers running/preserved, and keep every hostname referenced by any active Caddy generation reachable.

- [x] **Step 5: Verify the candidate privately**

Before changing public routing, verify health/restart count, revision label, new static fingerprints, three docs URLs and locale selector, `docs:check` contract, admin import endpoint authorization, ordinary-user denial, database/Redis error counts, and Caddy-to-candidate connectivity. Reuse the completed disposable local MP4 import/idempotency/play/rollback evidence; do not import a production sample after the user removed samples from scope. Do not use any active user's API Key or assets.

- [x] **Step 6: Record evidence and stop at the traffic gate**

Append redacted candidate facts to the handoff. Keep exact container identifiers and image digest in the protected server-local artifact; record private health, alias isolation, Caddy reachability, rollback readiness, and the user-canceled sample scope in Git. Obtain explicit user approval before switching the stable alias or public traffic.

### Task 8: Hot cutover, observation, and retained rollback

Scope amendment (2026-08-16): production sample generation and import are
canceled. A successful media-chain cutover must keep the production sample
count at zero and must not spend upstream generation credit.

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Re-run the final no-stop guard after approval**

Confirm live and candidate health/restart count, all currently referenced Caddy hostnames are reachable, staged Caddy config validates, database/Redis are healthy, and the candidate image digest still matches the approved evidence. Abort without changing traffic on any mismatch.

- [ ] **Step 2: Move only the stable live alias and gracefully reload Caddy if required**

Attach `yuapi-production-live` to the candidate before detaching/remapping it from the prior live app. Preserve old hostname aliases for existing Caddy generations. Use `caddy validate` and graceful `caddy reload`; never restart Caddy and never stop the old application first.

- [ ] **Step 3: Verify public traffic before data import**

Check public health, homepage, sign-in, `/keys`, pricing, Studio, Canvas, all three docs URLs, static fingerprints, restart counts, and aggregate Caddy 502/database/Redis errors. On failure, remap the stable alias to the retained old app while both remain running.

- [ ] **Step 4: Preserve the empty production sample scope**

Do not set an administrator authorization value, run the importer, or call an
upstream generation endpoint. Verify the managed sample count remains zero and
that no sample file or task was created or deleted during the switch.

- [ ] **Step 5: Verify the public media authorization boundary**

Verify anonymous task-asset access remains denied and the public Studio and
Canvas pages render normally. Do not borrow an active user's session, Key,
balance, or assets; the authenticated play/download/Canvas behavior is covered
by the completed isolated browser verification.

- [ ] **Step 6: Observe and retain rollback**

Take repeated samples for at least five minutes covering public 200s, health, restart count, Caddy 502, application fatal errors, database/Redis errors, docs assets, and sample content Range requests. Keep the previous live image/container, stable-alias rollback operation, result manifest, and original ten source files. Do not clean them up without separate approval.

- [ ] **Step 7: Record redacted final evidence and commit**

Update the handoff with deployed commit/image facts, hot-switch timestamps, public checks, 10/10 checksum result, role tests, zero billing delta, observation counts, and exact retained rollback identifier. Exclude server address, auth, user ID, file paths, source domain, and private URLs.

```powershell
git add docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md
git commit -m "docs: record video docs and sample release"
git push
```

Completion requires a clean worktree, pushed HEAD, three public docs editions,
no privacy-scan findings, zero production sample imports, no production 502,
and a live no-stop rollback target.
