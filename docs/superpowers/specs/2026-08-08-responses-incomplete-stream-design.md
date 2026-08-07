# Responses Incomplete Stream Design

## Context

Production evidence shows that some accepted `/v1/responses` streams end without a
protocol terminal event. The shared scanner currently classifies a clean upstream
EOF as a normal transport ending, while the OpenAI Responses adapter does not track
whether it received `response.completed` or another terminal Responses event. A
client therefore sees the connection close without a terminal event and reports
`stream disconnected before completion`.

The upstream may already have billed any request for which it returned HTTP 200.
Such a stream must never be replayed automatically and must not enter the existing
relay error path that refunds pre-consumed quota.

## Goals

- Give a Responses client a protocol-level terminal failure when an accepted
  upstream stream ends before any Responses terminal event.
- Preserve exactly one upstream attempt after an HTTP 200 stream has been accepted.
- Preserve the existing billing and usage estimation path, including no automatic
  refund for an accepted but incomplete stream.
- Stop writing after a downstream write or flush failure.
- Propagate downstream request cancellation to the upstream HTTP request.
- Keep the change local to transport and Responses protocol handling.

## Non-Goals

- No retry or replay after an accepted upstream stream.
- No pricing, pre-consume, settlement, refund, or database changes.
- No attempt to manufacture missing upstream usage fields.
- No Caddy, container, production configuration, or traffic changes.
- No promise that an unhealthy upstream will complete; the change makes failure
  deterministic and observable without creating duplicate upstream charges.

## Considered Approaches

### 1. Track completion in the Responses adapter (selected)

The adapter understands Responses event semantics, so it records whether it saw
`response.completed`, `response.done`, `response.incomplete`, `response.failed`, or
`response.error`. After the shared scanner returns, an upstream EOF or abnormal
transport ending without one of those events causes one synthetic
`response.failed` SSE event to be sent when the downstream is still writable.

This has the smallest protocol blast radius and leaves Chat Completions and other
providers unchanged.

### 2. Make the shared scanner require a terminal event

The scanner could accept a protocol-specific completion predicate. This would
centralize the check, but it would add protocol knowledge or configuration to a
shared helper used by many providers with different terminal rules. It is not
needed for this incident.

### 3. Retry an incomplete stream

This could hide some failures, but it is rejected. Once upstream accepted the
request, replay can create a second bill and duplicate side effects even when no
visible output reached the client.

## Detailed Design

### Terminal tracking

`OaiResponsesStreamHandler` tracks whether it observes one of these terminal event
types:

- `response.completed`
- `response.done`
- `response.incomplete`
- `response.failed`
- `response.error`

Existing events continue to pass through unchanged. `response.incomplete` remains
a legitimate upstream terminal event and is not replaced with a synthetic failure.

After `StreamScannerHandler` returns, the adapter sends a synthetic failure only
when all of the following are true:

- no Responses terminal event was observed;
- the downstream request context is still active;
- the scanner did not end because the downstream client disappeared or because a
  downstream write failed.

The synthetic event is standard Responses SSE framing:

```text
event: response.failed
data: {"type":"response.failed","sequence_number":<next>,"response":{"status":"failed","error":{"code":"server_error","message":"Upstream stream ended before completion."}}}
```

This follows the official OpenAI `response.failed` event shape. When upstream sent a
sequence number, the synthetic event uses the next number. Known response ID and
model metadata may be copied from an earlier event. No input, output, credential,
or request-header data is included.

### Downstream write failures

`sendResponsesStreamData` returns the error from `helper.ResponseChunkData` instead
of discarding it. The callback calls `StreamResult.Stop` on failure, which closes
the upstream response body and prevents stale writes. A failed downstream write
does not trigger a synthetic event because the connection is already unusable.

### Upstream cancellation

HTTP requests created by the relay use the incoming request context. When the
downstream disconnects, Go cancels the in-flight upstream request in addition to
the scanner closing the response body. Request payloads and headers are otherwise
unchanged.

### Billing invariant

An incomplete accepted stream returns usage and a nil relay error, just like the
current accepted-stream path. The controller therefore performs normal settlement
and does not enter retry or refund handling. Usage extraction and fallback token
estimation remain unchanged.

Pre-header HTTP failures retain the existing retry and refund behavior because the
upstream stream was not accepted.

## Testing

Tests use `testify/require` and `testify/assert` and cover observable contracts:

- a normal `response.completed` stream is forwarded without a synthetic failure;
- an EOF without a terminal event emits exactly one `response.failed` event;
- an upstream `response.failed` event is not duplicated;
- a downstream write failure stops the stream and does not append a synthetic
  failure;
- an incomplete accepted stream returns a nil relay error, which is the controller
  contract that prevents retry and refund;
- existing usage and cached-token extraction remain unchanged;
- cancellation of the incoming request cancels the upstream HTTP request context.

Focused package tests run first, followed by the repository backend test suite.

## Rollout Boundary

This design produces a local patch and local candidate only. It does not modify
production. Any later deployment requires explicit user confirmation and must keep
the current production image available for rollback.
