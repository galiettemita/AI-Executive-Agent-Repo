# backend/app/services/scheduler.py

from __future__ import annotations

import asyncio
import logging
from datetime import datetime
from typing import List

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from sqlalchemy.orm import Session
from sqlalchemy import text

from app.core.config import settings
from app.db.database import SessionLocal
from app.db.models import OAuthToken, User, WatchItem, SmartHomeEnergyAlert
from app.services.daily_brief import generate_and_store_daily_brief
from app.services.notification_delivery import deliver_pending_notifications
from app.services.messaging_service import deliver_pending_messages
from app.services.smart_home_service import evaluate_energy_alerts
from app.services.proactive_rules import run_due_rules
from app.services.voice_retention import purge_expired_calls
from app.services.data_retention import purge_expired_records
from app.services.email_monitoring import run_email_monitoring as run_email_monitoring_service
from app.services.wardrobe_rotation import run_rotation_for_all_users
from app.services.gift_reminders import enqueue_gift_reminders
from app.services.relationship_service import enqueue_relationship_reminders
from app.services.account_deletion_pipeline import run_due_account_deletion_jobs
from app.services.scheduled_notifications import run_due_scheduled_notifications
from app.blueprint.bones import refresh_bones_catalog
from app.blueprint.embedding_audit import run_embedding_reembed_audit_all_users
from app.blueprint.knowledge_review import run_nightly_consolidation, run_weekly_self_review
from app.blueprint.muscles import capture_muscles_snapshot
from app.blueprint.research import run_research_job
from app.core.redis import get_redis

logger = logging.getLogger(__name__)


def run_daily_briefs_for_all_users():
    """
    Run daily briefs for all users who have Google OAuth connected.
    This function is called by the scheduler.
    """
    db = SessionLocal()

    try:
        # Get all users with Google OAuth tokens
        oauth_users = (
            db.query(OAuthToken)
            .filter(OAuthToken.provider == "google")
            .all()
        )

        user_ids = [token.user_id for token in oauth_users]

        logger.info("Running daily briefs for %d users with Google OAuth", len(user_ids))

        success_count = 0
        error_count = 0

        for user_id in user_ids:
            try:
                result = generate_and_store_daily_brief(db=db, user_id=user_id)

                if result.get("success"):
                    success_count += 1
                    logger.info("Daily brief generated for user %s", user_id)
                else:
                    error_count += 1
                    logger.error("Failed to generate brief for user %s: %s", user_id, result.get("error"))

            except Exception as e:
                error_count += 1
                logger.error("Error generating brief for user %s: %s", user_id, e)

        logger.info("Daily briefs completed: %d success, %d errors", success_count, error_count)

    except Exception as e:
        logger.error("Fatal error in daily brief job: %s", e)
    finally:
        db.close()


