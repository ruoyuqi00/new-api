# NewAPI sidecar deployment - 2026-06-04

## Goal

Run NewAPI and Sub2API on the same server without sharing account state,
database state, or Redis token cache state.

This gives the deployment two separate roles:

- NewAPI: CPA/OpenAI OAuth account pools and standard OpenAI-compatible model
  aggregation.
- Sub2API: private Kiro/Windsurf adapters, Anthropic compatibility work, CCS
  import behavior, and other custom protocol routing.

## Server layout

Server:

- Host: `154.219.122.197`
- Existing Sub2API path: `/opt/sub2api`
- NewAPI path: `/opt/newapi`

Public URLs:

- Sub2API: `https://api.vyywcw.cn`
- NewAPI: `https://newapi.vyywcw.cn`

NewAPI compose services:

- `newapi`
- `newapi-mysql`
- `newapi-redis`

Images:

- `calciumion/new-api:latest`
- `mysql:8.4`
- `redis:7-alpine`

The NewAPI app also publishes `127.0.0.1:3001 -> 3000` for server-local
diagnostics only.

## Data isolation

NewAPI uses its own database, Redis, secrets, and data paths:

- `/opt/newapi/.env`
- `/opt/newapi/docker-compose.yml`
- `/opt/newapi/mysql_data`
- `/opt/newapi/redis_data`
- `/opt/newapi/data`

The NewAPI containers attach to the existing Docker network
`sub2api_sub2api-network` only so the existing Caddy container can reverse
proxy traffic to `newapi:3000`. They do not use the Sub2API Postgres or Redis
containers.

## Caddy routing

The existing `/opt/sub2api/Caddyfile` now includes
`newapi.vyywcw.cn` in the same site block as `api.vyywcw.cn` and
`www.vyywcw.cn`.

A host matcher routes NewAPI traffic first:

```caddy
@newapi host newapi.vyywcw.cn
handle @newapi {
    reverse_proxy newapi:3000
}
```

All existing Sub2API/Kiro/Windsurf routes remain in the same file and continue
to handle `api.vyywcw.cn`.

Backups were created before Caddy edits under `/opt/sub2api` with
`Caddyfile.bak-newapi-*` names.

## Verification

NewAPI:

- `newapi`, `newapi-mysql`, and `newapi-redis` containers are healthy.
- Logs show `New API v1.0.0-rc.10 started`.
- `http://127.0.0.1:3001/` returned HTTP 200.
- HTTPS routing to `newapi.vyywcw.cn` returned HTTP 200.
- `https://newapi.vyywcw.cn/` returned HTTP 200 from the local machine.

Sub2API regression check:

- `https://api.vyywcw.cn/health` returned `{"status":"ok"}`.
- The `sub2api` container remained healthy.

## Initialization

NewAPI is intentionally left uninitialized. The first root/admin user should be
created through the NewAPI web UI by the operator.

URL:

```text
https://newapi.vyywcw.cn/
```

## CPA account policy

Do not run the same CPA/OpenAI OAuth account set in both systems with
auto-refresh enabled.

Reason:

- OpenAI OAuth refresh tokens rotate.
- If NewAPI refreshes a CPA account first, it receives and stores the next
  refresh token.
- Sub2API would still hold the old refresh token and then fail with
  `refresh_token_reused`, or vice versa.

Recommended split:

- Import CPA/OpenAI OAuth pools into NewAPI.
- Keep Sub2API for Kiro/Windsurf/private adapter groups.
- If a shared public entrypoint is needed later, add a lightweight gateway that
  routes by model family or key group rather than duplicating OAuth accounts in
  both systems.
