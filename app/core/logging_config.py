# app/core/logging_config.py
# Structured logging configuration for production.

from __future__ import annotations

import logging
import sys

from app.core.config import settings


def setup_logging():
    """Configure logging for the application."""
    level = logging.DEBUG if settings.ENV == "dev" else logging.INFO

    fmt = "[%(asctime)s] %(levelname)s %(name)s: %(message)s"
    datefmt = "%Y-%m-%d %H:%M:%S"

    logging.basicConfig(
        level=level,
        format=fmt,
        datefmt=datefmt,
        stream=sys.stdout,
        force=True,
    )

    # Quiet noisy third-party loggers
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("httpcore").setLevel(logging.WARNING)
    logging.getLogger("apscheduler").setLevel(logging.WARNING)
    logging.getLogger("urllib3").setLevel(logging.WARNING)
