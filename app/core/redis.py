from __future__ import annotations

import json
import logging
import time
from typing import Any

import redis

from app.core.config import settings

logger = logging.getLogger(__name__)

_redis_client: redis.Redis | None = None
_mem_cache: dict[str, tuple[float | None, str]] = {}


def get_redis() -> redis.Redis | None:
    """
    Return a singleton Redis client if configured, otherwise None.
    Uses a connection pool and enables health checks.
    """
    global _redis_client

    if not settings.REDIS_URL:
        return None

    if _redis_client is None:
        _redis_client = redis.from_url(
            settings.REDIS_URL,
            decode_responses=True,
            socket_timeout=2,
            health_check_interval=30,
        )
    return _redis_client


def redis_ping() -> bool:
    client = get_redis()
    if not client:
        return False
    client.ping()
    return True


def cache_get_json(key: str) -> Any | None:
    client = get_redis()
    if not client:
        item = _mem_cache.get(key)
        if not item:
            return None
        expires_at, raw = item
        if expires_at is not None and expires_at <= time.time():
            _mem_cache.pop(key, None)
            return None
        try:
            return json.loads(raw)
        except Exception:
            return None
    try:
        raw = client.get(key)
        if raw is None:
            return None
        return json.loads(raw)
    except Exception as exc:
        logger.warning("Redis cache get failed for key=%s: %s", key, exc)
        return None


def cache_set_json(key: str, value: Any, ttl_seconds: int | None = None) -> None:
    client = get_redis()
    if not client:
        try:
            payload = json.dumps(value, ensure_ascii=False)
            expires_at = time.time() + int(ttl_seconds) if ttl_seconds else None
            _mem_cache[key] = (expires_at, payload)
        except Exception:
            pass
        return
    try:
        payload = json.dumps(value, ensure_ascii=False)
        if ttl_seconds:
            client.set(key, payload, ex=ttl_seconds)
        else:
            client.set(key, payload)
    except Exception as exc:
        logger.warning("Redis cache set failed for key=%s: %s", key, exc)


def cache_delete(key: str) -> None:
    client = get_redis()
    if not client:
        _mem_cache.pop(key, None)
        return
    try:
        client.delete(key)
    except Exception as exc:
        logger.warning("Redis cache delete failed for key=%s: %s", key, exc)
