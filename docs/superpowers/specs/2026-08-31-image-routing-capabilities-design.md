# Official Image Names, Resolution Capabilities, and Routing

Date: 2026-08-31 (Asia/Shanghai)

## Objective

Make image routing use the official public model name while selecting only a
channel that has been verified for the requested resolution and aspect ratio.
The selected channel must receive the provider-specific upstream model name
and the requested image shape without exposing channel or account-pool data.

This is a routing and request-conversion phase. The resolution pricing policy
from `2026-08-31-official-image-resolution-pricing-design.md` remains the sole
source of image prices.

## Scope and invariants

- The public canonical name is the official model name, such as
  `gpt-image-2`, `nano-banana-pro`, or `nano-banana2`.
- Existing resolution aliases remain accepted for backward compatibility, but
  they are normalized to the canonical name for capability and price lookup.
- A channel may map the canonical name to a provider-specific model through the
  existing model-mapping mechanism. The mapping is applied only after channel
  selection and before the upstream request.
- The request's exact width, height, and aspect ratio are preserved whenever
  the selected provider supports them. A provider that supports only a named
  aspect ratio receives the aspect ratio and resolution tier in its native
  fields.
- A channel with unknown image capability is eligible for square requests only.
  It is ineligible for non-square or explicit custom-dimension requests until
  an administrator marks it as verified.
- No lower-resolution fallback is allowed for a request that requires a higher
  tier. If no compatible channel exists, fail before upstream submission with a
  generic public error.
- Channel affinity, pool leases, retry/cooldown behavior, billing snapshots,
  violation fees, task billing, and actual-response-model recording remain on
  their existing paths.
- No upstream URL, credential, account-pool identity, or provider error body is
  returned to the customer.

## Capability representation

Extend the existing channel `settings` JSON without adding a database column.
The current scalar `image_dimension_support` remains accepted:

- `square`: square only;
- `any` or `ratio`: verified non-square support up to the provider's declared
  maximum;
- `pending` or `unknown`: square-only until verified;
- empty: treated as unknown and therefore square-only for non-square requests.

For channels whose limits differ by model, add an optional settings object:

```json
{
  "image_dimension_support": "any",
  "image_model_capabilities": {
    "gpt-image-2": {"max_tier": "4k", "shape": "exact"},
    "imagen-4.0-generate-001": {"max_tier": "2k", "shape": "ratio"}
  }
}
```

`shape` is `exact` for providers that accept arbitrary `width x height`, and
`ratio` for providers that accept a bounded set of aspect-ratio values. A
missing model entry inherits the channel-level capability. Invalid capability
settings fail validation when the channel is saved and never make a channel
eligible.

The existing `model_mapping` remains the only source of upstream model names.
No credential or account-pool field is added to this capability data.

## Canonical model and channel selection

1. Parse the customer request and resolve the canonical image model and
   requested resolution tier using the shared pricing resolver.
2. Build lookup candidates for the canonical model and its legacy tier aliases.
   A channel is a candidate when its ability is enabled for one of those keys,
   its request path is supported, its pool is available, and its capability
   covers the required tier and shape.
3. Keep the existing priority, weight, affinity, and retry ordering. Apply the
   capability filter inside both the memory-cache and database selection paths
   before priority selection.
4. If an affinity channel is incompatible with the request, do not clear a
   valid affinity record solely because of that request; select another
   compatible channel and preserve the affinity state for future compatible
   requests.
5. Store the canonical customer model separately from the selected upstream
   model in the existing `RelayInfo` fields. `OriginModelName` remains the
   customer-visible name; `UpstreamModelName` is the mapped provider name.
6. On retry, reuse the same canonical model, normalized size, tier, and
   capability requirements. A failed channel is skipped according to the
   existing retry rules, and no retry may relax the shape or tier constraint.

## Provider request conversion

The shared image request remains the source of truth. Provider adapters then
apply only provider-native conversion:

- OpenAI-compatible adapters send normalized exact `size` when the capability
  is `exact`; an aspect-ratio-only request is converted to the largest exact
  dimensions inside its selected tier and the original ratio is retained when
  supported.
- Imagen/Gemini-style adapters send the selected `imageSize` tier and
  `aspectRatio`. They reject tiers or ratios outside the model capability.
- Other adapters must declare their accepted shape in capability settings. An
  adapter with no verified declaration receives only square requests.

Parameter overrides may supply defaults, but they may not replace an explicit
customer width, height, or ratio. Conditional overrides are evaluated against
the canonical customer model and the mapped upstream model using the existing
override context.

## API, logs, and errors

- `/api/pricing` and the user model list expose the canonical official image
  names. Legacy aliases remain readable during migration and point to the same
  canonical price policy.
- Request logs retain the customer-requested model and the existing actual
  response model. Internal audit metadata may include normalized size, tier,
  capability decision, and selected channel ID, but never credentials or full
  upstream URLs.
- A missing or incompatible channel returns a stable generic error such as
  `no compatible image channel for requested size`; it must not include provider
  response text or channel details.

## Validation and rollback

- Validate capability JSON atomically with the existing channel settings update.
- Reject malformed tiers, unsupported shapes, non-positive dimensions, and
  model capability entries that exceed the provider's declared maximum.
- Keep the old settings value and a before/after `/api/pricing` snapshot for
  every production configuration change. Restoring the old settings value is
  the rollback; no database snapshot restore is needed.
- The implementation is first run in an isolated local candidate with the
  production-derived UI and an independent database. Production traffic and
  Caddy remain unchanged until an explicit approval.

## Verification

Backend tests cover:

- canonical and legacy model lookup;
- exact, ratio-only, square, unknown, and over-limit capability decisions;
- memory-cache and database channel selection;
- priority, affinity, pool lease, and retry preservation;
- upstream mapping after selection and customer-model retention in logs;
- provider payloads for exact dimensions and native tier/ratio fields;
- no compatible-channel errors without upstream leakage;
- no price or billing-expression changes.

Local Playwright/API verification covers the existing branded pages and one
non-square request per verified provider adapter using a mocked upstream. No
real paid image request is required for this phase.
