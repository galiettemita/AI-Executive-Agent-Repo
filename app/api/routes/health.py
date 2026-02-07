# app/api/routes/health.py

from __future__ import annotations

import logging

import os
from sqlalchemy import text
from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.core.config import settings

logger = logging.getLogger(__name__)

router = APIRouter(tags=["health"])


@router.get("/health")
def health():
    """General health check — returns key configuration status."""
    version = (
        settings.APP_VERSION
        or os.getenv("RENDER_GIT_COMMIT", "")
        or os.getenv("VERCEL_GIT_COMMIT_SHA", "")
    )
    return {
        "status": "ok",
        "env": settings.ENV,
        "version": version,
        "openai_configured": bool(settings.OPENAI_API_KEY),
        "stripe_configured": bool(settings.STRIPE_SECRET_KEY),
        "amadeus_configured": bool(settings.AMADEUS_API_KEY),
        "twilio_configured": bool(settings.TWILIO_ACCOUNT_SID),
        "deepgram_configured": bool(settings.DEEPGRAM_API_KEY),
        "elevenlabs_configured": bool(settings.ELEVENLABS_API_KEY),
        "google_configured": bool(settings.GOOGLE_CLIENT_ID),
        "redis_configured": bool(settings.REDIS_URL),
    }


@router.get("/health/live")
def liveness():
    """Liveness probe — is the process running? For Kubernetes."""
    return {"status": "alive"}


@router.get("/health/ready")
def readiness():
    """
    Readiness probe — are dependencies available?
    Returns 503 if critical checks fail.
    """
    checks: dict = {}
    all_ok = True

    # DB connectivity
    try:
        from app.db.database import SessionLocal
        db = SessionLocal()
        try:
            db.execute(text("SELECT 1"))
            checks["database"] = "ok"
        finally:
            db.close()
    except Exception as e:
        logger.error("Readiness: DB check failed: %s", e)
        checks["database"] = "unavailable"
        all_ok = False

    # Critical env vars
    critical_missing = []
    if not settings.OPENAI_API_KEY:
        critical_missing.append("OPENAI_API_KEY")
    if not settings.JWT_SECRET or settings.JWT_SECRET == "dev_only_change_me":
        if settings.ENV != "dev":
            critical_missing.append("JWT_SECRET")

    if critical_missing:
        checks["env_vars"] = f"missing: {', '.join(critical_missing)}"
        all_ok = False
    else:
        checks["env_vars"] = "ok"

    # Redis (if configured)
    if settings.REDIS_URL:
        try:
            import redis as redis_lib
            r = redis_lib.from_url(settings.REDIS_URL, socket_timeout=2)
            r.ping()
            checks["redis"] = "ok"
        except Exception as e:
            logger.error("Readiness: Redis check failed: %s", e)
            checks["redis"] = "unavailable"
            all_ok = False
    else:
        checks["redis"] = "not_configured"

    status_code = 200 if all_ok else 503
    return JSONResponse(
        status_code=status_code,
        content={"status": "ready" if all_ok else "not_ready", "checks": checks},
    )
