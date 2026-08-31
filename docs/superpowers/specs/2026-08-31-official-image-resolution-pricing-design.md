# Official Image Resolution Pricing Design

Date: 2026-08-31 (Asia/Shanghai)

## Objective

Introduce one configurable resolution-price policy per official image model so
that image pricing no longer depends on separate user-visible `-1k`, `-2k`, and
`-4k` model prices. This phase changes pricing only. It does not remove aliases,
change channel mappings, alter upstream request payloads, or deploy production.

## Scope

This phase covers:

- canonical 1K, 2K, and 4K resolution classification;
- official-model resolution price storage and validation;
- compatibility pricing for existing resolution aliases;
- frozen pre-consume and final-settlement pricing inputs;
- pricing API and audit-log metadata;
- regression coverage for existing billing systems.

Channel capability selection, official-name-only model publication, upstream
model mapping, and provider-specific dimension conversion are deferred to the
next phase.

## Resolution classification

The billing tier is determined by the smallest square boundary that contains
the requested dimensions:

| Request | Billing tier |
| --- | --- |
| width <= 1024 and height <= 1024 | 1K |
| width <= 2048 and height <= 2048 | 2K |
| width <= 4096 and height <= 4096 | 4K |
| either edge > 4096 | rejected |

Examples:

- `650x1024` -> 1K
- `1024x1536` -> 2K
- `2048x3072` -> 4K
- `512x4096` -> 4K
- `4097x512` -> rejected

The separators `x`, `X`, `*`, and `×` normalize to the same exact-size form.
Explicit `1k`, `2k`, and `4k` values select that tier directly. Empty or
`auto` uses the model policy's configured default tier; the initial policies
use 1K. Invalid dimensions never fall back to the cheapest tier.

`aspect_ratio` changes shape but not price when the request already supplies a
resolution tier. For example, `size=2k&aspect_ratio=3:2` remains a 2K request.

## Price policy

A new option-backed map stores prices by canonical official model name. It is
validated and replaced atomically through the existing option update path, so
it requires no database schema migration and remains compatible with SQLite,
MySQL, and PostgreSQL.

Initial policy:

| Official pricing model | 1K | 2K | 4K | Default |
| --- | ---: | ---: | ---: | --- |
| `gpt-image-2` | 0.01 | 0.04 | 0.045 | 1K |
| `nano-banana-pro` | 0.086666666667 | 0.108333333333 | 0.161416666667 | 1K |
| `nano-banana2` | 0.063916666667 | 0.086666666667 | 0.13 | 1K |

These are base per-image prices. The existing group ratio is applied exactly
once after the base price is selected. The existing `n` image count multiplier
is also applied exactly once.

Each policy must provide positive finite prices for every enabled tier, a valid
default tier, and monotonically non-decreasing prices. An incomplete or invalid
policy is rejected as a whole. Models without a resolution policy retain their
current pricing behavior.

## Legacy alias compatibility

The following aliases continue to work during the price phase:

- `gpt-image-2-1k`, `gpt-image-2-2k`, `gpt-image-2-4k`
- `nano-banana-pro-1k`, `nano-banana-pro-2k`, `nano-banana-pro-4k`
- `nano-banana2-1k`, `nano-banana2-2k`, `nano-banana2-4k`

Alias normalization is used only to resolve the pricing policy. It does not
change the requested model sent to channel selection or the upstream in this
phase.

The alias suffix is a minimum billing tier. The final tier is the higher of the
alias tier and the request-size tier:

- `gpt-image-2-4k` with `1024x1024` remains billed as 4K;
- `gpt-image-2-1k` with `1536x1024` is billed as 2K;
- an official `gpt-image-2` request is billed entirely from its requested size.

This rule preserves legacy expectations while preventing a low-tier alias from
undercharging a larger explicit request.

## Billing data flow

Before quota pre-consume, one resolver produces an immutable pricing result:

- user-requested model;
- canonical pricing model;
- normalized requested size;
- resolved resolution tier;
- base unit price;
- requested image count;
- group ratio.

The result is frozen in the existing request pricing snapshot. Pre-consume and
final settlement both use that same snapshot. Settlement must not re-read a
later administrator price change or infer a different tier from the upstream
response. This prevents mid-request price drift and ensures the group ratio and
image count are not multiplied twice.

The image-resolution policy is independent from token billing expressions,
task billing, channel affinity, violation fees, actual-response-model logging,
and retry behavior. Those systems continue using their existing paths.

## API and logs

`/api/pricing` adds optional resolution-pricing metadata for configured models:

- canonical pricing model;
- default tier;
- 1K, 2K, and 4K base prices.

Existing fields remain backward compatible. During the compatibility phase,
legacy aliases may still appear but must expose the canonical policy and their
minimum tier rather than independent conflicting prices.

Image usage logs retain the user-requested model and add non-sensitive billing
metadata:

- normalized requested size;
- resolution tier;
- base unit price;
- image count.

No upstream URL, channel credential, account-pool identity, or mapped upstream
model is exposed to ordinary users.

## Error handling

Reject before upstream submission when:

- exact dimensions are malformed or non-positive;
- either edge exceeds 4096;
- a configured official model is missing the requested tier price;
- the policy is invalid or cannot be loaded safely.

The public error names the invalid size or unavailable price tier without
revealing upstream or channel details. No quota is consumed when validation
fails before submission.

## Verification

Backend tests cover:

- every exact boundary and just-over-boundary value;
- all accepted dimension separators;
- explicit tier and tier-plus-aspect-ratio requests;
- invalid, negative, zero, and over-4K dimensions;
- legacy alias minimum-tier behavior;
- official model price selection;
- `n` and group ratio applied once;
- pre-consume and settlement using the frozen policy snapshot;
- atomic policy validation and rollback;
- `/api/pricing` compatibility and metadata;
- models without policies retaining existing pricing;
- billing expression, task billing, affinity, violation fee, and actual response
  model regression suites.

No real paid image request is required for this pricing-only phase.

## Rollout and rollback

Implementation stays on the production-derived isolated worktree. A local
candidate uses an independent database and loopback-only port. The complete
production frontend asset graph must be reproduced and verified before any
future production candidate is considered.

Production price options are not changed until the user approves the local UI,
pricing output, and simulated quota matrix. Before a production option update,
capture the current `/api/pricing` response and option value, apply the complete
map atomically, capture the new response, and retain the old value for rollback.

Rollback restores the previous option map. It does not restore a database
snapshot, alter balances, or delete logs.
