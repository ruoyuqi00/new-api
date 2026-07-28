# YuAPI Production Status - 2026-07-09

This is the short current-state entry for the YuAPI/NewAPI production line after
the Sub2API plus/pro migration and conservative Sub2API app retirement.

It intentionally contains no API keys, passwords, OAuth tokens, or credential
payloads.

## Current Shape

- Public GPT plus/pro traffic is served by YuAPI/NewAPI.
- Sub2API is no longer the plus/pro runtime hop.
- The `sub2api` app container is stopped.
- Sub2API PostgreSQL, Redis, Caddy, config, and volumes are retained as cold
  reference and rollback material.
- `sub2api-caddy` must stay running for now because it still proxies YuAPI
  domains such as `api.dtrljm.com`.

## Repo State

```text
workspace: D:\wflogin\new-api-ruoyu-push
branch: feature/yuapi-channel-pool-runtime-20260707
remote branch: ruoyu/feature/yuapi-channel-pool-runtime-20260707
latest deployed code commit: 0809480bc fix: expose embedding endpoint metadata
```

Important remotes:

```text
origin = https://github.com/QuantumNous/new-api.git
ruoyu  = https://github.com/ruoyuqi00/sub2api-provider-adapters.git
```

Do not push local `main` to `ruoyu/main`. Local `main` tracks upstream
`QuantumNous/new-api` and is not the YuAPI production feature line.

## YuCore UI Boundary

YuCore UI / brand / Studio / Canvas work is not part of the current production
deployment stream. The unfinished local UI lint cleanup is intentionally kept in
stash `wip: phase 20 yucore motion canvas lint cleanup` and must not be applied
or deployed during backend protocol/strategy phases.

Current backend production phases may update YuAPI/NewAPI server code,
protocol routing metadata, tests, and docs. They must not mix in YuCore UI
polish until a separate UI window deliberately resumes that work.

## Server State

Observed on `154.219.122.197` after the Phase 21 deployment:

```text
newapi              newapi:channel-pool-runtime-20260710-0809480bc   healthy
newapi-mysql        mysql:8.4                                       healthy
newapi-redis        redis:7-alpine                                  healthy
sub2api             sub2api-provider-adapters:...                   Exited (0)
sub2api-caddy       caddy:2-alpine                                  running
sub2api-postgres    postgres:18-alpine                              healthy
sub2api-redis       redis:8-alpine                                  healthy
```

Post-stop smoke:

```text
api.dtrljm.com /: HTTP 200
plus token 82 / gpt-5.4-mini: HTTP 200, log id 7013, channel id 2323
pro token 80 / gpt-5.4-mini: HTTP 200, log id 7014, channel id 2308
```

Phase 21 deploy smoke on 2026-07-10:

```text
compose backup: /opt/newapi/backups/docker-compose-before-phase21-compact-20260710094446.yml
local /: HTTP 200
local /api/pricing: HTTP 200
local /api/status: HTTP 200
local unauth /v1/models: HTTP 401
domain https://api.dtrljm.com/: HTTP 200
/api/pricing compact metadata: gpt-5.5-openai-compact -> openai-response-compact
```

No production database data, account pool settings, channel priorities, MySQL
volumes, Redis volumes, or retained Sub2API data services were modified.

Phase 22 deploy smoke on 2026-07-10:

```text
compose backup: /opt/newapi/backups/docker-compose-before-phase22-embedding-20260710101008.yml
local /: HTTP 200
local /api/pricing: HTTP 200
local /api/status: HTTP 200
domain https://api.dtrljm.com/: HTTP 200
/api/pricing embedding metadata: 0 live embedding items currently enabled
/api/pricing compact metadata: gpt-5.5-openai-compact -> openai-response-compact
```

No production database data, account pool settings, channel priorities, MySQL
volumes, Redis volumes, or retained Sub2API data services were modified.

## Channel State

Key YuAPI channels:

| Channel | Role | Status | Priority | Note |
| ---: | --- | ---: | ---: | --- |
| 2294 | old Sub2API plus bridge | 2 | 110 | disabled, not deleted |
| 2295 | old Sub2API pro bridge | 2 | 120 | disabled, not deleted |
| 2308 | existing pro direct duplicate | 1 | 100 | smoke passed |
| 2322 | migrated plus direct | 1 | 120 | primary |
| 2323 | migrated plus direct | 1 | 120 | primary |
| 2324 | migrated plus direct | 1 | 80 | fallback after load-test long tail |
| 2326 | migrated pro direct | 2 | 125 | disabled after upstream 403 group-access smoke failure |
| 2328 | migrated pro direct | 1 | 125 | smoke passed |
| 2329 | migrated pro direct | 1 | 125 | smoke passed |

The default YuAPI per-user concurrency limiter is back on its default behavior.
No temporary `UserConcurrencyLimit=80` override remains.

## Backups And Rollback

Important backup paths:

```text
/opt/newapi/backups/docker-compose-before-channel-pool-runtime-20260707182704.yml
/opt/migration-backups/yuapi-sub2api-20260707185017
/opt/migration-backups/yuapi-sub2api-retire-20260709165441
```

Rollback if the stopped Sub2API app is unexpectedly needed:

```bash
cd /opt/sub2api
docker compose start sub2api
```

Rollback for the plus/pro import is documented in:

```text
/opt/migration-backups/yuapi-sub2api-20260707185017/rollback_newapi_sub2_openai_migration.sql
```

Do not run rollback SQL unless YuAPI plus/pro direct routing actually fails.
The bridge channels are disabled but preserved.

## Do Not Remove Yet

- Do not remove `sub2api-postgres`.
- Do not remove `sub2api-redis`.
- Do not remove `/opt/sub2api` data/config directories.
- Do not stop `sub2api-caddy` until YuAPI domain proxying is moved elsewhere.
- Do not delete bridge channels `2294` or `2295` until the migration has passed
  a longer soak and rollback is no longer desired.

## Current Follow-Up Queue

1. Keep YuAPI-only plus/pro operation under observation.
2. Selectively backport important upstream `QuantumNous/new-api` fixes; do not
   wholesale merge `origin/main`.
3. Harden protocol capability surfaces before changing live strategy:
   - Responses/compact Responses endpoint metadata;
   - channel-test endpoint selection for Codex/subscription-style channels;
   - request-path support checks for advanced custom channels.
4. Harden YuAPI scheduling observability:
   - channel-pool full/cooldown counters;
   - clear admin log surface for skipped cooled/full channels;
   - safer per-channel fallback diagnostics.
5. Review billing/price edge cases before changing model exposure or pricing.
6. Review remaining non-plus/pro paths separately:
   - image routes;
   - Kiro/Windsurf/provider adapters;
   - CC/Anthropic-style pools;
   - admin-only media tooling.

## Related Documents

- `BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`
- `docs/YUAPI_CHANNEL_POOL_RUNTIME_2026-07-07.md`
- `docs/YUAPI_SUB2API_MINIMAL_MIGRATION_DRY_RUN_2026-07-07.md`
- `docs/YUAPI_UPSTREAM_NEWAPI_AUDIT_2026-07-09.md`
- `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`
