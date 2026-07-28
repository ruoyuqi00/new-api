# Production Cross-Server Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and locally verify a secret-safe migration kit that moves the accepted YuCore production candidate to a new server with pre-staging, a short forced maintenance window, preserved Cloudflare behavior, exact data validation, and lossless rollback.

**Architecture:** A standard-library Python guard owns confirmation, JSON status validation, Docker/Compose drift checks, manifest comparison, and Cloudflare DNS planning. PowerShell local rehearsals use disposable Docker resources to prove MySQL/Redis transfer and Caddy maintenance/forward/rollback behavior. The production runbook composes those verified primitives but cannot mutate either server without the new-host coordinates, accepted image digest, fresh backups, and the exact maintenance confirmation phrase.

**Tech Stack:** Python 3 standard library and `unittest`, PowerShell 7/Windows PowerShell, Docker Engine/Compose, MySQL 8.4, Redis 7, Caddy 2, Nginx 1.27, Bash/coreutils, Cloudflare v4 API

---

## File Map

- Create `scripts/production/yucore_migration_guard.py`: pure validation functions and a secret-safe CLI used on old and new servers.
- Create `scripts/production/tests/test_yucore_migration_guard.py`: deterministic unit contracts for confirmation, status, drift, manifests, and Cloudflare changes.
- Create `scripts/production/rehearse_cross_server_migration.ps1`: disposable local MySQL, Redis, Nginx, and Caddy rehearsal with verified cleanup.
- Create `scripts/production/tests/test_cross_server_rehearsal.ps1`: behavior test that executes the rehearsal and validates its machine-readable result.
- Modify `.dockerignore`: keep migration tooling out of the application image build context.
- Create `docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md`: exact old-host, new-host, cutover, Cloudflare, validation, rollback, and cleanup commands.
- Create `docs/superpowers/acceptance/2026-07-28-cross-server-migration-preparation-audit.md`: final evidence and remaining execution-time inputs.

## Task 1: Confirmation, Status, And Manifest Guard

**Files:**
- Create: `scripts/production/yucore_migration_guard.py`
- Create: `scripts/production/tests/test_yucore_migration_guard.py`

- [ ] **Step 1: Write the failing guard contracts**

Create `scripts/production/tests/test_yucore_migration_guard.py` with imports and these first contracts:

```python
import hashlib
import json
import pathlib
import sys
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SCRIPT_DIR))

import yucore_migration_guard as guard


class ConfirmationTests(unittest.TestCase):
    def test_requires_new_host_and_exact_confirmation(self):
        with self.assertRaisesRegex(guard.GuardError, "new host"):
            guard.require_maintenance_confirmation("", guard.MAINTENANCE_CONFIRMATION)
        with self.assertRaisesRegex(guard.GuardError, "confirmation"):
            guard.require_maintenance_confirmation("203.0.113.10", "yes")

        guard.require_maintenance_confirmation(
            "203.0.113.10", guard.MAINTENANCE_CONFIRMATION
        )


class StatusTests(unittest.TestCase):
    def test_accepts_only_successful_status_document(self):
        parsed = guard.validate_status_document('{"success":true,"data":{"system_name":"YUapi"}}')
        self.assertEqual("YUapi", parsed["data"]["system_name"])

        for payload in ("not json", "{}", '{"success":false}', '{"success":true,"data":{}}'):
            with self.subTest(payload=payload), self.assertRaises(guard.GuardError):
                guard.validate_status_document(payload)


class ManifestTests(unittest.TestCase):
    def test_canonical_manifest_digest_is_order_independent(self):
        left = {"tables": {"users": 45, "tokens": 145}, "redis": 2179}
        right = {"redis": 2179, "tables": {"tokens": 145, "users": 45}}
        self.assertEqual(guard.manifest_digest(left), guard.manifest_digest(right))

        expected = hashlib.sha256(
            json.dumps(left, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode()
        ).hexdigest()
        self.assertEqual(expected, guard.manifest_digest(left))

    def test_manifest_comparison_names_mismatched_sections(self):
        drift = guard.compare_manifests(
            {"tables": {"users": 45}, "redis": 2179},
            {"tables": {"users": 44}, "redis": 2179},
        )
        self.assertEqual(["tables"], drift)
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
python -m unittest discover -s scripts/production/tests -p 'test_yucore_migration_guard.py' -v
```

