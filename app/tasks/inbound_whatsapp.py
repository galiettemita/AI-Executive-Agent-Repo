from __future__ import annotations

import logging
import time
from typing import Any

from sqlalchemy.orm import Session

from app.blueprint.brain.responder import generate_reply
from app.blueprint.brain.tier_router import route_tier
from app.blueprint.contracts import Channel, MessageDirection
from app.blueprint.db import get_or_create_user_by_phone, insert_message
from app.blueprint.session import get_or_create_conversation_session
from app.channels.whatsapp import send_whatsapp_text
from app.core.celery_app import celery_app
from app.core.redis import get_redis
from app.db.database import SessionLocal

logger = logging.getLogger(__name__)


def _db() -> Session:
    return SessionLocal()


@celery_app.task(name="gateway.process_whatsapp_inbound")
def process_whatsapp_inbound(event: dict[str, Any]) -> dict[str, Any]:
    """
    Gateway async handler (Phase 1).
    Stores inbound/outbound messages in blueprint tables and sends WhatsApp reply.

    Expected event:
      {
        "external_id": "...",
        "from": "+1555...",
        "type": "text"|"location"|...,
        "text": "...",
        "location": {...},
        "raw": {...}
      }
    """
    started = time.perf_counter()
    r = get_redis()
    if r is None:
        raise RuntimeError("REDIS_URL not configured; async processing requires Redis")

    external_id = str(event.get("external_id") or "").strip()
    from_phone = str(event.get("from") or "").strip()
    event_type = str(event.get("type") or "text").strip()
    text = str(event.get("text") or "").strip()

    db = _db()
    try:
        user = get_or_create_user_by_phone(db, from_phone)
        conversation_id = get_or_create_conversation_session(
            db=db,
            r=r,
            user_id=user.id,
            channel=Channel.WHATSAPP,
        )

        inbound_content: dict[str, Any]
        if event_type == "location":
            inbound_content = {"location": event.get("location") or {}}
        else:
            inbound_content = {"text": text}

        inserted = insert_message(
            db,
            conversation_id=conversation_id,
            user_id=user.id,
            direction=MessageDirection.INBOUND,
            content=inbound_content,
            channel_msg_id=external_id or None,
        )
        if inserted is None:
            return {"ok": True, "deduped": True}

        tier = route_tier(text)
        reply, meta = generate_reply(user_text=text, tier=tier)

        insert_message(
            db,
            conversation_id=conversation_id,
            user_id=user.id,
            direction=MessageDirection.OUTBOUND,
            content={"text": reply},
            intent="general",
            tier=tier,
            latency_ms=int((time.perf_counter() - started) * 1000),
            cost_cents=0,
        )

        # Send outbound message (no-op if WA creds not configured)
        send_whatsapp_text(to_phone_e164=from_phone, text=reply)

        return {
            "ok": True,
            "user_id": user.id,
            "conversation_id": conversation_id,
            "tier": tier,
            "meta": meta,
        }
    except Exception:
        logger.exception("process_whatsapp_inbound failed")
        raise
    finally:
        try:
            db.close()
        except Exception:
            pass

