# Provider Reference Protocol Scan - 2026-05-21

This scan checks whether the reference projects have protocol-related updates
that should be pulled into our Sub2API provider adapters before official account
import or token refresh breaks. Do not place raw tokens, passwords, cookies, API
keys, or SSH credentials in this file.

## Summary

No deployed adapter update is required from this scan.

The three primary runtime references stayed at the previously recorded versions:

- `dwgx/WindsurfAPI`: still `c028576a56b9fa19f84810643610cae4af824238`, tag `v2.0.96`.
- `guanxiaol/WindsurfPoolAPI`: still `a8d2f4cf0c4c36d021debfe0428ec497660c55e6`, tag line `v2.0.7`.
- `hank9999/kiro.rs`: still `f1bbe9f1d14b962211592c661792d48a9855e32e`, tag `v2026.3.1`.

`jlcodes99/cockpit-tools` did update from the previously observed
`f6c92cbbdd86357405a0589ee0a09bae855d26e2` to
`2b148437ef19812ffbea50d62ccc5f52a47caaf2`, with latest tag `v0.24.3`.
Its changed files are Antigravity/Codex path detection, local API proxy routing,
and UI plumbing. Its Windsurf and Kiro account/protocol modules did not change in
this update window.

## Reference Matrix

| Project | Current observed state | Protocol-relevant change? | Impact |
| --- | --- | --- | --- |
| `dwgx/WindsurfAPI` | `master`/`HEAD` = `c028576a`, tag `v2.0.96` | No new commit/tag | Keep current Windsurf internal adapter. |
| `guanxiaol/WindsurfPoolAPI` | `main`/`HEAD` = `a8d2f4c`, tag `v2.0.7` | No new commit/tag | Keep as comparison only; not better than `dwgx/WindsurfAPI` for our deployment. |
| `hank9999/kiro.rs` | `master`/`HEAD` = `f1bbe9f`, tag `v2026.3.1` | No new master/tag | Keep as older Kiro IDE/API-key route reference. |
| `hank9999/kiro.rs` refactor branches | `refactor/kiro-endpoint` = `3d516f3`; `refactor/v2` = `4d792b0` | Branches fetched and inspected | Endpoint layering, retry, pool, and storage refactors; no Kiro Web Portal route. Do not replace our working Web Portal adapter with these branches. |
| `Jwadow/kiro-gateway` | `main`/`HEAD` = `a5292ca`, tag `v2.3` | No new commit/tag | Keep current older gateway for open-model path only. |
| `tickernelz/opencode-kiro-auth` | `master`/`HEAD` = `d0d9b18`, tag `v1.10.1` | No new commit/tag | Still useful for Kiro CLI/Builder ID/IdC login ideas, not for our Web Portal route. |
| `hongyilyu/pi-kiro` | `master`/`HEAD` = `43832737`, tag `v0.1.3` | No new commit/tag | Still CLI-style route. |
| `chaogei/Kiro-account-manager` | `main`/`HEAD` = `7ad57fd`, tag `v1.6.6` | No new commit/tag, but deeper inspection completed | Strongest Kiro IDE/Amazon Q account-pool reference: round-robin/sticky, circuit breaker, token refresh lock, endpoint fallback, model discovery. Reimplement ideas rather than copying AGPL code. |
| `Quorinex/Kiro-Go` | `main`/`HEAD` = `68110f3`, version `1.0.8` | New reference added | Go service reference for weighted round-robin, model-aware account routing, usage/overage handling, Docker deployment. `go test ./...` passed locally. |
| `jlcodes99/cockpit-tools` | `main`/`HEAD` = `2b14843`, tag `v0.24.3` | Yes, but not Windsurf/Kiro provider protocol | Local mirror fast-forwarded. No deployed adapter change needed. |

## Windsurf Findings

`dwgx/WindsurfAPI` remains the best short-term upstream for our Windsurf adapter.
No new upstream protocol change was found after `v2.0.96`.

Protocol-sensitive areas to keep watching:

- login/import: `src/auth.js`, `src/connect.js`, dashboard login handlers;
- account refresh and tier detection: `src/auth.js`;
- language server protocol: `src/windsurf.js`, `src/proto.js`, `src/grpc.js`;
- runtime path and LS bootstrap: `src/langserver.js`, `install-ls.sh`;
- chat conversion: `src/client.js`, `src/handlers/chat.js`, `src/handlers/messages.js`;
- model catalog and entitlement mapping: `src/models.js`, cloud-model merge logic.

Current action:

- Do not redeploy Windsurf just because of this scan.
- If import breaks later while the official IDE still works, first compare the
  latest `WindsurfAPI` tag against `v2.0.96`, then inspect the files above.
- Also verify the deployed Language Server binary is current enough, because
  Windsurf model discovery depends heavily on the LS binary and cloud catalog.

