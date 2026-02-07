from __future__ import annotations

from app.core.celery_app import celery_app


@celery_app.task(name="system.ping")
def ping() -> str:
    return "pong"
