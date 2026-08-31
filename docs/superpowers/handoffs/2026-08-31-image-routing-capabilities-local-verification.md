# Image Routing Capabilities: Local Verification

Date: 2026-09-01 (Asia/Shanghai)
Branch: `codex/first-token-image-capability-20260831`
Commit: `875e17903`

## Scope

- Public image model names remain canonical and provider model mapping remains internal.
- Image selection now carries canonical model, resolution tier, exact dimensions, and ratio requirements through retries, affinity, memory lookup, database lookup, and channel-pool filtering.
- Unknown or pending channel capabilities are square-only. Non-square requests require an explicitly verified channel capability; higher tiers cannot fall back to lower tiers.
- OpenAI-compatible payloads preserve explicit `width x height`; Imagen/Gemini payloads preserve native tier and aspect-ratio fields where supported.
- Existing billing expression, task billing, channel affinity, violation fee, and actual-response-model paths remain reused and were not redesigned.

## Local verification

- Isolated local server: `http://127.0.0.1:13016/`
- Independent SQLite database under the worktree `tmp/local-ticket-candidate/`; no production database or cookies.
- `go test ./... -count=1` passed.
- Image routing, selection, mapping, adapter, and regression tests passed as part of the full suite.
- `web/default`: `bun test tests/tickets-api.test.ts tests/tickets.test.ts` passed.
- `web/default`: `bun run typecheck` passed.
- `web/default`: `bun run build` passed.
- `git diff --check` passed.

## Review checklist

Before any production action, review the local branded UI at `/sign-in` and `/setup`, and exercise image requests only against deterministic mocks. Verify canonical model display, exact dimensions/aspect ratio, tier boundaries, generic no-compatible-channel errors, and that no upstream URL, credentials, or provider response body appears in user-facing errors or logs.

No paid production image request was issued for this verification. Production container, Caddy routing, production database, and production traffic were not changed.

## Rollback

The implementation can be reverted to the parent production-derived commit with a normal Git revert after review. Do not use a production deployment or database migration until the user explicitly approves the local candidate and a separate production rollback point is recorded.
