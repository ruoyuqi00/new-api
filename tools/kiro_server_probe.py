#!/usr/bin/env python3
"""Probe Kiro account capability from a server without printing secrets.

This script is intentionally small and dependency-free so it can be copied to
the deployment host and run with the system Python. It reads a Kiro credential
JSON file, optionally refreshes the access token, calls official model discovery,
and smokes selected generateAssistantResponse endpoints.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path
from typing import Any


DEFAULT_CREDS = "/opt/sub2api/kiro-gateway/creds/kiro-auth-token.json"
DEFAULT_MODELS = [
    "qwen3-coder-next",
    "deepseek-3.2",
    "glm-5",
    "minimax-m2.5",
    "claude-sonnet-4.6",
    "claude-opus-4.6",
    "CLAUDE_SONNET_4_20250514_V1_0",
]


def pick(data: dict[str, Any], *keys: str) -> Any:
    for key in keys:
        value = data.get(key)
        if value not in (None, ""):
            return value
    return None


def sha12(value: str | None) -> str:
    if not value:
        return ""
    return hashlib.sha256(value.encode("utf-8")).hexdigest()[:12]


def parse_time(value: str | None) -> dt.datetime | None:
    if not value:
        return None
    try:
        if value.endswith("Z"):
            value = value[:-1] + "+00:00"
        parsed = dt.datetime.fromisoformat(value)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=dt.timezone.utc)
        return parsed.astimezone(dt.timezone.utc)
    except ValueError:
        return None


def is_expired(value: str | None, skew_seconds: int = 60) -> bool:
    parsed = parse_time(value)
    if parsed is None:
        return True
    return parsed <= dt.datetime.now(dt.timezone.utc) + dt.timedelta(seconds=skew_seconds)


def request(
    method: str,
    url: str,
    headers: dict[str, str] | None = None,
    body: dict[str, Any] | None = None,
    timeout: int = 45,
) -> tuple[int | str, dict[str, str], bytes]:
    payload = None
    req_headers = dict(headers or {})
    if body is not None:
        payload = json.dumps(body, separators=(",", ":")).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    req = urllib.request.Request(url, data=payload, method=method, headers=req_headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, dict(resp.headers), resp.read(4096)
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read(4096)
    except Exception as exc:  # pragma: no cover - operational diagnostic path
        return "ERR", {}, f"{type(exc).__name__}: {exc}".encode("utf-8", "replace")


def derive_machine_id(args: argparse.Namespace, creds: dict[str, Any]) -> str:
    if args.machine_id:
        return args.machine_id.strip()
    source = (
        pick(creds, "machineId", "machine_id")
        or pick(creds, "accountId", "account_id", "userId", "user_id", "id")
        or pick(creds, "email", "loginHint", "login_hint")
        or pick(creds, "profileArn", "profile_arn")
        or "server"
    )
    return hashlib.sha256(f"kiro-device-{source}".encode("utf-8")).hexdigest()


def refresh_access_token(
    creds: dict[str, Any],
    region: str,
    machine_id: str,
    kiro_version: str,
) -> dict[str, Any] | None:
    refresh_token = pick(creds, "refreshToken", "refresh_token")
    if not refresh_token:
        print("refresh: skipped no_refresh_token")
        return None
    url = f"https://prod.{region}.auth.desktop.kiro.dev/refreshToken"
    headers = {
        "Accept": "application/json, text/plain, */*",
        "Content-Type": "application/json",
        "User-Agent": f"KiroIDE-{kiro_version}-{machine_id}",
        "host": f"prod.{region}.auth.desktop.kiro.dev",
    }
    status, _, raw = request("POST", url, headers, {"refreshToken": refresh_token})
    if status != 200:
        print(
            "refresh:",
            json.dumps(
                {
                    "status": status,
                    "body_head": raw[:240].decode("utf-8", "replace"),
                },
                ensure_ascii=True,
            ),
        )
        return None
    data = json.loads(raw.decode("utf-8"))
    access_token = pick(data, "accessToken", "access_token")
    print(
        "refresh:",
        json.dumps(
            {
                "status": status,
                "has_access": bool(access_token),
                "access_sha12": sha12(access_token),
                "expiresIn": pick(data, "expiresIn", "expires_in"),
                "has_profile": bool(pick(data, "profileArn", "profile_arn")),
            },
            ensure_ascii=True,
        ),
    )
    return data


def kiro_headers(
    access_token: str,
    machine_id: str,
    kiro_version: str,
    agent_mode: str,
    client_style: str,
    amz_target: str = "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
    streaming: bool = True,
) -> dict[str, str]:
    if client_style == "open-kiro":
        api_name = "codewhispererstreaming" if streaming else "codewhispererruntime"
        ua = (
            "aws-sdk-rust/1.3.14 ua/2.1 "
            f"api/{api_name}/0.1.14474 os/linux lang/rust/1.92.0 "
            "md/appVersion-1.28.1 app/AmazonQ-For-CLI"
        )
        x_ua = (
            "aws-sdk-rust/1.3.14 ua/2.1 "
            f"api/{api_name}/0.1.14474 os/linux lang/rust/1.92.0 "
            "m/F,C app/AmazonQ-For-CLI"
        )
        return {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/x-amz-json-1.0",
            "Accept": "*/*",
            "x-amz-target": amz_target,
            "User-Agent": ua,
            "x-amz-user-agent": x_ua,
            "x-amzn-codewhisperer-optout": "false",
            "amz-sdk-invocation-id": str(uuid.uuid4()),
            "amz-sdk-request": "attempt=1; max=3",
        }

    if client_style == "kam-amazonq":
        return {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json",
            "x-amz-target": amz_target,
            "User-Agent": (
                "aws-sdk-js/1.0.34 ua/2.1 os/linux#server "
                f"lang/js md/nodejs#22.22.0 api/codewhispererstreaming#1.0.34 m/E "
                f"KiroIDE-{kiro_version}-{machine_id}"
            ),
            "x-amz-user-agent": f"aws-sdk-js/1.0.34 KiroIDE {kiro_version} {machine_id}",
            "x-amzn-codewhisperer-optout": "true",
            "x-amzn-kiro-agent-mode": agent_mode,
            "amz-sdk-invocation-id": str(uuid.uuid4()),
            "amz-sdk-request": "attempt=1; max=3",
        }

    if client_style == "pi-cli":
        mid = uuid.uuid4().hex
        ua = (
            "aws-sdk-rust/1.0.0 ua/2.1 os/other lang/rust "
            "api/codewhispererstreaming#1.28.3 m/E "
            f"app/AmazonQ-For-CLI md/appVersion-1.28.3-{mid}"
        )
        return {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/x-amz-json-1.0",
            "Accept": "application/json",
            "x-amz-target": amz_target,
            "User-Agent": ua,
            "x-amz-user-agent": ua,
            "x-amzn-codewhisperer-optout": "true",
            "x-amzn-kiro-agent-mode": agent_mode,
            "amz-sdk-invocation-id": str(uuid.uuid4()),
            "amz-sdk-request": "attempt=1; max=1",
        }

    return {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/x-amz-json-1.0",
        "x-amz-target": amz_target,
        "User-Agent": (
            "aws-sdk-js/1.0.34 ua/2.1 os/linux#server "
            f"lang/js md/nodejs#22.22.0 api/codewhispererstreaming#1.0.34 m/E "
            f"KiroIDE-{kiro_version}-{machine_id}"
        ),
        "x-amz-user-agent": f"aws-sdk-js/1.0.34 KiroIDE {kiro_version} {machine_id}",
        "x-amzn-codewhisperer-optout": "true",
        "x-amzn-kiro-agent-mode": agent_mode,
        "amz-sdk-invocation-id": str(uuid.uuid4()),
        "amz-sdk-request": "attempt=1; max=3",
    }


def list_models(
    region: str,
    access_token: str,
    profile_arn: str | None,
    machine_id: str,
    kiro_version: str,
    agent_mode: str,
    client_style: str,
) -> list[str]:
    names: list[str] = []
    if client_style in {"open-kiro", "pi-cli"}:
        params = {"origin": "KIRO_CLI"}
        if profile_arn:
            params["profileArn"] = profile_arn
        url = f"https://q.{region}.amazonaws.com/?{urllib.parse.urlencode(params)}"
        body = {"origin": "KIRO_CLI"}
        if profile_arn:
            body["profileArn"] = profile_arn
        headers = kiro_headers(
            access_token,
            machine_id,
            kiro_version,
            agent_mode,
            client_style,
            "AmazonCodeWhispererService.ListAvailableModels",
            streaming=False,
        )
        status, _, raw = request("POST", url, headers, body)
        if status != 200:
            print(
                "list_models:",
                json.dumps(
                    {
                        "style": client_style,
                        "status": status,
                        "count": 0,
                        "body_head": raw[:240].decode("utf-8", "replace"),
                    },
                    ensure_ascii=True,
                ),
            )
            return names
        parsed = json.loads(raw.decode("utf-8"))
        chunk = parsed.get("models") or parsed.get("modelSummaries") or []
        for model in chunk:
            if isinstance(model, dict):
                names.append(str(model.get("modelId") or model.get("modelName") or model)[:160])
            else:
                names.append(str(model)[:160])
        print(
            "list_models:",
            json.dumps({"style": client_style, "status": 200, "count": len(names), "models": names}, ensure_ascii=True),
        )
        return names

    headers = kiro_headers(access_token, machine_id, kiro_version, agent_mode, client_style)
    next_token: str | None = None
    for _ in range(10):
        params = {"origin": "AI_EDITOR", "maxResults": "50"}
        if profile_arn:
            params["profileArn"] = profile_arn
        if next_token:
            params["nextToken"] = next_token
        url = f"https://q.{region}.amazonaws.com/ListAvailableModels?{urllib.parse.urlencode(params)}"
        status, _, raw = request("GET", url, headers)
        if status != 200:
            print(
                "list_models:",
                json.dumps(
                    {
                        "status": status,
                        "count": len(names),
                        "body_head": raw[:240].decode("utf-8", "replace"),
                    },
                    ensure_ascii=True,
                ),
            )
            return names
        body = json.loads(raw.decode("utf-8"))
        chunk = body.get("models") or body.get("modelSummaries") or []
        for model in chunk:
            if isinstance(model, dict):
                names.append(str(model.get("modelId") or model.get("modelName") or model)[:160])
            else:
                names.append(str(model)[:160])
        next_token = body.get("nextToken")
        if not next_token:
            break
    print("list_models:", json.dumps({"status": 200, "count": len(names), "models": names}, ensure_ascii=True))
    return names


def smoke_generate(
    region: str,
    endpoint: str,
    access_token: str,
    profile_arn: str | None,
    machine_id: str,
    kiro_version: str,
    agent_mode: str,
    client_style: str,
    model: str,
) -> None:
    if endpoint == "codewhisperer":
        base = f"https://codewhisperer.{region}.amazonaws.com"
        path = "/generateAssistantResponse"
    elif endpoint == "runtime":
        base = f"https://runtime.{region}.kiro.dev"
        path = "/generateAssistantResponse"
    elif endpoint == "sendmsg":
        base = f"https://q.{region}.amazonaws.com"
        path = "/SendMessageStreaming"
    elif endpoint == "qroot":
        base = f"https://q.{region}.amazonaws.com"
        path = "/"
    else:
        base = f"https://q.{region}.amazonaws.com"
        path = "/generateAssistantResponse"
    origin = "AmazonQ" if endpoint == "sendmsg" or client_style == "kam-amazonq" else (
        "KIRO_CLI" if client_style in {"open-kiro", "pi-cli"} else "AI_EDITOR"
    )
    amz_target = (
        "AmazonQDeveloperStreamingService.SendMessage"
        if endpoint == "sendmsg" or client_style == "kam-amazonq"
        else "AmazonCodeWhispererStreamingService.GenerateAssistantResponse"
    )
    payload = {
        "conversationState": {
            "chatTriggerType": "MANUAL",
            **({"agentTaskType": agent_mode} if origin == "KIRO_CLI" else {}),
            "conversationId": str(uuid.uuid4()),
            "currentMessage": {
                "userInputMessage": {
                    "content": "reply ok",
                    "modelId": model,
                    "origin": origin,
                    "userInputMessageContext": {},
                }
            },
            "history": [],
        }
    }
    if origin == "KIRO_CLI":
        payload["agentMode"] = agent_mode
    if profile_arn:
        payload["profileArn"] = profile_arn
    headers = kiro_headers(access_token, machine_id, kiro_version, agent_mode, client_style, amz_target)
    status, resp_headers, raw = request(
        "POST",
        f"{base}{path}",
        headers,
        payload,
        timeout=60,
    )
    content_type = resp_headers.get("content-type") or resp_headers.get("Content-Type") or ""
    print(
        "generate:",
        json.dumps(
            {
                "endpoint": endpoint,
                "style": client_style,
                "model": model,
                "status": status,
                "content_type": content_type,
                "body_head": "eventstream/ok" if status == 200 else raw[:240].decode("utf-8", "replace"),
            },
            ensure_ascii=True,
        ),
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--creds", default=DEFAULT_CREDS)
    parser.add_argument("--region")
    parser.add_argument("--machine-id")
    parser.add_argument("--kiro-version", default="0.12.155")
    parser.add_argument("--agent-mode", default="spec", choices=["spec", "vibe"])
    parser.add_argument(
        "--client-style",
        default="ide",
        choices=["ide", "pi-cli", "open-kiro", "kam-amazonq"],
        help="Request fingerprint to use for list/generate probes.",
    )
    parser.add_argument("--refresh", action="store_true", help="Refresh even if access token is still valid.")
    parser.add_argument("--write-refreshed", action="store_true", help="Write refreshed token fields back to creds.")
    parser.add_argument("--model", action="append", dest="models")
    parser.add_argument(
        "--endpoint",
        action="append",
        choices=["q", "qroot", "sendmsg", "codewhisperer", "runtime"],
        dest="endpoints",
    )
    args = parser.parse_args()

    path = Path(args.creds)
    loaded = json.loads(path.read_text(encoding="utf-8"))
    creds = loaded[0] if isinstance(loaded, list) and loaded else loaded
    if not isinstance(creds, dict):
        print("error: credential file must contain an object or non-empty object array", file=sys.stderr)
        return 2
    region = args.region or pick(creds, "apiRegion", "api_region", "region") or "us-east-1"
    profile_arn = pick(creds, "profileArn", "profile_arn")
    access_token = pick(creds, "accessToken", "access_token")
    machine_id = derive_machine_id(args, creds)
    expires_at = pick(creds, "expiresAt", "expires_at")

    print(
        "credential:",
        json.dumps(
            {
                "email": pick(creds, "email", "loginHint", "login_hint"),
                "region": region,
                "profile_tail": str(profile_arn or "")[-16:],
                "has_access": bool(access_token),
                "access_sha12": sha12(access_token),
                "expiresAt": expires_at,
                "machine_sha12": sha12(machine_id),
            },
            ensure_ascii=True,
        ),
    )

    if args.refresh or is_expired(expires_at):
        refreshed = refresh_access_token(creds, region, machine_id, args.kiro_version)
        if refreshed:
            access_token = pick(refreshed, "accessToken", "access_token") or access_token
            profile_arn = pick(refreshed, "profileArn", "profile_arn") or profile_arn
            expires_in = pick(refreshed, "expiresIn", "expires_in")
            if args.write_refreshed and access_token:
                creds["accessToken"] = access_token
                if pick(refreshed, "refreshToken", "refresh_token"):
                    creds["refreshToken"] = pick(refreshed, "refreshToken", "refresh_token")
                if profile_arn:
                    creds["profileArn"] = profile_arn
                if expires_in:
                    expires = dt.datetime.now(dt.timezone.utc) + dt.timedelta(seconds=int(expires_in))
                    creds["expiresAt"] = expires.isoformat()
                write_obj: Any = loaded
                if isinstance(loaded, list):
                    loaded[0] = creds
                else:
                    write_obj = creds
                path.write_text(json.dumps(write_obj, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                print("write_refreshed: true")

    if not access_token:
        print("error: no access token available", file=sys.stderr)
        return 2

    list_models(region, access_token, profile_arn, machine_id, args.kiro_version, args.agent_mode, args.client_style)
    default_endpoints = ["qroot"] if args.client_style == "open-kiro" else ["q"]
    if args.client_style == "kam-amazonq":
        default_endpoints = ["sendmsg"]
    if args.client_style == "ide":
        default_endpoints = ["q", "codewhisperer", "runtime"]
    for endpoint in args.endpoints or default_endpoints:
        for model in args.models or DEFAULT_MODELS:
            smoke_generate(
                region,
                endpoint,
                access_token,
                profile_arn,
                machine_id,
                args.kiro_version,
                args.agent_mode,
                args.client_style,
                model,
            )
            time.sleep(0.2)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
