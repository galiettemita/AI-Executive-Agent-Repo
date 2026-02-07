from __future__ import annotations

import json
import logging
from typing import Optional

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request

from app.core.config import settings
from app.core.log_context import get_request_id, get_user_id
from app.db.database import SessionLocal
from app.db.models import AuditLog

logger = logging.getLogger(__name__)

SKIP_PREFIXES = (
    "/health",
    "/docs",
    "/openapi.json",
    "/redoc",
    "/metrics",
    "/webhooks",
)


class AuditLogMiddleware(BaseHTTPMiddleware):
    """
    Logs write operations to an audit log table.
    """

    async def dispatch(self, request: Request, call_next):
        response = await call_next(request)

        if settings.AUDIT_LOG_ENABLED != "1":
            return response

        if request.method in {"GET", "HEAD", "OPTIONS"}:
            return response

        path = request.url.path
        if any(path.startswith(prefix) for prefix in SKIP_PREFIXES):
            return response

        try:
            user_id = get_user_id() or None
            ip = request.client.host if request.client else None
            user_agent = request.headers.get("user-agent")

            meta = {
                "request_id": get_request_id(),
                "query": dict(request.query_params),
            }

            db = SessionLocal()
            try:
                db.add(
                    AuditLog(
                        user_id=user_id,
                        actor_type="user" if user_id else "anonymous",
                        action="http_request",
                        method=request.method,
                        path=path,
                        status_code=response.status_code,
                        ip_address=ip,
                        user_agent=user_agent,
                        metadata_json=json.dumps(meta, ensure_ascii=False),
                    )
                )
                db.commit()
            finally:
                db.close()
        except Exception as exc:
            logger.warning("Audit log failed: %s", exc)

        return response
