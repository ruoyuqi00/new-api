# Log Cleanup Statistics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make historical log cleanup remove matching dashboard aggregates and reduce user, token, and channel historical usage counters without touching balances or payment data.

**Architecture:** Keep the existing asynchronous `log_cleanup` system task. Before deleting rows, snapshot consume-log totals grouped by user, token, and channel into the task state. Apply those deltas and the state marker in one main-database transaction so retries cannot subtract twice, then delete old `logs` and `quota_data` in resumable batches. Prune old in-memory aggregate buckets under the existing mutex.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL main DB, optional ClickHouse log DB, React/TypeScript/Bun.

---

### Task 1: Add portable consume-delta aggregation and aggregate cleanup primitives

**Files:**
- Create: `model/log_cleanup.go`
- Create: `model/log_cleanup_test.go`
- Modify: `model/log.go:712-780`
- Modify: `model/usedata.go:100-125`

- [ ] **Step 1: Write failing model tests**

Add deterministic SQLite tests that insert every log type plus consume rows, then assert `AggregateOldConsumeLogDeltas` returns only pre-cutoff consume sums/counts grouped by user, token, and channel. Add tests that `DeleteOldQuotaDataBatch` removes only rows with `created_at < cutoff`, and that `PruneQuotaDataCacheBefore` removes old buckets while retaining the cutoff bucket.

- [ ] **Step 2: Run the focused tests and verify RED**

Run `go test ./model -run 'LogCleanup|QuotaDataCleanup' -count=1`. It must fail because the aggregation, quota-data deletion, and cache-pruning APIs do not yet exist.

- [ ] **Step 3: Implement the minimal primitives**

Define a serializable `LogCleanupUsageDelta` with `UserQuota`, `UserRequests`, `TokenQuota`, and `ChannelQuota` maps. Query `LOG_DB` with `type = LogTypeConsume`, `created_at < ?`, and grouped columns. Use `Scan` into small row structs and merge duplicate keys. Implement portable GORM batch deletion for `QuotaData`; for ClickHouse use a synchronous `ALTER TABLE quota_data DELETE ...` mutation after a count. Add `PruneQuotaDataCacheBefore(cutoff)` under `CacheQuotaDataLock`, comparing each cached bucket timestamp to the cutoff.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the same focused command and confirm all tests pass on SQLite. Run `gofmt -w model/log_cleanup.go model/log_cleanup_test.go model/log.go model/usedata.go` and `git diff --check`.

- [ ] **Step 5: Commit**

```bash
git add model/log_cleanup.go model/log_cleanup_test.go model/log.go model/usedata.go
git commit -m "feat: aggregate usage deltas for log cleanup"
```

### Task 2: Apply usage corrections exactly once in the cleanup task

**Files:**
- Modify: `service/system_task.go:80-115,370-450`
- Modify: `model/system_task.go:309-345`
- Create: `service/system_task_test.go` (or extend the existing system-task test file)

- [ ] **Step 1: Write failing atomicity and idempotency tests**

Create a task state with a known delta and assert `ApplyLogCleanupUsageAdjustment` reduces user `used_quota` and `request_count`, token `used_quota`, and channel `used_quota` once. Call it twice and assert the second call changes nothing. Add a rollback test using a missing referenced row or forced transaction error and assert the state marker and all counters remain unchanged.

- [ ] **Step 2: Run focused tests and verify RED**

Run `go test ./service -run 'LogCleanupUsage|LogCleanupTask' -count=1`. It must fail before the atomic application API and state marker exist.

- [ ] **Step 3: Implement the transaction**

Extend `LogCleanupState` with the serialized delta maps and `UsageAdjustmentApplied`. Add `ApplyLogCleanupUsageAdjustment(taskID, runnerID)` in `model` that opens a transaction, locks the running `SystemTask`, returns immediately when the marker is true, applies `used_quota`/`request_count` decrements with GORM expressions, writes the marker and state JSON, and commits. Do not call refund, settlement, balance quota, or batch-update queue helpers. Treat missing zero-delta rows as harmless; return database errors before changing the marker.

