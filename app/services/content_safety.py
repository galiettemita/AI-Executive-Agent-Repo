from __future__ import annotations

import json
import logging
import threading
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.contracts import LLMRequest
from app.blueprint.llm.router import get_llm_router
from app.core.config import settings
from app.core.redis import get_redis
from app.db.database import SessionLocal

logger = logging.getLogger(__name__)

_THREAD_POOL = ThreadPoolExecutor(max_workers=4, thread_name_prefix="safety-classifier")
_TABLE_INIT_LOCK = threading.Lock()
_TABLE_READY = False

_PROMPT_INJECTION_MARKERS = (
    "ignore previous instructions",
    "ignore all previous instructions",
    "system prompt",
    "developer message",
    "jailbreak",
    "bypass safety",
    "reveal hidden prompt",
)

_HARASSMENT_MARKERS = (
    "i hate you",
    "kill yourself",
    "you're worthless",
    "idiot",
    "stupid",
)

_SELF_HARM_MARKERS = (
    "kill myself",
    "want to die",
    "self harm",
    "suicide",
    "hurt myself",
)

_ILLEGAL_MARKERS = (
    "make a bomb",
    "credit card fraud",
    "stolen identity",
    "buy stolen",
    "hack into",
)

_PROVISIONING_LINK_ALLOWLIST_MARKERS = (
    "/api/v1/provision/callback?state=",
    "/api/v1/provision/short/",
    "tap the secure link to authorize this server",
    "connected! return to chat.",
)


@dataclass
class SafetyVerdict:
    flagged: bool
    risk_score: float
    categories: list[str]
    reason: str
    classifier: str = "heuristic"


