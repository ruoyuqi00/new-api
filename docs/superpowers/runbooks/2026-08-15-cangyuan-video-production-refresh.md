# Video Production Refresh Runbook

> Date: 2026-08-15 (Asia/Shanghai)
>
> Scope: replace only the current video-generation routes, fixed task prices,
> capability catalog, and matching documentation.
>
> This file intentionally contains no server address, external provider URL,
> account identifier, channel credential, API key, password, or real user data.

## 1. Release invariants

- Use blue-green deployment. The current production container must remain
  running, healthy, and attached to its existing aliases throughout candidate
  preparation, traffic switch, observation, and any immediate rollback.
- Never restart Caddy for this release. Validate the complete configuration and
  use a graceful reload.
- Public traffic changes only after explicit user approval of the verified
  candidate facts. Candidate preparation does not imply switch approval.
- Change only video channel rows, the 14 listed `ModelPrice` entries,
  `TASK_PRICE_PATCH`, and a media capability override if runtime readback proves
  one is still required.
- Do not change image routes or prices, group ratios, GPT relay/cache/usage
  behavior, users, balances, existing tasks, MySQL, Redis, DNS, TLS, or other
  Caddy routes.
- Polling, content reads, downloads, refresh recovery, and Canvas restoration
  never create a second task and never charge again.

## 2. Target family channels

Stage these five channels as disabled until their private verification passes.
Their origin, credential boundary, channel type, priority, and weight must be
copied from the approved video upstream configuration without recording secrets
in Git.

```text
cangyuan-video-refresh-20260815-omni=omni-fast,omni-fast-no-water,omni-v2v,omni-v2v-no-water
cangyuan-video-refresh-20260815-grok=grok-video,grok-video-1.5
cangyuan-video-refresh-20260815-happyhouse=happyhouse-1.0,happyhouse-1.1
cangyuan-video-refresh-20260815-minimax=minimax-h3-2k
cangyuan-video-refresh-20260815-seedance=sd4-seedance-2.0,sd4-seedance-2.0-fast,sd7-seedance-2.0-1080p,sd7-seedance-2.0-720p,sd8-seedance-2.0
```

Do not delete legacy video channels. Preserve their full rows disabled as the
configuration rollback source. A model must not be active in both a legacy and
replacement channel at the same priority.

## 3. Fixed prices

Every target model is `per_call`. The base value is the audited upstream price
plus at least 20%, rounded upward to four decimal places:

```text
base_price = ceil(upstream_price * 1.20 * 10000) / 10000
final_price = base_price * existing_group_ratio
```

Expected unchanged group ratios:

```text
多模态创作=1.2
下游多模态=1.0
```

| Model | Base price | `多模态创作` final | `下游多模态` final |
| --- | ---: | ---: | ---: |
| `grok-video` | 0.828 | 0.9936 | 0.828 |
| `grok-video-1.5` | 1.668 | 2.0016 | 1.668 |
| `happyhouse-1.0` | 5.4 | 6.48 | 5.4 |
| `happyhouse-1.1` | 3.48 | 4.176 | 3.48 |
| `minimax-h3-2k` | 4.2 | 5.04 | 4.2 |
| `omni-fast` | 0.7949 | 0.95388 | 0.7949 |
| `omni-fast-no-water` | 0.972 | 1.1664 | 0.972 |
| `omni-v2v` | 1.0628 | 1.27536 | 1.0628 |
| `omni-v2v-no-water` | 1.242 | 1.4904 | 1.242 |
| `sd4-seedance-2.0` | 4.68 | 5.616 | 4.68 |
| `sd4-seedance-2.0-fast` | 3.48 | 4.176 | 3.48 |
| `sd7-seedance-2.0-1080p` | 5.88 | 7.056 | 5.88 |
| `sd7-seedance-2.0-720p` | 4.68 | 5.616 | 4.68 |
| `sd8-seedance-2.0` | 3.48 | 4.176 | 3.48 |

The exact fixed-task environment value is:

```text
grok-video,grok-video-1.5,happyhouse-1.0,happyhouse-1.1,minimax-h3-2k,omni-fast,omni-fast-no-water,omni-v2v,omni-v2v-no-water,sd4-seedance-2.0,sd4-seedance-2.0-fast,sd7-seedance-2.0-1080p,sd7-seedance-2.0-720p,sd8-seedance-2.0
```

## 4. Read-only preflight

Immediately before any server write, record redacted evidence for:

1. Current application image, source commit, container ID, health, restart
   count, private binding, networks, and aliases.
2. Retained rollback image and container health/state.
3. Candidate image digest and source commit.
4. Caddy runtime and file configuration, including exactly two active
   references to the current release alias.
