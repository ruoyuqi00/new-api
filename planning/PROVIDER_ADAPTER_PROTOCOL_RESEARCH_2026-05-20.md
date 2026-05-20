# Provider Adapter Protocol Research - 2026-05-20

This note records the current Windsurf/Kiro protocol research. Do not commit
account passwords, access tokens, refresh tokens, API keys, cookies, SSH
passwords, or raw `.env` values.

## Why This Round Happened

The same account can work in the official IDE while a server adapter fails. That
usually means the problem is not only account permission; it may be one of these
layers:

- exported credential shape is incomplete or stale;
- IDE/CLI now stores auth in a different local file or SQLite database;
- the adapter sends a stale endpoint, header, model id, origin, or machine id;
- the provider gates advanced models by account, region, or network exit.

## References Checked

Windsurf:

- `dwgx/WindsurfAPI`
  - GitHub: `https://github.com/dwgx/WindsurfAPI`
  - Local mirror: `D:\wflogin\_github_research\WindsurfAPI-dwgx`
  - Observed HEAD: `c028576a56b9fa19f84810643610cae4af824238`
  - Observed tag: `v2.0.96`
  - Role: current deployed internal Windsurf adapter.
- `jlcodes99/cockpit-tools`
  - GitHub: `https://github.com/jlcodes99/cockpit-tools`
  - Local mirror: `D:\wflogin\_github_research\cockpit-tools`
  - Observed HEAD: `f6c92cbbdd86357405a0589ee0a09bae855d26e2`
  - Role: reference for updated Windsurf Devin Auth, account switching, quota
    refresh, and IDE state injection.
- `pfcoperez/windsurfinabox`
  - GitHub: `https://github.com/pfcoperez/windsurfinabox`
  - Local mirror: `D:\wflogin\_github_research\windsurfinabox`
  - Observed HEAD: `86a7da7821413497756bc53f85508eeeb8b18945`
  - Role: weaker reference for headless Windsurf IDE token use, not a primary
    server proxy.

Kiro:

- `chaogei/Kiro-account-manager`
  - GitHub: `https://github.com/chaogei/Kiro-account-manager`
  - Local mirror: `D:\wflogin\_github_research\Kiro-account-manager`
  - Observed HEAD/tag: `7ad57fd26e67b3ea91b780b2ca983c78737ed88a`,
    `v1.6.6`
  - Role: strongest current reference for Kiro OAuth, Kiro CLI SQLite, account
    model discovery, quota query, machine id handling, and proxy runtime.
- `Jwadow/kiro-gateway`
  - GitHub: `https://github.com/Jwadow/kiro-gateway`
  - Local mirror: `D:\wflogin\_github_research\kiro-gateway`
  - Observed HEAD/tag: `a5292ca04c7c6231e0b47673ac3f981f5a706e1e`,
    `v2.3`
  - Role: current deployed Kiro runtime adapter for open-model smoke.
- `hank9999/kiro.rs`
  - GitHub: `https://github.com/hank9999/kiro.rs`
  - Local mirror: `D:\wflogin\_github_research\kiro.rs-latest`
  - Observed HEAD/tag: `f1bbe9f1d14b962211592c661792d48a9855e32e`,
    `v2026.3.1`
  - Role: Rust admin/import and IDE endpoint reference.

## Windsurf Findings

`dwgx/WindsurfAPI v2.0.96` already contains the important 2026-04/2026-05
Devin migration workaround. Its release notes for `v2.0.90` state that
`GetOneTimeAuthToken` became unreliable and that the working path is:

```text
auth1 password/login -> WindsurfPostAuth -> devin-session-token$
```

The session token can be used directly as the Windsurf API key. This means we do
not need to reimplement the old OTT/RegisterUser chain in Sub2API before testing
new Windsurf accounts.

`cockpit-tools` confirms the newer Devin chain in code:

```text
email/password
  -> /_devin-auth/password/login
  -> auth1_
  -> WindsurfPostAuth
  -> devin-session-token$
  -> GetCurrentUser for IDE status metadata
```

Short-term Windsurf action:

1. Keep `dwgx/WindsurfAPI` as the internal adapter.
2. Import accounts with token formats that WindsurfAPI now accepts:
   `sk-ws-*`, `auth1_*`-derived login, or `devin-session-token$*`.
3. If email/password import fails in Sub2API, use the native
   `/windsurf-dashboard` login path or paste a token from
   `https://windsurf.com/show-auth-token`.
4. After import, sync Sub2API model exposure from WindsurfAPI dashboard
   capability results, then prune any direct-smoke failures.

