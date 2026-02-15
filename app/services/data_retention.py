from __future__ import annotations

import logging
from datetime import datetime, timedelta

from sqlalchemy import and_
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import (
    AuditLog,
    ChatMessage,
    EmailAlert,
    EmailDraft,
    NotificationQueue,
    OutboundMessage,
    SmartHomeEnergyReading,
    UsageEvent,
    WatchOffer,
    WebhookDelivery,
)

logger = logging.getLogger(__name__)


def purge_expired_records(db: Session) -> dict[str, int]:
    """
    Purge data based on retention settings.

    Returns a dict of table name -> rows deleted.
    """
    now = datetime.utcnow()
    results: dict[str, int] = {}

    def _delete(model, cutoff_field, days: int, extra_filter=None) -> int:
        if days <= 0:
            return 0
        cutoff = now - timedelta(days=days)
        query = db.query(model).filter(cutoff_field < cutoff)
        if extra_filter is not None:
            query = query.filter(extra_filter)
        return query.delete(synchronize_session=False)

    results["chat_messages"] = _delete(
        ChatMessage,
        ChatMessage.created_at,
        settings.RETENTION_CHAT_MESSAGES_DAYS,
    )
    results["email_alerts"] = _delete(
        EmailAlert,
        EmailAlert.created_at,
        settings.RETENTION_EMAIL_ALERTS_DAYS,
    )
    results["email_drafts"] = _delete(
        EmailDraft,
        EmailDraft.created_at,
        settings.RETENTION_EMAIL_DRAFTS_DAYS,
        EmailDraft.status != "pending",
    )
    results["notification_queue"] = _delete(
        NotificationQueue,
        NotificationQueue.sent_at,
        settings.RETENTION_NOTIFICATION_QUEUE_DAYS,
        and_(NotificationQueue.is_sent.is_(True), NotificationQueue.sent_at.isnot(None)),
    )
    results["outbound_messages"] = _delete(
        OutboundMessage,
        OutboundMessage.created_at,
        settings.RETENTION_OUTBOUND_MESSAGES_DAYS,
        OutboundMessage.status.in_(["sent", "failed"]),
    )
    results["webhook_deliveries"] = _delete(
        WebhookDelivery,
        WebhookDelivery.created_at,
        settings.RETENTION_WEBHOOK_DELIVERIES_DAYS,
    )
    results["usage_events"] = _delete(
        UsageEvent,
        UsageEvent.created_at,
        settings.RETENTION_USAGE_EVENTS_DAYS,
    )
    results["audit_logs"] = _delete(
        AuditLog,
        AuditLog.created_at,
        settings.RETENTION_AUDIT_LOGS_DAYS,
    )
    results["watch_offers"] = _delete(
        WatchOffer,
        WatchOffer.fetched_at,
        settings.RETENTION_WATCH_OFFERS_DAYS,
    )
    results["smart_home_energy_readings"] = _delete(
        SmartHomeEnergyReading,
        SmartHomeEnergyReading.reading_time,
        settings.RETENTION_SMART_HOME_READINGS_DAYS,
    )

    db.commit()
    logger.info("Data retention purge completed: %s", results)
    return results
