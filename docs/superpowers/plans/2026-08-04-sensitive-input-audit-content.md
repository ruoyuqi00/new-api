# Sensitive Input Audit Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store enough admin-only evidence for newly blocked local sensitive-word requests, while automatically removing the original input and matched words after seven days without altering billing records.

**Architecture:** The relay passes only the already-normalized checked text and matched terms into the existing violation-fee log path. The log model owns redaction and portable batched purging, while the existing system-task scheduler runs cleanup daily and exposes a root-only manual trigger. The default frontend adds the admin detail view, retention setting, and cleanup action without exposing evidence through user log APIs.

**Tech Stack:** Go 1.22+, Gin, GORM v2 (SQLite/MySQL/PostgreSQL), React 19, TypeScript, React Hook Form/Zod, TanStack Query, Bun, i18next.

---

### Task 1: Capture bounded evidence and redact user log APIs

**Files:**
- Modify: `controller/relay.go`
- Modify: `service/violation_fee.go`
- Modify: `service/violation_fee_test.go`
- Modify: `model/log.go`
- Test: `model/log_sensitive_input_audit_test.go`

- [ ] **Step 1: Write failing service tests for bounded UTF-8 evidence**

Add deterministic tests which call the local violation logging path with duplicate terms and input larger than 64 KiB, then assert that `logs.content` is valid UTF-8 and no larger than 65,536 bytes, and that `other` contains deduplicated `sensitive_words`, `sensitive_input_original_bytes`, and `sensitive_input_truncated`.

```go
assert.LessOrEqual(t, len(saved.Content), 64*1024)
assert.True(t, utf8.ValidString(saved.Content))
assert.Equal(t, []any{"blocked"}, other["sensitive_words"])
assert.Equal(t, float64(len(input)), other["sensitive_input_original_bytes"])
assert.Equal(t, true, other["sensitive_input_truncated"])
```

- [ ] **Step 2: Write a failing model test for self-log redaction**

Create one local-sensitive violation log and one normal log, call `formatUserLogs`, and assert only the violation log is rewritten and stripped.

```go
formatUserLogs(logs, 0)
assert.Equal(t, SensitiveInputBlockedLogContent, logs[0].Content)
assert.NotContains(t, common.StrToMap(logs[0].Other), "sensitive_words")
assert.Equal(t, "normal content", logs[1].Content)
```

- [ ] **Step 3: Run the focused tests and observe failure**

Run: `go test ./service ./model -run 'Sensitive(Input|Violation)' -count=1`

Expected: FAIL because the charge API does not accept evidence and self-log formatting does not redact it.

- [ ] **Step 4: Implement minimal capture and redaction**

Define the durable log constants in `model/log.go`, update `formatUserLogs`, and extend the charge call without modifying charge calculation or retry behavior.

```go
const (
    SensitiveInputBlockedLogContent = "Sensitive input blocked"
    SensitiveInputViolationReason   = "local_sensitive_word"
)

if other["violation_fee_reason"] == SensitiveInputViolationReason {
    log.Content = SensitiveInputBlockedLogContent
    delete(other, "sensitive_words")
}
```

In `controller/relay.go`, retain the matcher result and pass `meta.CombineText` plus the matched words only after the existing quota calculation. In `service/violation_fee.go`, truncate at a UTF-8 boundary, deduplicate non-empty matched words, put the bounded content in `RecordConsumeLogParams.Content`, and store only the approved metadata. Never log the content or words through `logger`.

- [ ] **Step 5: Run focused tests and commit**

Run: `go test ./service ./model ./controller -run 'Sensitive(Input|Violation)' -count=1`

Expected: PASS.

```bash
git add controller/relay.go service/violation_fee.go service/violation_fee_test.go model/log.go model/log_sensitive_input_audit_test.go
git commit -m "feat: retain sensitive input audit evidence"
```

### Task 2: Add portable seven-day evidence purging

**Files:**
- Create: `model/log_sensitive_input_cleanup.go`
- Test: `model/log_sensitive_input_cleanup_test.go`
- Modify: `setting/sensitive.go`
- Modify: `setting/sensitive_test.go`
- Modify: `model/option.go`

- [ ] **Step 1: Write failing retention-setting tests**

Test the default of 7, accepted values 1 and 365, and rejection of 0, 366, and non-numeric values without changing the previous value.

```go
require.NoError(t, UpdateSensitiveInputRetentionDays("7"))
assert.Equal(t, 7, SensitiveInputRetentionDays)
assert.Error(t, UpdateSensitiveInputRetentionDays("0"))
assert.Equal(t, 7, SensitiveInputRetentionDays)
```

