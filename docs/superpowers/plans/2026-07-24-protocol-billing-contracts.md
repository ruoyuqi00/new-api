# Protocol And Billing Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve active OpenAI-compatible, Claude, and Gemini protocol and quota contracts while selectively backporting the parent Responses-to-Chat stream fix without importing advanced-custom routing or experimental UI.

**Architecture:** The branch already has cache-write billing normalization: `CacheCreationTokensTotal`, zero-clamped cache overlap, and OpenAI Responses usage propagation. This batch adds the independent Responses-to-Chat stream identity reconciliation and verifies active provider contracts.

**Tech Stack:** Go 1.22+, Gin, testify, GORM-compatible gateway code.

---

## File Map

- Modify: `service/relayconvert/chat_responses_compat_test.go` - Responses-to-Chat regression coverage.
- Modify: `service/relayconvert/responses_to_chat.go` - Reconcile equivalent stream tool identities before allocating a new Chat tool-call index.
- Modify: `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md` - Record the selective backport, evidence, and next advanced-custom-routing objective.

### Task 1: Lock The Responses Tool-Call Identity Contract

**Files:** `service/relayconvert/chat_responses_compat_test.go`

- [x] **Step 1: Write the failing regression test**

Feed one `ResponsesToChatStreamState` two function-call item events. The first uses `ID: "fc_item_1"`, `CallId: "call_1"`, `Name: "lookup"`, and `OutputIndex: 0`; the second describes the same call through another stream key while retaining `CallId: "call_1"`. Assert every emitted `tool_calls` entry uses index `0` and only one logical call ID appears.

- [x] **Step 2: Verify RED**

Run `go test ./service/relayconvert -run '^TestResponsesStreamEventToChatChunksReusesToolByCallID$' -count=1`. It must fail because the current converter allocates a second `responsesStreamTool` for the alternate key.

### Task 2: Reconcile Equivalent Responses Stream Tool Identities

**Files:** `service/relayconvert/responses_to_chat.go:463-511`; `service/relayconvert/chat_responses_compat_test.go`

- [x] **Step 1: Implement the minimal identity lookup**

In `ResponsesToChatStreamState.ensureToolForEvent`, after `tool := s.toolByKey[key]`, resolve by event item ID and then call ID before creating a new tool. Cache an existing tool under the new key. Keep output-index mapping, pending arguments, and call-ID updates unchanged. Do not alter advanced-custom routes, channel selection, mappings, UI, or billing formulas.

- [x] **Step 2: Verify GREEN**

Run `go test ./service/relayconvert -run '^TestResponsesStreamEventToChatChunksReusesToolByCallID$' -count=1`, then `go test ./service/relayconvert -count=1`. Both must pass.

- [x] **Step 3: Commit**

Run `git add service/relayconvert/responses_to_chat.go service/relayconvert/chat_responses_compat_test.go` and `git commit -m "fix: deduplicate response stream tool calls"`.

### Task 3: Verify Active Billing And Provider Conversion Contracts

**Files:** `dto/openai_cache_write_test.go`; `service/text_quota_test.go`; `service/tiered_settle_test.go`; `relay/channel/openai/relay_responses_test.go`; `relay/channel/claude/relay_claude_test.go`; `relay/channel/gemini/adaptor_responses_test.go`; `relay/channel/gemini/relay_gemini_usage_test.go`

- [x] **Step 1: Run focused contracts**

Run `go test ./dto ./service ./relay/channel/openai ./relay/channel/claude ./relay/channel/gemini -count=1`. This covers OpenAI `cache_write_tokens`, non-negative uncached input, compact cache-key forwarding, Claude cache semantics, and Gemini Responses conversion.

- [x] **Step 2: Run broad backend verification**

Run `go test ./relay/helper ./relay/channel ./relay ./service ./controller ./model ./middleware -count=1` and `git diff --check`. All must pass.

- [x] **Step 3: Build without experimental artifacts**

Run `docker build -t newapi:protocol-billing-contracts-20260724 .` and `docker image inspect newapi:protocol-billing-contracts-20260724 --format '{{.Id}}'`. Inspect Docker ignore rules and staged files: no `output/local-experiments`, poster assets, or YuCore experimental UI files may be included.

### Task 4: Deploy And Define The Next Boundary

**Files:** `docs/YUAPI_PHASED_FIX_PLAN_2026-07-09.md`

- [x] **Step 1: Deploy only the verified `newapi` image**

Back up production compose, point only `newapi` at `newapi:protocol-billing-contracts-20260724`, then run `docker compose up -d newapi`. Do not change MySQL, Redis, account pools, groups, channel priorities, private grants, or experimental assets.

- [x] **Step 2: Verify production**

Run `docker compose ps`, `curl -fsS http://127.0.0.1:3000/api/status`, and `curl -sS -o /dev/null -w '%{http_code}\n' https://api.yuaiapi.com/`. The service must be healthy, status must succeed, and the public endpoint must remain reachable.

- [x] **Step 3: Record the next goal**

Add commit, image, and deployment evidence to the phase plan. The next batch isolates advanced-custom upstream discovery and path matching with fixtures for every private group and model alias; it requires no cross-group routing, stable downstream `/v1/models`, and a per-channel rollback switch. Do not import the parent's broad protocol registry or unrelated UI changes.
