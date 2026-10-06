# Gemini model audit production hot cutover

Date: 2026-10-06 (Asia/Shanghai).

## Release identity

- Source commit: `d7622084cfee033f313c2b983ae5298ff7e549a1`, pushed to the `fork` remote on `codex/tokenpay-fixed-package-20260927`.
- Source archive SHA-256: `6598a6a8d4932c17a94f1dbb14906077cafbb68473cd72ccd73e97633bd6edae`.
- Image: `yuapi:production-gemini-model-audit-20261006-d7622084c`.
- Image ID: `sha256:2b5d8b345b1dd67cbf4e59ef6e8925ba037ff086188b3ad7ad062aae85e21f0b`.
- Binary SHA-256: `64fda76b0e5a289a1485ff0ce56356cddd115727f9d660fbd2a66c0df5393e32`.
- Runtime version: `d7622084c-gemini-model-audit-20261006`.
- Previous production container: `yuapi-production-image-price-e50c78445`.
- Bridge candidate: `yuapi-gemini-audit-candidate-d7622084c`, on `127.0.0.1:13066` while running.
- Final production target: `yuapi-production-gemini-audit-d7622084c`, on `127.0.0.1:13064`.

## Scope

The user approved hot switching after reviewing the local production-brand UI. The release adds Gemini `modelVersion` capture to the existing response-model audit helper and hooks non-streaming native, Chat/Claude conversion, and Responses conversion handlers into it. Native SSE uses the shared scanner. The existing admin log popover and details dialog display the captured model data.

No frontend source, response payload, billing, channel mapping, schema, balance, or payment configuration change is included. The public `index.6263f2e360.js` hash is identical to the previous production version: `7d9386c9f3302e35fd73b1a169a05125201ee0478dfd6fe9905aa4f8de968d14`. UI branding, theme, and effects therefore come from the same frontend build content.

## Verification

- Fresh pre-release checks passed: the complete Gemini package tests; targeted response-model capture, normalization, log persistence/privacy, and forwarded-model snapshot tests in helper/model/service; and `go vet` for helper/Gemini.
- The hashed source archive was transferred and the production Docker image built successfully.
- Candidate status was `running/healthy/0`. Eight frontend routes returned HTTP 200, including `/usage-logs/common`, `/wallet`, `/keys`, and image-price administration. Unauthenticated GET/PUT to options and GET to admin logs returned HTTP 401.
- After the bridge switch, all three public YuAPI domains returned the new runtime version. Public frontend asset hash matched the candidate and previous production frontend.
- Real post-switch native Gemini logs on channel 2486 recorded request model `gemini-3.8-flash` with actual response model `gemini-3.8-flash-n`. Example consume-log IDs: `52143350`, `52143379`, and `52144008`. These are normal customer requests observed read-only; no canary charge or synthetic production log was created.
- The initial bridge verification found unchanged IDs/start times/restart counts/states for all 32 other pre-existing containers and zero YuAPI Caddy 502/503/504 since switch start.
- The old production instance reached zero incoming connections for three consecutive checks and was stopped. The final master then became healthy with zero restarts on the expected image/binary, and both persisted/runtime Caddy targets point to it. Its master and system-task-runner settings were verified. Pricing snapshots matched byte-for-byte across this step: SHA-256 `0a14d4d1a8a9d42886b8b4b876bab48ff3954f8a2da4214a27fa678644603b70`.
- Read-only checks of requests finishing after the final container started confirmed native `generateContent` and `streamGenerateContent` log capture. Examples: log `52194429` (non-streaming) and `52194364` (streaming) request `gemini-3.8-flash` and record actual response model `gemini-3-flash`; log `52194124` records actual response model `gemini-3.8-flash-n`.

Final verification completed at `2026-10-06T10:28:11Z` (18:28:11 Asia/Shanghai). The candidate also reached zero connections for three consecutive checks and was stopped. Both the old production instance and bridge candidate remain stopped for rollback and consume no running-container memory. The final instance is `running/healthy/0`; all three public domains return the expected version and the public brand/model-audit asset hash matches the previous frontend. Pricing snapshots are identical, and all 32 protected containers retain their original IDs, start times, restart counts, and states. From switch start through this final check, the Caddy error-log scan found zero YuAPI 502/503/504 and the final application logs contained no panic, fatal error, or schema error. These are observations of the deployment window, not a guarantee against later upstream failures.

## Recovery and drain

Release files: `/opt/newapi/releases/gemini-model-audit-20261006-d7622084c/`.

Private backups and rollback script: `/opt/newapi/backups/20261006-gemini-model-audit-d7622084c/`. The directory contains old/new image metadata, private environment files, Caddy persisted/runtime snapshots, pricing-option snapshots, source/candidate asset identity, cutover timestamps/logs, and `hot-switch.sh`.

The switch first routes new requests to the candidate while the old master finishes its existing requests. Each instance must reach zero established incoming connections for three consecutive ten-second checks before stopping. There is no forced drain deadline or manual socket closure. After the old instance drains, the final master starts on the established port and takes new traffic; the candidate then drains. Stopped images/containers remain available for rollback. Application rollback uses `bash hot-switch.sh rollback`; no schema or data restore is required.