- [ ] **Step 2: Write failing cleanup model tests**

Using the project database fixture, insert expired and unexpired local-sensitive logs plus an unrelated consume log. Call one batch and assert only expired evidence is cleared: content becomes the generic marker, `sensitive_words` is removed, purge metadata is added, and quota, prompt tokens, model, user, status, and violation reason are unchanged.

```go
result, err := PurgeExpiredSensitiveInputAuditBatch(ctx, cutoff, 200, purgedAt)
require.NoError(t, err)
assert.Equal(t, int64(1), result.Purged)
assert.Equal(t, originalQuota, expired.Quota)
assert.Equal(t, true, expiredOther["sensitive_input_purged"])
```

- [ ] **Step 3: Run tests and observe failure**

Run: `go test ./setting ./model -run 'SensitiveInput(Retention|Cleanup)' -count=1`

Expected: FAIL because retention parsing and purge APIs do not exist.

- [ ] **Step 4: Implement settings and GORM cleanup**

Add `SensitiveInputRetentionDays = 7` plus a strict 1..365 parser in `setting/sensitive.go`; expose it from `InitOptionMap` and apply it through `updateOptionMap` in `model/option.go`.

Implement a GORM-only cleanup batch in the new model file. Select a bounded set using `type`, `created_at`, and a conservative `LIKE` marker; parse `Other` with `common.StrToMap`; verify `violation_fee_reason` in Go; update each qualifying row by ID. Do not use database JSON operators or dialect-specific SQL.

```go
other := common.StrToMap(log.Other)
if other["violation_fee_reason"] != SensitiveInputViolationReason {
    continue
}
delete(other, "sensitive_words")
other["sensitive_input_purged"] = true
other["sensitive_input_purged_at"] = purgedAt
```

- [ ] **Step 5: Run tests and commit**

Run: `go test ./setting ./model -run 'SensitiveInput(Retention|Cleanup)' -count=1`

Expected: PASS.

```bash
git add setting/sensitive.go setting/sensitive_test.go model/option.go model/log_sensitive_input_cleanup.go model/log_sensitive_input_cleanup_test.go
git commit -m "feat: purge expired sensitive input evidence"
```

### Task 3: Schedule cleanup daily and expose a root-only manual trigger

**Files:**
- Modify: `model/system_task.go`
- Create: `service/sensitive_input_cleanup_task.go`
- Test: `service/sensitive_input_cleanup_task_test.go`
- Modify: `controller/system_task.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write failing system-task tests**

Assert the handler type is registered and scheduled every 24 hours, snapshots the retention days into its payload, drains multiple batches, records only purge counts in state/result, and returns the active task when manual creation races.

```go
handler := sensitiveInputCleanupHandler{}
assert.Equal(t, 24*time.Hour, handler.Interval())
assert.True(t, handler.Enabled())
assert.Equal(t, SensitiveInputCleanupPayload{RetentionDays: 7, BatchSize: 200}, handler.NewPayload())
```

- [ ] **Step 2: Run tests and observe failure**

Run: `go test ./service -run SensitiveInputCleanupTask -count=1`

Expected: FAIL because the task type and handler do not exist.

- [ ] **Step 3: Implement the scheduled handler and start function**

Add `SystemTaskTypeSensitiveInputCleanup = "sensitive_input_cleanup"`. Implement `ScheduledSystemTaskHandler` in a focused file, register it in `init`, compute the cutoff from its payload, repeatedly call the model batch until zero rows are purged, update progress safely, and finish with `purged_count` only.

```go
func (sensitiveInputCleanupHandler) Enabled() bool { return setting.SensitiveInputRetentionDays > 0 }
func (sensitiveInputCleanupHandler) Interval() time.Duration { return 24 * time.Hour }
func (sensitiveInputCleanupHandler) NewPayload() any {
    return SensitiveInputCleanupPayload{RetentionDays: setting.SensitiveInputRetentionDays, BatchSize: 200}
}
```

Expose `POST /api/system-task/sensitive-input-cleanup` inside the existing `RootAuth` route group. The controller starts or returns the active task and never accepts an arbitrary cutoff from the request.

- [ ] **Step 4: Run backend tests and commit**

Run: `go test ./model ./setting ./service ./controller ./router -count=1`

Expected: PASS.

```bash
git add model/system_task.go service/sensitive_input_cleanup_task.go service/sensitive_input_cleanup_task_test.go controller/system_task.go router/api-router.go
git commit -m "feat: schedule sensitive input cleanup"
```

### Task 4: Add admin evidence UI, retention controls, and translations

**Files:**
- Modify: `web/default/src/features/usage-logs/types.ts`
- Modify: `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/api.ts`
- Modify: `web/default/src/features/system-settings/security/index.tsx`
- Modify: `web/default/src/features/system-settings/security/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/request-limits/sensitive-words-section.tsx`
- Modify: `web/default/src/features/system-info/components/system-tasks-panel.tsx`
- Temporarily create then delete: `web/default/scripts/add-missing-keys.mjs`
- Modify mechanically: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Add frontend types and API client**

Extend `LogOther` with the approved sensitive evidence fields, `SystemOptions` with `SensitiveInputRetentionDays`, and system-task types with the cleanup payload/state/result. Add a typed POST client for `/api/system-task/sensitive-input-cleanup`.

```ts
export type SensitiveInputCleanupTask = SystemTask<
  { retention_days: number; batch_size: number },
  { purged: number; progress: number },
  { purged_count: number }
