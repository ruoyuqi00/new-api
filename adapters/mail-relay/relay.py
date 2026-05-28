#!/usr/bin/env python3
"""SMTP-to-HTTPS mail relay for Sub2API deployments.

The relay accepts plain SMTP on an internal Docker network and forwards the
message through an HTTPS email API. It is intentionally small: no public port,
no local persistence, and no provider secrets in the image.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import re
import signal
import socket
import threading
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from email import policy
from email.message import Message
from email.parser import BytesParser
from email.utils import formataddr, getaddresses, parseaddr
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from aiosmtpd.controller import Controller


LOG_LEVEL = os.getenv("MAIL_RELAY_LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s %(levelname)s %(message)s",
)
LOGGER = logging.getLogger("mail-relay")


class RelayConfigError(RuntimeError):
    pass


class RelayProviderError(RuntimeError):
    pass


@dataclass(frozen=True)
class OutboundEmail:
    sender_address: str
    sender_name: str
    to: list[str]
    cc: list[str]
    bcc: list[str]
    reply_to: str
    subject: str
    html: str
    text: str


def env_int(name: str, default: int) -> int:
    value = os.getenv(name, "").strip()
    if not value:
        return default
    try:
        return int(value)
    except ValueError:
        LOGGER.warning("invalid integer env %s=%r; using %d", name, value, default)
        return default


def truncate(value: str, limit: int = 240) -> str:
    value = value.replace("\r", " ").replace("\n", " ")
    if len(value) <= limit:
        return value
    return value[: limit - 3] + "..."


def parse_address_list(headers: list[str]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for _, address in getaddresses(headers):
        address = address.strip()
        if address and address.lower() not in seen:
            seen.add(address.lower())
            out.append(address)
    return out


def first_message_part(message: Message, content_type: str) -> str:
    parts: list[str] = []
    if message.is_multipart():
        for part in message.walk():
            if part.is_multipart():
                continue
            if part.get_content_type() != content_type:
                continue
            disposition = (part.get_content_disposition() or "").lower()
            if disposition == "attachment":
                continue
            try:
                content = part.get_content()
            except Exception:
                payload = part.get_payload(decode=True)
                if payload is None:
                    continue
                charset = part.get_content_charset() or "utf-8"
                content = payload.decode(charset, errors="replace")
            if isinstance(content, str) and content.strip():
                parts.append(content)
    else:
        if message.get_content_type() == content_type:
            try:
                content = message.get_content()
            except Exception:
                payload = message.get_payload(decode=True) or b""
                charset = message.get_content_charset() or "utf-8"
                content = payload.decode(charset, errors="replace")
            if isinstance(content, str) and content.strip():
                parts.append(content)
    return "\n".join(parts).strip()


def html_to_text(html: str) -> str:
    if not html:
        return ""
    text = re.sub(r"(?is)<(script|style).*?>.*?</\1>", "", html)
    text = re.sub(r"(?s)<br\s*/?>", "\n", text)
    text = re.sub(r"(?s)</p\s*>", "\n", text)
    text = re.sub(r"(?s)<[^>]+>", "", text)
    return re.sub(r"\n{3,}", "\n\n", text).strip()


def parse_outbound(message: Message, envelope: Any) -> OutboundEmail:
    forced_from = os.getenv("MAIL_RELAY_FROM", "").strip()
    forced_from_name = os.getenv("MAIL_RELAY_FROM_NAME", "").strip()
    header_from = str(message.get("From", "")).strip()
    header_name, header_address = parseaddr(header_from)
    _, forced_address = parseaddr(forced_from)

    sender_address = forced_address or header_address or str(envelope.mail_from or "").strip()
    sender_name = forced_from_name or header_name
    if not sender_address:
        raise RelayConfigError("missing sender address; set MAIL_RELAY_FROM or Sub2API From Email")

    to = parse_address_list(message.get_all("To", []))
    cc = parse_address_list(message.get_all("Cc", []))
    bcc = parse_address_list(message.get_all("Bcc", []))
    if not to:
        to = [addr for addr in getattr(envelope, "rcpt_tos", []) if addr]
    if not to:
        raise RelayConfigError("missing recipient address")

    _, reply_to = parseaddr(str(message.get("Reply-To", "")).strip())
    subject = str(message.get("Subject", "")).strip() or "(no subject)"

    html = first_message_part(message, "text/html")
    text = first_message_part(message, "text/plain")
    if not text and html:
        text = html_to_text(html)
    if not html and not text:
        text = "(empty message)"

    return OutboundEmail(
        sender_address=sender_address,
        sender_name=sender_name,
        to=to,
        cc=cc,
        bcc=bcc,
        reply_to=reply_to,
        subject=subject,
        html=html,
        text=text,
    )


def json_request(url: str, token: str, payload: dict[str, Any], timeout_seconds: int) -> dict[str, Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
            "User-Agent": "sub2api-mail-relay/1.0",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            raw = response.read().decode("utf-8", errors="replace")
            if not raw:
                return {}
            return json.loads(raw)
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        raise RelayProviderError(f"HTTP {exc.code}: {truncate(raw)}") from exc
    except urllib.error.URLError as exc:
        raise RelayProviderError(f"network error: {exc.reason}") from exc


def send_cloudflare(email: OutboundEmail, timeout_seconds: int) -> None:
    account_id = os.getenv("CLOUDFLARE_ACCOUNT_ID", "").strip()
    token = os.getenv("CLOUDFLARE_API_TOKEN", "").strip()
    if not account_id or not token:
        raise RelayConfigError("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")

    sender: str | dict[str, str]
    if email.sender_name:
        sender = {"address": email.sender_address, "name": email.sender_name}
    else:
        sender = email.sender_address

    payload: dict[str, Any] = {
        "from": sender,
        "to": email.to,
        "subject": email.subject,
    }
    if email.html:
        payload["html"] = email.html
    if email.text:
        payload["text"] = email.text
    if email.cc:
        payload["cc"] = email.cc
    if email.bcc:
        payload["bcc"] = email.bcc
    if email.reply_to:
        payload["reply_to"] = email.reply_to

    url = f"https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send"
    result = json_request(url, token, payload, timeout_seconds)
    if result and result.get("success") is False:
        raise RelayProviderError(f"cloudflare rejected request: {truncate(json.dumps(result.get('errors', [])))}")


def send_resend(email: OutboundEmail, timeout_seconds: int) -> None:
    token = os.getenv("RESEND_API_KEY", "").strip()
    if not token:
        raise RelayConfigError("RESEND_API_KEY is required")

    sender = formataddr((email.sender_name, email.sender_address)) if email.sender_name else email.sender_address
    payload: dict[str, Any] = {
        "from": sender,
        "to": email.to,
        "subject": email.subject,
    }
    if email.html:
        payload["html"] = email.html
    if email.text:
        payload["text"] = email.text
    if email.cc:
        payload["cc"] = email.cc
    if email.bcc:
        payload["bcc"] = email.bcc
    if email.reply_to:
        payload["reply_to"] = email.reply_to

    result = json_request("https://api.resend.com/emails", token, payload, timeout_seconds)
    if "error" in result and result["error"]:
        raise RelayProviderError(f"resend rejected request: {truncate(json.dumps(result['error']))}")


def send_message(email: OutboundEmail) -> None:
    provider = os.getenv("MAIL_RELAY_PROVIDER", "resend").strip().lower()
    timeout_seconds = env_int("MAIL_RELAY_TIMEOUT_SECONDS", 20)
    if provider == "cloudflare":
        send_cloudflare(email, timeout_seconds)
    elif provider == "resend":
        send_resend(email, timeout_seconds)
    elif provider in {"log", "dry-run", "dryrun"}:
        LOGGER.info("dry-run accepted mail to=%d subject=%r", len(email.to), email.subject)
    else:
        raise RelayConfigError(f"unsupported MAIL_RELAY_PROVIDER={provider!r}")


class RelayHandler:
    async def handle_DATA(self, server: Any, session: Any, envelope: Any) -> str:
        try:
            message = BytesParser(policy=policy.default).parsebytes(envelope.content)
            outbound = parse_outbound(message, envelope)
            await asyncio.to_thread(send_message, outbound)
            LOGGER.info(
                "sent mail provider=%s to=%d cc=%d bcc=%d subject=%r",
                os.getenv("MAIL_RELAY_PROVIDER", "resend").strip().lower(),
                len(outbound.to),
                len(outbound.cc),
                len(outbound.bcc),
                outbound.subject,
            )
            return "250 2.0.0 Message accepted for delivery"
        except RelayConfigError as exc:
            LOGGER.error("configuration error: %s", exc)
            return "451 4.3.0 Mail relay configuration error"
        except RelayProviderError as exc:
            LOGGER.error("provider error: %s", exc)
            return "451 4.3.0 Mail relay provider error"
        except Exception:
            LOGGER.exception("unexpected relay failure")
            return "451 4.3.0 Mail relay internal error"


class HealthHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args: Any) -> None:
        return

    def do_GET(self) -> None:
        if self.path not in {"/", "/health"}:
            self.send_response(404)
            self.end_headers()
            return
        payload = {
            "ok": True,
            "provider": os.getenv("MAIL_RELAY_PROVIDER", "resend").strip().lower(),
            "time": int(time.time()),
        }
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def start_health_server() -> ThreadingHTTPServer | None:
    port = env_int("MAIL_RELAY_HEALTH_PORT", 8080)
    if port <= 0:
        return None
    server = ThreadingHTTPServer(("0.0.0.0", port), HealthHandler)
    thread = threading.Thread(target=server.serve_forever, name="health-server", daemon=True)
    thread.start()
    LOGGER.info("health server listening on :%d", port)
    return server


def main() -> None:
    host = os.getenv("MAIL_RELAY_HOST", "0.0.0.0").strip() or "0.0.0.0"
    port = env_int("MAIL_RELAY_PORT", 1025)

    health_server = start_health_server()
    controller = Controller(RelayHandler(), hostname=host, port=port, ready_timeout=10)
    controller.start()
    LOGGER.info("smtp relay listening on %s:%d", host, port)

    stop_event = threading.Event()

    def stop(_signum: int, _frame: Any) -> None:
        stop_event.set()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    stop_event.wait()

    LOGGER.info("stopping mail relay")
    controller.stop()
    if health_server is not None:
        health_server.shutdown()
        health_server.server_close()
    socket.setdefaulttimeout(None)


if __name__ == "__main__":
    main()
