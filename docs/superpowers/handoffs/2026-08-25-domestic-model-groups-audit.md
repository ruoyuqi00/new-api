# Domestic Model Groups Audit

Date: 2026-08-25 (Asia/Shanghai)
Branch: `codex/grok-production-baseline-20260822`

## Scope

This audit covers the two requested user-visible groups and the supplied upstream account pool. It does not change production containers, Caddy, production options, production channels, or production data. No paid completion request was sent.

## Upstream verification

- Base URL: `https://api.herohao.top/v1`
- Authentication: both supplied API keys authenticated successfully; this record stores no key material.
- Probe: `GET /v1/models` only.
- Models returned by both keys: `deepseek-v4-flash-0731`, `deepseek-v4-pro-0813`, `glm-5.2`, `kimi-k3`, `MiniMax-M2.7`, `qwen3.8-max`.
- The upstream key groups were observed as the call group and usage group respectively; the key values are not copied into this repository.
- The upstream public pricing/model-plaza endpoints did not provide a usable per-call price export during the read-only audit.

## Price evidence

The expression engine uses USD prices. The verified official tables were published in CNY, so a local apply must divide the source coefficients by the locally configured `USDExchangeRate`, freeze that rate in the apply snapshot, and never fetch prices at runtime.

Verified for usage pricing:

- GLM-5.2: input 8 CNY/M, output 28 CNY/M, cache read 2 CNY/M.
- MiniMax-M2.7: input 2.1 CNY/M, output 8.4 CNY/M, cache read 0.42 CNY/M, cache write 2.625 CNY/M.

Pending and disabled:

- DeepSeek date-suffixed aliases: exact official alias/Beijing peak schedule not verified.
- Kimi-K3: exact official price source not verified.
- Qwen3.8-max: deployment region and cache policy not confirmed.
- All six `-call` aliases: no official per-call price source was verified.

## Local implementation status

- `国模按量` uses existing `tiered_expr` billing.
- `国模按次` uses the isolated `per_call_expr` mode. Its expressions return a fixed USD price per accepted request and may select a price by `len`; token pricing variables are rejected in this mode.
- The existing group ratio, reservation, settlement, task billing, channel affinity, violation-fee, alias mapping, and `ActualResponseModel` paths are reused.
- `-call` aliases remain public/logged names while outbound model names use the official upstream identifiers.
- When a per-call upstream response has no authoritative usage, the frozen per-call reservation is settled and the audit log marks `usage_unconfirmed`, `estimated_tier`, and `settled_from_reservation`.
- No production channel or option has been created by this branch.

## Apply gate

Before local or production application, provide/confirm the effective local `USDExchangeRate` and official per-call price table. The manifest and validator intentionally keep those values unset/pending. A production import must first capture `/api/pricing` and secret-free channel/ability snapshots, apply atomically or with a compensating rollback, and remain blocked until the local candidate is approved.