>
```

- [ ] **Step 2: Render admin-only evidence and cleanup status**

In the details dialog, only render the blocked-input section when `isAdmin` and `violation_fee_reason === 'local_sensitive_word'`. Show matched terms, truncation/original-size metadata, or the purged state; constrain the full text in a scrollable, wrapping container. Suppress the duplicate generic content section for this case.

- [ ] **Step 3: Add retention and manual-cleanup controls**

Add a number input constrained to 1..365 to the sensitive-words form. Add a destructive-confirmation dialog and mutation for manual cleanup; on success show the returned task state and leave cleanup execution to the existing task runner. Add the task label to the system task panel.

- [ ] **Step 4: Add all six locale translations through the mandated script**

Create `web/default/scripts/add-missing-keys.mjs` using `readFileSync`/`writeFileSync` and explicit translations for every new source key. Run:

```bash
cd web/default
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Delete the temporary script with `apply_patch`; do not hand-edit locale JSON.

- [ ] **Step 5: Build frontend and commit**

Run: `cd web/default && bun run build`

Expected: production build succeeds with no TypeScript or i18n errors.

```bash
git add web/default/src/features web/default/src/i18n/locales
git commit -m "feat: manage sensitive input audit retention"
```

### Task 5: Full verification, push, and no-downtime production deployment

**Files:**
- Verify all changed files
- Update deployment image only on `199.231.85.194`

- [ ] **Step 1: Run backend and frontend verification**

Run:

```bash
gofmt -w controller/relay.go controller/system_task.go model/log.go model/log_sensitive_input_cleanup.go model/log_sensitive_input_cleanup_test.go model/option.go model/system_task.go service/violation_fee.go service/violation_fee_test.go service/sensitive_input_cleanup_task.go service/sensitive_input_cleanup_task_test.go setting/sensitive.go setting/sensitive_test.go
go test ./model ./setting ./service ./controller ./router -count=1
go test ./... -count=1
cd web/default && bun run build
```

Expected: every command exits 0.

- [ ] **Step 2: Review the final diff and commit residual formatting changes**

Run: `git diff --check && git status --short && git diff --stat HEAD~4`

Verify there are no secrets, raw request logging, database-specific JSON operations, or changes to unrelated pricing/routing behavior. Commit any required formatting-only residue.

- [ ] **Step 3: Push the feature branch**

Run: `git push -u origin codex/sensitive-input-audit-content-20260804`

Expected: branch is present on the configured origin.

- [ ] **Step 4: Build and stage a uniquely tagged production image**

Transfer only the committed source to the server using the existing SSH operations key, build a tag containing the commit prefix, and start a candidate container on the existing Docker network without changing the current Caddy upstream.

- [ ] **Step 5: Verify the candidate and perform the existing rolling swap**

Verify candidate health, `/api/status`, unauthorized `/v1/models`, the root-only cleanup route, and user/admin log redaction contracts without emitting any stored content. Switch the stable `newapi` container name only after the candidate is healthy; keep Caddy running throughout. Roll back immediately if health or direct requests fail.

- [ ] **Step 6: Verify production state**

Confirm the final `newapi` container is healthy with zero restarts, Caddy still routes to `newapi:3000`, all five YuAPI status endpoints remain reachable as previously configured, and the scheduled task type appears without exposing evidence. Report the deployed image tag, commit, test results, and retained seven-day policy.
