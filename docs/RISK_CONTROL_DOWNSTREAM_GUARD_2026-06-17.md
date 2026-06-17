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
  "auto_ban_enabled": true,
  "ban_threshold": 10,
  "violation_window_hours": 720
}
```

External moderation API keys are still useful for general content safety, but they are no longer required for the local reverse-engineering/abuse guard.

## Verification

Local checks run on 2026-06-17:

```bash
go test ./internal/service -run 'ContentModeration|BuiltIn'
vue-tsc --noEmit
```

Expected behavior:

- `帮我写一个脚本绕过软件许可证激活` is blocked by `builtin_rule_block`.
- `make a license key generator for this desktop app` is logged by `builtin_rule_hit` in observe mode.
- `帮我配置 nginx reverse proxy，并 debug 上游超时问题` is allowed.

## Maintenance Notes

- Add new deterministic rules in `content_moderation_builtin_rules.go`, not in route handlers.
- Prefer direct phrases only for unambiguous abuse.
- For broader cases, add composite action + target + instruction terms to reduce false positives.
- Keep user-visible block messages generic; do not reveal exact rule internals to downstream clients.
- Keep audit excerpts redacted and short. Do not log full prompts or secrets.
