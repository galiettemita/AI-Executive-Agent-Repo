from __future__ import annotations

import json
import logging
import math
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.core.alerting import send_alert
from app.db.models import EmailAlert, EmailMonitorConfig, NotificationQueue
from app.services.analytics_service import record_usage_event
from app.services.email_intelligence import summarize_inbox
from app.services.email_router import list_recent_emails

logger = logging.getLogger(__name__)


def _load_list(raw: Optional[str]) -> List[str]:
    if not raw:
        return []
    try:
        data = json.loads(raw)
        if isinstance(data, list):
            return [str(item) for item in data if item is not None]
    except Exception:
        return []
    return []


def _dump_list(values: Optional[List[str]]) -> str:
    if not values:
        return "[]"
    return json.dumps([v for v in values if v], ensure_ascii=False)


def list_email_monitor_configs(db: Session, user_id: str) -> List[EmailMonitorConfig]:
    return (
        db.query(EmailMonitorConfig)
        .filter(EmailMonitorConfig.user_id == user_id)
        .order_by(EmailMonitorConfig.created_at.desc())
        .all()
    )


def list_email_alerts(db: Session, user_id: str, limit: int = 50) -> List[EmailAlert]:
    return (
        db.query(EmailAlert)
        .filter(EmailAlert.user_id == user_id)
        .order_by(EmailAlert.created_at.desc())
        .limit(limit)
        .all()
    )


def upsert_email_monitor_config(
    db: Session,
    *,
    user_id: str,
    config_id: Optional[int] = None,
    provider: Optional[str] = None,
    enabled: bool = True,
    keywords: Optional[List[str]] = None,
    senders: Optional[List[str]] = None,
    subject_keywords: Optional[List[str]] = None,
    priority_threshold: Optional[int] = None,
    use_ai_priority: bool = False,
    alert_channel: str = "whatsapp",
    alert_title: Optional[str] = None,
    window_minutes: int = 60,
    max_results: int = 20,
) -> EmailMonitorConfig:
    config = None
    if config_id:
        config = (
            db.query(EmailMonitorConfig)
            .filter(EmailMonitorConfig.id == config_id, EmailMonitorConfig.user_id == user_id)
            .first()
        )
    if not config:
        config = EmailMonitorConfig(user_id=user_id)

    config.provider = provider
    config.enabled = bool(enabled)
    config.keywords_json = _dump_list(keywords)
    config.sender_allowlist_json = _dump_list(senders)
    config.subject_keywords_json = _dump_list(subject_keywords)
    config.priority_threshold = priority_threshold
    config.use_ai_priority = bool(use_ai_priority)
    channel = (alert_channel or "whatsapp").lower()
    allowed = {"whatsapp", "alerting", "slack", "pagerduty", "sentry"}
    if channel not in allowed:
        channel = "whatsapp"
    config.alert_channel = channel
    config.alert_title = alert_title
    config.window_minutes = max(5, int(window_minutes or 60))
    config.max_results = max(5, int(max_results or 20))
    config.updated_at = datetime.utcnow()

    db.add(config)
    db.commit()
    db.refresh(config)
    return config


def _match_keywords(text: str, keywords: List[str]) -> Optional[str]:
    if not text or not keywords:
        return None
    lower = text.lower()
    for kw in keywords:
        needle = kw.strip().lower()
        if needle and needle in lower:
            return kw
    return None


def _build_alert_message(msg: Dict[str, Any], reason: str, priority: Optional[int]) -> str:
    subject = msg.get("subject") or "(no subject)"
    sender = msg.get("from") or "unknown sender"
    snippet = msg.get("snippet") or ""
    pr = f"Priority {priority}" if priority else ""
    header = f"{subject}\nFrom: {sender}"
    if pr:
        header += f"\n{pr}"
    if reason:
        header += f"\nMatch: {reason}"
    if snippet:
        header += f"\n\n{snippet}"
    return header


def _ensure_alert_record(
    db: Session,
    *,
    user_id: str,
    provider: Optional[str],
    message_id: str,
    subject: Optional[str],
    sender: Optional[str],
    priority: Optional[int],
    reason: Optional[str],
    alert_channel: str,
) -> bool:
    alert = EmailAlert(
        user_id=user_id,
        provider=provider,
        message_id=message_id,
        subject=subject,
        sender=sender,
        priority=priority,
        reason=reason,
        alert_channel=alert_channel,
        created_at=datetime.utcnow(),
    )
    db.add(alert)
    try:
        db.commit()
        return True
    except IntegrityError:
        db.rollback()
        return False


