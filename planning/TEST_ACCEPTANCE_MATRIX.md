# Test and Acceptance Matrix

记录日期：2026-05-16

这个矩阵用于每次调研、上线和回滚前后验收。

## Local repository checks

| ID | Test | Command | Pass condition |
| --- | --- | --- | --- |
| L1 | Git clean | `git status --short --branch` | no unexpected changes |
| L2 | Upstream visible | `git remote -v` | origin private, upstream official, upstream push disabled |
| L3 | Plan docs tracked | `git status --short planning` | docs appear when edited, not ignored |
| L4 | Official sync preview | `git fetch upstream main && git log --oneline HEAD..upstream/main` | no surprise before merge |

## Windsurf direct proxy tests

| ID | Test | Endpoint | Pass condition |
| --- | --- | --- | --- |
| W1 | Container running | `docker compose ps windsurf-api` | running/healthy |
| W2 | Health | `GET /health` | 200 or documented health response |
| W3 | No account behavior | `POST /v1/messages` before import | clear no-active-account error |
| W4 | Single account import | `POST /auth/login {"token":"..."}` | success or clear duplicate/error |
| W5 | Batch account import | `POST /auth/login {"accounts":[...]}` | per-account result |
| W6 | Account list | `GET /auth/accounts` | imported accounts visible without secrets |
| W7 | Anthropic non-stream | `POST /v1/messages stream=false` | valid message response |
| W8 | Anthropic stream | `POST /v1/messages stream=true` | valid SSE sequence |
| W9 | OpenAI chat non-stream | `POST /v1/chat/completions` | valid OpenAI response |
| W10 | OpenAI chat stream | `POST /v1/chat/completions stream=true` | valid SSE chunks |
| W11 | Tool call | `/v1/messages` with tools | tool_use returned or documented unsupported |
| W12 | Rate limit | use exhausted/limited account | error classified clearly |
| W13 | Restart persistence | restart container | accounts persist |
| W14 | Public exposure | external curl to port/domain | not reachable |

## Kiro direct proxy tests

| ID | Test | Endpoint | Pass condition |
| --- | --- | --- | --- |
| K1 | Container running | `docker compose ps kiro-rs` | running/healthy |
| K2 | Models | `GET /v1/models` | valid model list or documented behavior |
| K3 | Social refresh | import social credential | access token refreshed |
| K4 | IdC refresh | import idc credential | access token refreshed |
| K5 | API key mode | import `kiroApiKey` | no refresh needed |
| K6 | Anthropic stream | `POST /v1/messages stream=true` | valid SSE sequence |
| K7 | Count tokens | `POST /v1/messages/count_tokens` | token count response |
| K8 | Claude Code route | `POST /cc/v1/messages` | buffered usage behavior verified |
| K9 | invalid_grant | invalid refresh token | account disabled/permanent failure |
| K10 | Restart persistence | restart container | credentials persist |
| K11 | Public exposure | external curl to port/domain | not reachable |

## Sub2API upstream account tests

| ID | Test | Account shape | Pass condition |
| --- | --- | --- | --- |
| S1 | Create Windsurf Anthropic account | `platform=anthropic,type=apikey,base_url=http://windsurf-api:3003` | account saves |
| S2 | Test Windsurf Anthropic account | admin account test | SSE test success |
| S3 | Public route to Windsurf | public `/v1/messages` | response from Windsurf-backed model |
| S4 | Create Windsurf OpenAI account | `platform=openai,type=apikey,base_url=http://windsurf-api:3003/v1` | account saves |
| S5 | Test Windsurf OpenAI account | admin account test | success or documented Responses limitation |
| S6 | Create Kiro Anthropic account | `platform=anthropic,type=apikey,base_url=http://kiro-rs:8990` | account saves |
| S7 | Test Kiro Anthropic account | admin account test | SSE test success |
| S8 | Model mapping | requested model differs from upstream model | mapped correctly |
| S9 | URL allowlist | Docker service host base_url | accepted or config updated |
| S10 | Error passthrough | invalid upstream key | user sees useful error, no secret leak |

## Security tests

| ID | Test | Pass condition |
| --- | --- | --- |
| SEC1 | No secrets in Git | `rg -n "<real secret patterns>"` finds nothing |
| SEC2 | No adapter public route | Caddy has no windsurf/kiro route |
| SEC3 | No public adapter port | `docker compose port windsurf-api 3003` empty or localhost-only |
| SEC4 | Dashboard protected | `DASHBOARD_PASSWORD` set |
| SEC5 | API protected | internal adapter `API_KEY` set |
| SEC6 | Logs redacted | logs do not print tokens |
| SEC7 | Temp import files deleted | no `/root/*token*` leftovers |
| SEC8 | Backup protected | backup files mode/root-only |

