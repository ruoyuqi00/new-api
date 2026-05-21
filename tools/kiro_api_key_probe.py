#!/usr/bin/env python3
"""Probe Kiro ControlPlane Bearer API key operations without printing secrets."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import stat
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


DEFAULT_CREDS = "/opt/sub2api/kiro-rs/config/credentials.json"
DEFAULT_ENDPOINT = "https://management.us-east-1.kiro.dev"


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


def sanitize(value: Any) -> Any:
    if isinstance(value, dict):
        clean: dict[str, Any] = {}
        for key, item in value.items():
            lower = str(key).lower()
            if lower in {"token", "access_token", "accessToken", "refresh_token", "refreshToken", "authorization", "rawkey", "raw_key"}:
                clean[key] = f"<redacted:{sha12(str(item))}>"
            elif lower == "keys" and isinstance(item, list):
                clean[key] = [sanitize_key_summary(entry) for entry in item[:50]]
                clean["key_count"] = len(item)
            else:
                clean[key] = sanitize(item)
        return clean
    if isinstance(value, list):
        return [sanitize(item) for item in value[:20]]
    return value


def sanitize_key_summary(value: Any) -> Any:
    if not isinstance(value, dict):
        return sanitize(value)
    return {
        "keyId_sha12": sha12(str(value.get("keyId") or "")),
        "keyPrefix": value.get("keyPrefix"),
        "label": value.get("label"),
        "createdAt": value.get("createdAt"),
        "lastUsedAt": value.get("lastUsedAt"),
    }


def request_json(
    url: str,
    body: dict[str, Any],
    access_token: str,
    target: str | None,
    timeout: int = 45,
) -> tuple[int | str, dict[str, str], bytes]:
    raw = json.dumps(body, separators=(",", ":")).encode("utf-8")
    headers = {
        "authorization": f"Bearer {access_token}",
        "accept": "application/json",
        "content-type": "application/x-amz-json-1.0",
        "user-agent": "Mozilla/5.0 KiroApiKeyProbe/1.0",
    }
    if target:
        headers["x-amz-target"] = target
    req = urllib.request.Request(url, data=raw, method="POST", headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, dict(resp.headers), resp.read(1024 * 1024)
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read(1024 * 1024)
    except Exception as exc:
        return "ERR", {}, f"{type(exc).__name__}: {exc}".encode("utf-8", "replace")


def decode_json(raw: bytes) -> Any:
    if not raw:
        return {}
    try:
        return json.loads(raw.decode("utf-8", "replace"))
    except Exception:
        return raw[:500].decode("utf-8", "replace")


def print_result(label: str, status: int | str, headers: dict[str, str], raw: bytes) -> Any:
    body = decode_json(raw)
    print(
        f"{label}:",
        json.dumps(
            {
                "status": status,
                "content_type": headers.get("content-type") or headers.get("Content-Type") or "",
                "x_error_type": (headers.get("x-amzn-errortype") or headers.get("x-amzn-ErrorType") or ""),
                "body": sanitize(body),
            },
            ensure_ascii=True,
        ),
    )
    return body


def save_secret(path: str, value: str) -> None:
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(value + "\n", encoding="utf-8")
    try:
        os.chmod(target, stat.S_IRUSR | stat.S_IWUSR)
    except Exception:
        pass


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--creds", default=DEFAULT_CREDS)
    parser.add_argument("--endpoint", default=DEFAULT_ENDPOINT)
    parser.add_argument("--profile-arn")
    parser.add_argument("--style", choices=["target", "path", "both"], default="both")
    parser.add_argument("--create-label")
    parser.add_argument("--expires-days", type=int)
    parser.add_argument("--out", default="/opt/sub2api/kiro-rs/config/generated-kiro-api-key.txt")
    args = parser.parse_args()

    loaded = json.loads(Path(args.creds).read_text(encoding="utf-8"))
    creds = loaded[0] if isinstance(loaded, list) and loaded else loaded
    if not isinstance(creds, dict):
        print("error: credential file must contain an object or non-empty object array", file=sys.stderr)
        return 2

    access_token = pick(creds, "accessToken", "access_token")
    provider = str(pick(creds, "idp", "provider", "authProvider", "auth_provider") or "Google")
    profile_arn = args.profile_arn or pick(creds, "profileArn", "profile_arn")
    if not access_token or not profile_arn:
        print("error: missing access token or profileArn", file=sys.stderr)
        return 2

    print(
        "credential:",
        json.dumps(
            {
                "email": pick(creds, "email", "loginHint", "login_hint"),
                "provider": provider,
                "has_access": bool(access_token),
                "access_sha12": sha12(access_token),
                "profile_tail": str(profile_arn)[-16:],
            },
            ensure_ascii=True,
        ),
    )

    styles = ["target", "path"] if args.style == "both" else [args.style]
    last_ok_style: str | None = None
    for style in styles:
        url = args.endpoint.rstrip("/") + ("/" if style == "target" else "/List-Api-Keys")
        target = "KiroControlPlaneBearerService.ListApiKeys" if style == "target" else None
        status, headers, raw = request_json(url, {"profileArn": profile_arn}, access_token, target)
        print_result(f"ListApiKeys[{style}]", status, headers, raw)
        if status == 200:
            last_ok_style = style
            break

    if not args.create_label:
        return 0 if last_ok_style else 1
    if not last_ok_style:
        print("create: skipped because ListApiKeys did not succeed", file=sys.stderr)
        return 1

    body = {"profileArn": profile_arn, "label": args.create_label}
    if args.expires_days:
        expires_at = (dt.datetime.now(dt.timezone.utc) + dt.timedelta(days=args.expires_days)).isoformat().replace("+00:00", "Z")
        body["expiresAt"] = expires_at
    url = args.endpoint.rstrip("/") + ("/" if last_ok_style == "target" else "/Create-Api-Key")
    target = "KiroControlPlaneBearerService.CreateApiKey" if last_ok_style == "target" else None
    status, headers, raw = request_json(url, body, access_token, target)
    decoded = print_result(f"CreateApiKey[{last_ok_style}]", status, headers, raw)
    raw_key = decoded.get("rawKey") if isinstance(decoded, dict) else None
    if isinstance(raw_key, str) and raw_key:
        save_secret(args.out, raw_key)
        print(
            "saved:",
            json.dumps({"path": args.out, "key_prefix": raw_key[:4], "key_sha12": sha12(raw_key)}, ensure_ascii=True),
        )
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