5. Database and Redis health, free disk/memory, and aggregate recent 4xx, 5xx,
   502, database, and Redis errors.
6. Current video channels/mappings/statuses, affected `ModelPrice` values, both
   group ratios, `TASK_PRICE_PATCH`, and the media capability override.

Stop on drift from the approved evidence. Runtime observations take precedence
over remembered names or an older handoff.

## 5. Scoped rollback artifact

Before the first write, create a timestamped server-local directory outside the
repository, owned by root and mode `0600`. Store only:

- complete legacy and replacement video channel rows, including status and
  model mapping;
- the previous values or absence markers for the 14 affected `ModelPrice` keys;
- the previous `TASK_PRICE_PATCH` value;
- the previous media capability override;
- candidate environment names without printing secret values;
- the exact two current Caddy references and a full timestamped Caddyfile copy.

Verify that the artifact parses and that every target key has either a value or
an explicit absence marker. Do not dump the database, users, balances, logs,
tasks, or unrelated settings.

## 6. Candidate preparation

1. Build a uniquely tagged image from the pushed reviewed commit and record its
   immutable digest.
2. Start a uniquely named candidate container on an unused localhost-only port
   with a unique release-network alias.
3. Keep the current production container, existing release alias, Caddy,
   database, Redis, and rollback containers running without restart.
4. Attach Caddy to the candidate release network only as needed for a private
   connectivity check; do not alter either public upstream reference.
5. Verify health, restart count, source commit, frontend asset fingerprints,
   database compatibility, private authenticated catalog, the 14 prices, four
   hidden probe models, and one bounded cheapest real task with same-ID polling
   and content retrieval.
6. Verify the unchanged image route and a bounded GPT text request. Video work
   must not inherit GPT cache, usage estimation, stream-disconnect, or token
   settlement behavior.

Record the candidate container, digest, private port as `localhost:<port>`,
unique alias, check results, scoped configuration delta, and rollback commands.
Then stop and request explicit traffic-switch approval.

## 7. Approved hot cutover

Run only after explicit approval and a fresh no-drift check:

1. Produce a new timestamped Caddyfile rollback copy.
2. Replace exactly the two current application references with the candidate
   release alias in the staged complete Caddy configuration.
3. Confirm the old alias still exists and the current container remains
   healthy and reachable.
4. Validate the complete staged Caddy configuration. Abort without reload on
   any warning, parse error, unexpected third reference, or target mismatch.
5. Gracefully reload Caddy. Do not restart it. Read back the runtime config and
   confirm exactly two candidate references and zero unintended changes.
6. Apply one scoped video configuration transaction: enable only the five
   replacement channels, disable only the identified legacy video channels,
   set exactly the 14 base prices, and set the exact `TASK_PRICE_PATCH` value.
7. Refresh the relevant settings/channel caches and read back every changed
   value. Confirm group ratios and all no-touch settings are unchanged.

Keep both old and candidate application containers and aliases running for
connection draining and the full observation window.

## 8. Public acceptance

Verify public health and asset delivery, authentication, console, pricing,
Studio, Canvas, and developer documentation. Confirm:

- exactly 14 enabled target video models and four probe models hidden from
  normal users;
- exact base and group prices from authenticated pricing/catalog responses;
- one cheapest target creation, one accepted ID, same-ID polling, one content
  download, one charge, and refresh recovery;
- no charge from polling, content reads, download, page refresh, or Canvas
  restoration;
- no new 502 spike, aggregate 5xx spike, container restart, DB/Redis error, or
  secret-bearing log;
- unchanged image generation and bounded GPT text behavior.

## 9. Immediate rollback

Rollback triggers include candidate health loss, any 502 increase, restart,
database/Redis error, price or model mismatch, duplicate creation/charge,
missing result recovery, credential exposure, or an unrelated image/GPT
regression.

Execute in reverse order:

1. Restore the scoped video settings from the rollback artifact, refresh the
   caches, and read back all legacy/replacement statuses, affected prices,
   capability override, group ratios, and `TASK_PRICE_PATCH`.
2. Restore the exact two old Caddy references in the full staged configuration.
3. Validate the complete old configuration and gracefully reload Caddy.
4. Confirm exactly two old references in runtime config and verify public
   health, authentication, one existing image route, pricing, and a bounded GPT
   request.
5. Keep both application containers and all aliases running until rollback
   verification passes. Never stop the current production app or restore a
   database snapshot during immediate rollback.

Do not delete legacy channels, images, containers, aliases, Caddy copies, or
rollback artifacts until a separate cleanup approval is given.
