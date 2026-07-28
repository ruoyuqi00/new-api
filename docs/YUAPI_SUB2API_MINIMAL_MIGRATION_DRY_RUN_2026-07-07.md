# YuAPI / Sub2API Minimal Migration Dry Run - 2026-07-07

This document is the non-invasive migration worksheet for consolidating the
production stack so only YuAPI/NewAPI remains as the public service, database,
and maintenance target.

It intentionally does not contain real API keys, OAuth refresh tokens,
passwords, account payloads, or production database credentials.

Current short entry after the 2026-07-09 Sub2API app retirement:

- `docs/YUAPI_PRODUCTION_STATUS_2026-07-09.md`
- `docs/README.md`

## Scope

Target shape:

- One public API/admin service: YuAPI/NewAPI.
- One YuAPI database and one YuAPI Redis.
- No hidden runtime hop from YuAPI to Sub2API after cutover.
- Sub2API remains only as historical reference and patch source.

Out of scope for the first pass:

- Importing Sub2API Ent tables into YuAPI.
- Porting the full Sub2API account scheduler.
- Migrating provider adapter runtimes before their route, model, and account
  health behavior is proven in YuAPI.
- Treating a successful single request as account-pool equivalence.

## Source Material Read

- `BASELINE_PROJECT_REMOTE_PRODUCTION_2026-07-07.md`
- `sub2api-private/docs/CHANNEL_GROUP_USER_MAPPING_RUNBOOK_2026-06-17.md`
- `sub2api-private/planning/PROVIDER_ACCOUNT_OPERATIONS_GUIDE.md`
- `sub2api-private/deploy/docker-compose.yml`
- `sub2api-private/deploy/config.example.yaml`
- YuAPI channel selection, retry, affinity, Codex, and image relay code.
- Sub2API account, gateway scheduler, failover, concurrency, group, and account
  schema code.

## Hard Guardrails

1. Do not merge YuCore/new-api snapshot UI work into the production feature line
   as part of this migration.
2. Do not keep Sub2API as a private upstream behind YuAPI in the final design.
3. Do not migrate secrets through Markdown, git patches, shell history, or
   chat logs.
4. Do not collapse stateful OAuth/account pools into one YuAPI multi-key
   channel.
5. Do not auto-disable accounts/channels on transient pool errors such as 429,
   529, ordinary 5xx, temporary empty response, or provider overload.
6. Do not expose image/video models until their exact media endpoint has passed
   smoke. Text-model success is not media success.

## Known Production Pools From Docs

| Source area | Current known boundary | Target YuAPI boundary | First-pass decision | Risk |
| --- | --- | --- | --- | --- |
| GPT Team | `gpt-team` Sub2API group via `newapi-bridge-gpt-team` | YuAPI group `gpt-team`, direct YuAPI channels | Migrate first | Medium |
| GPT Plus | `gpt-plus` Sub2API group via `newapi-bridge-gpt-plus` | YuAPI group `gpt-plus`, direct YuAPI channels | Migrate first | Medium |
| GPT Pro | `gpt-pro` Sub2API group via `newapi-bridge-gpt-pro` | YuAPI group `gpt-pro`, direct YuAPI channels | Migrate first | Medium |
| GPT Image2 | `gpt-image2-newapi`, only proven visible image model `gpt-image-2` | YuAPI image-capable channel(s), `gpt-image-2` only | Migrate separately with single-upstream-attempt policy | High |
| Provider mixed | `provider-mixed`, exclusive, contains Windsurf/Kiro internal accounts with model routing | Do not flatten into GPT groups | Defer until adapter path is designed | High |
| Kiro | `kiro-gateway-internal-anthropic` inside `provider-mixed` | Future YuAPI channel or dedicated adapter bridge | Defer | High |
| Windsurf | `windsurf-internal-anthropic` inside `provider-mixed` | Future YuAPI channel or dedicated adapter bridge | Defer | High |

Account counts, exact account IDs, key status, quota windows, and current
runtime errors are not present in this workspace. They must be pulled from a
read-only DB inventory before generating import data.

## Account Classification Rules

