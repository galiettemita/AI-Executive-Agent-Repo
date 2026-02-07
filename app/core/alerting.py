from __future__ import annotations

import logging
from typing import Literal

import httpx

from app.core.config import settings

logger = logging.getLogger(__name__)


def _send_slack(message: str) -> None:
    if not settings.SLACK_ALERT_WEBHOOK_URL:
        raise RuntimeError("SLACK_ALERT_WEBHOOK_URL not set")
    with httpx.Client(timeout=5) as client:
        client.post(settings.SLACK_ALERT_WEBHOOK_URL, json={"text": message})


def _send_pagerduty(message: str) -> None:
    if not settings.PAGERDUTY_ROUTING_KEY:
        raise RuntimeError("PAGERDUTY_ROUTING_KEY not set")
    payload = {
        "routing_key": settings.PAGERDUTY_ROUTING_KEY,
        "event_action": "trigger",
        "payload": {
            "summary": message,
            "severity": "error",
            "source": "executive-ai-agent",
        },
    }
    with httpx.Client(timeout=5) as client:
        client.post("https://events.pagerduty.com/v2/enqueue", json=payload)


def send_alert(message: str, provider: Literal["sentry", "slack", "pagerduty"] | None = None) -> None:
    """
    Send an alert to the configured provider. Sentry alerts are configured in Sentry UI,
    so this function logs a warning for visibility when provider is sentry.
    """
    provider = (provider or settings.ALERTING_PROVIDER or "").lower()
    if not provider:
        logger.warning("Alert dropped (no provider configured): %s", message)
        return

    if provider == "sentry":
        logger.error("Alert: %s", message)
        return

    if provider == "slack":
        _send_slack(message)
        return

    if provider == "pagerduty":
        _send_pagerduty(message)
        return

    raise RuntimeError(f"Unknown alerting provider: {provider}")
