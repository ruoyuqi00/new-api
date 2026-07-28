# YuAPI Upstream NewAPI Audit - 2026-07-09

This document records the first audit of recent upstream
`QuantumNous/new-api` changes after the YuAPI/Sub2API plus/pro migration.

It is an evaluation record, not a merge commit. No upstream code was merged as
part of this audit.

## Audited Upstream

```text
origin: https://github.com/QuantumNous/new-api.git
origin/main: a79f9691 fix(affiliate): update referral message
latest fetched tag: v1.0.0-rc.20
v1.0.0-rc.20: 6ce7305c feat(price): add token ratios for GPT-5.6 models
```

## 2026-07-10 Incremental Upstream Check

Fetched `origin/main` again during Phase 21. The audited range
`a79f96919..origin/main` contains one commit:

```text
246d62aa5 chore: remove dead files resurrected by v1.0 launch commit (#6041)
```

Decision:

- Do not backport this in the Phase 21 production deploy.
- It deletes dead files only: `controller/swag_video.go`,
  `controller/task_video.go`, and `service/pre_consume_quota.go`.
- No runtime protocol, routing, billing, Responses, channel capability, or
  scheduler behavior changes were found in this upstream increment.
- Treat it as optional later cleanup, not as a production hardening fix.

Phase 21 therefore proceeds with a local YuAPI compact endpoint capability fix
instead of an upstream backport.

The previous project baseline had observed `origin/main` at `12603a77`.
Between `12603a77` and `origin/main`, upstream added backend, billing, security,
web, and i18n changes.

High-level diff size from `12603a77..origin/main`:

```text
177 files changed
about 12.8k insertions, 1.7k deletions
```

High-level diff size from current YuAPI production branch to `origin/main` for
backend-ish paths is not directly merge-safe:

```text
106 backend-ish files differ
```

That direct diff would also remove local YuAPI production work such as:

- channel-pool runtime files;
- user concurrency limiter;
- quota-sync additions;
- YuCore/YuAPI media additions;
- local image/openai empty-response patches.

Conclusion: do not merge `origin/main` wholesale. Backport selected commits or
small patch sets by hand.

## Highest-Value Backport Candidates

### P0 / P1: Stream Disconnect And Billing Safety

Candidate:

```text
153d7f01 fix: avoid stale stream writes after client disconnect (#5710)
```

Why it matters:

- Closes upstream body promptly when the client disconnects.
- Avoids continuing provider generation after the user has gone away.
- Reduces risk of billing users for tokens generated after disconnect.
- Adds write deadlines so slow clients cannot pin stream goroutines forever.

Backport mode:

- Manual transplant, not blind cherry-pick.
- Touches stream scanner and API request ping lifecycle.
- Must verify with streaming chat tests and non-stream chat smoke.

Suggested tests:

```bash
go test ./relay/helper ./relay/channel ./relay
go test ./service ./middleware ./controller
```

### P0 / P1: Quota And Billing Hardening

Candidates:

```text
d0bd8aac fix(billing): validate quantity parameters and harden quota calculations
c9943d37 fix(billing): extend quantity validation and saturating conversions to remaining paths
bae799cc fix(billing): surface quota saturation events for admin auditing
48b7f491 fix(billing): adjust quota calculation to prevent exceeding int32 limits
3fbad6a7 fix(price): add default token estimate for tiered expression pre-consume
043720f9 fix: task differential settlement and Ali video duration adjustment
```

Why it matters:

- Prevents quota overflow/saturation edge cases.
- Hardens quantity validation for text, audio, task, and tiered expression
  billing paths.
- Adds admin-visible saturation evidence instead of silent truncation.
- Improves tiered expression pre-consume when `max_tokens` is omitted.

Backport mode:

- Split into smaller batches.
- Start with `3fbad6a7` and `48b7f491` if we need low-risk first wins.
- Treat `d0bd8aac/c9943d37/bae799cc` as a coordinated billing batch because
  they introduce shared quota math helpers and tests.

Suggested tests:

```bash
go test ./common ./pkg/billingexpr ./relay/helper ./service ./model
go test ./controller ./relay
```

### P1: Transaction Row Locking

Candidate:

```text
70ea899e fix(model): centralize row locking in transactional flows
```

Why it matters:

- Reduces concurrency hazards in top-up, redemption, subscription, and user
  quota mutation paths.
- Useful before increasing real production concurrency or expanding billing
  features.

Backport mode:

- Manual or cherry-pick into a separate branch, then inspect conflicts.
- Must review against YuAPI local subscription/quota patches.

Suggested tests:

```bash
go test ./model ./service ./controller
```

