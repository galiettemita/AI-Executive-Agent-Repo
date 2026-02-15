from __future__ import annotations

from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse

from celery import Celery

from app.core.config import settings


def _normalize_rediss_url(url: str) -> str:
    """
    Celery's Redis backend requires `ssl_cert_reqs` to be explicitly present for
    `rediss://` URLs. Redis-py also accepts the lowercase values: none|optional|required.
    """

    if not url or not url.startswith("rediss://"):
        return url

    parsed = urlparse(url)
    query = dict(parse_qsl(parsed.query, keep_blank_values=True))
    if "ssl_cert_reqs" not in query:
        # Safe default: require cert validation.
        query["ssl_cert_reqs"] = "required"
        parsed = parsed._replace(query=urlencode(query))
        return urlunparse(parsed)
    return url


def _select_broker() -> str:
    if settings.CELERY_BROKER_URL:
        return _normalize_rediss_url(settings.CELERY_BROKER_URL)
    if settings.REDIS_URL:
        return _normalize_rediss_url(settings.REDIS_URL)
    if settings.ENV in ("production", "staging"):
        raise RuntimeError("CELERY_BROKER_URL or REDIS_URL is required in non-dev environments")
    return "memory://"


def _select_backend(broker: str) -> str:
    if settings.CELERY_RESULT_BACKEND:
        return _normalize_rediss_url(settings.CELERY_RESULT_BACKEND)
    if broker == "memory://":
        return "cache+memory://"
    return _normalize_rediss_url(broker)


broker = _select_broker()
backend = _select_backend(broker)

celery_app = Celery("executive_ai_agent", broker=broker, backend=backend)
celery_app.conf.task_always_eager = settings.CELERY_TASK_ALWAYS_EAGER or broker == "memory://"
celery_app.conf.task_track_started = True
celery_app.conf.task_default_queue = "default"
celery_app.autodiscover_tasks(["app.tasks"])
