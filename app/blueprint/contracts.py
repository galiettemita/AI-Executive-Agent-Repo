from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any, Optional
from uuid import UUID

from pydantic import BaseModel, Field, model_validator
from pydantic.config import ConfigDict


class Channel(str, Enum):
    WHATSAPP = "whatsapp"
    IMESSAGE = "imessage"
    SLACK = "slack"
    WEB = "web"
    VOICE = "voice"


class MessageDirection(str, Enum):
    INBOUND = "inbound"
    OUTBOUND = "outbound"


class InputModality(str, Enum):
    TEXT = "text"
    VOICE = "voice"
    IMAGE = "image"
    DOCUMENT = "document"
    LOCATION = "location"


class OutputModality(str, Enum):
    TEXT = "text"
    VOICE = "voice"
    DOCUMENT = "document"


class EmotionState(str, Enum):
    NEUTRAL = "neutral"
    POSITIVE = "positive"
    FRUSTRATED = "frustrated"
    RUSHED = "rushed"
    STRESSED = "stressed"
    EXCITED = "excited"


class ContentProvenance(str, Enum):
    USER_DIRECT = "user_direct"
    EMAIL_BODY = "email_body"
    WEB_SEARCH = "web_search"
    CALENDAR_DESC = "calendar_desc"
    DOCUMENT = "document"
    MCP_RESULT = "mcp_result"
    # Backward-compat alias for older enum value/string.
    MCP_RESPONSE = "mcp_result"


class LLMProvider(str, Enum):
    OPENAI = "openai"
    ANTHROPIC = "anthropic"
    GOOGLE = "google"
    LOCAL = "local"
    MCP = "mcp"


class RiskLevel(str, Enum):
    NONE = "none"
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class DelegationStatus(str, Enum):
    PENDING = "pending"
    SENT = "sent"
    ACKNOWLEDGED = "acknowledged"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    OVERDUE = "overdue"
    CANCELLED = "cancelled"


class WorkflowTriggerType(str, Enum):
    SCHEDULE = "schedule"
    EVENT = "event"
    CONDITION = "condition"
    MANUAL = "manual"