Expected: FAIL with `ModuleNotFoundError: No module named 'yucore_migration_guard'`.

- [ ] **Step 3: Implement the minimal guard core and CLI**

Create `scripts/production/yucore_migration_guard.py` with this public surface:

```python
#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import sys
from typing import Any

MAINTENANCE_CONFIRMATION = "MIGRATE-YUCORE-PRODUCTION"


class GuardError(RuntimeError):
    pass


def require_maintenance_confirmation(new_host: str, confirmation: str) -> None:
    if not new_host.strip():
        raise GuardError("new host is required")
    if confirmation != MAINTENANCE_CONFIRMATION:
        raise GuardError("maintenance confirmation does not match")


def validate_status_document(payload: str) -> dict[str, Any]:
    try:
        document = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise GuardError(f"status response is not JSON: {exc.msg}") from exc
    if not isinstance(document, dict) or document.get("success") is not True:
        raise GuardError("status response is not successful")
    data = document.get("data")
    if not isinstance(data, dict) or not str(data.get("system_name", "")).strip():
        raise GuardError("status response has no system name")
    return document


def manifest_digest(document: Any) -> str:
    canonical = json.dumps(
        document, ensure_ascii=True, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    return hashlib.sha256(canonical).hexdigest()


def compare_manifests(source: dict[str, Any], target: dict[str, Any]) -> list[str]:
    sections = sorted(set(source) | set(target))
    return [name for name in sections if source.get(name) != target.get(name)]


def _read_json(path: str) -> dict[str, Any]:
    document = json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
    if not isinstance(document, dict):
        raise GuardError(f"{path} must contain a JSON object")
    return document


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    confirm = subparsers.add_parser("confirm")
    confirm.add_argument("--new-host", required=True)
    confirm.add_argument("--confirmation", required=True)

    subparsers.add_parser("validate-status")

    manifests = subparsers.add_parser("compare-manifests")
    manifests.add_argument("--source", required=True)
    manifests.add_argument("--target", required=True)

    args = parser.parse_args(argv)
    try:
        if args.command == "confirm":
            require_maintenance_confirmation(args.new_host, args.confirmation)
            print(json.dumps({"ok": True, "new_host": args.new_host}))
        elif args.command == "validate-status":
            document = validate_status_document(sys.stdin.read())
            print(json.dumps({"ok": True, "system_name": document["data"]["system_name"]}))
        elif args.command == "compare-manifests":
            drift = compare_manifests(_read_json(args.source), _read_json(args.target))
            if drift:
                raise GuardError("manifest drift: " + ",".join(drift))
            print(json.dumps({"ok": True, "digest": manifest_digest(_read_json(args.source))}))
    except (GuardError, OSError, json.JSONDecodeError) as exc:
        print(json.dumps({"ok": False, "error": str(exc)}), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] **Step 4: Run guard tests and CLI smoke checks**

Run:

```powershell
python -m unittest discover -s scripts/production/tests -p 'test_yucore_migration_guard.py' -v
'{"success":true,"data":{"system_name":"YUapi"}}' | python scripts/production/yucore_migration_guard.py validate-status
python scripts/production/yucore_migration_guard.py confirm --new-host 203.0.113.10 --confirmation wrong
```

Expected: unit tests PASS; status command returns `"ok": true`; wrong confirmation exits nonzero without printing a secret.

- [ ] **Step 5: Commit the guard core**

```powershell
git add scripts/production/yucore_migration_guard.py scripts/production/tests/test_yucore_migration_guard.py
git commit -m "feat: add production migration safety guard"
```

## Task 2: Secret-Safe Docker And Compose Drift Guard

**Files:**
- Modify: `scripts/production/yucore_migration_guard.py`
- Modify: `scripts/production/tests/test_yucore_migration_guard.py`

- [ ] **Step 1: Add failing runtime drift tests**

Append fixtures and contracts that prove matching runtime state passes, changed values report only field names, and secret values never appear:

```python
class RuntimeDriftTests(unittest.TestCase):
    def setUp(self):
        self.live = {
            "Config": {
                "Image": "newapi:old",
                "Env": ["PATH=/usr/bin", "SESSION_SECRET=live-secret", "PORT=3000"],
                "Healthcheck": {"Test": ["CMD", "wget", "http://localhost:3000/api/status"]},
            },
            "HostConfig": {
                "RestartPolicy": {"Name": "unless-stopped"},
                "Ulimits": [{"Name": "nofile", "Soft": 65535, "Hard": 65535}],
                "PortBindings": {"3000/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3001"}]},
            },
            "Mounts": [{"Type": "bind", "Source": "/opt/newapi/data", "Destination": "/data", "RW": True}],
            "NetworkSettings": {"Networks": {"sub2api_sub2api-network": {}}},
        }
        self.compose = {
            "services": {
                "newapi": {
                    "image": "newapi:old",
                    "environment": {"SESSION_SECRET": "live-secret", "PORT": "3000"},
                    "restart": "unless-stopped",
                    "volumes": [{"type": "bind", "source": "/opt/newapi/data", "target": "/data", "read_only": False}],
                    "networks": {"shared": None},
                    "ports": [{"target": 3000, "published": "3001", "host_ip": "127.0.0.1"}],
                    "ulimits": {"nofile": {"soft": 65535, "hard": 65535}},
                    "healthcheck": {"test": ["CMD", "wget", "http://localhost:3000/api/status"]},
                }
            },
            "networks": {"shared": {"name": "sub2api_sub2api-network"}},
        }

    def test_matching_runtime_has_no_drift(self):
        self.assertEqual([], guard.runtime_drift(self.live, self.compose, "newapi"))

    def test_secret_value_mismatch_reports_key_only(self):
        self.compose["services"]["newapi"]["environment"]["SESSION_SECRET"] = "target-secret"
        drift = guard.runtime_drift(self.live, self.compose, "newapi")
        rendered = json.dumps(drift)
        self.assertIn("environment:SESSION_SECRET", drift)
        self.assertNotIn("live-secret", rendered)
        self.assertNotIn("target-secret", rendered)

    def test_mount_network_and_ulimit_drift_are_blocking(self):
        self.compose["services"]["newapi"]["volumes"][0]["target"] = "/wrong"
        self.compose["networks"]["shared"]["name"] = "wrong-network"
        self.compose["services"]["newapi"]["ulimits"]["nofile"]["hard"] = 1024
        self.assertEqual(
            ["mounts", "networks", "ulimits"],
            guard.runtime_drift(self.live, self.compose, "newapi"),
        )