| Class | Examples | YuAPI representation | Notes |
| --- | --- | --- | --- |
| A. Plain API-key, homogeneous, low-risk | OpenAI-compatible API keys with same base URL, same model scope, same billing behavior | One YuAPI multi-key channel is allowed | Only use when per-key concurrency/cooldown/sticky is not important. |
| B. Plain API-key, production-important | Tiered GPT pools, account-specific base URL/proxy/model mapping, account-specific quota | One YuAPI channel per account/key | Better logs, targeted disable, weighted routing, safer rollback. |
| C. OpenAI/Codex OAuth/subscription | Codex/ChatGPT subscription accounts | One YuAPI Codex channel per account | Keep channel affinity and Codex credential refresh enabled. |
| D. Image generation | `gpt-image-2` and future real image endpoints | Dedicated YuAPI image channel(s) | Avoid duplicate upstream billing; do not retry image generation blindly. |
| E. Provider adapters | Kiro, Windsurf, Antigravity, Claude-like internal adapters | Defer or build a dedicated YuAPI adapter path | Not a phase-1 bulk migration target. |

## Scheduler Equivalence Matrix

| Behavior | Sub2API behavior | YuAPI current behavior | Migration action |
| --- | --- | --- | --- |
| Group boundary | API key group selects channel/account pool | Token group selects channels/abilities | Preserve group names where possible. |
| Model mapping | Channel and account mapping can both affect final upstream model | Channel model mapping is available | Export both mapping layers; fold only proven mappings into YuAPI channel mapping. |
| Priority direction | Lower number is higher priority | Higher number is higher priority | Reverse during conversion. Do not copy raw priority. |
| Weighted routing | Priority, load, LRU, optional reset window | Priority layer plus weight random | Use one-channel-per-account for important pools; set explicit priority/weight. |
| Per-account concurrency | Redis account slots and wait plans | No equivalent per-channel account slot | Do not assume equivalence. Optional later patch: YuAPI channel concurrency gate. |
| Load awareness | Batch load rate, waiting count, LRU | No per-channel load score | Accept for low-risk API-key pools only. High-risk pools stay deferred. |
| Temporary cooldown | Rate-limit reset, overload, temp unschedulable, selection cooldown | Auto-disable or retry, no time-based channel cooldown | Configure not to auto-ban transient errors. Optional later patch: channel/model cooldown cache. |
| Sticky session | Account binding by session, with health checks and clear rules | Channel affinity rules by headers/body/context | Enable affinity for Codex/CLI style traffic. Be strict where session continuity matters. |
| Same-account retry | Retry same account for selected transient errors before switching | Retry generally selects again by channel priority/weight | Do not rely on same-account semantics in phase 1. |
| OAuth refresh | Account credential lifecycle and refresh paths | YuAPI has Codex refresh support | Codex can migrate earlier; other OAuth providers need case-by-case treatment. |
| Image billing safety | Image-specific single-attempt/wait behavior documented in UAG/Sub2API docs | YuAPI has image empty-response and b64 fallback guards | Keep `gpt-image-2` isolated and disable broad retry for paid image work. |

## Target YuAPI Settings To Verify

Before importing channels, verify these YuAPI options in admin settings or DB:

- `RetryTimes`: enough to try intended fallback priority layers, but not so high
  that image or sticky workloads duplicate expensive upstream work.
- `AutomaticDisableChannelEnabled`: enabled only with conservative status rules.
- `AutomaticDisableStatusCodes`: keep to hard-auth failures such as `401` unless
  a specific provider is known safe to ban.
- `AutomaticRetryStatusCodes`: allow transient retry, but review `400`, `429`,
  `5xx`, and image routes separately.
- `channel_affinity_setting`: keep Codex/CLI sticky rules; decide per rule
  whether `skip_retry_on_failure` should be strict.
- Group ratios, model ratios, image ratios, and per-call image prices.
- Error log recording and admin visibility for multi-key index/channel ID.

## Dry-Run Inventory Queries

Run these read-only queries against the Sub2API production database from a safe
maintenance shell. Do not copy credential JSON or key values into the result.