class StrictModel(BaseModel):
    """
    Blueprint contracts are strict by default:
    - forbid unknown fields (catch drift early)
    - strip whitespace in strings
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class CapabilityEnvelope(StrictModel):
    envelope_id: str = Field(min_length=1)
    channel: Channel
    user_id: Optional[str] = None
    conversation_id: Optional[str] = None


class InboundMessage(StrictModel):
    """
    Section 4.1 contract with compatibility fields for existing phase-1 codepaths.
    """

    # Section 4.1 canonical fields
    channel: Channel
    channel_identifier: Optional[str] = None
    content: str = ""
    input_modality: InputModality = InputModality.TEXT
    media_url: Optional[str] = None
    media_type: Optional[str] = None
    wa_message_id: Optional[str] = None
    reply_to_id: Optional[str] = None
    timestamp: datetime = Field(default_factory=datetime.utcnow)

    # Compatibility fields still used by existing Gateway/Brain task code
    channel_msg_id: Optional[str] = None
    user_id: Optional[str] = None
    conversation_id: Optional[str] = None
    run_id: Optional[str] = None
    from_phone: Optional[str] = None
    text: str = Field(default="", max_length=4096)
    raw: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def _normalize_compat_fields(self) -> "InboundMessage":
        if not self.content and self.text:
            self.content = self.text
        if not self.text and self.content:
            self.text = self.content
        if not self.channel_identifier and self.from_phone:
            self.channel_identifier = self.from_phone
        if not self.wa_message_id and self.channel_msg_id:
            self.wa_message_id = self.channel_msg_id
        if not self.channel_msg_id and self.wa_message_id:
            self.channel_msg_id = self.wa_message_id
        return self


class ProcessedMessage(StrictModel):
    original: InboundMessage
    normalized_text: str
    modality: InputModality = InputModality.TEXT
    transcription_confidence: Optional[float] = None
    extracted_entities: dict[str, Any] = Field(default_factory=dict)
    emotion_detected: EmotionState = EmotionState.NEUTRAL
    content_provenance: ContentProvenance = ContentProvenance.USER_DIRECT


class WhatsAppButton(StrictModel):
    id: str
    title: str


class OutboundMessage(StrictModel):
    # Section 4.1 canonical fields
    channel: Channel
    recipient_id: Optional[str] = None
    content: str = Field(default="", max_length=4096)
    output_modality: OutputModality = OutputModality.TEXT
    attachment_url: Optional[str] = None
    reply_to_id: Optional[str] = None
    buttons: Optional[list[WhatsAppButton]] = None
    metadata: dict[str, Any] = Field(default_factory=dict)

    # Compatibility fields
    to_phone: Optional[str] = None
    text: str = Field(default="", max_length=4096)

    @model_validator(mode="after")
    def _normalize_compat_fields(self) -> "OutboundMessage":
        if not self.content and self.text:
            self.content = self.text
        if not self.text and self.content:
            self.text = self.content
        if not self.recipient_id and self.to_phone:
            self.recipient_id = self.to_phone
        if not self.to_phone and self.recipient_id:
            self.to_phone = self.recipient_id
        return self


class IntentClassification(StrictModel):
    intent: str = Field(min_length=1)
    confidence: float = Field(ge=0.0, le=1.0, default=0.5)
    tier: int = Field(ge=0, le=3, default=1)
    required_tools: list[str] = Field(default_factory=list)
    is_continuation: bool = False
    continuation_run_id: Optional[UUID] = None
    entities: dict[str, Any] = Field(default_factory=dict)
    requires_delegation: bool = False
    requires_research: bool = False
    emotion_context: EmotionState = EmotionState.NEUTRAL


class TierRoutingConfig(StrictModel):
    intent_pattern: str = ".*"
    default_tier: int = Field(ge=0, le=3, default=1)
    escalation_rules: dict[str, Any] = Field(default_factory=dict)
    required_tools: list[str] = Field(default_factory=list)
    max_cost_cents: int = 50
    preferred_provider: Optional[LLMProvider] = None


class ToolSpec(StrictModel):
    name: str = Field(min_length=1)
    description: str = ""
    input_schema: dict[str, Any] = Field(default_factory=dict)
    output_schema: dict[str, Any] = Field(default_factory=dict)
    risk_level: RiskLevel = RiskLevel.NONE
    is_reversible: bool = False
    requires_approval_above: RiskLevel = RiskLevel.MEDIUM
    timeout_ms: int = 10000
    retry_policy: dict[str, Any] = Field(default_factory=lambda: {"max_retries": 2, "backoff": "exponential"})
    rate_limit_per_min: int = 30
    tags: list[str] = Field(default_factory=list)
    is_mcp: bool = False
    mcp_server_id: Optional[str] = None
    capability_scope: list[str] = Field(default_factory=list)


class ToolCall(StrictModel):
    # Canonical fields
    tool_name: str = Field(min_length=1)
    arguments: dict[str, Any] = Field(default_factory=dict)
    idempotency_key: Optional[str] = None
    envelope_id: Optional[UUID] = None
    run_id: Optional[str] = None
    input_provenance: ContentProvenance = ContentProvenance.USER_DIRECT
    required_capabilities: list[str] = Field(default_factory=list)
    capability_token: Optional[str] = None

    # Compatibility fields
    tool: Optional[str] = None
    args: dict[str, Any] = Field(default_factory=dict)
    user_id: Optional[str] = None

    @model_validator(mode="after")
    def _normalize_compat_fields(self) -> "ToolCall":
        if not self.tool_name and self.tool:
            self.tool_name = self.tool
        if not self.tool and self.tool_name:
            self.tool = self.tool_name
        if not self.arguments and self.args:
            self.arguments = self.args
        if not self.args and self.arguments:
            self.args = self.arguments
        return self


class ToolResult(StrictModel):
    # Canonical fields
    tool_name: str = Field(min_length=1)
    status: str = "success"
    output: Optional[dict[str, Any]] = None
    error: Optional[dict[str, Any] | str] = None
    latency_ms: int = 0
    cost_cents: float = 0
    compensating_action: Optional[dict[str, Any]] = None

    # Compatibility fields
    tool: Optional[str] = None
    ok: bool = True
    result: Optional[dict[str, Any]] = None

    @model_validator(mode="after")
    def _normalize_compat_fields(self) -> "ToolResult":
        if not self.tool_name and self.tool:
            self.tool_name = self.tool
        if not self.tool and self.tool_name:
            self.tool = self.tool_name

        if self.output is None and self.result is not None:
            self.output = self.result
        if self.result is None and self.output is not None:
            self.result = self.output

        if self.ok and self.status in ("", "failed"):
            self.status = "success"
        if not self.ok and self.status == "success":
            self.status = "failed"

        if isinstance(self.error, str):
            self.error = {"message": self.error}
        return self


class TokenUsage(StrictModel):
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


class LLMRequest(StrictModel):
    model_preference: Optional[LLMProvider] = None
    messages: list[dict[str, Any]]
    tools: Optional[list[dict[str, Any]]] = None
    temperature: float = 0.7
    max_tokens: int = 2000
    structured_output: Optional[dict[str, Any]] = None
    task_type: str = "general"
    max_cost_cents: float = 10.0
    max_latency_ms: int = 15000
    pii_content: bool = False
    requires_safety_check: bool = False
    stream: bool = False


class LLMResponse(StrictModel):
    provider: LLMProvider
    model: str
    content: str
    tool_calls: Optional[list[dict[str, Any]]] = None
    usage: TokenUsage = Field(default_factory=TokenUsage)
    cost_cents: float = 0
    latency_ms: int = 0
    was_failover: bool = False
    safety_validated: bool = False


class ProviderHealth(StrictModel):
    provider: LLMProvider
    is_healthy: bool
    latency_p50_ms: int = 0
    latency_p95_ms: int = 0
    error_rate_1h: float = 0.0
    last_error_at: Optional[datetime] = None
    cost_multiplier: float = 1.0


class LLMProviderHealth(ProviderHealth):
    """
    Explicit alias for Operational Blueprint failover workstreams.
    Keeps backward compatibility with existing `ProviderHealth` references.
    """


class DelegationRequest(StrictModel):
    delegate_name: str
    delegate_contact: Optional[str] = None
    task_description: str
    context: dict[str, Any] = Field(default_factory=dict)
    deadline: Optional[datetime] = None
    priority: RiskLevel = RiskLevel.MEDIUM
    reminder_schedule: list[datetime] = Field(default_factory=list)
    completion_criteria: Optional[str] = None
    notify_channel: Optional[Channel] = None


class DelegationUpdate(StrictModel):
    delegation_id: UUID
    status: DelegationStatus
    result: Optional[str] = None
    updated_by: str


class WorkflowTrigger(StrictModel):
    type: WorkflowTriggerType
    config: dict[str, Any] = Field(default_factory=dict)


class WorkflowAction(StrictModel):
    step: int
    tool_name: str
    arguments_template: dict[str, Any] = Field(default_factory=dict)
    on_failure: str = "stop"
    approval_required: bool = False


class WorkflowCondition(StrictModel):
    field: str
    operator: str
    value: str


class WorkflowDefinition(StrictModel):
    name: str
    description: Optional[str] = None
    trigger: WorkflowTrigger
    actions: list[WorkflowAction]
    conditions: Optional[list[WorkflowCondition]] = None


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
