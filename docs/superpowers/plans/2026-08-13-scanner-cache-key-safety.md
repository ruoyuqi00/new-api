# Scanner, Cache Key, and Upstream Error Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent exact terminal streams from becoming `scanner_error`, restore their full channel-affinity commit, optionally inject a private stable `prompt_cache_key` for native Responses requests, and keep upstream infrastructure out of downstream errors.

**Architecture:** Normalize scanner termination only from the authoritative stream-recovery snapshot, so real pre-terminal failures remain errors. Extend affinity metadata with an opt-in HMAC-derived key and apply it to converted and raw Responses JSON only when the client omitted the field. Add a separate public error projection while retaining the original internal error for retry, cooldown, and violation classification.

**Tech Stack:** Go 1.22+, Gin, `tidwall/gjson`/`sjson`, existing `common` JSON/HMAC wrappers, `testify/require` and `testify/assert`.

---

### Task 1: Normalize Post-Terminal Scanner Cancellation

**Files:**
- Modify: `relay/helper/stream_scanner_test.go`
- Modify: `relay/helper/stream_scanner.go`

- [ ] **Step 1: Write the failing exact-terminal scanner test**

Create an `io.Pipe` body that emits `data: terminal`, then closes with a transport error when `MarkStreamTerminalUsage` cancels the recovery context. The callback marks terminal usage. Assert `StreamEndReasonDone`, no stream errors, exact usage, and completed recovery. Add the inverse case where the same read error occurs before terminal usage and remains `StreamEndReasonScannerErr`.

```go
snapshot := info.GetStreamRecoverySnapshot()
require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
require.False(t, info.StreamStatus.HasErrors())
require.Equal(t, relaycommon.StreamUsageStateExact, snapshot.UsageState)
require.Equal(t, relaycommon.StreamDrainResultCompleted, snapshot.DrainResult)
```

- [ ] **Step 2: Run the focused test and capture RED**

```powershell
go test ./relay/helper -run 'ExactTerminalCancellationIsDone|ScannerError' -count=1
```

Expected: exact-terminal test fails with `scanner_error`; pre-terminal test passes.

- [ ] **Step 3: Implement the minimal lifecycle-state check**

In the `scanner.Err()` branch, read `GetStreamRecoverySnapshot`. If and only if it reports `Enabled`, `UsageStateExact`, and `DrainResultCompleted`, set `done` without an error. Otherwise retain the existing scanner-error logic. Do not match error strings or modify client-gone, timeout, size-limit, or parser behavior.

```go
snapshot := info.GetStreamRecoverySnapshot()
terminalCompleted := snapshot.Enabled &&
    snapshot.UsageState == relaycommon.StreamUsageStateExact &&
    snapshot.DrainResult == relaycommon.StreamDrainResultCompleted
```

- [ ] **Step 4: Verify GREEN and commit**

```powershell
go test ./relay/helper ./relay/common -run 'StreamScanner|StreamRecovery' -count=1
git add relay/helper/stream_scanner.go relay/helper/stream_scanner_test.go
git commit -m "fix: normalize exact terminal stream cancellation"
```

### Task 2: Protect the Full Affinity Commit Contract

**Files:**
- Modify: `controller/relay_retry_test.go`

- [ ] **Step 1: Add a completed Responses regression**

Create a live `/v1/responses` context with an explicit primary key and a relay state containing `done`, `StreamTerminalMarkersRequired=true`, `StreamTerminalSuccess=true`, and `StreamTerminalUsageSeen=true`. Require `shouldCommitChannelAffinity=true`, call `commitChannelAffinityOutcome` and `RecordChannelAffinity`, then require the next matching request to select the committed channel. Retain existing negative cases for scanner error, missing terminal marker, and canceled context.

- [ ] **Step 2: Run and commit**

```powershell
go test ./controller -run 'ShouldCommitChannelAffinity|CommitResponseChainAffinityOutcome|ExactTerminal' -count=1
git add controller/relay_retry_test.go
git commit -m "test: protect exact terminal affinity commit"
```