@dataclass
class RateLimitDecision:
    allowed: bool
    retry_after_seconds: int | None = None
    reason: str = "ok"


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(
            text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def ensure_moderation_queue_table(db: Session) -> None:
    global _TABLE_READY
    if _TABLE_READY and _table_exists(db, "moderation_queue"):
        return
    with _TABLE_INIT_LOCK:
        if _TABLE_READY and _table_exists(db, "moderation_queue"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists moderation_queue (
                      id text primary key,
                      user_id text,
                      run_id text,
                      message_direction text not null,
                      channel text,
                      content_excerpt text,
                      categories text,
                      risk_score real default 0,
                      classifier text,
                      status text not null default 'pending',
                      source text not null default 'safety_classifier',
                      metadata_json text,
                      created_at datetime default current_timestamp,
                      resolved_at datetime,
                      resolver_id text,
                      resolution_notes text
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_moderation_queue_user_status on moderation_queue(user_id, status)"))
            db.execute(text("create index if not exists idx_moderation_queue_created on moderation_queue(created_at)"))
        else:
            db.execute(
                text(
                    """
                    create table if not exists moderation_queue (
                      id text primary key,
                      user_id text,
                      run_id text,
                      message_direction text not null,
                      channel text,
                      content_excerpt text,
                      categories jsonb,
                      risk_score double precision default 0,
                      classifier text,
                      status text not null default 'pending',
                      source text not null default 'safety_classifier',
                      metadata_json jsonb,
                      created_at timestamptz not null default now(),
                      resolved_at timestamptz,
                      resolver_id text,
                      resolution_notes text
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_moderation_queue_user_status on moderation_queue(user_id, status)"))
            db.execute(text("create index if not exists idx_moderation_queue_created on moderation_queue(created_at)"))
        db.commit()
        _TABLE_READY = True


def _heuristic_classify(text_value: str) -> SafetyVerdict:
    txt = str(text_value or "").strip().lower()
    if not txt:
        return SafetyVerdict(flagged=False, risk_score=0.0, categories=[], reason="empty")

    if _is_allowlisted_provisioning_text(txt):
        return SafetyVerdict(
            flagged=False,
            risk_score=0.0,
            categories=[],
            reason="allowlisted_provisioning_link",
            classifier="heuristic_allowlist",
        )

    categories: list[str] = []
    if any(marker in txt for marker in _PROMPT_INJECTION_MARKERS):
        categories.append("prompt_injection")
    if any(marker in txt for marker in _HARASSMENT_MARKERS):
        categories.append("harassment")
    if any(marker in txt for marker in _SELF_HARM_MARKERS):
        categories.append("self_harm")
    if any(marker in txt for marker in _ILLEGAL_MARKERS):
        categories.append("illegal_activity")

    risk_score = min(1.0, 0.2 + (0.22 * len(categories))) if categories else 0.0
    return SafetyVerdict(
        flagged=bool(categories),
        risk_score=round(risk_score, 3),
        categories=categories,
        reason="heuristic_match" if categories else "clean",
        classifier="heuristic",
    )


def _is_allowlisted_provisioning_text(text_value: str) -> bool:
    txt = str(text_value or "").strip().lower()
    if not txt:
        return False
    return any(marker in txt for marker in _PROVISIONING_LINK_ALLOWLIST_MARKERS)


def _llm_classify(text_value: str) -> SafetyVerdict:
    router = get_llm_router()
    schema = {
        "type": "object",
        "properties": {
            "risk_score": {"type": "number"},
            "categories": {"type": "array"},
            "reason": {"type": "string"},
            "flagged": {"type": "boolean"},
        },
        "required": ["risk_score", "categories", "reason", "flagged"],
    }
    req = LLMRequest(
        prompt_group="evaluator_prompt_safety",
        task_type="intent_classification",
        messages=[
            {
                "role": "system",
                "content": (
                    "Classify user-generated content for safety. "
                    "Categories: prompt_injection, harassment, self_harm, illegal_activity. "
                    "Return strict JSON only."
                ),
            },
            {"role": "user", "content": str(text_value or "")[:4000]},
        ],
        temperature=0.0,
        max_tokens=180,
        structured_output=schema,
    )
    resp = router.call(req)
    payload = json.loads(resp.content or "{}")
    categories = [str(item) for item in (payload.get("categories") or []) if str(item).strip()]
    return SafetyVerdict(
        flagged=bool(payload.get("flagged") or categories),
        risk_score=float(payload.get("risk_score") or 0.0),
        categories=categories,
        reason=str(payload.get("reason") or "llm"),
        classifier="llm",
    )


def classify_content(text_value: str, *, prefer_llm: bool = False) -> SafetyVerdict:
    base = _heuristic_classify(text_value)
    if not prefer_llm:
        return base
    if settings.ENV not in {"staging", "production"}:
        return base
    if not settings.OPENAI_API_KEY:
        return base
    try:
        llm = _llm_classify(text_value)
        # Keep strongest signal.
        if llm.risk_score >= base.risk_score:
            return llm
        return base
    except Exception:
        logger.warning("llm_safety_classifier_failed", exc_info=True)
        return base


def _record_safety_flag(user_id: str) -> int:
    client = get_redis()
    if client is None:
        return 0
    now = time.time()
    key = f"bp:safety:flags:{user_id}"
    member = f"{now:.6f}:{uuid.uuid4().hex[:10]}"
    client.zadd(key, {member: now})
    client.zremrangebyscore(key, 0, now - 3600)
    client.expire(key, 3700)
    count = int(client.zcard(key))
    if count >= 3:
        client.set(f"bp:safety:circuit:{user_id}", "1", ex=3600)
    return count


def _insert_moderation_row(
    db: Session,
    *,
    user_id: str | None,
    run_id: str | None,
    direction: str,
    channel: str | None,
    text_value: str,
    verdict: SafetyVerdict,
    metadata: dict[str, Any] | None = None,
) -> None:
    ensure_moderation_queue_table(db)
    dialect = db.bind.dialect.name if db.bind is not None else ""
    payload_categories = json.dumps(verdict.categories, ensure_ascii=False)
    payload_meta = json.dumps(metadata or {}, ensure_ascii=False)
    params = {
        "id": str(uuid.uuid4()),
        "user_id": user_id,
        "run_id": run_id,
        "message_direction": direction,
        "channel": channel,
        "content_excerpt": str(text_value or "")[:600],
        "categories": payload_categories,
        "risk_score": float(verdict.risk_score),
        "classifier": verdict.classifier,
        "metadata_json": payload_meta,
    }
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into moderation_queue (
                  id, user_id, run_id, message_direction, channel, content_excerpt,
                  categories, risk_score, classifier, metadata_json, status, source
                ) values (
                  :id, :user_id, :run_id, :message_direction, :channel, :content_excerpt,
                  :categories, :risk_score, :classifier, :metadata_json, 'pending', 'safety_classifier'
                )
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                insert into moderation_queue (
                  id, user_id, run_id, message_direction, channel, content_excerpt,
                  categories, risk_score, classifier, metadata_json, status, source
                ) values (
                  :id, :user_id, :run_id, :message_direction, :channel, :content_excerpt,
                  cast(:categories as jsonb), :risk_score, :classifier, cast(:metadata_json as jsonb), 'pending', 'safety_classifier'
                )
                """
            ),
            params,
        )
    db.commit()


def enqueue_moderation_item(
    *,
    user_id: str | None,
    run_id: str | None,
    direction: str,
    channel: str | None,
    text_value: str,
    categories: list[str] | None = None,
    risk_score: float = 0.5,
    classifier: str = "system",
    metadata: dict[str, Any] | None = None,
    increment_safety_flags: bool = False,
) -> dict[str, Any]:
    verdict = SafetyVerdict(
        flagged=True,
        risk_score=max(0.0, min(1.0, float(risk_score))),
        categories=[str(item) for item in (categories or []) if str(item).strip()],
        reason="manual_enqueue",
        classifier=classifier,
    )
    db = SessionLocal()
    try:
        _insert_moderation_row(
            db,
            user_id=user_id,
            run_id=run_id,
            direction=direction,
            channel=channel,
            text_value=text_value,
            verdict=verdict,
            metadata=metadata,
        )
    finally:
        try:
            db.close()
        except Exception:
            pass
    if increment_safety_flags and user_id:
        try:
            _record_safety_flag(user_id)
        except Exception:
            logger.warning("safety_flag_record_failed user_id=%s", user_id, exc_info=True)
    return {"ok": True, "categories": verdict.categories, "risk_score": verdict.risk_score}


def classify_and_record(
    *,
    user_id: str | None,
    run_id: str | None,
    direction: str,
    channel: str | None,
    text_value: str,
    prefer_llm: bool,
    metadata: dict[str, Any] | None = None,
) -> SafetyVerdict:
    verdict = classify_content(text_value, prefer_llm=prefer_llm)
    if verdict.flagged:
        db = SessionLocal()
        try:
            _insert_moderation_row(
                db,
                user_id=user_id,
                run_id=run_id,
                direction=direction,
                channel=channel,
                text_value=text_value,
                verdict=verdict,
                metadata=metadata,
            )
        except Exception:
            logger.warning("moderation_queue_insert_failed", exc_info=True)
        finally:
            try:
                db.close()
            except Exception:
                pass
        if user_id:
            try:
                _record_safety_flag(user_id)
            except Exception:
                logger.warning("safety_flag_record_failed user_id=%s", user_id, exc_info=True)
    return verdict


def classify_input_async(
    *,
    user_id: str | None,
    run_id: str | None,
    channel: str | None,
    text_value: str,
    metadata: dict[str, Any] | None = None,
) -> None:
    if not str(text_value or "").strip():
        return
    _THREAD_POOL.submit(
        classify_and_record,
        user_id=user_id,
        run_id=run_id,
        direction="inbound",
        channel=channel,
        text_value=text_value,
        prefer_llm=True,
        metadata=metadata,
    )


def classify_output_sync(
    *,
    user_id: str | None,
    run_id: str | None,
    channel: str | None,
    text_value: str,
    metadata: dict[str, Any] | None = None,
) -> SafetyVerdict:
    return classify_and_record(
        user_id=user_id,
        run_id=run_id,
        direction="outbound",
        channel=channel,
        text_value=text_value,
        prefer_llm=False,
        metadata=metadata,
    )


def enforce_safety_circuit_rate_limit(user_id: str | None) -> RateLimitDecision:
    if not user_id:
        return RateLimitDecision(allowed=True)
    client = get_redis()
    if client is None:
        return RateLimitDecision(allowed=True)

    if not client.get(f"bp:safety:circuit:{user_id}"):
        return RateLimitDecision(allowed=True)

    now = time.time()
    key = f"bp:safety:circuit:msgs:{user_id}"
    client.zremrangebyscore(key, 0, now - 3600)
    count = int(client.zcard(key))
    if count >= 5:
        oldest = client.zrange(key, 0, 0, withscores=True)
        retry_after = None
        if oldest:
            retry_after = int(max(1.0, (oldest[0][1] + 3600) - now))
        return RateLimitDecision(allowed=False, retry_after_seconds=retry_after, reason="safety_circuit_rate_limited")
    member = f"{now:.6f}:{uuid.uuid4().hex[:8]}"
    client.zadd(key, {member: now})
    client.expire(key, 3700)
    return RateLimitDecision(allowed=True)


def enforce_gateway_burst_limit(user_id: str | None, *, limit_per_minute: int = 10) -> RateLimitDecision:
    if not user_id:
        return RateLimitDecision(allowed=True)
    client = get_redis()
    if client is None:
        return RateLimitDecision(allowed=True)

    now = time.time()
    key = f"bp:safety:burst:{user_id}"
    window = 60
    client.zremrangebyscore(key, 0, now - window)
    count = int(client.zcard(key))
    if count >= max(1, int(limit_per_minute)):
        oldest = client.zrange(key, 0, 0, withscores=True)
        retry_after = None
        if oldest:
            retry_after = int(max(1.0, (oldest[0][1] + window) - now))
        return RateLimitDecision(allowed=False, retry_after_seconds=retry_after, reason="gateway_burst_rate_limited")
    member = f"{now:.6f}:{uuid.uuid4().hex[:8]}"
    client.zadd(key, {member: now})
    client.expire(key, window + 5)
    return RateLimitDecision(allowed=True)
