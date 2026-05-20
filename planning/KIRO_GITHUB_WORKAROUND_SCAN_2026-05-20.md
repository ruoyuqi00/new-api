# Kiro GitHub Workaround Scan - 2026-05-20

This document records the GitHub scan for Kiro Claude/Opus
`INVALID_MODEL_ID` workarounds. Do not put raw access tokens, refresh tokens,
passwords, cookies, SSH credentials, or API keys in this file.

## Goal

Find whether newer GitHub projects have a working workaround for the current
server symptom:

- current Kiro credential refreshes successfully;
- Kiro model discovery returns only:
  `deepseek-3.2`, `minimax-m2.5`, `minimax-m2.1`, `glm-5`,
  `qwen3-coder-next`;
- direct Claude/Opus requests return `INVALID_MODEL_ID`.

## GitHub Projects Checked

| Project | Latest observed version | Relevant idea | Result |
| --- | --- | --- | --- |
| `hongyilyu/pi-kiro` | `438327370eb86a35ba7ac89aa1249e3c2a18ec85`, tag `v0.1.3` | Kiro as a pi provider; uses `origin=KIRO_CLI`, Amazon Q CLI user-agent, Builder ID / IdC OIDC login, and Kiro dot-form model IDs | Implemented in probe and tested; current server token still only lists five models and rejects Claude/Opus |
| `hueyexe/open-kiro` | Go module `v0.0.0-20260324032827-cf5db84025da` | OpenAI/Anthropic proxy; uses Kiro CLI/AmazonQ-For-CLI fingerprint, POST to `q.us-east-1.amazonaws.com/`, and `AmazonCodeWhispererService.ListAvailableModels` | Implemented in probe and tested; current server token still only lists five models and rejects Claude/Opus |
| `chaogei/Kiro-account-manager` | `7ad57fd26e67b3ea91b780b2ca983c78737ed88a`, tag `v1.6.6` | Exact model matching, dynamic `ListAvailableModels`, full machine/account metadata preservation | Latest already checked; server has fixed `machineId`; model list still only five |
| `Jwadow/kiro-gateway` | `a5292ca04c7c6231e0b47673ac3f981f5a706e1e`, tag `v2.3` | Dynamic model cache and Kiro gateway runtime | Already deployed/reference checked; same five-model symptom with current credential |
| `jlcodes99/cockpit-tools` | local mirror inspected | Useful for shared account/runtime ideas; `MODEL_PLACEHOLDER_M26` is not a Kiro upstream model solution | Tested placeholder/alias idea; Kiro upstream rejects it |

References:

- `https://github.com/hongyilyu/pi-kiro`
- `https://pkg.go.dev/github.com/hueyexe/open-kiro/proxy`
- `https://github.com/chaogei/Kiro-account-manager`
- `https://github.com/Jwadow/kiro-gateway`
- `https://github.com/kirodotdev/Kiro/issues/8055`
- `https://kiro.dev/docs/cli/models/`
- `https://kiro.dev/changelog/models/`

## What Was Tested On The Server

The probe script now supports multiple Kiro request fingerprints:

- `ide`: existing Kiro IDE-style JS AWS SDK headers, `origin=AI_EDITOR`;
- `pi-cli`: `pi-kiro` style `origin=KIRO_CLI` plus AmazonQ-For-CLI Rust UA;
- `open-kiro`: `open-kiro` style POST to the Q service root with
  `AmazonCodeWhispererService.ListAvailableModels`;
- `kam-amazonq`: Kiro-account-manager's AmazonQCLI experiment shape,
  `SendMessageStreaming`.

Live results with the current imported Kiro credential:

| Style | Model list result | Generate result |
| --- | --- | --- |
| `open-kiro` | same five models | `qwen3-coder-next` HTTP 200; Claude/Opus/Haiku HTTP 400 `INVALID_MODEL_ID` |
| `pi-cli` | same five models | `qwen3-coder-next` HTTP 200; Claude/Opus/Haiku HTTP 400 `INVALID_MODEL_ID` |
| hidden/alias IDs | not listed | `claude-opus-4.7`, `claude-opus-4.6-1m`, `claude-sonnet-4.6-1m`, `claude-opus-4-20250918`, `MODEL_PLACEHOLDER_M26`, and `auto` all rejected |
| `kam-amazonq` | same five models | current generated payload is rejected as `Improperly formed request`; not a direct fix path for Sub2API |

## Interpretation

The GitHub scan did not find a model-name-only or header-only workaround for
the current server credential.

The most likely remaining variables are:

1. auth context: social/GitHub/Google/Builder ID/IdC can receive different model
   availability;
2. local runtime context: the previously working local runtime used a local
   proxy and the exact local Kiro environment;
3. account-side rollout/entitlement: public GitHub issue reports the same
   five-model result for a Kiro Pro-style account, so this symptom is not unique
   to our adapter.

## Actionable Next Steps

1. Keep the public Kiro exposure conservative.

   Do not expose Kiro Claude/Opus through Sub2API until one of the direct server
   probes returns HTTP 200 for a Claude-family model.

2. Preserve richer Kiro credential metadata.

   Keep these fields when available:

   ```text
   machineId
   authMethod
   provider
   clientId
   clientSecret
   region / apiRegion / authRegion
   profileArn
   ```

   This helps test IdC/Builder-ID exports later and avoids collapsing all auth
   paths into a minimal social refresh-token JSON.

3. If the user can currently use Opus in the official Kiro IDE/CLI, export from
   that exact runtime again.

   Best sources to compare, without committing secrets:

   ```text
   ~/.local/share/kiro-cli/data.sqlite3
   ~/.aws/sso/cache/*.json
   Kiro-account-manager full export JSON
   local proxy/runtime settings
   ```

4. Test a fresh Builder ID or IdC OIDC login flow.

   `pi-kiro` and `open-kiro` both implement Builder ID/IdC flows. A fresh OIDC
   credential may produce a different model list than the current social token.
   This requires browser authorization by the account owner.

5. Continue using Windsurf for public Claude/Opus while Kiro remains limited.

   Current public Opus/Sonnet success is coming from the Windsurf route, so
   Sub2API can still expose Claude models through Windsurf-backed groups while
   keeping Kiro's open models separate.

