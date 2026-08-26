# Group Settings And Availability Hot Cutover

## Release

- Commit: `bfbb67093155593e1fc8d9328333cd0d2adfb1bb`
- Image: `yuapi:production-20260826-user-group-monitor-bfbb67093`
- Active container: `newapi-user-monitor-bfbb67093`
- Rollback container retained: `newapi-user-monitor-3931dce17`
- Previous fallback container retained: `newapi-domestic-2d8b3d3de`
- Production database and Redis data were not restored, reset, or copied.

## Changes

- System settings now hydrate persisted user-specific group ratios and availability-monitor switches from their canonical option keys.
- Availability monitoring records only GPT text paths and retains at most 300 recent samples per group.
- Fewer than 20 samples are shown as observing; after 20 samples, status thresholds are stable at 90% or higher, degraded at 60% to below 90%, and unavailable below 60%.
- The API-key group monitor uses a fixed-width horizontal scroller with a success/failure bar and does not expose latency, upstream, model, or channel details.
- Image, video, audio, and asynchronous task paths remain outside this monitor.

## Cutover Evidence

- Pre-cutover Caddy configuration was validated and backed up under `/opt/newapi/backups/20260826T023332Z-user-monitor-tolerant/`.
- Candidate was bound privately on `127.0.0.1:13058` before cutover and passed health checks with restart count 0.
- Candidate was attached to the existing Caddy release network for an internal health probe.
- Caddy was gracefully reloaded after validating the candidate configuration; Caddy itself was not restarted.
- `https://api.yuaiapi.com/api/status`, `https://global.yuaiapi.com/api/status`, and `https://vip.yuaiapi.com/api/status` returned HTTP 200 after the switch.
- Five post-cutover health samples passed with the active candidate healthy and restart count 0.
- Public homepage static-resource fingerprint matched the candidate fingerprint.
- The old active container remains running and healthy for immediate Caddy rollback.

## Rollback

Restore `/opt/newapi/backups/20260826T023332Z-user-monitor-tolerant/Caddyfile.runtime-before` into the Caddy container, validate it, and gracefully reload Caddy. Keep both application containers and images until the release observation window is closed. Do not restore a database snapshot or mutate user balances during rollback.
