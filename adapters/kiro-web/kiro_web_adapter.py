#!/usr/bin/env python3
"""Small Kiro Web Portal adapter.

This service intentionally uses only the Python standard library so it can be
copied onto a server and run without a dependency bootstrap. It exposes a
minimal Anthropic/OpenAI compatible surface while calling Kiro's current web
portal RPC path internally.

Secrets are never logged by this process.
"""

from __future__ import annotations

import datetime as dt
import hashlib
import hmac
import json
import os
import secrets
import struct
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, Iterable, Iterator


PORTAL_BASE = "https://app.kiro.dev/service/KiroWebPortalService/operation"
DEFAULT_CREDS = "/config/credentials.json"
DEFAULT_API_KEY_FILE = "/config/generated-kiro-api-key.txt"
DEFAULT_MODEL_CONTEXT_TOKENS = 200000
DEFAULT_MAX_OUTPUT_TOKENS = 64000
DEFAULT_TOKEN_BUFFER_RESERVE = 20000

DEFAULT_MODELS = [
    "auto",
    "claude-opus-4.7",
    "claude-opus-4.6",
    "claude-sonnet-4.6",
    "claude-opus-4.5",
    "claude-sonnet-4.5",
    "claude-sonnet-4",
    "claude-haiku-4.5",
    "qwen3-coder-next",
    "deepseek-3.2",
    "minimax-m2.5",
    "minimax-m2.1",
    "glm-5",
]


def env_int(name: str, default: int, minimum: int, maximum: int) -> int:
    raw = os.environ.get(name)
    if raw is None:
        return default
    try:
        value = int(raw.strip())
    except ValueError:
        return default
    return max(minimum, min(maximum, value))


def env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def env_json_object(name: str) -> dict[str, Any]:
    raw = os.environ.get(name)
    if not raw:
        return {}
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return parsed if isinstance(parsed, dict) else {}


ENABLE_TOKEN_BUFFER_RESERVE = env_bool("KIRO_ENABLE_TOKEN_BUFFER_RESERVE", False)
TOKEN_BUFFER_RESERVE = env_int(
    "KIRO_TOKEN_BUFFER_RESERVE",
    DEFAULT_TOKEN_BUFFER_RESERVE,
    5000,
    150000,
)

MODEL_CAPABILITIES = {
    "auto": {"context_window": DEFAULT_MODEL_CONTEXT_TOKENS, "max_output_tokens": DEFAULT_MAX_OUTPUT_TOKENS},
    "claude-opus-4.7": {"context_window": DEFAULT_MODEL_CONTEXT_TOKENS, "max_output_tokens": DEFAULT_MAX_OUTPUT_TOKENS},
    "claude-opus-4.6": {"context_window": 1000000, "max_output_tokens": 128000},
    "claude-sonnet-4.6": {"context_window": 200000, "max_output_tokens": 64000},
    "claude-opus-4.5": {"context_window": 200000, "max_output_tokens": 64000},
    "claude-sonnet-4.5": {"context_window": 200000, "max_output_tokens": 64000},
    "claude-sonnet-4": {"context_window": 1000000, "max_output_tokens": 64000},
    "claude-haiku-4.5": {"context_window": 200000, "max_output_tokens": 64000},
    "qwen3-coder-next": {"context_window": 1048576, "max_output_tokens": 65536},
    "deepseek-3.2": {"context_window": 128000, "max_output_tokens": 8192},
    "minimax-m2.5": {"context_window": 1048576, "max_output_tokens": 65536},
    "minimax-m2.1": {"context_window": 1048576, "max_output_tokens": 65536},
    "glm-5": {"context_window": 128000, "max_output_tokens": 32768},
}
MODEL_CAPABILITY_OVERRIDES = env_json_object("KIRO_MODEL_CAPABILITIES_JSON")


class KiroCredentialAuthError(RuntimeError):
    """Raised when a stored credential is no longer accepted upstream."""


def now_utc() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def sha256_hex(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def pick(data: dict[str, Any], *keys: str) -> Any:
    for key in keys:
        value = data.get(key)
        if value not in (None, ""):
            return value
    return None


def parse_expiry(value: Any) -> dt.datetime | None:
    if value in (None, ""):
        return None
    if isinstance(value, (int, float)):
        return dt.datetime.fromtimestamp(float(value), tz=dt.timezone.utc)
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return None
        if text.isdigit():
            return dt.datetime.fromtimestamp(float(text), tz=dt.timezone.utc)
        if text.endswith("Z"):
            text = text[:-1] + "+00:00"
        try:
            parsed = dt.datetime.fromisoformat(text)
        except ValueError:
            return None
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=dt.timezone.utc)
        return parsed.astimezone(dt.timezone.utc)
    return None


