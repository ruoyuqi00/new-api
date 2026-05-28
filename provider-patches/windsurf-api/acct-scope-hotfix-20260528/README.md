# WindsurfAPI acct Scope Hotfix

Date: 2026-05-28

This patch is a small runtime hotfix for `dwgx/WindsurfAPI v2.0.97`
(`41a36b9176633a9e67eb7ca87d725b5eb98564b8`).

## Symptom

Public Sub2API calls through the internal Windsurf adapter reached Cascade and
the upstream model generated text, but WindsurfAPI returned HTTP 502:

```text
acct is not defined
```

The failure happened after the model response, during the non-streaming
success path that writes the sticky session binding.

## Cause

`nonStreamResponse()` referenced the outer `acct` variable while the function
does not receive `acct` in its local scope.

## Patch

The patch threads `acct.id` into `poolCtx.accountId` at the call site, then
uses `poolCtx.accountId` and `poolCtx.apiKey` when writing the sticky binding.

This keeps account selection, routing, cache, usage, and response shaping
unchanged.

## Server Deployment

On the live server this is built as a local image:

```bash
cd /opt/sub2api
docker compose build windsurf-api
docker compose up -d --no-deps --force-recreate windsurf-api
```

The live compose service uses:

```yaml
build:
  context: ./windsurf-api-acctfix
image: sub2api-windsurf-api-acctfix:20260528
```

## Removal

Remove this local image only after upstream WindsurfAPI has fixed the same
scope bug. Then switch `windsurf-api` back to:

```yaml
image: ghcr.io/dwgx/windsurf-api:latest
```

After switching, re-run the public Sub2API smoke tests for at least:

- `opus4.6`
- `gpt5.5-low`
- one cross-group rejection case
