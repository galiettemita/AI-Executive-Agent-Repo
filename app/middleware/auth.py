from __future__ import annotations

import base64
import hashlib
import hmac
import json
import logging
import time
from typing import Optional

import jwt
from fastapi import Request
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

from app.core.config import settings
from app.core.log_context import set_user_id

logger = logging.getLogger(__name__)


ALLOWLIST_PATHS = {
    "/",
    "/health",
    "/health/live",
    "/health/ready",
    "/docs",
    "/openapi.json",
    "/redoc",
    "/admin/google/callback",
    "/metrics",
}

ALLOWLIST_PREFIXES = (
    "/webhooks/whatsapp",
    "/webhooks/voice",
    "/billing/stripe/webhook",
    "/payment/webhooks/stripe",
)

# Signature replay window
USER_SIG_MAX_AGE_SECONDS = 300


def _is_allowlisted(path: str) -> bool:
    if path in ALLOWLIST_PATHS:
        return True
    return any(path.startswith(prefix) for prefix in ALLOWLIST_PREFIXES)


def _verify_jwt(token: str) -> Optional[str]:
    if not token:
        return None
    if not settings.JWT_SECRET:
        return None
    try:
        payload = jwt.decode(token, settings.JWT_SECRET, algorithms=["HS256"])
        return str(payload.get("user_id") or payload.get("sub") or "")
    except Exception:
        return None


def _verify_user_signature(user_id: str, ts: str, sig: str) -> bool:
    if not settings.STATE_SIGNING_SECRET:
        return False
    if not user_id or not ts or not sig:
        return False
    try:
        ts_int = int(ts)
    except ValueError:
        return False
    now = int(time.time())
    if abs(now - ts_int) > USER_SIG_MAX_AGE_SECONDS:
        return False
    msg = f"{user_id}.{ts}".encode("utf-8")
    secret = settings.STATE_SIGNING_SECRET.encode("utf-8")
    expected = base64.urlsafe_b64encode(hmac.new(secret, msg, hashlib.sha256).digest()).decode("utf-8").rstrip("=")
    return hmac.compare_digest(expected, sig)


class AuthMiddleware(BaseHTTPMiddleware):
    """
    Enforces authentication for non-webhook endpoints in staging/production.

    Accepted auth methods:
    - Authorization: Bearer <JWT> (HS256, signed with JWT_SECRET)
    - X-User-ID, X-User-Timestamp, X-User-Signature (HMAC-SHA256 with STATE_SIGNING_SECRET)
    """

    async def dispatch(self, request: Request, call_next):
        path = request.url.path

        if request.method == "OPTIONS" or _is_allowlisted(path):
            return await call_next(request)

        # In dev, allow unauthenticated requests to keep local workflows simple.
        if settings.ENV == "dev":
            return await call_next(request)

        user_id: Optional[str] = None

        auth_header = request.headers.get("Authorization", "")
        if auth_header.startswith("Bearer "):
            token = auth_header.split(" ", 1)[1].strip()
            user_id = _verify_jwt(token)

        if not user_id:
            header_user_id = request.headers.get("X-User-ID", "")
            header_ts = request.headers.get("X-User-Timestamp", "")
            header_sig = request.headers.get("X-User-Signature", "")
            if _verify_user_signature(header_user_id, header_ts, header_sig):
                user_id = header_user_id

        if not user_id:
            return JSONResponse(
                status_code=401,
                content={
                    "error": "unauthorized",
                    "message": "Authentication required.",
                },
            )

        request.state.user_id = user_id
        set_user_id(user_id)

        # Optional: enforce that user_id in query/body matches authenticated user
        query_user_id = request.query_params.get("user_id")
        if query_user_id and query_user_id != user_id:
            return JSONResponse(
                status_code=403,
                content={"error": "forbidden", "message": "user_id mismatch"},
            )

        if request.method in {"POST", "PUT", "PATCH"}:
            content_type = request.headers.get("content-type", "")
            if "application/json" in content_type:
                try:
                    body = await request.body()
                    if body:
                        payload = json.loads(body)
                        if isinstance(payload, dict) and "user_id" in payload:
                            if str(payload["user_id"]) != user_id:
                                return JSONResponse(
                                    status_code=403,
                                    content={"error": "forbidden", "message": "user_id mismatch"},
                                )
                    # Re-attach body for downstream handlers
                    request._body = body
                except Exception:
                    # If parsing fails, continue; handler will validate input.
                    pass

        return await call_next(request)