def run_email_monitoring(db: Session, user_id: Optional[str] = None) -> Dict[str, int]:
    query = db.query(EmailMonitorConfig).filter(EmailMonitorConfig.enabled == True)  # noqa: E712
    if user_id:
        query = query.filter(EmailMonitorConfig.user_id == user_id)

    configs = query.all()
    if not configs:
        return {"configs": 0, "alerts": 0, "errors": 0}

    alerts_sent = 0
    errors = 0

    for config in configs:
        try:
            keywords = _load_list(config.keywords_json)
            subject_keywords = _load_list(config.subject_keywords_json)
            senders = _load_list(config.sender_allowlist_json)

            hours_back = max(1, int(math.ceil(config.window_minutes / 60)))
            emails = list_recent_emails(
                db=db,
                user_id=config.user_id,
                max_results=config.max_results,
                hours_back=hours_back,
                unread_only=True,
                provider=config.provider,
                include_body=bool(keywords or subject_keywords),
            )

            priority_map: Dict[str, int] = {}
            if config.use_ai_priority and config.priority_threshold:
                summary = summarize_inbox(
                    db=db,
                    user_id=config.user_id,
                    max_results=config.max_results,
                    hours_back=hours_back,
                    provider=config.provider,
                )
                for item in summary.get("priorities", []):
                    if item.get("id"):
                        try:
                            priority_map[str(item.get("id"))] = int(item.get("priority", 0))
                        except Exception:
                            continue

            for msg in emails:
                message_id = msg.get("id")
                if not message_id:
                    continue
                message_id = str(message_id)

                priority = priority_map.get(message_id)
                reason = None

                if senders:
                    sender_match = _match_keywords(msg.get("from") or "", senders)
                    if sender_match:
                        reason = f"sender match: {sender_match}"

                if not reason and subject_keywords:
                    subject_match = _match_keywords(msg.get("subject") or "", subject_keywords)
                    if subject_match:
                        reason = f"subject match: {subject_match}"

                if not reason and keywords:
                    body = (msg.get("body") or "") + " " + (msg.get("snippet") or "")
                    keyword_match = _match_keywords(body, keywords)
                    if keyword_match:
                        reason = f"keyword match: {keyword_match}"

                meets_priority = False
                if config.priority_threshold and priority is not None:
                    meets_priority = priority >= config.priority_threshold

                if not reason and not meets_priority:
                    continue

                if not _ensure_alert_record(
                    db,
                    user_id=config.user_id,
                    provider=config.provider,
                    message_id=message_id,
                    subject=msg.get("subject"),
                    sender=msg.get("from"),
                    priority=priority,
                    reason=reason,
                    alert_channel=config.alert_channel,
                ):
                    continue

                alert_message = _build_alert_message(msg, reason or "", priority)
                title = config.alert_title or "Important email"

                if config.alert_channel == "whatsapp":
                    db.add(
                        NotificationQueue(
                            user_id=config.user_id,
                            event_type="email_alert",
                            title=title,
                            message=alert_message,
                            deep_link_url=None,
                            is_sent=False,
                        )
                    )
                    db.commit()
                else:
                    try:
                        provider = config.alert_channel if config.alert_channel in {"slack", "pagerduty", "sentry"} else None
                        send_alert(f"{title}\n\n{alert_message}", provider=provider)
                    except Exception as exc:
                        logger.warning("Alerting provider failed: %s", exc)

                record_usage_event(
                    db,
                    user_id=config.user_id,
                    event_type="email_monitor_alert",
                    source="email_monitoring",
                    channel=config.alert_channel,
                    metadata={"message_id": message_id, "reason": reason, "priority": priority},
                )
                alerts_sent += 1

            config.last_checked_at = datetime.utcnow()
            db.commit()

        except Exception as exc:
            errors += 1
            logger.warning("Email monitoring failed for user %s: %s", config.user_id, exc)

    return {"configs": len(configs), "alerts": alerts_sent, "errors": errors}
