from __future__ import annotations

import logging

from app.core.config import settings

logger = logging.getLogger(__name__)


def setup_sentry() -> None:
    if not settings.SENTRY_DSN:
        return

    try:
        import sentry_sdk

        integrations = []
        try:
            from sentry_sdk.integrations.fastapi import FastApiIntegration as _FastApiIntegration
        except Exception:
            try:
                from sentry_sdk.integrations.fastapi import FastAPIIntegration as _FastAPIIntegration
            except Exception as integration_exc:
                logger.warning("Sentry FastAPI integration unavailable: %s", integration_exc)
            else:
                integrations = [_FastAPIIntegration()]
        else:
            integrations = [_FastApiIntegration()]

        sentry_sdk.init(
            dsn=settings.SENTRY_DSN,
            environment=settings.ENV,
            integrations=integrations,
            traces_sample_rate=settings.SENTRY_TRACES_SAMPLE_RATE,
            profiles_sample_rate=settings.SENTRY_PROFILES_SAMPLE_RATE,
        )
        logger.info("Sentry initialized")
    except Exception as exc:
        logger.error("Failed to initialize Sentry: %s", exc)
