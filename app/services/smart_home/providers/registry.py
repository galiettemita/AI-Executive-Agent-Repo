# backend/app/services/smart_home/providers/registry.py

from __future__ import annotations

from typing import Dict

from sqlalchemy.orm import Session

from app.services.smart_home.providers.base import SmartHomeProvider
from app.services.smart_home.providers.home_assistant import HomeAssistantProvider
from app.services.smart_home.providers.stub import NotConfiguredProvider


_REGISTRY: Dict[str, SmartHomeProvider] = {}


def register_provider(name: str, provider: SmartHomeProvider) -> None:
    _REGISTRY[name] = provider


def get_provider(db: Session, name: str) -> SmartHomeProvider:
    if name in _REGISTRY:
        return _REGISTRY[name]

    name_lower = (name or "").lower()
    if name_lower in {"home_assistant", "homeassistant", "ha"}:
        return HomeAssistantProvider(db)

    if name_lower in {"google", "google_home", "homegraph"}:
        return NotConfiguredProvider("Google")

    if name_lower in {"alexa", "amazon"}:
        return NotConfiguredProvider("Alexa")

    if name_lower in {"homekit", "apple"}:
        return NotConfiguredProvider("HomeKit")

    return NotConfiguredProvider(name_lower or "Unknown")