## Kiro Findings

The old Kiro IDE/API-key route and the working Kiro Web Portal route are still
separate paths:

- `kiro.rs`, `kiro-gateway`, `opencode-kiro-auth`, `pi-kiro`, and
  `Kiro-account-manager` still focus on Kiro IDE / CLI / CodeWhisperer /
  Amazon Q style routes.
- None of the checked references added the Web Portal sequence that currently
  works for our Claude/Opus path:
  `GetUserInfo -> GetUserUsageAndLimits -> CreateSpace -> StreamSendMessage`,
  with `sessionId = spaceId`.
- The `kiro.rs` refactor branches are not a drop-in upgrade for our server.
  They improve layering and retry/pool behavior for the IDE endpoint, but they
  do not replace the Web Portal adapter we built.

Current action:

- Keep `adapters/kiro-web/kiro_web_adapter.py` as the Claude/Opus source of
  truth for Kiro traffic through Sub2API.
- Keep `kiro-gateway` only for the older open-model path.
- Do not switch the deployed Kiro traffic back to `kiro.rs` or `kiro-gateway`
  for Claude/Opus unless those projects add Web Portal support or our adapter
  fails a fresh smoke test.
- Borrow KAM/Kiro-Go routing ideas for the next internal Kiro runtime upgrade:
  account state, model-aware routing, refresh single-flight, quota/cooldown
  handling, and sticky session affinity.

## Cockpit-tools Delta

`cockpit-tools` changed since the previous scan. The meaningful update areas are:

- Antigravity legacy versus Antigravity IDE path and version detection;
- Antigravity account switching guard for version `2.0.0+`;
- Codex local API service proxy routing and diagnostics;
- Codex missing-refresh-token reauth handling;
- UI platform grouping and translation updates.

The update did not touch these provider-protocol files:

- `src-tauri/src/modules/windsurf_account.rs`
- `src-tauri/src/modules/windsurf_oauth.rs`
- `src-tauri/src/modules/windsurf_instance.rs`
- `src-tauri/src/modules/kiro_account.rs`
- `src-tauri/src/modules/kiro_oauth.rs`
- `src-tauri/src/modules/kiro_instance.rs`
- `src/pages/WindsurfAccountsPage.tsx`
- `src/pages/KiroAccountsPage.tsx`
- `src/stores/useWindsurfAccountStore.ts`
- `src/stores/useKiroAccountStore.ts`

So there is no Windsurf/Kiro account-import patch to borrow from this update.

## Recheck Commands

Use these when the user asks for the next protocol update check:

```powershell
git ls-remote https://github.com/dwgx/WindsurfAPI.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/guanxiaol/WindsurfPoolAPI.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/hank9999/kiro.rs.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/Jwadow/kiro-gateway.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/tickernelz/opencode-kiro-auth.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/hongyilyu/pi-kiro.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/chaogei/Kiro-account-manager.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/Quorinex/Kiro-Go.git HEAD refs/heads/* refs/tags/*
git ls-remote https://github.com/jlcodes99/cockpit-tools.git HEAD refs/heads/* refs/tags/*
```

For local mirrors:

```powershell
git fetch --tags origin
git log --oneline HEAD..origin/master
git log --oneline HEAD..origin/main
```

For `kiro.rs`, fetch non-default branches too:

```powershell
git fetch origin '+refs/heads/*:refs/remotes/origin/*'
git branch -r
git diff --name-only origin/master..origin/refactor/kiro-endpoint
git diff --name-only origin/master..origin/refactor/v2
```

For `cockpit-tools`, protocol-impact filter:

```powershell
git diff --name-only HEAD..origin/main -- `
  src-tauri/src/modules/windsurf_account.rs `
  src-tauri/src/modules/windsurf_oauth.rs `
  src-tauri/src/modules/windsurf_instance.rs `
  src-tauri/src/modules/kiro_account.rs `
  src-tauri/src/modules/kiro_oauth.rs `
  src-tauri/src/modules/kiro_instance.rs `
  src/pages/WindsurfAccountsPage.tsx `
  src/pages/KiroAccountsPage.tsx `
  src/stores/useWindsurfAccountStore.ts `
  src/stores/useKiroAccountStore.ts
```

## Sources

- https://github.com/dwgx/WindsurfAPI
- https://github.com/guanxiaol/WindsurfPoolAPI
- https://github.com/hank9999/kiro.rs
- https://github.com/Jwadow/kiro-gateway
- https://github.com/tickernelz/opencode-kiro-auth
- https://github.com/hongyilyu/pi-kiro
- https://github.com/chaogei/Kiro-account-manager
- https://github.com/Quorinex/Kiro-Go
- https://github.com/jlcodes99/cockpit-tools
