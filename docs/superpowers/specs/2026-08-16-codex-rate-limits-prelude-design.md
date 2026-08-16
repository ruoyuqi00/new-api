# Codex Rate-Limits Prelude Design

## Goal

Give downstream Codex clients an immediate, uniform first SSE metadata event
after the upstream has accepted a streaming Responses request, while preventing
upstream account-plan and quota metadata from reaching any downstream client.

## Scope

This behavior applies only to successful streaming `/v1/responses` requests
whose original downstream model name starts with `gpt-`, case-insensitively.
This model gate keeps Gemini, Claude, video, image, and all other non-GPT
traffic unchanged. Within that GPT-only scope, the gateway identifies a Codex
client from the original downstream request when at least one of these signals
is present:

- `Originator` contains `codex`, case-insensitively;
- `User-Agent` contains `codex`, case-insensitively;
- `X-Codex-Turn-Metadata` is non-empty;
- `X-Codex-Beta-Features` is non-empty.

`Session_id` alone is not sufficient because it is not uniquely identifying.
Non-streaming requests, other endpoints, and non-GPT models are unchanged.

## Fixed downstream event

After upstream HTTP status `200` is known, but before reading the upstream SSE
body, the gateway writes and flushes exactly one event for matching Codex
clients:

```text
event: codex.response.metadata
data: {"type":"codex.rate_limits","plan_type":"pro","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":null},"credits":null}

```

The event means only that the current request passed the upstream HTTP
acceptance boundary. It does not claim a real upstream plan, remaining quota,
credit balance, or reset time. `primary`, `secondary`, and `credits` remain
`null` rather than carrying fabricated usage data.

## Filtering and pass-through

For GPT Responses traffic, every upstream SSE payload whose JSON `type` is
`codex.rate_limits` is dropped, regardless of downstream client type. The
synthetic event is emitted only for matching Codex clients. Non-GPT traffic
retains its existing pass-through behavior. All other valid Responses events
continue through the existing normalization, privacy filtering, affinity-ID
observation, terminal usage collection, incomplete-stream handling, and
billing paths.

The early flush must preserve the already supported `X-Reasoning-Included` and
`X-Codex-Turn-State` response headers. Header preparation therefore becomes an
idempotent shared operation that both the prelude and stream scanner can call.

## Behavioral boundaries

- Upstream non-200 responses retain their original error handling and never
  receive a synthetic success event.
- Non-GPT models neither receive the synthetic event nor use the new upstream
  event filter.
- The synthetic event does not update upstream first-response timing or token
  counters.
- Channel affinity remains request-side and is unchanged.
- Terminal usage remains sourced from `response.completed`/`response.done`.
- The event improves time to first downstream SSE event, not time to first text
  delta or model generation latency.
- A downstream write failure during the prelude stops downstream streaming and
  preserves pre-consumed quota under the existing accepted-stream rules.

## Verification

Automated tests must prove exact first-event ordering and payload, all accepted
Codex signatures, no injection for ordinary GPT clients, GPT-only suppression
of upstream `codex.rate_limits`, unchanged non-GPT pass-through, preservation
of normal Responses events and terminal usage, single emission, and
preservation of Codex response headers.