```

- [ ] **Step 2: Run the runtime tests and verify RED**

Run the unittest command from Task 1.

Expected: FAIL because `runtime_drift` is missing.

- [ ] **Step 3: Implement normalized, secret-safe drift comparison**

Add `runtime_drift(live, compose, service_name)` plus private normalizers. Compare image, explicit Compose environment keys, mounts, resolved network names, host bindings, restart policy, health check, and `nofile`. Ignore image-provided environment keys such as `PATH` when Compose does not override them. Return only stable drift labels; never return values.

Add a `runtime-preflight` CLI subcommand that invokes these commands in memory:

```text
docker inspect <container>
docker compose -f <compose-file> config --format json
```

Its only successful output is:

```json
{"ok":true,"container":"newapi","service":"newapi"}
```

Its failure output contains only drift labels such as `environment:SESSION_SECRET`, never serialized inspect or Compose documents.

- [ ] **Step 4: Verify runtime drift and secret masking**

Run the full guard unittest file. Then run the CLI against a local fixture container or mocked fixture command and search captured output for `live-secret` and `target-secret`.

Expected: all tests PASS; both secret searches return no match.

- [ ] **Step 5: Commit the runtime drift guard**

```powershell
git add scripts/production/yucore_migration_guard.py scripts/production/tests/test_yucore_migration_guard.py
git commit -m "feat: block migration on production topology drift"
```

## Task 3: Cloudflare DNS Change Planner

**Files:**
- Modify: `scripts/production/yucore_migration_guard.py`
- Modify: `scripts/production/tests/test_yucore_migration_guard.py`

- [ ] **Step 1: Write failing Cloudflare planning tests**

Append:

```python
class CloudflarePlanTests(unittest.TestCase):
    def setUp(self):
        self.records = [
            {"id": "1", "type": "A", "name": "yuaiapi.com", "content": "192.0.2.10", "proxied": True, "ttl": 1},
            {"id": "2", "type": "A", "name": "api.yuaiapi.com", "content": "192.0.2.10", "proxied": True, "ttl": 1},
            {"id": "3", "type": "A", "name": "global.yuaiapi.com", "content": "192.0.2.10", "proxied": True, "ttl": 1},
            {"id": "4", "type": "A", "name": "vip.yuaiapi.com", "content": "192.0.2.10", "proxied": False, "ttl": 300},
            {"id": "5", "type": "MX", "name": "yuaiapi.com", "content": "mail.example", "priority": 10},
        ]

    def test_changes_only_approved_a_record_content(self):
        plan = guard.plan_cloudflare_records(self.records, "203.0.113.10")
        self.assertEqual(4, len(plan))
        self.assertTrue(all(change["content"] == "203.0.113.10" for change in plan))
        self.assertEqual([True, True, True, False], [change["proxied"] for change in plan])
        self.assertEqual([1, 1, 1, 300], [change["ttl"] for change in plan])

    def test_rejects_missing_duplicate_or_unapproved_records(self):
        with self.assertRaisesRegex(guard.GuardError, "missing"):
            guard.plan_cloudflare_records(self.records[:-2], "203.0.113.10")
        with self.assertRaisesRegex(guard.GuardError, "duplicate"):
            guard.plan_cloudflare_records(self.records + [dict(self.records[0], id="6")], "203.0.113.10")
        with self.assertRaisesRegex(guard.GuardError, "IPv4"):
            guard.plan_cloudflare_records(self.records, "not-an-ip")
