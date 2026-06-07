# NewAPI sidecar deployment - 2026-06-04

## Goal

Run NewAPI and Sub2API on the same server without sharing account state,
database state, or Redis token cache state.

This is a sidecar deployment, not a Sub2API replacement. Sub2API remains the
primary private fork in this repository and continues to own the custom
Kiro/Windsurf/provider-adapter work.

This gives the deployment two separate roles:

- NewAPI: CPA/OpenAI OAuth account pools and standard OpenAI-compatible model
  aggregation.
- Sub2API: private Kiro/Windsurf adapters, Anthropic compatibility work, CCS
  import behavior, and other custom protocol routing.

## Repository policy

NewAPI deployment notes are committed to this same private repository on
`main` because they are part of the same production server's operating model.
They are not placed on a separate branch because a branch would look like an
alternate product direction or a migration away from Sub2API.

For future changes:

- Keep Sub2API source, patches, and upstream merges on `main`.
- Keep NewAPI server operation notes in `planning/` unless a reusable,
  sanitized deployment template is intentionally added later.
- Do not add `/opt/newapi/.env`, generated passwords, API keys, CPA tokens, or
  live `docker-compose.yml` files containing secrets to Git.
- If a reusable NewAPI compose template is added later, place it under a clearly
  named path such as `deploy/newapi-sidecar/` and keep it secret-free.
- Always describe NewAPI as a sidecar/complement for CPA/OpenAI OAuth pools, not
  as evidence that Sub2API is abandoned.

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

Codex import rule:

- NewAPI Codex channels must store the real ChatGPT `account_id`.
- Do not use synthetic placeholders such as `pending-*`; NewAPI sends this
  field as the `chatgpt-account-id` request header.
- When importing CPA/Codex JSON manually, extract `chatgpt_account_id` from the
  `access_token` JWT or use an import path that refreshes and normalizes the
  credential chain like cockpit-tools.
- If cockpit-tools already refreshed a pasted credential, export/use the latest
  credential chain; the old `refresh_token` may have been rotated away.

## GPT-5.5 CPA groups

Two NewAPI groups were configured for the imported CPA/Codex account pools:

- `cpa-gpt55-a`: 6 channels from tag `cpa-codex-20260604`.
- `cpa-gpt55-b`: 10 channels from tag `cpa-codex-c72b8eef485f4865`.

Each group has its own API token, limited to `gpt-5.5`, with cross-group retry
disabled. Full token values are intentionally not stored in Git.

The channel model lists were updated to include `gpt-5.5`; NewAPI's enabled
model list confirms the model is routable. Both group tokens were smoke-tested
against:

```text
POST https://newapi.vyywcw.cn/v1/responses
```

Required request shape:

```json
{
  "model": "gpt-5.5",
  "stream": true,
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Reply OK only."
        }
      ]
    }
  ],
  "max_output_tokens": 8
}
```

Do not use `gpt-5.5-codex` for these ChatGPT/Codex accounts; upstream rejects
that model name.

## Manual HH CPA/Codex reimport

On 2026-06-07, four manually supplied HH CPA/Codex accounts were rechecked and
kept in NewAPI after the Codex `account_id` metadata repair.

Final channel placement:

- tag: `cpa-codex-manual-hh-reimport-20260607`
- group: `gpt`
- channel IDs: `54`, `55`, `56`, `57`
- model used for smoke: `gpt-5.5`

Verification:

- all four channels are enabled;
- all four channel keys include `type: codex`, a real ChatGPT `account_id`,
  and present access/refresh token fields;
- a temporary `gpt` group token was created only for smoke testing and deleted
  afterward;
- each channel completed a channel-specific call to
  `POST https://newapi.vyywcw.cn/v1/responses` with `stream: true`.

Operational note:

- When testing channel-specific keys through SSH automation, transfer the remote
  script as base64 and run it with `bash`. Passing multiline shell directly
  through Windows PowerShell can split curl headers and produce misleading
  NewAPI-layer `Invalid token` errors before the request reaches the channel.
