import json
import uuid

from app.db.database import SessionLocal
from app.db.models import User, Proposal, VoiceCall
from app.services.consent_service import grant_consent
from app.services.execution_engine import ExecutionEngine
import app.services.twilio_voice as twilio_voice


def test_voice_call_proposal_execution(monkeypatch):
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    db.add(User(id=user_id))
    db.commit()

    grant_consent(db, user_id, "voice")

    payload = {
        "to_number": "+15551230000",
        "from_number": "+15551239999",
        "purpose": "confirm appointment",
        "voice_profile": None,
    }

    proposal = Proposal(
        user_id=user_id,
        proposal_type="voice_call",
        status="approved",
        payload_json=json.dumps(payload),
    )
    db.add(proposal)
    db.commit()
    db.refresh(proposal)

    monkeypatch.setattr(twilio_voice, "create_outbound_call", lambda **kwargs: "CA123")

    result = ExecutionEngine._execute_voice_call_proposal(db, proposal, payload, dry_run=False)
    assert result.get("success") is True

    call = db.query(VoiceCall).filter(VoiceCall.proposal_id == proposal.id).first()
    assert call is not None
    assert call.call_sid == "CA123"

    db.close()