def auth_failure_from_response(status: int, message: str) -> bool:
    lower = message.lower()
    if status in (401, 403):
        return True
    return any(
        marker in lower
        for marker in (
            "bad credentials",
            "invalid_grant",
            "invalid grant",
            "invalid_token",
            "invalid token",
            "token expired",
            "token has expired",
            "unauthorized",
        )
    )


def estimate_tokens_from_string(value: str) -> int:
    if not value:
        return 0
    return int((len(value.encode("utf-8")) / 3.5) + 0.999)


def model_capabilities(model: str) -> dict[str, int]:
    canonical = map_model(model)
    base = dict(
        MODEL_CAPABILITIES.get(
            canonical,
            {
                "context_window": DEFAULT_MODEL_CONTEXT_TOKENS,
                "max_output_tokens": DEFAULT_MAX_OUTPUT_TOKENS,
            },
        )
    )
    override = MODEL_CAPABILITY_OVERRIDES.get(canonical) or MODEL_CAPABILITY_OVERRIDES.get(model)
    if isinstance(override, int):
        base["context_window"] = override
    elif isinstance(override, dict):
        for key in ("context_window", "max_input_tokens", "max_output_tokens"):
            raw_value = override.get(key)
            if isinstance(raw_value, int) and raw_value > 0:
                target = "context_window" if key == "max_input_tokens" else key
                base[target] = raw_value
    return base


def model_context_tokens(model: str) -> int:
    return model_capabilities(model)["context_window"]


def model_max_output_tokens(model: str) -> int:
    return model_capabilities(model)["max_output_tokens"]


def trim_prompt_for_model(prompt: str, model: str) -> str:
    if not ENABLE_TOKEN_BUFFER_RESERVE:
        return prompt

    limit = max(8000, model_context_tokens(model) - TOKEN_BUFFER_RESERVE)
    if estimate_tokens_from_string(prompt) <= limit:
        return prompt

    marker = (
        "[Earlier conversation was omitted by the adapter to fit the "
        f"{map_model(model)} context window.]\n\n"
    )
    marker_bytes = len(marker.encode("utf-8"))
    available_bytes = max(1024, int(limit * 3.5) - marker_bytes)
    encoded = prompt.encode("utf-8")
    tail = encoded[-available_bytes:].decode("utf-8", "ignore")
    return marker + tail.lstrip()


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
    raise TypeError(f"unsupported CBOR type: {type(value).__name__}")


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
            out: dict[Any, Any] = {}
            if ai == 31:
                while self.data[self.pos] != 0xFF:
                    key = self.value()
                    val = self.value()
                    out[hashable_key(key)] = val
                self.pos += 1
                return out
            for _ in range(self.length(ai)):
                key = self.value()
                val = self.value()
                out[hashable_key(key)] = val
            return out
        if major == 6:
            tag = self.length(ai)
            return {"$tag": tag, "value": self.value()}
        if major == 7:
            if ai == 20:
                return False
            if ai == 21:
                return True
            if ai == 22:
                return None
            if ai == 27:
                return struct.unpack(">d", self.read(8))[0]
        raise ValueError(f"unsupported cbor major={major} ai={ai}")


def cbor_decode(raw: bytes) -> Any:
    return CborReader(raw).value()


def hashable_key(value: Any) -> Any:
    try:
        hash(value)
        return value
    except TypeError:
        return json.dumps(value, ensure_ascii=True, sort_keys=True)


def read_eventstream_header_value(data: bytes, pos: int, value_type: int) -> tuple[Any, int]:
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


def parse_eventstream_frame(frame: bytes) -> dict[str, Any]:
    total_len = int.from_bytes(frame[0:4], "big")
    headers_len = int.from_bytes(frame[4:8], "big")
    headers_start = 12
    headers_end = headers_start + headers_len
    payload_start = headers_end
    payload_end = total_len - 4
    headers: dict[str, Any] = {}
    hpos = headers_start
    while hpos < headers_end:
        name_len = frame[hpos]
        hpos += 1
        name = frame[hpos : hpos + name_len].decode("utf-8", "replace")
        hpos += name_len
        value_type = frame[hpos]
        hpos += 1
        value, hpos = read_eventstream_header_value(frame, hpos, value_type)
        headers[name] = value
    payload = frame[payload_start:payload_end]
    decoded: Any = None
    if payload:
        if headers.get(":content-type") == "application/cbor":
            decoded = cbor_decode(payload)
        else:
            decoded = payload.decode("utf-8", "replace")
    return {"headers": headers, "payload": decoded}


