# Domestic Model Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans (recommended) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare and locally validate two YuAPI domestic-model groups using the existing alias, tiered billing, group-ratio, affinity, and audit paths without changing production.

**Architecture:** Keep model prices and group settings in the existing options, abilities, and channel configuration system. Store a secret-free catalog manifest for exact public aliases, provider names, source URLs, and verification states. Configure the local candidate through existing admin endpoints; do not add a second billing engine or a new UI.

**Tech Stack:** Go 1.22, Gin/GORM, existing `tiered_expr` billing, existing OpenAI-compatible relay, PowerShell local verification, Bun frontend checks only if source changes require them.

---

### Task 1: Freeze the secret-free domestic catalog

**Files:**
- Create: `docs/superpowers/configs/2026-08-25-domestic-model-groups.json`
- Test: `docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.test.ps1`

- [ ] **Step 1: Write the manifest test**

Load the JSON with PowerShell `ConvertFrom-Json` and assert group names, ratio `0.3`, six models in each group, `-call` suffixes only in the call group, exact upstream names without `-call`, and base URL `https://api.herohao.top/v1`. Assert that serialized manifest content contains neither supplied API key nor an `Authorization` field.

- [ ] **Step 2: Run the test and verify it fails**

```powershell
pester -Path docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.test.ps1
```

Expected result: fail because the manifest does not exist.

- [ ] **Step 3: Create the manifest**

Write the six exact upstream identifiers returned by the authenticated `/v1/models` probes. Keep DeepSeek date-suffixed pricing and Kimi-K3 `verification_state` as `pending` until an official exact-name price source is available. Record verified MiniMax and GLM sources, and keep Qwen region/cache fields pending until the upstream deployment region is confirmed. Include both groups, ratio `0.3`, channel base URL, public names, upstream names, billing mode, and source URLs only.

- [ ] **Step 4: Run the test and verify it passes**

Run the same Pester command. Expected result: pass with no secret matches.

- [ ] **Step 5: Commit the manifest and test**

```powershell
git add docs/superpowers/configs/2026-08-25-domestic-model-groups.json docs/superpowers/configs/2026-08-25-domestic-model-groups.schema.test.ps1
git commit -m "config: catalog domestic model groups"
```

### Task 2: Prove alias mapping and single group-ratio application

**Files:**
- Modify: `relay/helper/model_mapped_test.go`
- Modify: `relay/helper/price_test.go`
- Modify: `service/text_quota_test.go`

- [ ] **Step 1: Add the failing alias contract test**

Add a table test for each `-call` public name through the existing model-mapping helper. Assert `OriginModelName` remains the alias while outbound request model and `UpstreamModelName` equal the provider name.

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
go test ./relay/helper -run 'Domestic|ModelMapping' -count=1
```

Expected result: the new alias case fails until the fixture uses the existing mapping contract correctly.

- [ ] **Step 3: Add the pricing contract vectors**

Use synthetic usage vectors for one-million-token input/output, cache-read tokens, and context boundaries. Assert conversion as `expression output * QuotaPerUnit / 1,000,000 * 0.3` and assert expression coefficients are not pre-multiplied by `0.3`. Include a fixed-price `len`-tier expression for the call group and assert output-token changes do not change its result within a context tier.

- [ ] **Step 4: Run focused tests and verify they pass**

```powershell
go test ./relay/helper ./service -run 'Domestic|Tiered|ModelMapping' -count=1
```

- [ ] **Step 5: Commit the contract tests**

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

### Task 5: Track parent hardening separately

**Files:**
- Reference: `docs/superpowers/specs/2026-08-25-parent-feature-audit.md`

- [ ] **Step 1: Do not merge parent feature commits into this branch**

The user-scoped critical limiter and Ali `top_p` omission fix remain separate follow-up patches. Per-channel transport controls and parent tiered billing changes stay deferred until targeted evidence and a separate design approval exist.

