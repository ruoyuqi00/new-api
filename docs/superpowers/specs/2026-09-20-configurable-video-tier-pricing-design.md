# Configurable Video Tier Pricing Design

## Goal

Allow administrators to set independent prices for each supported video resolution and reference-video pricing profile, use those values for task billing, and expose the current effective prices in the model marketplace. Public integration documents must describe models and billing units without embedding mutable prices.

## Scope

This change covers the 11 video models that currently use request-aware pricing:

- Seedance video-token models: `seedance-2-0-mini-official`, `seedance-2-0-fast-official`, `seedance-2-0-official`, `seedance-2-5-official`
- Per-second models: `minimax-h3`, `wan3.0-video`, `wan3.0-video-prime`, `seedance2.0-9-3-3-PT`, `seedance2.5-30-10-10-PT`, `seedance2.0-fast-PT`
- Per-successful-task model: `grok-v1.5-video`

The existing image-resolution pricing system remains unchanged. Existing group multipliers, task reservation, final settlement, task retry, and idempotency behavior also remain unchanged.

## Current Problem

The generic model editor stores one `ModelPrice` or `ModelRatio` value per model. The video task adaptor then applies code-defined resolution multipliers. An administrator can change the lowest/base tier, but cannot independently change another tier. The generic UI also labels fixed-price video models as per-request even when task billing later multiplies the base by seconds.

Public documentation currently repeats prices that may diverge from administrator settings. The model marketplace is the correct user-facing source of current prices.

## Configuration Model

Register a new global configuration namespace:

```text
video_pricing_setting.models
```

The stored value contains only administrator overrides. An empty map preserves all existing behavior.

```json
{
  "minimax-h3": {
    "480p": { "standard": 0.1 },
    "768p": { "standard": 0.16 },
    "1080p": { "standard": 0.18 },
    "2k": { "standard": 0.26 },
    "4k": { "standard": 0.36 }
  },
  "seedance-2-0-official": {
    "480p": {
      "standard": 36.8,
      "with_reference_video": 22.4
    },
    "720p": {
      "standard": 36.8,
      "with_reference_video": 22.4
    },
    "1080p": {
      "standard": 40.8,
      "with_reference_video": 24.8
    },
    "4k": {
      "standard": 20.8,
      "with_reference_video": 12.8
    }
  }
}
```

The configuration does not store billing units, supported resolutions, or model capabilities. Those are server-owned definitions, so an administrator cannot accidentally turn a per-second model into a per-task model or create a tier that request validation cannot select.

Each configured price must be finite and greater than zero. The validator rejects unknown models, unsupported resolution keys, missing `standard` prices, unexpected reference-video prices, and incomplete overrides for a model. Saving one model writes all of that model's supported tiers, preventing a partially configured model from mixing explicit and proportional prices.

## Compatibility And Fallback

No deployment-time migration writes tier prices. If a model has no override, billing keeps the current base-price and multiplier calculation exactly:

- Per-second and per-task models continue from the configured `ModelPrice`.
- Seedance video-token models continue from the configured `ModelRatio` and the existing resolution/reference profile ratios.
- Existing custom base prices therefore remain effective immediately after deployment.

When an administrator first opens a model in the video tier editor, the server returns effective prices calculated from the current base configuration. Saving converts that model to explicit tier prices. Removing the model override returns it to proportional fallback behavior.

This creates an explicit boundary: a model is either fully overridden or fully inherited. It never combines arbitrary explicit tiers with hidden fallback tiers.

## Backend Resolution And Billing

Add a concurrency-safe video pricing index in `setting/operation_setting`, following the existing image-resolution pricing pattern. It provides:

- configuration validation and JSON serialization;
- model and resolution normalization;
- effective pricing metadata for administration and the marketplace;
- exact quote resolution for `(model, resolution, hasReferenceVideo)`;
- fallback metadata calculated from the current `ModelPrice` or `ModelRatio` when no override exists.

The task adaptor continues to own duration and reference-duration semantics:

- H3 and PT models multiply the selected per-second tier by output seconds.
- Wan models multiply by output seconds plus reference-video seconds.
- Grok charges the selected per-task tier once for a successful task.
- Seedance official models settle from authoritative video tokens using the selected resolution/reference tier.