def run_price_monitoring():
    """
    Run price monitoring for all users with active watch items.
    Refreshes prices and detects price drops/target hits.
    """
    db = SessionLocal()

    try:
        # Import here to avoid circular imports
        from app.services.price_lookup import lookup_price_google_shopping, PriceLookupError
        from app.db.models import WatchOffer, NotificationQueue
        from datetime import datetime, timedelta

        # Get all watch items
        watch_items = db.query(WatchItem).all()

        if not watch_items:
            logger.info("No watch items to monitor")
            return

        logger.info("Monitoring %d watch items", len(watch_items))

        updated_count = 0
        error_count = 0

        for item in watch_items:
            try:
                q = item.title_hint or item.url
                prev_best_price = item.best_price
                prev_last_seen = item.last_seen_price

                # Lookup current price (async function, need to run in event loop)
                best_price, currency, best_retailer, best_offer_url, best_meta, offers = asyncio.run(lookup_price_google_shopping(q))

                # Update last checked timestamp
                item.last_checked_at = datetime.utcnow()

                if best_price is not None:
                    item.last_seen_price = best_price
                    item.best_price = best_price
                    item.best_retailer = best_retailer
                    item.best_offer_url = best_offer_url
                    item.currency = currency

                    if isinstance(best_meta, dict):
                        item.best_title = best_meta.get("title")
                        item.product_key = best_meta.get("product_key")

                    updated_count += 1

                    # Price drop detection
                    did_drop = (prev_best_price is not None and best_price < prev_best_price)

                    # Target hit detection
                    target_hit = False
                    if item.desired_price is not None:
                        target = float(item.desired_price)
                        if prev_best_price is not None:
                            target_hit = (target < prev_best_price) and (prev_best_price > target) and (best_price <= target)

                    # Check if already queued (avoid duplicates)
                    def _already_queued(event_type: str) -> bool:
                        existing = (
                            db.query(NotificationQueue)
                            .filter(
                                NotificationQueue.user_id == item.user_id,
                                NotificationQueue.watch_item_id == item.id,
                                NotificationQueue.event_type == event_type,
                                NotificationQueue.sent_at.is_(None),
                                NotificationQueue.new_price == best_price,
                            )
                            .first()
                        )
                        return existing is not None

                    # Queue notifications
                    if did_drop and not _already_queued("price_drop"):
                        db.add(
                            NotificationQueue(
                                user_id=item.user_id,
                                watch_item_id=item.id,
                                event_type="price_drop",
                                title="Price dropped",
                                message=f"{item.title_hint or 'Item'} dropped from {prev_best_price} → {best_price} {currency}",
                                deep_link_url=best_offer_url or item.url,
                                prev_price=prev_best_price,
                                new_price=best_price,
                                currency=currency,
                                is_sent=False,
                            )
                        )
                        logger.info("Price drop detected for item %s", item.id)

                    if target_hit and not _already_queued("target_hit"):
                        db.add(
                            NotificationQueue(
                                user_id=item.user_id,
                                watch_item_id=item.id,
                                event_type="target_hit",
                                title="Target price hit",
                                message=f"{item.title_hint or 'Item'} is now {best_price} {currency} (target {item.desired_price})",
                                deep_link_url=best_offer_url or item.url,
                                prev_price=prev_last_seen,
                                new_price=best_price,
                                currency=currency,
                                is_sent=False,
                            )
                        )
                        logger.info("Target hit for item %s", item.id)

            except Exception as e:
                error_count += 1
                logger.error("Error monitoring item %s: %s", item.id, e)

        db.commit()
        logger.info("Price monitoring completed: %d updated, %d errors", updated_count, error_count)

    except Exception as e:
        logger.error("Fatal error in price monitoring job: %s", e)
    finally:
        db.close()


def run_notification_delivery():
    """
    Deliver all pending notifications via WhatsApp.
    """
    db = SessionLocal()

    try:
        result = deliver_pending_notifications(db)
        logger.info("Notification delivery completed: %s", result)

    except Exception as e:
        logger.error("Fatal error in notification delivery job: %s", e)
    finally:
        db.close()


def run_scheduled_notifications():
    """
    Deliver due scheduled notifications with quiet-hours and per-user rate limits.
    """
    db = SessionLocal()
    try:
        result = run_due_scheduled_notifications(db)
        logger.info("Scheduled notifications run completed: %s", result)
    except Exception as e:
        logger.error("Fatal error in scheduled notifications job: %s", e)
    finally:
        db.close()


def run_outbound_messages():
    """
    Deliver queued outbound messages (WhatsApp/SMS/etc).
    """
    db = SessionLocal()
    try:
        result = deliver_pending_messages(db)
        logger.info("Outbound messaging completed: %s", result)
    except Exception as e:
        logger.error("Fatal error in outbound messaging job: %s", e)
    finally:
        db.close()


def reset_billing_daily_counters() -> None:
    """
    Reset Redis-backed daily message counters at midnight UTC.

    Primary enforcement uses per-key expiry-to-midnight; this job is a safety net.
    """
    r = get_redis()
    if r is None:
        return
    deleted = 0
    batch: list[str] = []
    try:
        for key in r.scan_iter(match="billing:daily:*", count=500):
            batch.append(key)
            if len(batch) >= 500:
                deleted += int(r.delete(*batch) or 0)
                batch = []
        if batch:
            deleted += int(r.delete(*batch) or 0)
    except Exception:
        logger.exception("Billing daily counter reset failed")
        return
    logger.info("Billing daily counters reset: deleted=%s", deleted)


def run_proactive_rules():
    """
    Evaluate proactive rules and enqueue actions.
    """
    db = SessionLocal()
    try:
        result = run_due_rules(db)
        logger.info("Proactive rules run: %s", result)
    except Exception as e:
        logger.error("Fatal error in proactive rules job: %s", e)
    finally:
        db.close()


