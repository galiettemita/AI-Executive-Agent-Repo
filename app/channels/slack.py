from __future__ import annotations

import hashlib
import hmac
import re
import time
from typing import Any


_MAX_SLACK_CHARS = 3500
_TRIPLE_TICKS = re.compile(r"```")
_HEADING_RE = re.compile(r"^\s*#{1,6}\s*", re.MULTILINE)


def normalize_slack_event(payload: dict[str, Any]) -> dict[str, Any] | None:
    if not isinstance(payload, dict):
        return None
    event = payload.get("event") if isinstance(payload.get("event"), dict) else payload
    if not isinstance(event, dict):
        return None
    if str(event.get("type") or "") != "message":
        return None
    if event.get("bot_id"):
        return None

    text = str(event.get("text") or "").strip()
    sender = str(event.get("user") or "").strip()
    channel = str(event.get("channel") or "").strip()
    ts = str(event.get("ts") or payload.get("event_id") or "").strip()
    if not sender or not channel:
        return None

    return {
        "external_id": ts or f"slack-{channel}-{sender}",
        "from": sender,
        "channel_id": channel,
        "text": text,
        "type": "text",
        "raw": payload,
    }


def apply_slack_constraints(text: str) -> str:
    cleaned = str(text or "").strip()
    if not cleaned:
        return ""
    cleaned = _HEADING_RE.sub("", cleaned)
    cleaned = _TRIPLE_TICKS.sub("", cleaned)
    if len(cleaned) > _MAX_SLACK_CHARS:
        cleaned = cleaned[: _MAX_SLACK_CHARS - 1].rstrip() + "…"
    return cleaned


def verify_slack_signature(
    *,
    raw_body: bytes,
    signature: str,
    request_ts: str,
    signing_secret: str,
    tolerance_seconds: int = 300,
) -> bool:
    if not signing_secret:
        return False
    if not signature or not request_ts:
        return False
    try:
        ts_int = int(request_ts)
    except Exception:
        return False
    now = int(time.time())
    if abs(now - ts_int) > int(tolerance_seconds):
        return False
    base = f"v0:{request_ts}:{raw_body.decode('utf-8')}".encode("utf-8")
    expected = "v0=" + hmac.new(signing_secret.encode("utf-8"), base, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
