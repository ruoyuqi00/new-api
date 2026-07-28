# VIP Direct API Design

## Goal

Add `vip.yuaiapi.com` as a DNS-only API origin for image generation,
video generation, other long-running requests, and approved high-throughput
customers. Keep `api.yuaiapi.com` behind Cloudflare as the normal public API
entry point.

## Chosen Approach

Use the existing production NewAPI service instead of introducing a second
application instance. Caddy will expose a separate API-only virtual host for
`vip.yuaiapi.com`, while the current Cloudflare-backed hosts remain unchanged.

This keeps billing, tokens, user concurrency, routing, account pools, and logs
identical across both URLs. Any valid API key may use the direct URL. There is
no separate VIP group allowlist or additional admin setting.

## Alternatives Considered

### Separate direct application container

A separate `newapi-vip` container would isolate file descriptors and HTTP
client pools. It also adds another deployment unit and makes configuration
drift more likely. Current CPU, memory, MySQL, and Redis measurements do not
justify that complexity.

### Dedicated direct server

A dedicated server provides the strongest traffic and failure isolation, but
it adds cost and database/network coordination. It is unnecessary at current
traffic levels and remains an option if the single 200 Mbps origin becomes the
bottleneck.

## DNS And TLS

- Create or update an `A` record for `vip.yuaiapi.com` pointing to the current
  production origin.
- The Cloudflare proxy flag must be disabled (`proxied=false`).
- Caddy obtains and renews the public TLS certificate automatically.
- Verification must confirm that responses do not contain `CF-Ray` and that
  public DNS resolves to the real origin rather than Cloudflare addresses.

## Caddy Routing

Create a dedicated `vip.yuaiapi.com` site block. It accepts only relay API
paths used by the gateway:

- `/v1/*`
- `/v1beta/*`
- `/mj/*`
- `/suno/*`
- `/kling/*`
- `/jimeng` and `/jimeng/*`
- `/api/status` for health verification

All other paths return `404`. The VIP host must not expose login, registration,
wallet, dashboard, or admin pages. Requests are proxied to `newapi:3000` on the
existing Docker network with the same forwarded headers as the public API.

The request body limit remains 256 MB so existing image and multimodal uploads
continue to work.

## Authentication And Limits

- NewAPI token authentication remains the access boundary.
- Invalid or disabled API keys continue to return `401`.
- Existing per-user concurrency and optional group overrides apply equally on
  both public and VIP URLs because they are keyed by user ID in Redis.
- Existing channel/account-pool concurrency and cooldown behavior is unchanged.
- No hostname-specific bypass of billing, model access, or rate limits is
  introduced.

## Long-Running Requests

Cloudflare's proxy request limit is removed by the DNS-only route. The current
`RELAY_TIMEOUT=0` behavior remains unlimited. Increase `STREAMING_TIMEOUT` from
120 seconds to 1800 seconds so a valid stream may stay silent while an image or
video job is preparing without being terminated by the gateway.

Raise the `nofile` soft and hard limits for the NewAPI and Caddy containers to
65535. This removes the current 1024-file-descriptor ceiling before exposing a
high-concurrency origin. Per-user concurrency protection remains enabled.

## Documentation

Update the production in-app API documentation to show both base URLs:

- Cloudflare-backed endpoint: `https://api.yuaiapi.com/v1`
- DNS-only direct endpoint: `https://vip.yuaiapi.com/v1`

The documentation must explain that the VIP URL bypasses Cloudflare's timeout,
uses the same API key and billing account, and is available when a user's own
client or downstream configuration needs a direct connection for image, video,
or other requests that may run longer than 100 seconds. The gateway does not
automatically redirect, prefer, or rewrite either URL; the user explicitly
selects the base URL in their client according to the documentation. The docs
must include concrete configuration examples for both endpoints. They must not
describe the VIP URL as having separate models, prices, balances, or
permissions.

## Capacity Boundary

The current server has 8 vCPUs, 8 GB RAM, and a 200 Mbps network link. Recent
production traffic averaged about 32.8 seconds per completed request. Before
raising `nofile`, the practical limit is approximately 300-400 simultaneous
proxy requests, or roughly 550-730 RPM at that request duration. The direct
hostname does not add compute or upstream capacity; it removes Cloudflare from
the request path. After raising `nofile`, upstream channels and the 200 Mbps
link become the primary constraints.

## Failure Handling And Rollback

- Back up the active Caddyfile and Compose files before changing them.
- Validate the candidate Caddy configuration before reload.
- Keep `api.yuaiapi.com` serving traffic throughout the change.
- If VIP health, TLS, or authenticated relay checks fail, remove the VIP Caddy
  site and restore the previous Compose limits. The public API remains the
  unaffected fallback.
- DNS can be removed or switched back to proxied mode without changing the
  public API entry point.

## Verification

1. Confirm `api.yuaiapi.com/api/status` remains HTTP 200.
2. Confirm `vip.yuaiapi.com/api/status` is HTTP 200 without a `CF-Ray` header.
3. Confirm an invalid key receives HTTP 401 from the VIP host.
4. Confirm a current valid key can list models through the VIP host.
5. Confirm `/`, `/login`, and `/api/user/login` return HTTP 404 on the VIP host.
6. Run one minimum-cost image or video request that remains open for more than
   100 seconds and verify it completes without edge truncation.
7. Confirm Caddy and NewAPI container health, file-descriptor limits, resource
   use, and error logs after the change.

## Scope Boundary

This work changes production gateway configuration and production API
documentation only. Local experimental UI and preview assets are excluded and
must not be staged, committed, built into the image, transferred, or deployed.
