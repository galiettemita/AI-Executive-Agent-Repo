from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException

from app.core.config import settings
from app.core.metrics import metrics_response

router = APIRouter(tags=["metrics"])


@router.get("/metrics")
def metrics(authorization: str | None = Header(default=None)):
    if settings.ENV in ("production", "staging") and settings.METRICS_TOKEN:
        if not authorization or not authorization.startswith("Bearer "):
            raise HTTPException(status_code=401, detail="Missing metrics token")
        token = authorization.split(" ", 1)[1].strip()
        if token != settings.METRICS_TOKEN:
            raise HTTPException(status_code=403, detail="Invalid metrics token")
    return metrics_response()
