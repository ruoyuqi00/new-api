# Domestic Model Groups Design

> **Status:** approved design for local implementation only. Production database and traffic remain unchanged.

## Goal

Add two user-visible domestic-model groups while preserving the production UI baseline and all existing billing, task billing, channel affinity, violation-fee, cache-usage, and actual-response-model behavior.

## User-visible groups

The groups are named exactly:

- `国模按量`
- `国模按次`

`国模按量` exposes the six provider-facing names:

- `deepseek-v4-flash-0731`
- `deepseek-v4-pro-0813`
- `glm-5.2`
- `kimi-k3`
- `MiniMax-M2.7`
- `qwen3.8-max`

`国模按次` exposes only the corresponding `-call` aliases:

- `deepseek-v4-flash-0731-call`
- `deepseek-v4-pro-0813-call`
- `glm-5.2-call`
- `kimi-k3-call`
- `MiniMax-M2.7-call`
- `qwen3.8-max-call`

The aliases are public model identifiers and remain in request/log fields such as the origin model. Before an upstream request, the existing model-mapping path restores the provider model name. Existing `ActualResponseModel` capture records the model reported by the provider independently of the public alias.

## Upstream channels

The authenticated, read-only model discovery endpoint is:

`https://api.herohao.top/v1/models`

Both administrator-provided keys returned HTTP 200 and exposed all six target identifiers. The keys must be supplied at channel-creation time through the admin configuration flow or process secrets. They are never committed to source, documentation, snapshots, logs, tests, or client-visible responses.

Two channels are prepared conceptually:

- the usage channel uses the按量 key, group `国模按量`, and provider-facing model mappings;
- the call channel uses the按次 key, group `国模按次`, and `-call` mappings to the same provider-facing names.

The implementation must not create either production channel until the administrator supplies the base URL and confirms the local candidate. A local verifier may read the keys from environment variables without persisting them.

## Pricing architecture

Both groups use the existing `tiered_expr` billing mode. This avoids a second text billing engine and keeps pre-consumption, authoritative settlement, frozen billing snapshots, usage normalization, and audit logging on the existing path.

The group ratio is exactly `0.3` for both groups. Expressions contain provider prices only; group ratio is applied once by the existing quota conversion:

`quota = expression_output / 1,000,000 * QuotaPerUnit * group_ratio`

### Usage group

The six provider-facing models use expressions with explicit input (`p`), output (`c`), cache-read (`cr`), cache-write (`cc`/`cc1h` where applicable), context (`len`), and Beijing-time schedule branches only when those values are verified by an official provider source. Context branches use `len`, never cache-reduced `p`.

### Call group

The six aliases use expressions that return a fixed CNY/USD-normalized per-request amount selected by `len` context tiers. The expressions intentionally do not reference `p`, `c`, `cr`, or cache-write variables, so output-token volume cannot turn a single call into a token-priced request. Settlement still uses the existing frozen reservation when a provider does not return authoritative terminal usage.

### Price verification states

Every price entry carries a source and verification state in the local catalog:

- `verified`: exact model, region/currency, and cache/peak semantics are supported by the cited official documentation;
- `pending`: the provider model alias, region, cache rule, currency conversion, or peak schedule cannot be established from official documentation.

Pending entries are shown in the local audit output but are excluded from enabled abilities and production channel mappings. No runtime web lookup is used for pricing.

Current audit state:

- MiniMax M2.7 input/output/cache prices are documented by MiniMax;
- GLM-5.2 input/output/cache prices and one-million-token context are documented by Zhipu;
- Qwen3.8-max input/output and implicit-cache details are documented by Alibaba, subject to region/currency confirmation;
- the requested DeepSeek date-suffixed identifiers are visible upstream, but the cited DeepSeek pricing page uses family names and does not provide a verified Beijing peak-time schedule; these entries remain pending until the mapping and schedule are confirmed;
- Kimi-K3 pricing is pending until an official pricing source is available.

## Mapping and logging invariants

1. A `-call` alias must resolve to exactly one provider model before dispatch.
2. The outbound JSON model field must be the provider model, never the alias.
3. Origin/public model fields must retain the alias for user-facing logs and audit.
4. `ActualResponseModel` must retain the provider response model when present.
5. Channel affinity keys remain scoped by the existing token/group/model/session rules; aliases must not collapse distinct public models.
6. Upstream error projection remains sanitized and must never expose provider URLs, keys, response bodies, or request headers.

## Local validation matrix

The local candidate must verify, without a paid completion request:

- both keys authenticate to `/v1/models` with HTTP 200;
- all six target identifiers are present for both keys;
- `/api/pricing` exposes both groups;
- the usage group exposes only provider-facing names;
- the call group exposes only `-call` aliases;
- each alias maps outbound to the provider-facing model;
- a simulated one-million-token input/output request applies the group ratio exactly once;
- cache-read and cache-write vectors follow the usage expressions;
- context-boundary vectors select the intended call and usage tiers;
- DeepSeek normal/peak branches are disabled while unverified;
- existing billing, task billing, affinity, actual-response-model, and violation-fee tests remain green.

The test fixture uses synthetic responses and redacts credentials. It must not call a paid completion endpoint.

## Rollback and production gate

Before any production write, record an `/api/pricing` JSON snapshot and a hash of the snapshot, plus a secret-free channel/ability/group export. Apply group, pricing, ability, and channel changes in one transaction or an equivalent compensating transaction. The rollback bundle restores those snapshots and removes only the newly created domestic-model records. Production traffic and Caddy remain untouched until the user explicitly approves a local candidate and the post-cutover health/price checks.

