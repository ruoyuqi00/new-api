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
DEFAULT_RUNTIME_STATE_FILE = "/config/kiro-runtime-state.json"
DEFAULT_MODEL_CONTEXT_TOKENS = 200000
DEFAULT_MAX_OUTPUT_TOKENS = 64000
DEFAULT_TOKEN_BUFFER_RESERVE = 20000

DEFAULT_MODELS = [
    "auto",
    "claude-opus-4.8",
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
    "claude-opus-4.8": {"context_window": 1000000, "max_output_tokens": 128000},
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


def expiry_timestamp(value: Any) -> int | None:
    parsed = parse_expiry(value)
    if not parsed:
        return None
    return int(parsed.timestamp())


def credential_token_status(cred: dict[str, Any]) -> str:
    expires_at = parse_expiry(pick(cred, "expiresAt", "expires_at"))
    if not expires_at:
        return "unknown"
    now = now_utc()
    if expires_at <= now:
        return "expired"
    if expires_at <= now + dt.timedelta(minutes=5):
        return "expiring"
    return "valid"


def credential_id(cred: dict[str, Any], index: int) -> str:
    for key in ("id", "account_id", "accountId", "user_id", "userId"):
        value = cred.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    fingerprint = pick(cred, "email", "login_hint", "loginHint", "refreshToken", "refresh_token", "accessToken", "access_token")
    if isinstance(fingerprint, str) and fingerprint.strip():
        return f"kiro_{sha256_hex(fingerprint.strip())[:12]}"
    return f"kiro_account_{index + 1}"


def sanitize_credential_for_admin(cred: dict[str, Any], index: int) -> dict[str, Any]:
    disabled = bool(cred.get("disabled"))
    profile_arn = pick(cred, "profileArn", "profile_arn")
    email = pick(cred, "email", "login_hint", "loginHint")
    expires_at = pick(cred, "expiresAt", "expires_at")
    models = cred.get("availableModels") or cred.get("available_models") or cred.get("models") or DEFAULT_MODELS
    if not isinstance(models, list):
        models = DEFAULT_MODELS
    return {
        "id": credential_id(cred, index),
        "label": email or credential_id(cred, index),
        "email": email,
        "auth_method": pick(cred, "authMethod", "auth_method") or ("social" if pick(cred, "refreshToken", "refresh_token") else "access_token"),
        "provider": pick(cred, "idp", "provider", "authProvider", "auth_provider"),
        "engine": "kiro-web",
        "region": pick(cred, "authRegion", "auth_region", "region") or "us-east-1",
        "priority": cred.get("priority"),
        "disabled": disabled,
        "disabled_reason": cred.get("disabled_reason"),
        "disabled_at": cred.get("disabled_at"),
        "has_access_token": bool(pick(cred, "accessToken", "access_token")),
        "has_refresh_token": bool(pick(cred, "refreshToken", "refresh_token")),
        "has_profile_arn": bool(profile_arn),
        "profile_arn_present": bool(profile_arn),
        "expires_at": expiry_timestamp(expires_at),
        "expiresAt": expiry_timestamp(expires_at),
        "token_status": "disabled" if disabled else credential_token_status(cred),
        "runtime_status": "disabled" if disabled else "available",
        "availableModels": [str(model) for model in models if isinstance(model, str) and model.strip()],
        "supported_model_count": len([model for model in models if isinstance(model, str) and model.strip()]),
    }


def normalize_selection_strategy(raw: str | None) -> str:
    value = (raw or "round-robin").strip().lower().replace("_", "-")
    if value in {"priority", "priority-first", "first"}:
        return "priority-first"
    if value in {"sticky", "session-sticky", "affinity"}:
        return "sticky"
    return "round-robin"


def credential_model_ids(cred: dict[str, Any]) -> set[str]:
    raw_models = cred.get("availableModels") or cred.get("available_models") or cred.get("models")
    if not isinstance(raw_models, list) or not raw_models:
        return set()
    result: set[str] = set()
    for item in raw_models:
        if isinstance(item, str) and item.strip():
            result.add(map_model(item))
            result.add(item.strip())
    return result


def credential_supports_model(cred: dict[str, Any], model: str | None) -> bool:
    allowed = credential_model_ids(cred)
    if not allowed or not model:
        return True
    canonical = map_model(model)
    return canonical in allowed or model in allowed or "auto" in allowed


def request_affinity_key(headers: Any, payload: dict[str, Any], namespace: str) -> str | None:
    header_candidates = (
        "x-session-affinity",
        "x-claude-code-session-id",
        "x-opencode-session",
        "x-conversation-id",
        "x-thread-id",
    )
    for key in header_candidates:
        value = headers.get(key) if headers else None
        if isinstance(value, str) and value.strip():
            return f"{namespace}:header:{key}:{value.strip()[:256]}"

    body_candidates = (
        "conversation_id",
        "conversationId",
        "thread_id",
        "threadId",
        "session_id",
        "sessionId",
        "prompt_cache_key",
        "promptCacheKey",
        "user",
    )
    for key in body_candidates:
        value = payload.get(key)
        if isinstance(value, str) and value.strip():
            return f"{namespace}:body:{key}:{value.strip()[:256]}"

    metadata = payload.get("metadata")
    if isinstance(metadata, dict):
        for key in body_candidates + ("user_id", "userId"):
            value = metadata.get(key)
            if isinstance(value, str) and value.strip():
                return f"{namespace}:metadata:{key}:{value.strip()[:256]}"

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
                    tool_id = item.get("tool_use_id") or item.get("id")
                    label = f" id={tool_id}" if tool_id else ""
                    parts.append(f"[tool_result{label}]\n{tool_text}")
            elif item_type == "tool_use":
                name = item.get("name") or "tool"
                tool_id = item.get("id")
                label = f" id={tool_id}" if tool_id else ""
                tool_input = item.get("input")
                try:
                    input_text = json.dumps(tool_input, ensure_ascii=False, separators=(",", ":"))
                except TypeError:
                    input_text = str(tool_input)
                parts.append(f"[tool_use name={name}{label}]\n{input_text}")
            elif item_type in {"thinking", "redacted_thinking"}:
                thinking = item.get("thinking") or item.get("text") or "[redacted]"
                if isinstance(thinking, str) and thinking.strip():
                    parts.append(f"[{item_type}]\n{thinking.strip()}")
            elif item_type == "image":
                parts.append("[image omitted]")
        return "\n".join(part for part in parts if part)
    return ""


def message_metadata_to_text(message: dict[str, Any]) -> str:
    parts: list[str] = []

    if message.get("name"):
        parts.append(f"name={message['name']}")
    if message.get("tool_call_id"):
        parts.append(f"tool_call_id={message['tool_call_id']}")

    function_call = message.get("function_call")
    if isinstance(function_call, dict):
        try:
            parts.append(
                "[function_call]\n"
                + json.dumps(function_call, ensure_ascii=False, separators=(",", ":"))
            )
        except TypeError:
            parts.append(f"[function_call]\n{function_call}")

    tool_calls = message.get("tool_calls")
    if isinstance(tool_calls, list) and tool_calls:
        try:
            parts.append(
                "[tool_calls]\n"
                + json.dumps(tool_calls, ensure_ascii=False, separators=(",", ":"))
            )
        except TypeError:
            parts.append(f"[tool_calls]\n{tool_calls}")

    return "\n".join(part for part in parts if part)


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
        meta = message_metadata_to_text(message).strip()
        message_parts = [part for part in (meta, text) if part]
        if message_parts:
            parts.append(f"{role.capitalize()}:\n" + "\n".join(message_parts))
    return "\n\n".join(parts).strip() or "Hello"


def openai_prompt(payload: dict[str, Any]) -> str:
    parts: list[str] = []
    for message in payload.get("messages") or []:
        if not isinstance(message, dict):
            continue
        role = str(message.get("role") or "user")
        text = content_to_text(message.get("content")).strip()
        meta = message_metadata_to_text(message).strip()
        message_parts = [part for part in (meta, text) if part]
        if message_parts:
            parts.append(f"{role.capitalize()}:\n" + "\n".join(message_parts))
    return "\n\n".join(parts).strip() or "Hello"


def map_model(model: str | None) -> str:
    raw = (model or "").strip()
    lower = raw.lower()
    normalized = lower.replace("_", "-")

    direct = {
        "auto": "auto",
        "claude-opus-4.8": "claude-opus-4.8",
        "claude-opus-4-8": "claude-opus-4.8",
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
    if "opus" in normalized and "4-8" in normalized:
        return "claude-opus-4.8"
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
        self.lock = threading.RLock()
        self.state_lock = threading.Lock()
        self.session_cache: dict[str, PortalSession] = {}
        self.runtime_state_path = Path(os.environ.get("KIRO_RUNTIME_STATE_FILE") or DEFAULT_RUNTIME_STATE_FILE)
        self.selection_strategy = normalize_selection_strategy(os.environ.get("KIRO_ACCOUNT_SELECTION_STRATEGY"))
        self.session_affinity_enabled = env_bool("KIRO_SESSION_AFFINITY_ENABLED", True)
        self.session_affinity_ttl_seconds = env_int(
            "KIRO_SESSION_AFFINITY_TTL_SECONDS",
            3600,
            60,
            86400,
        )

    def load_credentials(self, model: str | None = None) -> tuple[list[tuple[dict[str, Any], int | None]], Any]:
        loaded = json.loads(self.creds_path.read_text(encoding="utf-8"))
        candidates = loaded if isinstance(loaded, list) else [loaded]
        if not isinstance(candidates, list):
            raise RuntimeError("credentials file must contain an object or object array")
        usable: list[tuple[dict[str, Any], int | None]] = []
        for idx, item in enumerate(candidates):
            if isinstance(item, dict) and not item.get("disabled"):
                if (
                    pick(item, "refreshToken", "refresh_token", "accessToken", "access_token")
                    and credential_supports_model(item, model)
                ):
                    usable.append((dict(item), idx if isinstance(loaded, list) else None))
        if not usable:
            suffix = f" for model {map_model(model)}" if model else ""
            raise RuntimeError(f"no enabled Kiro credential with token material{suffix}")
        usable.sort(key=lambda pair: (int(pair[0].get("priority") or 0), self.credential_runtime_id(pair[0], pair[1])))
        return usable, loaded

    def admin_credentials(self) -> dict[str, Any]:
        try:
            loaded = json.loads(self.creds_path.read_text(encoding="utf-8"))
        except FileNotFoundError:
            return {
                "provider": "kiro-web",
                "engine": "kiro-web",
                "credentials": [],
                "accounts": [],
                "total": 0,
                "available": 0,
                "disabled": 0,
                "with_profile_arn": 0,
                "models": [
                    {
                        "id": model,
                        "source": "adapter_default",
                        "supported_account_count": 0,
                        "last_smoke_status": "not_run",
                        "public_enabled": True,
                    }
                    for model in DEFAULT_MODELS
                ],
                "error": "credentials file not found",
            }
        candidates = loaded if isinstance(loaded, list) else [loaded]
        if not isinstance(candidates, list):
            candidates = []
        credentials = [
            sanitize_credential_for_admin(item, idx)
            for idx, item in enumerate(candidates)
            if isinstance(item, dict)
        ]
        available = [item for item in credentials if item.get("runtime_status") == "available"]
        model_counts: dict[str, int] = {}
        for item in available:
            for model in item.get("availableModels") or DEFAULT_MODELS:
                model_counts[str(model)] = model_counts.get(str(model), 0) + 1
        models = [
            {
                "id": model,
                "source": "adapter_default",
                "supported_account_count": model_counts.get(model, len(available)),
                "last_smoke_status": "not_run",
                "public_enabled": True,
            }
            for model in DEFAULT_MODELS
        ]
        return {
            "provider": "kiro-web",
            "engine": "kiro-web",
            "credentials": credentials,
            "accounts": credentials,
            "total": len(credentials),
            "available": len(available),
            "disabled": len(credentials) - len(available),
            "with_profile_arn": sum(1 for item in credentials if item.get("profile_arn_present")),
            "models": models,
            "model_count": len(models),
            "routing": {
                "default_strategy": self.selection_strategy,
                "session_sticky": self.session_affinity_enabled,
                "session_affinity_ttl_seconds": self.session_affinity_ttl_seconds,
                "runtime_state_file": str(self.runtime_state_path),
                "model_aware_routing": True,
                "auto_switch_on_quota": False,
                "allow_overage": True,
            },
        }

    def load_credential(self) -> tuple[dict[str, Any], int | None, Any]:
        usable, loaded = self.load_credentials()
        preferred_ids = self.select_credential_ids(None, None)
        ordered = self.order_usable_credentials(usable, preferred_ids)
        cred, idx = ordered[0]
        return dict(cred), idx, loaded

    def credential_runtime_id(self, cred: dict[str, Any], index: int | None) -> str:
        return credential_id(cred, index if isinstance(index, int) else 0)

    def read_runtime_state(self) -> dict[str, Any]:
        try:
            parsed = json.loads(self.runtime_state_path.read_text(encoding="utf-8"))
        except (FileNotFoundError, OSError, json.JSONDecodeError):
            return {"version": 1}
        return parsed if isinstance(parsed, dict) else {"version": 1}

    def write_runtime_state(self, state: dict[str, Any]) -> None:
        try:
            self.runtime_state_path.parent.mkdir(parents=True, exist_ok=True)
            tmp = self.runtime_state_path.with_suffix(self.runtime_state_path.suffix + ".tmp")
            tmp.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")
            tmp.replace(self.runtime_state_path)
        except OSError:
            # State persistence is an optimization. Requests should keep working
            # even if the mounted config directory is temporarily read-only.
            pass

    def prune_runtime_state(self, state: dict[str, Any], valid_ids: set[str]) -> None:
        now = int(time.time())
        affinity = state.get("affinity")
        if isinstance(affinity, dict):
            for key in list(affinity.keys()):
                item = affinity.get(key)
                if not isinstance(item, dict):
                    affinity.pop(key, None)
                    continue
                credential_id_value = item.get("credential_id")
                expires_at = int(item.get("expires_at") or 0)
                if credential_id_value not in valid_ids or expires_at <= now:
                    affinity.pop(key, None)
            if len(affinity) > 5000:
                sorted_items = sorted(
                    affinity.items(),
                    key=lambda pair: int(pair[1].get("expires_at") or 0) if isinstance(pair[1], dict) else 0,
                )
                for key, _ in sorted_items[: len(affinity) - 5000]:
                    affinity.pop(key, None)

    def order_usable_credentials(
        self,
        usable: list[tuple[dict[str, Any], int | None]],
        preferred_ids: list[str] | None,
    ) -> list[tuple[dict[str, Any], int | None]]:
        if not preferred_ids:
            return usable
        preferred = {value: idx for idx, value in enumerate(preferred_ids)}
        return sorted(
            usable,
            key=lambda pair: preferred.get(self.credential_runtime_id(pair[0], pair[1]), len(preferred)),
        )

    def select_credential_ids(self, model: str | None, affinity_key: str | None) -> list[str]:
        usable, _ = self.load_credentials(model)
        candidate_ids = [self.credential_runtime_id(cred, idx) for cred, idx in usable]
        if len(candidate_ids) <= 1:
            return candidate_ids

        route_key = map_model(model) if model else "global"
        selected_id: str | None = None
        affinity_hash = sha256_hex(affinity_key) if affinity_key and self.session_affinity_enabled else None

        with self.state_lock:
            state = self.read_runtime_state()
            state["version"] = 1
            state.setdefault("cursor", {})
            state.setdefault("affinity", {})
            state.setdefault("last_selected", {})
            self.prune_runtime_state(state, set(candidate_ids))

            affinity = state.get("affinity") if isinstance(state.get("affinity"), dict) else {}
            if affinity_hash:
                entry = affinity.get(affinity_hash)
                if isinstance(entry, dict):
                    candidate = entry.get("credential_id")
                    expires_at = int(entry.get("expires_at") or 0)
                    if candidate in candidate_ids and expires_at > int(time.time()):
                        selected_id = str(candidate)

            if selected_id is None and self.selection_strategy == "sticky":
                last_selected = state.get("last_selected") if isinstance(state.get("last_selected"), dict) else {}
                candidate = last_selected.get(route_key)
                if candidate in candidate_ids:
                    selected_id = str(candidate)

            if selected_id is None:
                if self.selection_strategy == "priority-first":
                    selected_id = candidate_ids[0]
                else:
                    cursor = state.get("cursor") if isinstance(state.get("cursor"), dict) else {}
                    raw_cursor = int(cursor.get(route_key) or 0)
                    selected_id = candidate_ids[raw_cursor % len(candidate_ids)]
                    cursor[route_key] = raw_cursor + 1
                    state["cursor"] = cursor

            if affinity_hash and selected_id:
                affinity[affinity_hash] = {
                    "credential_id": selected_id,
                    "expires_at": int(time.time()) + self.session_affinity_ttl_seconds,
                }
                state["affinity"] = affinity

            if selected_id:
                last_selected = state.get("last_selected") if isinstance(state.get("last_selected"), dict) else {}
                last_selected[route_key] = selected_id
                state["last_selected"] = last_selected
            state["updated_at"] = now_utc().isoformat()
            self.write_runtime_state(state)

        if selected_id in candidate_ids:
            start = candidate_ids.index(selected_id)
            return candidate_ids[start:] + candidate_ids[:start]
        return candidate_ids

    def remember_selected_credential(self, credential_id_value: str, model: str | None, affinity_key: str | None) -> None:
        route_key = map_model(model) if model else "global"
        affinity_hash = sha256_hex(affinity_key) if affinity_key and self.session_affinity_enabled else None
        with self.state_lock:
            state = self.read_runtime_state()
            state["version"] = 1
            last_selected = state.get("last_selected") if isinstance(state.get("last_selected"), dict) else {}
            last_selected[route_key] = credential_id_value
            state["last_selected"] = last_selected
            if affinity_hash:
                affinity = state.get("affinity") if isinstance(state.get("affinity"), dict) else {}
                affinity[affinity_hash] = {
                    "credential_id": credential_id_value,
                    "expires_at": int(time.time()) + self.session_affinity_ttl_seconds,
                }
                state["affinity"] = affinity
            state["updated_at"] = now_utc().isoformat()
            self.write_runtime_state(state)

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

    def ensure_access_token(
        self,
        force_refresh: bool = False,
        model: str | None = None,
        preferred_ids: list[str] | None = None,
        skip_ids: set[str] | None = None,
    ) -> tuple[dict[str, Any], str]:
        with self.lock:
            usable, loaded = self.load_credentials(model)
            usable = self.order_usable_credentials(usable, preferred_ids)
            last_error: Exception | None = None
            for cred, idx in usable:
                runtime_id = self.credential_runtime_id(cred, idx)
                if skip_ids and runtime_id in skip_ids:
                    continue
                access_token = pick(cred, "accessToken", "access_token")
                expires_at = parse_expiry(pick(cred, "expiresAt", "expires_at"))
                if (
                    not force_refresh
                    and access_token
                    and expires_at
                    and expires_at > now_utc() + dt.timedelta(minutes=5)
                ):
                    return cred, runtime_id
                refresh_token = pick(cred, "refreshToken", "refresh_token")
                if not refresh_token:
                    if access_token and not force_refresh:
                        return cred, runtime_id
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
                return refreshed, runtime_id
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

    def prepare_session(self, model: str | None = None, affinity_key: str | None = None) -> PortalSession:
        preferred_ids = self.select_credential_ids(model, affinity_key)
        last_error: Exception | None = None
        for force_refresh in (False, True):
            skipped: set[str] = set()
            while True:
                try:
                    cred, runtime_id = self.ensure_access_token(
                        force_refresh=force_refresh,
                        model=model,
                        preferred_ids=preferred_ids,
                        skip_ids=skipped,
                    )
                except Exception as exc:
                    last_error = exc
                    break

                access_token = pick(cred, "accessToken", "access_token")
                if not access_token:
                    last_error = RuntimeError("credential has no access token")
                    skipped.add(runtime_id)
                    continue
                cache_key = sha256_hex(access_token)[:24]
                cached = self.session_cache.get(cache_key)
                if not force_refresh and cached and cached.expires_at > time.time() + 120:
                    self.remember_selected_credential(runtime_id, model, affinity_key)
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
                    skipped.add(runtime_id)
                    continue

                try:
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
                except Exception as exc:
                    last_error = exc
                    self.session_cache.pop(cache_key, None)
                    skipped.add(runtime_id)
                    continue

                self.session_cache[cache_key] = session
                self.remember_selected_credential(runtime_id, model, affinity_key)
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

    def send_message_events(self, prompt: str, model: str, affinity_key: str | None = None) -> Iterator[dict[str, Any]]:
        session = self.prepare_session(model=model, affinity_key=affinity_key)
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

    def collect_text(
        self,
        prompt: str,
        model: str,
        affinity_key: str | None = None,
    ) -> tuple[str, dict[str, Any] | None]:
        chunks: list[str] = []
        completion: dict[str, Any] | None = None
        for event in self.send_message_events(prompt, model, affinity_key=affinity_key):
            text, _ = event_text(event)
            if text:
                chunks.append(text)
            maybe_completion = event_completion(event)
            if maybe_completion:
                completion = maybe_completion
        return "".join(chunks), completion

    def usage(self) -> dict[str, Any]:
        session = self.prepare_session(model="auto", affinity_key="admin:usage")
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
            if path in ("/api/admin/credentials", "/api/admin/accounts", "/api/admin/status"):
                if not self.require_auth():
                    return
                self.write_json(200, self.server.kiro.admin_credentials())
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
        affinity_key = request_affinity_key(self.headers, payload, "anthropic")
        if payload.get("stream"):
            self.stream_anthropic(prompt, model, affinity_key)
            return
        text, _ = self.server.kiro.collect_text(prompt, model, affinity_key=affinity_key)
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

    def stream_anthropic(self, prompt: str, model: str, affinity_key: str | None = None) -> None:
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
        for event in self.server.kiro.send_message_events(prompt, model, affinity_key=affinity_key):
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
        affinity_key = request_affinity_key(self.headers, payload, "openai")
        if payload.get("stream"):
            self.stream_openai(prompt, model, affinity_key)
            return
        text, _ = self.server.kiro.collect_text(prompt, model, affinity_key=affinity_key)
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

    def stream_openai(self, prompt: str, model: str, affinity_key: str | None = None) -> None:
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
        for event in self.server.kiro.send_message_events(prompt, model, affinity_key=affinity_key):
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
