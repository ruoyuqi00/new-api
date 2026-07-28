# Cross-Server Migration Preparation Audit

## Verdict

Local migration preparation is complete and suitable for provisioning the new
server. Production cutover is not authorized and is not yet ready to execute.
It remains blocked on the execution-time inputs and security cleanup listed
below.

No production container, database, Redis instance, file, firewall, DNS record,
Cloudflare setting, channel, account, price, or traffic path was changed during
this preparation. No branch was pushed and no production image was deployed.

## Evidence Boundary

- Worktree: `D:\yucore-local-production`
- Branch: `codex/local-production-brand-performance-20260725`
- Heavy verification tree: `6faeddad91ea77cc043119cee3d2d38ab96649a2`
- Current pre-audit tree: `d0c9aa55b466f05be3758048290a5a6caf82d915`
- The only change after heavy verification was migration documentation and
  removal of a credential literal from the verification plan. Application,
  migration guard, and rehearsal code did not change.
- `ACCEPTED_COMMIT` is intentionally unset. It must name the later clean,
  reviewed commit used to build the immutable `linux/amd64` image.
- No production image was built or transferred during this local audit.

The current local history is not an acceptable push boundary. An earlier local
commit contains a now-removed legacy OLD SSH credential. The current tracked
tree is clean of that value, but Git history retains it. Before any push:

1. Rotate the legacy OLD SSH credential.
2. Create a clean squashed branch from the reviewed current tree, or otherwise
   purge the credential-bearing history.
3. Re-run the current-tree and history secret scans.
4. Set `ACCEPTED_COMMIT` only to that clean reviewed commit.

Direct push of this branch is prohibited.

## Migration Tool Verification

All commands ran locally and serially.

| Check | Result | Duration and evidence |
|---|---|---|
| Migration guard unit tests | PASS | 40/40 tests; 4.002 s wall time, 3.578 s reported by unittest |
| Disposable cross-server rehearsal | PASS | 44.516 s; all eight JSON acceptance fields were true |
| Rehearsal cleanup | PASS | Zero prefixed containers, networks, volumes, and temp directories; port 18080 clear |
| Docker volume stability | PASS | Full volume inventory remained 41 -> 41 |
| Local UI isolation | PASS | Port 13000 remained owned by PID 14220 throughout |

The rehearsal proved:

- exact MySQL forward restore equality;
- exact MySQL reverse-migration equality after a simulated post-cutover write;
- binary-safe Unicode data and schema/content hash preservation;
- Redis persistence restoration, representative expiry behavior, and key
  inventory equality;
- maintenance HTTP 503 with `Retry-After`;
- Caddy old -> maintenance -> new -> old rollback routing;
- cleanup after the successful rehearsal without global Docker pruning; the
  failure-cleanup implementation was checked statically but was not exercised
  by a Task 6 failure-injection run.

## Application Verification

| Command | Result | Duration and notes |
|---|---|---|
| `go test ./middleware ./service ./model ./controller ./relay/... ./pkg/billingexpr ./setting/... -count=1` | PASS on identical retry | 26.568 s |
| `go build ./...` | PASS | 110.276 s |
| `web/default: bun test` | PASS | 131/131 tests, 0 failures, 0.835 s |
| `web/default: bun run typecheck` | PASS | 5.946 s |
| `web/default: bun run build` | PASS | 15.271 s |
| `web/classic: bun run build` | PASS | 9.035 s |
| `git diff --check` | PASS | 0.715 s |

The first invocation of the exact Go test command began with approximately
1.96 GiB free host memory. The `aws` and `moonshot` test runtimes failed to
allocate memory before executing assertions. No assertion failed. After all Go
tool processes exited, free memory recovered to approximately 9.10 GiB. The
identical unmodified command then passed every planned package. This was a
local resource exhaustion event, not a code failure; it is recorded here rather
than hidden.

Local host evidence after verification:

- CPU: AMD Ryzen 5 4600H, 12 logical processors;
- visible RAM: approximately 15.42 GiB;
- free RAM at the final evidence sample: approximately 5.76 GiB;
- local preview: `http://127.0.0.1:13000`, PID 14220.

## Scope Verification

Before this audit document, `git diff --name-only 95fc952e8..HEAD` contained
exactly these seven migration-preparation files:

```text
.dockerignore
docs/superpowers/plans/2026-07-28-production-cross-server-migration.md
docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md
scripts/production/rehearse_cross_server_migration.ps1
scripts/production/tests/test_cross_server_rehearsal.ps1
scripts/production/tests/test_yucore_migration_guard.py
scripts/production/yucore_migration_guard.py
```

