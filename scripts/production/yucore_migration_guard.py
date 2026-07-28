#!/usr/bin/env python3
import argparse
import hashlib
import ipaddress
import json
import re
import subprocess
import sys
from decimal import Decimal
from pathlib import Path


MAINTENANCE_CONFIRMATION = "MIGRATE-YUCORE-PRODUCTION"
PRODUCTION_A_RECORDS = (
    "yuaiapi.com",
    "api.yuaiapi.com",
    "global.yuaiapi.com",
    "vip.yuaiapi.com",
)


class GuardError(RuntimeError):
    pass


def require_maintenance_confirmation(new_host, confirmation):
    if not new_host or not new_host.strip():
        raise GuardError("new host is required")
    if confirmation != MAINTENANCE_CONFIRMATION:
        raise GuardError("maintenance confirmation does not match")


def validate_status_document(payload):
    try:
        document = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise GuardError("status document is not valid JSON") from exc

    if not isinstance(document, dict):
        raise GuardError("status document must be a JSON object")
    if document.get("success") is not True:
        raise GuardError("status document does not report success")

    data = document.get("data")
    if not isinstance(data, dict):
        raise GuardError("status document is missing system_name")
    system_name = data.get("system_name")
    if not isinstance(system_name, str) or not system_name.strip():
        raise GuardError("status document is missing system_name")
    return document


