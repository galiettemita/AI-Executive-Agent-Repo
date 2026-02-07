# app/api/routes/voice.py

from __future__ import annotations

import asyncio
import base64
import json
import logging
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request, WebSocket, WebSocketDisconnect
from fastapi.responses import Response
from pydantic import BaseModel
from sqlalchemy.orm import Session
from twilio.request_validator import RequestValidator

from app.db.database import get_db, SessionLocal
from app.db.models import VoiceCall, VoiceCallScript
from app.services.voice_call_service import (
    create_voice_call,
    update_call_status,
    set_call_stream_sid,
    append_transcript,
    store_summary_and_actions,
)
from app.services.proposals import create_proposal_with_link
from app.services.consent_service import require_consent
from app.middleware.rate_limiter import rate_limit_user, rate_limit_webhook
from app.services.twilio_voice import create_outbound_call, generate_twiml_stream
from app.services.voice_ai import generate_call_response, summarize_call
from app.services.voice_profiles import resolve_voice_id
from app.services.stt_deepgram import DeepgramStream
from app.services.tts_elevenlabs import stream_tts_audio
from app.core.config import settings

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/voice", tags=["voice"])
webhook_router = APIRouter(prefix="/webhooks/voice", tags=["webhooks"])


class OutboundCallRequest(BaseModel):
    user_id: str
    to_number: str
    purpose: Optional[str] = None
    voice_profile: Optional[str] = None
    from_number: Optional[str] = None
    script_id: Optional[int] = None


class VoiceCallProposalRequest(BaseModel):
    user_id: str
    to_number: str
    purpose: Optional[str] = None
    voice_profile: Optional[str] = None
    from_number: Optional[str] = None
    script_id: Optional[int] = None
    script: Optional[dict] = None
    summary: Optional[str] = None


class VoiceScriptRequest(BaseModel):
    user_id: str
    name: str
    description: Optional[str] = None
    script: Optional[dict] = None


class VoiceScriptUpdateRequest(BaseModel):
    name: Optional[str] = None
    description: Optional[str] = None
    script: Optional[dict] = None


class VoiceOutcomeRequest(BaseModel):
    outcome_status: str
    outcome_notes: Optional[str] = None


def _validate_twilio(request: Request, params: dict) -> bool:
    auth_token = settings.TWILIO_AUTH_TOKEN
    if not auth_token:
        if settings.ENV in ("production", "staging") and settings.ENFORCE_WEBHOOK_SIGNATURES == "1":
            logger.error("TWILIO_AUTH_TOKEN not set; signature verification required")
            return False
        logger.warning("TWILIO_AUTH_TOKEN not set; skipping validation")
        return True
    signature = request.headers.get("X-Twilio-Signature", "")
    validator = RequestValidator(auth_token)
    return validator.validate(str(request.url), params, signature)


@router.get("/user/{user_id}")
def list_user_calls(user_id: str, db: Session = Depends(get_db)):
    calls = (
        db.query(VoiceCall)
        .filter(VoiceCall.user_id == user_id)
        .order_by(VoiceCall.created_at.desc())
        .limit(50)
        .all()
    )
    return {
        "ok": True,
        "calls": [
            {
                "id": c.id,
                "direction": c.direction,
                "purpose": c.purpose,
                "status": c.status,
                "created_at": c.created_at.isoformat() if c.created_at else None,
            }
            for c in calls
        ],
    }


@router.get("/{call_id}")
def get_call(call_id: int, db: Session = Depends(get_db)):
    call = db.query(VoiceCall).filter(VoiceCall.id == call_id).first()
    if not call:
        raise HTTPException(status_code=404, detail="Call not found")
    return {
        "ok": True,
        "call": {
            "id": call.id,
            "user_id": call.user_id,
            "direction": call.direction,
            "to_number": call.to_number,
            "from_number": call.from_number,
            "purpose": call.purpose,
            "voice_profile": call.voice_profile,
            "status": call.status,
            "duration_seconds": call.duration_seconds,
            "recording_url": call.recording_url,
            "transcript": call.transcript,
            "summary": call.summary,
            "action_items": json.loads(call.action_items_json) if call.action_items_json else [],
            "created_at": call.created_at.isoformat() if call.created_at else None,
        },
    }


