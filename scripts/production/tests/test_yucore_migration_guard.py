import hashlib
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from io import StringIO
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).resolve().parents[1] / "yucore_migration_guard.py"


def load_guard_module(test_case):
    if not MODULE_PATH.is_file():
        test_case.fail(f"migration guard module is missing: {MODULE_PATH}")

    spec = importlib.util.spec_from_file_location("yucore_migration_guard", MODULE_PATH)
    if spec is None or spec.loader is None:
        test_case.fail(f"cannot load migration guard module: {MODULE_PATH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def runtime_documents():
    live = {
        "Config": {
            "Image": "newapi:old",
            "Env": [
                "PATH=/usr/bin",
                "SESSION_SECRET=live-secret",
                "PORT=3000",
            ],
            "Healthcheck": {
                "Test": [
                    "CMD",
                    "wget",
                    "http://localhost:3000/api/status",
                ]
            },
        },
        "HostConfig": {
            "RestartPolicy": {"Name": "unless-stopped"},
            "Ulimits": [
                {"Name": "nofile", "Soft": 65535, "Hard": 65535}
            ],
            "PortBindings": {
                "3000/tcp": [
                    {"HostIp": "127.0.0.1", "HostPort": "3001"}
                ]
            },
        },
        "Mounts": [
            {
                "Type": "bind",
                "Source": "/opt/newapi/data",
                "Destination": "/data",
                "RW": True,
            }
        ],
        "NetworkSettings": {
            "Networks": {"sub2api_sub2api-network": {}}
        },
    }
    compose = {
        "services": {
            "newapi": {
                "image": "newapi:old",
                "environment": {
                    "SESSION_SECRET": "live-secret",
                    "PORT": "3000",
                },
                "restart": "unless-stopped",
                "volumes": [
                    {
                        "type": "bind",
                        "source": "/opt/newapi/data",
                        "target": "/data",
                        "read_only": False,
                    }
                ],
                "networks": {"shared": None},
                "ports": [
                    {
                        "target": 3000,
                        "published": "3001",
                        "host_ip": "127.0.0.1",
                    }
                ],
                "ulimits": {"nofile": {"soft": 65535, "hard": 65535}},
                "healthcheck": {
                    "test": [
                        "CMD",
                        "wget",
                        "http://localhost:3000/api/status",
                    ]
                },
            }
        },
        "networks": {"shared": {"name": "sub2api_sub2api-network"}},
    }
    return live, compose


def cloudflare_records():
    return [
        {
            "id": "1",
            "type": "A",
            "name": "yuaiapi.com",
            "content": "192.0.2.10",
            "proxied": True,
            "ttl": 1,
        },
        {
            "id": "2",
            "type": "A",
            "name": "api.yuaiapi.com",
            "content": "192.0.2.10",
            "proxied": True,
            "ttl": 1,
        },
        {
            "id": "3",
            "type": "A",
            "name": "global.yuaiapi.com",
            "content": "192.0.2.10",
            "proxied": True,
            "ttl": 1,
        },
        {
            "id": "4",
            "type": "A",
            "name": "vip.yuaiapi.com",
            "content": "192.0.2.10",
            "proxied": False,
            "ttl": 300,
        },
        {
            "id": "5",
            "type": "MX",
            "name": "yuaiapi.com",
            "content": "mail.example",
            "priority": 10,
        },
        {
            "id": "6",
            "type": "A",
            "name": "unapproved.yuaiapi.com",
            "content": "sensitive-unapproved-content",
            "proxied": False,
            "ttl": 120,
        },
    ]


class MigrationGuardTests(unittest.TestCase):
    @property
    def guard(self):
        if not hasattr(self, "_guard"):
            self._guard = load_guard_module(self)
        return self._guard

    def test_confirmation_requires_a_new_host(self):
        with self.assertRaisesRegex(self.guard.GuardError, "new host"):
            self.guard.require_maintenance_confirmation(
                "", self.guard.MAINTENANCE_CONFIRMATION
            )

    def test_confirmation_requires_the_exact_phrase(self):
        with self.assertRaisesRegex(self.guard.GuardError, "confirmation"):
            self.guard.require_maintenance_confirmation("new.example", "wrong")

        with self.assertRaisesRegex(self.guard.GuardError, "confirmation"):
            self.guard.require_maintenance_confirmation(
                "new.example", self.guard.MAINTENANCE_CONFIRMATION + " "
            )

    def test_confirmation_accepts_a_new_host_and_exact_phrase(self):
        self.guard.require_maintenance_confirmation(
            "new.example", self.guard.MAINTENANCE_CONFIRMATION
        )

    def test_status_document_accepts_success_with_system_name(self):
        payload = '{"success":true,"data":{"system_name":"YUapi"}}'

        document = self.guard.validate_status_document(payload)

        self.assertEqual(
            {"success": True, "data": {"system_name": "YUapi"}}, document
        )

    def test_status_document_rejects_invalid_json(self):
        with self.assertRaises(self.guard.GuardError):
            self.guard.validate_status_document("not-json")

    def test_status_document_rejects_empty_object(self):
        with self.assertRaises(self.guard.GuardError):
            self.guard.validate_status_document("{}")

    def test_status_document_rejects_unsuccessful_response(self):
        payload = '{"success":false,"data":{"system_name":"YUapi"}}'

        with self.assertRaises(self.guard.GuardError):
            self.guard.validate_status_document(payload)

    def test_status_document_rejects_missing_system_name(self):
        with self.assertRaises(self.guard.GuardError):
            self.guard.validate_status_document('{"success":true,"data":{}}')

        with self.assertRaises(self.guard.GuardError):
            self.guard.validate_status_document(
                '{"success":true,"data":{"system_name":"   "}}'
            )

    def test_status_document_rejects_non_string_system_name(self):
        for system_name in (None, 1, True, {}, []):
            with self.subTest(system_name=system_name), self.assertRaises(
                self.guard.GuardError
            ):
                self.guard.validate_status_document(
                    json.dumps(
                        {
                            "success": True,
                            "data": {"system_name": system_name},
                        }
                    )
                )

    def test_manifest_digest_uses_canonical_ascii_json(self):
        expected = hashlib.sha256(b'{"a":"\\u96ea","b":2}').hexdigest()

        first = self.guard.manifest_digest({"b": 2, "a": "雪"})
        second = self.guard.manifest_digest({"a": "雪", "b": 2})

        self.assertEqual(expected, first)
        self.assertEqual(first, second)

    def test_compare_manifests_returns_sorted_different_sections(self):
        source = {"z": 1, "same": {"value": True}, "a": 2}
        target = {"z": 3, "same": {"value": True}, "b": 4}

        self.assertEqual(
            ["a", "b", "z"], self.guard.compare_manifests(source, target)
        )

    def test_compare_manifests_distinguishes_null_from_missing_section(self):
        self.assertEqual(
            ["nullable"], self.guard.compare_manifests({"nullable": None}, {})
        )

    def test_compare_manifests_distinguishes_booleans_from_numbers(self):
        for source_value, target_value in ((True, 1), (False, 0)):
            with self.subTest(
                source_value=source_value, target_value=target_value
            ):
                self.assertEqual(
                    ["section"],
                    self.guard.compare_manifests(
                        {"section": source_value}, {"section": target_value}
                    ),
                )

    def test_read_json_accepts_only_objects(self):
        with tempfile.TemporaryDirectory() as directory:
            object_path = Path(directory) / "object.json"
            array_path = Path(directory) / "array.json"
            object_path.write_text('{"tables":{"users":45}}', encoding="utf-8")
            array_path.write_text("[]", encoding="utf-8")

            self.assertEqual(
                {"tables": {"users": 45}}, self.guard._read_json(object_path)
            )
            with self.assertRaisesRegex(self.guard.GuardError, "JSON object"):
                self.guard._read_json(array_path)


class RuntimeDriftTests(unittest.TestCase):
    def setUp(self):
        self.guard = load_guard_module(self)
        self.live, self.compose = runtime_documents()

    def test_matching_runtime_has_no_drift(self):
        self.assertEqual(
            [], self.guard.runtime_drift(self.live, self.compose, "newapi")
        )

    def test_secret_value_mismatch_reports_key_only(self):
        self.compose["services"]["newapi"]["environment"][
            "SESSION_SECRET"
        ] = "target-secret"

        drift = self.guard.runtime_drift(self.live, self.compose, "newapi")
        rendered = json.dumps(drift)

        self.assertIn("environment:SESSION_SECRET", drift)
        self.assertNotIn("live-secret", rendered)
        self.assertNotIn("target-secret", rendered)

    def test_mount_network_and_ulimit_drift_are_blocking(self):
        self.compose["services"]["newapi"]["volumes"][0]["target"] = "/wrong"
        self.compose["networks"]["shared"]["name"] = "wrong-network"
        self.compose["services"]["newapi"]["ulimits"]["nofile"][
            "hard"
        ] = 1024

        self.assertEqual(
            ["mounts", "networks", "ulimits"],
            self.guard.runtime_drift(self.live, self.compose, "newapi"),
        )

    def test_image_ports_restart_and_healthcheck_drift_are_blocking(self):
        service = self.compose["services"]["newapi"]
        service["image"] = "newapi:new"
        service["ports"][0]["published"] = "3002"
        service["restart"] = "always"
        service["healthcheck"]["test"][-1] = "http://localhost:3000/other"

        self.assertEqual(
            ["healthcheck", "image", "ports", "restart"],
            self.guard.runtime_drift(self.live, self.compose, "newapi"),
        )

    def test_healthcheck_durations_use_exact_nanoseconds(self):
        cases = (
            ("4.1s", 4_100_000_000),
            ("1.001ms", 1_001_000),
        )
        for compose_duration, live_nanoseconds in cases:
            with self.subTest(compose_duration=compose_duration):
                self.live, self.compose = runtime_documents()
                self.live["Config"]["Healthcheck"][
                    "Interval"
                ] = live_nanoseconds
                self.compose["services"]["newapi"]["healthcheck"][
                    "interval"
                ] = compose_duration

                self.assertEqual(
                    [],
                    self.guard.runtime_drift(
                        self.live, self.compose, "newapi"
                    ),
                )

    def test_runtime_drift_rejects_invalid_healthcheck_durations(self):
        for duration in (
            float("inf"),
            float("-inf"),
            float("nan"),
            "not-a-duration",
        ):
            with self.subTest(duration=duration):
                self.live, self.compose = runtime_documents()
                self.compose["services"]["newapi"]["healthcheck"][
                    "interval"
                ] = duration

                with self.assertRaises(ValueError):
                    self.guard.runtime_drift(
                        self.live, self.compose, "newapi"
                    )

    def test_named_volume_uses_resolved_compose_name(self):
        self.live["Mounts"].append(
            {
                "Type": "volume",
                "Name": "sub2api_cache",
                "Source": "/var/lib/docker/volumes/sub2api_cache/_data",
                "Destination": "/cache",
                "RW": False,
            }
        )
        self.compose["services"]["newapi"]["volumes"].append(
            {
                "type": "volume",
                "source": "cache",
                "target": "/cache",
                "read_only": True,
            }
        )
        self.compose["volumes"] = {"cache": {"name": "sub2api_cache"}}
        self.assertEqual(
            [], self.guard.runtime_drift(self.live, self.compose, "newapi")
        )

        self.compose["volumes"]["cache"]["name"] = "wrong-volume"
        self.assertEqual(
            ["mounts"],
            self.guard.runtime_drift(self.live, self.compose, "newapi"),
        )


class MigrationGuardCliTests(unittest.TestCase):
    def run_guard(self, *arguments, stdin=None):
        return subprocess.run(
            [sys.executable, str(MODULE_PATH), *arguments],
            input=stdin,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_confirm_emits_json_on_success_and_failure(self):
        accepted = self.run_guard(
            "confirm",
            "--new-host",
            "203.0.113.10",
            "--confirmation",
            "MIGRATE-YUCORE-PRODUCTION",
        )
        self.assertEqual(0, accepted.returncode)
        self.assertEqual(
            {"ok": True, "new_host": "203.0.113.10"}, json.loads(accepted.stdout)
        )
        self.assertEqual("", accepted.stderr)

        rejected = self.run_guard(
            "confirm",
            "--new-host",
            "203.0.113.10",
            "--confirmation",
            "wrong",
        )
        self.assertNotEqual(0, rejected.returncode)
        self.assertEqual("", rejected.stdout)
        self.assertEqual(False, json.loads(rejected.stderr)["ok"])

    def test_parser_failures_emit_generic_json_and_exit_one(self):
        cases = (
            (),
            ("confirm", "--new-host", "203.0.113.10"),
            ("sensitive-invalid-command",),
            ("validate-status", "--unexpected", "sensitive-value"),
        )
        for arguments in cases:
            with self.subTest(arguments=arguments):
                result = self.run_guard(*arguments)

                self.assertEqual(1, result.returncode)
                self.assertEqual("", result.stdout)
                self.assertEqual(
                    {"ok": False, "error": "invalid command line arguments"},
                    json.loads(result.stderr),
                )
                self.assertNotIn("usage:", result.stderr)
                for argument in arguments:
                    self.assertNotIn(argument, result.stderr)

    def test_validate_status_emits_json_on_success_and_failure(self):
        accepted = self.run_guard(
            "validate-status",
            stdin='{"success":true,"data":{"system_name":"YUapi"}}',
        )
        self.assertEqual(0, accepted.returncode)
        self.assertEqual(
            {"ok": True, "system_name": "YUapi"}, json.loads(accepted.stdout)
        )
        self.assertEqual("", accepted.stderr)

        rejected = self.run_guard("validate-status", stdin="not-json")
        self.assertNotEqual(0, rejected.returncode)
        self.assertEqual("", rejected.stdout)
        self.assertEqual(False, json.loads(rejected.stderr)["ok"])

    def test_compare_manifests_emits_digest_or_drift_error(self):
        with tempfile.TemporaryDirectory() as directory:
            source_path = Path(directory) / "source.json"
            target_path = Path(directory) / "target.json"
            source = {"tables": {"users": 45}, "redis": 2179}
            source_path.write_text(json.dumps(source), encoding="utf-8")
            target_path.write_text(
                json.dumps({"redis": 2179, "tables": {"users": 45}}),
                encoding="utf-8",
            )

            equal = self.run_guard(
                "compare-manifests",
                "--source",
                str(source_path),
                "--target",
                str(target_path),
            )
            self.assertEqual(0, equal.returncode)
            self.assertEqual(
                {"ok": True, "digest": load_guard_module(self).manifest_digest(source)},
                json.loads(equal.stdout),
            )
            self.assertEqual("", equal.stderr)

            target_path.write_text('{"tables":{"users":44},"redis":2179}', encoding="utf-8")
            drift = self.run_guard(
                "compare-manifests",
                "--source",
                str(source_path),
                "--target",
                str(target_path),
            )
            self.assertNotEqual(0, drift.returncode)
            self.assertEqual("", drift.stdout)
            error = json.loads(drift.stderr)
            self.assertEqual(False, error["ok"])
            self.assertIn("tables", error["error"])


class RuntimePreflightCliTests(unittest.TestCase):
    def setUp(self):
        self.guard = load_guard_module(self)
        self.live, self.compose = runtime_documents()

    def run_preflight(self, side_effect):
        stdout = StringIO()
        stderr = StringIO()
        with mock.patch("subprocess.run", side_effect=side_effect) as run_command:
            with redirect_stdout(stdout), redirect_stderr(stderr):
                exit_code = self.guard.main(
                    [
                        "runtime-preflight",
                        "--container",
                        "newapi",
                        "--compose-file",
                        "compose.yml",
                        "--service",
                        "newapi",
                    ]
                )
        return exit_code, stdout.getvalue(), stderr.getvalue(), run_command

    def successful_commands(self):
        return [
            subprocess.CompletedProcess(
                ["docker", "inspect", "newapi"],
                0,
                stdout=json.dumps([self.live]),
                stderr="",
            ),
            subprocess.CompletedProcess(
                [
                    "docker",
                    "compose",
                    "-f",
                    "compose.yml",
                    "config",
                    "--format",
                    "json",
                ],
                0,
                stdout=json.dumps(self.compose),
                stderr="",
            ),
        ]

    def test_runtime_preflight_runs_inspection_commands_in_memory(self):
        exit_code, stdout, stderr, run_command = self.run_preflight(
            self.successful_commands()
        )

        self.assertEqual(0, exit_code)
        self.assertEqual(
            {"ok": True, "container": "newapi", "service": "newapi"},
            json.loads(stdout),
        )
        self.assertEqual("", stderr)
        self.assertEqual(
            [
                mock.call(
                    ["docker", "inspect", "newapi"],
                    capture_output=True,
                    text=True,
                    check=True,
                ),
                mock.call(
                    [
                        "docker",
                        "compose",
                        "-f",
                        "compose.yml",
                        "config",
                        "--format",
                        "json",
                    ],
                    capture_output=True,
                    text=True,
                    check=True,
                ),
            ],
            run_command.call_args_list,
        )

    def test_runtime_preflight_reports_only_drift_labels(self):
        self.compose["services"]["newapi"]["environment"][
            "SESSION_SECRET"
        ] = "target-secret"

        exit_code, stdout, stderr, _ = self.run_preflight(
            self.successful_commands()
        )

        self.assertEqual(1, exit_code)
        self.assertEqual("", stdout)
        error = json.loads(stderr)
        self.assertEqual(False, error["ok"])
        self.assertIn("environment:SESSION_SECRET", error["error"])
        self.assertNotIn("live-secret", stdout + stderr)
        self.assertNotIn("target-secret", stdout + stderr)

    def test_runtime_preflight_masks_command_failure_output(self):
        failure = subprocess.CalledProcessError(
            1,
            ["docker", "inspect", "newapi"],
            stderr="live-secret",
        )

        exit_code, stdout, stderr, _ = self.run_preflight(failure)

        self.assertEqual(1, exit_code)
        self.assertEqual("", stdout)
        self.assertEqual(
            {"ok": False, "error": "runtime inspection command failed"},
            json.loads(stderr),
        )
        self.assertNotIn("live-secret", stdout + stderr)

    def test_runtime_preflight_masks_malformed_topology_values(self):
        sensitive_value = "sensitive-topology-value"
        cases = (
            "compose_nofile",
            "live_nofile",
            "live_restart",
            "compose_healthcheck",
            "live_healthcheck",
            "compose_duration",
            "compose_ulimits_type",
            "compose_network_type",
            "live_mount_type",
        )
        for case in cases:
            with self.subTest(case=case):
                self.live, self.compose = runtime_documents()
                service = self.compose["services"]["newapi"]
                if case == "compose_nofile":
                    service["ulimits"]["nofile"]["hard"] = sensitive_value
                elif case == "live_nofile":
                    self.live["HostConfig"]["Ulimits"][0][
                        "Soft"
                    ] = sensitive_value
                elif case == "live_restart":
                    self.live["HostConfig"]["RestartPolicy"][
                        "MaximumRetryCount"
                    ] = sensitive_value
                elif case == "compose_healthcheck":
                    service["healthcheck"]["retries"] = sensitive_value
                elif case == "live_healthcheck":
                    self.live["Config"]["Healthcheck"][
                        "Retries"
                    ] = sensitive_value
                elif case == "compose_duration":
                    service["healthcheck"]["interval"] = sensitive_value
                elif case == "compose_ulimits_type":
                    service["ulimits"] = sensitive_value
                elif case == "compose_network_type":
                    self.compose["networks"]["shared"] = sensitive_value
                else:
                    self.live["Mounts"] = [sensitive_value]

                exit_code, stdout, stderr, _ = self.run_preflight(
                    self.successful_commands()
                )

                self.assertEqual(1, exit_code)
                self.assertEqual("", stdout)
                self.assertEqual(
                    {
                        "ok": False,
                        "error": "runtime topology data is invalid",
                    },
                    json.loads(stderr),
                )
                self.assertNotIn("Traceback", stderr)
                self.assertNotIn(sensitive_value, stdout + stderr)

class CloudflarePlanTests(unittest.TestCase):
    def setUp(self):
        self.guard = load_guard_module(self)
        self.records = cloudflare_records()

    def test_changes_only_approved_a_record_content(self):
        plan = self.guard.plan_cloudflare_records(
            self.records, "203.0.113.10"
        )

        self.assertEqual(
            [
                {
                    "id": "1",
                    "type": "A",
                    "name": "yuaiapi.com",
                    "content": "203.0.113.10",
                    "proxied": True,
                    "ttl": 1,
                },
                {
                    "id": "2",
                    "type": "A",
                    "name": "api.yuaiapi.com",
                    "content": "203.0.113.10",
                    "proxied": True,
                    "ttl": 1,
                },
                {
                    "id": "3",
                    "type": "A",
                    "name": "global.yuaiapi.com",
                    "content": "203.0.113.10",
                    "proxied": True,
                    "ttl": 1,
                },
                {
                    "id": "4",
                    "type": "A",
                    "name": "vip.yuaiapi.com",
                    "content": "203.0.113.10",
                    "proxied": False,
                    "ttl": 300,
                },
            ],
            plan,
        )
        rendered = json.dumps(plan)
        self.assertNotIn("MX", rendered)
        self.assertNotIn("unapproved", rendered)
        self.assertNotIn("sensitive-unapproved-content", rendered)

    def test_rejects_missing_or_duplicate_production_records(self):
        missing = [
            record
            for record in self.records
            if record.get("name") != "vip.yuaiapi.com"
        ]
        with self.assertRaisesRegex(self.guard.GuardError, "missing"):
            self.guard.plan_cloudflare_records(missing, "203.0.113.10")

        duplicate = self.records + [dict(self.records[0], id="duplicate")]
        with self.assertRaisesRegex(self.guard.GuardError, "duplicate"):
            self.guard.plan_cloudflare_records(
                duplicate, "203.0.113.10"
            )

    def test_requires_a_strict_ipv4_string(self):
        for new_ip in (
            "not-an-ip",
            "2001:db8::1",
            " 203.0.113.10 ",
            123,
            True,
            None,
        ):
            with self.subTest(new_ip=new_ip), self.assertRaisesRegex(
                self.guard.GuardError, "IPv4"
            ):
                self.guard.plan_cloudflare_records(self.records, new_ip)

    def test_rejects_malformed_snapshot_types_without_values(self):
        sensitive_value = "sensitive-snapshot-value"
        invalid_record = [dict(record) for record in self.records]
        invalid_record[0]["ttl"] = sensitive_value
        cases = (
            sensitive_value,
            self.records + [sensitive_value],
            invalid_record,
        )
        for records in cases:
            with self.subTest(records_type=type(records).__name__):
                with self.assertRaises(self.guard.GuardError) as raised:
                    self.guard.plan_cloudflare_records(
                        records, "203.0.113.10"
                    )
                self.assertEqual(
                    "Cloudflare record snapshot is invalid",
                    str(raised.exception),
                )
                self.assertNotIn(sensitive_value, str(raised.exception))

    def test_requires_unique_opaque_nonblank_record_ids(self):
        sensitive_id = "sensitive-duplicate-record-id"
        duplicate_ids = [dict(record) for record in self.records]
        for record in duplicate_ids:
            if record.get("type") == "A" and record.get("name") in (
                "yuaiapi.com",
                "api.yuaiapi.com",
                "global.yuaiapi.com",
                "vip.yuaiapi.com",
            ):
                record["id"] = sensitive_id

        cases = [("duplicate", duplicate_ids)]
        for invalid_id in (True, 123, "", "   "):
            records = [dict(record) for record in self.records]
            records[0]["id"] = invalid_id
            cases.append((type(invalid_id).__name__, records))

        for case, records in cases:
            with self.subTest(case=case):
                with self.assertRaises(self.guard.GuardError) as raised:
                    self.guard.plan_cloudflare_records(
                        records, "203.0.113.10"
                    )
                self.assertEqual(
                    "Cloudflare record snapshot is invalid",
                    str(raised.exception),
                )
                self.assertNotIn(sensitive_id, str(raised.exception))


class CloudflarePlanCliTests(unittest.TestCase):
    def setUp(self):
        self.guard = load_guard_module(self)

    def run_guard(self, *arguments):
        return subprocess.run(
            [sys.executable, str(MODULE_PATH), *arguments],
            text=True,
            capture_output=True,
            check=False,
        )

    def test_cloudflare_plan_outputs_only_four_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            records_path = Path(directory) / "records.json"
            records_path.write_text(
                json.dumps(cloudflare_records()), encoding="utf-8"
            )

            result = self.run_guard(
                "cloudflare-plan",
                "--records",
                str(records_path),
                "--new-ip",
                "203.0.113.10",
            )

        self.assertEqual(0, result.returncode)
        self.assertEqual("", result.stderr)
        plan = json.loads(result.stdout)
        self.assertEqual(4, len(plan))
        self.assertEqual(
            [
                "yuaiapi.com",
                "api.yuaiapi.com",
                "global.yuaiapi.com",
                "vip.yuaiapi.com",
            ],
            [change["name"] for change in plan],
        )
        self.assertNotIn("unapproved", result.stdout)
        self.assertNotIn("MX", result.stdout)

    def test_cloudflare_plan_masks_malformed_snapshot_values(self):
        sensitive_value = "sensitive-snapshot-value"
        duplicate_ids = cloudflare_records()
        for record in duplicate_ids:
            if record.get("type") == "A" and record.get("name") in (
                "yuaiapi.com",
                "api.yuaiapi.com",
                "global.yuaiapi.com",
                "vip.yuaiapi.com",
            ):
                record["id"] = sensitive_value
        cases = (
            {"records": sensitive_value},
            cloudflare_records() + [sensitive_value],
            duplicate_ids,
        )
        with tempfile.TemporaryDirectory() as directory:
            records_path = Path(directory) / "records.json"
            for snapshot in cases:
                with self.subTest(snapshot_type=type(snapshot).__name__):
                    records_path.write_text(
                        json.dumps(snapshot), encoding="utf-8"
                    )
                    result = self.run_guard(
                        "cloudflare-plan",
                        "--records",
                        str(records_path),
                        "--new-ip",
                        "203.0.113.10",
                    )

                    self.assertEqual(1, result.returncode)
                    self.assertEqual("", result.stdout)
                    self.assertEqual(
                        {
                            "ok": False,
                            "error": "Cloudflare record snapshot is invalid",
                        },
                        json.loads(result.stderr),
                    )
                    self.assertNotIn(sensitive_value, result.stderr)
                    self.assertNotIn("Traceback", result.stderr)

    def test_cloudflare_plan_rejects_token_and_masks_arguments(self):
        sensitive_token = "sensitive-cloudflare-token"

        result = self.run_guard(
            "cloudflare-plan",
            "--records",
            "unused.json",
            "--new-ip",
            "203.0.113.10",
            "--token",
            sensitive_token,
        )

        self.assertEqual(1, result.returncode)
        self.assertEqual("", result.stdout)
        self.assertEqual(
            {"ok": False, "error": "invalid command line arguments"},
            json.loads(result.stderr),
        )
        self.assertNotIn(sensitive_token, result.stderr)
        self.assertNotIn("usage:", result.stderr)

    def test_cloudflare_plan_masks_unreadable_record_paths(self):
        sensitive_path = "sensitive-records-path"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            paths = (
                root / f"{sensitive_path}-missing.json",
                root / f"{sensitive_path}-directory",
            )
            paths[1].mkdir()
            for records_path in paths:
                with self.subTest(path_kind=records_path.name.rsplit("-", 1)[-1]):
                    result = self.run_guard(
                        "cloudflare-plan",
                        "--records",
                        str(records_path),
                        "--new-ip",
                        "203.0.113.10",
                    )

                    self.assertEqual(1, result.returncode)
                    self.assertEqual("", result.stdout)
                    self.assertEqual(
                        {
                            "ok": False,
                            "error": "Cloudflare record snapshot is invalid",
                        },
                        json.loads(result.stderr),
                    )
                    self.assertNotIn(sensitive_path, result.stderr)
                    self.assertNotIn("Traceback", result.stderr)

    def test_cloudflare_plan_masks_invalid_utf8_snapshot(self):
        sensitive_path = "sensitive-invalid-utf8"
        with tempfile.TemporaryDirectory() as directory:
            records_path = Path(directory) / f"{sensitive_path}.json"
            records_path.write_bytes(b"\xff\xfe\x80")

            result = self.run_guard(
                "cloudflare-plan",
                "--records",
                str(records_path),
                "--new-ip",
                "203.0.113.10",
            )

        self.assertEqual(1, result.returncode)
        self.assertEqual("", result.stdout)
        self.assertEqual(
            {
                "ok": False,
                "error": "Cloudflare record snapshot is invalid",
            },
            json.loads(result.stderr),
        )
        self.assertNotIn(sensitive_path, result.stderr)
        self.assertNotIn("Traceback", result.stderr)

    def test_cloudflare_snapshot_reader_masks_local_read_failures(self):
        sensitive_value = "sensitive-local-read-value"
        open_failure = PermissionError(sensitive_value)
        with mock.patch.object(
            self.guard.Path, "open", side_effect=open_failure
        ):
            with self.assertRaises(self.guard.GuardError) as raised:
                self.guard._read_cloudflare_records("unused.json")
        self.assertEqual(
            "Cloudflare record snapshot is invalid", str(raised.exception)
        )
        self.assertNotIn(sensitive_value, str(raised.exception))

        for failure in (
            MemoryError(sensitive_value),
            RecursionError(sensitive_value),
        ):
            with self.subTest(failure_type=type(failure).__name__):
                with mock.patch.object(
                    self.guard.Path,
                    "open",
                    mock.mock_open(read_data="[]"),
                ), mock.patch.object(
                    self.guard.json, "load", side_effect=failure
                ):
                    with self.assertRaises(self.guard.GuardError) as raised:
                        self.guard._read_cloudflare_records("unused.json")
                self.assertEqual(
                    "Cloudflare record snapshot is invalid",
                    str(raised.exception),
                )
                self.assertNotIn(sensitive_value, str(raised.exception))


if __name__ == "__main__":
    unittest.main()
