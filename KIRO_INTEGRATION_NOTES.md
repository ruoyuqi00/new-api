# Kiro Integration Notes

Local fork setup:

- Working tree: `D:\wflogin\sub2api-fork`
- Branch: `custom/kiro-support`
- Official remote: `upstream = https://github.com/Wei-Shaw/sub2api.git`

Current upstream account model in Sub2API:

- Platforms: `anthropic`, `openai`, `gemini`, `antigravity`
- Account types: `oauth`, `setup-token`, `apikey`, `upstream`, `bedrock`, `service_account`
- Anthropic API key accounts already support custom `credentials.base_url`.
- Anthropic API key accounts also support an `extra.anthropic_passthrough` mode for forwarding requests with authentication replacement.

Latest production note:

- 2026-06-16: Kiro group was aligned to Anthropic routing for Claude Code usage.
- Production image `sub2api-provider-adapters:kiro-counttokens-20260616b` adds a Kiro-specific `/v1/messages/count_tokens` fallback that returns local `input_tokens` estimates when the Kiro Web adapter returns generic 404.
- Detailed record: `docs/KIRO_CLAUDE_CODE_2026-06-16.md`.

Kiro credential model observed from `kiro.rs`:

- OAuth-like credentials:
  - `accessToken`
  - `refreshToken`
  - `expiresAt`
  - `authMethod`: `social` or `idc`
  - optional `clientId` / `clientSecret` for IDC auth
  - optional `profileArn`
  - optional `region`, `authRegion`, `apiRegion`
  - optional credential-level proxy
- Static bearer-style credential:
  - `kiroApiKey`
- Kiro-rs exposes an Anthropic-compatible API:
  - `GET /v1/models`
  - `POST /v1/messages`
  - `POST /v1/messages/count_tokens`
  - `POST /cc/v1/messages`
  - `POST /cc/v1/messages/count_tokens`

Implementation options:

1. Short path, no native Kiro code in Sub2API:
   - Run `kiro.rs` as a separate upstream service.
   - Add a Sub2API Anthropic API Key account.
   - Set `base_url` to the Kiro-rs service URL.
   - Set `api_key` to the Kiro-rs configured API key.
   - Enable Anthropic API key passthrough when needed.
   - This uses Kiro-rs for token refresh and Kiro protocol details.

2. Native path, real Sub2API Kiro support:
   - Add Kiro credential import UI and admin API.
   - Store Kiro credential fields in account credentials.
   - Add a Kiro token provider that refreshes `refreshToken` into `accessToken`.
   - Reuse the Anthropic messages request/response shape as much as possible.
   - Add model mapping defaults for Claude model names supported by Kiro.
   - Add account testing and usage/quota handling after basic routing works.

Working assumption:

- Model naming and request shape are not the hard part because Kiro-rs is already Anthropic-compatible.
- The hard part is credential acquisition, token refresh, headers, region/machine identity, and failure handling.

## Windsurf Short-Path Notes

Local Windsurf projects observed:

- `D:\wflogin\windsurf-register-free-2\windsurf-auto-free-main`
  - Electron account manager.
  - Stores accounts in `data/accounts.db`.
  - Existing service code can import `email:password` or JSON accounts.
  - Login result stores `api_key`, `refresh_token`, `id_token`, `auth1_token`, `expires_at`.
  - Has batch refresh logic.
- `D:\wflogin\windsurf-auto-tools`
  - Browser automation for signup/trial flows.
  - Not needed for the API gateway path.
- `D:\wflogin\Windsurf-Tool`
  - Larger desktop/tooling codebase, useful as a reference.

Recommended deployment shape:

- Public exposure:
  - Only expose Sub2API through Caddy/HTTPS.
- Internal-only services:
  - `kiro-rs`, bound to Docker network or `127.0.0.1`.
  - `windsurf-token-service`, bound to Docker network or `127.0.0.1`.
  - Any future account adapters.
- Sub2API accounts:
  - Add internal upstream/API-key style accounts pointing to those services.
  - Keep all provider tokens out of public routes.

Windsurf short-path implementation:

1. Extract the existing Windsurf login/refresh logic into a headless Node service.
2. Add an internal API for owned account operations:
   - `POST /admin/accounts/import`
   - `POST /admin/accounts/refresh`
   - `GET /admin/accounts`
   - `GET /health`
3. Add an internal model/API proxy only if we confirm the request protocol needed by Windsurf runtime.
4. In Sub2API, first integrate it as an internal upstream rather than native code.
5. Native Sub2API support can come later after the token and request proxy behavior is stable.

Guardrail:

- Use this path for owned accounts and internal credential management.
- Avoid deploying signup, anti-detect, or trial automation components as part of the production API gateway.
