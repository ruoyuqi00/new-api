# Input Moderation Audit Design

## Goal

Audit user input before selecting an upstream channel, block policy-flagged requests, and charge the amount the original request would have reserved. Do not deploy account-registration automation or rotate through keys from multiple accounts.

## Scope

- Cover the main synchronous relay flow after request parsing and pricing: OpenAI Chat/Completions/Responses, Claude, Gemini, and image prompts handled by `controller.Relay`.
- Skip realtime WebSocket sessions, explicit `/v1/moderations` relay requests, and asynchronous task controllers. Those paths have different input extraction and billing lifecycles.
- Send only `TokenCountMeta.CombineText`; do not inspect generated output.
- Do not store or log the input text, the moderation API key, the Authorization header, or the moderation response body.

## Moderation Provider

Use the official `POST https://api.openai.com/v1/moderations` endpoint with `omni-moderation-latest`. The endpoint accepts a text input and returns `results[].flagged` plus category booleans. OpenAI documents moderation models as free, but rate limits still apply.

Configuration is environment-only:

- `INPUT_MODERATION_ENABLED`: disabled by default.
- `INPUT_MODERATION_API_KEY`: a single independently managed OpenAI API key.
- `INPUT_MODERATION_MODEL`: defaults to `omni-moderation-latest`.
- `INPUT_MODERATION_TIMEOUT_SECONDS`: defaults to 3 seconds.

The endpoint remains fixed to the official OpenAI host. This avoids turning an environment value into an arbitrary outbound request target.

## Request Flow

1. Parse and validate the downstream request.
2. Build the full token metadata when moderation is enabled.
3. Estimate prompt tokens and calculate `PriceData` using the existing pricing path.
4. Apply the existing local sensitive-word rule first. A local match does not call OpenAI.
5. For non-empty input, call OpenAI Moderation before pre-consume and channel selection.
6. On a non-flagged result, continue the existing relay flow unchanged.
7. On timeout, transport failure, non-2xx status, malformed JSON, or an empty result array, log only a sanitized reason and continue the existing relay flow (fail open).
8. On a flagged result, return a stable non-retryable error, charge the request, write a structured consume log, and stop before channel selection.

## Billing

A blocked request has no upstream completion, so a true post-response cost cannot exist. The deterministic charge is `PriceData.QuotaToPreConsume`, already calculated for the original request from its model, user group, requested output allowance, per-call multipliers, and tiered billing expression.

- Do not use the Grok fixed violation amount.
- Do not apply a second multiplier or penalty.
- Use the existing billing session so wallet and subscription preferences remain consistent.
- Record user/token usage but no channel usage because no upstream channel was selected.
- If the user cannot cover the charge, keep the request blocked and write an audit record indicating that charging failed.
- A free request records a zero-quota block and still increments request count.

## Audit Record

Write a consume log with channel ID `0`, the stable error code, moderation model, flagged category names, requested quota, charged quota, charge success, request ID, model, group, token ID, and stream flag. Never include category scores or request content.

## Error Contract

Return HTTP 400 with error code `violation_fee.input_moderation`, a generic policy message, and skip-retry metadata. Do not expose which category matched in the client error; category names are limited to the structured operator log.

## Verification

- Unit tests cover disabled, allowed, flagged, timeout/non-2xx, malformed, and empty-result moderation behavior.
- Billing tests prove the exact supplied request quota is deducted once from wallet and token, no channel usage is added, and sanitized metadata is recorded.
- Controller tests prove flagged input never selects a channel and moderation failure proceeds to normal channel selection.
- Existing service, controller, middleware, relay, model, and constant tests remain green.
- Production deployment remains disabled until a single API key is configured, then a benign live request must pass and a controlled test input must block without reaching an upstream channel.

## References

- https://platform.openai.com/docs/api-reference/moderations
- https://developers.openai.com/api/docs/models/omni-moderation-latest
