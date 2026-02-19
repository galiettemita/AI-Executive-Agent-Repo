from __future__ import annotations

import json
import time
import uuid
from typing import Any

from app.core.redis import get_redis

_MEMORY_SESSIONS: dict[str, tuple[float, dict[str, Any]]] = {}


def _session_key(token: str) -> str:
    return f"bp:provision:session:{token}"


def create_provisioning_session(payload: dict[str, Any], *, ttl_seconds: int = 15 * 60) -> str:
    token = uuid.uuid4().hex
    ttl = max(60, int(ttl_seconds or 15 * 60))
    client = get_redis()
    if client is not None:
        client.set(_session_key(token), json.dumps(payload, ensure_ascii=False), ex=ttl)
        return token
    _MEMORY_SESSIONS[token] = (time.time() + ttl, dict(payload or {}))
    return token


def get_provisioning_session(token: str) -> dict[str, Any] | None:
    key = _session_key(token)
    client = get_redis()
    if client is not None:
        raw = client.get(key)
        if not raw:
            return None
        if isinstance(raw, bytes):
            raw = raw.decode("utf-8", errors="ignore")
        try:
            parsed = json.loads(str(raw))
            return parsed if isinstance(parsed, dict) else None
        except Exception:
            return None

    item = _MEMORY_SESSIONS.get(token)
    if not item:
        return None
    expires_at, payload = item
    if expires_at <= time.time():
        _MEMORY_SESSIONS.pop(token, None)
        return None
    return dict(payload or {})


def delete_provisioning_session(token: str) -> None:
    key = _session_key(token)
    client = get_redis()
    if client is not None:
        try:
            client.delete(key)
        except Exception:
            pass
        return
    _MEMORY_SESSIONS.pop(token, None)
