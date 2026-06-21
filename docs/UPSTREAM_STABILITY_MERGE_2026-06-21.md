# Upstream Stability Merge - 2026-06-21

This note records the upstream protocol, reliability, scheduling, and operations updates merged from `Wei-Shaw/sub2api` on 2026-06-21.

## Merged

- OpenAI Images failover fixes, including server-error failover and preserved upstream error bodies.
- Non-streaming gateway failover for non-JSON `2xx` upstream responses.
- zstd upstream response decompression.
- Chat Completions to Responses compatibility fix: missing tool `strict` now defaults to `false`.
- Anthropic 5h / 7d window cooldown preservation.
- OAuth token refresh retry amplification reduction, plus non-retryable `invalid_refresh_token` and `app_session_terminated`.
- Responses fallback anchoring and `/responses` probe tool-call capability validation.
- Streaming Haiku `max_tokens=1` probe interception.
- API key ACL denial message now includes client IP.
- Antigravity system-role message merge.
- Account list parameter-limit fix, account ID display, frontend `form-data` override, and queue hot-path accounting cleanup.
- Channel monitor jitter setting.
- Claude OAuth system prompt block settings.
- OpenAI quota query/reset admin support.
- Chinese LLM and multimodal fallback pricing additions.
- Thinking/reasoning protocol updates:
  - protocol-aware thinking block filtering;
  - DeepSeek `reasoning_effort=max` normalized to `xhigh`;
  - MiniMax `thinking.type=enabled` rewritten to `adaptive`;
  - thinking-enabled Chinese model default `reasoning_effort`.
- Scheduler outbox high-concurrency fixes:
  - snapshot coalescing;
  - dedup key migrations;
  - invalid dedup-index recovery;
  - dedup release on claim;
  - consumed row cleanup with grace window;
  - typed-nil scheduler payload dedup fix.

## Verified

Local Go toolchain used for verification: `D:\wflogin\.tools\go1.26.4\go\bin\go.exe`.

Focused tests run while merging:

```powershell
go test -count=1 ./internal/repository -run 'Test.*(Decompress|TempUnsched|RefreshCandidates|SchedulerOutbox)'
go test -count=1 ./internal/pkg/apicompat -run 'Test.*(Strict|ChatCompletions|Responses)'
go test -count=1 ./internal/service -run 'Test.*(OpenAI.*Image|Failover|NonJSON|429|AnthropicWindow|TokenRefresh|SessionWindow)'
go test -count=1 ./internal/handler -run 'Test.*(OpenAI.*Image|Failover)'
go test -count=1 ./internal/service -run 'Test.*(Thinking|Reasoning|DeepSeek|MiniMax|GatewayRequest|ForwardAs|Hotpath)'
go test -count=1 ./internal/repository -run 'Test.*(SchedulerOutbox|Outbox|Dedup|Migration|AccountRepo|RefreshCandidates|Notx)'
go test -count=1 ./internal/service -run 'Test.*(Scheduler|Outbox|Snapshot|Cleanup|Dedup)'
```

Final backend suite:

```powershell
go test -count=1 ./internal/repository ./internal/pkg/apicompat ./internal/pkg/antigravity ./internal/handler ./internal/handler/admin ./internal/server/middleware ./internal/service
```

Result: passed.

## Not Merged Yet

- `b62b573f feat(openai): cyber_policy ...`
  - Useful for downstream risk control, but it conflicts with our existing content-moderation extensions for built-in rules and downstream API key auto-disable.
  - Should be merged as a dedicated risk-control task that preserves both feature sets.
- `f8c80bf0 fix(auth): apply promo codes to oauth signups`
  - Business-policy related. It should be reviewed together with the current signup reward plan before enabling.
- `b63b4116 fix: remove unused billing attribution helper`
  - Cleanup-only change, conflicted with local billing code and was not required for runtime correctness.
- `b8a482e1 fix(ci): unblock main after recent merges`
  - CI-only upstream maintenance, not needed locally.
- `4a5665da chore: sync VERSION to 0.1.137`
  - Version-only upstream commit. Production image tags should continue using our private adapter tag convention.

## Deployment Status

Deployment was not completed during this pass because the production server control plane stopped responding after earlier containerized Go tests:

- SSH port was reachable at TCP level, but SSH banner exchange timed out.
- Public HTTPS health checks also timed out.
- No container restart or compose change could be safely performed while SSH was unavailable.

Next deployment step after server access returns:

1. Check load and stop any leftover Go test containers.
2. Build a new `sub2api-provider-adapters` image from this commit.
3. Update only the `sub2api` service image in `/opt/sub2api/docker-compose.yml`.
4. Run `docker compose up -d sub2api`.
5. Verify `sub2api` health and NewAPI bridge status.

