# Upstream Usage Safety Design

## Goal

Prevent malformed upstream token usage from becoming a user charge while
preserving the gateway's cost protection when an upstream request may already
have been accepted.

## Scope

This change covers GPT text requests only: OpenAI Responses, Chat
Completions, and their existing compatibility/conversion paths. Image, video,
audio, and asynchronous media billing are unchanged.

No production database repair, balance mutation, route change, Caddy change,
or provider credential change is part of this change.

## Usage Trust Boundary

Every upstream usage payload entering text settlement must pass one shared
validation policy:

- all token fields are non-negative;
- `total_tokens`, when present, is consistent with input plus output;
- cache read/write fields do not exceed the corresponding input total;
- input/output values are below a configured hard safety ceiling suitable for
  a single text request;
- zero or structurally incomplete terminal usage is unconfirmed, not
  authoritative.

Invalid usage is retained only for internal diagnostics. It is never marked as
`upstream`, used for tiered pricing, or used to update confirmed cache-hit
statistics.

## Settlement Rules

- Valid terminal usage: settle once from upstream usage and record confirmed
  usage/cache statistics.
- Invalid usage, missing terminal usage, accepted stream failure, or ambiguous
  submission: do not retry or switch channels; settle at most the frozen
  pre-consume reservation using a locally estimated prompt/output basis.
- The fallback never trusts malformed upstream token fields and never refunds
  an accepted/ambiguous submission solely because usage is missing.
- Settlement and consumption logging remain idempotent through the existing
  billing/submission coordinator.

## Error and Privacy Behavior

The downstream response continues to use the existing gateway-owned error
projection. Raw upstream usage/error bodies, provider URLs, credentials, and
request identifiers are not exposed.

## Cache and Affinity Behavior

Unconfirmed usage is excluded from confirmed cache-hit counters. Existing
prompt-cache-key/session affinity remains intact, and invalid usage does not
trigger an affinity switch or a second upstream submission.

## Verification

Add deterministic regression tests for:

- normal valid usage;
- zero/empty usage;
- negative and inconsistent totals;
- cache tokens greater than input;
- `MaxInt64` and geometric usage amplification;
- invalid usage on Responses and Chat/compatibility streams;
- accepted/ambiguous settlement using the frozen reservation exactly once;
- no retry, no channel switch, no confirmed cache-stat update;
- unchanged image/video paths.

The implementation is complete only after focused relay/service tests, full
`go test` for touched packages, `git diff --check`, and a local UI smoke check
against the existing production-brand baseline.
