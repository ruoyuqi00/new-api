# Upstream and protocol check - 2026-06-15

## Summary

Checked the previously referenced projects and the deployed Sub2API stack on 2026-06-15.

Result:

- No new required protocol patch was found for the deployed Sub2API gateway.
- `upstream/main` only added `e34ad2b1 chore: sync VERSION to 0.1.136 [skip ci]` after the last deployed upstream merge.
- The protocol/runtime fixes from upstream `v0.1.136` were already included in the 2026-06-10 deployment because `v0.1.136` points at `0acf00c4`, which was merged previously.
- Local `main` now includes the version-sync merge commit `89fb8997`.
- Server was not restarted for this check because the only new upstream change was version metadata, not gateway behavior.

## Sources checked

- Sub2API upstream: `https://github.com/Wei-Shaw/sub2api`
- Kiro-Go reference: `https://github.com/Quorinex/Kiro-Go`
- Cockpit tools reference: `https://github.com/jlcodes99/cockpit-tools`
- Additional notes from the local reference list:
  - `https://github.com/TheSmallHanCat/flow2api`
  - `https://github.com/basketikun/chatgpt2api`
  - `https://github.com/FakeOAI/tokens`

The old `432539/gpt2api` reference could not be resolved by `git ls-remote`; it appears unavailable or renamed from this environment.

## Sub2API upstream status

Last deployed image:

```text
sub2api-provider-adapters:upstream-merge-20260610-147d5515
```

Latest upstream delta after that deploy:

```text
e34ad2b1 chore: sync VERSION to 0.1.136 [skip ci]
```

Changed file:

```text
backend/cmd/server/VERSION
```

Protocol impact: none.

## Kiro protocol status

Current deployed Kiro Web adapter protocol remains:

- `KiroWebPortalService` RPC v2 CBOR path.
- `smithy-protocol: rpc-v2-cbor`.
- `ListAvailableModels?origin=KIRO_CONSOLE` for Claude model visibility.
- `CreateSpace`, then `StreamSendMessage`.
- `sessionId = spaceId` for `StreamSendMessage`.
- Session consistency across IdP, CSRF token, `UserId` cookie, and `profileArn`.

This still matches the path documented and implemented in `adapters/kiro-web/kiro_web_adapter.py`.

Kiro-Go was checked as a reference. It is useful for UI/account-management shape and has richer local gateway translation logic, including tool-related transforms, but it is not a direct replacement for our current Kiro Web Portal adapter path. The local Kiro-Go checkout still tracks `a2e3971 fix: improve update check UI consistency`; no newer remote head was found during this check.

Important limitation:

- The Sub2API gateway has broad Anthropic Messages / Responses / tool-use compatibility tests.
- The Kiro Web adapter itself is still text-first and does not claim full Anthropic tool_use or image-input parity.
- Therefore, Kiro text/chat protocol is OK for the currently wired use case, but Kiro Web should not be advertised as a fully compatible Claude tools endpoint until an explicit tool round-trip implementation is added and live-tested.

## Windsurf / Cockpit / account-tool references

No direct Windsurf protocol update was found that needs to be merged into this Sub2API deployment.

Cockpit-tools is still useful as an operational reference for local account management and import flows, but the observable remote refs look like release/channel churn rather than a server-side protocol change we should merge into Sub2API.

## Validation performed

Local protocol-focused tests:

```bash
go test ./internal/pkg/apicompat ./internal/pkg/antigravity ./internal/service -run "Responses|Tool|tool|Anthropic|Bedrock|Codex|OpenAI|Image|Kiro|Gemini|Forward"
```

Result:

```text
ok github.com/Wei-Shaw/sub2api/internal/pkg/apicompat
ok github.com/Wei-Shaw/sub2api/internal/pkg/antigravity
ok github.com/Wei-Shaw/sub2api/internal/service
```

Kiro Web adapter syntax check:

```bash
python -m py_compile adapters/kiro-web/kiro_web_adapter.py
```

Result:

```text
OK
```

Server health:

```text
sub2api-provider-adapters:upstream-merge-20260610-147d5515,running,healthy
https://api.vyywcw.cn/health -> HTTP 200 {"status":"ok"}
```

## Recommendation

No urgent deploy is required for protocol correctness.

Optional cleanup:

- Rebuild and redeploy later with `VERSION=0.1.136` if the displayed server version should match the upstream tag exactly.
- Add a dedicated Kiro Web tool-use implementation only if Kiro accounts need to serve Claude Code-style tools through `/v1/messages`.
