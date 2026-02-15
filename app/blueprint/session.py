from __future__ import annotations

import json
import time
from dataclasses import dataclass

import redis
from sqlalchemy.orm import Session

from app.blueprint.contracts import Channel
from app.blueprint.db import get_or_create_conversation


SESSION_TTL_SECONDS = 60 * 60 * 24  # 24h
INACTIVITY_SECONDS = 60 * 30  # 30m


def _session_key(user_id: str, channel: Channel) -> str:
    return f"bp:session:{channel.value}:{user_id}"


def _dedup_key(channel: Channel, channel_msg_id: str) -> str:
    return f"bp:dedup:{channel.value}:{channel_msg_id}"


@dataclass(frozen=True)
class ConversationSession:
    conversation_id: str
    last_active_ts: int


def dedup_inbound(
    r: redis.Redis,
    *,
    channel: Channel,
    channel_msg_id: str,
    ttl_seconds: int = 60 * 60 * 24,  # 24h
) -> bool:
    """
    Returns True if this message is NEW and should be processed.
    Returns False if already seen.
    """
    key = _dedup_key(channel, channel_msg_id)
    # NX == only set if not exists
    ok = r.set(key, "1", nx=True, ex=ttl_seconds)
    return bool(ok)


def get_or_create_conversation_session(
    *,
    db: Session,
    r: redis.Redis,
    user_id: str,
    channel: Channel,
) -> str:
    """
    Session manager: reuse conversation within inactivity window, otherwise create a new one.
    """
    key = _session_key(user_id, channel)
    now = int(time.time())

    raw = r.get(key)
    if raw:
        try:
            data = json.loads(raw)
            convo_id = str(data.get("conversation_id") or "")
            last_active = int(data.get("last_active_ts") or 0)
            if convo_id and (now - last_active) <= INACTIVITY_SECONDS:
                # Refresh last active time only.
                r.set(
                    key,
                    json.dumps({"conversation_id": convo_id, "last_active_ts": now}),
                    ex=SESSION_TTL_SECONDS,
                )
                return convo_id
        except Exception:
            pass

    convo_id = get_or_create_conversation(db, user_id=user_id, channel=channel)
    r.set(
        key,
        json.dumps({"conversation_id": convo_id, "last_active_ts": now}),
        ex=SESSION_TTL_SECONDS,
    )
    return convo_id

