# Upstream Proxy Plan: Kiro and Windsurf

This fork keeps Sub2API as the only public gateway. Provider-specific
credential adapters run as internal upstream services first, then native
Sub2API account types can be added after the protocol and token behavior is
stable.

## Current Public Surface

- Public HTTPS entrypoint: Sub2API only.
- Internal network services:
  - Kiro adapter/proxy.
  - Windsurf adapter/proxy.
  - Future provider-specific token importers.

## Windsurf GitHub Research

Best short-term candidate:

- Repository: `dwgx/WindsurfAPI`
- Local research checkout: `D:\wflogin\_github_research\WindsurfAPI`
- Observed latest local tag: `v2.0.96`
- Observed latest local commit: `c028576 release: 2.0.96`
- Runtime: Node.js, Docker supported.
- Compatible APIs:
  - `POST /v1/chat/completions` for OpenAI-compatible clients.
  - `POST /v1/messages` for Anthropic-compatible clients.
- Account management:
  - `POST /auth/login` with `{ "token": "..." }`.
  - Batch import with `{ "accounts": [{ "token": "t1" }, { "token": "t2" }] }`.
  - Also has email/password and direct api_key flows, but token import is the
    cleaner production path.
- Persistence:
  - `accounts.json`
  - `proxy.json`
  - `stats.json`
  - `runtime-config.json`
  - `model-access.json`
- Required runtime asset:
  - Windsurf language server binary at
    `/opt/windsurf/language_server_linux_x64`, or installed by the project
    helper script/container startup path.

Secondary candidate:

- Repository: `guanxiaol/WindsurfPoolAPI`
- Local research checkout: `D:\wflogin\_github_research\WindsurfPoolAPI`
- Observed latest local commit: `a8d2f4c v2.0.7`
- Similar idea, but `dwgx/WindsurfAPI` currently looks more mature for
  internal upstream deployment.

Not recommended for production gateway path:

- Signup/trial automation tools.
- Anti-detect/browser automation flows.
- Desktop-only account managers unless extracted into a small headless
  service.

## Recommended Short-Term Deployment

Run `dwgx/WindsurfAPI` as an internal Docker service on the same Docker network
as Sub2API. Do not expose it through Caddy or a public port.

Suggested internal service settings:

- `HOST=0.0.0.0`
- `PORT=3003`
- `API_KEY=<strong internal key>`
- `DASHBOARD_PASSWORD=<strong admin password>`
- `DATA_DIR=/data`
- `LS_BINARY_PATH=/opt/windsurf/language_server_linux_x64`

Then add a Sub2API upstream account pointing to the internal service:

- Anthropic-compatible route:
  - `base_url = http://windsurf-api:3003`
  - `api_key = <same internal API_KEY>`
  - enable Anthropic passthrough if Sub2API requires it for headers/body shape.
- OpenAI-compatible route:
  - `base_url = http://windsurf-api:3003/v1`
  - `api_key = <same internal API_KEY>`

## Account Import Flow

Import only owned Windsurf account tokens into the internal service.

Example internal-only batch request:

```bash
curl -sS http://127.0.0.1:3003/auth/login \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <internal-api-key>' \
  -d '{"accounts":[{"token":"token-1"},{"token":"token-2"}]}'
```

For server operations, write account tokens into a root-only file temporarily,
post them from the server shell, then remove the temporary file.

## Verification Flow

1. Check the internal Windsurf service health/logs.
2. Import one test account token.
3. Call `POST /v1/messages` directly on the internal service.
4. Add the internal service as a Sub2API account.
5. Call Sub2API publicly and verify the request routes through Windsurf.
6. Keep Caddy exposing only Sub2API.

## Long-Term Native Sub2API Work

After the internal upstream is stable:

1. Add native account platform/type metadata for `windsurf` and `kiro`.
2. Add admin import UI/API fields for each provider credential shape.
3. Store provider credentials in Sub2API account credentials.
4. Add token refresh providers.
5. Reuse the Anthropic/OpenAI request translators where possible.
6. Add provider-specific model mapping and quota/error handling.

The short path gets the server running quickly. The native path makes Sub2API
own credential lifecycle later, which is cleaner but higher risk.
