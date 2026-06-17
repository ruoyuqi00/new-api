# Scheduler Concurrency And Cache Update - 2026-06-17

## Decision

The short path for higher concurrency is to keep NewAPI as the public control plane and use Sub2API as the internal strong adapter and scheduler layer.

- NewAPI remains responsible for users, keys, quota, logs, groups, and public-facing operations.
- Sub2API remains responsible for provider-specific protocol adaptation, account pools, sticky routing, scheduling, retry/failover behavior, and adapter-side cache decisions.

## Change

Sub2API now has a Redis-backed short account selection cooldown:

- when failover passes failed accounts through `excludedIDs`, the scheduler stores those account IDs in a short Redis cooldown bucket keyed by group and requested model;
- subsequent load-aware selections fetch the active cooldown set once per candidate list and skip those accounts across routing, sticky, and normal load-aware selection;
- the same mechanism is wired into both `GatewayService` and `OpenAIGatewayService`, covering Anthropic/Gemini/Kiro-style routes and OpenAI/Codex/CPA-style routes;
- sticky bindings are not deleted just because of this short cooldown, so a healthy account can be reused after the cooldown expires;
- long-lived account state remains handled by existing `temp_unschedulable`, rate-limit, overload, and error-policy logic.

Default config:

```yaml
gateway:
  scheduling:
    account_selection_cooldown_ttl_seconds: 8
```

Set the value to `0` to disable the short cooldown.

## Why

Under high concurrency, a recently failed upstream account can otherwise be selected by many simultaneous requests before slower account-state updates propagate through scheduler snapshots. The short Redis cooldown reduces repeated collisions against the same failed account and improves cache hit behavior without rewriting the scheduler.

## Verification

Local tests:

```text
go test ./internal/repository -run TestGatewayCacheAccountSelectionCooldowns -count=1
go test -tags unit ./internal/service -run TestGatewayService_SelectAccountWithLoadAwareness -count=1
go test ./internal/service -run 'TestOpenAISelectAccountWithLoadAwareness_(FiltersUnschedulable|SelectionCooldownSkipsFailedAccount)|TestShouldStopOpenAIOAuth429Failover_OnlyDuringStorm' -count=1
go test ./internal/config -count=1
```

Deployment:

- Built image: `sub2api-provider-adapters:scheduler-cooldown-20260617`
- Deployed service: `sub2api`
- NewAPI was not restarted.
- Sub2API health: HTTP 200, `{"status":"ok"}`

Public smoke:

- `POST /v1/messages/count_tokens` with the `kiro` group key: HTTP 200
- `POST /v1/messages` with the `kiro` group key: HTTP 200
- `POST /v1/chat/completions` with existing `GPT5.5` keys: HTTP 502, caused by upstream OpenAI/Codex tokens being revoked/invalidated. This is account-pool state, not a deployment or scheduler crash.
- Redis contains `account_selection_cooldown:*` keys after GPT5.5 failover attempts, confirming the new short cooldown path is active.
