# Scanner, Cache Key, and Upstream Error Safety Design

## Context

The production candidate generated a large number of `scanner_error` outcomes even though many OpenAI-compatible streams had already delivered authoritative terminal usage. The stream-recovery lifecycle cancels its upstream context after terminal usage is accepted; the scanner observes that internal cancellation and records it as a transport failure. Because stream status is first-write-wins, the controller then refuses the normal channel-affinity commit and retains only a short provisional response-chain mapping, if a response ID was observed at all.

Requests already preserve an explicit `prompt_cache_key` on native `/v1/responses` forwarding and already normalize `input_tokens_details.cached_tokens`. The remaining compatibility gap is requests without an explicit cache key but with a stable session signal. Any automatic key must be opt-in, session-scoped, and must not expose the caller's raw session identifier upstream.

Upstream request-field passthrough and upstream error passthrough are separate contracts. Cache-related request fields may be forwarded, while public errors must never reveal upstream URLs, hosts, IP addresses, channel/provider identifiers, credentials, authorization values, request headers, redirects, or raw response bodies.

## Goals

1. Stop classifying the relay's own post-terminal cancellation as `scanner_error`.
2. Allow a genuinely completed stream to perform the existing full channel-affinity commit.
3. Preserve explicit client `prompt_cache_key` values unchanged.
4. Add an opt-in compatibility switch that derives a stable cache key only when a configured stable session source exists and the client omitted the key.
5. Keep upstream diagnostics available for internal routing decisions while returning only sanitized errors to downstream clients.
6. Preserve the production brand UI and all existing billing, database, retry, and pricing behavior.

## Non-goals

- No Caddy, Cloudflare, DNS, production-container, database-data, schema, billing-expression, tokenizer, or UI changes.
- No full-request or full-conversation hash as a cache key.
- No global cache-key injection for every provider or endpoint.
- No multi-key or provider-account pinning in this patch. The audited production window used single-key channels and no provider-account pool, so that is not the confirmed cause of this incident.
- No automatic retry, refund, or channel switch for accepted or ambiguous requests.
- No fabrication of confirmed cached-token usage.

## Considered Approaches

### A. Exact-terminal normalization plus rule-scoped key injection (selected)

Treat a scanner read error as normal completion only when the stream-recovery snapshot already proves exact terminal usage and completed recovery. Keep the existing controller success checks and let them perform the full affinity commit. Add an explicit affinity-rule switch that derives a private, stable `prompt_cache_key` from a configured session source when the request omitted one.

This directly fixes the observed false failure without hiding real scanner errors, keeps existing explicit keys intact, and limits compatibility behavior to administrators who know the upstream supports the field.

### B. Ignore all context-canceled scanner errors

This is smaller but unsafe. A real downstream disconnect or upstream cancellation before terminal usage would be mislabeled as success and could commit a failed route. It is rejected.

### C. Always inject a hash of the complete request body

This avoids configuration but changes every turn as conversation history grows, so it destroys session stability and can fragment the provider cache. It can also cause unsupported providers to reject requests. It is rejected.

## Design

### 1. Exact-terminal scanner normalization

The stream scanner keeps its current error behavior unless all of the following are true:

- stream recovery is enabled;
- the recovery snapshot reports `usage_state=exact`;
- the recovery snapshot reports `drain_result=completed`;
- the error happened after the terminal callback marked that exact state.

In that proven state, the scanner records `done` with no error instead of `scanner_error`. The rule is based on authoritative lifecycle state, not on matching an error string, because cancellation may surface as `context canceled`, a closed HTTP/2 body, or another transport-specific read error.

Pre-terminal cancellation, EOF without a required terminal marker, timeout, parser errors, handler-stop, and downstream disconnect behavior remain unchanged. For Responses streams, `StreamTerminalSuccess` and `StreamTerminalUsageSeen` remain required by the controller, so an error envelope or empty usage cannot become a successful affinity commit.

### 2. Full affinity commit

No new affinity writer is added. After exact-terminal normalization, the existing controller predicate must see:

- an active downstream request context;
- `StreamTerminalSuccess=true` when terminal markers are required;
- a normal stream end;
- no recorded stream errors.