```

- [ ] **Step 2: Run and verify RED**

Run the guard unittest command.

Expected: FAIL because `plan_cloudflare_records` is missing.

- [ ] **Step 3: Implement the exact-record planner**

Add:

```python
import ipaddress

PRODUCTION_A_RECORDS = (
    "yuaiapi.com",
    "api.yuaiapi.com",
    "global.yuaiapi.com",
    "vip.yuaiapi.com",
)


def plan_cloudflare_records(records: list[dict[str, Any]], new_ip: str) -> list[dict[str, Any]]:
    try:
        parsed = ipaddress.ip_address(new_ip)
    except ValueError as exc:
        raise GuardError("new origin must be a valid IPv4 address") from exc
    if parsed.version != 4:
        raise GuardError("new origin must be a valid IPv4 address")

    changes = []
    for name in PRODUCTION_A_RECORDS:
        matches = [record for record in records if record.get("type") == "A" and record.get("name") == name]
        if not matches:
            raise GuardError(f"missing production A record: {name}")
        if len(matches) != 1:
            raise GuardError(f"duplicate production A record: {name}")
        record = matches[0]
        changes.append(
            {
                "id": record["id"],
                "type": "A",
                "name": name,
                "content": str(parsed),
                "proxied": bool(record.get("proxied")),
                "ttl": int(record.get("ttl", 1)),
            }
        )
    return changes
