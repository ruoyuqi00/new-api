# Domestic Model Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans (recommended) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare and locally validate two YuAPI domestic-model groups using the existing alias, tiered billing, group-ratio, affinity, and audit paths without changing production.

**Architecture:** Keep model prices and group settings in the existing options, abilities, and channel configuration system. Store a secret-free catalog manifest for exact public aliases, provider names, source URLs, and verification states. Keep usage pricing on `tiered_expr`; add an isolated `per_call_expr` conversion mode for the `-call` aliases while reusing the existing expression evaluator, reservation, settlement, logging, and group-ratio path.

**Tech Stack:** Go 1.22, Gin/GORM, existing expression billing and OpenAI-compatible relay, PowerShell local verification, Bun frontend checks only if source changes require them.

---

### Task 1: Freeze the secret-free domestic catalog

**Files:**
- Create: `docs/superpowers/configs/2026-08-25-domestic-model-groups.json`
- Test: `docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.ps1`

- [x] **Step 1: Write the manifest validator**

Load the JSON with PowerShell `ConvertFrom-Json` and use built-in `throw` assertions for group names, ratio `0.3`, six models in each group, `-call` suffixes only in the call group, exact upstream names without `-call`, and base URL `https://api.herohao.top/v1`. Assert that serialized manifest content contains neither supplied API key nor an `Authorization` field.

- [x] **Step 2: Run the validator and verify the missing manifest is reported**

```powershell
pwsh -NoProfile -File docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.ps1
```

Expected result: fail because the manifest does not exist; this is a configuration-file validation exception, not a production-code TDD cycle.

- [x] **Step 3: Create the manifest**

Write the six exact upstream identifiers returned by the authenticated `/v1/models` probes. Keep DeepSeek date-suffixed pricing and Kimi-K3 `verification_state` as `pending` until an official exact-name price source is available. Record verified MiniMax and GLM sources, and keep Qwen region/cache fields pending until the upstream deployment region is confirmed. Include both groups, ratio `0.3`, channel base URL, public names, upstream names, billing mode, and source URLs only.

- [x] **Step 4: Run the validator and verify it passes**

Run the same `pwsh` command. Expected result: pass with no secret matches.

- [x] **Step 5: Commit the manifest and test**

```powershell
git add docs/superpowers/configs/2026-08-25-domestic-model-groups.json docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.ps1
git commit -m "config: catalog domestic model groups"
```

### Task 2: Prove alias mapping and single group-ratio application

**Files:**
- Test: `relay/helper/model_mapped_test.go`
- Test: `relay/helper/price_test.go`
- Test: `service/text_quota_test.go`

- [x] **Step 1: Add the alias contract regression test**

Add a table test for each `-call` public name through the existing model-mapping helper. Assert `OriginModelName` remains the alias while outbound request model and `UpstreamModelName` equal the provider name. This is a verification-only test because the mapping path already exists; no production code is changed unless this regression fails.

- [x] **Step 2: Run the focused regression test**

```powershell
go test ./relay/helper -run 'Domestic|ModelMapping' -count=1
```

Expected result: pass on the current YuAPI baseline. A failure is a blocker requiring a separate TDD fix before continuing.

- [x] **Step 3: Add the pricing contract vectors**

Use synthetic usage vectors for one-million-token input/output, cache-read tokens, and context boundaries. Assert `tiered_expr` conversion as `expression output * QuotaPerUnit / 1,000,000 * 0.3`, `per_call_expr` conversion as `fixed call price * QuotaPerUnit * 0.3`, and assert coefficients are not pre-multiplied by `0.3`. Include a fixed-price `len`-tier expression for the call group and assert output-token changes do not change its result within a context tier.

- [x] **Step 4: Run focused tests and verify they pass**

```powershell
go test ./relay/helper ./service -run 'Domestic|Tiered|ModelMapping' -count=1
```

- [x] **Step 5: Commit the contract tests**

```powershell
git add relay/helper/model_mapped_test.go relay/helper/price_test.go service/text_quota_test.go
git commit -m "test: verify domestic aliases and group pricing"
```

### Task 3: Start an isolated local candidate

**Files:**
- Create: `docs/superpowers/handoffs/2026-08-25-domestic-model-groups-local.md`

- [ ] **Step 1: Build the backend and frontend from the approved baseline worktree**

Run existing backend and frontend build commands without changing the production image or Caddy. Use a free localhost port, leaving the existing UI instance untouched.

- [ ] **Step 2: Capture the local pre-apply `/api/pricing` snapshot**

Save JSON and SHA-256 hash outside the repository. Confirm the snapshot contains no domestic group or target model before applying the candidate configuration.

- [ ] **Step 3: Apply only verified local settings through existing admin APIs**

Use the existing option endpoint for `GroupRatio`, `UserUsableGroups`, `billing_setting.billing_mode`, `billing_setting.billing_expr`, and model price maps. Apply changes in one local database transaction or restore the pre-apply snapshot on failure. Do not enable pending models. Create two disabled-by-default local OpenAI-compatible channels with the supplied keys entered interactively or through process environment variables; never write them to the handoff document.

- [ ] **Step 4: Capture the local post-apply snapshot**

Record `/api/pricing` JSON and hash, then assert exact group/model visibility rules and both ratios equal `0.3`.

- [ ] **Step 5: Commit the handoff record**

The handoff records snapshot hashes, enabled/pending model lists, channel IDs, and key-presence booleans only. It must not contain keys, request headers, or response bodies.

### Task 4: Validate routing, billing, and upstream reachability without paid completion

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-25-domestic-model-groups-local.md`

- [ ] **Step 1: Re-run authenticated model discovery**

Read the two key values from process environment and issue only `GET /v1/models`. Assert HTTP 200 and the six target IDs for both keys; report counts and model IDs only.

- [ ] **Step 2: Validate synthetic relay mapping**

For every `-call` alias, assert generated outbound request contains only the official upstream model name. Assert public request/log model remains the alias and upstream response model is stored in `ActualResponseModel`.

- [ ] **Step 3: Validate pricing vectors**

Run one-million-token input/output, cache-hit, context-boundary, normal-time, and pending-price vectors. Assert one `0.3` multiplication, fixed call pricing within a context tier, and no enabled pending model.

- [ ] **Step 4: Run regression suites**

```powershell
go test ./relay/helper ./service ./controller ./model -count=1
git diff --check
```

Run frontend typecheck only if source files under `web/default` changed.

- [ ] **Step 5: Keep production blocked**

Do not change Caddy, production containers, production database, or production option values. Present the local URL, pricing snapshot hashes, enabled/pending list, and regression output for user approval before any production import.

Current gate: the feature and secret-free manifest are ready, but local option/channel application remains blocked until the user confirms the local candidate. No production channel or option was created by this branch.

Price gate: the manifest records official CNY source values, but no USD exchange rate or official per-call price was guessed. A local apply must read the configured `USDExchangeRate`, convert verified usage expressions once, persist the conversion snapshot, and leave all pending aliases disabled.

### Task 5: Track parent hardening separately

**Files:**
- Reference: `docs/superpowers/specs/2026-08-25-parent-feature-audit.md`

- [ ] **Step 1: Do not merge parent feature commits into this branch**

The user-scoped critical limiter and Ali `top_p` omission fix remain separate follow-up patches. Per-channel transport controls and parent tiered billing changes stay deferred until targeted evidence and a separate design approval exist.
