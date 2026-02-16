from __future__ import annotations

import json
import threading
import time
from collections import defaultdict
from typing import Any

from app.core.redis import get_redis


_PROGRESS_TTL_SECONDS = 60 * 60
_RESULT_TTL_SECONDS = 60 * 60

_lock = threading.Lock()
_mem_events: dict[str, list[dict[str, Any]]] = defaultdict(list)
_mem_results: dict[str, dict[str, Any]] = {}


def _progress_key(run_id: str) -> str:
    return f"bp:progress:{run_id}"


def _result_key(run_id: str) -> str:
    return f"bp:run-result:{run_id}"


def append_progress(
    run_id: str,
    *,
    step: str,
    status: str,
    partial_result: dict[str, Any] | None = None,
    metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    event = {
        "ts": int(time.time() * 1000),
        "step": step,
        "status": status,
        "partial_result": partial_result or {},
        "metadata": metadata or {},
    }

    client = get_redis()
    if client is not None:
        payload = json.dumps(event, ensure_ascii=False)
        key = _progress_key(run_id)
        client.rpush(key, payload)
        client.expire(key, _PROGRESS_TTL_SECONDS)
        return event

    with _lock:
        _mem_events[run_id].append(event)
    return event


def get_progress_events(run_id: str, *, after_index: int = 0) -> tuple[list[dict[str, Any]], int]:
    client = get_redis()
    if client is not None:
        key = _progress_key(run_id)
        raw_items = client.lrange(key, after_index, -1)
        events = []
        for raw in raw_items or []:
            try:
                events.append(json.loads(raw))
            except Exception:
                continue
        new_index = after_index + len(events)
        return events, new_index

    with _lock:
        rows = list(_mem_events.get(run_id, []))
    sliced = rows[after_index:]
    return sliced, after_index + len(sliced)


def set_run_result(run_id: str, payload: dict[str, Any]) -> None:
    client = get_redis()
    if client is not None:
        key = _result_key(run_id)
        client.set(key, json.dumps(payload, ensure_ascii=False), ex=_RESULT_TTL_SECONDS)
        return

    with _lock:
        _mem_results[run_id] = payload


def get_run_result(run_id: str) -> dict[str, Any] | None:
    client = get_redis()
    if client is not None:
        raw = client.get(_result_key(run_id))
        if not raw:
            return None
        try:
            return json.loads(raw)
        except Exception:
            return None

    with _lock:
        return _mem_results.get(run_id)