### Groups

```sql
select
  id,
  name,
  platform,
  status,
  is_exclusive,
  rate_multiplier,
  allow_image_generation,
  image_rate_independent,
  image_rate_multiplier,
  model_routing_enabled,
  supported_model_scopes,
  require_oauth_only,
  require_privacy_set,
  rpm_limit,
  deleted_at
from groups
where deleted_at is null
order by name;
```

### Account Pool Summary

```sql
select
  a.id,
  a.name,
  a.platform,
  a.type,
  a.status,
  a.schedulable,
  a.concurrency,
  a.load_factor,
  a.priority,
  a.rate_multiplier,
  a.auto_pause_on_expired,
  a.expires_at,
  a.rate_limit_reset_at,
  a.overload_until,
  a.temp_unschedulable_until,
  a.session_window_end,
  count(ag.group_id) as group_count
from accounts a
left join account_groups ag on ag.account_id = a.id
where a.deleted_at is null
group by a.id
order by a.platform, a.priority, a.id;
```

### Group To Account Binding

```sql
select
  g.id as group_id,
  g.name as group_name,
  a.id as account_id,
  a.name as account_name,
  a.platform,
  a.type,
  a.status,
  a.schedulable,
  a.concurrency,
  a.priority as account_priority,
  ag.priority as binding_priority
from account_groups ag
join groups g on g.id = ag.group_id and g.deleted_at is null
join accounts a on a.id = ag.account_id and a.deleted_at is null
order by g.name, a.priority, a.id;
```

### Channel / Model Mapping Summary

```sql
select
  c.id,
  c.name,
  c.platform,
  c.status,
  c.restrict_models,
  c.billing_model_source,
  c.model_mapping,
  c.features_config
from channels c
where c.deleted_at is null
order by c.id;
```

### Bridge Keys To Retire

```sql
select
  id,
  name,
  group_id,
  status,
  expires_at,
  created_at,
  last_used_at
from api_keys
where deleted_at is null
  and (
    name like 'newapi-bridge-%'
    or name like 'server-provider-%'
  )
order by name;
```

## Conversion Rules

### Groups

Sub2API group names should become YuAPI groups when they are user-facing product
tiers:

- `gpt-team` -> YuAPI `gpt-team`
- `gpt-plus` -> YuAPI `gpt-plus`
- `gpt-pro` -> YuAPI `gpt-pro`

Internal bridge groups should not become public products unless they already
represent a user-facing tier.

### Accounts To Channels

For each Sub2API account selected for phase 1:

| Sub2API field | YuAPI target | Rule |
| --- | --- | --- |
| `platform/type` | channel type | Map only if YuAPI supports that provider path. |
| credential key | `channels.key` | Store one account per channel unless Class A multi-key is approved. |
| group binding | `channels.group` and abilities | Use comma group list only when the same account truly serves multiple YuAPI groups. |
| model support | `channels.models` | Use explicit list; avoid catch-all for paid media. |
| account model mapping | `channels.model_mapping` | Merge with channel mapping after smoke. |
| `priority` | `channels.priority` | Reverse direction. Example: YuAPI priority = 100000 - Sub2API priority. |
| load/concurrency | no native target | Document as lost behavior; do not use multi-key for these accounts. |
| `rate_multiplier` | pricing/group ratio | Preserve only after checking billing semantics. |
| proxy | channel setting/proxy if supported | Do not silently drop proxy-bound accounts. |

### Multi-Key Policy

Use multi-key only when all keys share:

- Same provider type.
- Same base URL and proxy behavior.
- Same model whitelist.
- Same billing multiplier.
- No account-specific OAuth refresh.
- No need for per-account cooldown, concurrency, or sticky routing.

Otherwise create one YuAPI channel per source account.

### Priority And Weight

Use explicit priority tiers:

| Source intent | Example Sub2API priority | Suggested YuAPI priority |
| --- | ---: | ---: |
| Primary | 0-20 | 1000 |
| Secondary | 21-50 | 500 |
| Emergency fallback | 51+ | 100 |

