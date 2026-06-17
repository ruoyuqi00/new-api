# GPT-Only NewAPI Bridge Layout - 2026-06-17

This note narrows the bridge plan to the current operating goal: expose only the GPT product line first, keep all other families reserved for later, and let NewAPI stay user-facing while Sub2API stays the internal supply/scheduler layer.

Do not paste real API keys, refresh tokens, upstream keys, or account JSON into this file.

## Current Decision

For now, only the GPT line should be connected to NewAPI.

Other families such as Kiro, Windsurf, Opus, Grok, Gemini, and image-only products are not abandoned. They should simply stay out of the current NewAPI bridge until they are intentionally created as separate product lines.

## Target Shape

```text
User
  -> NewAPI user key
  -> NewAPI user-visible group: gpt
  -> NewAPI admin-only channel: sub2api-gpt
  -> Sub2API internal bridge key: newapi-bridge-gpt
  -> Sub2API GPT supply group: GPT5.5 for now, later supply-gpt
  -> Sub2API account scheduler / upstream fallback
```

The user should only see the NewAPI-side product naming, model list, quota, and price. The Sub2API bridge key, bridge group, account pool, and upstream API fallback are internal supply-layer details.

## Two-Sided Change

Yes, the bridge should be configured on both sides:

- NewAPI owns the user-facing product layer:
  - user-visible group, for example `gpt`;
  - user key issuance, quota, package, and price display;
  - admin-only upstream channel, for example `sub2api-gpt`;
  - public model list shown to users.
- Sub2API owns the internal supply layer:
  - dedicated bridge key, for example `newapi-bridge-gpt`;
  - GPT supply group/account pool;
  - model mapping, scheduler behavior, fallback accounts, risk logs, and account health;
  - optional channel-level pricing/restriction used by the adapter layer.

Do not try to make the NewAPI group automatically become a Sub2API group. The mapping happens through the Sub2API bridge key that NewAPI uses for that upstream channel.

## Current Production Snapshot

Read-only snapshot taken on 2026-06-17:

- Existing Sub2API channels are currently named for Windsurf product lines:
  - `channel-windsurf-opus4.6`
  - `channel-windsurf-opus4.7`
  - `channel-windsurf-gpt5.5`
  - `channel-windsurf-gpt5.4`
  - `channel-windsurf-grok`
- Those channels are attached to `windsurf-*` Anthropic-platform groups.
- The current GPT account pool is stored in the historical `GPT5.5` group, group id `8`, platform `openai`.
- The `GPT5.5` group name should be treated as a current pool name, not as a promise that the pool can only serve `gpt-5.5`.
- `GPT5.5` currently has active OpenAI supply capacity and several historical/deleted accounts.
- `GPT5.5` currently has multiple active API keys, but their names look historical/manual rather than dedicated NewAPI bridge keys.
- `GPT5.5` is not currently attached to any Sub2API channel in `channel_groups`.
- `kiro-gpt5.5` exists but is disabled and should not be used for the current GPT bridge.

Operational interpretation:

- Treat `GPT5.5` as the current GPT supply group, even though the eventual cleaner name should be `supply-gpt`.
- Do not mix NewAPI GPT traffic into the existing `windsurf-*` channels.
- Do not reuse historical/manual keys as the long-term NewAPI bridge key.
- Create one dedicated Sub2API key for NewAPI, bound to `GPT5.5`, and name it clearly.

## GPT Model Scope

The GPT supply group should support multiple GPT-family models, not only `gpt-5.5`.

Recommended rule:

```text
NewAPI gpt product line
  -> one Sub2API GPT bridge key
  -> one Sub2API GPT supply group
  -> many GPT-family model names
```

Examples of models that may belong to the same GPT product line if the account pool and mapping support them:

