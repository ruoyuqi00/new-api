# Image Price Editor Production Hot Cutover

Date: 2026-10-04 (Asia/Shanghai)

## Release identity

- Source commit: `e50c784454a7d972168f8b0d995cde77a3654006` on `codex/tokenpay-fixed-package-20260927`, pushed to the `fork` GitHub remote.
- Source archive SHA-256: `cd9a21ccb2751727126c94f4e93ea3dae249f0e8eb52d2f4da856b1665d9fd4c`.
- Image: `yuapi:production-image-price-editor-20261004-e50c78445`.
- Image ID: `sha256:8af479219c078e705ab6c81d26a5f8e0a7d4d89a24680a8045db0fab9a8c9569`.
- Binary SHA-256: `db417ef9cd8892c2cf7af306f1e70fe2b1e471d94ce9833895fcba0642194e8d`.
- Runtime version: `e50c78445-image-price-editor-20261004`.
- Active container: `yuapi-production-image-price-e50c78445`, on `127.0.0.1:13064`.
- Previous production container retained for rollback: `yuapi-production-usdt-ui-c1874d09d`.
- Temporary bridge candidate: `yuapi-image-price-candidate-e50c78445`, on `127.0.0.1:13066` while running.

## Change and configuration

Administration path: System settings → Billing & payments → Model pricing → Image resolution prices.

The editor now lists unconfigured image models with a Configure action, and offers Add image model for custom or newly enabled models. The drawer edits default tier and positive, non-decreasing 1K/2K/4K per-image prices using the existing option API. Saved configured policies remain editable even when the pricing catalog hides a model. Existing UI theme, branding, effects, and translations in all six locales are preserved.

Before this cutover, the user-authorized configuration update had already set `gpt-image-2`, `gpt-image-2.5-sunburst`, and `gpt-image-2.5-flare` to 1K `0.01`, 2K `0.02`, and 4K `0.03`, with default tier 1K. The two Nano Banana policies remain intact. The live `生图按次` group ratio is `1`. This deployment did not modify prices, group ratios, balances, customer keys, channels, or payment configuration. No backend source or schema change is included.

## Verification

- Fresh pre-release verification: 7 targeted Bun helper tests; operation_setting and relay/helper Go image-resolution/pricing regression tests; TypeScript typecheck; changed-file lint and format checks.
- Production Docker build succeeded from the hashed source archive on the server.
- The local preview had passed Playwright checks for editing/saving/reloading, adding custom models, default tier selection, duplicate rejection, invalid-price rejection, unrelated settings preservation, and desktop/light/dark/mobile display.
- The candidate returned HTTP 200 for `/`, `/sign-in`, `/keys`, `/wallet`, `/docs`, `/pricing`, and `/system-settings/billing/model-pricing`. Unauthenticated GET and PUT to `/api/option/` returned HTTP 401.
- Candidate authenticated read-only checks confirmed all three model policies.
- `api.yuaiapi.com`, `yuaiapi.com`, and `vip.yuaiapi.com` returned the expected runtime version after routing changed.
- The public `index.6263f2e360.js` asset contains the new editor; SHA-256 `7d9386c9f3302e35fd73b1a169a05125201ee0478dfd6fe9905aa4f8de968d14` matches the candidate asset.
- The final container is `running/healthy/0`. Persisted and live Caddy configurations point their two YuAPI upstreams to the final container, and all other proxy configuration is preserved.
- Snapshots of image policies, ModelPrice, GroupRatio, and GroupGroupRatio matched byte-for-byte across cutover: SHA-256 `e4a7953497fce037bbede6e0c8d32676b814aee38e58cbdd0ddcd77699c6ea9c`.
- All 29 pre-existing unrelated containers retained their IDs, start times, restart counts, and running/stopped state. No unrelated services were started.

## Drain and rollback

The old production container was kept running until its established incoming connections were zero for three consecutive ten-second checks. It was then stopped, and the final master instance was started before Caddy moved new requests from the bridge candidate to it. The candidate is allowed to finish its existing requests before stopping; there is no forced drain deadline or manual socket closure.

The bridge candidate also reached zero connections for three consecutive ten-second checks and was stopped. Both it and the previous production container remain stopped for rollback, consuming no running-container memory. Final verification at `2026-10-04T01:02:32Z` (09:02:32 Asia/Shanghai) confirmed the final container healthy with zero restarts, the public editor asset, identical pricing snapshots, and unchanged protected containers. From cutover start through this check, the Caddy error-log scan found zero YuAPI 502/503/504 responses, and the final application logs contained no panic, fatal error, or schema error. These are bounded observations of the deployment window, not a guarantee against future upstream failures.

Backups and executable rollback workflow: `/opt/newapi/backups/20261004-image-price-editor-e50c78445/`.

The directory contains old/new image and container metadata, private environment files, Caddy persisted/runtime snapshots, pricing-option snapshots, and `hot-switch.sh`. Use `bash hot-switch.sh rollback` there for an application rollback. This UI release has no schema change and does not require restoring a database dump.
