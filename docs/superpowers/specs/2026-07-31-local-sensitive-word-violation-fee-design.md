# Local Sensitive-Word Violation Fee Design

## Goal

Charge the configured fixed violation fee when YuAPI blocks a prompt locally
because it contains a configured sensitive word. The request must never reach
an upstream provider, but the local rejection must still have a financial
deterrent.

## Current Behavior

YuAPI checks configured sensitive words before token estimation, pricing, and
normal pre-consumption. A match returns `sensitive_words_detected` immediately,
so no upstream request and no charge occur.

The existing violation-fee path only recognizes upstream Grok CSAM markers. It
refunds normal pre-consumption and then charges the configured fixed violation
amount.

## Approved Behavior

- A local sensitive-word match is rejected before channel selection and before
  any upstream request.
- The request is charged exactly once using the existing configured violation
  amount multiplied by the resolved effective group ratio.
- The request is not charged the estimated or normal model usage price.
- The client-facing error remains `sensitive_words_detected` for compatibility.
- The charge is recorded as a separate consume log with a local-sensitive-word
  violation reason. Matched words and prompt content are not written to that
  billing log.
- The rejection is never retried.
- If the fee cannot be charged, the prompt remains blocked and the fee failure
  is recorded server-side without replacing the client-facing violation error.

## Request Flow

1. Parse and validate the request and build `RelayInfo`.
2. Run the configured sensitive-word check and retain only the boolean match
   for billing control flow.
3. Estimate prompt tokens and run `ModelPriceHelper` so the effective automatic
   group and group ratio are resolved consistently with normal billing.
4. If no sensitive word matched, continue through the existing normal
   pre-consumption and relay path unchanged.
5. If a sensitive word matched, calculate the fixed violation quota, charge it
   through the existing billing-source selection, record one violation consume
   log, return HTTP 400 with `sensitive_words_detected`, and do not select a
   channel.

This ordering performs local parsing and pricing only. It does not upload the
request body to any provider.

## Billing Rules

The fee calculation remains:

```text
configured violation amount * QuotaPerUnit * effective group ratio
```

The existing Grok violation setting remains the configuration source for this
change to avoid a configuration migration. Wallet/subscription selection must
follow the user's normal billing preference. A local violation has no upstream
channel, so channel usage quota is not incremented; user/token usage and the
dedicated consume log are incremented only after the fee charge succeeds.

## Error And Retry Semantics

- Local match: HTTP 400, code `sensitive_words_detected`, skip retry.
- Upstream Grok CSAM: existing normalized
  `violation_fee.grok.csam` behavior remains unchanged.
- Fee failure: retain the local violation response and emit an internal billing
  error log.
- One request can produce at most one violation-fee consume record.

## Tests

Backend regression tests must prove:

- a local match is not sent to channel selection or an upstream handler;
- the fixed fee uses the effective group ratio;
- normal model pre-consumption is not charged for the rejected request;
- one local match creates one fee charge and one consume record;
- insufficient quota still blocks the request and does not create a successful
  fee record;
- non-matching requests preserve the existing relay and billing flow;
- existing upstream Grok CSAM fee behavior remains unchanged.

## API Base URL Notice

The separate API-key-page notice is already implemented and deployed. It shows
`https://api.yuaiapi.com/v1` for standard requests,
`https://vip.yuaiapi.com/v1` for high-concurrency or long-running requests, and
warns users not to use `global.yuaiapi.com` as an API Base URL. No additional
URL-notice code change is required in this feature.

## Out Of Scope

- Charging normal model usage for a request that never reaches an upstream.
- Per-word fee levels or escalating penalties.
- Exposing matched sensitive words to users or consume logs.
- Changing channel retry, priority, weight, or account-pool scheduling.
