# GPT Stream Estimated Usage Design

## Goal

Keep downstream GPT text billing aligned when an accepted upstream stream ends before a trusted terminal usage event, without changing image/video behavior or exposing upstream errors.

## Scope

This design applies only to GPT/OpenAI-compatible text streaming paths: Chat Completions, legacy Completions where the existing stream handler is used, and Responses/Responses-to-Chat conversion. Non-stream text, Claude, image, video, audio, and task endpoints keep their existing behavior unless they already share a tested GPT text helper.

## Behavior

When the gateway has accepted a 2xx upstream stream and the stream ends without authoritative terminal usage:

1. Preserve the text and reasoning content already safely received from upstream.
2. Estimate input tokens from the existing request metadata/tokenizer path.
3. Estimate output tokens from the accumulated text/reasoning/tool-call material using the existing model-aware tokenizer path.
4. Use one immutable estimated usage snapshot for both downstream serialization and gateway settlement.
5. If the downstream request is still connected, emit a gateway-owned terminal event:
   - Chat Completions: usage-only final chunk with `finish_reason: "length"`, then normal `[DONE]`.
   - Responses: `response.incomplete` carrying the estimated usage and a gateway-owned public reason.
6. Mark the usage as estimated and preserve the existing pre-consumed quota ceiling. Never claim that estimated values came from the upstream provider.
7. If the downstream client is already gone, do not write a response. Settle internally from the same snapshot, do not retry, and do not refund an accepted upstream submission.

An upstream error body, URL, credentials, request headers, provider response ID that was never public, and raw provider diagnostics must never be copied into the synthetic terminal event.

## Billing Invariants

- Authoritative terminal usage remains the only basis for authoritative tiered settlement.
- Estimated settlement is explicitly marked and cannot exceed the frozen reservation for accepted/ambiguous submissions.
- The estimated output token count is never silently replaced with zero merely because the provider omitted usage.
- Existing violation fee, channel affinity, billing expression, task billing, and logging paths remain unchanged.
- Images and videos are excluded from this behavior.

## Data Flow

The stream handler owns collection of output material and construction of the estimated `dto.Usage`. A small shared terminal-emission helper serializes the already-normalized usage for the requested GPT text protocol. The controller/service path continues to call the existing accepted-stream settlement coordinator, but receives the same usage object that was emitted downstream. No schema or database migration is needed.

## Failure Handling

- Empty accepted stream: estimate input only; emit the gateway-owned incomplete terminal event when the client remains connected.
- Partial text stream: estimate input plus accumulated output/tool text.
- Client cancellation: skip response writes, settle once internally, and suppress retry/refund.
- Serialization/write failure: retain the internal estimate and settlement; do not leak upstream details.
- Non-accepted upstream failure: keep existing retry/refund classification; this design does not convert it into a successful response.

## Verification

Tests must cover Chat Completions and Responses with: partial output, empty output, missing usage, provider error payload sanitization, connected downstream terminal emission, disconnected downstream no-write behavior, estimated usage reuse for settlement, and unchanged image/video routing. Existing relay, service, billing expression, affinity, violation fee, and model test suites must remain green.
