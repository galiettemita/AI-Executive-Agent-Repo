# app/services/voice_call_service.py

from __future__ import annotations

import json
from typing import Dict, List, Optional
from datetime import datetime

from sqlalchemy.orm import Session

from app.db.models import VoiceCall, TaskItem


def create_voice_call(
    db: Session,
    user_id: str,
    direction: str,
    to_number: Optional[str],
    from_number: Optional[str],
    purpose: Optional[str],
    voice_profile: Optional[str] = None,
    proposal_id: Optional[int] = None,
    script_id: Optional[int] = None,
    script_json: Optional[str] = None,
    status: str = "initiating",
    call_sid: Optional[str] = None,
) -> VoiceCall:
    call = VoiceCall(
        user_id=user_id,
        direction=direction,
        to_number=to_number,
        from_number=from_number,
        purpose=purpose,
        voice_profile=voice_profile,
        proposal_id=proposal_id,
        script_id=script_id,
        script_json=script_json,
        status=status,
        call_sid=call_sid,
    )
    db.add(call)
    db.commit()
    db.refresh(call)
    return call


def update_call_status(
    db: Session,
    call: VoiceCall,
    status: str,
    duration_seconds: Optional[int] = None,
    recording_url: Optional[str] = None,
    outcome_status: Optional[str] = None,
    outcome_notes: Optional[str] = None,
    error_message: Optional[str] = None,
) -> VoiceCall:
    call.status = status
    now = datetime.utcnow()
    if status in ("connected", "in-progress") and not call.answered_at:
        call.answered_at = now
    if status in ("ended", "failed") and not call.ended_at:
        call.ended_at = now
    if duration_seconds is not None:
        call.duration_seconds = duration_seconds
    if recording_url:
        call.recording_url = recording_url
    if outcome_status:
        call.outcome_status = outcome_status
    if outcome_notes:
        call.outcome_notes = outcome_notes
    if error_message:
        call.error_message = error_message
    db.commit()
    db.refresh(call)
    return call


def set_call_stream_sid(db: Session, call: VoiceCall, stream_sid: str) -> VoiceCall:
    call.stream_sid = stream_sid
    db.commit()
    db.refresh(call)
    return call


def append_transcript(db: Session, call: VoiceCall, text: str) -> VoiceCall:
    existing = call.transcript or ""
    joined = (existing + "\n" + text).strip()
    call.transcript = joined
    db.commit()
    db.refresh(call)
    return call


def store_summary_and_actions(
    db: Session,
    call: VoiceCall,
    summary: str,
    action_items: Optional[List[str]] = None,
) -> VoiceCall:
    call.summary = summary
    action_items = action_items or []
    call.action_items_json = json.dumps(action_items)
    db.commit()
    db.refresh(call)

    # Link outcomes to tasks
    for item in action_items:
        title = f"Call follow-up: {item}"
        db.add(TaskItem(user_id=call.user_id, title=title, due_at=None, completed=False))
    db.commit()

    return call
