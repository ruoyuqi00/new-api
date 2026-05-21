#!/usr/bin/env python3
"""Sanitized server-side diagnostics for the Kiro Web adapter."""

from __future__ import annotations

import json
import sys

sys.path.insert(0, "/app")

import kiro_web_adapter as kwa  # noqa: E402


def main() -> int:
    client = kwa.KiroWebClient("/config/credentials.json")
    cred = client.ensure_access_token()
    access_token = kwa.pick(cred, "accessToken", "access_token")
    profile_arn = kwa.pick(cred, "profileArn", "profile_arn")
    visitor_id = "debug-" + kwa.sha256_hex(str(access_token))[:12]
    idp_statuses = []
    for idp in ["Google", "Github", "BuilderId"]:
        body = {"origin": "KIRO_IDE"}
        if profile_arn:
            body["profileArn"] = profile_arn
        headers = client.portal_headers(
            access_token,
            idp,
            None,
            None,
            None,
            visitor_id,
            "application/cbor",
        )
        status, resp_headers, raw = kwa.http_json_request(
            "POST",
            kwa.PORTAL_BASE + "/GetUserInfo",
            headers,
            kwa.cbor_encode(body),
            30,
        )
        decoded_keys = []
        if status == 200:
            try:
                decoded = kwa.cbor_decode(raw)
                if isinstance(decoded, dict):
                    decoded_keys = sorted(str(k) for k in decoded.keys())
            except Exception:
                decoded_keys = ["<decode-failed>"]
        idp_statuses.append({"idp": idp, "status": status, "keys": decoded_keys})
    print(json.dumps({"get_user_info": idp_statuses}, ensure_ascii=False))
    session = client.prepare_session()
    print(
        json.dumps(
            {
                "idp": session.idp,
                "csrf_len": len(session.csrf_token or ""),
                "has_user_id": bool(session.user_id),
                "profile_tail": (session.profile_arn or "")[-16:],
                "cookie_names": [
                    p.split("=", 1)[0]
                    for p in (session.cookie_header or "").split("; ")
                    if "=" in p
                ],
            },
            ensure_ascii=False,
        )
    )
    body = {"spaceType": "VIBE"}
    if session.profile_arn:
        body["profileArn"] = session.profile_arn
    encoded = kwa.cbor_encode(body)
    headers = client.portal_headers(
        session.access_token,
        session.idp,
        session.csrf_token,
        session.cookie_header,
        session.user_id,
        session.visitor_id,
        "application/cbor",
    )
    status, resp_headers, raw = kwa.http_json_request(
        "POST",
        kwa.PORTAL_BASE + "/CreateSpace",
        headers,
        encoded,
        45,
    )
    lower_headers = {k.lower(): v for k, v in resp_headers.items()}
    print(
        json.dumps(
            {
                "status": status,
                "content_type": resp_headers.get("content-type")
                or resp_headers.get("Content-Type"),
                "x_error_type": lower_headers.get("x-amzn-errortype", ""),
                "body_head": raw[:300].decode("utf-8", "replace"),
            },
            ensure_ascii=False,
        )
    )
    return 0 if status == 200 else 1


if __name__ == "__main__":
    raise SystemExit(main())