Within the same priority tier, use `weight` only for intentional traffic share.
If preserving Sub2API LRU/load behavior matters, prefer one channel per account
and keep weights low/equal until production smoke confirms distribution.

## Phase Plan

### Phase 0 - Inventory Only

- Run the read-only inventory queries.
- Classify each account as A/B/C/D/E.
- Mark accounts with active cooldown, overload, temp unschedulable, expired
  credentials, missing privacy setting, or provider adapter dependency.
- Produce a redacted CSV with only IDs, names, groups, provider type, model
  scope, priority, status, and risk class.

Exit criteria:

- No secret material in the inventory artifact.
- Every user-facing group has a target YuAPI group or an explicit defer note.

### Phase 1 - GPT Text Pools

- Migrate `gpt-team`, `gpt-plus`, `gpt-pro`.
- Prefer one YuAPI channel per production-important source account.
- Use multi-key only for homogeneous low-risk API-key sources.
- Configure retry/disable conservatively.
- Smoke `/v1/chat/completions` and `/v1/responses` where applicable.

Exit criteria:

- Each tier has at least one enabled YuAPI channel for required models.
- Failure of one channel/key does not disable an unrelated tier.
- Usage logs show expected group, channel ID, model, and billing.

### Phase 2 - Codex / OpenAI Subscription

- Migrate Codex-capable accounts one channel per account.
- Ensure real `account_id`, refresh token, expiry, and Codex channel type.
- Keep channel affinity enabled for Codex CLI trace/prompt-cache keys.
- Smoke `/v1/responses` and `/v1/responses/compact`.

Exit criteria:

- Credential refresh succeeds.
- Sticky behavior is visible in affinity logs.
- `responses/compact` uses the intended upstream model.

### Phase 3 - GPT Image2

- Keep only `gpt-image-2` visible until other image/video routes pass real media
  smoke.
- Use `/v1/images/generations` and `/v1/images/edits`.
- Avoid automatic retry that can duplicate upstream image billing.
- Confirm empty image responses are rejected.

Exit criteria:

- Text-to-image and image-to-image both succeed with real output.
- Failed image responses are not billed as success.
- Grok/Gemini/Veo remain hidden unless their exact media endpoint passes smoke.

### Phase 4 - Provider Mixed / Kiro / Windsurf

- Do not flatten `provider-mixed` into GPT groups.
- First decide whether YuAPI will receive a dedicated adapter channel type,
  an advanced custom route, or a small provider adapter module.
- Preserve model routing semantics so overlapping Kiro/Windsurf model names do
  not randomly hit the wrong upstream.

Exit criteria:

- Route preview or equivalent dry run exists before exposure.
- Smoke passes for each public alias.
- Sticky/cache reuse requirements are documented per provider.

## Minimal Runtime Patch Status

2026-07-07 update: the lightweight YuAPI channel-pool runtime has been added
behind per-channel settings. It is default-off and requires no schema migration.

Implemented:

- Redis-backed channel concurrency gate with process-local fallback.
- `(group, model, channel_id)` transient cooldown cache.
- Selection skip for cooled/full channels while preserving YuAPI priority and
  weight behavior.
- Affinity fallback that keeps sticky cache when the preferred channel is only
  temporarily cooled/full.

Configuration lives in `channels.settings` JSON:

```json
{
  "channel_pool_concurrency_limit": 8,
  "channel_pool_cooldown_seconds": 20
}
```

See `docs/YUAPI_CHANNEL_POOL_RUNTIME_2026-07-07.md` for deployment details.

## Optional Minimal Patches After Phase 1

Only consider these if dry-run or smoke shows the behavior is required:

1. Channel/model cooldown cache in YuAPI. Implemented default-off.
   - Key: `(group, model, channel_id)` or `(model, channel_id)`.
   - Set on configured transient status/keyword.
   - Selection skips cooled channels without changing DB status.
   - No schema migration required if Redis-backed.

2. Channel concurrency gate in YuAPI. Implemented default-off.
   - Redis counter by channel ID.
   - Optional per-channel setting stored in existing channel settings JSON.
   - Use only for accounts that previously depended on Sub2API concurrency.

