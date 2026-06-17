# Scalability And Risk-Control Automation Roadmap

Date: 2026-06-17

## Why This Exists

The current Sub2API deployment is now usable for the first round of downstream expansion, but it is not yet a finished large-scale commercial operations stack. The immediate guardrail is in place: obvious reverse-engineering, license-bypass, credential-abuse, and account-automation prompts are blocked before upstream accounts are touched, and repeated hits can disable only the offending downstream API key.

This document records what is already true, what is still incomplete, and how the follow-up automation should continue the work without relying on chat memory.

## Current Production Baseline

- Active maintenance repo: `D:\wflogin\sub2api-private`
- Private GitHub remote: `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`
- Production stack path: `/opt/sub2api`
- Current deployed Sub2API image: `sub2api-provider-adapters:risk-log-key-filter-20260617`
- Current risk-control commit: `6a4b1591 Add risk log API key filter`
- NewAPI was not restarted during this deployment.
- The independent Chinese-named registration project under `D:\wflogin` must stay isolated and must not be inspected unless the user explicitly changes that constraint.
- Server storage note: if the root disk is tight, prefer moving Docker's data root and the persistent compose volumes to the spare server disk such as `/www`. Moving only the app directory usually helps much less, because image layers, build cache, and container metadata can still stay under `/var/lib/docker`.
- Server storage status on 2026-06-17: DockerRootDir was migrated to `/www/docker`; Sub2API and NewAPI were brought back healthy after the move.

## What Is Done

- Built-in local risk rules run before upstream routing.
- Risk hits are logged with short, redacted excerpts.
- `pre_block` mode returns local HTTP 403 for blocked prompts.
- The async risk log queue has a synchronous fallback for hit records, so queue saturation does not skip audit and side effects.
- Repeated risk hits can disable the specific downstream API key.
- User-level auto-ban remains as a broader fallback.
- Admin Risk Control UI exposes key-level auto-disable settings.
- Admin Risk Control logs can now be filtered by `api_key_id`, so an operator can quickly inspect the exact downstream key that triggered risk hits before deciding whether to re-enable, delete, or keep it disabled.
- The service test suite now includes a cross-protocol built-in block matrix for Anthropic, OpenAI Chat, OpenAI Responses, Gemini, and OpenAI Images.
- The content-moderation input extractor now has cross-protocol multimodal regression coverage for Anthropic source images, OpenAI Chat image URLs, OpenAI Responses `input_image`, Gemini inline image data, and OpenAI Images reference images.
- Production verification on 2026-06-17:
  - public `/health` returned HTTP 200;
  - Sub2API container was healthy on `risk-keyguard-20260617`;
  - normal prompt was not blocked by the risk guard;
  - risky prompt returned HTTP 403;
  - a temporary test key was disabled after the configured hit threshold and then soft-deleted.
- Follow-up deployment on 2026-06-17:
  - `sub2api-provider-adapters:risk-log-key-filter-20260617` was deployed with `docker compose up -d --no-deps sub2api`;
  - public `/health` returned HTTP 200;
  - Sub2API container was healthy on the new image.

## What Is Still Not Finished

### 1. Abuse Prevention Depth

The current guard catches obvious requests. It is not yet a complete abuse-prevention layer.

Needed next:

- Add more deterministic patterns for account theft, token exfiltration, mass account creation, cracking, and bypass workflows.
- Add model/protocol-specific extraction tests for OpenAI Chat, OpenAI Responses, Anthropic Messages, Gemini, and image endpoints.
- Add safe allowlist handling for legitimate defensive/security audit contexts.
- Keep downstream error messages generic so rule internals are not exposed.

### 2. Downstream Tenant Isolation

Large user expansion should assume each customer, application, or reseller gets a separate key.

Needed next:

- Document and enforce one-customer-per-key operating practice.
- Prefer per-key quotas, RPM limits, IP ACLs, and group binding.
- Add aggregate admin reporting that ranks risky keys by hit count, IP, group, model, and endpoint.
- Consider key quarantine state separate from ordinary disabled state, so admin can distinguish risk-control action from manual disabling.

### 3. High-Concurrency Readiness

The stack has several good pieces already, including Redis caches, auth cache invalidation, account scheduling, and queue-based usage/risk logging. It still needs load-oriented hardening.

Needed next:

- Run repeatable load tests for auth, scheduler, risk guard, and hot gateway endpoints.
- Record p50/p95/p99 latency and error rate before and after changes.
- Review DB indexes for high-volume tables such as usage logs, risk logs, API keys, account groups, and account status.
- Add backpressure behavior notes for when upstream pools, Redis, DB, or risk queues are stressed.
- Avoid restarting NewAPI during Sub2API-only work.

### 4. Cache And Scheduler Hardening

Kiro, Windsurf, OpenAI/Codex, Gemini, and image flows have different failure modes. The current scheduler improvements are useful, but not enough for aggressive expansion.

