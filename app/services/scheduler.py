# backend/app/services/scheduler.py

from __future__ import annotations

import asyncio
import os
from typing import List

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from sqlalchemy.orm import Session

from app.db.database import SessionLocal
from app.db.models import OAuthToken, User, WatchItem
from app.services.daily_brief import generate_and_store_daily_brief
from app.services.notification_delivery import deliver_pending_notifications


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

        print(f"[Scheduler] Running daily briefs for {len(user_ids)} users with Google OAuth")

        success_count = 0
        error_count = 0

        for user_id in user_ids:
            try:
                result = generate_and_store_daily_brief(db=db, user_id=user_id)

                if result.get("success"):
                    success_count += 1
                    print(f"[Scheduler] ✓ Daily brief generated for user {user_id}")
                else:
                    error_count += 1
                    print(f"[Scheduler] ✗ Failed to generate brief for user {user_id}: {result.get('error')}")

            except Exception as e:
                error_count += 1
                print(f"[Scheduler] ✗ Error generating brief for user {user_id}: {e}")

        print(f"[Scheduler] Daily briefs completed: {success_count} success, {error_count} errors")

    except Exception as e:
        print(f"[Scheduler] Fatal error in daily brief job: {e}")
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
            print("[Scheduler] No watch items to monitor")
            return

        print(f"[Scheduler] Monitoring {len(watch_items)} watch items")

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
                        print(f"[Scheduler] ✓ Price drop detected for item {item.id}")

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
                        print(f"[Scheduler] ✓ Target hit for item {item.id}")

            except Exception as e:
                error_count += 1
                print(f"[Scheduler] ✗ Error monitoring item {item.id}: {e}")

        db.commit()
        print(f"[Scheduler] Price monitoring completed: {updated_count} updated, {error_count} errors")

    except Exception as e:
        print(f"[Scheduler] Fatal error in price monitoring job: {e}")
    finally:
        db.close()


def run_notification_delivery():
    """
    Deliver all pending notifications via WhatsApp.
    """
    db = SessionLocal()

    try:
        result = deliver_pending_notifications(db)
        print(f"[Scheduler] Notification delivery completed: {result}")

    except Exception as e:
        print(f"[Scheduler] Fatal error in notification delivery job: {e}")
    finally:
        db.close()


def setup_scheduler() -> BackgroundScheduler:
    """
    Set up the APScheduler for background jobs.

    Returns:
        Configured BackgroundScheduler instance
    """
    scheduler = BackgroundScheduler()

    # Get schedule from environment or use default (7 AM daily)
    # Format: "hour minute" e.g., "7 0" for 7:00 AM
    schedule_time = os.getenv("DAILY_BRIEF_SCHEDULE", "7 0").split()
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

    print(f"[Scheduler] Daily brief job scheduled for {hour:02d}:{minute:02d} UTC daily")

    # Add price monitoring job (runs every hour)
    # Can be customized with PRICE_MONITORING_INTERVAL_MINUTES env var
    monitoring_interval = int(os.getenv("PRICE_MONITORING_INTERVAL_MINUTES", "60"))
    scheduler.add_job(
        run_price_monitoring,
        trigger="interval",
        minutes=monitoring_interval,
        id="price_monitoring_job",
        name="Price Monitoring for Watch Items",
        replace_existing=True,
    )

    print(f"[Scheduler] Price monitoring job scheduled every {monitoring_interval} minutes")

    # Add notification delivery job (runs every 5 minutes)
    # Can be customized with NOTIFICATION_DELIVERY_INTERVAL_MINUTES env var
    delivery_interval = int(os.getenv("NOTIFICATION_DELIVERY_INTERVAL_MINUTES", "5"))
    scheduler.add_job(
        run_notification_delivery,
        trigger="interval",
        minutes=delivery_interval,
        id="notification_delivery_job",
        name="Notification Delivery",
        replace_existing=True,
    )

    print(f"[Scheduler] Notification delivery job scheduled every {delivery_interval} minutes")

    return scheduler


def start_scheduler():
    """Start the scheduler."""
    scheduler = setup_scheduler()
    scheduler.start()
    print("[Scheduler] Background scheduler started")
    return scheduler


def stop_scheduler(scheduler: BackgroundScheduler):
    """Stop the scheduler gracefully."""
    if scheduler:
        scheduler.shutdown()
        print("[Scheduler] Background scheduler stopped")
