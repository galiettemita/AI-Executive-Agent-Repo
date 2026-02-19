from __future__ import annotations

import json
import time
import uuid
from typing import Any

from app.core.redis import get_redis

_MEMORY_SESSIONS: dict[str, tuple[float, dict[str, Any]]] = {}
_MEMORY_USER_INDEX: dict[str, set[str]] = {}


def _session_key(token: str) -> str:
    return f"bp:provision:session:{token}"


def _user_index_key(user_id: str) -> str:
    return f"bp:provision:user-sessions:{user_id}"


def create_provisioning_session(payload: dict[str, Any], *, ttl_seconds: int = 15 * 60) -> str:
    token = uuid.uuid4().hex
    ttl = max(60, int(ttl_seconds or 15 * 60))
    user_id = str((payload or {}).get("user_id") or "").strip()
    client = get_redis()
    if client is not None:
        client.set(_session_key(token), json.dumps(payload, ensure_ascii=False), ex=ttl)
        if user_id:
            client.sadd(_user_index_key(user_id), token)
            client.expire(_user_index_key(user_id), ttl)
        return token
    _MEMORY_SESSIONS[token] = (time.time() + ttl, dict(payload or {}))
    if user_id:
        _MEMORY_USER_INDEX.setdefault(user_id, set()).add(token)
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
            payload = get_provisioning_session(token) or {}
            user_id = str(payload.get("user_id") or "").strip()
            client.delete(key)
            if user_id:
                client.srem(_user_index_key(user_id), token)
        except Exception:
            pass
        return
    payload = (_MEMORY_SESSIONS.pop(token, None) or (None, {}))[1]
    user_id = str((payload or {}).get("user_id") or "").strip()
    if user_id and user_id in _MEMORY_USER_INDEX:
        _MEMORY_USER_INDEX[user_id].discard(token)
        if not _MEMORY_USER_INDEX[user_id]:
            _MEMORY_USER_INDEX.pop(user_id, None)


def delete_provisioning_sessions_for_user(user_id: str) -> int:
    uid = str(user_id or "").strip()
    if not uid:
        return 0

    deleted = 0
    client = get_redis()
    if client is not None:
        try:
            index_key = _user_index_key(uid)
            tokens = client.smembers(index_key) or []
            token_values = [
                token.decode("utf-8", errors="ignore") if isinstance(token, bytes) else str(token)
                for token in tokens
                if token
            ]
            for token in token_values:
                deleted += int(client.delete(_session_key(token)) or 0)
            client.delete(index_key)
        except Exception:
            return deleted
        return deleted

    tokens = set(_MEMORY_USER_INDEX.get(uid, set()))
    for token in tokens:
        if token in _MEMORY_SESSIONS:
            _MEMORY_SESSIONS.pop(token, None)
            deleted += 1
    _MEMORY_USER_INDEX.pop(uid, None)
    return deleted