Expected: PASS after Task 1; this protects the cross-module behavior without adding controller production code.

### Task 3: Derive an Opt-In Stable Prompt Cache Key

**Files:**
- Modify: `setting/operation_setting/channel_affinity_setting.go`
- Modify: `service/channel_affinity.go`
- Modify: `service/channel_affinity_template_test.go`

- [ ] **Step 1: Write failing derivation tests**

Add `InjectPromptCacheKey: true` to a test rule with a `Session_id` source. Require the same token/group/model/session to produce the same key, and different token or group scopes to produce different keys. Require the key not to contain the raw session. Add negative cases for a disabled rule, explicit body `prompt_cache_key`, response-chain-only source, unsupported path, missing token ID, and missing stable source.

- [ ] **Step 2: Capture RED**

```powershell
go test ./service ./setting/operation_setting -run 'ChannelAffinityPromptCacheKey|ChannelAffinity.*RequestHeader' -count=1
```

Expected: missing rule field/helper or absent derived key.

- [ ] **Step 3: Add configuration and private metadata**

Add this field to `ChannelAffinityRule` and set it true only on the built-in Codex `/v1/responses` rule:

```go
InjectPromptCacheKey bool `json:"inject_prompt_cache_key,omitempty"`
```

Add a private `PromptCacheKey` field to `channelAffinityMeta`. When the matched source is a configured conversation or request-header source, the switch is enabled, token ID is positive, and the exact request path is `/v1/responses`, derive:

```go
payload := fmt.Sprintf("pck-v1\x00%d\x00%d:%s\x00%d:%s\x00%d:%s\x00%d:%s\x00%d:%s",
    tokenID,
    len(usingGroup), usingGroup,
    len(modelName), modelName,
    len(sourceType), sourceType,
    len(sourceIdentity), sourceIdentity,
    len(affinityValue), affinityValue,
)
derived := "yuapi-pck-v1-" + common.GenerateHMAC(payload)
```

Never derive from `response_chain`, the explicit `gjson prompt_cache_key` source, a full request body, or mutable history. Expose only `GetChannelAffinityPromptCacheKey(c) (string, bool)`.

- [ ] **Step 4: Verify GREEN and commit**

```powershell
go test ./service ./setting/operation_setting -run 'ChannelAffinityPromptCacheKey|ChannelAffinity.*RequestHeader' -count=1
git add setting/operation_setting/channel_affinity_setting.go service/channel_affinity.go service/channel_affinity_template_test.go
git commit -m "feat: derive scoped prompt cache keys"
```

### Task 4: Inject the Key Into Converted and Raw Responses Requests

**Files:**
- Modify: `relay/stream_recovery_handler_test.go`
- Modify: `relay/responses_handler.go`

- [ ] **Step 1: Write failing outbound-body tests**

Use an `httptest.Server` to decode the actual upstream JSON with `common.DecodeJson`. Cover normal conversion and `PassThroughBodyEnabled=true`. Require the derived key when absent, exact preservation of an explicit client key, preservation of an unknown raw passthrough field, and no field when the switch or stable source is absent.

- [ ] **Step 2: Capture RED**

```powershell
go test ./relay -run 'ResponsesHelper.*PromptCacheKey' -count=1
```

Expected: derived key is absent, especially in raw passthrough mode.

- [ ] **Step 3: Implement structured injection**

Make both request branches produce JSON bytes. Inspect `prompt_cache_key` with `gjson`; only when it is absent or an empty string, insert the already-derived value with `sjson.SetBytes`. Create the outbound reader with `relaycommon.NewOutboundJSONBody`. Preserve all unknown passthrough fields and do not turn on global body passthrough or unrelated overrides.

```go
existing := gjson.GetBytes(jsonData, "prompt_cache_key")
if !existing.Exists() || strings.TrimSpace(existing.String()) == "" {
    jsonData, err = sjson.SetBytes(jsonData, "prompt_cache_key", derivedKey)
}
```

