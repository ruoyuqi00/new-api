# Hide Upstream Model Mapping Design

## Goal

Keep a channel's model mapping private from ordinary API consumers. A caller
should see the public model name it requested in supported API responses,
regardless of the upstream model selected by the channel mapping.

Administrators retain enough internal audit data to troubleshoot routing and
billing. This work does not change billing: quota remains based on the public
model name already carried by `OriginModelName`.

## Evidence

`relay/helper.ModelMappedHelper` preserves `OriginModelName` and changes only
`UpstreamModelName`. Billing consistently uses `OriginModelName`. Several
adapter and conversion paths then serialize `UpstreamModelName` into a client
response's `model` field, which reveals the private mapping.

## Chosen Approach

Normalize client-facing model fields at the response-construction boundary:

1. Add one narrowly scoped helper that returns the public response model name
   from `RelayInfo.OriginModelName`, falling back to the existing model only
   when the origin is unavailable.
2. Apply it only to response DTOs and SSE events that explicitly expose a
   `model` field: OpenAI-compatible chat, Responses conversion, embeddings,
   and supported asynchronous task result responses.
3. Keep `UpstreamModelName` unchanged for request construction, provider
   routing, task polling, internal debug logs, and the administrator-only
   consume-log metadata.

This avoids a raw HTTP/SSE string rewrite. Such a global rewrite would be hard
to constrain safely and could mutate user content, tool arguments, or native
provider payloads.

## Behavioural Contract

For a request for `public-model` mapped to `upstream-model`:

- The request sent upstream uses `upstream-model`.
- The API response and streamed completion events expose `public-model`.
- A task's client-visible result exposes `public-model`.
- Billing, user-visible consumption records, and pricing remain keyed by
  `public-model`.
- Admin-only routing diagnostics may retain the mapped upstream model name.

Unmapped requests retain their existing response model value.

## Scope Boundaries

- No experimental UI, poster assets, generated output, or `web/experimental`
  changes.
- No production channel, group, mapping, account-pool, or pricing data
  changes.
- No raw-body mutation for unconverted native provider responses.
- No removal of administrator audit data.

## Verification

Add focused regressions for a mapped request through the supported response
shapes. Each verifies that upstream request construction still uses the mapped
target while the client response uses the public source model. Cover both
non-stream and streaming output where the protocol exposes a model field.

Run the affected relay/package suites, then the broader backend test suite and
`git diff --check`. Production deployment remains a separate, backend-only
step after test review.
