# Production Relay Reliability Implementation Plan

> **For Codex:** REQUIRED SKILL: Use superpowers:executing-plans to implement this plan task by task. Because the repository instructions prohibit unsolicited subagents, execute the tasks inline in this worktree and stop before any production mutation.

**Goal:** Make synchronous text relay failures cool the failing channel for a stable 10-second default, prevent retries after downstream cancellation, and make outbound JSON request bodies safely replayable by Go's HTTP transport without changing billing behavior or asynchronous media routing.

**Architecture:** Keep channel cooldown in the existing channel-pool runtime, scoped by channel, group, and model. Preserve the incoming Gin request context on the upstream request, but reject application-level retries once that context is canceled. Add independent replay readers to the existing memory/disk body storage abstraction, expose them through outbound JSON bodies, and attach `ContentLength`/`GetBody` to upstream HTTP requests so HTTP/2 transport retries resend the exact payload.

**Tech Stack:** Go 1.22+, Gin, GORM-compatible existing services, `net/http`, `testify`, existing React production build for final local preview.

---

## Constraints

- Work only in `codex/production-reliability-20260809` at `C:\Users\ASUS\.config\superpowers\worktrees\newapi-710-yuapi\production-baseline-reconstruction`.
- Do not modify production containers, Caddy, production traffic, or production database.
- Do not merge or deploy `codex/affiliate-rebate-20260805` or the old detached recovery branch.
- Do not change billing expressions, quota calculation, pre-consumption, completed-response charging, or refund behavior.
- Preserve the running production image/container as rollback material.
- Do not expose secrets, request headers, database rows, or access logs in artifacts or output.

## Task 1: Default Text-Channel Cooldown

**Files:**
- Modify: `service/channel_pool.go`
- Modify: `service/channel_pool_test.go`

1. Add failing table-driven tests for the cooldown setting contract:
   - omitted/zero setting resolves to 10 seconds;
   - a positive setting overrides the default;
   - a negative setting explicitly disables cooldown;
   - text responses cool down on retryable upstream errors;
   - existing image, Midjourney, and video modes remain excluded.

2. Run the focused tests and confirm the new default/disable cases fail:

   ```powershell
   go test ./service -run 'TestEffectiveChannelPoolCooldownSeconds|TestMaybeCooldownSelectedChannelPool' -count=1
   ```

3. Add a stable domain helper and use it at the existing cooldown call site:

   ```go
   const defaultChannelPoolCooldownSeconds = 10

   func effectiveChannelPoolCooldownSeconds(configured int) int {
       if configured < 0 {
           return 0
       }
       if configured == 0 {
           return defaultChannelPoolCooldownSeconds
       }
       return configured
   }
   ```

4. Keep `shouldCooldownChannelPool` status behavior unchanged: retryable `409`, `425`, `429`, `529`, and retryable `5xx` cool the selected channel. Keep channel/group/model scoping and Redis TTL behavior unchanged.

5. Run focused tests, then commit:

   ```powershell
   go test ./service ./model -run 'ChannelPool|Cooldown' -count=1
   git add service/channel_pool.go service/channel_pool_test.go
   git commit -m "fix: default text channel cooldown to ten seconds"
   ```

## Task 2: Stop Application Retries After Client Cancellation

**Files:**
- Modify: `controller/relay.go`
- Create: `controller/relay_retry_test.go`

1. Add failing tests around `shouldRetry` for these observable decisions:
   - an active client context may retry a retryable `503` when attempts remain;
   - a canceled incoming request must not retry a `503`;
   - a canceled incoming request must not retry even a channel-classified error;
   - an error whose cause is `context.Canceled` must not retry.

2. Run and confirm the cancellation tests fail:

   ```powershell
   go test ./controller -run TestShouldRetry -count=1
   ```

3. Put cancellation checks immediately after the nil check, before affinity and channel-error branches:

   ```go
   if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
       return false
   }
   if errors.Is(openaiErr, context.Canceled) {
       return false
   }
   ```

4. Do not detach upstream requests from `c.Request.Context()`. Do not add background draining, post-accept stream retry, or any billing/refund branch.

5. Run focused tests, then commit:

   ```powershell
   go test ./controller -run TestShouldRetry -count=1
   git add controller/relay.go controller/relay_retry_test.go
   git commit -m "fix: stop relay retries after client cancellation"
   ```

## Task 3: Add Independent Replay Readers to Body Storage

**Files:**
- Modify: `common/body_storage.go`
- Modify: `common/body_storage_test.go`

1. Add failing memory and disk storage tests that prove:
   - two replay readers both start at byte zero and have independent cursors;
   - closing a replay reader does not close the backing storage;
   - `NewReader` after storage close returns `ErrStorageClosed`;
   - `NewReplayableBodyReader` exposes replay capability without exposing storage ownership.

2. Run and confirm the new tests fail to compile before implementation:

   ```powershell
   go test ./common -run 'Test.*BodyStorage.*NewReader|TestReplayableBodyReader' -count=1
   ```

3. Extend `BodyStorage` and add `ReplayableBody`:

   ```go
   type ReplayableBody interface {
       io.Reader
       Size() int64
       NewReader() (io.ReadCloser, error)
   }
   ```

4. Implement memory replay with a fresh `bytes.Reader` over immutable stored bytes. Implement disk replay with a new file descriptor opened on the cache file. Guard both with the existing close state and mutex.

5. Add `NewReplayableBodyReader(storage BodyStorage)` as a non-closing adapter so `net/http` cannot take ownership of the storage lifecycle.