## Deployment tests

| ID | Test | Pass condition |
| --- | --- | --- |
| D1 | Compose validates | `docker compose config` exits 0 |
| D2 | Backup succeeds | backup file exists and non-empty |
| D3 | Restart safe | `docker compose restart windsurf-api` no data loss |
| D4 | Sub2API public health | `curl -I https://api.vyywcw.cn/` good response |
| D5 | DB/Redis untouched | existing accounts/groups still visible |
| D6 | Rollback tested | stop adapter + disable account restores stable route |

## Native Windsurf import tests

Use when implementing Sub2API native import.

| ID | Test | Input | Pass condition |
| --- | --- | --- | --- |
| NW1 | Parse line tokens | one token per line | normalized account list |
| NW2 | Parse JSON tokens | `{"accounts":[...]}` | normalized account list |
| NW3 | Reject empty input | empty body | 400 with clear message |
| NW4 | Redact logs | import with token | logs do not contain token |
| NW5 | Partial success | one good, one bad token | per-account result |
| NW6 | Duplicate | same token twice | duplicate reported |
| NW7 | Internal proxy down | import when WindsurfAPI stopped | clear internal error |
| NW8 | Idempotency | retry same request | no duplicate side effects |
| NW9 | Frontend i18n | zh/en UI | strings present |
| NW10 | Per-item result | one good and one bad item | `succeeded`, `failed`, and `items[]` match rows |

## Native Kiro import tests

Use when implementing Sub2API native Kiro support.

| ID | Test | Input | Pass condition |
| --- | --- | --- | --- |
| NK1 | Social credential validation | missing refreshToken | rejected |
| NK2 | IdC validation | missing clientSecret | rejected |
| NK3 | API key validation | missing kiroApiKey | rejected |
| NK4 | Region fallback | credential and config regions | priority matches spec |
| NK5 | Duplicate detection | same refreshToken/API key | duplicate reported |
| NK6 | invalid_grant | revoked token | account disabled |
| NK7 | Secret redaction | failed refresh | no secret in logs |
| NK8 | Machine id | same credential | stable id |
| NK9 | Account test | imported credential | `/v1/messages` success |

## Acceptance gates

Gate 1: Internal upstream ready.

- W1-W10 pass.
- SEC1-SEC5 pass.
- No public exposure.

Gate 2: Sub2API routed.

- S1-S3 pass for Windsurf.
- S6-S7 pass for Kiro if enabled.
- D1-D5 pass.

Gate 3: Production stable.

- 24h no unexplained container restarts.
- No disk growth issue.
- No token leakage in logs.
- Backup and rollback tested.

Gate 4: Native import ready.

- NW1-NW9 pass for Windsurf.
- NK1-NK9 pass for Kiro.
- Backend and frontend tests pass.
- Server deployment runbook updated.

## 2026-05-18 verified results

Windsurf Stage A server deployment:

| ID | Result | Note |
| --- | --- | --- |
| W1 | PASS | `windsurf-api` running and healthy |
| W4 | PASS | email/password import succeeded through Sub2API import endpoint |
| W5 | PASS | batch-shaped `accounts[].api_key` import succeeded |
| W6 | PASS | account list visible internally without printing secrets |
| W7 | PASS | direct internal `/v1/messages` returned 200 for `claude-sonnet-4.6` |
| W14 | PASS | no public host port or Caddy route for `windsurf-api` |
| S1 | PASS | internal Anthropic API key account exists as `windsurf-internal-anthropic` |
| S3 | PASS | public Sub2API `/v1/messages` returned 200 for `gemini-2.5-flash` and `claude-sonnet-4.6` |
| S9 | PASS | Docker service base URL works after private/insecure internal URL settings |
| SEC2 | PASS | only Sub2API is publicly exposed |
| SEC3 | PASS | `windsurf-api` has no public host port |
| SEC5 | PASS | internal adapter API key is configured |
| NW2 | PASS | JSON account import accepted |
| NW5 | PASS | upstream import response summarized without leaking secrets |
| NW8 | PASS | idempotency payload hashes secrets instead of storing raw credentials |

Observed upstream issue:

- WindsurfAPI v2.0.96 can have a Pro/Trial account where
  `capabilities["gemini-2.5-flash"].ok == true` but `availableModels` does not
  include `gemini-2.5-flash`.
- Symptom is HTTP 403 `model_not_entitled` before the request reaches the
  account selection path.
- Short-term server correction is documented in
  `planning/SERVER_DEPLOYMENT_RUNBOOK.md`.
- Long-term fix candidate: accept successful capability probes as available
  unless a model is explicitly blocked or marked `not_entitled`.
