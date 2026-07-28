# Production-Baseline Strategy And UI Optimization Design

## Status

Approved for implementation planning on 2026-07-26.

## Goal

Use the currently deployed YuCore UI lineage as the only local baseline, port
only missing backend reliability contracts, improve the existing UI without
replacing its visual system, and present anonymous, user, and super-admin
previews before any production build.

## Baseline And Evidence

- Production image: `newapi:veo-auto-resolution-20260724`.
- Production source baseline: `1254d9055` (`fix: normalize Veo auto
  resolution`).
- Local target worktree: `D:\yucore-local-production`.
- Local target branch: `codex/local-production-brand-performance-20260725`.
- The target branch keeps the production frontend dependency and asset
  lineage. Its shared vendor bundles match the deployed site.
- The rejected fusion worktree changes 176 frontend files relative to the
  deployed baseline and is not an integration source for UI code.
- The existing local performance work in `yucore-home.tsx` and
  `yucore-home-details.tsx` is preserved for review rather than discarded.

## Scope

The work is split into four independently verifiable streams. They land on the
same target branch but must remain separate commits so a failing stream can be
reverted without removing the others.

### 1. Backend Reliability Gap Audit

The current target already contains the following production contracts and
must retain them:

- provider-account priority, weight, concurrency, cooldown, adapter matching,
  per-account route overrides, request-local failed-account exclusion, and
  Codex/Grok/Sub2API import reconciliation;
- pooled-account retry before channel fallback, `401` retry without permanent
  account isolation, and fallback to another eligible channel;
- channel-affinity cleanup after retryable upstream failures;
- authorized private-group model discovery and administrator routing coverage;
- asynchronous task compare-and-swap settlement and failed-refund
  reconciliation;
- Advanced Custom `/v1/models` discovery;
- OpenAI video endpoint routing and mapped-model response privacy;
- strict persisted proxy validation and compatibility parsing for legacy proxy
  URLs.

Missing behavior is determined by contract tests, not by commit count. The
known gaps to implement are:

1. return `Retry-After` and rate-limit reset information for both Redis and
   in-memory HTTP rate-limit rejections;
2. log a bounded body preview when an upstream returns valid structured JSON
   whose parsed message is empty;
3. replace broad proxy-client cache resets with targeted invalidation when the
   proxy itself changes, while leaving credential refresh, model refresh, and
   channel status changes connection-stable.

Each additional NewAPI patch is eligible only when a failing local contract
test proves that the target branch lacks the behavior. Patch-equivalent or
weaker implementations from the rejected fusion branch are not ported.

### 2. Routing And Billing Regression Gate

Before UI work is accepted, focused suites must protect the live gateway
contracts that have caused user-visible failures:

- a failed pooled account is not selected twice in one request;
- exhausted pooled accounts allow channel fallback inside the authorized
  group;
- `401`, `429`, `5xx`, and retryable transport failures follow their configured
  account and channel retry rules;
- a stale affinity entry cannot pin a retry to the failed channel;
- private groups remain invisible to unauthorized users and discoverable to
  authorized downstream users;
- mapped upstream model names do not leak to downstream responses;
- failed asynchronous tasks refund at most once and remain reconcilable after
  a funding adjustment failure.

These are compatibility gates. The implementation must not rewrite the routing
architecture, group model, billing expression system, authentication system,
or public API shapes unless a separately approved design requires it.

### 3. Production-Lineage UI Optimization

The public home, authentication pages, brand entrance, background system,
earth treatment, Studio, typography, theme behavior, and navigation remain the
production design. Optimization must preserve the visible motion rather than
removing it.

The UI changes are limited to:

- finishing the existing below-the-fold home split so initial interaction does
  not parse and render all detail sections before the hero is ready;
- pausing canvas and WebGL loops when their surface is hidden, offscreen, or the
  document is not visible, while preserving wall-clock animation continuity;
- preventing duplicate animation loops and unnecessary React work across route
  changes;
- applying a bounded authenticated-console rendering profile without changing
  the public or Studio motion profile;
