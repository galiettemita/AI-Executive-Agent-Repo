from __future__ import annotations

from types import SimpleNamespace

import stripe
from fastapi.testclient import TestClient

from app.core.config import settings
from app.db.database import SessionLocal
from app.db.models import Invoice
from app.main import app
from app.services.subscriptions import get_subscription


def test_v1_billing_checkout_portal_and_webhook(monkeypatch):
    user_id = "v1-billing-user"

    monkeypatch.setattr(settings, "STRIPE_SECRET_KEY", "sk_test_dummy")
    monkeypatch.setattr(settings, "STRIPE_WEBHOOK_SECRET", "whsec_test")
    monkeypatch.setattr(settings, "STRIPE_PRICE_ID_PERSONAL", "price_personal")
    monkeypatch.setattr(settings, "CHECKOUT_SUCCESS_URL", "https://example.com/success")
    monkeypatch.setattr(settings, "CHECKOUT_CANCEL_URL", "https://example.com/cancel")
    monkeypatch.setattr(settings, "BILLING_PORTAL_RETURN_URL", "https://example.com/return")

    monkeypatch.setattr(stripe.Customer, "create", lambda **kwargs: {"id": "cus_123"})
    monkeypatch.setattr(
        stripe.checkout.Session,
        "create",
        lambda **kwargs: SimpleNamespace(url="https://stripe.test/checkout", id="cs_test_123"),
    )
    monkeypatch.setattr(
        stripe.billing_portal.Session,
        "create",
        lambda **kwargs: SimpleNamespace(url="https://stripe.test/portal"),
    )

    client = TestClient(app)
    checkout = client.post(
        "/api/v1/billing/checkout",
        json={"user_id": user_id, "plan": "personal"},
    )
    assert checkout.status_code == 200
    body = checkout.json()
    assert body["ok"] is True
    assert body["checkout_url"].startswith("https://stripe.test/")

    db = SessionLocal()
    try:
        sub = get_subscription(db, user_id)
        assert sub is not None
        assert sub.plan == "personal"
        assert sub.status == "pending"
        assert sub.provider == "stripe"
        assert sub.provider_customer_id == "cus_123"
    finally:
        db.close()

    portal = client.get("/api/v1/billing/portal", params={"user_id": user_id}, follow_redirects=False)
    assert portal.status_code == 303
    assert portal.headers.get("location", "").startswith("https://stripe.test/portal")

    def _construct_event(*args, **kwargs):
        return {
            "type": "invoice.paid",
            "data": {
                "object": {
                    "id": "in_123",
                    "customer": "cus_123",
                    "subscription": "sub_123",
                    "status": "paid",
                    "amount_due": 1999,
                    "amount_paid": 1999,
                    "currency": "usd",
                    "hosted_invoice_url": "https://stripe.test/invoice",
                    "invoice_pdf": "https://stripe.test/invoice.pdf",
                    "status_transitions": {"paid_at": 1700000000},
                    "metadata": {"user_id": user_id},
                }
            },
        }

    monkeypatch.setattr(stripe.Webhook, "construct_event", _construct_event)

    webhook = client.post(
        "/api/v1/billing/webhooks/stripe",
        data=b"{}",
        headers={"stripe-signature": "sig"},
    )
    assert webhook.status_code == 200
    assert webhook.json()["ok"] is True

    db2 = SessionLocal()
    try:
        inv = db2.query(Invoice).filter(Invoice.provider_invoice_id == "in_123").first()
        assert inv is not None
        assert inv.user_id == user_id
        assert inv.status == "paid"

        sub2 = get_subscription(db2, user_id)
        assert sub2 is not None
        assert sub2.status == "active"
    finally:
        db2.close()
