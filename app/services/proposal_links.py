# backend/app/services/proposal_links.py

from __future__ import annotations

import base64
import hmac
import json
import os
import time
from hashlib import sha256
from typing import Dict


def _secret() -> bytes:
    return os.getenv("JWT_SECRET", "dev_only_change_me").encode("utf-8")


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("utf-8").rstrip("=")


def _b64url_decode(data: str) -> bytes:
    pad = "=" * (-len(data) % 4)
    return base64.urlsafe_b64decode(data + pad)


def sign_token(payload: Dict, *, ttl_seconds: int = 900) -> str:
    body = payload.copy()
    body["exp"] = int(time.time()) + ttl_seconds
    raw = json.dumps(body, separators=(",", ":"), sort_keys=True).encode("utf-8")
    sig = hmac.new(_secret(), raw, sha256).digest()
    return _b64url(raw) + "." + _b64url(sig)


def verify_token(token: str) -> Dict | None:
    try:
        raw_b64, sig_b64 = token.split(".", 1)
        raw = _b64url_decode(raw_b64)
        sig = _b64url_decode(sig_b64)
        expected = hmac.new(_secret(), raw, sha256).digest()
        if not hmac.compare_digest(sig, expected):
            return None
        data = json.loads(raw.decode("utf-8"))
        if int(data.get("exp", 0)) < int(time.time()):
            return None
        return data
    except Exception:
        return None