- selectively porting parent NewAPI overview, usage, cache-saving, loading,
  empty-state, and error-state improvements into the existing YuCore console
  components;
- keeping existing responsive layout, role permissions, i18n conventions, and
  theme support.

No bulk copy from the rejected fusion frontend is allowed. A parent UI change
must be adapted component by component and must pass visual and behavior review
in the current shell.

### 4. Conservative Worktree Cleanup

Cleanup removes disk-resident worktrees and local branches, not production
state. A worktree is removable only when it is clean and all needed behavior is
already present in the target branch.

Initial safe cleanup candidates are:

- `D:\yucore-protocol-billing`;
- `D:\yucore-provider-perf`;
- `D:\yucore-ui-baseline`.

The following remain until their unique patches are audited or ported:

- `D:\yucore-newapi-fusion`;
- `D:\yucore-parent-reliability`.

The following dirty worktrees are explicitly preserved:

- `D:\newapi-710-yuapi`;
- `D:\yucore-api-export`;
- `D:\yucore-dual-ui`;
- `D:\yucore-local-production`;
- `D:\yucore-ui`.

Removal uses `git worktree remove` followed by deletion of the corresponding
local branch only after ancestry and status checks pass. Generated runtime and
build caches created by this work may be removed separately after their paths
and owning processes are verified.

## Data And Request Flow

The routing flow remains:

1. authenticate the caller and resolve the authorized group;
2. select an eligible channel using group, model, priority, weight, and
   affinity rules;
3. select and lease a compatible provider account when the channel uses a
   pool;
4. apply account-specific credentials, base URL, and model mapping;
5. on a retryable failure, record the failed account, clear stale affinity,
   retry another account when available, then fall back to another eligible
   channel;
6. settle quota once after the terminal result.

No UI optimization may change this flow. Dashboard queries remain ordinary
authenticated API calls and use the current query/cache layer.

## Error Handling

- Rate-limit responses expose actionable retry timing without disclosing
  internal Redis keys or limiter marks.
- Upstream error logs store only the existing bounded and masked preview.
- Proxy parsing errors remain validation errors when saving a channel, while
  legacy persisted values continue to run through compatibility parsing.
- A failed cache invalidation closes only the affected client's idle
  connections; unrelated proxy clients remain reusable.
- UI route errors use the existing error boundary and translated empty/error
  states. Performance optimizations must fail open to a visible static
  background rather than a blank page.

## Verification

Backend verification requires:

- red-green tests for every newly ported contract;
- focused tests for `middleware`, `service`, `model`, `controller`, `relay`,
  and affected provider adapters;
- `go build ./...`;
- cross-database review for SQLite, MySQL, and PostgreSQL code paths.

Frontend verification requires:

- focused behavior tests for new scheduling or serialization logic;
- `bun run typecheck`;
- targeted lint and formatter checks for changed files;
- `bun run build` for `web/default` and `web/classic`;
- local browser inspection at desktop and mobile widths for anonymous, normal
  user, and super-admin roles;
- confirmation that home, sign-in, dashboard, keys, usage logs, wallet,
  Studio, users, channels, account pools, private-group pricing, and system
  settings render without `500` responses or blank canvases.

## Production Gate

This design authorizes local source changes, tests, builds, previews, and the
conservative worktree cleanup above. It does not authorize a push, production
image build, container restart, database migration, or deployment. Production
work begins only after the user reviews the completed local anonymous, user,
and super-admin surfaces and gives a separate approval.

## Acceptance Criteria

- The local preview remains recognizably the deployed YuCore UI.
- Existing routing, private-group, account-pool, mapped-model, video, and task
  settlement contracts continue to pass.
- Every newly identified backend gap has a failing regression test before its
  implementation and passes afterward.
- Public and Studio motion is preserved, while ordinary console interaction
  and route readiness improve without blank or stalled states.
- Only approved clean worktrees are removed; dirty worktrees remain intact.
- No production system is changed during implementation or review.