def run_energy_monitoring():
    """
    Poll energy sensors and send alerts for smart home energy thresholds.
    """
    db = SessionLocal()
    try:
        providers = (
            db.query(SmartHomeEnergyAlert.provider)
            .distinct()
            .all()
        )
        for (provider,) in providers:
            evaluate_energy_alerts(db, provider_name=provider)
    except Exception as e:
        logger.error("Fatal error in energy monitoring job: %s", e)
    finally:
        db.close()


def run_email_monitoring():
    """
    Run email monitoring for all configured users.
    """
    db = SessionLocal()
    try:
        result = run_email_monitoring_service(db)
        logger.info("Email monitoring completed: %s", result)
    except Exception as e:
        logger.error("Fatal error in email monitoring job: %s", e)
    finally:
        db.close()


def run_voice_retention():
    """
    Purge expired voice call recordings/transcripts based on retention policy.
    """
    db = SessionLocal()
    try:
        retention_days = settings.VOICE_RECORDING_RETENTION_DAYS
        purged = purge_expired_calls(db, retention_days)
        logger.info("Voice retention purge completed: %d calls redacted", purged)
    except Exception as e:
        logger.error("Fatal error in voice retention job: %s", e)
    finally:
        db.close()

def run_data_retention():
    """
    Purge expired data based on retention policy.
    """
    db = SessionLocal()
    try:
        result = purge_expired_records(db)
        logger.info("Data retention purge completed: %s", result)
    except Exception as e:
        logger.error("Fatal error in data retention job: %s", e)
    finally:
        db.close()


def run_account_deletion_pipeline():
    """
    Process due account deletion pipeline jobs (24h/7d/30d stages).
    """
    db = SessionLocal()
    try:
        result = run_due_account_deletion_jobs(db, limit=100)
        logger.info("Account deletion pipeline run: %s", result)
    except Exception as e:
        logger.error("Fatal error in account deletion pipeline job: %s", e)
    finally:
        db.close()


def run_wardrobe_rotation():
    """
    Queue wardrobe rotation reminders for users with stale items.
    """
    db = SessionLocal()
    try:
        result = run_rotation_for_all_users(db)
        logger.info("Wardrobe rotation reminders queued: %s", result)
    except Exception as e:
        logger.error("Fatal error in wardrobe rotation job: %s", e)
    finally:
        db.close()


def run_gift_reminders():
    """
    Queue gift reminders for upcoming occasions.
    """
    db = SessionLocal()
    try:
        result = enqueue_gift_reminders(db)
        logger.info("Gift reminders queued: %s", result)
    except Exception as e:
        logger.error("Fatal error in gift reminders job: %s", e)
    finally:
        db.close()


def run_relationship_reminders():
    """
    Queue relationship reach-out reminders based on cadence.
    """
    db = SessionLocal()
    try:
        result = enqueue_relationship_reminders(db, limit_per_user=settings.RELATIONSHIP_REMINDER_MAX_PER_USER)
        logger.info("Relationship reminders queued: %s", result)
    except Exception as e:
        logger.error("Fatal error in relationship reminders job: %s", e)
    finally:
        db.close()


def _distinct_blueprint_user_ids(db: Session) -> list[str]:
    user_ids: list[str] = []
    try:
        rows = db.execute(
            text("select distinct user_id from knowledge_files where user_id is not null")
        ).mappings().all()
        user_ids = [str(r.get("user_id") or "").strip() for r in rows if str(r.get("user_id") or "").strip()]
    except Exception:
        user_ids = []
    return sorted(set(user_ids))


def run_knowledge_consolidation():
    db = SessionLocal()
    try:
        user_ids = _distinct_blueprint_user_ids(db)
        results = []
        for user_id in user_ids:
            try:
                results.append(run_nightly_consolidation(db, user_id=user_id))
            except Exception as exc:
                results.append({"ok": False, "user_id": user_id, "error": str(exc)})
        logger.info("Knowledge consolidation run complete for %d users", len(user_ids))
        return {"users": len(user_ids), "results": results}
    except Exception as e:
        logger.error("Fatal error in knowledge consolidation job: %s", e)
        return {"users": 0, "results": [], "error": str(e)}
    finally:
        db.close()


