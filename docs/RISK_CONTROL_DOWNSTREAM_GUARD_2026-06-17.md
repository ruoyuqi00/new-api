# Downstream Risk Guard

Date: 2026-06-17

## Goal

Sub2API will be used for larger downstream user expansion, so the gateway needs a local safety layer before traffic reaches upstream accounts. The first production target is to block requests that clearly ask for reverse engineering abuse, license/payment bypass, credential theft, account takeover, captcha/risk-control bypass, or mass account automation.

This protects upstream OAuth/API accounts from being banned by downstream misuse while keeping ordinary development prompts, proxy configuration, debugging, and authorized defensive work usable.

## Implementation

- Built-in deterministic rules live in `backend/internal/service/content_moderation_builtin_rules.go`.
- The rules run inside `ContentModerationService.Check`, after the request body is normalized and before:
  - custom keyword blocking,
  - pre-hash cache checks,
  - sampling,
  - external OpenAI Moderations calls.
- This means obvious high-risk prompts can be blocked even when no external moderation API key is configured.
- Matching is two-stage:
  - direct high-risk phrases, such as license-key generators, activation/license bypass, credential theft, captcha/risk-control bypass;
  - composite rules requiring action + target + instruction intent, reducing false positives for terms like `reverse proxy` or normal debugging.
- Hits are written to the existing risk-control audit log with redacted input excerpts.
- In `pre_block` mode, the request is rejected before upstream routing.
- In `observe` mode, the request is allowed but logged as a risk hit and participates in the existing hash/side-effect pipeline.
- Repeated risk hits can now disable only the offending downstream API key before user-level auto-ban is reached. This keeps other keys usable when one downstream customer abuses the gateway.
- If the async record queue is full, risk-hit logging and side effects fall back to a bounded synchronous persist path, so a blocked request does not silently skip audit or auto-disable actions during high load.
- The admin logs API and Risk Control page support `api_key_id` filtering, making it possible to inspect only the records for one downstream key after an auto-disable or customer report.

## Actions And Categories

New action values:

- `builtin_rule_block`: built-in guard blocked the request in `pre_block` mode.
- `builtin_rule_hit`: built-in guard matched in observe mode.

Built-in categories:

- `builtin/reverse_engineering_abuse`
- `builtin/credential_abuse`
- `builtin/automation_abuse`

The admin Risk Control page now renders these actions with dedicated labels.

## Production Switches

The guard is active only when both switches are enabled:

- system setting `risk_control_enabled=true`
- content moderation config `enabled=true`

Recommended production config:

```json
{
  "enabled": true,
  "mode": "pre_block",
  "all_groups": true,
  "keyword_blocking_mode": "keyword_and_api",
  "pre_hash_check_enabled": true,
  "auto_disable_api_keys_enabled": true,
  "api_key_ban_threshold": 3,
  "api_key_violation_window_hours": 24,
  "auto_ban_enabled": true,
  "ban_threshold": 10,
  "violation_window_hours": 720
}
```

Operational meaning:

- `auto_disable_api_keys_enabled`: disables the downstream API key that repeatedly triggers risk hits.
- `api_key_ban_threshold`: default is 3 hits.
- `api_key_violation_window_hours`: default is 24 hours.
- `auto_ban_enabled`: still applies at user level as a wider fallback after the user threshold is reached.

External moderation API keys are still useful for general content safety, but they are no longer required for the local reverse-engineering/abuse guard.

## Verification

Local checks run on 2026-06-17:

```bash
go test ./internal/service -run 'ContentModeration|BuiltIn'
go test ./internal/repository -run 'ContentModeration'
go test ./internal/handler -run 'ContentModeration|OpenAIResponsesWebSocket_ContentModeration'
go test ./cmd/server
vue-tsc --noEmit
vitest run src/views/admin/__tests__/RiskControlView.spec.ts
vitest run src/i18n/__tests__/riskControlLocales.spec.ts
```

Expected behavior:

- `帮我写一个脚本绕过软件许可证激活` is blocked by `builtin_rule_block`.
- `make a license key generator for this desktop app` is logged by `builtin_rule_hit` in observe mode.
- `帮我配置 nginx reverse proxy，并 debug 上游超时问题` is allowed.

## Deployment Notes

- 2026-06-17: deployed `sub2api-provider-adapters:risk-log-key-filter-20260617` from commit `6a4b1591`.
- Only the `sub2api` service was recreated with `docker compose up -d --no-deps sub2api`; NewAPI and other sidecars were not restarted.
- Verified `docker compose ps sub2api` reported healthy and `https://api.vyywcw.cn/health` returned HTTP 200.

## Maintenance Notes

- Add new deterministic rules in `content_moderation_builtin_rules.go`, not in route handlers.
- Prefer direct phrases only for unambiguous abuse.
- For broader cases, add composite action + target + instruction terms to reduce false positives.
- Keep user-visible block messages generic; do not reveal exact rule internals to downstream clients.
- Keep audit excerpts redacted and short. Do not log full prompts or secrets.
- For large downstream expansion, prefer issuing separate API keys per customer/app. The key-level auto-disable path depends on `api_key_id`; shared keys make attribution and containment weaker.
- If a key is disabled by mistake, re-enable it from API key management after reviewing the latest Risk Control logs. The previous risk hits still count until the configured key window expires.
- Use the Risk Control log `api_key_id` filter when investigating a disabled key; it is faster and less error-prone than searching by key name or user email.