```

Add `cloudflare-plan --records <secure-snapshot.json> --new-ip <ip>` to print the four-record plan. It must not accept a token or call the API. The runbook obtains and stores the timestamped snapshot separately under mode `0700` and applies the reviewed plan only after the maintenance confirmation gate.

- [ ] **Step 4: Verify Cloudflare preservation contracts**

Run the full guard tests.

Expected: all tests PASS; the MX fixture is absent from the plan; proxy flags and TTLs remain unchanged.

- [ ] **Step 5: Commit the Cloudflare planner**

```powershell
git add scripts/production/yucore_migration_guard.py scripts/production/tests/test_yucore_migration_guard.py
git commit -m "feat: constrain production origin DNS changes"
```

## Task 4: Disposable Cross-Server Data And Proxy Rehearsal

**Files:**
- Create: `scripts/production/rehearse_cross_server_migration.ps1`
- Create: `scripts/production/tests/test_cross_server_rehearsal.ps1`
- Modify: `.dockerignore`

- [ ] **Step 1: Write the failing rehearsal behavior test**

Create `scripts/production/tests/test_cross_server_rehearsal.ps1`:

```powershell
$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot '..\rehearse_cross_server_migration.ps1'
$output = & $script -Json
if ($LASTEXITCODE -ne 0) { throw "rehearsal failed with exit $LASTEXITCODE" }
$result = $output | ConvertFrom-Json
if (-not $result.mysql_forward_equal) { throw 'MySQL forward manifest mismatch' }
if (-not $result.mysql_rollback_equal) { throw 'MySQL rollback manifest mismatch' }
if (-not $result.redis_forward_equal) { throw 'Redis forward key mismatch' }
if (-not $result.maintenance_status_503) { throw 'Maintenance response was not 503' }
if (-not $result.maintenance_retry_after) { throw 'Maintenance response has no Retry-After' }
if (-not $result.forward_marker_new) { throw 'Forward proxy did not reach new marker' }
if (-not $result.rollback_marker_old) { throw 'Rollback proxy did not reach old marker' }
if (-not $result.cleanup_complete) { throw 'Disposable resources remain' }
```

- [ ] **Step 2: Run the behavior test and verify RED**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/production/tests/test_cross_server_rehearsal.ps1
```

Expected: FAIL because `rehearse_cross_server_migration.ps1` does not exist.

- [ ] **Step 3: Implement the disposable rehearsal**

Create `scripts/production/rehearse_cross_server_migration.ps1` with `[CmdletBinding()] param([switch]$Json)`. It must:

1. Verify Docker is available and the fixed rehearsal network and container names do not exist.
2. Create one isolated Docker network.
3. Start source and target `mysql:8.4` containers with generated rehearsal-only passwords, health checks, and no published database ports.
4. Create deterministic `users`, `tokens`, and `logs` fixtures in source MySQL.
5. Dump source through `docker exec` with `MYSQL_PWD`, restore target, and compare exact counts plus maximum IDs.
6. Insert a simulated post-cutover row into target, dump target, restore source, and prove the rollback manifest matches.
7. Start source and target `redis:7-alpine`, create string, hash, expiring, affinity, and cooldown fixtures, force `SAVE`, restore the target persistence artifact, and compare key-prefix counts plus representative TTL ranges.
8. Start old and new `nginx:1.27-alpine` marker containers plus a maintenance Nginx container returning 503 and `Retry-After: 180`.
9. Start `caddy:2-alpine` on loopback port `18080`; atomically reload configs for old, maintenance, new, and old rollback states and assert marker/status results.
10. In `finally`, verify every resolved container/network path belongs to the fixed rehearsal prefix, remove only those resources, remove the generated temp directory, and prove cleanup.
11. Emit only the JSON fields asserted by the behavior test when `-Json` is present.

Use condition polling for MySQL, Redis, Nginx, and Caddy readiness. Do not use fixed sleeps longer than one second and do not prune Docker globally.

- [ ] **Step 4: Exclude migration tooling from the application build context**

Add this exact line to `.dockerignore`:

```text
scripts/production/
```

Run `git diff --check` and confirm the line appears once.

- [ ] **Step 5: Run the complete local rehearsal twice**

Run the PowerShell behavior test twice consecutively.

Expected: both runs PASS, proving cleanup makes the rehearsal repeatable. `docker ps -a` and `docker network ls` contain no `yucore-migration-rehearsal` resources afterward.

- [ ] **Step 6: Commit the rehearsal**

```powershell
git add .dockerignore scripts/production/rehearse_cross_server_migration.ps1 scripts/production/tests/test_cross_server_rehearsal.ps1
git commit -m "test: rehearse production server migration locally"
```

## Task 5: Exact Cross-Server Production Runbook

**Files:**
- Create: `docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md`
- Reference: `docs/superpowers/specs/2026-07-28-production-cross-server-migration-design.md`
- Reference: `scripts/production/yucore_migration_guard.py`

- [ ] **Step 1: Write the scope, variables, and mutation gate**

The runbook must start by stating that all commands are inert until the user authorizes production execution. Define variables without embedding credentials:

```bash
export OLD_HOST='156.239.252.210'
export OLD_SSH_PORT='46276'
export OLD_USER='root'
export NEW_HOST="${NEW_HOST:?set NEW_HOST}"
export NEW_SSH_PORT="${NEW_SSH_PORT:?set NEW_SSH_PORT}"
export NEW_USER="${NEW_USER:?set NEW_USER}"
export ACCEPTED_COMMIT="${ACCEPTED_COMMIT:?set ACCEPTED_COMMIT}"
export ACCEPTED_IMAGE="newapi:yucore-$ACCEPTED_COMMIT"
export MAINTENANCE_CONFIRMATION="${MAINTENANCE_CONFIRMATION:-}"
python3 /opt/newapi/releases/yucore_migration_guard.py confirm \
  --new-host "$NEW_HOST" \
  --confirmation "$MAINTENANCE_CONFIRMATION"
```

The default path before maintenance omits `MAINTENANCE_CONFIRMATION`, so the guard fails closed.

- [ ] **Step 2: Add old-server and new-server preflight commands**

Include exact read-only commands for CPU/memory/NVMe/RAID, OS/time, Docker/Compose, firewall/listeners, DNS, image IDs, container state, mounts, environment keys, secret-safe Compose drift, database version/schema/size, Redis persistence, filesystem sizes, backup timers, Caddy validation, and current public endpoint status.

The old source container names are explicit: `newapi`, `newapi-mysql`, `newapi-redis`, and `yuapi-caddy`. Never select the first MySQL image on the host.

- [ ] **Step 3: Add immutable image and deployment transfer**

Use `git archive` from the accepted commit, build `linux/amd64`, save the image, record image ID/commit/SHA-256, and transfer the image plus guard. Transfer the live Compose/env/Caddy deployment directly old-to-new or through a mode-0700 staging directory without printing values. Validate all hashes after transfer.

- [ ] **Step 4: Add pre-staging and private acceptance**

Document the initial MySQL logical dump/restore, Redis persistence copy, application/Caddy/static-brand pre-sync, backup-unit installation, and new-host candidate startup with public ports blocked. Use `curl --resolve` and the guard's `validate-status` subcommand. Include the full anonymous/user/admin and relay acceptance checklist from the design.

- [ ] **Step 5: Add maintenance freeze and final transfer**

Provide exact maintenance Nginx creation, Caddy upstream replacement, Caddy validation/reload, five-second forced application stop, final MySQL dump, Redis `SAVE`/stop/archive, final file delta, direct SSH transfer, checksums, and source manifest capture. MySQL password must remain inside the container environment via `MYSQL_PWD`.

- [ ] **Step 6: Add final restore, master start, and validation**

Document target database replacement, Redis restore, final file application, final master startup, health polling, zero-restart/image checks, row/ID/option/Redis/file manifest comparison, private host probes, and paid minimum probes. Public traffic remains blocked until every gate passes.

- [ ] **Step 7: Add old-origin bridge and Cloudflare change**

Document secure Cloudflare snapshot collection using a temporary least-privilege `CF_API_TOKEN`, `cloudflare-plan`, human-readable review, four exact A-record updates, and after-state comparison. Preserve proxy flags and TTLs. Configure the old Caddy to forward stale origin/VIP traffic to the new origin before changing records.

- [ ] **Step 8: Add both rollback paths**

Provide exact pre-traffic rollback to the frozen old database and post-traffic reverse migration from new to old. Post-traffic rollback must re-enter maintenance, stop new writes, dump new MySQL/Redis, restore old, compare manifests, start the old immutable image, restore old Caddy, and revert only the four changed records.

- [ ] **Step 9: Add observation and delayed cleanup**

Include 1/5/15/30/60-minute samples, OOM/kernel checks, resource and HTTP monitoring, routing/account/billing validation, daily backup verification, seven-day old-server retention, and explicit confirmation before deleting any old resource or revoking temporary access.

- [ ] **Step 10: Review the runbook against the design**

Verify every design heading has an executable runbook section. Run:

