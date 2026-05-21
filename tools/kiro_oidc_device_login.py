#!/usr/bin/env python3
"""Run Kiro/AWS Builder ID or IdC device-code login without printing tokens.

The script is intentionally dependency-free so it can run on the deployment
host. It has two phases:

1. start: register an OIDC public client, create a device authorization, and
   write the sensitive polling state to a local file.
2. poll: wait for browser authorization, then write a Kiro credential JSON that
   can be tested with tools/kiro_server_probe.py.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import stat
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


BUILDER_ID_START_URL = "https://view.awsapps.com/start"
BUILDER_ID_REGION = "us-east-1"
SSO_SCOPES = [
    "codewhisperer:completions",
    "codewhisperer:analysis",
    "codewhisperer:conversations",
    "codewhisperer:transformations",
    "codewhisperer:taskassist",
]
DEVICE_GRANT = "urn:ietf:params:oauth:grant-type:device_code"


def sha12(value: str | None) -> str:
    if not value:
        return ""
    return hashlib.sha256(value.encode("utf-8")).hexdigest()[:12]


def post_json(url: str, body: dict[str, Any], user_agent: str = "pi-kiro") -> tuple[int, bytes]:
    payload = json.dumps(body, separators=(",", ":")).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=payload,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "User-Agent": user_agent,
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=45) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read(4096)


def read_json(path: Path) -> dict[str, Any]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return data


def write_secret_json(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    try:
        os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)
    except OSError:
        pass


def command_start(args: argparse.Namespace) -> int:
    region = args.region
    start_url = args.start_url
    oidc = f"https://oidc.{region}.amazonaws.com"

    reg_status, reg_raw = post_json(
        f"{oidc}/client/register",
        {
            "clientName": args.client_name,
            "clientType": "public",
            "scopes": SSO_SCOPES,
            "grantTypes": [DEVICE_GRANT, "refresh_token"],
        },
    )
    if reg_status != 200:
        print(
            "register:",
            json.dumps({"status": reg_status, "body_head": reg_raw[:240].decode("utf-8", "replace")}),
            file=sys.stderr,
        )
        return 1
    registered = json.loads(reg_raw.decode("utf-8"))
    client_id = registered["clientId"]
    client_secret = registered["clientSecret"]

    dev_status, dev_raw = post_json(
        f"{oidc}/device_authorization",
        {"clientId": client_id, "clientSecret": client_secret, "startUrl": start_url},
    )
    if dev_status != 200:
        print(
            "device_authorization:",
            json.dumps({"status": dev_status, "body_head": dev_raw[:240].decode("utf-8", "replace")}),
            file=sys.stderr,
        )
        return 1
    dev = json.loads(dev_raw.decode("utf-8"))

    state = {
        "authMethod": args.auth_method,
        "region": region,
        "startUrl": start_url,
        "oidcEndpoint": oidc,
        "clientId": client_id,
        "clientSecret": client_secret,
        "deviceCode": dev["deviceCode"],
        "userCode": dev["userCode"],
        "verificationUri": dev["verificationUri"],
        "verificationUriComplete": dev.get("verificationUriComplete") or dev["verificationUri"],
        "interval": int(dev.get("interval") or 5),
        "expiresAt": int(time.time()) + int(dev.get("expiresIn") or 600),
        "createdAt": int(time.time()),
        "clientName": args.client_name,
    }
    write_secret_json(Path(args.state), state)

    print(
        json.dumps(
            {
                "status": "authorization_required",
                "verification_uri": state["verificationUri"],
                "verification_uri_complete": state["verificationUriComplete"],
                "user_code": state["userCode"],
                "expires_in": int(dev.get("expiresIn") or 600),
                "interval": state["interval"],
                "state": args.state,
                "client_id_sha12": sha12(client_id),
            },
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


def command_poll(args: argparse.Namespace) -> int:
    state_path = Path(args.state)
    state = read_json(state_path)
    deadline = int(state["expiresAt"])
    interval = int(state.get("interval") or 5)
    base_interval = interval
    oidc = state["oidcEndpoint"]

    while int(time.time()) < deadline:
        time.sleep(interval)
        status, raw = post_json(
            f"{oidc}/token",
            {
                "clientId": state["clientId"],
                "clientSecret": state["clientSecret"],
                "deviceCode": state["deviceCode"],
                "grantType": DEVICE_GRANT,
            },
        )
        try:
            data = json.loads(raw.decode("utf-8"))
        except json.JSONDecodeError:
            if status >= 500:
                continue
            print("token:", json.dumps({"status": status, "body_head": raw[:160].decode("utf-8", "replace")}))
            return 1

        if status == 200 and data.get("accessToken") and data.get("refreshToken"):
            expires = dt.datetime.now(dt.timezone.utc) + dt.timedelta(seconds=int(data.get("expiresIn") or 3600))
            credential = {
                "authMethod": state.get("authMethod") or "builder-id",
                "provider": "aws-sso-oidc",
                "region": state["region"],
                "apiRegion": state["region"],
                "startUrl": state.get("startUrl"),
                "clientId": state["clientId"],
                "clientSecret": state["clientSecret"],
                "accessToken": data["accessToken"],
                "access_token": data["accessToken"],
                "refreshToken": data["refreshToken"],
                "refresh_token": data["refreshToken"],
                "tokenType": data.get("tokenType") or "Bearer",
                "expiresAt": expires.isoformat(),
                "expiresIn": int(data.get("expiresIn") or 3600),
                "createdAt": dt.datetime.now(dt.timezone.utc).isoformat(),
            }
            write_secret_json(Path(args.out), credential)
            print(
                json.dumps(
                    {
                        "status": "authorized",
                        "out": args.out,
                        "access_sha12": sha12(data["accessToken"]),
                        "refresh_sha12": sha12(data["refreshToken"]),
                        "expires_at": credential["expiresAt"],
                    },
                    ensure_ascii=False,
                    indent=2,
                )
            )
            return 0

        error = data.get("error")
        if error == "authorization_pending":
            continue
        if error == "slow_down":
            interval += base_interval
            continue
        print("token:", json.dumps({"status": status, "error": error or data, "body_head": ""}, ensure_ascii=False))
        return 1

    print("token:", json.dumps({"status": "timeout", "state": args.state}, ensure_ascii=False))
    return 1


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    start = sub.add_parser("start")
    start.add_argument("--state", default="/tmp/kiro_oidc_device_login_state.json")
    start.add_argument("--region", default=BUILDER_ID_REGION)
    start.add_argument("--start-url", default=BUILDER_ID_START_URL)
    start.add_argument("--auth-method", choices=["builder-id", "idc"], default="builder-id")
    start.add_argument("--client-name", default="pi-kiro")
    start.set_defaults(func=command_start)

    poll = sub.add_parser("poll")
    poll.add_argument("--state", default="/tmp/kiro_oidc_device_login_state.json")
    poll.add_argument("--out", default="/tmp/kiro_oidc_candidate.json")
    poll.set_defaults(func=command_poll)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
