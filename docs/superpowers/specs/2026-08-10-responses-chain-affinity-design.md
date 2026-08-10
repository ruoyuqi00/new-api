# Responses Chain Affinity Design

## Production Baseline

The only accepted code and branding baseline for this change is commit
`9da9d049b`, which is the commit running in the current production container.
The patch is developed in the existing isolated worktree and branch created
from that commit.

This patch must not modify `web/`, built frontend assets, branding resources,
theme configuration, Caddy, production containers, production data, or the
database schema. A candidate with any frontend asset difference is rejected
before deployment consideration.

## Context

The current Codex affinity rule uses `prompt_cache_key`, `Session_id`, and stable
fields from `X-Codex-Turn-Metadata`. Production aggregates show that this
affinity works when one of those values is present, while requests without an
affinity context account for most confirmed cache misses.

The affected downstream can share one YuAPI token among many final users and
concurrent conversations. Token-level affinity is therefore unsafe: it would
collapse unrelated conversations onto one channel and create a hot spot.

OpenAI Responses requests can carry stable conversation state through
`conversation` or `previous_response_id`. Both the buffered and streaming
Responses handlers already parse the authoritative upstream `response.id`.
That ID can extend the selected channel to the next request in the same response
chain without inspecting prompts or changing the upstream request.

## Goals

- Preserve all existing explicit affinity key priorities and behavior.
- Keep each Responses chain on the channel that produced its last successful
  response, even when many chains share one YuAPI token.
- Add direct support for a stable `conversation` identifier when supplied.
- Record response-chain affinity only after an authoritative successful outcome.
- Preserve request bytes, provider request semantics, billing, retries, and
  database contents.
- Keep raw conversation and response identifiers out of application logs.

## Non-Goals

- Do not use a token ID, user ID, IP address, or User-Agent as the conversation
  identity.
- Do not hash prompt content or infer a session from `instructions`, `tools`, or
  `input`.
- Do not generate or inject `prompt_cache_key`, `conversation`, or
  `previous_response_id` into the upstream body.
- Do not change channel weights, cooldowns, account-pool selection, retries,
  pricing, cache ratios, quota settlement, refund rules, or tiered billing.
- Do not claim affinity for requests that provide no stable identifier.
- Do not change frontend code or branding.

## Approaches Considered

### 1. Token-level fallback

Use the authenticated YuAPI token ID when no explicit affinity key exists. This
is simple but unsafe for a shared downstream token because all final users would
compete for the same preferred channel. Rejected.

### 2. Prompt-prefix fingerprint

Hash selected request fields such as `instructions`, `tools`, and early input.
The selected fields are not guaranteed to remain stable between turns, while
common client templates can still collapse unrelated users onto one channel.
It also expands the privacy and canonicalization surface. Rejected.

### 3. Conversation and response-chain affinity

Use an explicit conversation identifier when present. Otherwise, carry channel
affinity forward by recording a successful upstream `response.id` and looking
it up when the next request supplies that value as `previous_response_id`.
This isolates concurrent chains under a shared token and reuses the existing
authoritative stream-success gate. Selected.

## Key Resolution

The `codex cli trace` rule keeps the following resolution order:

1. request body `prompt_cache_key`;
2. request header `Session_id`;
3. `X-Codex-Turn-Metadata.session_id`;
4. `X-Codex-Turn-Metadata.thread_id`;
5. request body `conversation.id`, or the scalar `conversation` value, through a
   dedicated scoped conversation lookup;
6. a response-chain lookup for request body `previous_response_id`.

The first non-empty value wins. Existing explicit keys keep their current cache
namespace and priority, so the patch does not invalidate working affinity
entries during rollout.

`conversation` values and response IDs use distinct namespaces. Both new key
types are scoped by token, group, and model and use only a one-way fingerprint
of the opaque identifier in the storage key. They cannot collide with existing
`prompt_cache_key` or header-derived entries, and neither raw identifier becomes
part of a Redis or in-memory cache key.

## Response-Chain Storage

Conversation and response-chain entries map an opaque identifier to the selected
channel ID. Their storage keys are scoped by:

- a dedicated type-specific namespace and version;
- the authenticated YuAPI token ID;
- the effective group;
- the requested model;
- a one-way fingerprint of the response ID.