Server update check on 2026-05-20:

- `docker compose pull windsurf-api` completed.
- `docker compose up -d windsurf-api` kept the existing container running.
- No public host port is exposed for `windsurf-api`; it stays behind Sub2API.

## Kiro Findings

The strongest new Kiro reference is `Kiro-account-manager`, not
`kiro-gateway`. Important differences it shows:

- official model discovery uses `q.<region>.amazonaws.com/ListAvailableModels`;
- `profileArn` is passed to model discovery;
- Kiro CLI auth can be stored in `~/.local/share/kiro-cli/data.sqlite3`;
- social login and IdC login are handled differently;
- social login falls back to the social profile ARN when missing;
- request `x-amzn-kiro-agent-mode` is `spec` for non-IdC/social and `vibe`
  for IdC/CLI-like auth;
- CodeWhisperer endpoint requests resolve user-facing model names through the
  official model list rather than hardcoding only dotted ids.

Server observations on 2026-05-20:

- Current server Kiro credential shape is social login with profile ARN present.
- Direct official `ListAvailableModels` from the server returned only:
  - `deepseek-3.2`
  - `minimax-m2.5`
  - `minimax-m2.1`
  - `glm-5`
  - `qwen3-coder-next`
- The same call did not return Claude/Sonnet/Opus models.
- Direct server calls to `q.<region>.amazonaws.com/generateAssistantResponse`
  with `claude-opus-4.7`, `claude-opus-4.6`, and `claude-sonnet-4.6`
  returned `INVALID_MODEL_ID`.
- Changing `x-amzn-kiro-agent-mode` between `spec` and `vibe`, and testing
  `AI_EDITOR`/`MD_IDE` origins, did not make Claude-family models appear in the
  official model list.
- `qwen3-coder-next` still succeeds from the same server credential.

Current Kiro conclusion:

The public Kiro adapter should not expose Claude/Sonnet/Opus yet. The official
model discovery result seen from the US server does not include those models for
the current exported credential. This points to one of two likely causes:

1. the exported credential is not equivalent to the official IDE/CLI state that
   can use Claude locally;
2. Kiro gates advanced models by network exit, region, machine identity, or a
   related runtime signal.

## Kiro Upgrade Plan

Stage 1: credential parity test.

Ask for a fresh export from the exact machine where Kiro IDE can use Opus/Sonnet.
Preferred sources, in order:

1. Kiro CLI SQLite database:
   `~/.local/share/kiro-cli/data.sqlite3`
2. Kiro IDE SSO cache:
   `~/.aws/sso/cache/kiro-auth-token.json` plus any related hashed client
   registration JSON file.
3. A `Kiro-account-manager` export that includes refresh token, access token,
   profile ARN, provider/auth method, region, client id/client secret when
   present, and machine id when available.

Acceptance check:

- On the server, import into a temporary internal Kiro adapter.
- Run official `ListAvailableModels`.
- Only if Claude/Sonnet/Opus appear there, smoke `claude-opus-4.7` and
  `claude-sonnet-4.6`.
- Only after public Sub2API smoke passes, add them to `provider-mixed`.

Stage 2: adapter patch.

Patch our Kiro path using `Kiro-account-manager` as the current behavior
reference:

- preserve `provider`, `authMethod`, `machineId`, `clientId`, `clientSecret`,
  `profileArn`, `region`, `accessToken`, and `refreshToken`;
- add official `ListAvailableModels` discovery and cache;
- select `spec` vs `vibe` by auth method instead of hardcoding one mode;
- try `q.<region>.amazonaws.com` and CodeWhisperer-compatible endpoint behavior
  before relying on `runtime.<region>.kiro.dev`;
- add a smoke command that prints model ids and HTTP status only, never tokens.

Stage 3: network-exit test.

If Stage 1 still returns only the five open models from the US VPS:

- test a proxy exit matching the user's working IDE environment;
- test a US/EU residential-quality proxy if available;
- configure the adapter proxy setting instead of exposing the adapter publicly;
- rerun `ListAvailableModels` before enabling any advanced model.

## Rules For Public Exposure

- Sub2API remains the only public API surface.
- Native adapter dashboards may be linked from Sub2API admin, but adapter API
  keys/passwords remain separate.
- Do not add Kiro Claude/Sonnet/Opus models to Sub2API until direct official
  discovery, direct adapter smoke, and public Sub2API smoke all pass.
- Do not expose model names just because a static fallback list contains them.
  Public models must be synced from observed account capability or smoke.