def run_self_review():
    db = SessionLocal()
    try:
        user_ids = _distinct_blueprint_user_ids(db)
        results = []
        for user_id in user_ids:
            try:
                results.append(run_weekly_self_review(db, user_id=user_id))
            except Exception as exc:
                results.append({"ok": False, "user_id": user_id, "error": str(exc)})
        logger.info("Self-review run complete for %d users", len(user_ids))
        return {"users": len(user_ids), "results": results}
    except Exception as e:
        logger.error("Fatal error in self-review job: %s", e)
        return {"users": 0, "results": [], "error": str(e)}
    finally:
        db.close()


def run_due_research():
    db = SessionLocal()
    try:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        due_predicate = "(next_run_at is null or next_run_at <= :now)" if dialect == "sqlite" else "(next_run_at is null or next_run_at <= now())"
        rows = db.execute(
            text(
                """
                select id, user_id
                from research_jobs
                where status = 'active'
                  and """
                + due_predicate
                + """
                limit 50
                """
            ),
            {"now": datetime.utcnow()} if dialect == "sqlite" else {},
        ).mappings().all()
        if not rows:
            return {"processed": 0}
        processed = 0
        for row in rows:
            rid = str(row.get("id") or "")
            uid = str(row.get("user_id") or "")
            if not rid or not uid:
                continue
            try:
                run_research_job(db, user_id=uid, research_id=rid)
                processed += 1
            except Exception as exc:
                logger.warning("Research job failed id=%s: %s", rid, exc)
        logger.info("Research scheduler processed %d jobs", processed)
        return {"processed": processed}
    except Exception as e:
        logger.error("Fatal error in research scheduler: %s", e)
        return {"processed": 0, "error": str(e)}
    finally:
        db.close()


def run_bones_refresh():
    db = SessionLocal()
    try:
        user_ids = _distinct_blueprint_user_ids(db)
        results = []
        for user_id in user_ids:
            try:
                results.append(refresh_bones_catalog(db, user_id=user_id))
            except Exception as exc:
                results.append({"ok": False, "user_id": user_id, "error": str(exc)})
        logger.info("Bones refresh complete for %d users", len(user_ids))
        return {"users": len(user_ids), "results": results}
    except Exception as e:
        logger.error("Fatal error in bones refresh job: %s", e)
        return {"users": 0, "results": [], "error": str(e)}
    finally:
        db.close()


def run_muscles_snapshot():
    db = SessionLocal()
    try:
        user_ids = _distinct_blueprint_user_ids(db)
        results = []
        for user_id in user_ids:
            try:
                results.append(capture_muscles_snapshot(db, user_id=user_id))
            except Exception as exc:
                results.append({"ok": False, "user_id": user_id, "error": str(exc)})
        logger.info("Muscles snapshot complete for %d users", len(user_ids))
        return {"users": len(user_ids), "results": results}
    except Exception as e:
        logger.error("Fatal error in muscles snapshot job: %s", e)
        return {"users": 0, "results": [], "error": str(e)}
    finally:
        db.close()


def run_embedding_audit():
    db = SessionLocal()
    try:
        result = run_embedding_reembed_audit_all_users(db)
        logger.info("Embedding audit job complete: %s", result)
        return result
    except Exception as e:
        logger.error("Fatal error in embedding audit job: %s", e)
        return {"ok": False, "error": str(e)}
    finally:
        db.close()


