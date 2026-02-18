# backend/app/api/routes/billing_stripe.py

from __future__ import annotations

from datetime import datetime
from typing import Any, Dict

import stripe
from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.db.models import Subscription
from app.services.subscriptions import get_subscription, upsert_subscription
from app.services.oauth_vault import store_stripe_billing_tokens
from app.core.config import settings
from app.middleware.rate_limiter import rate_limit_payment, rate_limit_webhook


router = APIRouter(prefix="/billing/stripe", tags=["billing"])


def _stripe_client() -> None:
    key = settings.STRIPE_SECRET_KEY or ""
    if not key:
        raise HTTPException(status_code=500, detail="Stripe not configured")
    stripe.api_key = key


def _plan_from_metadata(meta: Dict[str, Any] | None) -> str:
    if not meta:
        return "starter"
    return meta.get("plan") or "starter"


@rate_limit_payment()
@router.post("/checkout")
def stripe_checkout(
    request: Request,
    user_id: str,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    _stripe_client()

    price_id = settings.STRIPE_PRICE_ID_STARTER or ""
    if not price_id:
        raise HTTPException(status_code=500, detail="STRIPE_PRICE_ID_STARTER not set")

    success_url = settings.CHECKOUT_SUCCESS_URL.strip()
    cancel_url = settings.CHECKOUT_CANCEL_URL.strip()
    if not success_url or not cancel_url:
        raise HTTPException(status_code=500, detail="Checkout URLs not configured")

    sub = get_subscription(db, user_id)
    customer_id = sub.provider_customer_id if sub else None
    if not customer_id:
        customer = stripe.Customer.create(metadata={"user_id": user_id})
        customer_id = customer["id"]

    session = stripe.checkout.Session.create(
        mode="subscription",
        customer=customer_id,
        line_items=[{"price": price_id, "quantity": 1}],
        success_url=success_url,
        cancel_url=cancel_url,
        metadata={"user_id": user_id, "plan": "starter"},
        subscription_data={"metadata": {"user_id": user_id, "plan": "starter"}},
    )

    # Record as pending until webhook confirms
    upsert_subscription(
        db,
        user_id,
        plan="starter",
        status="pending",
        provider="stripe",
        provider_customer_id=customer_id,
    )
    store_stripe_billing_tokens(
        db,
        user_id=user_id,
        customer_id=customer_id,
        subscription_id=None,
        plan="starter",
        status="pending",
    )

    return {"checkout_url": session.url}


@rate_limit_payment()
@router.get("/checkout")
def stripe_checkout_get(
    request: Request,
    user_id: str,
    db: Session = Depends(get_db),
):
    return stripe_checkout(request=request, user_id=user_id, db=db)


@rate_limit_webhook()
@router.post("/webhook")
async def stripe_webhook(request: Request, db: Session = Depends(get_db)):
    _stripe_client()
    payload = await request.body()
    sig = request.headers.get("stripe-signature", "")
    secret = settings.STRIPE_WEBHOOK_SECRET or ""
    if not secret:
        raise HTTPException(status_code=500, detail="STRIPE_WEBHOOK_SECRET not set")

    try:
        event = stripe.Webhook.construct_event(payload=payload, sig_header=sig, secret=secret)
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid signature: {e}")

    event_type = event["type"]
    data = event["data"]["object"]

    # Helpers to resolve user_id
    def _resolve_user_id(obj: Dict[str, Any]) -> str | None:
        meta = obj.get("metadata") if isinstance(obj, dict) else None
        if meta and meta.get("user_id"):
            return meta.get("user_id")
        customer_id = obj.get("customer")
        if customer_id:
            sub = (
                db.query(Subscription)
                .filter(Subscription.provider_customer_id == customer_id)
                .first()
            )
            return sub.user_id if sub else None
        return None

    # checkout.session.completed
    if event_type == "checkout.session.completed":
        user_id = _resolve_user_id(data)
        if user_id:
            plan = _plan_from_metadata(data.get("metadata"))
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status="active",
                provider="stripe",
                provider_customer_id=data.get("customer"),
                provider_subscription_id=data.get("subscription"),
            )
            store_stripe_billing_tokens(
                db,
                user_id=user_id,
                customer_id=str(data.get("customer") or ""),
                subscription_id=str(data.get("subscription") or "") or None,
                plan=plan,
                status="active",
            )

    # customer.subscription.updated or created
    if event_type in ("customer.subscription.updated", "customer.subscription.created"):
        user_id = _resolve_user_id(data)
        if user_id:
            period_end = None
            if data.get("current_period_end"):
                period_end = datetime.utcfromtimestamp(data["current_period_end"])
            plan = _plan_from_metadata(data.get("metadata"))
            status = data.get("status") or "active"
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status=status,
                provider="stripe",
                provider_customer_id=data.get("customer"),
                provider_subscription_id=data.get("id"),
                current_period_end=period_end,
            )
            store_stripe_billing_tokens(
                db,
                user_id=user_id,
                customer_id=str(data.get("customer") or ""),
                subscription_id=str(data.get("id") or "") or None,
                plan=plan,
                status=str(status),
            )

    if event_type == "customer.subscription.deleted":
        user_id = _resolve_user_id(data)
        if user_id:
            plan = _plan_from_metadata(data.get("metadata"))
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status="canceled",
                provider="stripe",
                provider_customer_id=data.get("customer"),
                provider_subscription_id=data.get("id"),
            )
            store_stripe_billing_tokens(
                db,
                user_id=user_id,
                customer_id=str(data.get("customer") or ""),
                subscription_id=str(data.get("id") or "") or None,
                plan=plan,
                status="canceled",
            )

    if event_type == "invoice.paid":
        user_id = _resolve_user_id(data)
        if user_id:
            upsert_subscription(
                db,
                user_id,
                plan="starter",
                status="active",
                provider="stripe",
                provider_customer_id=data.get("customer"),
                provider_subscription_id=data.get("subscription"),
            )
            store_stripe_billing_tokens(
                db,
                user_id=user_id,
                customer_id=str(data.get("customer") or ""),
                subscription_id=str(data.get("subscription") or "") or None,
                plan="starter",
                status="active",
            )

    if event_type == "invoice.payment_failed":
        user_id = _resolve_user_id(data)
        if user_id:
            upsert_subscription(
                db,
                user_id,
                plan="starter",
                status="past_due",
                provider="stripe",
                provider_customer_id=data.get("customer"),
                provider_subscription_id=data.get("subscription"),
            )
            store_stripe_billing_tokens(
                db,
                user_id=user_id,
                customer_id=str(data.get("customer") or ""),
                subscription_id=str(data.get("subscription") or "") or None,
                plan="starter",
                status="past_due",
            )

    return {"ok": True}
