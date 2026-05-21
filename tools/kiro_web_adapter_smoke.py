#!/usr/bin/env python3
"""Smoke test the Kiro Web adapter without printing API keys."""

from __future__ import annotations

import argparse
import json
import urllib.error
import urllib.request
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:8991")
    parser.add_argument("--api-key-file", default="/config/generated-kiro-api-key.txt")
    parser.add_argument("--model", default="claude-opus-4.7")
    parser.add_argument("--prompt", default="Reply with exactly: adapter-ok")
    parser.add_argument("--mode", choices=["anthropic", "openai"], default="anthropic")
    parser.add_argument("--stream", action="store_true")
    args = parser.parse_args()

    key = Path(args.api_key_file).read_text(encoding="utf-8").strip()
    if args.mode == "openai":
        endpoint = "/v1/chat/completions"
        payload = {
            "model": args.model,
            "stream": args.stream,
            "messages": [{"role": "user", "content": args.prompt}],
        }
    else:
        endpoint = "/v1/messages"
        payload = {
            "model": args.model,
            "max_tokens": 64,
            "stream": args.stream,
            "messages": [{"role": "user", "content": args.prompt}],
        }
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        f"{args.base_url.rstrip('/')}{endpoint}",
        data=body,
        method="POST",
        headers={"content-type": "application/json", "x-api-key": key},
    )
    try:
        resp_ctx = urllib.request.urlopen(req, timeout=150)
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode("utf-8", "replace")
        print(json.dumps({"status": exc.code, "error_body": body_text[:500]}, ensure_ascii=False))
        return 1

    with resp_ctx as resp:
        if args.stream:
            chunks = 0
            text = ""
            for raw_line in resp:
                line = raw_line.decode("utf-8", "replace").strip()
                if not line.startswith("data:"):
                    continue
                data = line[5:].strip()
                if not data:
                    continue
                if data == "[DONE]":
                    break
                chunks += 1
                try:
                    item = json.loads(data)
                except json.JSONDecodeError:
                    continue
                if args.mode == "openai":
                    choices = item.get("choices")
                    if isinstance(choices, list) and choices:
                        delta = choices[0].get("delta")
                        if isinstance(delta, dict) and isinstance(delta.get("content"), str):
                            text += delta["content"]
                else:
                    if item.get("type") == "message_stop":
                        break
                    delta = item.get("delta")
                    if isinstance(delta, dict) and isinstance(delta.get("text"), str):
                        text += delta["text"]
                if len(text) >= 120:
                    break
            print(
                json.dumps(
                    {"status": resp.status, "model": args.model, "chunks": chunks, "text": text[:160]},
                    ensure_ascii=False,
                )
            )
            return 0 if chunks > 0 else 1

        data = json.loads(resp.read().decode("utf-8"))
        text = ""
        if args.mode == "openai":
            choices = data.get("choices")
            if isinstance(choices, list) and choices and isinstance(choices[0], dict):
                message = choices[0].get("message")
                if isinstance(message, dict):
                    text = str(message.get("content") or "")
        else:
            content = data.get("content")
            if isinstance(content, list) and content and isinstance(content[0], dict):
                text = str(content[0].get("text") or "")
        print(
            json.dumps(
                {"status": resp.status, "model": data.get("model"), "text": text[:160]},
                ensure_ascii=False,
            )
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