For per-second and per-task models, the adaptor converts an explicit absolute tier price into `selectedPrice / PriceData.ModelPrice`, then uses the existing `OtherRatios` task path. For Seedance token models, it converts the selected USD-per-million rate into a ratio against the request's frozen base input price (`PriceData.ModelRatio * 2`). This preserves the current reservation and settlement snapshots while making the selected tier independently configurable.

Requests with unsupported durations, resolutions, or required fields continue to fail before submission. A configured model with an invalid or missing quote must fail closed with `400 unsupported_pricing_tier`; it must not silently use another tier.

Updating `video_pricing_setting.models` rebuilds the in-memory index and invalidates pricing caches. The option uses the existing `options` storage, so SQLite, MySQL, and PostgreSQL require no schema migration.

## Administration UI

Add a `Video tier prices` tab to the existing Model Pricing settings area. Keep it separate from the generic token/per-request editor because its dimensions and units are different.

The tab contains a searchable model list. Each row shows:

- model ID;
- billing unit (`per second`, `per successful task`, or `per 1M video tokens`);
- inherited or explicit status;
- supported resolution tiers;
- an edit action.

The edit drawer renders one row per supported resolution. Standard models have one price input. Seedance video-token models have `Without reference video` and `With reference video` inputs. The drawer shows the unit beside every input, validates all required tiers, and provides:

- `Save tier prices`, which stores a complete override for the model;
- `Use inherited prices`, which removes that model's override after confirmation;
- a read-only preview of the resulting tiers.

The raw JSON settings mode also exposes `video_pricing_setting.models` for recovery and bulk editing, using the same server validation.

All new UI text uses the six default frontend locales. The layout must work in light and dark themes and remain usable on mobile without page-level horizontal overflow.

## Model Marketplace

Extend each pricing model with optional `video_tier_pricing` metadata containing:

- billing unit;
- whether the values are inherited or explicitly configured;
- supported resolution tiers;
- standard prices;
- optional reference-video prices.

Cards display the lowest applicable value as `From`, followed by the correct unit. Model details replace the misleading generic per-request breakdown with a tier table. The table applies the same group multipliers already used elsewhere, so values shown for each enabled group match task billing. Seedance reference and non-reference prices appear as separate columns.

The marketplace remains the only public source of numeric model prices.

## Documentation

Update simplified Chinese, traditional Chinese, and English documents together:

- Remove all numeric video and image prices, including legacy model tables, new video tables, Grok Imagine tables, and image price examples.
- Rename price-oriented sections to model and billing-method sections.
- Keep model IDs, billing units, supported resolutions, durations, request fields, runnable examples, idempotency guidance, and polling guidance.
- Link users to `/pricing` for current prices.

The documentation checker must validate model membership, billing units, supported tier labels, required paths, statuses, JSON examples, language parity, and absence of price columns in marked public catalogs. It must no longer contain expected numeric prices.

## Testing

Backend tests must cover:

- empty configuration preserving legacy calculations;
- independent overrides for every billing family;
- reference-video and non-reference Seedance prices;
- Wan reference-duration billing;
- Grok charging once regardless of duration;
- validation failures for unknown models, tiers, incomplete models, zero, negative, NaN, and infinity;
- option update rebuilding indexes and invalidating marketplace pricing;
- pricing API metadata matching actual task quotes;
- concurrent reads during configuration replacement.

Frontend tests must cover:

- parsing inherited and explicit models;
- editing and saving a complete model override;
- returning a model to inherited pricing;
- validation for missing or invalid tier values;
- marketplace unit labels and tier tables;
- all locale keys.

Documentation checks, desktop/mobile documentation E2E, frontend typecheck, targeted lint, production build, and full Go tests are required before local review.

## Rollout

1. Deploy with the override map empty so billing behavior is unchanged.
2. Verify the marketplace effective tier table matches current production billing.
3. Configure one low-risk video model through the new admin tab and make test requests for at least two resolutions.
4. Compare task logs, charged quota, and marketplace values.
5. Configure the remaining models only after the first model matches exactly.

Production replacement remains gated on local UI approval. No unrelated project or container may be restarted.