@rate_limit_user()
@router.post("/outbound")
def start_outbound_call(request: Request, payload: OutboundCallRequest, db: Session = Depends(get_db)):
    """
    Initiate an outbound call via Twilio Voice.
    """
    if settings.ENABLE_VOICE_CALLS != "1":
        raise HTTPException(status_code=403, detail="Voice calls are disabled")

    app_base_url = settings.APP_BASE_URL
    if not app_base_url:
        raise HTTPException(status_code=500, detail="APP_BASE_URL not configured")

    try:
        require_consent(db, payload.user_id, "voice")
    except Exception as exc:
        raise HTTPException(status_code=403, detail=str(exc))

    script_json = None
    if payload.script_id:
        script_row = db.query(VoiceCallScript).filter(VoiceCallScript.id == payload.script_id).first()
        if not script_row:
            raise HTTPException(status_code=404, detail="Voice script not found")
        script_json = script_row.script_json

    call = create_voice_call(
        db=db,
        user_id=payload.user_id,
        direction="outbound",
        to_number=payload.to_number,
        from_number=payload.from_number,
        purpose=payload.purpose,
        voice_profile=payload.voice_profile,
        script_id=payload.script_id,
        script_json=script_json,
        status="initiating",
    )

    twiml_url = f"{app_base_url}/webhooks/voice/twiml?call_id={call.id}"
    status_url = f"{app_base_url}/webhooks/voice/status"
    recording_url = f"{app_base_url}/webhooks/voice/recording"

    try:
        call_sid = create_outbound_call(
            to_number=payload.to_number,
            from_number=payload.from_number,
            twiml_url=twiml_url,
            status_callback_url=status_url,
            recording_status_callback_url=recording_url,
        )
    except Exception as exc:
        update_call_status(
            db,
            call,
            status="failed",
            outcome_status="failed",
            outcome_notes="Outbound call failed to start",
            error_message=str(exc),
        )
        raise HTTPException(status_code=502, detail=f"Twilio outbound call failed: {exc}")

    call.call_sid = call_sid
    call.status = "ringing"
    db.commit()
    db.refresh(call)

    return {"ok": True, "call_id": call.id, "call_sid": call_sid}


@rate_limit_user()
@router.post("/proposals")
def create_voice_call_proposal(request: Request, payload: VoiceCallProposalRequest, db: Session = Depends(get_db)):
    if settings.ENABLE_VOICE_CALLS != "1":
        raise HTTPException(status_code=403, detail="Voice calls are disabled")
    proposal_payload = {
        "to_number": payload.to_number,
        "from_number": payload.from_number,
        "purpose": payload.purpose,
        "voice_profile": payload.voice_profile,
    }
    if payload.script_id:
        proposal_payload["script_id"] = payload.script_id
    if payload.script:
        proposal_payload["script"] = payload.script

    created = create_proposal_with_link(
        db,
        user_id=payload.user_id,
        proposal_type="voice_call",
        payload=proposal_payload,
    )
    summary = payload.summary or "Voice call proposal created."
    return {
        "ok": True,
        "proposal_id": created.get("proposal_id"),
        "approval_url": created.get("approval_url"),
        "summary": summary,
    }