- [ ] **Step 4: Integrate task startup and resume**

At the start of `runLogCleanupTask`, aggregate old consume deltas once when the state snapshot is empty, persist the snapshot with `UpdateSystemTaskState`, then call the atomic application method. On a resumed task, use the persisted snapshot and marker instead of re-aggregating. Keep existing lock-loss handling and fail the task before deleting rows when correction fails.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run `go test ./service -run 'LogCleanupUsage|LogCleanupTask' -count=1` and `go test ./model -run 'LogCleanup' -count=1`. Confirm the second invocation is a no-op and all balances stay unchanged.

- [ ] **Step 6: Commit**

```bash
git add model/system_task.go service/system_task.go service/*system_task*_test.go
git commit -m "fix: apply log cleanup usage corrections once"
```

### Task 3: Delete logs and statistics together with observable counts

**Files:**
- Modify: `model/log.go:712-780`
- Modify: `model/usedata.go:100-125`
- Modify: `service/system_task.go:390-465`
- Modify: `web/default/src/features/system-settings/types.ts:75-95`
- Modify: `web/default/src/features/system-settings/maintenance/log-settings-section.tsx:390-430,590-620`
- Modify: `web/default/src/features/system-settings/api.ts:50-62`

- [ ] **Step 1: Write failing task/result tests**

Extend cleanup tests to insert old/new logs and `quota_data`, execute the handler, and assert old rows are gone, new rows remain, the result contains `deleted_count` and `deleted_quota_data_count`, and the UI-facing state reports both processed totals. Assert payment/order/task fixtures are not touched.

- [ ] **Step 2: Run tests and verify RED**

Run `go test ./service ./model -run 'LogCleanup' -count=1`. It must fail because quota-data deletion and the second result count are not integrated.

- [ ] **Step 3: Implement resumable dual deletion**

Track separate `TotalLogs`, `ProcessedLogs`, `RemainingLogs`, `TotalQuotaData`, `ProcessedQuotaData`, and `RemainingQuotaData` fields while retaining existing `Total`, `Processed`, `Remaining`, and `Progress` for old clients. Each worker pass deletes bounded batches from both tables, persists state after each batch, and finishes with `LogCleanupResult{DeletedCount: logCount, DeletedQuotaDataCount: quotaDataCount}`. Prune old cache buckets before the first aggregate deletion. Keep the existing ClickHouse behavior and strict cutoff predicate.

- [ ] **Step 4: Update frontend result typing and copy**

Add optional `deleted_quota_data_count` to `LogCleanupTaskResult`. Display both deletion counts in the completed-task toast and update the confirmation description to state that historical counters and dashboard statistics will be adjusted while balances and payment records remain unchanged. Add all new user-facing strings through the project i18n workflow for en/zh/fr/ja/ru/vi.

- [ ] **Step 5: Run focused tests and frontend checks**

Run `go test ./service ./model ./controller -run 'LogCleanup' -count=1`, `bun run typecheck` from `web/default`, and `git diff --check`.

- [ ] **Step 6: Commit**

```bash
git add model service web/default/src/features/system-settings
git commit -m "feat: bind log cleanup to usage statistics"
```

### Task 4: Full verification and handoff

**Files:**
- No new production files; inspect all task diffs.

- [ ] **Step 1: Run the complete relevant test suite**

Run `go test ./model ./service ./controller -count=1`, `bun run typecheck` from `web/default`, and `git diff --check`.

- [ ] **Step 2: Audit the safety boundary**

Confirm the diff contains no order, subscription, task, media, balance-quota, refund, price, channel configuration, Caddy, or environment changes. Confirm only historical usage counters are decremented and all updates are retry-idempotent.

- [ ] **Step 3: Commit verification notes**

Record the commands and outcomes in the final response, then use the finishing-a-development-branch workflow to present the branch integration options. Do not deploy production until separately authorized.
