from __future__ import annotations

from enum import Enum
from typing import Any, Optional

from pydantic import BaseModel, Field
from pydantic.config import ConfigDict


class Channel(str, Enum):
    WHATSAPP = "whatsapp"
    IMESSAGE = "imessage"
    WEB = "web"


class MessageDirection(str, Enum):
    INBOUND = "inbound"
    OUTBOUND = "outbound"


class StrictModel(BaseModel):
    """
    Blueprint contracts are strict by default:
    - forbid unknown fields (catch drift early)
    - strip whitespace in strings
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class CapabilityEnvelope(StrictModel):
    """
    Top-level envelope for plane-to-plane calls (Blueprint).

    Phase 1 keeps this minimal; we will expand with auth, trace IDs,
    capabilities, and structured routing metadata.
    """

    envelope_id: str = Field(min_length=1)
    channel: Channel
    user_id: Optional[str] = None
    conversation_id: Optional[str] = None


class InboundMessage(StrictModel):
    """
    Gateway → Brain (internal) message envelope.

    Kept intentionally small for Phase 1. We’ll expand this as we implement
    the full contracts section of BLUEPRINT.pdf.
    """

    channel: Channel
    channel_msg_id: str = Field(min_length=1)
    user_id: Optional[str] = None
    conversation_id: Optional[str] = None
    run_id: Optional[str] = None
    from_phone: Optional[str] = None  # E.164 if applicable (e.g., WhatsApp)
    text: str = Field(default="", max_length=4096)
    raw: dict[str, Any] = Field(default_factory=dict)


class OutboundMessage(StrictModel):
    channel: Channel
    to_phone: Optional[str] = None
    text: str = Field(min_length=1, max_length=4096)
    metadata: dict[str, Any] = Field(default_factory=dict)


class IntentClassification(StrictModel):
    intent: str = Field(min_length=1)
    confidence: float = Field(ge=0.0, le=1.0, default=0.5)
    tier: int = Field(ge=0, le=3, default=1)


class TierRoutingConfig(StrictModel):
    t0_max_tokens: int = 256
    t1_max_tokens: int = 512
    t2_max_tokens: int = 1024
    t3_max_tokens: int = 2048


class ToolSpec(StrictModel):
    name: str = Field(min_length=1)
    description: str = ""
    input_schema: dict[str, Any] = Field(default_factory=dict)


class ToolCall(StrictModel):
    """
    Brain → Hands tool invocation (Phase 1).

    Full Blueprint tool contract will add structured schemas and execution metadata.
    """

    tool: str = Field(min_length=1)
    args: dict[str, Any] = Field(default_factory=dict)
    idempotency_key: Optional[str] = None
    user_id: Optional[str] = None
    run_id: Optional[str] = None


class ToolResult(StrictModel):
    tool: str = Field(min_length=1)
    ok: bool = True
    result: Optional[dict[str, Any]] = None
    error: Optional[str] = None


class MemoryEntry(StrictModel):
    key: str = Field(min_length=1)
    type: str = Field(min_length=1)
    content: dict[str, Any] = Field(default_factory=dict)
    confidence: float = Field(ge=0.0, le=1.0, default=0.8)


class MemoryQuery(StrictModel):
    query: str = Field(min_length=1)
    top_k: int = Field(ge=1, le=50, default=5)


class MemoryRetrievalResult(StrictModel):
    entries: list[MemoryEntry] = Field(default_factory=list)
