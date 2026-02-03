# backend/app/services/scheduler.py

from __future__ import annotations

import os
from typing import List

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from sqlalchemy.orm import Session

from app.db.database import SessionLocal
from app.db.models import OAuthToken, User
from app.services.daily_brief import generate_and_store_daily_brief


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
