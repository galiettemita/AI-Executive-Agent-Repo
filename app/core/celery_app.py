from __future__ import annotations

from celery import Celery

from app.core.config import settings


def _select_broker() -> str:
    if settings.CELERY_BROKER_URL:
        return settings.CELERY_BROKER_URL
    if settings.REDIS_URL:
        return settings.REDIS_URL
    if settings.ENV in ("production", "staging"):
        raise RuntimeError("CELERY_BROKER_URL or REDIS_URL is required in non-dev environments")
    return "memory://"


def _select_backend(broker: str) -> str:
    if settings.CELERY_RESULT_BACKEND:
        return settings.CELERY_RESULT_BACKEND
    if broker == "memory://":
        return "cache+memory://"
    return broker


broker = _select_broker()
backend = _select_backend(broker)

celery_app = Celery("executive_ai_agent", broker=broker, backend=backend)
celery_app.conf.task_always_eager = settings.CELERY_TASK_ALWAYS_EAGER or broker == "memory://"
celery_app.conf.task_track_started = True
celery_app.conf.task_default_queue = "default"
celery_app.autodiscover_tasks(["app.tasks"])
