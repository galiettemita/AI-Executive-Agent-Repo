import types

from fastapi.testclient import TestClient

from app.main import app
import app.api.routes.billing_stripe as billing_stripe


def test_stripe_checkout_returns_url(monkeypatch):
    # Env vars required by the handler
    monkeypatch.setenv("STRIPE_SECRET_KEY", "sk_test_dummy")
    monkeypatch.setenv("STRIPE_PRICE_ID_STARTER", "price_dummy")
    monkeypatch.setenv("CHECKOUT_SUCCESS_URL", "https://example.com/success")
    monkeypatch.setenv("CHECKOUT_CANCEL_URL", "https://example.com/cancel")

    # Patch stripe client calls
    def fake_customer_create(metadata=None):
        return {"id": "cus_test"}

    def fake_session_create(**kwargs):
        return types.SimpleNamespace(url="https://stripe.test/checkout")

    monkeypatch.setattr(billing_stripe.stripe, "Customer", types.SimpleNamespace(create=fake_customer_create))
    monkeypatch.setattr(
        billing_stripe.stripe,
        "checkout",
        types.SimpleNamespace(Session=types.SimpleNamespace(create=fake_session_create)),
    )

    client = TestClient(app)
    resp = client.get("/billing/stripe/checkout", params={"user_id": "test_user"})
    assert resp.status_code == 200
    data = resp.json()
    assert data.get("checkout_url") == "https://stripe.test/checkout"