@rate_limit_user()
@router.post("/scripts")
def create_voice_script(request: Request, payload: VoiceScriptRequest, db: Session = Depends(get_db)):
    script_json = json.dumps(payload.script or {}, ensure_ascii=False)
    row = VoiceCallScript(
        user_id=payload.user_id,
        name=payload.name,
        description=payload.description,
        script_json=script_json,
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return {"ok": True, "script_id": row.id}


@rate_limit_user()
@router.get("/scripts")
def list_voice_scripts(request: Request, user_id: str, db: Session = Depends(get_db)):
    rows = (
        db.query(VoiceCallScript)
        .filter(VoiceCallScript.user_id == user_id)
        .order_by(VoiceCallScript.created_at.desc())
        .all()
    )
    return {
        "items": [
            {
                "id": r.id,
                "name": r.name,
                "description": r.description,
                "script": json.loads(r.script_json) if r.script_json else {},
                "created_at": r.created_at.isoformat() if r.created_at else None,
            }
            for r in rows
        ]
    }


@rate_limit_user()
@router.get("/scripts/{script_id}")
def get_voice_script(request: Request, script_id: int, db: Session = Depends(get_db)):
    row = db.query(VoiceCallScript).filter(VoiceCallScript.id == script_id).first()
    if not row:
        raise HTTPException(status_code=404, detail="Voice script not found")
    return {
        "id": row.id,
        "user_id": row.user_id,
        "name": row.name,
        "description": row.description,
        "script": json.loads(row.script_json) if row.script_json else {},
        "created_at": row.created_at.isoformat() if row.created_at else None,
    }


@rate_limit_user()
@router.patch("/scripts/{script_id}")
def update_voice_script(request: Request, script_id: int, payload: VoiceScriptUpdateRequest, db: Session = Depends(get_db)):
    row = db.query(VoiceCallScript).filter(VoiceCallScript.id == script_id).first()
    if not row:
        raise HTTPException(status_code=404, detail="Voice script not found")
    if payload.name is not None:
        row.name = payload.name
    if payload.description is not None:
        row.description = payload.description
    if payload.script is not None:
        row.script_json = json.dumps(payload.script or {}, ensure_ascii=False)
    db.commit()
    db.refresh(row)
    return {"ok": True, "script_id": row.id}


@rate_limit_user()
@router.post("/calls/{call_id}/outcome")
def set_voice_call_outcome(request: Request, call_id: int, payload: VoiceOutcomeRequest, db: Session = Depends(get_db)):
    call = db.query(VoiceCall).filter(VoiceCall.id == call_id).first()
    if not call:
        raise HTTPException(status_code=404, detail="Call not found")
    update_call_status(
        db,
        call,
        status=call.status,
        outcome_status=payload.outcome_status,
        outcome_notes=payload.outcome_notes,
    )
    return {"ok": True, "call_id": call.id, "outcome_status": call.outcome_status}


@rate_limit_webhook()
@webhook_router.api_route("/twiml", methods=["GET", "POST"])
async def twiml_webhook(request: Request):
    """
    Twilio webhook to fetch TwiML for inbound/outbound calls.
    """
    form = await request.form() if request.method == "POST" else {}
    params = dict(request.query_params)

    def _get(key: str) -> Optional[str]:
        return form.get(key) or params.get(key)

    merged_params = {**params, **dict(form)}
    if not _validate_twilio(request, merged_params):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")

    call_sid = _get("CallSid")
    from_number = _get("From")
    to_number = _get("To")
    call_id = _get("call_id")

    db = SessionLocal()
    try:
        call: Optional[VoiceCall] = None
        if call_id:
            call = db.query(VoiceCall).filter(VoiceCall.id == int(call_id)).first()

        if not call:
            call = create_voice_call(
                db=db,
                user_id=from_number or "unknown",
                direction="inbound",
                to_number=to_number,
                from_number=from_number,
                purpose=None,
                voice_profile=None,
                status="initiating",
                call_sid=call_sid,
            )

        if call_sid and not call.call_sid:
            call.call_sid = call_sid
            db.commit()

        ws_base = settings.APP_BASE_URL.replace("https://", "wss://").replace("http://", "ws://")
        ws_url = f"{ws_base}/webhooks/voice/stream/{call.id}"

        greeting = (
            "Hi. This call may be recorded to help with your request. "
            "One moment while I connect."
        )
        twiml = generate_twiml_stream(websocket_url=ws_url, greeting=greeting)
        return Response(content=twiml, media_type="text/xml")
    finally:
        db.close()


@rate_limit_webhook()
@webhook_router.post("/status")
async def voice_status_webhook(request: Request):
    """
    Twilio status callback for call lifecycle updates.
    """
    form = await request.form()
    if not _validate_twilio(request, dict(form)):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")
    call_sid = form.get("CallSid")
    call_status = form.get("CallStatus")
    duration = form.get("CallDuration")

    db = SessionLocal()
    try:
        call = db.query(VoiceCall).filter(VoiceCall.call_sid == call_sid).first()
        if not call:
            return {"ok": True}

        status_map = {
            "initiated": "initiating",
            "ringing": "ringing",
            "answered": "connected",
            "in-progress": "connected",
            "completed": "ended",
            "failed": "failed",
            "busy": "failed",
            "no-answer": "failed",
        }
        outcome_map = {
            "completed": "completed",
            "failed": "failed",
            "busy": "busy",
            "no-answer": "no_answer",
        }
        status = status_map.get(call_status, call.status)
        outcome_status = outcome_map.get(call_status)
        duration_seconds = int(duration) if duration else None
        update_call_status(
            db,
            call,
            status=status,
            duration_seconds=duration_seconds,
            outcome_status=outcome_status,
        )

        if call.proposal_id and outcome_status:
            from app.db.models import Proposal
            proposal = db.query(Proposal).filter(Proposal.id == call.proposal_id).first()
            if proposal:
                proposal.status = "completed" if outcome_status == "completed" else "failed"
                db.commit()
        return {"ok": True}
    finally:
        db.close()


@rate_limit_webhook()
@webhook_router.post("/recording")
async def voice_recording_webhook(request: Request):
    """
    Twilio recording callback to store recording URL.
    """
    form = await request.form()
    if not _validate_twilio(request, dict(form)):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")
    call_sid = form.get("CallSid")
    recording_url = form.get("RecordingUrl")

    db = SessionLocal()
    try:
        call = db.query(VoiceCall).filter(VoiceCall.call_sid == call_sid).first()
        if not call:
            return {"ok": True}
        update_call_status(db, call, status=call.status, recording_url=recording_url)
        return {"ok": True}
    finally:
        db.close()


@webhook_router.websocket("/stream/{call_id}")
async def voice_stream(websocket: WebSocket, call_id: int):
    """
    Twilio Media Streams websocket endpoint.
    """
    await websocket.accept()

    db = SessionLocal()
    call = db.query(VoiceCall).filter(VoiceCall.id == call_id).first()
    if not call:
        await websocket.close()
        db.close()
        return

    conversation = []
    send_lock = asyncio.Lock()
    stream_sid: Optional[str] = None
    voice_id = resolve_voice_id(call.voice_profile)

    async def handle_transcript(text: str, is_final: bool) -> None:
        nonlocal conversation
        if not is_final:
            return

        append_transcript(db, call, f"User: {text}")
        conversation.append({"role": "user", "content": text})

        # Simple hold detection
        if "hold" in text.lower() or "one moment" in text.lower():
            update_call_status(db, call, status="on_hold")

        try:
            reply = await asyncio.to_thread(
                generate_call_response,
                call.purpose,
                conversation,
                text,
                call.script_json,
            )
        except Exception:
            reply = "Sorry about that. Could you repeat the last part?"

        conversation.append({"role": "assistant", "content": reply})
        append_transcript(db, call, f"Assistant: {reply}")

        # Stream TTS audio back to Twilio
        async with send_lock:
            try:
                async for chunk in stream_tts_audio(reply, voice_id=voice_id):
                    if not stream_sid:
                        continue
                    payload = base64.b64encode(chunk).decode("utf-8")
                    await websocket.send_text(
                        json.dumps({
                            "event": "media",
                            "streamSid": stream_sid,
                            "media": {"payload": payload},
                        })
                    )
            except Exception:
                pass

    dg = DeepgramStream(on_transcript=handle_transcript)
    try:
        await dg.connect()

        async for message in websocket.iter_text():
            data = json.loads(message)
            event = data.get("event")

            if event == "start":
                stream_sid = data.get("start", {}).get("streamSid")
                call_sid = data.get("start", {}).get("callSid")
                if call_sid and not call.call_sid:
                    call.call_sid = call_sid
                    db.commit()
                if stream_sid:
                    set_call_stream_sid(db, call, stream_sid)
                update_call_status(db, call, status="connected")

            elif event == "media":
                payload = data.get("media", {}).get("payload", "")
                if payload:
                    audio = base64.b64decode(payload)
                    await dg.send_audio(audio)

            elif event == "dtmf":
                digit = data.get("dtmf", {}).get("digit")
                if digit:
                    append_transcript(db, call, f"User pressed: {digit}")

            elif event == "stop":
                break

    except WebSocketDisconnect:
        pass
    finally:
        try:
            await dg.close()
        except Exception:
            pass

        # Summarize call when complete
        try:
            if call.transcript:
                result = await asyncio.to_thread(summarize_call, call.transcript, call.purpose)
                summary = str(result.get("summary", ""))
                actions = result.get("action_items", []) if isinstance(result, dict) else []
                store_summary_and_actions(db, call, summary=summary, action_items=actions)
                update_call_status(
                    db,
                    call,
                    status="ended",
                    outcome_status=call.outcome_status or "completed",
                    outcome_notes=summary,
                )
            else:
                update_call_status(db, call, status="ended", outcome_status=call.outcome_status or "completed")
        except Exception:
            update_call_status(db, call, status="ended")
        db.close()
        await websocket.close()
