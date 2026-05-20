# Kiro/Windsurf Recheck - 2026-05-20

This note records the latest upstream recheck and the live server probe. Keep
all raw access tokens, refresh tokens, passwords, API keys, cookies, and SSH
secrets out of this file.

## Short Conclusion

- The live server is in the US and the Kiro account refresh/profile data is
  valid enough for Kiro usage and open-model requests.
- The current blocker is not proven to be "US server cannot use Kiro". The
  hard evidence is narrower: official Kiro model discovery for the imported
  credential returns only five non-Claude models, and official generate calls
  reject Claude-family model ids with `INVALID_MODEL_ID`.
- The user clarified that the desired Kiro path is the old IDE/refresh-token
  method, not Kiro `ksk_...` API-key import. Re-testing that old path with the
  local fixed `machineId` still leaves the server-side result unchanged:
  `qwen3-coder-next` succeeds, while Opus/Sonnet/Haiku return
  `INVALID_MODEL_ID`.
- Windsurf's current deployed adapter is still aligned with the newest public
  reference. No source/image update was found in the checked upstreams.
- A follow-up GitHub workaround scan tested `pi-kiro` and `open-kiro` style
  `KIRO_CLI`/AmazonQ-For-CLI request fingerprints on the live server. They did
  not unlock Claude/Opus for the current credential. See
  `planning/KIRO_GITHUB_WORKAROUND_SCAN_2026-05-20.md`.
- Kiro needs one of two next inputs before Claude/Sonnet/Opus should be exposed
  through Sub2API:
  1. a fresh export from the exact IDE/CLI environment that can use Claude; or
  2. a Kiro API key (`ksk_...`) from the Pro account, which is now the official
     headless/server auth path.

## Upstream Version Check

Checked with `git ls-remote` on 2026-05-20:

| Project | Latest HEAD | Latest tag/role | Result |
| --- | --- | --- | --- |
| `dwgx/WindsurfAPI` | `c028576a56b9fa19f84810643610cae4af824238` | `v2.0.96` | Same as deployed reference |
| `Jwadow/kiro-gateway` | `a5292ca04c7c6231e0b47673ac3f981f5a706e1e` | `v2.3` | Same as deployed reference |
| `hank9999/kiro.rs` | `f1bbe9f1d14b962211592c661792d48a9855e32e` | `v2026.3.1` | Same as local mirror |
| `chaogei/Kiro-account-manager` | `7ad57fd26e67b3ea91b780b2ca983c78737ed88a` | `v1.6.6` | Same as local mirror |

## Live Server Kiro Probe

Server path checked:

```text
/opt/sub2api/kiro-gateway/creds/kiro-auth-token.json
```

Sanitized credential facts:

- email label is present;
- refresh token is present;
- access token was present;
- profile ARN is present and points to the Kiro social profile;
- no `machineId`, `provider`, `clientId`, or `clientSecret` is present in the
  current deployed credential file.

Official model discovery result from the server:

```text
GET https://q.us-east-1.amazonaws.com/ListAvailableModels
origin=AI_EDITOR
profileArn=<social-profile-arn>
```

Returned models:

```text
deepseek-3.2
minimax-m2.5
minimax-m2.1
glm-5
qwen3-coder-next
```

Direct generate smoke using the same access token:

| Endpoint | Model | Result |
| --- | --- | --- |
| `codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse` | `qwen3-coder-next` | HTTP 200 |
| `q.us-east-1.amazonaws.com/generateAssistantResponse` | `qwen3-coder-next` | HTTP 200 |
| `runtime.us-east-1.kiro.dev/generateAssistantResponse` | `qwen3-coder-next` | HTTP 200 |
| all three endpoints | `claude-sonnet-4.6` | HTTP 400 `INVALID_MODEL_ID` |
| all three endpoints | `claude-opus-4.6` | HTTP 400 `INVALID_MODEL_ID` |

`runtime.us-east-1.kiro.dev/ListAvailableModels` returned
`UnknownOperationException`, matching `kiro-gateway`'s note that runtime does
not provide dynamic model listing.

## Important External Signals

- Kiro's official CLI docs now document API-key/headless operation. The API key
  path is meant for non-browser/server automation and consumes subscription
  credits.
- Kiro's official model changelog says some new model availability is segmented
  by auth method. For example, newer Opus support may arrive first for IAM
  Identity Center while Google/GitHub/Builder ID support can lag.
- A public Kiro issue reports the exact five-model result seen here for a Pro
  account. That does not prove the user's working IDE cannot use Claude; it does
  prove this symptom exists in the official Kiro model selector and should be
  treated as an account/auth-context issue, not a Sub2API routing bug.

References:

- `https://kiro.dev/docs/cli/authentication/`
- `https://kiro.dev/docs/cli/headless/`
- `https://kiro.dev/changelog/models/`
- `https://github.com/kirodotdev/Kiro/issues/8055`
- `https://github.com/chaogei/Kiro-account-manager`
- `https://github.com/Jwadow/kiro-gateway`
- `https://github.com/hank9999/kiro.rs`
- `https://github.com/dwgx/WindsurfAPI`

## What This Means For Sub2API

Keep the public Sub2API group conservative:

- continue exposing Windsurf models that have already passed public smoke;
- continue exposing Kiro open models that have already passed public smoke;
- do not expose Kiro Claude/Sonnet/Opus until official discovery or direct smoke
  from the same credential succeeds.

If Sub2API exposed Claude-family Kiro names now, external clients would receive
400s from upstream. That would look like a broken public API key even though
the Sub2API scheduler itself is behaving correctly.

## Next Kiro Upgrade Path

1. Prefer official Kiro API key import for server use.

   Generate a Kiro API key from the Pro account and import it in Sub2API admin
   as Kiro `API Key` mode. This is the cleanest current server path because
   Kiro's own docs target API keys at headless automation.

2. If using IDE/CLI token export, export from the exact working environment.

   Best sources:

   ```text
   ~/.local/share/kiro-cli/data.sqlite3
   ~/.aws/sso/cache/kiro-auth-token.json
   related ~/.aws/sso/cache/<clientIdHash>.json registration file
   Kiro-account-manager export including machineId/provider/authMethod/accountId
   ```

   The current JSON import contains profile and tokens, but not the machine or
   client registration context that newer Kiro tooling tracks.

3. Re-run the server probe before changing public model exposure.

   ```bash
   python3 tools/kiro_server_probe.py \
     --creds /opt/sub2api/kiro-gateway/creds/kiro-auth-token.json
   ```

   The probe prints token hashes, model ids, status codes, and short error
   bodies only. It does not print raw tokens.

4. Only then update Sub2API mappings.

   Acceptance gate for any Kiro Claude model:

   ```text
   official ListAvailableModels contains it OR direct runtime smoke returns 200
   adapter smoke returns 200
   public Sub2API /v1/messages smoke returns 200
   ```

## Windsurf Status

No immediate Windsurf code update is needed. `dwgx/WindsurfAPI v2.0.96` already
contains the newer Devin-session-token path:

```text
auth1 password/login -> WindsurfPostAuth -> devin-session-token$ as apiKey
```

Important operational notes:

- if email/password import fails for an OAuth-created Windsurf account, use the
  token path from the Windsurf dashboard/backup-token URL;
- current Windsurf model exposure should continue to be based on dashboard
  capability/probe results and public Sub2API smoke;
- deprecated failures such as `gpt-4o-mini` and `grok-3-mini` should stay
  pruned even if static model catalogs still mention them.
