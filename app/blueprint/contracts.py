from __future__ import annotations

from enum import Enum
from typing import Any, Optional

from pydantic import BaseModel, Field


class Channel(str, Enum):
    WHATSAPP = "whatsapp"
    IMESSAGE = "imessage"
    WEB = "web"


class MessageDirection(str, Enum):
    INBOUND = "inbound"
    OUTBOUND = "outbound"


class InboundMessage(BaseModel):
    """
    Gateway → Brain (internal) message envelope.

    Kept intentionally small for Phase 1. We’ll expand this as we implement
    the full contracts section of BLUEPRINT.pdf.
    """

    channel: Channel
    channel_msg_id: str = Field(min_length=1)
    from_phone: Optional[str] = None  # E.164 if applicable (e.g., WhatsApp)
    text: str = Field(default="", max_length=4096)
    raw: dict[str, Any] = Field(default_factory=dict)


class OutboundMessage(BaseModel):
    channel: Channel
    to_phone: Optional[str] = None
    text: str = Field(min_length=1, max_length=4096)
    metadata: dict[str, Any] = Field(default_factory=dict)