6. Run focused and package tests, then commit:

   ```powershell
   go test ./common -run 'BodyStorage|ReplayableBody' -count=1
   go test ./common -count=1
   git add common/body_storage.go common/body_storage_test.go
   git commit -m "feat: make outbound body storage replayable"
   ```

## Task 4: Wire Safe HTTP Transport Replay

**Files:**
- Modify: `relay/common/outbound_body.go`
- Modify: `relay/channel/api_request.go`
- Create: `relay/channel/api_request_getbody_test.go`
- Modify: `relay/chat_completions_via_responses.go`
- Modify: `relay/responses_handler.go`
- Modify: `relay/rerank_handler.go`
- Modify: `relay/image_handler.go`
- Modify: `relay/gemini_handler.go`
- Modify: `relay/embedding_handler.go`
- Modify: `relay/compatible_handler.go`
- Modify: `relay/claude_handler.go`
- Modify: `relay/common/relay_info.go` only if the old size field has no remaining callers

1. Add failing request metadata tests that prove:
   - replayable bodies set exact `ContentLength` and non-nil `GetBody`;
   - calling `GetBody` after the first body is consumed returns the exact original bytes;
   - ordinary non-replayable readers are left untouched;
   - task requests retain the correct `GetBody` generated by `http.NewRequest` for `bytes.Reader` rather than returning the same consumed reader.

2. Port the bounded HTTP/2 GOAWAY replay regression test from upstream fix `d6b5ce99d`: the test server rejects the first connection before processing the stream, the transport retries, and the upstream receives the complete body exactly once.

3. Run and confirm the new tests fail:

   ```powershell
   go test ./relay/channel -run 'TestApplyUpstreamBodyMetadata|TestDoTaskApiRequest|TestDoApiRequest_HTTP2' -count=1
   ```

4. Change `NewOutboundJSONBody` to return `common.ReplayableBody` plus the separately owned closer:

   ```go
   func NewOutboundJSONBody(data []byte) (common.ReplayableBody, io.Closer, error) {
       storage, err := common.CreateBodyStorage(data)
       if err != nil {
           return nil, nil, err
       }
       return common.NewReplayableBodyReader(storage), storage, nil
   }
   ```

5. Replace `applyUpstreamContentLength` with `ApplyUpstreamBodyMetadata(req, originalBody)`. Set `ContentLength` from `Size()` and `GetBody` from `NewReader()`. If raw `BodyStorage` is ever passed, hide its `Close` method from the transport.

6. Call the metadata helper immediately after `http.NewRequest` in API, form, and task paths. Remove the task path closure that returns the same `requestBody` reader.

7. Update all outbound JSON call sites to the new return signature and remove stale `UpstreamRequestBodySize` assignments. Keep direct multipart/Claude sizing behavior only if it still has a live caller.

8. Run targeted relay tests and inspect for accidental billing changes:

   ```powershell
   go test ./relay/channel ./relay/common ./relay -run 'GetBody|Replay|HTTP2|OutboundJSONBody' -count=1
   git diff -- relay service controller common | rg -n 'quota|billing|refund|pre.?consum|PostConsume|PreConsume'
   ```

9. Commit:

   ```powershell
   git add common relay controller service
   git commit -m "fix: replay upstream request bodies on transport retry"
   ```

## Task 5: Regression and Billing-Invariant Verification

**Files:**
- Test only unless a directly related regression is found

1. Run focused channel selection, retry, relay, cache, and billing tests:

   ```powershell
   go test ./service ./model ./controller ./relay/... -count=1 -timeout 600s
   go test ./service -run 'Quota|Billing|PreConsume|PostConsume|Refund' -count=1
   go test ./relay/... -run 'Quota|Billing|PreConsume|PostConsume|Refund|Cache' -count=1
   ```

2. Run the complete backend suite from the production-baseline branch:

   ```powershell
   go test -p 2 ./... -count=1 -timeout 600s
   ```

3. Confirm the final diff contains no schema migration, SQL, billing expression, price ratio, quota, refund, Caddy, compose, or deployment changes:

   ```powershell
   git diff 36b44efd4707515558bdbe36f2846e28de229074...HEAD --stat
   git diff 36b44efd4707515558bdbe36f2846e28de229074...HEAD -- '*.go' | rg -n 'AutoMigrate|ALTER TABLE|billingexpr|model_ratio|group_ratio|refund|Refund|quota|Quota'
   ```

4. Record test commands and outcomes in the final handoff; do not declare completion unless all required checks pass.

## Task 6: Build and Run the Local Baseline Candidate

**Files:**
- Build artifacts only; do not commit generated artifacts unless already required by the repository build

1. Build the exact existing frontend theme without UI source changes:

   ```powershell
   Set-Location web/default
   bun install --frozen-lockfile
   bun run build
   Set-Location ../..
   ```

2. Build a uniquely tagged local candidate image/binary from this worktree. Bind the candidate only to `127.0.0.1` and use isolated local state/configuration; do not connect it to production database or Redis.

3. Run health/API checks and Playwright comparisons for home, login/register, console layout, API keys, system settings, infinite canvas, docs, branding, animations, and model configuration. Confirm screenshots match the approved production-baseline UI apart from dynamic data.

4. Provide the local-only URL and test evidence to the user. Keep the candidate running for their inspection when safe to do so.

5. Stop here. Production cutover is a separate action requiring the user's explicit confirmation after local review. The eventual cutover must keep the current production image/container available and define immediate rollback triggers for UI, database, billing, cache, health, and stream regressions.
