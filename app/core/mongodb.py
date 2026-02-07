from __future__ import annotations

import logging

from app.core.config import settings

logger = logging.getLogger(__name__)

try:
    from motor.motor_asyncio import AsyncIOMotorClient  # type: ignore[import-not-found]
except ModuleNotFoundError:  # pragma: no cover
    AsyncIOMotorClient = None

_mongo_client = None


def get_mongo_client():
    global _mongo_client
    if not settings.MONGODB_URI:
        return None
    if AsyncIOMotorClient is None:
        raise RuntimeError("motor is not installed. Install with: pip install motor")
    if _mongo_client is None:
        try:
            _mongo_client = AsyncIOMotorClient(settings.MONGODB_URI)
        except Exception as exc:
            logger.error("Failed to initialize MongoDB client: %s", exc)
            return None
    return _mongo_client


def get_mongo_db():
    client = get_mongo_client()
    if not client:
        return None
    return client[settings.MONGODB_DB]
