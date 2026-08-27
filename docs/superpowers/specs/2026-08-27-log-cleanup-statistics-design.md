# Log Cleanup Statistics Design

## Goal

When an administrator cleans historical logs before a selected timestamp, remove the matching log records and dashboard usage aggregates together, while leaving billing balances and all non-log data unchanged.

## Scope

The cleanup scope is the historical log data and the counters derived from it:

- `logs`: every log type before the cutoff, including consume, top-up, refund, login, management, system, and error records.
- `quota_data`: dashboard aggregation rows before the same cutoff.
- `users.used_quota` and `users.request_count`: subtract the sum/count of deleted
  consume logs per user.
- `tokens.used_quota` and `channels.used_quota`: subtract the quota sum of deleted
  consume logs for each token/channel referenced by those logs.

The cleanup does not modify user/token/channel balances (`quota`, `remain_quota`),
orders, subscriptions, media/canvas data, options, model prices, or any other
fields. It does not refund or debit money. It does update only the three
historical usage counters listed above, and it does not delete payment orders or
subscription orders; their payment state is not a log record.

## Architecture

The existing asynchronous `log_cleanup` system task remains the single entry point.
At the start of a task, it snapshots grouped consume-log deltas for users, tokens,
and channels. The snapshot is persisted in task state. A main-database transaction
applies those deltas and marks the snapshot applied before any log rows are
deleted; a retry therefore never applies the same correction twice. Each worker
pass then deletes bounded batches from both `logs` and `quota_data` using the same
`created_at < target_timestamp` predicate. The task state records separate log and
aggregate counts while keeping the existing progress fields for compatibility.
The in-memory `CacheQuotaData` entries older than the cutoff are removed before
database aggregate deletion so a later flush cannot recreate rows that were just
removed.

Database operations use GORM and the existing ClickHouse branch. For regular databases, each batch uses GORM `Delete`; for ClickHouse, each table uses a synchronous mutation with a count taken before deletion. No database-specific SQL is introduced for SQLite, MySQL, or PostgreSQL.

## Data Flow

1. The admin selects a cutoff timestamp and starts the existing cleanup task.
2. Pending asynchronous usage-counter updates are flushed before the historical snapshot, so counters and consume logs share the same boundary.
3. The task counts old rows in both tables and initializes separate totals.
4. The task applies the persisted user/token/channel counter deltas in one main-DB transaction and marks them applied.
5. Each pass removes up to the configured batch size from `logs` and `quota_data`, updates processed/remaining counts, and persists task state.
6. Before aggregate deletion, the process removes cached aggregate entries whose bucket timestamp is before the cutoff. Entries at or after the cutoff stay available for the normal periodic flush.
7. The task finishes with `{deleted_count, deleted_quota_data_count}`. Existing `deleted_count` remains the log count for old clients.
8. The UI reports both counts and explains that balances are not changed; historical usage counters now reflect only retained logs.

## Failure and Recovery

- A failure in accounting or either table stops the task before it is marked successful; the existing system-task retry/inspection path remains usable.
- A task may resume after a lease loss because deletion is cutoff-based and idempotent.
- Counter correction is committed before deletion and guarded by the durable task-state marker; a crash between correction and deletion resumes without double subtraction.
- Deleting logs never calls refund, settlement, balance quota update, or order mutation code.
- Cache pruning is protected by the existing cache mutex. If the database deletion fails, cache entries remain removed only for old buckets; this is safe because those entries are outside the selected retention window and cannot be persisted as current statistics.

## Compatibility

The public cleanup endpoint and task type remain unchanged. The result adds an optional `deleted_quota_data_count` field; old clients can continue reading `deleted_count`. The cutoff is interpreted identically by both tables as Unix seconds with a strict `<` comparison.

## Verification

Regression tests must prove:

- all log types before the cutoff are deleted;
- old `quota_data` rows are deleted and rows at/after the cutoff remain;
- cache entries before the cutoff are not flushed back;
- user/token/channel balances remain unchanged while their historical `used_quota` and user request count are reduced by the deleted consume rows;
- current orders, subscriptions, tasks, and media rows remain unchanged;
- the task reports both deletion counts and resumes safely when one batch is already empty;
- SQLite, MySQL, PostgreSQL-compatible GORM paths use the existing portable operations.
