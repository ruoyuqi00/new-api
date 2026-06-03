# Sub2API upstream merge and deployment - 2026-06-03

## Scope

The goal was to pull the latest official `Wei-Shaw/sub2api` `main`, include the
new protocol changes, redeploy the private server, verify behavior, and push the
private branch.

The directory `D:\wflogin\注册机相关项目` was intentionally not inspected.

## Upstream merge

- Official upstream branch: `upstream/main`
- Upstream head merged: `aa69e394`
- Local merge commit: `0ae8d71f`

Notable upstream changes included:

- Responses API to Chat Completions bridge redesign.
- Chat Completions to Responses bridge support.
- Anthropic Messages API compatibility updates.
- Antigravity Gemini/Codex related fixes.
- WS Codex image bridge fixes.
- apicompat streaming tests.

After another `git fetch --all --prune`, `upstream/main` had no newer commits
to merge. Only the upstream `cla-signatures` branch had moved.

## Local recovery fix

The private deployment previously treated OpenAI OAuth `401` responses such as
`token_invalidated`, `token_revoked`, and `{"detail":"Unauthorized"}` as a
permanent account failure. That is too aggressive for CPA/imported OpenAI OAuth
accounts that still have a `refresh_token`.

Changes made:

- Added `UpdateCredentialFields` in `account_repo.go` to patch selected JSONB
  credential fields without rewriting the full credentials document.
- Updated `RateLimitService` so refreshable OpenAI OAuth `401` responses:
  - invalidate the token cache,
  - set `credentials.expires_at` to the past,
  - bump `_token_version`,
  - temporarily unschedule the account,
  - avoid permanent `SetError` unless the account has no usable
    `refresh_token`.
- Updated passthrough and unit-tag tests for the new behavior.

This fix prevents a transient or refreshable OAuth `401` from destroying account
state, but it cannot recover an account whose refresh token has already been
rotated elsewhere.

## Deployment

Image built locally:

- `sub2api-provider-adapters:upstream-merge-20260603b`

Image archive:

- `D:\wflogin\sub2api-provider-adapters-upstream-merge-20260603b-image.tar`
- SHA-256:
  `3aeea6db92092c7c1364a6332913ecdca58ac358752f08c2470fd848fab2ae2e`

Server:

- Host: `154.219.122.197`
- App path: `/opt/sub2api`
- Public URL: `https://api.vyywcw.cn`

Actions:

- Uploaded and loaded the image on the server.
- Updated compose to run
  `sub2api-provider-adapters:upstream-merge-20260603b`.
- Restarted `sub2api`.
- Cleared Redis access-token cache entries for GPT5.5 OpenAI OAuth accounts.
- Cleared GPT5.5 OpenAI scheduler cache entries.

Health checks:

- `http://127.0.0.1:8080/health`: OK
- `https://api.vyywcw.cn/health`: OK

## Verification

Passed:

```powershell
go test ./internal/service -run TestOpenAIGatewayService_OpenAIPassthrough_AccountPoolErrorsTriggerFailover
go test ./internal/repository ./internal/pkg/apicompat
go test ./internal/service
go test ./internal/handler/admin ./internal/handler
```

Blocked by an existing unrelated unit-test compile issue:

```powershell
go test -tags unit ./internal/service -run TestRateLimitService_HandleUpstreamError_OpenAITokenInvalidatedWithRefreshTokenRecovers
```

Failure:

- `internal/service/openai_account_runtime_block_fastpath_test.go`
  currently has an unused/undefined `passthroughAccount` symbol under the
  `unit` build tag.

## Live smoke result

Using an existing GPT5.5 API key on the server:

- `/v1/chat/completions`, model `gpt-5.5`: HTTP 502
- `/v1/responses`, model `gpt-5.5`: HTTP 502
- `/v1/messages`, model `claude-opus-4-8`: HTTP 502

The deployment itself is healthy, but the upstream accounts selected by the
GPT5.5 group are not currently callable.

## CPA account finding

GPT5.5 group state after deployment and cache cleanup, before the final smoke:

- OpenAI OAuth accounts: 251 total
- Active OpenAI OAuth accounts: 136
- Error OpenAI OAuth accounts: 115
- DB-schedulable OpenAI OAuth accounts: 129

After the final smoke and background refresh pass:

- OpenAI OAuth accounts: 251 total
- Active OpenAI OAuth accounts: 48
- Error OpenAI OAuth accounts: 203
- DB-schedulable OpenAI OAuth accounts: 41
- Recent logs showed 201 `refresh_token_reused` refresh failures in the last
  10 minutes.

Refresh logs show the failing CPA accounts return OpenAI
`refresh_token_reused`:

```text
Your refresh token has already been used to generate a new access token.
code: refresh_token_reused
```

Interpretation:

- This is not a Sub2API rate-limit error.
- The refresh token stored in Sub2API is stale or has already been consumed by
  another system/import path.
- Native NewAPI CPA import can appear healthy if it owns the latest rotated
  refresh token. A second system using the same older CPA refresh token will
  fail on refresh.

Recommended next action:

- Re-import fresh/current CPA credentials into the affected group, or disable
  the stale `refresh_token_reused` accounts so the scheduler cannot select
  them.
- Avoid running the same CPA account set in multiple systems unless only one
  system is allowed to refresh and persist the rotated refresh token.
