import uuid
from datetime import datetime, timedelta

from app.db.database import SessionLocal
from app.db.models import User, VoiceCall
from app.services.voice_retention import purge_expired_calls


def test_voice_retention_purges_expired_calls():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"
    db.add(User(id=user_id))
    db.commit()

    old_time = datetime.utcnow() - timedelta(days=10)
    call = VoiceCall(
        user_id=user_id,
        direction="outbound",
        to_number="+15551230000",
        from_number="+15550001111",
        status="ended",
        created_at=old_time,
        ended_at=old_time,
        recording_url="https://example.com/recording.mp3",
        transcript="hello",
        summary="summary",
        action_items_json="[]",
    )
    db.add(call)
    db.commit()
    db.refresh(call)

    purged = purge_expired_calls(db, retention_days=1)
    assert purged == 1

    refreshed = db.query(VoiceCall).filter(VoiceCall.id == call.id).first()
    assert refreshed.recording_url is None
    assert refreshed.transcript is None
    assert refreshed.summary is None
    assert refreshed.action_items_json is None

    db.close()