def manifest_digest(document):
    canonical = json.dumps(
        document, ensure_ascii=True, sort_keys=True, separators=(",", ":")
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def compare_manifests(source, target):
    drift = []
    for section in sorted(set(source) | set(target)):
        if section not in source or section not in target:
            drift.append(section)
            continue

        source_value = json.dumps(
            source[section], ensure_ascii=True, sort_keys=True, separators=(",", ":")
        ).encode("utf-8")
        target_value = json.dumps(
            target[section], ensure_ascii=True, sort_keys=True, separators=(",", ":")
        ).encode("utf-8")
        if source_value != target_value:
            drift.append(section)
    return drift


def plan_cloudflare_records(records, new_ip):
    if not isinstance(new_ip, str):
        raise GuardError("new origin must be a valid IPv4 address")
    try:
        parsed_ip = ipaddress.IPv4Address(new_ip)
    except ipaddress.AddressValueError as exc:
        raise GuardError("new origin must be a valid IPv4 address") from exc

    if not isinstance(records, list) or not all(
        isinstance(record, dict) for record in records
    ):
        raise GuardError("Cloudflare record snapshot is invalid")

    changes = []
    selected_ids = set()
    for name in PRODUCTION_A_RECORDS:
        matches = [
            record
            for record in records
            if record.get("type") == "A" and record.get("name") == name
        ]
        if not matches:
            raise GuardError(f"missing production A record: {name}")
        if len(matches) != 1:
            raise GuardError(f"duplicate production A record: {name}")

        record = matches[0]
        record_id = record.get("id")
        proxied = record.get("proxied")
        ttl = record.get("ttl")
        if (
            not isinstance(record_id, str)
            or not record_id.strip()
            or record_id in selected_ids
            or not isinstance(proxied, bool)
            or type(ttl) is not int
            or ttl <= 0
        ):
            raise GuardError("Cloudflare record snapshot is invalid")
        selected_ids.add(record_id)
        changes.append(
            {
                "id": record_id,
                "type": record["type"],
                "name": record["name"],
                "content": str(parsed_ip),
                "proxied": proxied,
                "ttl": ttl,
            }
        )
    return changes


def _normalize_environment(environment):
    if isinstance(environment, dict):
        normalized = {}
        for key, value in environment.items():
            if value is None or isinstance(value, str):
                normalized[key] = value
            elif isinstance(value, bool):
                normalized[key] = json.dumps(value)
            else:
                normalized[key] = str(value)
        return normalized

    normalized = {}
    if isinstance(environment, list):
        for entry in environment:
            key, separator, value = str(entry).partition("=")
            normalized[key] = value if separator else None
    return normalized


def _normalize_live_mounts(live):
    mounts = []
    for mount in live.get("Mounts") or []:
        mount_type = str(mount.get("Type", ""))
        source = (
            mount.get("Name") if mount_type == "volume" else mount.get("Source")
        )
        mounts.append(
            (
                mount_type,
                str(source or ""),
                str(mount.get("Destination", "")),
                not bool(mount.get("RW")),
            )
        )
    return sorted(mounts)


def _normalize_compose_mounts(service, compose):
    volume_definitions = compose.get("volumes") or {}
    mounts = []
    for mount in service.get("volumes") or []:
        if not isinstance(mount, dict):
            continue
        mount_type = str(mount.get("type", ""))
        source = mount.get("source")
        if mount_type == "volume":
            definition = volume_definitions.get(source) or {}
            source = definition.get("name", source)
        mounts.append(
            (
                mount_type,
                str(source or ""),
                str(mount.get("target", "")),
                bool(mount.get("read_only", False)),
            )
        )
    return sorted(mounts)


def _normalize_live_networks(live):
    networks = live.get("NetworkSettings", {}).get("Networks") or {}
    return sorted(str(name) for name in networks)


def _normalize_compose_networks(service, compose):
    service_networks = service.get("networks") or {}
    if isinstance(service_networks, list):
        aliases = service_networks
    else:
        aliases = service_networks.keys()

    definitions = compose.get("networks") or {}
    resolved = []
    for alias in aliases:
        definition = definitions.get(alias) or {}
        resolved.append(str(definition.get("name", alias)))
    return sorted(resolved)


def _normalize_live_ports(live):
    port_bindings = live.get("HostConfig", {}).get("PortBindings") or {}
    bindings = []
    for container_port, host_bindings in port_bindings.items():
        target, separator, protocol = str(container_port).partition("/")
        protocol = protocol if separator else "tcp"
        for binding in host_bindings or []:
            bindings.append(
                (
                    target,
                    protocol,
                    str(binding.get("HostIp", "")),
                    str(binding.get("HostPort", "")),
                )
            )
    return sorted(bindings)


def _normalize_compose_ports(service):
    bindings = []
    for port in service.get("ports") or []:
        if not isinstance(port, dict):
            continue
        bindings.append(
            (
                str(port.get("target", "")),
                str(port.get("protocol", "tcp")),
                str(port.get("host_ip", "")),
                str(port.get("published", "")),
            )
        )
    return sorted(bindings)


def _normalize_live_restart(live):
    policy = live.get("HostConfig", {}).get("RestartPolicy") or {}
    name = str(policy.get("Name", ""))
    if name == "no":
        name = ""
    return name, int(policy.get("MaximumRetryCount", 0) or 0)


def _normalize_compose_restart(service):
    restart = str(service.get("restart") or "")
    if restart == "no":
        return "", 0
    name, separator, attempts = restart.partition(":")
    return name, int(attempts) if separator and attempts.isdigit() else 0


def _duration_nanoseconds(value):
    if value is None:
        return 0
    if isinstance(value, bool):
        raise ValueError("invalid duration")
    if isinstance(value, (int, float)):
        numeric = Decimal(str(value))
        if (
            not numeric.is_finite()
            or numeric < 0
            or numeric != numeric.to_integral_value()
        ):
            raise ValueError("invalid duration")
        return int(numeric)
    if not isinstance(value, str):
        raise ValueError("invalid duration")

    rendered = value
    units = {
        "ns": 1,
        "us": 1_000,
        "\u00b5s": 1_000,
        "\u03bcs": 1_000,
        "ms": 1_000_000,
        "s": 1_000_000_000,
        "m": 60_000_000_000,
        "h": 3_600_000_000_000,
    }
    parts = list(
        re.finditer(
            r"(\d+(?:\.\d+)?)(ns|us|\u00b5s|\u03bcs|ms|s|m|h)", rendered
        )
    )
    if not parts or "".join(part.group(0) for part in parts) != rendered:
        raise ValueError("invalid duration")
    nanoseconds = sum(
        (
            Decimal(part.group(1)) * units[part.group(2)]
            for part in parts
        ),
        Decimal(0),
    )
    if nanoseconds != nanoseconds.to_integral_value():
        raise ValueError("invalid duration")
    return int(nanoseconds)


def _normalize_healthcheck(healthcheck, compose=False):
    if not healthcheck:
        return None
    if compose and healthcheck.get("disable"):
        test = ["NONE"]
    else:
        test = healthcheck.get("test" if compose else "Test") or []
    if compose:
        keys = {
            "interval": "interval",
            "timeout": "timeout",
            "start_period": "start_period",
            "start_interval": "start_interval",
            "retries": "retries",
        }
    else:
        keys = {
            "interval": "Interval",
            "timeout": "Timeout",
            "start_period": "StartPeriod",
            "start_interval": "StartInterval",
            "retries": "Retries",
        }
    return (
        tuple(test),
        _duration_nanoseconds(healthcheck.get(keys["interval"])),
        _duration_nanoseconds(healthcheck.get(keys["timeout"])),
        _duration_nanoseconds(healthcheck.get(keys["start_period"])),
        _duration_nanoseconds(healthcheck.get(keys["start_interval"])),
        int(healthcheck.get(keys["retries"], 0) or 0),
    )


def _normalize_live_nofile(live):
    for ulimit in live.get("HostConfig", {}).get("Ulimits") or []:
        if ulimit.get("Name") == "nofile":
            return int(ulimit.get("Soft", 0)), int(ulimit.get("Hard", 0))
    return None


def _normalize_compose_nofile(service):
    nofile = (service.get("ulimits") or {}).get("nofile")
    if nofile is None:
        return None
    if isinstance(nofile, dict):
        return int(nofile.get("soft", 0)), int(nofile.get("hard", 0))
    return int(nofile), int(nofile)


def runtime_drift(live, compose, service_name):
    services = compose.get("services") or {}
    service = services.get(service_name)
    if not isinstance(service, dict):
        raise GuardError("compose service is missing")

    drift = set()
    if live.get("Config", {}).get("Image") != service.get("image"):
        drift.add("image")

    live_environment = _normalize_environment(live.get("Config", {}).get("Env"))
    compose_environment = _normalize_environment(service.get("environment"))
    for key, value in compose_environment.items():
        if live_environment.get(key) != value:
            drift.add(f"environment:{key}")

    if _normalize_live_mounts(live) != _normalize_compose_mounts(
        service, compose
    ):
        drift.add("mounts")
    if _normalize_live_networks(live) != _normalize_compose_networks(
        service, compose
    ):
        drift.add("networks")
    if _normalize_live_ports(live) != _normalize_compose_ports(service):
        drift.add("ports")
    if _normalize_live_restart(live) != _normalize_compose_restart(service):
        drift.add("restart")
    if _normalize_healthcheck(live.get("Config", {}).get("Healthcheck")) != (
        _normalize_healthcheck(service.get("healthcheck"), compose=True)
    ):
        drift.add("healthcheck")
    if _normalize_live_nofile(live) != _normalize_compose_nofile(service):
        drift.add("ulimits")
    return sorted(drift)


def _run_json_command(arguments):
    try:
        completed = subprocess.run(
            arguments, capture_output=True, text=True, check=True
        )
    except (OSError, subprocess.CalledProcessError) as exc:
        raise GuardError("runtime inspection command failed") from exc
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise GuardError("runtime inspection output is not valid JSON") from exc


def _read_json(path):
    with Path(path).open("r", encoding="utf-8") as handle:
        document = json.load(handle)
    if not isinstance(document, dict):
        raise GuardError("manifest must be a JSON object")
    return document


def _read_cloudflare_records(path):
    try:
        with Path(path).open("r", encoding="utf-8") as handle:
            records = json.load(handle)
    except (
        OSError,
        UnicodeError,
        json.JSONDecodeError,
        MemoryError,
        RecursionError,
    ) as exc:
        raise GuardError("Cloudflare record snapshot is invalid") from exc
    if not isinstance(records, list):
        raise GuardError("Cloudflare record snapshot is invalid")
    return records


class GuardArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise GuardError("invalid command line arguments")


def _build_parser():
    parser = GuardArgumentParser(description="YUcore production migration guard")
    subparsers = parser.add_subparsers(dest="command", required=True)

    confirm_parser = subparsers.add_parser("confirm")
    confirm_parser.add_argument("--new-host", required=True)
    confirm_parser.add_argument("--confirmation", required=True)

    subparsers.add_parser("validate-status")

    compare_parser = subparsers.add_parser("compare-manifests")
    compare_parser.add_argument("--source", required=True)
    compare_parser.add_argument("--target", required=True)

    runtime_parser = subparsers.add_parser("runtime-preflight")
    runtime_parser.add_argument("--container", required=True)
    runtime_parser.add_argument("--compose-file", required=True)
    runtime_parser.add_argument("--service", required=True)

    cloudflare_parser = subparsers.add_parser("cloudflare-plan")
    cloudflare_parser.add_argument("--records", required=True)
    cloudflare_parser.add_argument("--new-ip", required=True)
    return parser


def main(argv=None):
    try:
        args = _build_parser().parse_args(argv)
        if args.command == "confirm":
            require_maintenance_confirmation(args.new_host, args.confirmation)
            result = {"ok": True, "new_host": args.new_host}
        elif args.command == "validate-status":
            document = validate_status_document(sys.stdin.read())
            result = {
                "ok": True,
                "system_name": document["data"]["system_name"],
            }
        elif args.command == "compare-manifests":
            source = _read_json(args.source)
            target = _read_json(args.target)
            drift = compare_manifests(source, target)
            if drift:
                raise GuardError(
                    "manifest drift detected in sections: " + ", ".join(drift)
                )
            result = {"ok": True, "digest": manifest_digest(source)}
        elif args.command == "runtime-preflight":
            inspected = _run_json_command(
                ["docker", "inspect", args.container]
            )
            if (
                not isinstance(inspected, list)
                or len(inspected) != 1
                or not isinstance(inspected[0], dict)
            ):
                raise GuardError("runtime inspection output is invalid")
            compose = _run_json_command(
                [
                    "docker",
                    "compose",
                    "-f",
                    args.compose_file,
                    "config",
                    "--format",
                    "json",
                ]
            )
            if not isinstance(compose, dict):
                raise GuardError("compose configuration output is invalid")
            try:
                drift = runtime_drift(inspected[0], compose, args.service)
            except (
                AttributeError,
                KeyError,
                OverflowError,
                TypeError,
                ValueError,
            ) as exc:
                raise GuardError("runtime topology data is invalid") from exc
            if drift:
                raise GuardError("runtime drift detected: " + ",".join(drift))
            result = {
                "ok": True,
                "container": args.container,
                "service": args.service,
            }
        else:
            records = _read_cloudflare_records(args.records)
            result = plan_cloudflare_records(records, args.new_ip)
    except (GuardError, OSError, json.JSONDecodeError) as exc:
        print(
            json.dumps({"ok": False, "error": str(exc)}, separators=(",", ":")),
            file=sys.stderr,
        )
        return 1

    print(json.dumps(result, ensure_ascii=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    sys.exit(main())
