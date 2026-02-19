from __future__ import annotations

import uuid

from app.api.routes import v1_gateway
from app.blueprint.contracts import (
    Channel,
    ContentProvenance,
    EmotionState,
    InboundMessage,
    InputModality,
    OutboundMessage,
    ProcessedMessage,
)
from app.blueprint.progress import get_run_result
from app.services.billing_middleware import BillingDecision
from app.services.content_safety import RateLimitDecision


def _mk_inbound(*, channel: Channel = Channel.WEB) -> InboundMessage:
    return InboundMessage(
        channel=channel,
        channel_identifier="test-recipient",
        content="hello",
        input_modality=InputModality.TEXT,
        user_id=None,
        conversation_id="conv-test",
        run_id=str(uuid.uuid4()),
    )


def test_gateway_processing_lock_blocks_when_already_processing(monkeypatch) -> None:
    run_id = f"lock-{uuid.uuid4()}"
    inbound = _mk_inbound()

    monkeypatch.setattr(v1_gateway, "_acquire_processing_lock", lambda **kwargs: (False, None))

    v1_gateway._process_message_run(run_id, inbound)
    result = get_run_result(run_id)
    assert result is not None
    assert result.get("blocked") is True
    assert result.get("reason") == "concurrent_message_in_progress"


def test_gateway_routes_outbound_to_active_channel(monkeypatch) -> None:
    run_id = f"route-{uuid.uuid4()}"
    inbound = _mk_inbound(channel=Channel.WEB)

    monkeypatch.setattr(
        v1_gateway,
        "enforce_billing_for_inbound_message",
        lambda db, user_id: BillingDecision(allowed=True, plan="free_trial", status="trialing"),
    )
    monkeypatch.setattr(
        v1_gateway,
        "preprocess_inbound_message",
        lambda msg: ProcessedMessage(
            original=msg,
            normalized_text="hello",
            modality=InputModality.TEXT,
            transcription_confidence=None,
            extracted_entities={},
            emotion_detected=EmotionState.NEUTRAL,
            content_provenance=ContentProvenance.USER_DIRECT,
        ),
    )
    monkeypatch.setattr(v1_gateway, "classify_action", lambda *args, **kwargs: {"action_type": "respond"})
    monkeypatch.setattr(v1_gateway, "route_tier", lambda *args, **kwargs: 1)
    monkeypatch.setattr(v1_gateway, "_persist_v1_message", lambda **kwargs: None)

    monkeypatch.setattr(v1_gateway, "_get_active_channel", lambda **kwargs: Channel.IMESSAGE)

    def _mock_brain(msg: InboundMessage) -> OutboundMessage:
        return OutboundMessage(
            channel=msg.channel,
            recipient_id=msg.channel_identifier,
            content="## Hello **there**",
            metadata={"tier": 1},
        )

    monkeypatch.setattr(v1_gateway, "_brain_respond", _mock_brain)

    v1_gateway._process_message_run(run_id, inbound)
    result = get_run_result(run_id)
    assert result is not None
    assert result.get("ok") is True
    outbound = result.get("outbound") or {}
    assert outbound.get("channel") == Channel.IMESSAGE.value
    # iMessage formatter removes markdown syntax.
    assert "##" not in str(outbound.get("content") or "")
    assert "**" not in str(outbound.get("content") or "")


def test_gateway_blocks_when_safety_circuit_rate_limited(monkeypatch) -> None:
    run_id = f"safety-{uuid.uuid4()}"
    inbound = _mk_inbound()
    inbound.user_id = "user-123"

    monkeypatch.setattr(
        v1_gateway,
        "enforce_billing_for_inbound_message",
        lambda db, user_id: BillingDecision(allowed=True, plan="free_trial", status="trialing"),
    )
    monkeypatch.setattr(
        v1_gateway,
        "enforce_gateway_burst_limit",
        lambda user_id, limit_per_minute=10: RateLimitDecision(allowed=True),
    )
    monkeypatch.setattr(
        v1_gateway,
        "enforce_safety_circuit_rate_limit",
        lambda user_id: RateLimitDecision(allowed=False, retry_after_seconds=20, reason="safety_circuit_rate_limited"),
    )

    v1_gateway._process_message_run(run_id, inbound)
    result = get_run_result(run_id)
    assert result is not None
    assert result.get("blocked") is True
    assert result.get("reason") == "safety_circuit_rate_limited"
