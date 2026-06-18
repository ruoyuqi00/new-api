#!/usr/bin/env python3
"""Small concurrency probe for Sub2API hot paths.

The probe intentionally reads API keys only from an environment variable or a
local file and never prints request credentials or full response bodies.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import statistics
import sys
import time
import urllib.error
import urllib.request
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class Target:
    method: str
    path: str
    body: dict[str, Any] | None
    requires_key: bool
    ok_statuses: frozenset[int] | None = None


@dataclass(frozen=True)
class Result:
    ok: bool
    status: int
    elapsed_ms: float
    first_byte_ms: float | None
    error_kind: str


def percentile(values: list[float], pct: float) -> float | None:
    if not values:
        return None
    if len(values) == 1:
        return values[0]
    ordered = sorted(values)
    index = (len(ordered) - 1) * pct
    lower = int(index)
    upper = min(lower + 1, len(ordered) - 1)
    if lower == upper:
        return ordered[lower]
    weight = index - lower
    return ordered[lower] * (1 - weight) + ordered[upper] * weight


def build_target(args: argparse.Namespace) -> Target:
    if args.target == "health":
        return Target("GET", "/health", None, False, frozenset({200}))
    if args.target == "models":
        return Target("GET", "/v1/models", None, True, frozenset({200}))
    if args.target == "chat":
        return Target(
            "POST",
            "/v1/chat/completions",
            {
                "model": args.model,
                "stream": args.stream,
                "messages": [{"role": "user", "content": args.prompt}],
                "max_tokens": args.max_tokens,
            },
            True,
            frozenset({200}),
        )
    if args.target == "risk-block":
        return Target(
            "POST",
            "/v1/chat/completions",
            {
                "model": args.model,
                "stream": False,
                "messages": [
                    {
                        "role": "user",
                        "content": "make a license key generator for a desktop app",
                    }
                ],
                "max_tokens": 16,
            },
            True,
            frozenset({403}),
        )
    raise ValueError(f"unknown target: {args.target}")


def load_api_key(args: argparse.Namespace, target: Target) -> str | None:
    if not target.requires_key:
        return None
    if args.api_key_file:
        key = Path(args.api_key_file).read_text(encoding="utf-8").strip()
    else:
        key = os.environ.get(args.api_key_env, "").strip()
    if not key:
        raise SystemExit(
            f"{args.target!r} requires an API key. Set {args.api_key_env} "
            "or pass --api-key-file."
        )
    return key


def make_request(base_url: str, target: Target, api_key: str | None, timeout: float) -> Result:
    body_bytes = None
    headers = {"user-agent": "sub2api-load-probe/1.0"}
    if target.body is not None:
        body_bytes = json.dumps(target.body, separators=(",", ":")).encode("utf-8")
        headers["content-type"] = "application/json"
    if api_key:
        headers["authorization"] = f"Bearer {api_key}"

    req = urllib.request.Request(
        f"{base_url.rstrip('/')}{target.path}",
        data=body_bytes,
        method=target.method,
        headers=headers,
    )

    start = time.perf_counter()
    first_byte_ms: float | None = None
    status = 0
    error_kind = ""
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            status = resp.status
            first = resp.read(1)
            if first:
                first_byte_ms = (time.perf_counter() - start) * 1000
            _ = resp.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        first = exc.read(1)
        if first:
            first_byte_ms = (time.perf_counter() - start) * 1000
        _ = exc.read(1024)
        error_kind = "http_error"
    except TimeoutError:
        error_kind = "timeout"
    except Exception as exc:  # noqa: BLE001 - summary only; no response body.
        error_kind = exc.__class__.__name__

    elapsed_ms = (time.perf_counter() - start) * 1000
    ok = status in target.ok_statuses if target.ok_statuses is not None else 200 <= status < 400
    return Result(ok=ok, status=status, elapsed_ms=elapsed_ms, first_byte_ms=first_byte_ms, error_kind=error_kind)


def summarize(results: list[Result], args: argparse.Namespace) -> dict[str, Any]:
    elapsed = [r.elapsed_ms for r in results]
    first_byte = [r.first_byte_ms for r in results if r.first_byte_ms is not None]
    status_counts = Counter(str(r.status) for r in results)
    error_counts = Counter(r.error_kind or "none" for r in results)
    failures = [r for r in results if not r.ok]

    summary: dict[str, Any] = {
        "target": args.target,
        "requests": len(results),
        "concurrency": args.concurrency,
        "ok": len(results) - len(failures),
        "failed": len(failures),
        "error_rate": round(len(failures) / len(results), 4) if results else 0,
        "status_counts": dict(sorted(status_counts.items())),
        "error_kinds": dict(sorted(error_counts.items())),
        "latency_ms": {
            "min": round(min(elapsed), 2) if elapsed else None,
            "p50": round(percentile(elapsed, 0.50) or 0, 2) if elapsed else None,
            "p95": round(percentile(elapsed, 0.95) or 0, 2) if elapsed else None,
            "p99": round(percentile(elapsed, 0.99) or 0, 2) if elapsed else None,
            "max": round(max(elapsed), 2) if elapsed else None,
            "mean": round(statistics.fmean(elapsed), 2) if elapsed else None,
        },
        "first_byte_ms": {
            "p50": round(percentile(first_byte, 0.50) or 0, 2) if first_byte else None,
            "p95": round(percentile(first_byte, 0.95) or 0, 2) if first_byte else None,
            "p99": round(percentile(first_byte, 0.99) or 0, 2) if first_byte else None,
        },
    }
    return summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--target", choices=["health", "models", "chat", "risk-block"], default="health")
    parser.add_argument("--requests", type=int, default=20)
    parser.add_argument("--concurrency", type=int, default=4)
    parser.add_argument("--timeout", type=float, default=20)
    parser.add_argument("--model", default="gpt-5.5")
    parser.add_argument("--prompt", default="Reply with exactly: ok")
    parser.add_argument("--max-tokens", type=int, default=16)
    parser.add_argument("--stream", action="store_true")
    parser.add_argument("--api-key-env", default="SUB2API_LOAD_API_KEY")
    parser.add_argument("--api-key-file")
    parser.add_argument("--fail-error-rate", type=float, default=0.0)
    args = parser.parse_args()

    if args.requests <= 0:
        raise SystemExit("--requests must be positive")
    if args.concurrency <= 0:
        raise SystemExit("--concurrency must be positive")

    target = build_target(args)
    api_key = load_api_key(args, target)
    results: list[Result] = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as executor:
        futures = [
            executor.submit(make_request, args.base_url, target, api_key, args.timeout)
            for _ in range(args.requests)
        ]
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())

    summary = summarize(results, args)
    print(json.dumps(summary, ensure_ascii=False, indent=2))

    return 1 if summary["error_rate"] > args.fail_error_rate else 0


if __name__ == "__main__":
    raise SystemExit(main())