- `gpt-5.5`
- `gpt-5.5-pro`
- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.4-pro`
- `gpt-5.3-codex`
- future GPT models that are intentionally tested and enabled.

Model availability is controlled by three things:

1. NewAPI channel `models`: what users are allowed to request.
2. Sub2API channel pricing/mapping/restriction: how requested model names are mapped and billed.
3. Sub2API account pool capabilities: whether selected accounts can actually serve the mapped model.

If a model is visible in NewAPI but no account in the Sub2API GPT pool can serve it, users will still see runtime failures. Add each GPT model only after one live call has been tested through the bridge.

## Recommended Names

Use stable, boring names so future expansion stays readable:

| Layer | Name | Visibility | Purpose |
| --- | --- | --- | --- |
| NewAPI group | `gpt` | User-visible | The product group users select/use. |
| NewAPI channel | `sub2api-gpt` | Admin-only | Routes NewAPI GPT traffic to Sub2API. |
| Sub2API API key | `newapi-bridge-gpt` | Internal only | Bridge credential used only by NewAPI. |
| Sub2API group | `GPT5.5` now, later `supply-gpt` | Internal/supply | Current GPT account pool. It can serve multiple GPT models if accounts/mapping support them. |
| Sub2API channel | `channel-newapi-gpt` | Internal/admin | Optional but recommended for channel-level pricing/mapping/restriction. |

Avoid renaming `GPT5.5` during the first cleanup pass. A rename is cosmetic, while breaking existing keys or scripts would be expensive.

## Setup Steps

1. In Sub2API, keep `GPT5.5` as the current GPT supply group, but treat it as the multi-model GPT pool.
2. In Sub2API, create a new API key named `newapi-bridge-gpt` and bind it to `GPT5.5`.
3. In Sub2API, create `channel-newapi-gpt` only if channel-level pricing, visible supported models, model restrictions, or model mapping are needed now.
   - Attach only `GPT5.5` to this channel.
   - Add every GPT model that NewAPI will expose, not just `gpt-5.5`.
   - Keep `restrict_models` enabled only if the intended GPT model list is complete and tested.
   - Keep model mapping stable so NewAPI can expose your chosen model names.
4. In NewAPI, create one channel named `sub2api-gpt`.
   - `base_url`: Sub2API OpenAI-compatible base URL, preferably internal network URL if NewAPI and Sub2API share a Docker network; otherwise use the public HTTPS Sub2API endpoint.
   - `key`: the `newapi-bridge-gpt` Sub2API key.
   - `models`: all GPT names users should see/use now, and only those already tested through the bridge.
   - `group`: only the NewAPI `gpt` group.
5. In NewAPI, give users NewAPI keys assigned to the `gpt` group.

## What Not To Do Yet

- Do not expose Sub2API keys directly to users.
- Do not bind NewAPI `gpt` traffic to `windsurf-*` groups.
- Do not mix future Opus/Grok/Gemini traffic into the GPT NewAPI channel.
- Do not delete existing non-GPT channels during this cleanup; leave them parked until each family is intentionally rebuilt.
- Do not restart NewAPI just to document or inspect this mapping.

## Future Expansion Pattern

When adding another product family later, copy the same pattern:

```text
NewAPI user-visible group
  -> NewAPI admin-only channel
  -> dedicated Sub2API bridge key
  -> dedicated Sub2API supply group
  -> dedicated Sub2API channel/mapping/pricing
```

Examples:

- `opus` -> `sub2api-opus` -> `newapi-bridge-opus` -> `supply-opus`
- `grok` -> `sub2api-grok` -> `newapi-bridge-grok` -> `supply-grok`
- `gemini` -> `sub2api-gemini` -> `newapi-bridge-gemini` -> `supply-gemini`
- `image` -> `sub2api-image` -> `newapi-bridge-image` -> `supply-image`

Each product family gets its own bridge key so NewAPI group-level routing maps cleanly back to the intended Sub2API supply pool.

## Quick Verification

For the GPT-only bridge, verify:

- NewAPI user key belongs to NewAPI group `gpt`.
- NewAPI channel `sub2api-gpt` is enabled for group `gpt`.
- NewAPI channel key is the dedicated Sub2API key `newapi-bridge-gpt`.
- Sub2API key `newapi-bridge-gpt` is bound to group `GPT5.5`.
- `GPT5.5` has active OpenAI accounts that can serve every GPT model exposed by NewAPI.
- A request from a NewAPI user key creates usage on the NewAPI side and reaches Sub2API with the bridge key.
- Sub2API usage/risk logs show the bridge key and group `GPT5.5`.
- Each exposed GPT model has at least one successful bridge smoke test.

If any of these fail, do not add more product families yet. Fix the GPT bridge first.
