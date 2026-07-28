# Production Candidate Acceptance And Cutover Design

## Status

Approved in principle on 2026-07-27; written acceptance contract pending user
review before implementation planning.

## Goal

Finish one production-lineage YuCore candidate locally, prove that its frontend
performance, routing, account-pool failover, billing, private groups, and mapped
model privacy are safe, then replace the live application without exposing an
application outage. Production remains untouched until the user gives a
separate deployment approval after reviewing the evidence.

## Fixed Boundaries

- Worktree: `D:\yucore-local-production`.
- Branch: `codex/local-production-brand-performance-20260725`.
- The deployed YuCore frontend lineage remains the only UI baseline.
- Visible public and Studio motion stays enabled. Performance work changes
  scheduling and resource lifetime, not the design or animation sequence.
- Experimental UI is not a build source and is never uploaded.
- No production push, image replacement, container restart, migration, DNS
  change, Caddy reload, channel mutation, price mutation, or account-pool
  mutation is authorized by this design.

## Work Streams

### 1. Frontend Renderer Handoff

The trace shows that the entrance signal field is destroyed and compiled again
for the stable home background. The Earth shader is compiled between those two
signal-field compilations, and the below-the-fold detail tree mounts immediately
after the handoff. The telemetry-ledger dashboard component is not responsible
for these public-home tasks.

The candidate will separate WebGL resource lifetime from visible activation:

1. Create each persistent background context and shader program once.
2. Prewarm the persistent renderers behind the entrance layer during an idle
   portion of the existing sequence.
3. Activate, pause, and resume through the existing render-loop controller
   without destroying GPU resources when only `active` changes.
4. Preserve document-hidden and viewport-intersection ownership.
5. Reveal below-the-fold details in bounded idle batches so one React commit
   does not mount the entire tree during the GPU handoff.

If WebGL initialization fails, the existing CSS background remains visible.
The page must never wait indefinitely for a renderer readiness signal.

### 2. Routing, Account Pool, And Billing Gate

Existing commits provide the intended contracts, but their presence is not
accepted as proof. The current candidate must pass the contracts together:

- priority and weight apply within the highest eligible priority tier;
- a failed provider account is excluded for the rest of the request;
- another compatible account is tried before the channel is abandoned;
- exhausted pools permit fallback to another eligible channel;
- `401`, `429`, retryable `5xx`, and transport failures do not pin the request
  to stale account or channel affinity;
- retryable affinity failures clear the current affinity entry before channel
  reselection;
- private groups remain invisible to unauthorized users and discoverable to
  explicitly authorized downstream users;
- asynchronous task quota is settled or refunded exactly once;
- public model pricing remains keyed by `OriginModelName` even when routing
  uses a mapped `UpstreamModelName`;
- per-request image/video prices, token ratios, cache-read/cache-write prices,
  and tiered expressions preserve their existing contracts.

Tiered-expression checks follow `pkg/billingexpr/expr.md`: the expression is the
single pricing contract, optional token categories are normalized only when the
expression references them, `len` retains full input length, and settlement
uses the frozen pre-consume snapshot.

The audit starts with focused tests and only changes production code when a
failing contract demonstrates a real gap. It does not rewrite the distributor,
group model, billing architecture, or provider protocol surface.

### 3. Mapped Model Privacy

Mapped upstream names are an administrator routing detail, not a client API
feature. The safe default is therefore an invariant rather than an ordinary
user toggle:

- upstream requests continue to use the mapped target;
- billing and user usage records use the public requested model;
- client-facing Chat, Responses, supported SSE conversion, embedding, and
  asynchronous task result fields expose the public requested model;
- administrator-only channel configuration and routing diagnostics may retain
  the upstream target;
- unrelated nested user or tool data is never rewritten.

The OpenAI Chat, Responses, and conversion paths already have local regression
coverage. Acceptance adds a protocol inventory so unsupported response shapes
cannot silently leak a mapped target.

### 4. Zero-Downtime Cutover And Rollback

The live topology must be inspected read-only immediately before deployment.
The expected cutover uses Caddy and a temporary candidate container:

1. Record the current image digest, compose file, container health, restart
   counts, Caddy upstream, database version, and Redis health.
2. Back up the compose and Caddy files and produce an immutable candidate image
   tagged with the accepted commit.
3. Review the candidate's migration delta against the live image. Any
   destructive or non-additive schema change blocks deployment.
4. Start the temporary candidate on a loopback-only alternate port with the
   live configuration but `NODE_TYPE=slave` and batch jobs disabled. This
   prevents duplicate credential, subscription, polling, and model-update work.
5. Run health, login, refresh, authorized private-group discovery, mapped-model
   privacy, ordinary relay, streaming relay, image/video task, billing-log, and
   direct VIP endpoint checks against the alternate port.
6. Use zero-downtime Caddy reloads to keep the old healthy container serving
   while the accepted image is recreated in its final master configuration.
   Traffic moves only after the final container is healthy.
7. Keep the old image and compose backup available during the observation
   window. Rollback restores the old upstream immediately, then restores the
   old application image without changing MySQL, Redis, volumes, channels,
   groups, account pools, or prices.

If the actual server topology does not support an isolated alternate upstream
and atomic Caddy reload, the operation is reclassified as a short maintenance
restart and requires a new explicit user decision. It must not be described as
seamless.

## Verification Evidence

Frontend acceptance requires:

- focused render-loop and handoff lifecycle tests;
- default frontend tests, typecheck, changed-file lint and format checks;
- default and classic production builds;
- three serial same-machine control/candidate performance runs;
- a CPU trace showing that stable-home activation does not compile the signal
  field a second time;
- desktop and mobile light/dark screenshots for anonymous, user, and
  super-admin routes with no blank canvas, overlap, or horizontal overflow.

Backend acceptance requires:

- focused account-pool, affinity, retry, private-group, mapped-model, task
  settlement, cache billing, per-call billing, and tiered-expression tests;
- affected `middleware`, `service`, `model`, `controller`, and `relay` suites;
- `go build ./...`;
- migration and query review for SQLite, MySQL, and PostgreSQL compatibility;
- local end-to-end calls using healthy and intentionally failing test
  providers without production keys or data.

Cutover acceptance requires a written runbook containing exact health probes,
alternate-port checks, Caddy validation and reload commands, observation
signals, rollback triggers, and the previous immutable image reference. Secrets
must not be written into the runbook or command output.

## Production Gate

Production replacement may begin only when all of the following are true:

1. The local candidate commit is clean, reproducible, and reviewed in all
   three roles.
2. Frontend trace evidence proves the duplicate handoff compilation is gone
   without reduced motion.
3. Routing, account-pool, billing, private-group, and mapped-model privacy
   contracts pass together.
4. The migration delta is rollback-compatible and the server supports the
   alternate-port Caddy cutover.
5. The user reviews the final evidence and explicitly authorizes production
   replacement.

After those gates pass, the production operation itself should fit in a
15-30 minute controlled window. External traffic should remain available
throughout; the previous upstream can be restored by one Caddy reload while the
old container and image are still retained.