3. Import preview script.
   - Reads redacted Sub2API inventory CSV.
   - Emits YuAPI channel/group/ability import plan.
   - Does not write DB until explicitly approved.

Do not start with these patches. Start with inventory and classification.

## Smoke Matrix

| Area | Request | Required evidence |
| --- | --- | --- |
| GPT text | `/v1/chat/completions` non-stream | HTTP 200, expected channel/group log. |
| GPT text stream | `/v1/chat/completions` stream | Complete stream, post-consume billing correct. |
| Responses | `/v1/responses` | Model preserved/mapped as intended. |
| Codex compact | `/v1/responses/compact` | Expected Codex/OpenAI channel, no unsupported endpoint. |
| Affinity | Repeat same sticky key | Same channel until failure policy says otherwise. |
| Bad key | Force one bad key/channel | Only that key/channel is disabled or skipped. |
| 429/529 | Simulated transient upstream error | Retried or cooled, not permanently banned. |
| Image generation | `/v1/images/generations` | Real `url` or `b64_json`, no false success. |
| Image edit | `/v1/images/edits` | Real output and one upstream attempt unless manually retried. |

## Rollback

Before cutover:

- Keep old Sub2API deployment stopped but not destroyed.
- Keep PostgreSQL and Redis volumes intact until YuAPI has passed live smoke.
- Keep DNS/proxy rollback path documented.
- Preserve old NewAPI bridge tokens disabled, not deleted, until billing and
  logs are verified.

Rollback trigger examples:

- More than one user-facing GPT tier has no usable YuAPI channel.
- 429/529 starts causing broad permanent channel bans.
- Codex refresh fails for migrated accounts.
- Image route bills failed/empty outputs as success.
- Provider-mixed traffic routes to the wrong adapter.

## Current Recommendation

Proceed with Phase 0 inventory and Phase 1 GPT text pools first. Keep
`provider-mixed`, Kiro, Windsurf, and other adapter-heavy pools out of the first
cutover. Treat `gpt-image-2` as a separate media migration with stricter retry
and billing checks.

## Production Migration Record

2026-07-07 18:50-19:00 Asia/Shanghai:

- Server: `154.219.122.197`
- Backup directory:
  `/opt/migration-backups/yuapi-sub2api-20260707185017`
- Backups:
  - `newapi.sql.gz`
  - `sub2api.sql.gz`
  - `SHA256SUMS`
- Import SQL:
  `/opt/migration-backups/yuapi-sub2api-20260707185017/newapi_import_sub2_openai.sql`
- Redacted preview:
  `/opt/migration-backups/yuapi-sub2api-20260707185017/newapi_import_sub2_openai_preview.tsv`
- Rollback SQL:
  `/opt/migration-backups/yuapi-sub2api-20260707185017/rollback_newapi_sub2_openai_migration.sql`

Applied scope:

- Migrated Sub2API OpenAI `apikey` accounts for:
  - `gpt-plus`
  - `gpt-pro` -> existing YuAPI group `gpt-pro原价版`
- Skipped Anthropic / CC / Kiro-style pools.
- Skipped one duplicate key already present in YuAPI.
- Did not migrate `gpt-team`, because the Sub2API group had no account binding
  in the production inventory.
- Did not migrate image routes in this pass.

Result:

```text
migrated_channels status=1 count=6
migrated_channels status=2 count=5
migrated_abilities enabled=1 count=34
migrated_abilities enabled=0 count=28
bridge channel 2294 sub2api-gpt-plus status=2
bridge channel 2295 sub2api-gpt-pro status=2
newapi HTTP / = 200
newapi container = healthy
```

Upstream smoke:

```text
gpt-plus / gpt-5.4-mini HTTP 200
  log id 6499, token id 74, channel id 2323
  channel: sub2 gpt-plus #7930 plus (0.08) https://mdkj.lol
  tag: sub2-account-7930

gpt-pro原价版 / gpt-5.4-mini HTTP 200
  log id 6500, token id 88, channel id 2308
  channel: https://mdkj.lol/ 0.1

bridge 2294 sub2api-gpt-plus status=2
bridge 2295 sub2api-gpt-pro status=2
```

