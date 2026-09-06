# Channel Test Response And Renderer-Aware UI Hot Cutover

Date: 2026-09-06 (Asia/Shanghai)

## Release identity

- Source commit: `a2099648d76ba52b80b5594fa9cd89e5f1888a34`
- Branch: `codex/channel-test-response-ui-performance-20260906`
- Image: `yuapi:production-20260906-channel-test-a2099648d`
- Image ID: `sha256:687c00cab3eb2e5ee78f1e87a8edc9fb8af007dcf4616ce7c842a61fea9030cd`
- Active container: `newapi-channel-test-a2099648d`
- Rollback container: `newapi-log-cleanup-f9584b54f`
- Rollback image: `yuapi:production-20260905-log-cleanup-f9584b54f`

The image was built as an immutable incremental layer over the accepted
production image. The database, Redis, volumes, channels, groups, prices,
balances, usage logs, and Caddy routes were not otherwise changed.

## Changes

- Channel full-test success results can expose a safe extracted response body
  in the admin dialog, capped at 8 KB and excluding raw JSON metadata, headers,
  keys, and upstream addresses.
- Chat, Responses, Anthropic, Gemini, and their supported SSE response shapes
  are normalized for the display-only test result.
- WebGL is now gated by the actual renderer: software/no-WebGL browsers use the
  existing static YuCore earth and brand layers, while real hardware renderers
  retain the dynamic effects.
- CPU, memory, screen-density, and viewport heuristics no longer disable the
  visual effects on capable devices.

## Cutover and rollback

- Recovery artifacts: `/opt/newapi/backups/20260906-channel-test-a2099648d-retry/`
- Caddy was atomically switched from the rollback container to the active
  container. The old container remains running and was not removed.
- The rollback path is to restore the saved Caddy configuration from that
  directory and reload Caddy. Do not restore a database snapshot.

## Verification

- Candidate state: `running/healthy/0`.
- Rollback state: `running/healthy/0`.
- Caddy runtime and persisted configuration each contain the new target twice
  and the old target zero times.
- Candidate `/`, `/sign-in`, `/sign-up`, `/docs`, and `/api/status` returned
  HTTP 200 on the private port.
- Playwright against the candidate found zero page errors and zero failed
  requests; software rendering showed the static fallback and an AMD GPU run
  retained the dynamic Canvas layers.
- Public `api` and `vip` status checks returned HTTP 200 in three samples.
  The public `global` probe returned the same 403 seen before cutover, while
  the Caddy-to-candidate origin check returned 200; no application or route
  change was made for that existing Cloudflare behavior.
- Candidate and Caddy bounded post-cutover checks found no transport errors,
  fatal errors, panics, or migration errors.
- No paid upstream completion or media request was sent.

