# Production Relay Reliability Design

## Context

The production baseline for this work is commit `36b44efd4`, which is the source
used by the currently running production image. The new branch must start from
that commit and must not include the later detached stream-recovery candidate.

Read-only production evidence for user 81 identified two independent reliability
problems:

- after the current production cutover, 489 of 2,440 consumption requests retried
  across channels, producing 683 additional upstream attempts;
- in the Pro group, 35 of 37 requests retried and every retried request selected
  the same higher-priority channel first. That channel had cooldown disabled and
  produced mostly retryable 503 responses before the relay switched channels;
- production also emitted the HTTP/2 error stating that a request could not be
  retried after its body was written because `Request.GetBody` was missing. The
  official project has merged a replayable-body fix for retry-safe
  `REFUSED_STREAM` and graceful `GOAWAY` failures.

No duplicate consumption rows were found for the inspected user. The design must
therefore reduce unnecessary upstream attempts without changing pricing or
introducing a new refund path.

## Goals

- Give every synchronous text channel a 10-second transient cooldown by default
  after a retryable 429, 529, or 5xx upstream response.
- Preserve the existing channel/group/model cooldown scope and Redis-backed
  coordination.
- Keep image, video, Midjourney, and other asynchronous media relay modes exempt.
- Port the official replayable request-body behavior so Go can perform transport-
  safe HTTP/2 retries without surfacing them as application-level channel errors.
- Treat downstream cancellation as terminal and never start another channel
  attempt after the incoming request context is canceled.
- Keep accepted-stream billing, tiered pricing, database schema, and UI behavior
  unchanged.
- Build and verify a localhost-only candidate before one production cutover.

## Non-Goals

- No Caddy, production container, production database, or traffic change during
  implementation and local verification.
- No adaptive or escalating circuit breaker in this patch.
- No retry of an accepted response stream.
- No detached upstream draining after the downstream client disappears.
- No change to quota formulas, pre-consume estimates, tiered billing expressions,
  cache-token ratios, or refund rules.
- No provider-account affinity change in this patch.

## Considered Approaches

### 1. Code-level default cooldown plus transport and retry fixes (selected)

Treat an unset or zero channel cooldown as 10 seconds for eligible synchronous
text relay modes. Preserve positive per-channel values as explicit overrides. A
negative value remains the explicit way to disable cooldown. Port the official
independent request-body readers and stop application retry when the downstream
context is canceled.

This applies to existing and newly created text channels without a production
database rewrite and addresses both the observed unhealthy-channel loop and the
missing HTTP/2 replay contract.

### 2. Bulk-update existing channel records

Setting all current channel cooldown values to 10 seconds would mitigate the
immediate routing loop, but new channels could still default to zero and the
HTTP/2 and cancellation defects would remain. It also creates a production data
mutation before the code is locally verified, so it is rejected.

### 3. Adaptive exponential cooldown

Increasing cooldown from 10 to 30 to 60 seconds after consecutive failures would
reduce repeated probes of persistently unhealthy channels. It requires new state,
reset rules, metrics, and broader rollout testing, so it is deferred.

## Detailed Design

### Cooldown default and scope

Define one backend constant for the default synchronous channel cooldown of 10
seconds. Resolve the effective value as follows:

- configured value greater than zero: use that value;
- configured value equal to zero or omitted: use 10 seconds;
- configured value less than zero: disable cooldown explicitly.

The effective value is applied only after the existing retryable error
classification accepts the error. The cooldown key remains scoped by channel ID,
selected group, and model name, so a failure for one model does not suppress a
healthy model on the same channel.

The existing relay-mode exclusion remains authoritative. Image generation and
editing, Midjourney media modes, and video submit/fetch paths never create a
channel cooldown entry. Provider-account failures continue to use account-level
cooldown and do not cool the whole channel.

### HTTP/2 replayable request bodies

Port the official `Request.GetBody` fix as a minimal patch adapted to the
production baseline:

- body storage exposes a method that returns an independent reader starting at
  offset zero;
- memory-backed readers share immutable bytes but have independent cursors;
- disk-backed readers use independent file descriptors;
- outbound relay requests attach `ContentLength` and `GetBody` metadata without
  replacing a correct `GetBody` already derived by `net/http`;
- task requests must not replace a correct replay hook with a closure around an
  already-consumed reader;
- channel retries must bind metadata from the current attempt only.

Go's HTTP/2 transport may then replay only failures it already classifies as
retry-safe, including `REFUSED_STREAM` and graceful `GOAWAY`. This transport-level
replay does not authorize a general application retry after ambiguous acceptance.

### Cancellation and application retry

The upstream request remains bound to the incoming request context so generation
stops when the downstream client disappears. If relay failure wraps
`context.Canceled` or the incoming request context is already canceled, the error
is marked non-retryable before the controller selects another channel.

The existing accepted-stream rule remains unchanged: after upstream response
headers are accepted, stream termination settles through the stream path and does
not re-enter channel retry. The patch does not detach the upstream request from
the client context and does not drain a full completion after the client is gone.

### Billing invariants

The production billing behavior remains the contract:

- one consumption settlement per successful or accepted request;
- an accepted client disconnect remains in the existing usage-settlement path
  and never enters the controller's full-refund path;
- no second upstream application request after acceptance;
- tiered billing uses the final selected group exactly as it does in the
  production baseline;
- cooldown and transport replay never call billing functions directly.

Tests must assert that a retry-safe HTTP/2 replay does not create a second relay
settlement and that a canceled downstream request does not enter another channel
attempt.

## Verification

### Focused backend tests

- zero or omitted cooldown resolves to 10 seconds for synchronous text requests;
- a positive channel cooldown overrides the default;
- a negative channel cooldown disables it;
- retryable 429, 529, and 5xx responses create cooldown entries;
- non-retryable errors do not create cooldown entries;
- image, video, and other asynchronous media modes remain exempt;
- the next selection skips a channel during its channel/group/model cooldown;
- independent memory and disk replay readers always return the complete body;
- an HTTP/2 retry-safe stream reset transparently replays the same complete body;
- task requests never replay an empty body;
- downstream cancellation is non-retryable and starts no additional channel;
- accepted stream failures remain single-attempt and preserve billing invariants.

New or substantially rewritten Go tests use `testify/require` and
`testify/assert`.

### Repository verification

- run focused cooldown, request-body, controller, stream, and billing tests;
- run the complete Go test suite on Windows;
- run targeted Linux race tests for the modified shared packages;
- build the backend and existing frontend assets without changing the UI;
- build a candidate image from this branch only;
- bind the candidate to a private `127.0.0.1` port;
- compare public and authenticated UI surfaces against the approved production
  baseline and run API, cache, retry, and billing regression probes.

## Rollout Boundary

Implementation and verification are local only. Production retains the current
image and container baseline. After the user approves the localhost candidate,
deployment uses one controlled container switch with no database migration and no
Caddy configuration change. Health, UI, API, retry, cache, and billing checks run
immediately after cutover. Any regression triggers an immediate rollback to the
preserved production image.