Safety notes:

- Sub2API data was not deleted or modified.
- NewAPI bridge channels were disabled, not deleted.
- Imported YuAPI channels are tagged `sub2-account-<id>`.
- The rollback SQL restores the plus/pro bridge and disables imported direct
  channels without deleting them.

## Post-Migration Channel Audit

2026-07-07 follow-up:

- Sub2API `gpt-plus` OpenAI/apikey inventory contained 6 accounts; all 6 were
  imported as YuAPI direct channels.
- Sub2API `gpt-pro` OpenAI/apikey inventory contained 6 accounts; 5 were
  imported as YuAPI direct channels and 1 duplicate was already present as
  YuAPI channel `2308`.
- Imported enabled/disabled state initially mirrored Sub2API `schedulable`:
  schedulable accounts became YuAPI `status=1`, non-schedulable accounts became
  YuAPI `status=2`.
- Channel `2326` (`sub2-account-7922`) was later disabled after direct smoke
  returned upstream `403` group-access errors across multiple supported text
  models. Its `abilities` rows were disabled as well.
- Sub2API data remained untouched. Bridge channels `2294` and `2295` remain
  disabled, not deleted.

Current imported channel state after the `2326` safety disable:

```text
imported_channels status=1 count=5
imported_channels status=2 count=6
imported_abilities enabled=1 count=28
imported_abilities enabled=0 count=34
newapi container = healthy
```

Per-channel smoke:

```text
gpt-plus enabled migrated channels:
  2322 sub2-account-7929 walkcoding.top       HTTP 200
  2323 sub2-account-7930 mdkj.lol             HTTP 200
  2324 sub2-account-7935 ppsubapi.com         HTTP 200

gpt-pro enabled migrated/direct channels:
  2326 sub2-account-7922 zz1cc.cc.cd          HTTP 403, disabled after smoke
  2328 sub2-account-7928 walkcoding.top       HTTP 200
  2329 sub2-account-7934 ppsubapi.com         HTTP 200
  2308 existing duplicate mdkj.lol            HTTP 200

normal user-token smoke after disabling 2326:
  plus token 74 / gpt-5.4-mini HTTP 200, log id 6507, channel id 2322
  pro token 88 / gpt-5.4-mini  HTTP 200, log id 6508, channel id 2318

forced disabled-channel check:
  channel 2326 HTTP 403, "channel disabled"
```

## Plus / Pro Load Smoke

2026-07-07 follow-up load smoke:

- The default YuAPI user concurrency limiter is active. With the default
  `UserConcurrencyLimit=5`, single-token tests above 5 concurrent requests
  returned expected `429 concurrent request limit reached` responses. This is
  an ingress/user protection layer, not a channel-pool failure.
- To test backend pool behavior, `UserConcurrencyLimit` was temporarily set to
  `80`, YuAPI was restarted, load smoke was run, then the temporary option was
  deleted and YuAPI was restarted again. The database has no remaining explicit
  `UserConcurrencyLimit` override and the service is back on the default limit.

Backend pool smoke with the limiter temporarily raised:

```text
plus / gpt-5.4-mini / concurrency 20:
  19/20 HTTP 200, 1 client timeout
  channels: 2322=5, 2323=8, 2324=6
  note: 2324 showed long-tail latency under load.

plus / gpt-5.4-mini / concurrency 50:
  48/50 HTTP 200, 2 client timeouts
  channels: 2322=21, 2323=20, 2324=7
  note: timeouts correlated with slow 2324 traffic.

pro / gpt-5.4-mini / concurrency 20:
  20/20 HTTP 200
  channels: 2306=6, 2308=7, 2311=2, 2318=5

pro / gpt-5.4-mini / concurrency 50:
  50/50 HTTP 200
  channels: 2306=14, 2308=9, 2311=7, 2318=20
```

Action taken after the plus long-tail result:

- Channel `2324` (`sub2-account-7935`, `ppsubapi.com`) stayed enabled but was
  moved from primary priority `120` to fallback priority `80`.
