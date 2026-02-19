from __future__ import annotations

import json
import threading
import time
import uuid
from typing import Any

from app.core.redis import get_redis

_QUEUE_KEY = "bp:llm:degraded:retry_queue"
_QUEUE_TTL_SECONDS = 60 * 60 * 24  # 24h
_NOTICE_TTL_SECONDS = 60 * 30  # 30m

_mem_lock = threading.Lock()
_mem_queue: list[dict[str, Any]] = []
_mem_seen_runs: set[str] = set()
_mem_notice_until: dict[str, float] = {}


def _dedupe_key(run_id: str) -> str:
    return f"bp:llm:degraded:retry:seen:{run_id}"


def enqueue_degraded_retry(
    *,
    run_id: str | None,
    user_id: str | None,
    conversation_id: str | None,
    user_text: str,
    tier: int,
    task_type: str,
    reason: str,
) -> dict[str, Any]:
    """
    Persist a degraded-mode retry job so a worker can replay it later.
    """
    now = int(time.time())
    job = {
        "id": str(uuid.uuid4()),
        "queued_at": now,
        "run_id": (run_id or "").strip() or None,
        "user_id": (user_id or "").strip() or None,
        "conversation_id": (conversation_id or "").strip() or None,
        "user_text": str(user_text or ""),
        "tier": int(tier),
        "task_type": str(task_type or "general"),
        "reason": str(reason or "degraded_mode"),
        "state": "queued",
    }

    rid = str(job.get("run_id") or "").strip()
    client = get_redis()
    if client is not None:
        if rid:
            inserted = client.set(_dedupe_key(rid), job["id"], ex=_QUEUE_TTL_SECONDS, nx=True)
            if not inserted:
                return {
                    "queued": False,
                    "duplicate": True,
                    "reason": "already_queued",
                    "run_id": rid,
                }
        payload = json.dumps(job, ensure_ascii=False)
        position = int(client.rpush(_QUEUE_KEY, payload))
        client.expire(_QUEUE_KEY, _QUEUE_TTL_SECONDS)
        return {"queued": True, "job_id": job["id"], "queue_position": position, "job": job}

    with _mem_lock:
        if rid and rid in _mem_seen_runs:
            return {
                "queued": False,
                "duplicate": True,
                "reason": "already_queued",
                "run_id": rid,
            }
        if rid:
            _mem_seen_runs.add(rid)
        _mem_queue.append(job)
        return {"queued": True, "job_id": job["id"], "queue_position": len(_mem_queue), "job": job}


def should_send_degraded_notice(user_id: str | None) -> bool:
    """
    Returns True exactly once per user per cooldown window.
    """
    scope = (user_id or "anonymous").strip() or "anonymous"
    key = f"bp:llm:degraded:notice:{scope}"
    client = get_redis()
    if client is not None:
        return bool(client.set(key, "1", nx=True, ex=_NOTICE_TTL_SECONDS))

    now = time.time()
    with _mem_lock:
        until = float(_mem_notice_until.get(scope, 0))
        if until > now:
            return False
        _mem_notice_until[scope] = now + _NOTICE_TTL_SECONDS
        return True
