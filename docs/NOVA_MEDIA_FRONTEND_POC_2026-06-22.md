# Nova Media Frontend POC - 2026-06-22

This document records the decision to use `tianjiangqiji/nova-image-studio` as the preferred next media frontend base.

Do not write real API keys, passwords, refresh tokens, account payloads, or upstream credentials in this document.

## Decision

Use Nova Image Studio as the new image-site UI base, then add video generation into the same task/workspace pattern later.

Reason:

- The current UAG user web is useful for internal validation, but it is too thin as a public media product.
- Nova already has a polished Chinese workspace, image history, assets, prompt gallery, Agent mode, canvas mode, GIF mode, WebSocket task status, SQLite task state, and local image storage.
- Open-Generative-AI has wider media coverage, but the default product is tightly shaped around MuAPI key entry and needs heavier auth/API rewiring before it can become our frontend.
- Nova is image-first today, so video should be added deliberately as another task family instead of trying to fake video as a chat/image request.

## Local POC

Local path:

```text
D:\wflogin\media-site-poc\nova-image-studio
```

Local preview:

```text
http://127.0.0.1:3311
```

The first POC patch is stored here:

```text
patches/nova/server-managed-gpt-image2-poc-20260622.patch
```

The POC intentionally does not include real keys.

## POC Change

The patch adds a server-managed image model mode:

- The frontend has a default `gpt-image-2` model on first load.
- The browser stores only placeholder values:
  - `apiKey = __nova_server_managed__`
  - `baseUrl = server-managed`
- Task creation sends `registryModelId` to the backend.
- The backend resolves the real upstream model, base URL, and key from environment variables.
- `/api/nova/config` can expose public model metadata without exposing real credentials.

Server environment variables:

```text
NOVA_MEDIA_OPENAI_BASE_URL=https://newapi-or-uag-compatible.example/v1
NOVA_MEDIA_OPENAI_API_KEY=sk-...
NOVA_MEDIA_GPT_IMAGE_MODEL=gpt-image-2
```

Fallback variables supported by the POC:

```text
NOVA_API_BASE_URL
NOVA_API_KEY
```

## Intended Production Shape

Recommended product loop:

```text
User
  -> Nova media frontend
  -> Nova backend task queue
  -> server-managed upstream route
  -> NewAPI or UAG/OpenAI-compatible media gateway
  -> final upstream image/video provider
```

For public user billing, keep NewAPI/UAG as the authoritative wallet layer until a single media-wallet decision is made. Nova should not create an independent customer ledger unless we intentionally productize it that way.

## Why Server-Managed Models Matter

The upstream key and base URL must not live in browser localStorage for a public site.

Server-managed mode lets us:

- Hide upstream keys from users.
- Keep the visible model list controlled by our backend.
- Swap NewAPI/UAG/Sub2API bridge targets without changing the browser state.
- Add account-pool and provider routing later without exposing internal supply details.
- Keep the frontend usable for normal users without asking them to paste API keys.

## Image Next Steps

Before any production cutover:

1. Move the Nova POC into a maintained repo or branch.
2. Decide whether the upstream target is NewAPI image channel, UAG OpenAI endpoint, or a dedicated media gateway.
3. Add server-managed model loading from `/api/nova/config` instead of only the hardcoded first-load default.
4. Remove or hide self-service API key settings for public users.
5. Add real user auth/session wiring or put Nova behind the existing product login.
6. Smoke test:
   - text-to-image
   - image-to-image
   - task polling
   - image storage/download
   - upstream billing trace
7. Confirm no real key appears in browser localStorage, page source, logs, screenshots, or committed files.

## Video Plan

Video should be added as a new task family, not patched into image generation.

Suggested Nova additions:

- Add `videoModels` registry alongside `imageModels`.
- Add `video-to-video`, `text-to-video`, and `image-to-video` task modes.
- Add backend tables or fields for video task metadata:
  - duration
  - aspect ratio
  - resolution
  - seed/reference assets
  - upstream task id
  - poll URL or provider route
  - final media URL/path
- Add a `/api/nova/video/tasks` or generalized `/api/nova/tasks` mode branch.
- Add provider adapters for OpenAI-compatible async video endpoints first, then provider-specific routes such as Veo/Kling/Runway/Sora when tested.

Do not rely on streaming chat semantics for video. Treat video as async submit -> poll -> download/store.

## Relationship To Existing Stack

Nova is a candidate frontend/product surface. It does not replace these responsibilities by itself:

| System | Keep responsible for |
| --- | --- |
| NewAPI | public API users, keys, wallet, text/image channel management if used as bridge |
| Sub2API | internal account/upstream supply and group scheduling |
| UAG | existing media task/account-pool capability while Nova is still POC |
| Nova | future polished media workspace UI and task front door |

If Nova becomes the production media site, either:

- Nova uses NewAPI/UAG as a server-managed upstream and user/wallet stays outside Nova, or
- Nova is extended with its own login/wallet/admin system intentionally.

Do not accidentally run two independent ledgers for the same user request.

## Verification Done

Local verification on 2026-06-22:

```text
cd D:\wflogin\media-site-poc\nova-image-studio\frontend
npm run test:run
npm run build

cd D:\wflogin\media-site-poc\nova-image-studio\backend
node --check server.js
```

Result:

- Frontend tests passed: 34 tests.
- Frontend production build passed.
- Backend syntax check passed.

Known test noise:

- Existing React `act(...)` warnings appear in `TextToImageForm` tests, but tests pass.

## Not Done Yet

- No production deployment.
- No real upstream key committed.
- No real image generation smoke through Nova server-managed mode yet.
- No video UI or video provider adapter yet.
- No NewAPI/UAG user/wallet SSO integration yet.
- No final license/compliance decision for Nova AGPL deployment yet.