```powershell
$forbidden = @('T' + 'BD', 'T' + 'ODO', 'CHANGE' + '_ME', 'password=', ('CF_' + 'API_TOKEN=.+'))
Select-String -Path docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md -Pattern $forbidden
git diff --check
```

Expected: no placeholder or embedded-secret match; `git diff --check` passes.

- [ ] **Step 11: Commit the runbook**

```powershell
git add docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md
git commit -m "docs: add production server migration runbook"
```

## Task 6: Full Local Verification And Preparation Audit

**Files:**
- Create: `docs/superpowers/acceptance/2026-07-28-cross-server-migration-preparation-audit.md`

- [ ] **Step 1: Run migration-tool tests**

```powershell
python -m unittest discover -s scripts/production/tests -p 'test_yucore_migration_guard.py' -v
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/production/tests/test_cross_server_rehearsal.ps1
```

Expected: all Python tests PASS and the full disposable rehearsal PASSes.

- [ ] **Step 2: Re-run the accepted application preflight serially**

```powershell
go test ./middleware ./service ./model ./controller ./relay/... ./pkg/billingexpr ./setting/... -count=1
go build ./...
Set-Location web/default
bun test
bun run typecheck
bun run build
Set-Location ../classic
bun run build
Set-Location ../..
git diff --check
```

Expected: backend tests/build PASS, default reports 131 or more passing tests with zero failures, typecheck/build PASS, classic build PASS, and diff check PASS. Keep heavy commands serial to avoid the previously observed memory OOM.

- [ ] **Step 3: Verify scope and secret hygiene**

Run:

```powershell
git diff --name-only 95fc952e8..HEAD
$knownSecret = $env:YUCORE_LEGACY_OLD_SSH_PASSWORD
if ([string]::IsNullOrEmpty($knownSecret)) { throw 'known-secret scan input is required' }
$knownSecretMatches = @($knownSecret | git grep -l -I -F -f - -- . ':!docs/archive/**')
$knownSecretScanExit = $LASTEXITCODE
Remove-Item Env:YUCORE_LEGACY_OLD_SSH_PASSWORD
Remove-Variable knownSecret
if ($knownSecretScanExit -eq 0) {
  throw "known secret remains in tracked current-tree files: $($knownSecretMatches -join ', ')"
}
if ($knownSecretScanExit -ne 1) { throw "known-secret scan failed with exit $knownSecretScanExit" }
$genericPatterns = @(
  ('s' + 'k-[A-Za-z0-9]{16,}'),
  ('CF_' + 'API_TOKEN=.+')
)
$genericMatches = @(git grep -l -I -E ($genericPatterns -join '|') -- . ':!docs/archive/**')
$genericSecretScanExit = $LASTEXITCODE
if ($genericSecretScanExit -eq 0) {
  throw "generic secret pattern remains in tracked current-tree files: $($genericMatches -join ', ')"
}
if ($genericSecretScanExit -ne 1) { throw "generic secret scan failed with exit $genericSecretScanExit" }
git status --short
```

Expected: only migration tooling, runbook, `.dockerignore`, and acceptance evidence changed; secret grep returns no match; only the existing untracked `.superpowers/` remains outside committed scope.

- [ ] **Step 4: Record exact evidence and execution-time inputs**

Create the acceptance document with command outputs, test counts, Docker rehearsal result, current data-size/backup-duration snapshot, accepted commit/image boundary, and these remaining inputs:

- new server IPs, SSH port/user, and temporary access;
- verified CPU/memory/NVMe/RAID/network evidence;
- selected primary IPv4 and optional IPv6 policy;
- temporary least-privilege Cloudflare token or explicit manual-DNS choice;
- maintenance start time and final production authorization;
- disposable downstream/user/admin probe credentials.

State explicitly that no production mutation occurred during preparation.

- [ ] **Step 5: Commit the preparation audit**

```powershell
git add docs/superpowers/acceptance/2026-07-28-cross-server-migration-preparation-audit.md
git commit -m "docs: verify cross-server migration preparation"
```

- [ ] **Step 6: Final branch integrity check**

```powershell
git status --short --branch
git log --oneline -10
git diff --check
```

Expected: tracked worktree clean, branch remains `codex/local-production-brand-performance-20260725`, and no push or production deployment has occurred.
