# End-to-End Bridge And Capacity Map - 2026-06-17

This note summarizes the full operating chain in one place:

- the user-facing site;
- the Sub2API supply layer;
- the upstream bridge layer;
- and the controls that keep future account growth and concurrency manageable.

Do not record real API keys, refresh tokens, passwords, or full account payloads here.

## 1. Current Live Chain

```text
User
  -> NewAPI user key
  -> NewAPI user-visible group: gpt
  -> NewAPI admin-only channel: sub2api-gpt
  -> Sub2API internal bridge key: newapi-bridge-gpt
  -> Sub2API GPT supply group: GPT5.5
  -> Sub2API GPT channel / scheduler / account pool
  -> upstream account or external fallback channel
```

This is the important rule:

- NewAPI is the public product/control plane.
- Sub2API is the internal supply/scheduler plane.
- The bridge key is internal only.
- Users should only see the NewAPI-facing product group and model list.

## 2. Upstream Bridge Shape

There are two distinct bridge directions:

1. NewAPI -> Sub2API
   - used for the public GPT product line;
   - maps user traffic into the internal Sub2API supply group.

2. Sub2API -> upstream provider
   - used when Sub2API itself needs to fan out to Kiro, Windsurf, OpenAI-compatible upstreams, or other provider pools;
   - keeps account pools and provider quirks hidden from NewAPI users.

Do not collapse these into one shared channel. Keep the public bridge and the upstream bridge separate so failures stay easier to isolate.

## 3. Stability Controls

The shortest path to safer growth is:

- one customer, app, or reseller per API key;
- one intended group per channel;
- one family per supply pool;
- separate bridge keys per family;
- separate fallback channels per family.

Operational control points:

- per-key quotas;
- per-group RPM limits;
- Redis-backed or persisted cooldown state;
- scheduler snapshots and account health state;
- key-level auto-disable or quarantine for abusive downstream keys;
- hard-dead upstream credential deletion for terminal failures only;
- soft cooldown for quota, rate-limit, or overage text.

## 4. What Is Already In Place

- Kiro hard-dead credential deletion is live.
- The Sub2API and NewAPI containers are healthy on the server.
- Docker data root has already been moved off the tight root disk.
- GPT-only bridge wiring has already been documented and applied.
- An external GPT-compatible upstream fallback channel also exists for admin-only supply.

## 5. What Still Needs Load-Scale Hardening

- repeatable load tests for auth, scheduler, and hot gateway paths;
- p50/p95/p99 latency and error-rate tracking;
- stronger Redis-backed account-health and cooldown behavior;
- aggregate reporting for risky keys, groups, models, and endpoints;
- clearer quarantine state for risky downstream keys;
- more explicit per-family fallback policy for future growth.

## 6. Expansion Rule

When a new product family is added later, use the same pattern:

```text
NewAPI user group
  -> NewAPI channel
  -> dedicated Sub2API bridge key
  -> dedicated Sub2API supply group
  -> dedicated upstream fallback or account pool
```

That keeps GPT, Opus, Grok, Kiro, Gemini, and image traffic separable instead of turning into one mixed pool.

