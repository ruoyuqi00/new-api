#!/usr/bin/env python3
"""Probe Kiro Web Portal RPC v2 CBOR APIs without printing secrets.

The Kiro web app calls:
  POST /service/KiroWebPortalService/operation/<Operation>

Most mutating operations require an X-CSRF-Token header. This probe tries the
smallest useful sequence:
  GetUserInfo -> GetUserUsageAndLimits -> CreateSpace -> StreamSendMessage

It is dependency-free so it can run on the deployment host.
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


DEFAULT_CREDS = "/opt/sub2api/kiro-rs/config/credentials.json"
PORTAL_BASE = "https://app.kiro.dev/service/KiroWebPortalService/operation"


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


def cbor_uint(major: int, value: int) -> bytes:
    prefix = major << 5
    if value < 24:
        return bytes([prefix | value])
    if value < 256:
        return bytes([prefix | 24, value])
    if value < 65536:
        return bytes([prefix | 25]) + value.to_bytes(2, "big")
    if value < 4294967296:
        return bytes([prefix | 26]) + value.to_bytes(4, "big")
    return bytes([prefix | 27]) + value.to_bytes(8, "big")


def cbor_encode(value: Any) -> bytes:
    if value is None:
        return b"\xf6"
    if value is False:
        return b"\xf4"
    if value is True:
        return b"\xf5"
    if isinstance(value, int):
        if value >= 0:
            return cbor_uint(0, value)
        return cbor_uint(1, -1 - value)
    if isinstance(value, float):
        import struct

        return b"\xfb" + struct.pack(">d", value)
    if isinstance(value, str):
        raw = value.encode("utf-8")
        return cbor_uint(3, len(raw)) + raw
    if isinstance(value, bytes):
        return cbor_uint(2, len(value)) + value
    if isinstance(value, list):
        return cbor_uint(4, len(value)) + b"".join(cbor_encode(item) for item in value)
    if isinstance(value, dict):
        items = [(k, v) for k, v in value.items() if v is not None]
        return cbor_uint(5, len(items)) + b"".join(
            cbor_encode(str(k)) + cbor_encode(v) for k, v in items
        )
    raise TypeError(f"unsupported CBOR value: {type(value).__name__}")


class CborReader:
    def __init__(self, data: bytes) -> None:
        self.data = data
        self.pos = 0

    def read(self, n: int) -> bytes:
        if self.pos + n > len(self.data):
            raise ValueError("truncated cbor")
        chunk = self.data[self.pos : self.pos + n]
        self.pos += n
        return chunk

    def length(self, ai: int) -> int:
        if ai < 24:
            return ai
        if ai == 24:
            return self.read(1)[0]
        if ai == 25:
            return int.from_bytes(self.read(2), "big")
        if ai == 26:
            return int.from_bytes(self.read(4), "big")
        if ai == 27:
            return int.from_bytes(self.read(8), "big")
        raise ValueError(f"unsupported additional info {ai}")

    def value(self) -> Any:
        head = self.read(1)[0]
        major, ai = head >> 5, head & 31
        if major == 0:
            return self.length(ai)
        if major == 1:
            return -1 - self.length(ai)
        if major == 2:
            return self.read(self.length(ai))
        if major == 3:
            if ai == 31:
                chunks: list[str] = []
                while self.data[self.pos] != 0xFF:
                    chunks.append(self.value())
                self.pos += 1
                return "".join(chunks)
            return self.read(self.length(ai)).decode("utf-8", "replace")
        if major == 4:
            if ai == 31:
                items: list[Any] = []
                while self.data[self.pos] != 0xFF:
                    items.append(self.value())
                self.pos += 1
                return items
            return [self.value() for _ in range(self.length(ai))]
        if major == 5:
            if ai == 31:
                out: dict[Any, Any] = {}
                while self.data[self.pos] != 0xFF:
                    key = self.value()
                    val = self.value()
                    out[hashable_key(key)] = val
                self.pos += 1
                return out
            out: dict[Any, Any] = {}
            for _ in range(self.length(ai)):
                key = self.value()
                val = self.value()
                out[hashable_key(key)] = val
            return out
        if major == 6:
            tag = self.length(ai)
            tagged = self.value()
            return {"$tag": tag, "value": tagged}
        if major == 7:
            if ai == 20:
                return False
            if ai == 21:
                return True
            if ai == 22:
                return None
            if ai == 27:
                import struct

                return struct.unpack(">d", self.read(8))[0]
        raise ValueError(f"unsupported cbor major={major} ai={ai}")


def cbor_decode(raw: bytes) -> Any:
    return CborReader(raw).value()


def hashable_key(value: Any) -> Any:
    try:
        hash(value)
        return value
    except TypeError:
        return json.dumps(sanitize(value), ensure_ascii=True, sort_keys=True)


def request_bytes(
    method: str,
    url: str,
    headers: dict[str, str],
    body: bytes | None = None,
    timeout: int = 45,
    max_read: int = 1024 * 1024,
) -> tuple[int | str, dict[str, str], bytes]:
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, dict(resp.headers), resp.read(max_read)
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read(max_read)
    except Exception as exc:
        return "ERR", {}, f"{type(exc).__name__}: {exc}".encode("utf-8", "replace")


def portal_headers(
    access_token: str,
    idp: str,
    csrf_token: str | None,
    cookie_header: str | None = None,
    user_id: str | None = None,
    visitor_id: str | None = None,
    accept: str = "application/cbor",
) -> dict[str, str]:
    headers = {
        "accept": accept,
        "content-type": "application/cbor",
        "smithy-protocol": "rpc-v2-cbor",
        "authorization": f"Bearer {access_token}",
        "cookie": merge_cookie_header(
            cookie_header or f"Idp={idp}; AccessToken={access_token}",
            {"kiro-visitor-id": visitor_id} if visitor_id else {},
        ),
        "amz-sdk-invocation-id": str(uuid.uuid4()),
        "amz-sdk-request": "attempt=1; max=1",
        "user-agent": "Mozilla/5.0 KiroWebPortalProbe/1.0",
    }
    if csrf_token:
        headers["X-CSRF-Token"] = csrf_token
    if user_id:
        headers["x-kiro-userid"] = user_id
    if visitor_id:
        headers["x-kiro-visitorid"] = visitor_id
    return headers


def merge_cookie_header(base: str, extras: dict[str, str | None]) -> str:
    parts = [part.strip() for part in base.split(";") if part.strip()]
    seen = {part.split("=", 1)[0] for part in parts if "=" in part}
    for name, value in extras.items():
        if value and name not in seen:
            parts.append(f"{name}={value}")
    return "; ".join(parts)


def redact_body(raw: bytes, headers: dict[str, str]) -> dict[str, Any]:
    content_type = (headers.get("content-type") or headers.get("Content-Type") or "").lower()
    if "application/cbor" in content_type and raw:
        try:
            decoded = cbor_decode(raw)
            return {"decoded": sanitize(decoded)}
        except Exception as exc:
            return {"decode_error": str(exc), "body_head": raw[:180].hex()}
    if "application/vnd.amazon.eventstream" in content_type and raw:
        try:
            return {"events": sanitize(decode_eventstream(raw))}
        except Exception as exc:
            return {
                "eventstream_decode_error": str(exc),
                "body_head": raw[:300].decode("utf-8", "replace"),
            }
    text = raw[:300].decode("utf-8", "replace")
    return {"body_head": text}


def read_eventstream_header_value(data: bytes, pos: int, value_type: int) -> tuple[Any, int]:
    if value_type is True:
        return True, pos
    if value_type is False:
        return False, pos
    if value_type == 0:
        return True, pos
    if value_type == 1:
        return False, pos
    if value_type == 2:
        return data[pos], pos + 1
    if value_type == 3:
        return int.from_bytes(data[pos : pos + 2], "big", signed=True), pos + 2
    if value_type == 4:
        return int.from_bytes(data[pos : pos + 4], "big", signed=True), pos + 4
    if value_type == 5:
        return int.from_bytes(data[pos : pos + 8], "big", signed=True), pos + 8
    if value_type == 6:
        size = int.from_bytes(data[pos : pos + 2], "big")
        pos += 2
        return data[pos : pos + size], pos + size
    if value_type == 7:
        size = int.from_bytes(data[pos : pos + 2], "big")
        pos += 2
        return data[pos : pos + size].decode("utf-8", "replace"), pos + size
    if value_type == 8:
        return int.from_bytes(data[pos : pos + 8], "big", signed=True), pos + 8
    if value_type == 9:
        size = int.from_bytes(data[pos : pos + 2], "big")
        pos += 2
        return data[pos : pos + size].hex(), pos + size
    return f"<unsupported-header-type:{value_type}>", len(data)


def decode_eventstream(raw: bytes, max_events: int = 12) -> list[dict[str, Any]]:
    events: list[dict[str, Any]] = []
    pos = 0
    while pos + 16 <= len(raw) and len(events) < max_events:
        total_len = int.from_bytes(raw[pos : pos + 4], "big")
        headers_len = int.from_bytes(raw[pos + 4 : pos + 8], "big")
        if total_len < 16 or pos + total_len > len(raw):
            break
        headers_start = pos + 12
        headers_end = headers_start + headers_len
        payload_start = headers_end
        payload_end = pos + total_len - 4
        headers_map: dict[str, Any] = {}
        hpos = headers_start
        while hpos < headers_end:
            name_len = raw[hpos]
            hpos += 1
            name = raw[hpos : hpos + name_len].decode("utf-8", "replace")
            hpos += name_len
            value_type = raw[hpos]
            hpos += 1
            value, hpos = read_eventstream_header_value(raw, hpos, value_type)
            headers_map[name] = value
        payload = raw[payload_start:payload_end]
        decoded_payload: Any
        if headers_map.get(":content-type") == "application/cbor" and payload:
            try:
                decoded_payload = cbor_decode(payload)
            except Exception as exc:
                decoded_payload = {"decode_error": str(exc), "payload_head": payload[:120].hex()}
        elif payload:
            decoded_payload = payload[:300].decode("utf-8", "replace")
        else:
            decoded_payload = None
        events.append({"headers": headers_map, "payload": decoded_payload})
        pos += total_len
    if pos < len(raw):
        events.append({"truncated_or_unparsed_bytes": len(raw) - pos})
    return events


def sanitize(value: Any) -> Any:
    if isinstance(value, dict):
        clean: dict[str, Any] = {}
        for key, item in value.items():
            lowered = str(key).lower()
            if "token" in lowered or lowered in {"authorization", "cookie"}:
                clean[key] = f"<redacted:{sha12(str(item))}>"
            else:
                clean[key] = sanitize(item)
        return clean
    if isinstance(value, list):
        return [sanitize(item) for item in value[:12]]
    if isinstance(value, bytes):
        return f"<bytes:{len(value)}>"
    return value


def portal_call(
    operation: str,
    body: dict[str, Any],
    access_token: str,
    idp: str,
    csrf_token: str | None,
    profile_arn: str | None = None,
    cookie_header: str | None = None,
    user_id: str | None = None,
    visitor_id: str | None = None,
    profile_location: str = "query",
    timeout: int = 45,
    max_read: int = 1024 * 1024,
    accept: str = "application/cbor",
) -> tuple[int | str, dict[str, str], bytes]:
    params = {}
    call_body = dict(body)
    if profile_arn and profile_location in {"query", "both"}:
        params["profileArn"] = profile_arn
    if profile_arn and profile_location in {"body", "both"} and "profileArn" not in call_body:
        call_body["profileArn"] = profile_arn
    qs = f"?{urllib.parse.urlencode(params)}" if params else ""
    return request_bytes(
        "POST",
        f"{PORTAL_BASE}/{operation}{qs}",
        portal_headers(access_token, idp, csrf_token, cookie_header, user_id, visitor_id, accept),
        cbor_encode(call_body),
        timeout=timeout,
        max_read=max_read,
    )


def html_meta(body: str, name: str) -> str | None:
    needle = f'meta name="{name}"'
    idx = body.find(needle)
    if idx < 0:
        return None
    frag = body[idx : idx + 300]
    for attr in ("content", "value"):
        marker = f'{attr}="'
        start = frag.find(marker)
        if start >= 0:
            start += len(marker)
            end = frag.find('"', start)
            if end >= 0:
                return frag[start:end]
    return None


def fetch_index_meta(access_token: str, idp: str, visitor_id: str | None = None) -> dict[str, Any]:
    cookie_header = merge_cookie_header(
        f"Idp={idp}; AccessToken={access_token}",
        {"kiro-visitor-id": visitor_id} if visitor_id else {},
    )
    headers = {
        "authorization": f"Bearer {access_token}",
        "cookie": cookie_header,
        "user-agent": "Mozilla/5.0 KiroWebPortalProbe/1.0",
    }
    req = urllib.request.Request("https://app.kiro.dev/", method="GET", headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            status: int | str = resp.status
            response_headers = resp.headers
            raw = resp.read(300000)
    except urllib.error.HTTPError as exc:
        status = exc.code
        response_headers = exc.headers
        raw = exc.read(300000)
    except Exception as exc:
        return {
            "status": "ERR",
            "error": f"{type(exc).__name__}: {exc}",
            "has_csrf": False,
            "csrf_len": 0,
            "csrf_token": None,
            "cookie_header": None,
            "cookie_names": [],
            "has_user_id": False,
            "user_id": None,
            "idp_meta": None,
            "has_profile": False,
            "user_status": None,
            "set_cookie": False,
        }
    set_cookies = response_headers.get_all("Set-Cookie") or []
    cookie_parts = [part.strip() for part in cookie_header.split(";") if part.strip()]
    cookie_names: list[str] = []
    for line in set_cookies:
        first = line.split(";", 1)[0].strip()
        if not first or "=" not in first:
            continue
        cookie_names.append(first.split("=", 1)[0])
        cookie_parts.append(first)
    cookie_header = "; ".join(cookie_parts)
    lower_headers = {k.lower(): v for k, v in dict(response_headers).items()}
    body = raw.decode("utf-8", "replace")
    csrf = html_meta(body, "csrf-token")
    user_id = html_meta(body, "user-id")
    return dict(
        status=status,
        has_csrf=bool(csrf),
        csrf_len=len(csrf or ""),
        csrf_token=csrf,
        cookie_header=cookie_header,
        cookie_names=cookie_names,
        has_user_id=bool(user_id),
        user_id=user_id,
        idp_meta=html_meta(body, "idp"),
        has_profile=bool(html_meta(body, "profile-arn")),
        user_status=html_meta(body, "user-status"),
        set_cookie="set-cookie" in lower_headers,
    )


def print_result(label: str, status: int | str, headers: dict[str, str], raw: bytes) -> None:
    lower_headers = {k.lower(): v for k, v in headers.items()}
    print(
        f"{label}:",
        json.dumps(
            {
                "status": status,
                "content_type": headers.get("content-type") or headers.get("Content-Type") or "",
                "has_set_cookie": "set-cookie" in lower_headers,
                "x_error_type": lower_headers.get("x-amzn-errortype", ""),
                **redact_body(raw, headers),
            },
            ensure_ascii=True,
        ),
    )


def extract_space_id(raw: bytes, headers: dict[str, str]) -> str | None:
    content_type = (headers.get("content-type") or headers.get("Content-Type") or "").lower()
    if "application/cbor" not in content_type:
        return None
    try:
        body = cbor_decode(raw)
    except Exception:
        return None
    if isinstance(body, dict):
        value = body.get("spaceId")
        if isinstance(value, str):
            return value
    return None


def decode_response(raw: bytes, headers: dict[str, str]) -> Any:
    content_type = (headers.get("content-type") or headers.get("Content-Type") or "").lower()
    if "application/cbor" in content_type and raw:
        return cbor_decode(raw)
    if raw:
        return json.loads(raw.decode("utf-8"))
    return None


def parse_provider_resource(value: str) -> dict[str, str]:
    parts = value.split(":", 2)
    if len(parts) < 2:
        raise argparse.ArgumentTypeError("provider resource must be TYPE:NAME or TYPE:NAME:BRANCH")
    out = {"providerType": parts[0].strip(), "name": parts[1].strip()}
    if len(parts) == 3 and parts[2].strip():
        out["branch"] = parts[2].strip()
    if not out["providerType"] or not out["name"]:
        raise argparse.ArgumentTypeError("provider resource TYPE and NAME cannot be empty")
    return out


def list_provider_resources(
    provider_type: str,
    access_token: str,
    idp: str,
    csrf_token: str | None,
    profile_arn: str | None,
    cookie_header: str | None,
    user_id: str | None,
    visitor_id: str | None,
    profile_location: str,
) -> list[dict[str, Any]]:
    resources: list[dict[str, Any]] = []
    next_token: str | None = None
    for _ in range(30):
        body: dict[str, Any] = {"providerType": provider_type, "csrfToken": csrf_token}
        if next_token:
            body["nextToken"] = next_token
        status, headers, raw = portal_call(
            "ListProviderResources",
            body,
            access_token,
            idp,
            csrf_token,
            profile_arn,
            cookie_header,
            user_id,
            visitor_id,
            profile_location,
            timeout=45,
        )
        print_result(f"ListProviderResources[{provider_type}]", status, headers, raw)
        if status != 200:
            break
        try:
            decoded = decode_response(raw, headers)
        except Exception:
            break
        if isinstance(decoded, dict):
            chunk = decoded.get("resources")
            if isinstance(chunk, list):
                resources.extend([item for item in chunk if isinstance(item, dict)])
            token = decoded.get("nextToken")
            next_token = token if isinstance(token, str) and token else None
            if next_token:
                time.sleep(0.2)
                continue
        break
    return resources


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--creds", default=DEFAULT_CREDS)
    parser.add_argument("--idp", action="append", help="IdP cookie value to try, e.g. Google/Github/BuilderId.")
    parser.add_argument("--csrf-token")
    parser.add_argument("--model", default="claude-opus-4.7")
    parser.add_argument("--skip-send", action="store_true")
    parser.add_argument("--no-create-space", action="store_true")
    parser.add_argument("--no-index-csrf", action="store_true")
    parser.add_argument("--space-id")
    parser.add_argument("--space-type", default="VIBE")
    parser.add_argument(
        "--omit-session-id",
        action="store_true",
        help="Do not include sessionId in SendMessage/StreamSendMessage.",
    )
    parser.add_argument("--visitor-id")
    parser.add_argument("--agent-mode")
    parser.add_argument("--profile-location", choices=["query", "body", "both", "none"], default="query")
    parser.add_argument("--send-operation", choices=["StreamSendMessage", "SendMessage"], default="StreamSendMessage")
    parser.add_argument("--send-accept", default="application/cbor")
    parser.add_argument(
        "--provider-resource",
        action="append",
        type=parse_provider_resource,
        default=[],
        help="Attach a provider resource to CreateSpace, as TYPE:NAME or TYPE:NAME:BRANCH.",
    )
    parser.add_argument(
        "--probe-provider",
        action="append",
        default=[],
        help="List resources for a provider type, e.g. GITHUB or MIDWAY. Defaults to both.",
    )
    parser.add_argument(
        "--set-network-access",
        choices=["OPEN_INTERNET", "COMMON_DEPENDENCIES", "INTEGRATIONS_ONLY"],
        help="Optionally update Kiro Web network access before sending.",
    )
    args = parser.parse_args()

    loaded = json.loads(Path(args.creds).read_text(encoding="utf-8"))
    creds = loaded[0] if isinstance(loaded, list) and loaded else loaded
    if not isinstance(creds, dict):
        print("error: credential file must contain an object or non-empty object array", file=sys.stderr)
        return 2

    access_token = pick(creds, "accessToken", "access_token")
    profile_arn = pick(creds, "profileArn", "profile_arn")
    csrf_token = args.csrf_token or pick(creds, "csrfToken", "csrf_token")
    email = pick(creds, "email", "loginHint", "login_hint")
    visitor_id = args.visitor_id or str(pick(creds, "visitorId", "visitor_id") or uuid.uuid4())
    provider = pick(creds, "idp", "provider", "authProvider", "auth_provider")
    idps = args.idp or ([str(provider)] if provider else [])
    for candidate in ["Google", "Github", "BuilderId"]:
        if candidate not in idps:
            idps.append(candidate)

    print(
        "credential:",
        json.dumps(
            {
                "email": email,
                "has_access": bool(access_token),
                "access_sha12": sha12(access_token),
                "profile_tail": str(profile_arn or "")[-16:],
                "has_csrf": bool(csrf_token),
                "visitor_sha12": sha12(visitor_id),
                "idps": idps,
            },
            ensure_ascii=True,
        ),
    )
    if not access_token:
        print("error: no access token", file=sys.stderr)
        return 2

    working_idp: str | None = None
    cookie_header: str | None = None
    user_id: str | None = None
    for idp in idps:
        status, headers, raw = portal_call(
            "GetUserInfo",
            {"origin": "KIRO_IDE"},
            access_token,
            idp,
            csrf_token,
            profile_arn,
            visitor_id=visitor_id,
            profile_location=args.profile_location,
            timeout=30,
        )
        print_result(f"GetUserInfo[{idp}]", status, headers, raw)
        if status == 200:
            working_idp = idp
            break
        time.sleep(0.3)

    if not working_idp:
        return 1

    status, headers, raw = portal_call(
        "GetUserUsageAndLimits",
        {"isEmailRequired": True, "origin": "KIRO_IDE"},
        access_token,
        working_idp,
        csrf_token,
        profile_arn,
        visitor_id=visitor_id,
        profile_location=args.profile_location,
        timeout=30,
    )
    print_result("GetUserUsageAndLimits", status, headers, raw)

    if not csrf_token and not args.no_index_csrf:
        index_meta = fetch_index_meta(access_token, working_idp, visitor_id)
        csrf_token = index_meta.pop("csrf_token", None)
        cookie_header = index_meta.pop("cookie_header", None)
        user_id = index_meta.pop("user_id", None)
        print("FetchIndexForCsrf:", json.dumps(index_meta, ensure_ascii=True))

    status, headers, raw = portal_call(
        "ListAvailableProviders",
        {"csrfToken": csrf_token},
        access_token,
        working_idp,
        csrf_token,
        profile_arn,
        cookie_header,
        user_id,
        visitor_id,
        args.profile_location,
        timeout=45,
    )
    print_result("ListAvailableProviders", status, headers, raw)

    discovered_resources: list[dict[str, Any]] = []
    provider_types = args.probe_provider or ["GITHUB", "MIDWAY"]
    for provider_type in provider_types:
        discovered_resources.extend(
            list_provider_resources(
                provider_type,
                access_token,
                working_idp,
                csrf_token,
                profile_arn,
                cookie_header,
                user_id,
                visitor_id,
                args.profile_location,
            )
        )

    if args.set_network_access:
        status, headers, raw = portal_call(
            "UpdateNetworkConfiguration",
            {
                "csrfToken": csrf_token,
                "networkConfig": {
                    "networkAccess": args.set_network_access,
                    "customDomains": [],
                },
            },
            access_token,
            working_idp,
            csrf_token,
            profile_arn,
            cookie_header,
            user_id,
            visitor_id,
            args.profile_location,
            timeout=45,
        )
        print_result("UpdateNetworkConfiguration", status, headers, raw)
        if status != 200:
            return 1

    space_id = args.space_id
    if not args.no_create_space and not space_id:
        create_body: dict[str, Any] = {"spaceType": args.space_type}
        provider_resources = args.provider_resource
        if provider_resources:
            create_body["providerResources"] = provider_resources
        status, headers, raw = portal_call(
            "CreateSpace",
            create_body,
            access_token,
            working_idp,
            csrf_token,
            profile_arn,
            cookie_header,
            user_id,
            visitor_id,
            args.profile_location,
            timeout=45,
        )
        print_result(f"CreateSpace[{args.space_type}]", status, headers, raw)
        if status == 200:
            space_id = extract_space_id(raw, headers)

    if args.skip_send or not space_id:
        print("send: skipped", json.dumps({"has_space_id": bool(space_id)}, ensure_ascii=True))
        return 0 if space_id else 1

    content_blocks = [{"text": {"text": "reply with OK only"}}]
    send_body: dict[str, Any] = {
        "spaceId": space_id,
        "contentBlocks": content_blocks,
        "modelId": args.model,
    }
    if not args.omit_session_id:
        send_body["sessionId"] = space_id
    if args.agent_mode:
        send_body["agentMode"] = args.agent_mode
    status, headers, raw = portal_call(
        args.send_operation,
        send_body,
        access_token,
        working_idp,
        csrf_token,
        profile_arn,
        cookie_header,
        user_id,
        visitor_id,
        args.profile_location,
        timeout=60,
        max_read=4096,
        accept=args.send_accept,
    )
    print_result(args.send_operation, status, headers, raw)
    return 0 if status == 200 else 1


if __name__ == "__main__":
    raise SystemExit(main())
