from __future__ import annotations

import os
from datetime import datetime

from fastapi import APIRouter, Header, HTTPException
from fastapi.responses import JSONResponse
from sqlalchemy import text

from app.core.config import settings
from app.core.metrics import metrics_response


router = APIRouter(prefix="/internal", tags=["internal-platform"])


def _db_status() -> str:
    try:
        from app.db.database import SessionLocal

        db = SessionLocal()
        try:
            db.execute(text("select 1"))
            return "ok"
        finally:
            db.close()
    except Exception:
        return "unavailable"


def _redis_status() -> str:
    if not settings.REDIS_URL:
        return "not_configured"
    try:
        from app.core.redis import redis_ping

        redis_ping()
        return "ok"
    except Exception:
        return "unavailable"


@router.get("/health")
def internal_health():
    return {
        "status": "ok",
        "env": settings.ENV,
        "time": datetime.utcnow().isoformat(),
        "version": settings.APP_VERSION
        or os.getenv("RENDER_GIT_COMMIT", "")
        or os.getenv("VERCEL_GIT_COMMIT_SHA", ""),
    }


@router.get("/health/deep")
def internal_health_deep():
    db = _db_status()
    redis = _redis_status()
    providers = {
        "openai": bool(settings.OPENAI_API_KEY),
        "anthropic": bool(settings.ANTHROPIC_API_KEY),
        "google_ai": bool(settings.GOOGLE_AI_API_KEY),
    }
    all_ok = db == "ok" and redis in {"ok", "not_configured"}
    return JSONResponse(
        status_code=200 if all_ok else 503,
        content={
            "status": "ready" if all_ok else "degraded",
            "checks": {
                "database": db,
                "redis": redis,
                "providers_configured": providers,
            },
        },
    )


@router.get("/metrics")
def internal_metrics(authorization: str | None = Header(default=None)):
    if settings.ENV in ("production", "staging") and settings.METRICS_TOKEN:
        if not authorization or not authorization.startswith("Bearer "):
            raise HTTPException(status_code=401, detail="Missing metrics token")
        token = authorization.split(" ", 1)[1].strip()
        if token != settings.METRICS_TOKEN:
            raise HTTPException(status_code=403, detail="Invalid metrics token")
    return metrics_response()
