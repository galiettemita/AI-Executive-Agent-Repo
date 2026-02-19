#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
from hashlib import sha256

import jwt
import requests


def _request_bucket(*, task_type: str, first_user: str, tools: list[str] | None = None) -> int:
    payload = json.dumps(
        {
            "task_type": task_type,
            "first_user": first_user[:400],
            "tools": list(tools or []),
        },
        sort_keys=True,
        ensure_ascii=False,
    )
    digest = sha256(payload.encode("utf-8")).hexdigest()
    return int(digest[:8], 16) % 100


def _pick_recovery_prompt(*, task_type: str = "general", min_bucket: int = 10) -> tuple[str, int]:
    for idx in range(5000):
        message = f"staging failover recovery probe #{idx}"
        bucket = _request_bucket(task_type=task_type, first_user=message, tools=[])
        if bucket >= min_bucket:
            return message, bucket
    raise RuntimeError("failed to find deterministic recovery probe prompt")


def _call(
    session: requests.Session,
    *,
    method: str,
    base_url: str,
    path: str,
    token: str,
    payload: dict | None = None,
) -> dict:
    url = f"{base_url.rstrip('/')}{path}"
    headers = {"Authorization": f"Bearer {token}"}
    if method == "GET":
        resp = session.get(url, headers=headers, timeout=30)
    elif method == "POST":
        resp = session.post(url, headers=headers, json=payload or {}, timeout=30)
    elif method == "DELETE":
        resp = session.delete(url, headers=headers, timeout=30)
    else:
        raise ValueError(f"Unsupported method: {method}")
    if resp.status_code >= 400:
        raise RuntimeError(f"{method} {path} failed status={resp.status_code} body={resp.text[:500]}")
    return resp.json()


def _set_override(
    session: requests.Session,
    *,
    base_url: str,
    token: str,
    provider: str,
    healthy: bool,
    ttl_seconds: int,
) -> dict:
    return _call(
        session,
        method="POST",
        base_url=base_url,
        path="/internal/llm/health/override",
        token=token,
        payload={"provider": provider, "healthy": healthy, "ttl_seconds": ttl_seconds},
    )


def _clear_override(
    session: requests.Session,
    *,
    base_url: str,
    token: str,
    provider: str,
) -> dict:
    return _call(
        session,
        method="DELETE",
        base_url=base_url,
        path=f"/internal/llm/health/override/{provider}",
        token=token,
    )


def _route_test(
    session: requests.Session,
    *,
    base_url: str,
    token: str,
    user_id: str,
    message: str,
    task_type: str = "general",
    force_refresh: bool = False,
) -> dict:
    payload = {
        "user_id": user_id,
        "task_type": task_type,
        "messages": [{"role": "user", "content": message}],
        "force_refresh": bool(force_refresh),
    }
    result = _call(
        session,
        method="POST",
        base_url=base_url,
        path="/internal/llm/route-test",
        token=token,
        payload=payload,
    )
    return result["route"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Verify staging LLM failover + recovery ramp behavior.")
    parser.add_argument("--base-url", required=True, help="Staging base URL (e.g., http://<alb-dns>)")
    parser.add_argument("--jwt-secret", required=True, help="JWT secret used by staging auth middleware")
    parser.add_argument("--user-id", default="staging-failover-check", help="Synthetic user ID for checks")
    parser.add_argument("--ttl-seconds", type=int, default=300, help="Override TTL for staged outage simulation")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    now = int(time.time())
    token = jwt.encode(
        {
            "user_id": args.user_id,
            "role": "admin",
            "iat": now,
            "exp": now + 900,
        },
        args.jwt_secret,
        algorithm="HS256",
    )

    session = requests.Session()
    providers = ("openai", "anthropic", "google", "local")
    summary: dict[str, object] = {"base_url": args.base_url, "user_id": args.user_id}

    try:
        _call(
            session,
            method="GET",
            base_url=args.base_url,
            path="/internal/llm/health?force_refresh=true",
            token=token,
        )

        for provider in ("openai", "anthropic", "google"):
            _set_override(
                session,
                base_url=args.base_url,
                token=token,
                provider=provider,
                healthy=False,
                ttl_seconds=args.ttl_seconds,
            )
        _set_override(
            session,
            base_url=args.base_url,
            token=token,
            provider="local",
            healthy=True,
            ttl_seconds=args.ttl_seconds,
        )

        degraded_route = _route_test(
            session,
            base_url=args.base_url,
            token=token,
            user_id=args.user_id,
            message="degraded-mode routing probe",
            task_type="single_tool_call",
            force_refresh=True,
        )
        summary["degraded_route"] = degraded_route
        if degraded_route.get("system_mode") != "degraded":
            raise RuntimeError(f"expected degraded mode, got {degraded_route}")
        if degraded_route.get("selected_provider") != "local":
            raise RuntimeError(f"expected local provider in degraded mode, got {degraded_route}")

        for provider in providers:
            _clear_override(
                session,
                base_url=args.base_url,
                token=token,
                provider=provider,
            )

        probe_message, probe_bucket = _pick_recovery_prompt(task_type="general", min_bucket=10)
        recovery_route = _route_test(
            session,
            base_url=args.base_url,
            token=token,
            user_id=args.user_id,
            message=probe_message,
            task_type="general",
            force_refresh=True,
        )
        summary["recovery_route"] = recovery_route
        summary["recovery_probe_bucket"] = probe_bucket

        if recovery_route.get("system_mode") != "normal":
            raise RuntimeError(f"expected normal mode after recovery, got {recovery_route}")
        if bool(recovery_route.get("recovery_forced_local")) is not True:
            raise RuntimeError(f"expected recovery_forced_local=true during early ramp, got {recovery_route}")

    finally:
        for provider in providers:
            try:
                _clear_override(
                    session,
                    base_url=args.base_url,
                    token=token,
                    provider=provider,
                )
            except Exception:
                pass

    print(json.dumps({"ok": True, **summary}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