### P1: SSRF Protection

Candidate:

```text
df087b02 feat(ssrf): implement SSRF protection in HTTP clients and validation functions
```

Why it matters:

- Protects URL fetch/download/webhook/proxy-style features from private-network
  and local-metadata abuse.
- Relevant to any feature that accepts user-controlled URLs.

Backport mode:

- Manual batch; this is security-relevant but broad.
- Review interaction with existing proxy/channel settings and allowed internal
  service calls before deployment.

Suggested tests:

```bash
go test ./common ./service ./controller ./relay
```

### P1 / P2: Auth And User Input Hardening

Candidates:

```text
0d5995eb fix(auth): allow read-only access for non-disabled tokens
bed4a3f9 fix(user): trim whitespace from username and validate input
5fc35e28 fix(user): harden account email and password handling
4a64b870 test(user): cover self-service password update guard
```

Why it matters:

- `0d5995eb` closes a small but real gap: read-only token auth should reject
  explicitly disabled tokens while still allowing non-disabled read-only usage.
- Username trimming and account validation prevent duplicate/blank/whitespace
  account states.
- Password/email hardening reduces account management edge cases.

Backport mode:

- `0d5995eb` is a small first-batch candidate.
- User hardening should be reviewed as a small group because validation and
  i18n keys move together.

Suggested tests:

```bash
go test ./middleware ./model ./controller
```

## Useful But Lower-Risk / Product UX Candidates

These are worth considering but should not block backend stabilization:

```text
4ae34175 fix(channels): show field passthrough controls for Codex (#5902)
57865fc1 fix: restore default channel connection paste
394b023d fix: keep group ratio input as string draft to allow decimal typing (#5995)
90fa6fe6 fix(wallet): honor configured quota units for reward transfers (#5808)
28e0115a fix(web): prevent browser translation from mutating React roots (#5963)
df01273b fix(web): let resized tables fill available width (#6031)
8739c05c feat(web): support manual channel-list column resizing (#5948)
6a437a33 feat(oauth): add OAuth callback URL display and copy functionality
97bbb7c8 feat(pricing): enhance dynamic pricing calculations with group selection support
6ce7305c feat(price): add token ratios for GPT-5.6 models
```

Suggested handling:

- Keep UI/UX fixes out of the first backend hardening batch unless an admin
  workflow is currently broken.
- Consider `4ae34175` and `57865fc1` if channel editing/Codex admin operations
  are painful during migration work.
- Consider `6ce7305c` only when GPT-5.6 aliases are deliberately exposed.

## Not Recommended As A Direct Merge

Do not merge `origin/main` into the production branch in one shot.

Reasons:

- The production branch contains local YuAPI migration patches not present
  upstream.
- A direct merge would mix backend billing fixes with large web/i18n changes.
- The server is newly stabilized after Sub2API app retirement; smaller
  backports are easier to test and roll back.

## Proposed Backport Order

### Batch 1: Small Safety Fixes

- `0d5995eb` read-only token disabled-state guard.
- `bed4a3f9` username trim/blank validation.
- `3fbad6a7` tiered expression default pre-consume estimate, if tiered billing
  is enabled or planned.

### Batch 2: Streaming Stability

- Manual backport of `153d7f01`.
- Add a disconnect/slow-client smoke if practical.

### Batch 3: Billing And Quota Math

- `48b7f491`
- `d0bd8aac`
- `c9943d37`
- `bae799cc`
- Related tests from upstream.

### Batch 4: Security / SSRF

- Manual backport of `df087b02`.
- Review internal host allow/deny behavior before production deploy.

### Batch 5: Admin UI / Operational UX

- Channel connection paste.
- Codex passthrough controls.
- Group ratio decimal draft input.
- Table sizing/column resizing if the new UI remains the admin surface.

## First Manual Bug Hunt Areas If Upstream Is Not Enough

If we start looking for YuAPI-specific bugs ourselves, begin here:

1. Channel-pool runtime observability:
   - cooled/full channel counters;
   - per-channel skipped reason logs;
   - admin visibility for fallback selection.
2. Affinity plus cooldown interaction:
   - sticky cache should not be cleared for temporary cooldown/full;
   - hard-disable should still clear or bypass affinity.
3. Image route billing and retry:
   - no duplicate paid image attempts;
   - empty image payloads must not become success;
   - task polling should not double-settle quota.
4. Pricing and quota saturation:
   - very large prompts/completions;
   - tiered expression with omitted `max_tokens`;
   - group-ratio edge cases.
5. User/token control-plane edges:
   - disabled tokens and read-only endpoints;
   - whitespace username/email cases;
   - concurrent quota updates.
