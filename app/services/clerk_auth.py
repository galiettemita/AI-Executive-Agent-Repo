from __future__ import annotations

import logging

import httpx

from app.core.config import settings

logger = logging.getLogger(__name__)


def verify_clerk_user_id(clerk_user_id: str) -> bool:
    """
    Lightweight Clerk integration check for Gateway API.

    Uses Clerk Backend API with CLERK_SECRET_KEY. If key is missing, returns False.
    """
    if not clerk_user_id or not settings.CLERK_SECRET_KEY:
        return False

    try:
        with httpx.Client(timeout=8) as client:
            resp = client.get(
                f"https://api.clerk.com/v1/users/{clerk_user_id}",
                headers={"Authorization": f"Bearer {settings.CLERK_SECRET_KEY}"},
            )
            return resp.status_code == 200
    except Exception as exc:
        logger.warning("Clerk verification request failed: %s", exc)
        return False
