# app/api/routes/voice.py

from __future__ import annotations

import asyncio
import base64
import json
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request, WebSocket, WebSocketDisconnect
from fastapi.responses import Response
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db, SessionLocal
from app.db.models import VoiceCall
from app.services.voice_call_service import (
    create_voice_call,
    update_call_status,
    set_call_stream_sid,
    append_transcript,
    store_summary_and_actions,
)
from app.services.twilio_voice import create_outbound_call, generate_twiml_stream
from app.services.voice_ai import generate_call_response, summarize_call
from app.services.voice_profiles import resolve_voice_id
from app.services.stt_deepgram import DeepgramStream
from app.services.tts_elevenlabs import stream_tts_audio
from app.core.config import settings


router = APIRouter(prefix="/voice", tags=["voice"])
webhook_router = APIRouter(prefix="/webhooks/voice", tags=["webhooks"])


class OutboundCallRequest(BaseModel):
    user_id: str
    to_number: str
    purpose: Optional[str] = None
    voice_profile: Optional[str] = None
    from_number: Optional[str] = None


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


@router.post("/outbound")
def start_outbound_call(request: OutboundCallRequest, db: Session = Depends(get_db)):
    """
    Initiate an outbound call via Twilio Voice.
    """
    app_base_url = settings.APP_BASE_URL
    if not app_base_url:
        raise HTTPException(status_code=500, detail="APP_BASE_URL not configured")

    call = create_voice_call(
        db=db,
        user_id=request.user_id,
        direction="outbound",
        to_number=request.to_number,
        from_number=request.from_number,
        purpose=request.purpose,
        voice_profile=request.voice_profile,
        status="initiating",
    )

    twiml_url = f"{app_base_url}/webhooks/voice/twiml?call_id={call.id}"
    status_url = f"{app_base_url}/webhooks/voice/status"
    recording_url = f"{app_base_url}/webhooks/voice/recording"

    call_sid = create_outbound_call(
        to_number=request.to_number,
        from_number=request.from_number,
        twiml_url=twiml_url,
        status_callback_url=status_url,
        recording_status_callback_url=recording_url,
    )

    call.call_sid = call_sid
    call.status = "ringing"
    db.commit()
    db.refresh(call)

    return {"ok": True, "call_id": call.id, "call_sid": call_sid}


@webhook_router.api_route("/twiml", methods=["GET", "POST"])
async def twiml_webhook(request: Request):
    """
    Twilio webhook to fetch TwiML for inbound/outbound calls.
    """
    form = await request.form() if request.method == "POST" else {}
    params = dict(request.query_params)

    def _get(key: str) -> Optional[str]:
        return form.get(key) or params.get(key)

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


@webhook_router.post("/status")
async def voice_status_webhook(request: Request):
    """
    Twilio status callback for call lifecycle updates.
    """
    form = await request.form()
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
        status = status_map.get(call_status, call.status)
        duration_seconds = int(duration) if duration else None
        update_call_status(db, call, status=status, duration_seconds=duration_seconds)
        return {"ok": True}
    finally:
        db.close()


@webhook_router.post("/recording")
async def voice_recording_webhook(request: Request):
    """
    Twilio recording callback to store recording URL.
    """
    form = await request.form()
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
        except Exception:
            pass

        update_call_status(db, call, status="ended")
        db.close()
        await websocket.close()