def setup_scheduler() -> BackgroundScheduler:
    """
    Set up the APScheduler for background jobs.

    Returns:
        Configured BackgroundScheduler instance
    """
    scheduler = BackgroundScheduler()

    # Get schedule from settings
    schedule_time = settings.DAILY_BRIEF_SCHEDULE.split()
    hour = int(schedule_time[0])
    minute = int(schedule_time[1]) if len(schedule_time) > 1 else 0

    # Add daily brief job
    scheduler.add_job(
        run_daily_briefs_for_all_users,
        trigger=CronTrigger(hour=hour, minute=minute),
        id="daily_brief_job",
        name="Daily Brief for All Users",
        replace_existing=True,
    )

    logger.info("Daily brief job scheduled for %02d:%02d UTC daily", hour, minute)

    # Add price monitoring job
    scheduler.add_job(
        run_price_monitoring,
        trigger="interval",
        minutes=settings.PRICE_MONITORING_INTERVAL_MINUTES,
        id="price_monitoring_job",
        name="Price Monitoring for Watch Items",
        replace_existing=True,
    )

    logger.info("Price monitoring job scheduled every %d minutes", settings.PRICE_MONITORING_INTERVAL_MINUTES)

    # Add notification delivery job
    scheduler.add_job(
        run_notification_delivery,
        trigger="interval",
        minutes=settings.NOTIFICATION_DELIVERY_INTERVAL_MINUTES,
        id="notification_delivery_job",
        name="Notification Delivery",
        replace_existing=True,
    )

    logger.info("Notification delivery job scheduled every %d minutes", settings.NOTIFICATION_DELIVERY_INTERVAL_MINUTES)

    scheduler.add_job(
        run_scheduled_notifications,
        trigger="interval",
        minutes=5,
        id="scheduled_notifications_job",
        name="Scheduled Notifications",
        replace_existing=True,
    )
    logger.info("Scheduled notifications job runs every 5 minutes")

    if settings.ENABLE_MESSAGING == "1":
        scheduler.add_job(
            run_outbound_messages,
            trigger="interval",
            minutes=1,
            id="outbound_messages_job",
            name="Outbound Messages",
            replace_existing=True,
        )
        logger.info("Outbound messaging job scheduled every 1 minute")
    else:
        logger.info("Messaging disabled; skipping outbound messaging scheduler")

    # Add proactive rules job
    scheduler.add_job(
        run_proactive_rules,
        trigger="interval",
        minutes=settings.PROACTIVE_RULE_POLL_MINUTES,
        id="proactive_rules_job",
        name="Proactive Rules",
        replace_existing=True,
    )

    logger.info("Proactive rules job scheduled every %d minutes", settings.PROACTIVE_RULE_POLL_MINUTES)

    if settings.ENABLE_SMART_HOME == "1":
        # Add energy monitoring job
        scheduler.add_job(
            run_energy_monitoring,
            trigger="interval",
            minutes=settings.ENERGY_MONITOR_INTERVAL_MINUTES,
            id="energy_monitoring_job",
            name="Energy Monitoring",
            replace_existing=True,
        )

        logger.info(
            "Energy monitoring job scheduled every %d minutes",
            settings.ENERGY_MONITOR_INTERVAL_MINUTES,
        )
    else:
        logger.info("Smart home disabled; skipping energy monitoring scheduler")

    # Email monitoring job
    scheduler.add_job(
        run_email_monitoring,
        trigger="interval",
        minutes=settings.EMAIL_MONITOR_INTERVAL_MINUTES,
        id="email_monitoring_job",
        name="Email Monitoring",
        replace_existing=True,
    )
    logger.info("Email monitoring job scheduled every %d minutes", settings.EMAIL_MONITOR_INTERVAL_MINUTES)

    # Voice data retention job (daily)
    scheduler.add_job(
        run_voice_retention,
        trigger=CronTrigger(hour=2, minute=15),
        id="voice_retention_job",
        name="Voice Data Retention",
        replace_existing=True,
    )
    logger.info("Voice retention job scheduled daily at 02:15 UTC")

    # Data retention job (daily)
    retention_time = settings.DATA_RETENTION_SCHEDULE.split()
    retention_hour = int(retention_time[0])
    retention_minute = int(retention_time[1]) if len(retention_time) > 1 else 0
    scheduler.add_job(
        run_data_retention,
        trigger=CronTrigger(hour=retention_hour, minute=retention_minute),
        id="data_retention_job",
        name="Data Retention",
        replace_existing=True,
    )
    logger.info("Data retention job scheduled for %02d:%02d UTC daily", retention_hour, retention_minute)

    # Account deletion pipeline processing (runs every hour).
    scheduler.add_job(
        run_account_deletion_pipeline,
        trigger="interval",
        hours=1,
        id="account_deletion_pipeline_job",
        name="Account Deletion Pipeline",
        replace_existing=True,
    )
    logger.info("Account deletion pipeline job scheduled every 1 hour")

    # Billing daily counter reset (UTC midnight)
    scheduler.add_job(
        reset_billing_daily_counters,
        trigger=CronTrigger(hour=0, minute=0),
        id="billing_daily_reset_job",
        name="Billing Daily Counter Reset",
        replace_existing=True,
    )
    logger.info("Billing daily counter reset job scheduled daily at 00:00 UTC")

    # Wardrobe rotation reminders (daily)
    rotation_time = settings.WARDROBE_ROTATION_SCHEDULE.split()
    rotation_hour = int(rotation_time[0])
    rotation_minute = int(rotation_time[1]) if len(rotation_time) > 1 else 0
    scheduler.add_job(
        run_wardrobe_rotation,
        trigger=CronTrigger(hour=rotation_hour, minute=rotation_minute),
        id="wardrobe_rotation_job",
        name="Wardrobe Rotation Reminders",
        replace_existing=True,
    )
    logger.info("Wardrobe rotation reminders scheduled for %02d:%02d UTC daily", rotation_hour, rotation_minute)

    # Gift reminders (daily)
    gift_time = settings.GIFT_REMINDER_SCHEDULE.split()
    gift_hour = int(gift_time[0])
    gift_minute = int(gift_time[1]) if len(gift_time) > 1 else 0
    scheduler.add_job(
        run_gift_reminders,
        trigger=CronTrigger(hour=gift_hour, minute=gift_minute),
        id="gift_reminders_job",
        name="Gift Reminders",
        replace_existing=True,
    )
    logger.info("Gift reminders scheduled for %02d:%02d UTC daily", gift_hour, gift_minute)

    # Relationship reminders (daily)
    rel_time = settings.RELATIONSHIP_REMINDER_SCHEDULE.split()
    rel_hour = int(rel_time[0])
    rel_minute = int(rel_time[1]) if len(rel_time) > 1 else 0
    scheduler.add_job(
        run_relationship_reminders,
        trigger=CronTrigger(hour=rel_hour, minute=rel_minute),
        id="relationship_reminders_job",
        name="Relationship Reminders",
        replace_existing=True,
    )
    logger.info("Relationship reminders scheduled for %02d:%02d UTC daily", rel_hour, rel_minute)

    if settings.FEATURE_CONSOLIDATION_ENABLED:
        scheduler.add_job(
            run_knowledge_consolidation,
            trigger=CronTrigger(hour=1, minute=30),
            id="knowledge_consolidation_job",
            name="Knowledge Consolidation",
            replace_existing=True,
        )
        logger.info("Knowledge consolidation job scheduled daily at 01:30 UTC")

    if settings.FEATURE_SELF_REVIEW_ENABLED:
        scheduler.add_job(
            run_self_review,
            trigger=CronTrigger(day_of_week="mon", hour=2, minute=30),
            id="knowledge_self_review_job",
            name="Knowledge Self Review",
            replace_existing=True,
        )
        logger.info("Knowledge self-review job scheduled weekly Monday 02:30 UTC")

    if settings.FEATURE_RESEARCH_ENGINE:
        scheduler.add_job(
            run_due_research,
            trigger="interval",
            minutes=30,
            id="research_scheduler_job",
            name="Research Jobs Scheduler",
            replace_existing=True,
        )
        logger.info("Research scheduler job scheduled every 30 minutes")

    if settings.FEATURE_BEHAVIORAL_INTELLIGENCE:
        scheduler.add_job(
            run_bones_refresh,
            trigger=CronTrigger(hour=3, minute=0),
            id="bones_refresh_job",
            name="Bones Layer Refresh",
            replace_existing=True,
        )
        logger.info("Bones refresh job scheduled daily at 03:00 UTC")

        scheduler.add_job(
            run_muscles_snapshot,
            trigger="interval",
            hours=6,
            id="muscles_snapshot_job",
            name="Muscles Layer Snapshot",
            replace_existing=True,
        )
        logger.info("Muscles snapshot job scheduled every 6 hours")

    scheduler.add_job(
        run_embedding_audit,
        trigger=CronTrigger(day=1, hour=4, minute=20),
        id="embedding_audit_job",
        name="Monthly Embedding Audit",
        replace_existing=True,
    )
    logger.info("Embedding audit job scheduled monthly on day 1 at 04:20 UTC")

    return scheduler


def start_scheduler():
    """Start the scheduler."""
    scheduler = setup_scheduler()
    scheduler.start()
    logger.info("Background scheduler started")
    return scheduler


def stop_scheduler(scheduler: BackgroundScheduler):
    """Stop the scheduler gracefully."""
    if scheduler:
        scheduler.shutdown()
        logger.info("Background scheduler stopped")