def iter_eventstream(response: Any) -> Iterator[dict[str, Any]]:
    buffer = b""
    while True:
        chunk = response.read(8192)
        if not chunk:
            break
        buffer += chunk
        while len(buffer) >= 12:
            total_len = int.from_bytes(buffer[0:4], "big")
            if total_len < 16:
                raise ValueError("invalid eventstream frame length")
            if len(buffer) < total_len:
                break
            frame = buffer[:total_len]
            buffer = buffer[total_len:]
            yield parse_eventstream_frame(frame)


def event_text(event: dict[str, Any]) -> tuple[str | None, dict[str, Any] | None]:
    payload = event.get("payload")
    if not isinstance(payload, dict) or payload.get("eventType") != "agent_message_chunk":
        return None, None
    raw = payload.get("payload")
    if not isinstance(raw, str):
        return None, None
    try:
        decoded = json.loads(raw)
    except json.JSONDecodeError:
        return None, None
    text = decoded.get("text")
    return (text if isinstance(text, str) else None), decoded


def event_completion(event: dict[str, Any]) -> dict[str, Any] | None:
    payload = event.get("payload")
    if not isinstance(payload, dict) or payload.get("eventType") != "session_info_update":
        return None
    raw = payload.get("payload")
    if not isinstance(raw, str):
        return None
    try:
        decoded = json.loads(raw)
    except json.JSONDecodeError:
        return None
    meta = decoded.get("_meta")
    if isinstance(meta, dict):
        kiro = meta.get("kiro")
        if isinstance(kiro, dict) and kiro.get("kind") == "turn_completion":
            return kiro
    return None