Needed next:

- Keep account health state in Redis where possible, not only in process memory.
- Make transient failures, 401/403 auth failures, 429 rate limits, and stream timeouts produce different cooldown behavior.
- Keep sticky-session behavior fast but bounded, with clear TTLs and cleanup.
- Expand tests around model routing, fallback groups, and account capability filtering.

### 5. Protocol Drift Tracking

Provider protocols change often. This cannot be treated as a one-time merge.

Needed next:

- Check upstream Sub2API changes before major deploys.
- Compare provider reference projects for Kiro/Windsurf/OpenAI/Codex protocol changes.
- Update compatibility tests whenever request or response shapes change.
- Record every protocol merge in a dated doc.

## Automation Operating Rules

The follow-up automation should:

1. Work only in `D:\wflogin\sub2api-private`.
2. Never inspect the isolated Chinese-named registration-project folder under `D:\wflogin`.
3. Preserve NewAPI unless the user explicitly asks for NewAPI changes.
4. Prefer small, reviewable hardening increments.
5. Run focused backend/frontend tests after each code change.
6. Deploy Sub2API only after tests pass and only for changes that affect production behavior.
7. Update docs and push to `origin main` after meaningful progress.
8. Never print, commit, or document tokens, API keys, refresh tokens, passwords, or full account payloads.

## Suggested Automation Backlog

Priority order:

1. Add more risk-control extraction tests across all supported request protocols.
2. Add aggregate reporting for risk hits by API key, group, endpoint, and IP.
3. Add a dedicated `risk_disabled` or `quarantined` status if it fits the existing API key status model.
4. Add load-test scripts for auth + risk pre-block + scheduler hot paths.
5. Review and add DB indexes for risk log and API key lookup paths if missing.
6. Add Redis-backed counters for suspicious downstream behavior beyond content hits, such as high invalid-request rate or repeated blocked models.
7. Add a short production runbook for scaling users, issuing keys, handling false positives, and unblocking users safely.

## Pre-Launch Domain Access Checklist

Use this before opening the service to more downstream users.

1. Keep at least two API hostnames:
   - a production hostname that blocks normal mainland China access;
   - a short-term fallback or transition hostname that can be disabled or given
     the same block rule quickly.
2. Put all user-facing API hostnames behind the same CDN/WAF control plane.
   Do not rely on changing to an overseas domain registrar by itself.
3. Block mainland China at the edge while allowing Hong Kong and Taiwan:
   - block country code `CN`;
   - do not block `HK` or `TW`;
   - decide separately whether `MO` should be allowed.
4. Prevent origin bypass:
   - allow only the CDN/WAF IP ranges to reach origin ports 80/443, or use an
     origin tunnel;
   - do not publish a user-facing hostname that points directly at the server IP.
5. Keep the admin hostname separate from user API hostnames. Prefer IP allowlist
   or access-control login for the admin hostname.
6. Before final launch, make all generated configs, CCS snippets, docs, and user
   onboarding point to the blocked production hostname. Retire or block the
   transition hostname once migration is complete.

## Server Storage Runbook

Use this when the production root disk is tight and `/www` or another mounted
disk has enough spare capacity.

1. Diagnose before moving anything:
   - `df -h`
   - `docker info --format '{{.DockerRootDir}}'`
   - `docker system df -v`
   - `du -sh /var/lib/docker/* /opt/sub2api /opt/newapi 2>/dev/null`
2. If `/var/lib/docker` is the main consumer, schedule a maintenance window. A
   Docker data-root move requires stopping Docker, so it affects both Sub2API
   and NewAPI even if no application config changes.
3. Back up compose files, `.env` files, and local persistent data before the
   move. Do not print secrets while collecting diagnostics.
4. Prefer moving Docker's `data-root` to the spare disk, for example
   `/www/docker`, instead of only moving `/opt/sub2api` or `/opt/newapi`.
   The repository includes `deploy/migrate-docker-data-root.sh` for this flow.
   Its default mode is a dry-run diagnosis; the actual move requires
   `--execute` and should only be run during a maintenance window.
5. After the move, verify:
   - `docker info --format '{{.DockerRootDir}}'` points at the new disk;
   - `docker compose ps` shows Sub2API and NewAPI healthy;
   - public Sub2API `/health` returns 200;
   - NewAPI `/v1/models` still exposes the intended `gpt` models.
6. Keep the old Docker directory until the stack has been stable long enough to
   be confident rollback is not needed.

## Stop Conditions

Automation should stop and ask the user before:

- changing NewAPI;
- changing server DNS, TLS, Caddy, database schema in a risky migration, or billing logic;
- deleting production data;
- rotating production secrets;
- inspecting the isolated registration-project folder;
- making broad UI redesigns unrelated to risk control or operations.