The existing normal commit then refreshes the configured primary affinity key and response-chain key with the configured full TTL. Incomplete outcomes continue to use only the bounded provisional response-chain behavior when an actual response ID was observed. Existing mappings are not deleted on failure.

### 3. Safe prompt-cache-key injection

Each channel-affinity rule gains an `inject_prompt_cache_key` boolean, defaulting to false for persisted/custom rules. The built-in Codex `/v1/responses` rule declares the flag as true in its default value, but an existing persisted operation setting is not silently rewritten. Enabling the behavior in production remains an explicit configuration action after local verification.

Injection occurs only when:

- the matched affinity rule has the switch enabled;
- the request is a native OpenAI-compatible `/v1/responses` request;
- the parsed request has no non-empty `prompt_cache_key`;
- the matched rule produced a stable configured source such as `Session_id`, Codex turn metadata session/thread ID, or another explicit session/conversation source.

The derived key is an HMAC-SHA-256 value using the server secret and a versioned payload containing token ID, group, model, source identity, and source value. Only a bounded encoded digest with a gateway prefix is sent upstream. The raw session value is neither injected nor added to public logs. Explicit client keys always win and are forwarded byte-for-byte.

The implementation never derives a key from the entire mutable request body, messages, current timestamp, metadata blob, or tool ordering. The switch does not affect `/v1/chat/completions`, media APIs, async tasks, or providers that are not using the native Responses request path.

### 4. Public upstream-error projection

The relay preserves the upstream HTTP status and internal error classification for retry, cooldown, and operator diagnostics. Before serializing an upstream-originated error to a downstream client, it projects the error to a stable public code/type/message chosen by status category. Locally generated validation errors keep their existing actionable messages.

The public projection contains no raw upstream message. Internal diagnostics use the existing bounded/redacted logging path and are never placed in API response JSON or SSE data. Responses terminal error events continue to preserve the protocol's public top-level shape while replacing provider text with gateway-owned code and message.

This separation means enabling request-field passthrough or cache-key injection cannot enable error passthrough.

### 5. Configuration and rollout

The code change contains no schema migration and no frontend modification. Local tests exercise the switch by constructing settings directly. A local candidate is built from the production-aligned branch and its homepage/static fingerprints and branded sign-in surface are compared with the accepted local baseline.

Production keeps the rolled-back container until the user separately approves the candidate. At deployment time, the error/scanner fix can run with key injection disabled. The key switch is enabled only for the intended affinity rule and verified with a bounded request before wider observation.

## Test Strategy

Deterministic Go regression tests cover:

1. terminal usage followed by recovery cancellation ends as `done`, records no scanner error, and preserves the authoritative usage state;
2. the same transport error before terminal usage remains `scanner_error`;
3. a completed Responses stream satisfies the controller's full affinity-commit predicate and refreshes the normal mapping;
4. incomplete and client-gone streams cannot perform a normal affinity commit;
5. explicit `prompt_cache_key` is never overwritten;
6. enabled rule plus stable session source injects the same digest for the same token/group/model/session and a different digest across isolation scopes;
7. disabled rule, missing stable source, unsupported path, and unsupported relay form inject nothing;
8. derived keys do not contain the raw session value;
9. seeded structured upstream errors containing a URL, IP, provider/channel name, key, authorization text, redirect, and raw body produce a generic public error without those values;
10. local validation errors remain actionable, and retry/cooldown classification still sees the original status/category.

Focused tests run first, followed by full tests for affected packages, `go vet` on those packages, `git diff --check`, candidate build, and local brand/UI fingerprint verification.

## Acceptance Criteria

- A valid terminal stream cannot become `scanner_error` merely because stream recovery cancels its reader after exact usage.
- Successful completed streams refresh full channel affinity; failed or incomplete streams do not.
- Explicit client cache keys are preserved, and opt-in derived keys are stable, scoped, private, and absent when unsafe.
- Confirmed cached-token accounting still comes only from authoritative upstream usage.
- Public API and SSE errors contain no upstream infrastructure, credentials, or raw response content.
- Billing, retry, database compatibility, pricing, themes, branding, animation, canvas, documentation, and model configuration remain unchanged.
- No production traffic or configuration changes occur without a separate user confirmation after local verification.