def content_to_text(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for item in content:
            if not isinstance(item, dict):
                continue
            item_type = item.get("type")
            if item_type == "text" and isinstance(item.get("text"), str):
                parts.append(item["text"])
            elif item_type == "tool_result":
                tool_text = content_to_text(item.get("content"))
                if tool_text:
                    parts.append(f"[tool_result]\n{tool_text}")
            elif item_type == "image":
                parts.append("[image omitted]")
        return "\n".join(part for part in parts if part)
    return ""


def anthropic_prompt(payload: dict[str, Any]) -> str:
    parts: list[str] = []
    system = payload.get("system")
    if isinstance(system, str) and system.strip():
        parts.append(f"System:\n{system.strip()}")
    elif isinstance(system, list):
        text = content_to_text(system)
        if text.strip():
            parts.append(f"System:\n{text.strip()}")

    for message in payload.get("messages") or []:
        if not isinstance(message, dict):
            continue
        role = str(message.get("role") or "user")
        text = content_to_text(message.get("content")).strip()
        if text:
            parts.append(f"{role.capitalize()}:\n{text}")
    return "\n\n".join(parts).strip() or "Hello"


def openai_prompt(payload: dict[str, Any]) -> str:
    parts: list[str] = []
    for message in payload.get("messages") or []:
        if not isinstance(message, dict):
            continue
        role = str(message.get("role") or "user")
        text = content_to_text(message.get("content")).strip()
        if text:
            parts.append(f"{role.capitalize()}:\n{text}")
    return "\n\n".join(parts).strip() or "Hello"


def map_model(model: str | None) -> str:
    raw = (model or "").strip()
    lower = raw.lower()
    normalized = lower.replace("_", "-")

    direct = {
        "auto": "auto",
        "claude-opus-4.7": "claude-opus-4.7",
        "claude-opus-4-7": "claude-opus-4.7",
        "claude-opus-4.6": "claude-opus-4.6",
        "claude-opus-4-6": "claude-opus-4.6",
        "claude-sonnet-4.6": "claude-sonnet-4.6",
        "claude-sonnet-4-6": "claude-sonnet-4.6",
        "claude-opus-4.5": "claude-opus-4.5",
        "claude-opus-4-5": "claude-opus-4.5",
        "claude-sonnet-4.5": "claude-sonnet-4.5",
        "claude-sonnet-4-5": "claude-sonnet-4.5",
        "claude-sonnet-4": "claude-sonnet-4",
        "claude-haiku-4.5": "claude-haiku-4.5",
        "claude-haiku-4-5": "claude-haiku-4.5",
    }
    if normalized in direct:
        return direct[normalized]
    if "opus" in normalized and "4-7" in normalized:
        return "claude-opus-4.7"
    if "opus" in normalized and "4-6" in normalized:
        return "claude-opus-4.6"
    if "opus" in normalized and "4-5" in normalized:
        return "claude-opus-4.5"
    if "sonnet" in normalized and "4-6" in normalized:
        return "claude-sonnet-4.6"
    if "sonnet" in normalized and "4-5" in normalized:
        return "claude-sonnet-4.5"
    if "sonnet" in normalized and "-4" in normalized:
        return "claude-sonnet-4"
    if "haiku" in normalized:
        return "claude-haiku-4.5"
    if raw in DEFAULT_MODELS:
        return raw
    return raw or "claude-sonnet-4.6"


def http_json_request(
    method: str,
    url: str,
    headers: dict[str, str],
    body: bytes | None = None,
    timeout: int = 60,
) -> tuple[int, dict[str, str], bytes]:
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, dict(resp.headers), resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read()


def merge_cookie_header(base: str, extras: dict[str, str | None]) -> str:
    parts = [part.strip() for part in base.split(";") if part.strip()]
    seen = {part.split("=", 1)[0] for part in parts if "=" in part}
    for name, value in extras.items():
        if value and name not in seen:
            parts.append(f"{name}={value}")
    return "; ".join(parts)


def html_meta(body: str, name: str) -> str | None:
    needle = f'meta name="{name}"'
    idx = body.find(needle)
    if idx < 0:
        return None
    frag = body[idx : idx + 320]
    for attr in ("content", "value"):
        marker = f'{attr}="'
        start = frag.find(marker)
        if start >= 0:
            start += len(marker)
            end = frag.find('"', start)
            if end >= 0:
                return frag[start:end]
    return None


@dataclass
class PortalSession:
    access_token: str
    idp: str
    csrf_token: str | None
    cookie_header: str | None
    user_id: str | None
    visitor_id: str
    profile_arn: str | None
    expires_at: float


class KiroWebClient:
    def __init__(self, creds_path: str) -> None:
        self.creds_path = Path(creds_path)
        self.lock = threading.Lock()
        self.session_cache: dict[str, PortalSession] = {}

    def load_credentials(self) -> tuple[list[tuple[dict[str, Any], int | None]], Any]:
        loaded = json.loads(self.creds_path.read_text(encoding="utf-8"))
        candidates = loaded if isinstance(loaded, list) else [loaded]
        if not isinstance(candidates, list):
            raise RuntimeError("credentials file must contain an object or object array")
        usable: list[tuple[dict[str, Any], int | None]] = []
        for idx, item in enumerate(candidates):
            if isinstance(item, dict) and not item.get("disabled"):
                if pick(item, "refreshToken", "refresh_token", "accessToken", "access_token"):
                    usable.append((dict(item), idx if isinstance(loaded, list) else None))
        if not usable:
            raise RuntimeError("no enabled Kiro credential with token material")
        usable.sort(key=lambda pair: int(pair[0].get("priority") or 0))
        return usable, loaded

    def load_credential(self) -> tuple[dict[str, Any], int | None, Any]:
        usable, loaded = self.load_credentials()
        cred, idx = usable[0]
        return dict(cred), idx, loaded

    def persist_credential(self, updated: dict[str, Any], index: int | None, loaded: Any) -> None:
        if index is None:
            to_write = updated
        else:
            to_write = loaded
            to_write[index] = updated
        tmp = self.creds_path.with_suffix(self.creds_path.suffix + ".tmp")
        tmp.write_text(json.dumps(to_write, ensure_ascii=False, indent=2), encoding="utf-8")
        tmp.replace(self.creds_path)

    def disable_credential(self, cred: dict[str, Any], index: int | None, loaded: Any, reason: str) -> None:
        updated = dict(cred)
        updated["disabled"] = True
        updated["disabled_reason"] = reason[:500]
        updated["disabled_at"] = now_utc().isoformat()
        self.persist_credential(updated, index, loaded)

    def ensure_access_token(self, force_refresh: bool = False) -> dict[str, Any]:
        with self.lock:
            usable, loaded = self.load_credentials()
            last_error: Exception | None = None
            for cred, idx in usable:
                access_token = pick(cred, "accessToken", "access_token")
                expires_at = parse_expiry(pick(cred, "expiresAt", "expires_at"))
                if (
                    not force_refresh
                    and access_token
                    and expires_at
                    and expires_at > now_utc() + dt.timedelta(minutes=5)
                ):
                    return cred
                refresh_token = pick(cred, "refreshToken", "refresh_token")
                if not refresh_token:
                    if access_token and not force_refresh:
                        return cred
                    last_error = RuntimeError("credential has no refresh token")
                    continue
                try:
                    refreshed = self.refresh_social_token(cred, refresh_token)
                except KiroCredentialAuthError as exc:
                    self.disable_credential(cred, idx, loaded, str(exc))
                    self.session_cache.clear()
                    last_error = exc
                    continue
                except Exception as exc:
                    last_error = exc
                    continue
                self.persist_credential(refreshed, idx, loaded)
                self.session_cache.clear()
                return refreshed
            if last_error:
                raise RuntimeError(f"no Kiro credential could be refreshed: {last_error}") from last_error
            raise RuntimeError("no enabled Kiro credential with token material")

    def refresh_social_token(self, cred: dict[str, Any], refresh_token: str) -> dict[str, Any]:
        region = str(pick(cred, "authRegion", "auth_region", "region") or "us-east-1")
        machine_id = pick(cred, "machineId", "machine_id")
        if not machine_id:
            machine_id = sha256_hex(f"KotlinNativeAPI/{refresh_token}")
        url = f"https://prod.{region}.auth.desktop.kiro.dev/refreshToken"
        body = json.dumps({"refreshToken": refresh_token}).encode("utf-8")
        headers = {
            "Accept": "application/json, text/plain, */*",
            "Content-Type": "application/json",
            "User-Agent": f"KiroIDE-0.9.2-{machine_id}",
            "host": f"prod.{region}.auth.desktop.kiro.dev",
            "Connection": "close",
        }
        status, _, raw = http_json_request("POST", url, headers, body=body, timeout=60)
        if status < 200 or status >= 300:
            message = raw.decode("utf-8", "replace")[:500] if raw else ""
            detail = f"Kiro token refresh failed with status {status}"
            if message:
                detail = f"{detail}: {message}"
            if auth_failure_from_response(status, message):
                raise KiroCredentialAuthError(detail)
            raise RuntimeError(detail)
        decoded = json.loads(raw.decode("utf-8"))
        updated = dict(cred)
        access_token = pick(decoded, "accessToken", "access_token")
        if access_token:
            if "access_token" in updated and "accessToken" not in updated:
                updated["access_token"] = access_token
            else:
                updated["accessToken"] = access_token
        new_refresh = pick(decoded, "refreshToken", "refresh_token")
        if new_refresh:
            if "refresh_token" in updated and "refreshToken" not in updated:
                updated["refresh_token"] = new_refresh
            else:
                updated["refreshToken"] = new_refresh
        profile_arn = pick(decoded, "profileArn", "profile_arn")
        if profile_arn:
            if "profile_arn" in updated and "profileArn" not in updated:
                updated["profile_arn"] = profile_arn
            else:
                updated["profileArn"] = profile_arn
        expires_in = pick(decoded, "expiresIn", "expires_in")
        if expires_in:
            expires_at = now_utc() + dt.timedelta(seconds=int(expires_in))
            updated["expiresAt"] = expires_at.isoformat()
        return updated

    def portal_headers(
        self,
        access_token: str,
        idp: str,
        csrf_token: str | None,
        cookie_header: str | None,
        user_id: str | None,
        visitor_id: str | None,
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
            "user-agent": "Mozilla/5.0 KiroWebAdapter/0.1",
        }
        if csrf_token:
            headers["X-CSRF-Token"] = csrf_token
        if user_id:
            headers["x-kiro-userid"] = user_id
        if visitor_id:
            headers["x-kiro-visitorid"] = visitor_id
        return headers

    def portal_call(
        self,
        session: PortalSession,
        operation: str,
        body: dict[str, Any],
        timeout: int = 60,
        accept: str = "application/cbor",
        stream: bool = False,
    ) -> Any:
        url = f"{PORTAL_BASE}/{operation}"
        call_body = dict(body)
        if session.profile_arn and "profileArn" not in call_body:
            call_body["profileArn"] = session.profile_arn
        headers = self.portal_headers(
            session.access_token,
            session.idp,
            session.csrf_token,
            session.cookie_header,
            session.user_id,
            session.visitor_id,
            accept,
        )
        encoded = cbor_encode(call_body)
        req = urllib.request.Request(url, data=encoded, method="POST", headers=headers)
        if stream:
            return urllib.request.urlopen(req, timeout=timeout)
        status, headers_out, raw = http_json_request("POST", url, headers, encoded, timeout)
        if status < 200 or status >= 300:
            raise RuntimeError(f"Kiro portal {operation} failed with status {status}")
        content_type = (headers_out.get("content-type") or headers_out.get("Content-Type") or "").lower()
        if "application/cbor" in content_type:
            return cbor_decode(raw)
        if raw:
            return json.loads(raw.decode("utf-8"))
        return None

    def fetch_index_meta(self, access_token: str, idp: str, visitor_id: str) -> dict[str, Any]:
        cookie_header = merge_cookie_header(
            f"Idp={idp}; AccessToken={access_token}",
            {"kiro-visitor-id": visitor_id},
        )
        headers = {
            "authorization": f"Bearer {access_token}",
            "cookie": cookie_header,
            "user-agent": "Mozilla/5.0 KiroWebAdapter/0.1",
        }
        req = urllib.request.Request("https://app.kiro.dev/", method="GET", headers=headers)
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read(300000)
            set_cookies = resp.headers.get_all("Set-Cookie") or []
        cookie_parts = [part.strip() for part in cookie_header.split(";") if part.strip()]
        for line in set_cookies:
            first = line.split(";", 1)[0].strip()
            if first and "=" in first:
                cookie_parts.append(first)
        body = raw.decode("utf-8", "replace")
        return {
            "csrf_token": html_meta(body, "csrf-token"),
            "cookie_header": "; ".join(cookie_parts),
            "user_id": html_meta(body, "user-id"),
            "idp": html_meta(body, "idp") or idp,
        }

    def prepare_session(self) -> PortalSession:
        last_error: Exception | None = None
        for force_refresh in (False, True):
            cred = self.ensure_access_token(force_refresh=force_refresh)
            access_token = pick(cred, "accessToken", "access_token")
            if not access_token:
                last_error = RuntimeError("credential has no access token")
                continue
            cache_key = sha256_hex(access_token)[:24]
            cached = self.session_cache.get(cache_key)
            if not force_refresh and cached and cached.expires_at > time.time() + 120:
                return cached

            visitor_id = str(pick(cred, "visitorId", "visitor_id") or uuid.uuid4())
            profile_arn = pick(cred, "profileArn", "profile_arn")
            provider = pick(cred, "idp", "provider", "authProvider", "auth_provider")
            idps = [str(provider)] if provider else []
            for candidate in ("Google", "Github", "BuilderId"):
                if candidate not in idps:
                    idps.append(candidate)

            working_idp: str | None = None
            for idp in idps:
                tmp = PortalSession(
                    access_token=access_token,
                    idp=idp,
                    csrf_token=None,
                    cookie_header=None,
                    user_id=None,
                    visitor_id=visitor_id,
                    profile_arn=profile_arn,
                    expires_at=time.time() + 600,
                )
                try:
                    self.portal_call(tmp, "GetUserInfo", {"origin": "KIRO_IDE"}, timeout=30)
                    working_idp = idp
                    break
                except Exception as exc:
                    last_error = exc
                    continue
            if not working_idp:
                self.session_cache.pop(cache_key, None)
                continue

            meta = self.fetch_index_meta(access_token, working_idp, visitor_id)
            session = PortalSession(
                access_token=access_token,
                idp=working_idp,
                csrf_token=meta.get("csrf_token"),
                cookie_header=meta.get("cookie_header"),
                user_id=meta.get("user_id"),
                visitor_id=visitor_id,
                profile_arn=profile_arn,
                expires_at=time.time() + 900,
            )
            self.warm_portal_session(session)
            self.session_cache[cache_key] = session
            return session
        if last_error:
            raise RuntimeError(f"Kiro portal authentication failed: {last_error}") from last_error
        raise RuntimeError("Kiro portal authentication failed")

    def warm_portal_session(self, session: PortalSession) -> None:
        # The official web app touches usage and provider state before creating a
        # space. Mirroring that sequence avoids a 401 seen when CreateSpace is
        # called immediately after fetching CSRF/cookies.
        warmups = [
            (
                "GetUserUsageAndLimits",
                {"isEmailRequired": True, "origin": "KIRO_IDE"},
            ),
            ("ListAvailableProviders", {"csrfToken": session.csrf_token}),
            (
                "ListProviderResources",
                {"providerType": "GITHUB", "csrfToken": session.csrf_token},
            ),
        ]
        for operation, body in warmups:
            try:
                self.portal_call(session, operation, body, timeout=45)
            except Exception:
                pass

    def create_space(self, session: PortalSession) -> str:
        decoded = self.portal_call(session, "CreateSpace", {"spaceType": "VIBE"}, timeout=45)
        if not isinstance(decoded, dict) or not isinstance(decoded.get("spaceId"), str):
            raise RuntimeError("Kiro CreateSpace response did not include spaceId")
        return decoded["spaceId"]

    def send_message_events(self, prompt: str, model: str) -> Iterator[dict[str, Any]]:
        session = self.prepare_session()
        space_id = self.create_space(session)
        prompt = trim_prompt_for_model(prompt, model)
        body = {
            "spaceId": space_id,
            "sessionId": space_id,
            "contentBlocks": [{"text": {"text": prompt}}],
            "modelId": map_model(model),
        }
        response = self.portal_call(
            session,
            "StreamSendMessage",
            body,
            timeout=120,
            accept="application/cbor",
            stream=True,
        )
        with response:
            for event in iter_eventstream(response):
                yield event

    def collect_text(self, prompt: str, model: str) -> tuple[str, dict[str, Any] | None]:
        chunks: list[str] = []
        completion: dict[str, Any] | None = None
        for event in self.send_message_events(prompt, model):
            text, _ = event_text(event)
            if text:
                chunks.append(text)
            maybe_completion = event_completion(event)
            if maybe_completion:
                completion = maybe_completion
        return "".join(chunks), completion

    def usage(self) -> dict[str, Any]:
        session = self.prepare_session()
        decoded = self.portal_call(
            session,
            "GetUserUsageAndLimits",
            {"isEmailRequired": True, "origin": "KIRO_IDE"},
            timeout=30,
        )
        return decoded if isinstance(decoded, dict) else {}


class AdapterServer(ThreadingHTTPServer):
    def __init__(self, server_address: tuple[str, int], handler: type[BaseHTTPRequestHandler]) -> None:
        super().__init__(server_address, handler)
        creds = os.environ.get("KIRO_CREDENTIALS_FILE", DEFAULT_CREDS)
        self.kiro = KiroWebClient(creds)
        self.api_key = load_api_key()


def load_api_key() -> str | None:
    env_key = os.environ.get("KIRO_ADAPTER_API_KEY")
    if env_key:
        return env_key.strip()
    key_file = os.environ.get("KIRO_ADAPTER_API_KEY_FILE", DEFAULT_API_KEY_FILE)
    try:
        value = Path(key_file).read_text(encoding="utf-8").strip()
        return value or None
    except FileNotFoundError:
        return None


class Handler(BaseHTTPRequestHandler):
    server: AdapterServer

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def read_json(self) -> dict[str, Any]:
        size = int(self.headers.get("Content-Length") or "0")
        raw = self.rfile.read(size)
        if not raw:
            return {}
        decoded = json.loads(raw.decode("utf-8"))
        if not isinstance(decoded, dict):
            raise ValueError("JSON body must be an object")
        return decoded

    def write_json(self, status: int, payload: dict[str, Any]) -> None:
        raw = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def write_error_json(self, status: int, message: str, error_type: str = "api_error") -> None:
        self.write_json(status, {"error": {"type": error_type, "message": message}})

    def authorized(self) -> bool:
        expected = self.server.api_key
        if not expected:
            return True
        supplied = self.headers.get("x-api-key") or ""
        auth = self.headers.get("authorization") or ""
        if auth.lower().startswith("bearer "):
            supplied = auth[7:].strip()
        return hmac.compare_digest(supplied, expected)

    def require_auth(self) -> bool:
        if self.authorized():
            return True
        self.write_error_json(401, "unauthorized", "authentication_error")
        return False

    def do_GET(self) -> None:
        try:
            path = urllib.parse.urlsplit(self.path).path
            if path in ("/health", "/healthz"):
                self.write_json(200, {"status": "ok"})
                return
            if path == "/v1/models":
                if not self.require_auth():
                    return
                now = int(time.time())
                self.write_json(
                    200,
                    {
                        "object": "list",
                        "data": [
                            {
                                "id": model,
                                "object": "model",
                                "created": now,
                                "owned_by": "kiro-web",
                                "context_window": model_context_tokens(model),
                                "max_output_tokens": model_max_output_tokens(model),
                            }
                            for model in DEFAULT_MODELS
                        ],
                    },
                )
                return
            if path == "/admin/usage":
                if not self.require_auth():
                    return
                usage = self.server.kiro.usage()
                self.write_json(200, {"provider": "kiro-web", "usage": usage})
                return
            self.write_error_json(404, "not found", "not_found")
        except Exception as exc:
            self.write_error_json(502, str(exc))

    def do_POST(self) -> None:
        try:
            if not self.require_auth():
                return
            payload = self.read_json()
            path = urllib.parse.urlsplit(self.path).path
            if path == "/v1/messages":
                self.handle_anthropic_messages(payload)
                return
            if path == "/v1/chat/completions":
                self.handle_openai_chat(payload)
                return
            self.write_error_json(404, "not found", "not_found")
        except json.JSONDecodeError:
            self.write_error_json(400, "invalid JSON", "invalid_request_error")
        except Exception as exc:
            self.write_error_json(502, str(exc))

    def sse_start(self) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()

    def sse_event(self, event: str | None, payload: dict[str, Any] | str) -> None:
        if event:
            self.wfile.write(f"event: {event}\n".encode("utf-8"))
        data = payload if isinstance(payload, str) else json.dumps(payload, ensure_ascii=False)
        for line in data.splitlines() or [""]:
            self.wfile.write(f"data: {line}\n".encode("utf-8"))
        self.wfile.write(b"\n")
        self.wfile.flush()

    def handle_anthropic_messages(self, payload: dict[str, Any]) -> None:
        model = str(payload.get("model") or "claude-sonnet-4.6")
        prompt = anthropic_prompt(payload)
        if payload.get("stream"):
            self.stream_anthropic(prompt, model)
            return
        text, _ = self.server.kiro.collect_text(prompt, model)
        self.write_json(
            200,
            {
                "id": f"msg_{secrets.token_hex(12)}",
                "type": "message",
                "role": "assistant",
                "model": model,
                "content": [{"type": "text", "text": text}],
                "stop_reason": "end_turn",
                "stop_sequence": None,
                "usage": {"input_tokens": 0, "output_tokens": 0},
            },
        )

    def stream_anthropic(self, prompt: str, model: str) -> None:
        message_id = f"msg_{secrets.token_hex(12)}"
        self.sse_start()
        self.sse_event(
            "message_start",
            {
                "type": "message_start",
                "message": {
                    "id": message_id,
                    "type": "message",
                    "role": "assistant",
                    "model": model,
                    "content": [],
                    "stop_reason": None,
                    "stop_sequence": None,
                    "usage": {"input_tokens": 0, "output_tokens": 0},
                },
            },
        )
        self.sse_event(
            "content_block_start",
            {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}},
        )
        for event in self.server.kiro.send_message_events(prompt, model):
            text, _ = event_text(event)
            if text:
                self.sse_event(
                    "content_block_delta",
                    {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": text}},
                )
        self.sse_event("content_block_stop", {"type": "content_block_stop", "index": 0})
        self.sse_event(
            "message_delta",
            {
                "type": "message_delta",
                "delta": {"stop_reason": "end_turn", "stop_sequence": None},
                "usage": {"output_tokens": 0},
            },
        )
        self.sse_event("message_stop", {"type": "message_stop"})

    def handle_openai_chat(self, payload: dict[str, Any]) -> None:
        model = str(payload.get("model") or "claude-sonnet-4.6")
        prompt = openai_prompt(payload)
        if payload.get("stream"):
            self.stream_openai(prompt, model)
            return
        text, _ = self.server.kiro.collect_text(prompt, model)
        created = int(time.time())
        self.write_json(
            200,
            {
                "id": f"chatcmpl-{secrets.token_hex(12)}",
                "object": "chat.completion",
                "created": created,
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": text},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
            },
        )

    def stream_openai(self, prompt: str, model: str) -> None:
        completion_id = f"chatcmpl-{secrets.token_hex(12)}"
        created = int(time.time())
        self.sse_start()
        first = {
            "id": completion_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": model,
            "choices": [{"index": 0, "delta": {"role": "assistant"}, "finish_reason": None}],
        }
        self.sse_event(None, first)
        for event in self.server.kiro.send_message_events(prompt, model):
            text, _ = event_text(event)
            if text:
                self.sse_event(
                    None,
                    {
                        "id": completion_id,
                        "object": "chat.completion.chunk",
                        "created": created,
                        "model": model,
                        "choices": [{"index": 0, "delta": {"content": text}, "finish_reason": None}],
                    },
                )
        self.sse_event(
            None,
            {
                "id": completion_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model,
                "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
            },
        )
        self.sse_event(None, "[DONE]")


def main() -> int:
    host = os.environ.get("KIRO_ADAPTER_HOST", "0.0.0.0")
    port = int(os.environ.get("KIRO_ADAPTER_PORT", "8991"))
    server = AdapterServer((host, port), Handler)
    print(f"kiro-web-adapter listening on {host}:{port}", flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