Scoping by token, group, and model prevents a guessed or repeated upstream ID or
conversation ID from crossing tenant or routing boundaries. Only the fingerprint
is used in the cache key and diagnostics; the raw ID is neither logged, stored in
the cache key, nor persisted in the database.

Entries use the existing channel-affinity cache and its bounded maximum size.
The default TTL remains 3600 seconds. Expiration affects routing locality only;
it cannot change request or billing semantics.

## Request Flow

1. Authentication establishes the token context before distribution.
2. Distribution evaluates the existing explicit key sources.
3. If no explicit source matches and `previous_response_id` is present, it
   computes the scoped response-chain key and looks up the preferred channel.
4. The normal channel eligibility, group/model support, cooldown, and
   concurrency checks still decide whether that channel is usable.
5. A missing or stale chain entry falls through to normal weighted channel
   selection. It is not an error.
6. The original request body is forwarded unchanged.

The first request in a chain can therefore distribute normally. Once it returns
an authoritative successful response ID, later turns can return to that channel.

## Success And Retry Semantics

The response ID is a candidate until the existing affinity success gate accepts
the request:

- a buffered Responses reply must parse successfully and contain a non-empty
  upstream `response.id`;
- a Responses stream may observe the ID in an early event, but the entry is not
  committed until `response.completed` or `response.done`, normal stream end,
  no recorded stream error, and an active client context;
- `response.incomplete`, `response.failed`, `response.error`, EOF without a
  terminal success marker, handler stop, `client_gone`, and context cancellation
  do not create or replace a response-chain entry.

Retry attempts reset any candidate response ID together with the existing
per-attempt stream outcome. Only the final authoritative success can commit a
mapping. Failed retries leave any previous mapping unchanged.

## Billing And Data Invariants

The response-chain index is consulted only during channel selection. It does not
participate in token counting or settlement.

- Billing continues to use actual upstream usage and `cached_tokens`.
- No synthetic cache hit or discount is created.
- Pre-consumption, final settlement, incomplete-stream preservation, and refund
  behavior remain unchanged.
- No database migration, table write, schema change, or snapshot restore occurs.
- No request or response content is stored by this feature.

## Observability

Affinity diagnostics may report the source type `conversation` or
`response_chain`, the existing short one-way key fingerprint, hit/miss outcome,
and selected channel ID. They must not report raw conversation IDs, response IDs,
request bodies, headers, tokens, or credentials.

A missing-key counter distinguishes requests that supplied none of the supported
stable identifiers. Those requests remain on normal distribution because the
server cannot safely reconstruct a conversation identity.

## Tests

Focused deterministic tests must prove:

- existing `prompt_cache_key` and header priorities are unchanged;
- scalar and object-form conversation identifiers isolate mappings correctly;
- two response chains sharing one YuAPI token can map to different channels;
- token, group, and model scopes cannot read each other's chain entries;
- a successful buffered response records its upstream response ID;
- a completed stream records its upstream response ID;
- incomplete, failed, disconnected, and canceled streams record nothing;
- an early `response.created` ID without terminal success records nothing;
- retries cannot commit a failed attempt's response ID;
- a stale or unavailable preferred channel uses existing fallback behavior;
- request body bytes remain unchanged;
- quota, cached-token accounting, and billing regression tests remain green.

The focused service, controller, OpenAI Responses, retry, stream-safety, and
billing tests run before the broader backend suite.

## Local Candidate And Rollout Boundary

Implementation remains on the isolated local branch based on `9da9d049b`.
Before any deployment discussion:

1. verify the Git diff contains no frontend, branding, schema, Caddy, or billing
   changes;
2. build a zero-frontend-change candidate from the production baseline plus this
   backend patch;
3. bind the candidate only to a private localhost port;
4. compare its frontend asset fingerprints and branded pages with the current
   production baseline;
5. run the focused API, stream, affinity, database, and billing regressions;
6. let the user inspect the local candidate.

No Caddy change, production container replacement, database restore, old-image
deletion, or traffic switch is allowed without a separate explicit user
confirmation. The current and previous known-good production images remain
available for immediate rollback.
