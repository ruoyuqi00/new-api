# Infinite Canvas Media Model Integration Design

## Context

The Infinite Canvas and YuCore Studio currently use a separate media catalog and task runner. The catalog is a hard-coded list, while production routing is configured through enabled channel abilities and user-usable groups. This causes the canvas to show stale or fictional model capabilities, omit most production image and video models, and hide model selection from the canvas agent panel.

The task runner already supports routing back through YuAPI with a managed token. That path uses the normal relay for channel selection and billing, but it always uses one server-configured group and does not validate the selected model against the logged-in user's current access.

## Goals

- Show only image and video models that the logged-in user can actually use now.
- Let users select a billing group and model in both Studio generation views and the Infinite Canvas agent panel.
- Route each task through the selected, authorized YuAPI group.
- Use the existing user account balance, standard model pricing, group ratio, channel scheduling, usage logs, and task polling.
- Stop presenting unsupported dimensions, styles, modes, or provider descriptions as if they were real capabilities.
- Preserve existing canvases, tasks, generated assets, and asynchronous result backflow.

## Non-Goals

- Text models are not added as canvas agent reasoning models.
- Channel configuration, model pricing, and group ratios are not changed by this feature.
- The browser will not receive or store a YuAPI API key.
- The existing media task system will not be replaced with a new job queue.

## Architecture

### Dynamic Media Catalog

Add `GET /api/yucore/media/catalog`, an authenticated media catalog response built from current server state:

1. Resolve the logged-in user's usable groups with the same rules as the normal Playground.
2. Expand the `auto` group using the configured auto-group order.
3. Read enabled models for each usable group from channel abilities.
4. Keep only models whose effective endpoint types include image generation or OpenAI-compatible video.
5. Attach current pricing for the user/group combination and explicit media capability metadata when configured.
6. Return groups in stable order and models in stable, deduplicated order.

The server-configured managed media group remains the preferred initial selection when the user can use it. Otherwise, the first usable group containing a media model is selected.

Hard-coded model entries will no longer determine availability. Known explicit capability configuration may enrich a model, but it cannot make an unavailable model appear.

### Capability Presentation

Every catalog item contains its exact model ID, media kind, endpoint-backed modes, and current price. Optional controls are shown only when the server reports support for them.

For models without explicit capability metadata:

- Image models expose text-to-image and only conservative common options.
- Video models expose their asynchronous task transport and only conservative common options.
- Unknown sizes, durations, styles, counts, or edit support are omitted from the UI instead of being guessed.
- The model ID is used as the display name when no model metadata exists.

This keeps newly added production models usable without inventing provider-specific configuration.

### Task Group and Validation

Persist the selected group on each `YucoreMediaTask` in a `billing_group` database column. Task creation, task responses, and canvas agent execution use a `group` JSON field.

Before creating a task, the backend verifies that:

- the group is usable by the logged-in user;
- the model is enabled in that group (or in one of the authorized groups represented by `auto`);
- the model has an image or video endpoint matching the requested task kind;
- the requested mode and optional parameters are allowed by the reported capability.

Validation failures return a clear client error before a task row or agent run is created.

Old task rows remain valid. Requests from an older frontend that omit `group` use the configured managed media group when authorized, preserving compatibility during rollout.

### Billing and Routing

For the `yuapi-channel` adapter, the internal managed token is created per user and selected group. The token is never returned to the browser. The internal request continues through YuAPI's standard relay, so the existing implementation remains authoritative for:

- user balance checks and deductions;
- model price and group ratio;
- channel selection, affinity, cooldown, and retries;
- usage and error logs;
- asynchronous image and video task creation.

The media task `cost` field is an estimate for display. Actual balance changes come only from the standard relay, preventing duplicate deductions.

### Frontend Behavior

Studio loads the media catalog once and keeps a selected group plus separate selected image/video model IDs.

- The image and video workspaces show models from the selected group and matching media kind.
- The Infinite Canvas agent panel includes visible group and image-model selectors above the prompt.
- Changing group selects the first valid model of the active kind when the previous model is unavailable.
- Empty groups show a real empty state instead of “loading models” forever.
- Template application never silently switches to a model unavailable in the selected group.
- Submitted tasks include the selected group, and task/history nodes display the exact group and model used.
- Balance refresh uses the existing user wallet endpoint after a task is accepted and after it reaches a terminal state.

## Compatibility and Migration

- Add the task group column through the existing cross-database GORM migration path.
- Existing rows receive no destructive rewrite; an empty stored group means legacy configured-group behavior.
- Existing canvas snapshots need no schema migration.
- Existing API response fields remain; the new group field is additive.
- Keep `GET /api/yucore/media/models` as a compatibility wrapper that returns the preferred group's models from the same dynamic catalog builder.

## Error Handling

- Catalog failures leave generation disabled and display the server error.
- A model removed between catalog load and submission returns a refresh-required validation error.
- Insufficient balance is returned by the standard relay and marks the media task failed without a second charge.
- Async provider failures continue to update the task and canvas agent run through existing backflow.

## Testing

Backend tests will cover:

- catalog filtering by user-usable group, enabled ability, and media endpoint type;
- exclusion of text-only and disabled models;
- selected-group authorization and model availability validation;
- selected-group managed-token creation;
- legacy missing-group fallback;
- task persistence and response compatibility;
- no separate media-layer quota deduction.

Frontend tests will cover catalog normalization and deterministic group/model fallback. Type checking, linting, frontend production build, targeted Go tests, and the broader relevant Go test set will run before deployment.

## Deployment

Build a candidate image, start it beside production, verify health and authenticated catalog behavior without exposing credentials, then switch Caddy's YuAPI upstream to the candidate. After direct and public checks pass, replace the production container while retaining the previous image for rollback. No unrelated container or channel configuration is changed.
