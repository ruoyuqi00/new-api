# Log Cleanup Statistics Design

## Goal

When an administrator cleans historical logs before a selected timestamp, remove the matching log records and dashboard usage aggregates together, while leaving billing balances and all non-log data unchanged.

## Scope

The cleanup scope is exactly these two storage layers:

- `logs`: every log type before the cutoff, including consume, top-up, refund, login, management, system, and error records.
- `quota_data`: dashboard aggregation rows before the same cutoff.

The cleanup does not modify `users`, `tokens`, `channels`, orders, subscriptions, tasks, media/canvas data, options, model prices, or any other table. It does not refund, debit, or rewrite `quota`, `used_quota`, `request_count`, `remain_quota`, or `used_quota` fields. It does not delete payment orders or subscription orders; their payment state is not a log record.

## Architecture

The existing asynchronous `log_cleanup` system task remains the single entry point. Each worker pass deletes bounded batches from both `logs` and `quota_data` using the same `created_at < target_timestamp` predicate. The task state records separate log and aggregate counts while keeping the existing progress fields for compatibility. The in-memory `CacheQuotaData` entries older than the cutoff are removed before database aggregate deletion so a later flush cannot recreate rows that were just removed.

Database operations use GORM and the existing ClickHouse branch. For regular databases, each batch uses GORM `Delete`; for ClickHouse, each table uses a synchronous mutation with a count taken before deletion. No database-specific SQL is introduced for SQLite, MySQL, or PostgreSQL.

## Data Flow

1. The admin selects a cutoff timestamp and starts the existing cleanup task.
2. The task counts old rows in both tables and initializes separate totals.
3. Each pass removes up to the configured batch size from `logs` and `quota_data`, updates processed/remaining counts, and persists task state.
4. Before aggregate deletion, the process removes cached aggregate entries whose bucket timestamp is before the cutoff. Entries at or after the cutoff stay available for the normal periodic flush.
5. The task finishes with `{deleted_count, deleted_quota_data_count}`. Existing `deleted_count` remains the log count for old clients.
6. The UI reports both counts and explains that balances and billing totals are unchanged.

## Failure and Recovery

- A failure in either table stops the task before it is marked successful; the existing system-task retry/inspection path remains usable.
- A task may resume after a lease loss because deletion is cutoff-based and idempotent.
- Deleting logs never calls refund, settlement, quota update, or order mutation code.
- Cache pruning is protected by the existing cache mutex. If the database deletion fails, cache entries remain removed only for old buckets; this is safe because those entries are outside the selected retention window and cannot be persisted as current statistics.

## Compatibility

The public cleanup endpoint and task type remain unchanged. The result adds an optional `deleted_quota_data_count` field; old clients can continue reading `deleted_count`. The cutoff is interpreted identically by both tables as Unix seconds with a strict `<` comparison.

## Verification

Regression tests must prove:

- all log types before the cutoff are deleted;
- old `quota_data` rows are deleted and rows at/after the cutoff remain;
- cache entries before the cutoff are not flushed back;
- user balance, `used_quota`, and request count remain byte-for-byte unchanged;
- current orders, subscriptions, tasks, and media rows remain unchanged;
- the task reports both deletion counts and resumes safely when one batch is already empty;
- SQLite, MySQL, PostgreSQL-compatible GORM paths use the existing portable operations.
