# backend/app/services/oauth_state.py

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
from datetime import datetime, timedelta, timezone


STATE_TTL_MINUTES = 20


def _require_env(name: str) -> str:
    val = os.getenv(name)
    if not val:
        raise RuntimeError(f"Missing required env var: {name}")
    return val


def _sign_state(payload_b64: str) -> str:
    secret = _require_env("STATE_SIGNING_SECRET").encode("utf-8")
    sig = hmac.new(secret, payload_b64.encode("utf-8"), hashlib.sha256).digest()
    return base64.urlsafe_b64encode(sig).decode("utf-8").rstrip("=")


def make_state(user_id: str) -> str:
    now = datetime.now(timezone.utc)
    payload = {
        "user_id": user_id,
        "iat": int(now.timestamp()),
        "exp": int((now + timedelta(minutes=STATE_TTL_MINUTES)).timestamp()),
    }
    payload_json = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    payload_b64 = base64.urlsafe_b64encode(payload_json).decode("utf-8").rstrip("=")
    sig = _sign_state(payload_b64)
    return f"{payload_b64}.{sig}"


def parse_state(state: str) -> str:
    try:
        payload_b64, sig = state.split(".", 1)
    except ValueError:
        raise ValueError("Invalid state format")

    expected = _sign_state(payload_b64)
    if not hmac.compare_digest(expected, sig):
        raise ValueError("Invalid state signature")

    padded = payload_b64 + "=" * (-len(payload_b64) % 4)
    payload = json.loads(base64.urlsafe_b64decode(padded.encode("utf-8")))
    now = int(datetime.now(timezone.utc).timestamp())
    if now > int(payload.get("exp", 0)):
        raise ValueError("State expired")
    return str(payload["user_id"])
