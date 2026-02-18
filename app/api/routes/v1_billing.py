from __future__ import annotations

from datetime import datetime
from typing import Any, Dict

import stripe
from fastapi import APIRouter, Depends, HTTPException, Query, Request
from fastapi.responses import RedirectResponse
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.core.config import settings
from app.db.models import Invoice, Subscription
from app.middleware.rate_limiter import rate_limit_payment, rate_limit_webhook
from app.services.subscriptions import get_subscription, upsert_subscription


router = APIRouter(prefix="/api/v1/billing", tags=["billing-v1"])


def _stripe_client() -> None:
    key = (settings.STRIPE_SECRET_KEY or "").strip()
    if not key:
        raise HTTPException(status_code=500, detail="Stripe not configured (STRIPE_SECRET_KEY)")
    stripe.api_key = key


def _plan_to_price_id(plan: str) -> str:
    p = (plan or "").strip().lower()
    if p == "personal":
        return (settings.STRIPE_PRICE_ID_PERSONAL or "").strip()
    if p == "professional":
        return (settings.STRIPE_PRICE_ID_PROFESSIONAL or "").strip()
    raise HTTPException(status_code=400, detail="Invalid plan (expected personal|professional)")


def _checkout_urls() -> tuple[str, str]:
    success = (settings.CHECKOUT_SUCCESS_URL or "").strip()
    cancel = (settings.CHECKOUT_CANCEL_URL or "").strip()
    if not success:
        success = f"{settings.APP_BASE_URL.rstrip('/')}/billing/success"
    if not cancel:
        cancel = f"{settings.APP_BASE_URL.rstrip('/')}/billing/cancel"
    return success, cancel


def _portal_return_url() -> str:
    url = (settings.BILLING_PORTAL_RETURN_URL or "").strip()
    if url:
        return url
    return f"{settings.APP_BASE_URL.rstrip('/')}/billing/success"


def _resolve_user_id_from_obj(db: Session, obj: Dict[str, Any]) -> str | None:
    meta = obj.get("metadata") if isinstance(obj, dict) else None
    if isinstance(meta, dict) and meta.get("user_id"):
        return str(meta.get("user_id"))

    customer_id = obj.get("customer") if isinstance(obj, dict) else None
    if customer_id:
        sub = db.query(Subscription).filter(Subscription.provider_customer_id == str(customer_id)).first()
        if sub:
            return sub.user_id
    return None


def _upsert_invoice_from_stripe(db: Session, user_id: str, invoice_obj: Dict[str, Any]) -> None:
    inv_id = str(invoice_obj.get("id") or "").strip()
    if not inv_id:
        return

    row = db.query(Invoice).filter(Invoice.provider_invoice_id == inv_id).first()
    if row is None:
        row = Invoice(user_id=user_id, provider="stripe", provider_invoice_id=inv_id)
        db.add(row)

    row.provider_customer_id = str(invoice_obj.get("customer") or "") or None
    row.provider_subscription_id = str(invoice_obj.get("subscription") or "") or None
    row.status = str(invoice_obj.get("status") or row.status or "open")
    row.amount_due = int(invoice_obj.get("amount_due")) if invoice_obj.get("amount_due") is not None else None
    row.amount_paid = int(invoice_obj.get("amount_paid")) if invoice_obj.get("amount_paid") is not None else None
    row.currency = str(invoice_obj.get("currency") or "") or None
    row.hosted_invoice_url = str(invoice_obj.get("hosted_invoice_url") or "") or None
    row.invoice_pdf_url = str(invoice_obj.get("invoice_pdf") or "") or None

    paid_at_ts = None
    transitions = invoice_obj.get("status_transitions")
    if isinstance(transitions, dict):
        paid_at_ts = transitions.get("paid_at")
    if paid_at_ts:
        row.paid_at = datetime.utcfromtimestamp(int(paid_at_ts))

    db.commit()


class CheckoutRequest(BaseModel):
    user_id: str
    plan: str = Field(..., description="personal|professional")


