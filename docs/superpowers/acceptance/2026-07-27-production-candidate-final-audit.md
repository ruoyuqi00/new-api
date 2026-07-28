# YuCore Production Candidate Final Audit

## Decision

The local production candidate satisfies the local-only objective. Production
replacement remains a separate operation and requires explicit user approval.

Candidate worktree and branch:

```text
D:\yucore-local-production
codex/local-production-brand-performance-20260725
```

The accepted code and cutover runbook end at `a303d9023`. This audit is an
evidence-only follow-up commit; repository Markdown is excluded from the Docker
build context.

## Requirement Audit

| Requirement | Authoritative evidence | Result |
| --- | --- | --- |
| Preserve production UI lineage and visible brand motion | Renderer lifecycle commits `953ef8f1b`, `fff06c06b`, and `7eb2d11f4`; two active canvases with nonzero pixel motion in every accepted profile | Passed |
| Remove the expensive stable-home renderer handoff | Three same-machine candidate runs recorded zero shader compilations after stable activation; the control recorded four in every run | Passed |
| Anonymous, user, and super-admin UI | 38 route/profile cases with desktop light and mobile dark coverage; no unexpected console error, error-page redirect, horizontal overflow, or local API 5xx | Passed |
| Routing, account pools, affinity, and failover | Fresh affected-package test gate plus focused contracts for `401`, `429`, retryable `5xx`, transport failure, failed-account exclusion, lower-priority pool/channel fallback, and stale-affinity clearing | Passed |
| Private groups | Unauthorized pricing discovery remains filtered; authorized user model and pricing discovery tests pass | Passed |
| Billing correctness | Fixed-price, per-call image/video, cache, token-ratio, and tiered-expression tests pass; mapped requests bill by `OriginModelName` | Passed |
| Hide mapped upstream details from users | Ordinary user log/task DTOs remove `is_model_mapped` and `upstream_model_name`; supported mapped response shapes return the public model; admin diagnostics retain upstream metadata | Passed |
| SQLite, MySQL, and PostgreSQL compatibility | Current candidate Linux binary completed full empty-schema migration and a second healthy start on SQLite, MySQL 5.7.44, and PostgreSQL 9.6.24 | Passed |
| Production builds | Go build, default frontend build, and classic frontend build all exit zero | Passed |
| Fast rollback and traffic continuity | Marker-upstream Caddy rehearsal completed forward and rollback reloads with zero failed requests; the runbook also drains in-flight relay counters before recreating or deleting an upstream | Passed |
| Keep experimental UI and production untouched | No changed path under `web/experimental`, `local-ui`, or `output`; no production push, image replacement, restart, migration, Caddy reload, DNS change, or channel/price/account mutation occurred | Passed |

## Fresh Verification

Run serially on 2026-07-27 after temporary cross-database resources were
removed and Docker Desktop was shut down:

```text
go test ./middleware ./service ./model ./controller ./relay/... ./pkg/billingexpr ./setting/... -count=1
  PASS

go build ./...
  PASS

web/default: bun test
  131 passed, 0 failed

web/default: bun run typecheck
  PASS

web/default: oxlint on every TypeScript/JavaScript file changed by the
production reliability and UI batch
  PASS

web/default: bun run build
  PASS

web/classic: bun run build
  PASS

git diff --check
  PASS
```

An earlier parallel verification attempt exhausted local memory inside `tsgo`.
The failure was `runtime: cannot allocate memory`, not a type diagnostic. The
same typecheck and every other gate passed when rerun serially, so the final
evidence uses only serial results.

## Cross-Database Migration Evidence

The current candidate was compiled as a static Linux binary and mounted into a
temporary local container. Each database used an empty ephemeral database,
`NODE_TYPE=master`, disabled batch/task work, and a loopback-only health port.

| Database | Server evidence | First start | Second start | Tables after first start |
| --- | --- | --- | --- | ---: |
| SQLite | current embedded driver | healthy | healthy | full candidate schema |
| MySQL | `5.7.44`, `utf8mb4`, `utf8mb4_unicode_ci` | healthy | healthy | 44 |
| PostgreSQL | `9.6.24` | healthy | healthy | 44 |

PostgreSQL 9.6 performed slow `information_schema` metadata queries during the
second GORM migration pass and took longer than the first 50-second probe. It
continued without an error and became healthy during the next condition-based
probe. This is a startup-duration observation, not a migration failure.

All temporary application/database containers, network, SQLite volume, test
binary, Windows test binary, and the newly pulled `postgres:9.6` image were
removed. Docker Desktop was shut down. Ports `13002`, `13306`, and `15432` are
closed; the pre-existing local previews on `3000`, `3001`, and `13000` remain
listening.

## Renderer And Role Evidence

The committed performance record is
`docs/superpowers/perf/2026-07-25-local-baseline.md`. Local visual artifacts are
under:

```text
C:\Users\ASUS\.codex\visualizations\2026\07\15\019f63c5-4a5a-7f83-b0c0-527099503d45\local-production-role-audit-20260726
```

The artifact set contains:

- 38 anonymous/user/admin route-profile results;
- desktop/mobile and light/dark real-scroll viewport captures;
- 50 renderer-handoff viewport screenshots;
- control/candidate CPU profiles and handoff reports;
- cache-read/cache-write table and detail inspection evidence;
- production-build performance samples.

The candidate does not claim a startup-speed improvement. FCP/LCP were
effectively unchanged, and the long-task sample ranges overlap. The proven
improvement is resource lifetime: stable activation no longer recompiles the
signal-field and Earth shaders, offscreen/hidden render loops release frame
ownership, and visible motion remains enabled.

## Caddy Rehearsal Evidence

The first real-app load probe reached the local gateway rate limit and returned
HTTP 429. That was not treated as a Caddy disconnect. The isolated marker
rehearsal then measured only proxy continuity:

```text
Forward switch: 240 requests, 0 failures
  36 control responses, 204 candidate responses

Rollback: 240 requests, 0 failures
  35 candidate responses, 205 control responses
```

The production runbook is
`docs/superpowers/runbooks/2026-07-27-production-cutover.md`. It validates a
candidate Caddyfile before reload, retains the immutable old image, starts the
candidate as a slave, and waits for `http_stats.active_connections` to remain
zero on three consecutive samples before recreating the old master or deleting
the candidate. A failed drain leaves both containers running and does not cut
an existing stream.

## Known Non-Blocking Baseline Debt

- Whole-default lint currently reports legacy errors outside this candidate's
  implementation batch. Candidate TypeScript/JavaScript files pass targeted
  lint.
- Whole-default formatting would rewrite unrelated files and more than two
  thousand lines of the shared CSS file. That mechanical churn is excluded
  from this production candidate.
- PostgreSQL 9.6 repeat migration is slow. The production runbook keeps traffic
  on the healthy candidate while the final master starts and blocks the switch
  until master health is green.

These items are recorded rather than mislabeled as passing, and none changes
the validated runtime contracts above.

## Production Boundary

Local candidate work is complete. The next authorized action, if approved, is
Phase 1 of the production runbook: build an immutable image from a clean
`git archive`, transfer it, repeat the read-only topology guard, and create
backups. No production command is implied by this audit.