- [ ] **Step 4: Verify GREEN and commit**

```powershell
go test ./relay -run 'ResponsesHelper.*PromptCacheKey|ResponsesHelperStreamRecovery|Accepted' -count=1
git add relay/responses_handler.go relay/stream_recovery_handler_test.go
git commit -m "feat: inject stable responses cache keys"
```

### Task 5: Separate Internal and Public Upstream Errors

**Files:**
- Modify: `types/error.go`
- Modify: `types/error_test.go`
- Modify: `service/error.go`
- Modify: `service/error_test.go`
- Modify: `controller/relay.go`

- [ ] **Step 1: Write failing public-projection tests**

Seed a structured upstream message containing a URL, IP, provider/channel name, bearer value, redirect, and raw body. Require the internal `NewAPIError.Error()` to retain the original message for policy classification while the public OpenAI and Claude projections contain none of those strings. Require a stable gateway-owned code/type/message plus request ID. Add a local validation error case and require its actionable message to remain.

- [ ] **Step 2: Capture RED**

```powershell
go test ./types ./service -run 'Public.*Error|RelayErrorHandler.*Structured' -count=1
```

Expected: missing public projection or leakage assertion failure.

- [ ] **Step 3: Add an optional public error projection**

Add a private `publicError *OpenAIError` to `NewAPIError`, an `ErrOptionWithPublicError` option, and `ToPublicOpenAIError(requestID)` / `ToPublicClaudeError(requestID)`. When `publicError` is absent, preserve current local-error behavior. When present, return the gateway-owned error and append the gateway request ID through `common.MessageWithRequestId`.

In `RelayErrorHandler`, attach a public projection to every upstream non-2xx response. Use a stable 4xx rejection message and a stable 5xx temporary-unavailable message, type `upstream_error`, and code `bad_response_status_code`. Keep the original parsed error in `Err` and `RelayError` for internal decisions.

- [ ] **Step 4: Serialize only the public projection**

In `controller/relay.go`, log the bounded internal error, then use the public projection for WebSocket, Claude, and OpenAI JSON output. Do not overwrite the internal error with the request-ID message.

- [ ] **Step 5: Verify GREEN and commit**

```powershell
go test ./types ./service ./controller -run 'Public.*Error|RelayErrorHandler.*Structured|Relay.*Error' -count=1
git add types/error.go types/error_test.go service/error.go service/error_test.go controller/relay.go
git commit -m "fix: sanitize public upstream errors"
```

### Task 6: Full Verification and Local Candidate

**Files:**
- No source changes unless a regression is first reproduced by a focused test.

- [ ] **Step 1: Run focused and full verification**

```powershell
go test ./relay/helper ./relay/common ./relay ./service ./types ./controller -run 'StreamScanner|StreamRecovery|ChannelAffinity|PromptCacheKey|Public.*Error|RelayErrorHandler|ShouldCommit' -count=1
go test ./relay/... ./service ./types ./controller ./setting/operation_setting -count=1
go vet ./relay/... ./service ./types ./controller ./setting/operation_setting
git diff --check
```

Expected: all commands exit 0 with no new warnings.

- [ ] **Step 2: Audit scope**

```powershell
git diff --stat 5f1a89e32..HEAD
git diff --name-only 5f1a89e32..HEAD
```

Require no `web/`, theme, branding, migration/schema, Caddy, environment, credential, pricing, tokenizer, or billing-expression changes.

- [ ] **Step 3: Build and inspect a local-only candidate**

Build the existing production-aligned Docker target and bind it only to `127.0.0.1` on an unused port. Do not alter Caddy. Compare homepage HTML/static fingerprints and the branded sign-in page with the accepted local baseline. Any brand/UI difference blocks release.

- [ ] **Step 4: Stop before production**

Report the exact commits, tests, local URL, and the remaining production opt-in setting. Wait for explicit user confirmation before changing a production container, Caddy, traffic, or setting.
