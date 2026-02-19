from __future__ import annotations

import time
import uuid

from app.core.config import settings
from app.core.redis import get_redis

_MEMORY_SHORT_URLS: dict[str, tuple[float, str]] = {}


def _short_key(token: str) -> str:
    return f"bp:short:url:{token}"


def shorten_url(url: str, *, ttl_seconds: int = 15 * 60) -> dict[str, str]:
    token = uuid.uuid4().hex[:12]
    ttl = max(60, int(ttl_seconds or 15 * 60))
    client = get_redis()
    if client is not None:
        client.set(_short_key(token), str(url or ""), ex=ttl)
    else:
        _MEMORY_SHORT_URLS[token] = (time.time() + ttl, str(url or ""))
    base = (settings.APP_BASE_URL or "").rstrip("/")
    short_url = f"{base}/api/v1/provision/short/{token}" if base else f"/api/v1/provision/short/{token}"
    return {"token": token, "short_url": short_url}


def resolve_short_url(token: str) -> str | None:
    client = get_redis()
    if client is not None:
        raw = client.get(_short_key(token))
        if not raw:
            return None
        if isinstance(raw, bytes):
            return raw.decode("utf-8", errors="ignore")
        return str(raw)

    item = _MEMORY_SHORT_URLS.get(token)
    if not item:
        return None
    expires_at, value = item
    if expires_at <= time.time():
        _MEMORY_SHORT_URLS.pop(token, None)
        return None
    return value
