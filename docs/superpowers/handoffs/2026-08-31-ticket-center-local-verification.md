# Ticket Center: Local Verification

Date: 2026-09-01 (Asia/Shanghai)
Branch: `codex/first-token-image-capability-20260831`
Commit: `73bd3ffaac5d36f2f53edb3a1f7ff1dbd70a6eb3`

## Scope

- Added ownership-scoped user tickets and AdminAuth-protected admin ticket views.
- Added append-only multi-turn messages with transactional status, timestamps, and unread-counter updates.
- Added `general` and manual `refund` categories. Refund tickets are communication only: there is no automatic balance credit, refund transaction, or billing mutation.
- Added private authenticated attachments with content sniffing, safe display names, per-message count limits, size limits, and owner/admin authorization.
- Added branded user/admin routes, sidebar entries, filters, conversation view, reply, close/reopen, and i18n coverage without changing existing billing or navigation behavior.

## Local verification

- Isolated local server: `http://127.0.0.1:13016/`
- Independent SQLite database under the worktree `tmp/local-ticket-candidate/`; no production database or cookies.
- `go test ./... -count=1` passed, including model migration, controller authorization, service state-machine, attachment, billing, affinity, violation-fee, image, and actual-response-model regression suites.
- `web/default`: `bun test tests/tickets-api.test.ts tests/tickets.test.ts` passed.
- `web/default`: `bun run typecheck` passed.
- `web/default`: `bun run build` passed.
- Scoped ticket lint passed. The repository-wide lint command still reports pre-existing unrelated errors outside this change.
- Locale synchronization completed with the existing i18n script; no missing new keys were reported.

## Review checklist

Review the local branded UI at `/sign-in`, `/setup`, `/tickets`, and `/admin-tickets`. After creating a ticket, verify a user reply and admin reply, closed-ticket behavior, unread state, ownership isolation, manual-refund wording, and private attachment authorization. Do not enter real API keys, tokens, or production customer data in the local candidate.

Production container, Caddy routing, production database, balances, logs, and traffic were not changed. Production migration and deployment require explicit approval after local UI review.

## Rollback

The implementation is isolated on the branch above. Use a normal Git revert to return to the parent production-derived state if review finds a regression; never restore or overwrite a production database snapshot as part of rollback.