@rate_limit_payment()
@router.post("/checkout")
def create_checkout(request: Request, payload: CheckoutRequest, db: Session = Depends(get_db)):  # noqa: ARG001
    _stripe_client()
    get_or_create_user(db, payload.user_id)

    price_id = _plan_to_price_id(payload.plan)
    if not price_id:
        raise HTTPException(status_code=500, detail="Stripe price ID not configured for this plan")

    success_url, cancel_url = _checkout_urls()

    sub = get_subscription(db, payload.user_id)
    customer_id = sub.provider_customer_id if sub else None
    if not customer_id:
        customer = stripe.Customer.create(metadata={"user_id": payload.user_id})
        customer_id = customer["id"]

    session = stripe.checkout.Session.create(
        mode="subscription",
        customer=customer_id,
        line_items=[{"price": price_id, "quantity": 1}],
        success_url=success_url,
        cancel_url=cancel_url,
        metadata={"user_id": payload.user_id, "plan": payload.plan},
        subscription_data={"metadata": {"user_id": payload.user_id, "plan": payload.plan}},
    )

    upsert_subscription(
        db,
        payload.user_id,
        plan=payload.plan,
        status="pending",
        provider="stripe",
        provider_customer_id=customer_id,
    )

    return {"ok": True, "checkout_url": session.url, "session_id": session.id}


@rate_limit_payment()
@router.get("/checkout")
def create_checkout_get(
    request: Request,  # noqa: ARG001
    user_id: str = Query(...),
    plan: str = Query(...),
    db: Session = Depends(get_db),
):
    result = create_checkout(request=request, payload=CheckoutRequest(user_id=user_id, plan=plan), db=db)
    return RedirectResponse(url=str(result["checkout_url"]), status_code=303)


@rate_limit_payment()
@router.post("/portal")
def create_portal(request: Request, user_id: str = Query(...), db: Session = Depends(get_db)):  # noqa: ARG001
    _stripe_client()
    get_or_create_user(db, user_id)

    sub = get_subscription(db, user_id)
    customer_id = (sub.provider_customer_id if sub else None) or ""
    if not customer_id:
        raise HTTPException(status_code=400, detail="No Stripe customer on file for this user")

    session = stripe.billing_portal.Session.create(
        customer=customer_id,
        return_url=_portal_return_url(),
    )
    return {"ok": True, "portal_url": session.url}


@rate_limit_payment()
@router.get("/portal")
def create_portal_get(
    request: Request,  # noqa: ARG001
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    result = create_portal(request=request, user_id=user_id, db=db)
    return RedirectResponse(url=str(result["portal_url"]), status_code=303)


@rate_limit_webhook()
@router.post("/webhooks/stripe")
async def stripe_webhook(request: Request, db: Session = Depends(get_db)):
    _stripe_client()
    payload = await request.body()
    sig = request.headers.get("stripe-signature", "")
    secret = (settings.STRIPE_WEBHOOK_SECRET or "").strip()
    if not secret:
        raise HTTPException(status_code=500, detail="STRIPE_WEBHOOK_SECRET not configured")

    try:
        event = stripe.Webhook.construct_event(payload=payload, sig_header=sig, secret=secret)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Invalid signature: {exc}")

    event_type = str(event.get("type") or "")
    obj = event.get("data", {}).get("object", {}) if isinstance(event, dict) else {}
    if not isinstance(obj, dict):
        obj = {}

    user_id = _resolve_user_id_from_obj(db, obj)

    if event_type in {"customer.subscription.created", "customer.subscription.updated"}:
        if user_id:
            period_end = None
            if obj.get("current_period_end"):
                period_end = datetime.utcfromtimestamp(int(obj["current_period_end"]))
            meta = obj.get("metadata") if isinstance(obj.get("metadata"), dict) else {}
            plan = str((meta or {}).get("plan") or "personal")
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status=str(obj.get("status") or "active"),
                provider="stripe",
                provider_customer_id=str(obj.get("customer") or "") or None,
                provider_subscription_id=str(obj.get("id") or "") or None,
                current_period_end=period_end,
            )

    if event_type == "customer.subscription.deleted":
        if user_id:
            meta = obj.get("metadata") if isinstance(obj.get("metadata"), dict) else {}
            plan = str((meta or {}).get("plan") or "personal")
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status="canceled",
                provider="stripe",
                provider_customer_id=str(obj.get("customer") or "") or None,
                provider_subscription_id=str(obj.get("id") or "") or None,
            )

    if event_type in {"invoice.paid", "invoice.payment_failed"}:
        if user_id:
            _upsert_invoice_from_stripe(db, user_id, obj)

            new_status = "active" if event_type == "invoice.paid" else "past_due"
            # Keep plan as-is if we already have one.
            sub = get_subscription(db, user_id)
            plan = (sub.plan if sub and sub.plan else "personal")
            upsert_subscription(
                db,
                user_id,
                plan=plan,
                status=new_status,
                provider="stripe",
                provider_customer_id=str(obj.get("customer") or "") or None,
                provider_subscription_id=str(obj.get("subscription") or "") or None,
            )

    return {"ok": True, "event_type": event_type}
