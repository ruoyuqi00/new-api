# Upstream Usage Safety Implementation Plan

## Goal

Protect GPT text billing from malformed or amplified upstream usage while
preserving one-shot settlement for accepted/ambiguous requests. Leave image,
video, audio, UI, database schema, routing, Caddy, and production unchanged.

## Tasks

1. Add a shared GPT-text usage validator with deterministic tests for valid
   usage, empty/negative/inconsistent values, cache bounds, integer overflow,
   and geometric amplification.
2. Apply the validator only to OpenAI Responses, Chat Completions, and their
   existing text conversion paths. Invalid terminal usage must be treated as
   unconfirmed, sanitized before downstream serialization, and replaced by a
   local estimate for diagnostics only.
3. Cap unconfirmed GPT-text settlement at the frozen pre-consume reservation
   when an upstream submission may already have been accepted. Preserve the
   existing no-retry/no-switch and idempotent billing coordinator behavior.
4. Add regression tests proving malformed usage does not become confirmed
   usage, cache-hit statistics, tiered authoritative billing, or a second
   submission; accepted/ambiguous settlement occurs at most once and uses the
   reservation boundary. Keep media tests unchanged.
5. Run focused relay/service tests, full touched Go packages, `go vet` for
   touched packages, `git diff --check`, and the existing local UI smoke check.

## Verification Commands

```text
go test ./service -run 'TextUsage|TextQuota|Billing' -count=1
go test ./relay/channel/openai ./relay ./controller -run 'Usage|Responses|Chat|Recovery|Billing' -count=1
go test ./service ./relay/channel/openai ./relay ./controller -count=1
go vet ./service ./relay/channel/openai ./relay ./controller
git diff --check
```

## Safety Constraints

- Do not connect to or mutate production.
- Do not print credentials, request headers, upstream URLs, raw database rows,
  or access logs.
- Do not modify media/image/video code paths.
- Do not change the production-brand frontend baseline.
