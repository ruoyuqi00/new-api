# Sub2API Upstream Scan - 2026-05-21

This scan checks official `Wei-Shaw/sub2api` updates without merging them into
the private fork. Do not put deployment secrets or provider tokens in this file.

## Current State

- Official upstream remote: `https://github.com/Wei-Shaw/sub2api.git`
- Previous private-fork merge base: `3d22dd34d3de9076804858f979f60fccdf9f2de1`
- Latest upstream `main`: `35901a174b281367ca4c9655dfc14e2c2347c7ae`
- Latest upstream tag observed: `v0.1.129`
- Upstream `backend/cmd/server/VERSION`: `0.1.129`
- Private fork `backend/cmd/server/VERSION`: `0.1.127`
- Ahead/behind from private `HEAD` to upstream `main`: private-only `40`,
  upstream-only `64`.

No merge or deployment was performed during this scan.

## High-Value Upstream Changes

These changes are worth pulling into the private fork after a focused merge:

1. OpenAI / Responses compatibility:
   - `/v1/responses` now respects `force_chat_completions`.
   - A new Chat Completions bridge exists under
     `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`.
   - Responses reasoning output is handled in channel monitor.
   - Empty thinking-block upstream errors now trigger retry.

2. OpenAI image handling:
   - Image `n` parameter pass-through fixes.
   - Upstream image moderation errors are surfaced more clearly.

3. Codex / OpenAI OAuth resilience:
   - Codex OAuth browser user-agent rewrite to avoid Cloudflare challenge.
   - Reused refresh tokens are marked non-retryable.

4. Account and scheduler correctness:
   - Clear scheduler cache when deleting accounts.
   - Unschedule errored accounts.
   - Group available-account counts were corrected.
   - Disabled/deleted groups now block API key access.

5. API key ACL / real client IP:
   - API Key ACL can optionally use trusted forwarded IP headers.
   - New migration:
     `backend/migrations/140_extend_user_provider_default_grants_check.sql`.

6. Bedrock compatibility:
   - Bedrock accounts gained Claude Code compatibility transformations,
     including thinking conversion and tool-use ID sanitization.

7. Usage and redeem-code admin features:
   - User API key usage page supports daily detail.
   - Redeem codes support batch update.

8. Email / notification improvements:
   - Email template editor.
   - Payment success notification emails.
   - Balance and subscription expiry reminder emails.
   - Subscription expiry email toggle.
   - Email locale is preserved and passed through verification flows.

9. Risk control:
   - Content audit added keyword blocking.

10. OIDC login flow:
    - If the upstream email is already verified, login/registration can skip
      the choice page and continue directly.

## Potential Merge Conflicts With Our Fork

Only a small set of actual file overlaps were detected between private changes
and upstream changes:

- `backend/internal/server/routes/admin.go`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/types/index.ts`

There is also a migration-number collision risk:

- Our private fork added `backend/migrations/140_windsurf_opus47_aliases.sql`.
- Official upstream added
  `backend/migrations/140_extend_user_provider_default_grants_check.sql`.
- Official upstream also added
  `backend/migrations/141_subscription_expiry_notify_enabled.sql`.

When merging, renumber the private Windsurf alias migration to the next free
number, or verify the migration runner can tolerate non-unique numeric prefixes.
Do not deploy with ambiguous migration ordering.

## Provider Adapter Impact

Official upstream does not contain our private files, so a raw
`git diff HEAD..upstream/main` shows deletions for:

- `adapters/kiro-web/*`
- Windsurf/Kiro admin import handlers
- provider adapter frontend panels
- provider planning docs and probe scripts

Those deletions are not desired. They only mean upstream does not know about our
private provider-adapter work.

Merge rule:

- Keep all private Kiro/Windsurf adapter files.
- Merge official backend/frontend fixes around them.
- Pay close attention to `routes/admin.go`, i18n files, and shared frontend
  types, because both sides changed those.

## Recommended Next Merge Plan

1. Commit or stash the current scan/share docs first.
2. Create a branch, for example `merge-upstream-20260521`.
3. Merge `upstream/main`.
4. Resolve the four overlapping files.
5. Renumber the private Windsurf migration if needed.
6. Run backend tests around:
   - admin routes;
   - gateway OpenAI / Responses;
   - API key auth;
   - settings;
   - provider adapter import handlers.
7. Run frontend build/tests.
8. Build a new immutable Docker image.
9. Deploy to server only after local tests pass.
10. Smoke public routes:
    - normal OpenAI chat completions;
    - `claude-sonnet-4.6` through Kiro Web;
    - Windsurf `/v1/messages`;
    - Sub2API admin login and provider adapter panels.

## Commands Used

```powershell
git fetch upstream main --tags
git ls-remote https://github.com/Wei-Shaw/sub2api.git HEAD refs/heads/main refs/tags/*
git merge-base HEAD upstream/main
git log --oneline --decorate <merge-base>..upstream/main
git diff --stat <merge-base>..upstream/main
git diff --name-status <merge-base>..upstream/main
```

## Sources

- https://github.com/Wei-Shaw/sub2api
- https://github.com/Wei-Shaw/sub2api/commits/main
