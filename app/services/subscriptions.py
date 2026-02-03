# backend/app/services/subscriptions.py

from __future__ import annotations

from datetime import datetime
from typing import Dict

from sqlalchemy.orm import Session

from app.db.models import Subscription


PREMIUM_PLANS = {"starter", "plus", "pro"}


def is_premium_user(entitlements: Dict[str, str]) -> bool:
    plan = (entitlements.get("plan") or "free").lower()
    status = (entitlements.get("status") or "active").lower()
    return plan in PREMIUM_PLANS and status == "active"


def upgrade_prompt(user_id: str) -> str:
    return (
        "This feature is part of the Starter plan. "
        "Upgrade here: "
        f"https://ai-shopping-assistant-backend-6bgf.onrender.com/billing/stripe/checkout?user_id={user_id}"
    )


def get_subscription(db: Session, user_id: str) -> Subscription | None:
    return db.query(Subscription).filter(Subscription.user_id == user_id).first()


def get_entitlements(db: Session, user_id: str) -> Dict[str, str]:
    """
    Minimal entitlements stub. Defaults to free if no record exists.
    """
    sub = get_subscription(db, user_id)
    if not sub:
        return {"plan": "free", "status": "active"}
    return {
        "plan": sub.plan or "free",
        "status": sub.status or "active",
    }


def upsert_subscription(
    db: Session,
    user_id: str,
    *,
    plan: str,
    status: str,
    provider: str | None = None,
    provider_customer_id: str | None = None,
    provider_subscription_id: str | None = None,
    current_period_end: datetime | None = None,
) -> None:
    sub = get_subscription(db, user_id)
    if not sub:
        sub = Subscription(
            user_id=user_id,
            plan=plan,
            status=status,
            provider=provider,
            provider_customer_id=provider_customer_id,
            provider_subscription_id=provider_subscription_id,
            current_period_end=current_period_end,
        )
        db.add(sub)
    else:
        sub.plan = plan
        sub.status = status
        sub.provider = provider
        sub.provider_customer_id = provider_customer_id
        sub.provider_subscription_id = provider_subscription_id
        sub.current_period_end = current_period_end
    db.commit()
