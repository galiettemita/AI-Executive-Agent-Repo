#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import statistics
import time

import requests


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--api-key", default=os.getenv("TAVILY_API_KEY", ""))
    parser.add_argument(
        "--queries",
        nargs="*",
        default=[
            "best carry-on luggage with laptop compartment",
            "noise cancelling headphones under 300",
            "best espresso machine for small kitchen",
        ],
    )
    args = parser.parse_args()

    if not args.api_key:
        raise SystemExit("Missing Tavily API key. Set --api-key or TAVILY_API_KEY.")

    latencies_ms = []
    ok = 0
    for query in args.queries:
        t0 = time.perf_counter()
        resp = requests.post(
            "https://api.tavily.com/search",
            json={
                "api_key": args.api_key,
                "query": query,
                "search_depth": "basic",
                "max_results": 5,
                "include_answer": False,
                "include_raw_content": False,
            },
            timeout=20,
        )
        dt = (time.perf_counter() - t0) * 1000
        latencies_ms.append(dt)
        if resp.ok:
            body = resp.json()
            ok += 1
            print(
                json.dumps(
                    {
                        "query": query,
                        "status": "ok",
                        "results": len(body.get("results", []) or []),
                        "latency_ms": round(dt, 1),
                    }
                )
            )
        else:
            print(
                json.dumps(
                    {
                        "query": query,
                        "status": f"error_{resp.status_code}",
                        "latency_ms": round(dt, 1),
                        "body": resp.text[:160],
                    }
                )
            )

    p50 = statistics.median(latencies_ms) if latencies_ms else 0
    ordered = sorted(latencies_ms)
    idx = min(len(ordered) - 1, int((len(ordered) - 1) * 0.95)) if ordered else 0
    p95 = ordered[idx] if ordered else 0
    print(
        json.dumps(
            {
                "ok_queries": ok,
                "total_queries": len(args.queries),
                "p50_ms": round(p50, 1),
                "p95_ms": round(p95, 1),
            }
        )
    )
    return 0 if ok == len(args.queries) else 1


if __name__ == "__main__":
    raise SystemExit(main())