This audit document is the eighth expected file. The existing untracked
`.superpowers/` directory remains untouched. Generated Python caches were
removed.

Current-tree secret hygiene checks passed:

- known legacy OLD SSH credential matches: 0;
- generic long `sk-` credential matches: 0;
- committed Cloudflare API token assignment matches: 0;
- Cloudflare runbook commands keep the token out of process arguments and use
  mode-0600 temporary curl configuration with trapped cleanup.

The historical credential condition described in Evidence Boundary remains a
hard no-push gate despite the clean current tree.

## Production Snapshot Reference

No production connection was made during this final audit. The following
values come from the previously committed read-only audit snapshot and must be
revalidated immediately before maintenance:

- MySQL 8.4 application data: approximately 1.35 GB;
- `logs` table: approximately 1.33 GB and about 805,000 rows;
- compressed full database backup: approximately 59 MB;
- scheduled logical backup duration: about six seconds;
- Redis persistence: approximately 57 MB and about 2,200 keys;
- `/opt/newapi/data`: approximately 1.96 GB, almost entirely historical logs;
- Caddy certificate/configuration storage: below 1 MB;
- current immutable application image: approximately 78 MB;
- `yuaiapi.com`, `api.yuaiapi.com`, and `global.yuaiapi.com`: Cloudflare
  proxied;
- `vip.yuaiapi.com`: DNS-only with a 300-second TTL.

These are planning evidence, not permission to execute and not a substitute
for fresh preflight manifests.

## Cloudflare Cutover Boundary

No Cloudflare change is required before the new server passes private
acceptance. During the approved maintenance window, only the `content` field of
these four A records may change from OLD IPv4 to NEW IPv4:

| Record | Proxy state to preserve | TTL to preserve |
|---|---|---|
| `yuaiapi.com` | Proxied | Existing/automatic |
| `api.yuaiapi.com` | Proxied | Existing/automatic |
| `global.yuaiapi.com` | Proxied | Existing/automatic |
| `vip.yuaiapi.com` | DNS-only | 300 seconds |

MX, SPF, DKIM, DMARC, SSL/TLS mode, WAF, rulesets, cache behavior, Turnstile,
redirects, and unrelated DNS records must remain unchanged. Origin Rules and
Load Balancer pools require modification only if a fresh snapshot proves they
contain the OLD IP.

The required order is:

1. Validate NEW privately with public traffic blocked.
2. Activate and record the OLD -> NEW write-authority bridge.
3. Apply the reviewed provider-firewall transition, then host firewall rules.
4. Pass direct NEW and OLD-bridge probes.
5. Snapshot, plan, review, and update exactly four A records.
6. Observe for at least two VIP TTL periods before bridge/firewall cleanup.

## Remaining Execution-Time Inputs

The following inputs are mandatory before production execution:

- new server IPv4 and optional IPv6 addresses;
- new SSH port, user, and temporary access method;
- verified CPU, memory, NVMe model/health, RAID layout, filesystem, OS, clock,
  and network evidence from the purchased host;
- reviewed primary IPv4 and explicit IPv6 enabled/disabled policy;
- independently approved exact OLD and NEW Redis `/data` mount sources;
- provider-firewall rule evidence for staging, public cutover, and rollback;
- Cloudflare Zone ID;
- a temporary least-privilege Cloudflare Zone DNS Read/Edit token and
  two-person review of the generated four-record plan;
- disposable anonymous, user, admin, downstream relay, registration email,
  image, and video probe credentials;
- the exact authorized private-group identifier;
- the exact public mapped-model identifier;
- the selected minimum-cost ordinary chat, image, and video model identifiers;
- maintenance start time;
- explicit final production authorization after every fresh preflight passes;
- rotated legacy OLD SSH credential and a clean non-secret Git history;
- final clean `ACCEPTED_COMMIT`, immutable image ID, image archive SHA-256, and
  transferred artifact SHA-256 evidence.

Receiving server credentials alone does not authorize migration. The mutation
gate in the runbook must still be satisfied with a separate explicit approval.

## Final State

- Local migration guard: verified.
- Local disposable rehearsal: verified and repeatable.
- Backend and frontend candidate: verified.
- Production runbook: specification and quality review passed.
- Current tracked tree secret scan: clean.
- Current branch history: not safe to push until credential rotation and clean
  history preparation are complete.
- Production state: unchanged.
- Cloudflare state: unchanged.
- Push/deployment state: none.