- Its key, status, concurrency setting, and Sub2API source data were not
  modified.

Retest after moving `2324` to fallback:

```text
plus / gpt-5.4-mini / concurrency 50:
  50/50 HTTP 200
  p50=3269ms, p95=5559ms, max=32048ms
  channels: 2322=22, 2323=28
  error samples: none
```

## Observation Window

2026-07-08 14:32 Asia/Shanghai check:

- `newapi` was up for 19 hours and healthy on
  `newapi:channel-pool-runtime-20260707-59688c50`.
- `newapi-mysql` and `newapi-redis` remained healthy.
- Sub2API bridge channels `2294` and `2295` remained disabled.
- Channel `2324` remained enabled as fallback priority `80`.
- Channel `2326` remained disabled after the upstream 403 smoke failure.
- No explicit `UserConcurrencyLimit` override remained in the options table;
  YuAPI was back on the default per-user concurrency limit.
- Container logs for the last 24 hours had no matching panic, fatal, database,
  Redis, or channel-pool runtime errors.

Logs after the plus retest ended at log id `6724`:

```text
all logs after 6724:
  type=2 consume records: 140
  type=3 admin/manage records: 3
  type=7 login records: 1

real plus traffic:
  token 82, channel 2323, 83 consume records
  avg use_time=13.02s, max use_time=53s
  no non-consume error records

real pro traffic:
  token 80, channel 2306, 7 consume records
  token 80, channel 2318, 2 consume records
  token 88, channel 2306, 1 consume record
  no non-consume error records

system/admin channel-test style records:
  token_id=0 generated 47 pro-group consume records and was excluded from the
  real-user traffic read.
```

Observation summary:

- Plus real traffic used `2323` and stayed clean.
- `2324` was not selected for real plus traffic after being moved to fallback.
- Pro real traffic stayed on the existing YuAPI pro pool and stayed clean.
- No evidence appeared that Sub2API bridge fallback was needed during the
  observation window.

## Conservative Sub2API App Retirement

2026-07-09 16:54 Asia/Shanghai:

- `newapi` had been healthy for about 45 hours on
  `newapi:channel-pool-runtime-20260707-59688c50`.
- NewAPI bridge channels `2294` and `2295` remained disabled and had no new
  hits after the migration cutover.
- YuAPI plus/pro real traffic after the previous observation window remained
  clean:
  - plus token `82` used channel `2322` for 69 successful consume records.
  - pro token `80` used channel `2308` for 19 successful consume records.
  - no plus/pro non-consume error records were found.
- Sub2API `usage_logs` had no records since `2026-07-01`; its latest usage row
  was from `2026-06-28`.
- Sub2API public logs still showed health checks, unauthorized probes, and web
  scans, but no successful billed Sub2API usage.

Action taken:

- Backed up current Sub2API runtime config to:
  `/opt/migration-backups/yuapi-sub2api-retire-20260709165441`
- Stopped only the `sub2api` app service:

```bash
cd /opt/sub2api
docker compose stop sub2api
```

Kept running:

- `sub2api-caddy` because it still proxies YuAPI domains such as
  `api.dtrljm.com`.
- `sub2api-postgres`.
- `sub2api-redis`.
- Kiro/Windsurf/mail helper containers.
- All Sub2API data volumes and config files.

Post-stop smoke:

```text
newapi container: healthy
api.dtrljm.com /: HTTP 200
plus token 82 / gpt-5.4-mini: HTTP 200, log id 7013, channel id 2323
pro token 80 / gpt-5.4-mini: HTTP 200, log id 7014, channel id 2308
sub2api app container: Exited (0)
sub2api-postgres: healthy
sub2api-redis: healthy
sub2api-caddy: running
```

Rollback command if Sub2API app is unexpectedly needed:

```bash
cd /opt/sub2api
docker compose start sub2api
```

Do not remove Sub2API Postgres/Redis volumes until YuAPI-only operation has
completed a longer soak and the remaining non-plus/pro adapter paths are either
migrated, explicitly retired, or documented as out of scope.
