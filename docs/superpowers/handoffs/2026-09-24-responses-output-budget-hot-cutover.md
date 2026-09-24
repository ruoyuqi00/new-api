# Responses Output Budget Hot Cutover

Date: 2026-09-24 (Asia/Shanghai)

## Release identity

- Source commit: `dc6f0bd01`
- Branch: `codex/image-api-resolution-routing-20260907-local`
- GitHub archive: `fork/codex/image-api-resolution-routing-20260907-local`
- Image: `yuapi:production-responses-budget-20260924-dc6f0bd01`
- Image ID: `sha256:54524cb0e1ee6ceffde706cc8da95eaf452ffdff42b5cc64c4da04c188342739`
- Runtime version: `dc6f0bd01-responses-budget-20260924`
- Active container: `yuapi-production-responses-budget-dc6f0bd01`
- Previous container: `yuapi-production-protocol-routing-20260909` (stopped and retained)
- Previous image: `yuapi:production-image-resolution-ui-20260924-f0205c45c`

The release changed only the YuAPI application, its Caddy upstream target, and
the new channel-scoped output-budget setting. Sub2API, MySQL, Redis, unrelated
containers, prices, balances, usage logs, and model assignments were not
restarted or rewritten.

## Behavior

- OpenAI channels can opt into removing client `max_output_tokens` before an
  upstream Responses request is sent.
- The option applies to native Responses, Chat-to-Responses conversion, and
  raw pass-through requests, and is server-gated to OpenAI channels.
- The default remains disabled. Other providers and channels preserve the
  client's explicit limit.
- Sanitized `response.incomplete` events retain authoritative Responses-shaped
  usage and standard incomplete reasons without exposing provider-private cost
  or metadata fields.
- Automatic replay was not added because replaying an accepted stream can
  duplicate output, tool calls, side effects, and charges.

## Channel rollout

The option was enabled in one transaction for 56 enabled OpenAI text channels
whose model list matches `gpt-<number>`. This excludes image, video, Claude,
DeepSeek, and other non-GPT channels. Channel `2606` is included.

The before-state and a settings-only rollback script are retained as:

- `gpt-channels-before.tsv`
- `channel-settings-rollback.sql`

The new instance's periodic channel cache synchronization was observed after
the transaction. Database verification returned `56/56` JSON boolean `true`
values.

## Recovery artifacts

Recovery artifacts are stored at:

`/opt/newapi/backups/20260924T0425Z-responses-budget-dc6f0bd01/`

They include the source archive and hash, old and new image/container metadata,
Caddy runtime and persisted configurations, channel-setting rollback SQL,
retired heartbeat rows, environment files with mode `0600`, checksums, and an
executable `rollback.sh`.

Normal application rollback runs `rollback.sh`. It starts the retained old
container, waits for health, restores the previous Caddy target, and restores
the persisted Caddyfile. It does not restore database data. The channel setting
rollback is separate and should only be applied if the setting itself must be
undone.

## Verification

- Full `go test ./... -count=1` passed.
- Frontend form tests, type checking, targeted lint/format checks, and the
  production build passed.
- The user approved the local UI on port `13035` before production changes.
- The private candidate and final master returned HTTP 200 for `/`, `/sign-in`,
  `/docs`, `/pricing`, and `/api/status`, with the expected runtime version.
- The binary contains the new channel setting and the final container is
  `running/healthy/0`.
- Across cutover, observation, old-container stop, and final verification, 36
  public status samples succeeded with the new version.
- Structured Caddy checks found zero 502/503/504 responses and zero proxy
  errors after cutover. New application logs contained zero fatal, panic,
  database, or migration errors.
- The old application's established connections drained from six to zero
  before it was stopped. No fixed forced-stop deadline was used.
- The temporary slave candidate was removed. `system_instances` contains only
  `yuapi-production-responses-budget-dc6f0bd01`; the two retired rows were
  backed up before deletion.
- Sub2API, YuAPI MySQL, YuAPI Redis, and Caddy retained their original container
  IDs, start times, and restart counts.
- Local Playwright browser processes could not launch in the Windows host
  environment. This was not treated as browser evidence; UI approval and
  candidate HTTP/resource checks are recorded separately above.
