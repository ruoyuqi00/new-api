# YuAPI Channel Pool Runtime - 2026-07-07

This note records the minimal Sub2API scheduler capability absorbed into
YuAPI/NewAPI for the one-service migration.

For the current live service status and rollback notes, read
`docs/YUAPI_PRODUCTION_STATUS_2026-07-09.md` first.

## Goal

Keep YuAPI as the only maintained API service while preserving the two Sub2API
pool behaviors that matter most under concurrency:

- per-channel in-flight request caps;
- short-lived transient cooldowns after provider overload/rate-limit failures.

This is intentionally not a Sub2API account-table port. It does not import
Sub2API Ent schemas, account state, provider adapter state, or production data.

## Runtime Fields

The feature is configured per YuAPI channel through the existing
`channels.settings` JSON (`dto.ChannelOtherSettings`). No schema migration is
required.

```json
{
  "channel_pool_concurrency_limit": 8,
  "channel_pool_cooldown_seconds": 20
}
```

Field behavior:

- `channel_pool_concurrency_limit <= 0`: disabled. This is the default.
- `channel_pool_cooldown_seconds <= 0`: disabled. This is the default.
- Both fields can be used independently.

## Selection Behavior

Channel selection still keeps YuAPI's existing priority and weight semantics:

- higher priority is selected first;
- retry moves through priority layers as before;
- weight controls traffic share inside a priority layer.

The new runtime only removes temporary bad candidates before the existing random
selection:

- channel is cooling down for `(group, model, channel_id)`;
- channel has reached `channel_pool_concurrency_limit`;
- current retry already skipped the channel after a race on slot acquisition.

If every high-priority channel is cooling/full, YuAPI can select the next
available priority layer without changing the database channel status.

## Concurrency

When Redis is enabled, the concurrency counter is Redis-backed:

- key namespace: `new-api:channel_pool:concurrency:v1`;
- lease TTL: 6 hours, to avoid leaked slots after a process crash;
- release happens after each upstream attempt, including stream completion,
  body-read failures, and async task submit failures.

When Redis is unavailable, the runtime falls back to process-local memory. That
protects a single YuAPI process but is not cross-replica protection.

Redis errors fail open and are logged; they do not block production traffic.

## Cooldown

Cooldown is scoped to:

```text
(selected_group, original_model, channel_id)
```

This prevents a transient `gpt-plus` overload from suppressing the same channel
for another product group.

Cooldown is set only when the channel has
`channel_pool_cooldown_seconds > 0` and the upstream failure is transient:

- HTTP 429;
- HTTP 529;
- ordinary 5xx except status codes explicitly marked as always-skip-retry;
- HTTP 409 / 425.

Auth/config failures such as channel errors are not cooled; existing auto-ban
rules still decide permanent channel/key disablement.

## Affinity Interaction

YuAPI channel affinity remains channel-level. With one account per channel, it
approximates Sub2API sticky account routing.

If an affinity-preferred channel is temporarily cooling or full:

- YuAPI falls back to normal selection for this request;
- the affinity cache is not cleared;
- future requests can return to the sticky channel after recovery.

If the channel is disabled, missing, or no longer supports the group/model/path,
the existing affinity-clear policy still applies.

## Migration Use

Recommended first-pass values for migrated one-account-per-channel pools:

| Pool type | Suggested limit | Suggested cooldown |
| --- | ---: | ---: |
| GPT text account | account's old Sub2API `concurrency` or lower | 10-30 seconds |
| Codex / subscription account | 1-3 until smoke proves stable | 10-30 seconds |
| Image generation | 1 unless provider explicitly supports more | 30-120 seconds |
| Homogeneous low-risk API-key channel | disabled or conservative | 10-20 seconds |

Do not use multi-key for stateful accounts that need account-level concurrency,
cooldown, or sticky behavior.

## Deployment Safety

This change is data-safe by default:

- no database migration;
- no production account import;
- no Sub2API DB reads/writes;
- no channel behavior change unless a channel's settings JSON opts in.

Before enabling on production channels:

1. Deploy code with both new fields unset.
2. Smoke normal chat/responses/image routes.
3. Enable the fields on one low-risk channel.
4. Confirm logs show fallback instead of permanent disable on 429/5xx.
5. Roll out per group, keeping one-account-per-channel for important pools.

Rollback is clearing the two JSON fields or redeploying the previous image.

## Production Deployment Record

2026-07-07 18:27 Asia/Shanghai:

- Server: `154.219.122.197`
- Existing production source copied from:
  `/opt/deploy-images/newapi-image-url-b64-src`
- Patched build source:
  `/opt/deploy-images/newapi-channel-pool-runtime-59688c50-20260707181527-src`
- Patch applied:
  `yuapi-channel-pool-runtime-59688c50.patch`
- Built image:
  `newapi:channel-pool-runtime-20260707-59688c50`
- Previous image:
  `newapi:image-url-b64-20260628`
- Compose backup:
  `/opt/newapi/backups/docker-compose-before-channel-pool-runtime-20260707182704.yml`
- Recreated service:
  `newapi` only.
- Data services left running:
  `newapi-mysql`, `newapi-redis`.
- Data paths left untouched:
  `/opt/newapi/mysql_data`, `/opt/newapi/redis_data`, `/opt/newapi/data`.

Verification:

```text
go test ./model ./service ./middleware ./controller
HTTP_ROOT=200
newapi newapi:channel-pool-runtime-20260707-59688c50 Up (healthy)
newapi-redis redis:7-alpine Up 2 weeks (healthy)
newapi-mysql mysql:8.4 Up 2 weeks (healthy)
```

Notes:

- The code is deployed, but the new runtime remains default-off until channel
  settings include `channel_pool_concurrency_limit` or
  `channel_pool_cooldown_seconds`.
- Sub2API was not stopped or modified in this deployment; account migration is
  still a separate, data-sensitive phase.
