#!/usr/bin/env python3
"""Validate M7 dashboard signals (latency, error rate, tier mix) in Axiom."""

from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import requests


@dataclass
class AxiomConfig:
    token: str
    dataset: str


def _run(cmd: list[str]) -> str:
    return subprocess.check_output(cmd, text=True).strip()


def _load_stage_secret(stage: str) -> dict[str, Any]:
    secret_id = f"executive-os/{stage}/app"
    raw = _run(
        [
            "aws",
            "secretsmanager",
            "get-secret-value",
            "--region",
            "us-east-1",
            "--secret-id",
            secret_id,
            "--query",
            "SecretString",
            "--output",
            "text",
        ]
    )
    return json.loads(raw)


def _parse_otlp_headers(raw: str | None) -> dict[str, str]:
    out: dict[str, str] = {}
    if not raw:
        return out
    for item in raw.split(","):
        if "=" not in item:
            continue
        key, value = item.split("=", 1)
        key = key.strip()
        value = value.strip()
        if key:
            out[key] = value
    return out


def _axiom_config_from_secret(secret: dict[str, Any]) -> AxiomConfig:
    raw_headers = secret.get("OTEL_EXPORTER_OTLP_TRACES_HEADERS") or secret.get("OTEL_EXPORTER_OTLP_HEADERS")
    parsed = _parse_otlp_headers(raw_headers)
    auth = parsed.get("Authorization", "")
    dataset = parsed.get("X-Axiom-Dataset", "")
    if not auth.startswith("Bearer ") or not dataset:
        raise RuntimeError("OTEL headers are missing Authorization or X-Axiom-Dataset")
    token = auth.removeprefix("Bearer ").strip()
    return AxiomConfig(token=token, dataset=dataset)


def _query_dataset(cfg: AxiomConfig, start_time: str, end_time: str) -> list[dict[str, Any]]:
    url = f"https://api.axiom.co/v1/datasets/{cfg.dataset}/query"
    headers = {
        "Authorization": f"Bearer {cfg.token}",
        "Content-Type": "application/json",
    }
    body = {"startTime": start_time, "endTime": end_time}
    resp = requests.post(url, headers=headers, json=body, timeout=30)
    resp.raise_for_status()
    payload = resp.json()
    return payload.get("matches", []) or []


_DURATION_RE = re.compile(r"^([0-9]+(?:\.[0-9]+)?)(ns|us|ms|s)$")


def _duration_to_ms(raw: str | None) -> float | None:
    if not raw:
        return None
    m = _DURATION_RE.match(raw.strip())
    if not m:
        return None
    value = float(m.group(1))
    unit = m.group(2)
    if unit == "ns":
        return value / 1_000_000
    if unit == "us":
        return value / 1_000
    if unit == "ms":
        return value
    if unit == "s":
        return value * 1_000
    return None


def _p95(values: list[float]) -> float | None:
    if not values:
        return None
    values = sorted(values)
    rank = int(math.ceil(0.95 * len(values))) - 1
    rank = max(0, min(rank, len(values) - 1))
    return values[rank]


def _to_int(value: Any) -> int | None:
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def analyze(matches: list[dict[str, Any]]) -> dict[str, Any]:
    http_durations: list[float] = []
    status_codes: list[int] = []
    tier_counts: dict[int, int] = {}
    tier_span_count = 0

    for row in matches:
        data = row.get("data", {}) or {}
        attrs = (data.get("attributes", {}) or {}).get("custom", {}) or {}
        span_name = str(data.get("name") or "")

        duration_ms = _duration_to_ms(data.get("duration"))
        status_code = _to_int(attrs.get("http.status_code"))

        if duration_ms is not None and status_code is not None:
            http_durations.append(duration_ms)
            status_codes.append(status_code)

        tier = _to_int(attrs.get("exec.tier"))
        if tier is not None:
            tier_span_count += 1
            tier_counts[tier] = tier_counts.get(tier, 0) + 1

    total_http = len(status_codes)
    errors_5xx = sum(1 for code in status_codes if code >= 500)

    return {
        "rows_sampled": len(matches),
        "http_spans": total_http,
        "latency_p95_ms": _p95(http_durations),
        "error_rate_5xx": (errors_5xx / total_http) if total_http else None,
        "tier_counts": tier_counts,
        "tier_spans": tier_span_count,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate Axiom telemetry for M7 dashboard readiness.")
    parser.add_argument("--stage", choices=["staging", "prod"], default="prod")
    parser.add_argument("--hours", type=int, default=2, help="Lookback window in hours")
    parser.add_argument("--lookback", default=None, help="Override startTime with a relative offset like 10m, 2h, 7d")
    args = parser.parse_args()

    now = datetime.now(timezone.utc)
    start_time = f"now-{args.lookback}" if args.lookback else f"now-{args.hours}h"
    end_time = "now"

    secret = _load_stage_secret(args.stage)
    cfg = _axiom_config_from_secret(secret)
    matches = _query_dataset(cfg, start_time=start_time, end_time=end_time)
    results = analyze(matches)

    print(f"stage: {args.stage}")
    print(f"dataset: {cfg.dataset}")
    print(f"window: {start_time} to {end_time} ({now.isoformat()})")
    print(f"rows_sampled: {results['rows_sampled']}")
    print(f"http_spans: {results['http_spans']}")
    print(f"latency_p95_ms: {results['latency_p95_ms']}")
    print(f"error_rate_5xx: {results['error_rate_5xx']}")
    print(f"tier_spans: {results['tier_spans']}")
    print(f"tier_counts: {results['tier_counts']}")

    if results["http_spans"] == 0:
        print("FAIL: no HTTP spans found.")
        return 1
    if results["tier_spans"] == 0:
        print("FAIL: no spans with exec.tier found.")
        return 2

    print("PASS: M7 dashboard signals are present (latency, error rate, tier mix).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
