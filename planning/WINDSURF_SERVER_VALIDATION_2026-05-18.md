# Windsurf Server Validation - 2026-05-18

This note records the actual server validation for the Windsurf Stage A
deployment. It intentionally uses placeholders for secrets.

## Scope

- Server: US Ubuntu host running Docker Compose under `/opt/sub2api`.
- Public surface: Sub2API only.
- Internal surface: `windsurf-api` reachable only on the Compose network.
- Sub2API fork image: `sub2api-provider-adapters:544f553b`.
- WindsurfAPI image: `ghcr.io/dwgx/windsurf-api:latest`.
- WindsurfAPI reference version: `v2.0.96`.

## What Was Verified

Upstream freshness:

- `dwgx/WindsurfAPI` reference repo was fetched with tags.
- Latest observed tag: `v2.0.96`.
- Latest observed commit: `c028576`.
- Sub2API official upstream had no new commits ahead of the private fork at
  the time of verification.

Sub2API import endpoint:

- `POST /api/v1/admin/accounts/import/windsurf`
- Supported shapes verified:
  - `{"token":"..."}`
  - `{"tokens":["..."]}`
  - raw line input
  - `{"email":"...","password":"..."}`
  - `{"api_key":"..."}`
  - `{"accounts":[{"api_key":"..."}]}`

Runtime smoke:

- Internal WindsurfAPI `/v1/messages` returned 200 for:
  - `gemini-2.5-flash`
  - `claude-sonnet-4.6`
  - `claude-4.5-haiku`
- Public Sub2API `/v1/messages` returned 200 for:
  - `gemini-2.5-flash`
  - `claude-sonnet-4.6`

## Important Finding

WindsurfAPI v2.0.96 can produce this state:

- Account tier: Pro Trial.
- Capability probe for `gemini-2.5-flash`: successful.
- `availableModels`: does not include `gemini-2.5-flash`.
- Result: `/v1/messages` returns 403 `model_not_entitled` before selecting an
  account.

The short-term server fix was to set the trial account tier to `pro` through
the internal dashboard API, then clear Sub2API's error state for
`windsurf-internal-anthropic` and restart Sub2API.

The long-term source fix should be either:

- Patch or fork WindsurfAPI so successful capability probes count as available
  models.
- Or add a small adapter-side reconciliation layer before Sub2API relies on
  `availableModels`.

Manual blocklists and explicit `not_entitled` records must still win over a
generic successful probe.

## Safe Recheck Commands

Internal Windsurf account/model state:

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

docker compose exec -T -e WKEY="$WINDSURF_API_KEY" windsurf-api node - <<'JS'
const res = await fetch('http://127.0.0.1:3003/auth/accounts', {
  headers: { 'x-api-key': process.env.WKEY },
});
const data = await res.json();
for (const a of data.accounts || []) {
  const available = a.availableModels || [];
  const cap = a.capabilities?.['gemini-2.5-flash'];
  console.log({
    id: a.id,
    status: a.status,
    tier: a.tier,
    hasGemini: available.includes('gemini-2.5-flash'),
    geminiCapOk: cap?.ok,
    geminiCapReason: cap?.reason,
  });
}
JS
```

Public Sub2API smoke:

```bash
curl -sS http://127.0.0.1:8080/v1/messages \
  -H "authorization: Bearer <sub2api-test-key>" \
  -H 'content-type: application/json' \
  -H 'anthropic-version: 2023-06-01' \
  -d '{
    "model": "claude-sonnet-4.6",
    "max_tokens": 32,
    "messages": [{"role": "user", "content": "reply with ok"}]
  }'
```

## Update Rule

When updating either Sub2API or WindsurfAPI:

1. Back up `/opt/sub2api` config and database.
2. Pull or build the new image.
3. Restart only the changed service first.
4. Check `docker compose ps`.
5. Re-run internal Windsurf smoke.
6. Re-run public Sub2API smoke.
7. If a model fails with `model_not_entitled`, inspect `availableModels` before
   assuming the account is actually not entitled.

## Security Rule

Do not expose `windsurf-api` publicly. Keep all account import and dashboard
operations on the server or through Sub2API's authenticated admin path.
