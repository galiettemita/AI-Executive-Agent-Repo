# tests/test_invoice_generation.py

import pytest
import os
import uuid
import json
from datetime import datetime
from unittest.mock import patch

from app.db.database import SessionLocal, Base, engine
from app.db.models import User, Transaction, Proposal, Subscription
from app.services.invoice_service import InvoiceService


# Create tables before running tests
@pytest.fixture(scope="module", autouse=True)
def setup_database():
    """Create all tables before running tests"""
    Base.metadata.create_all(bind=engine)
    yield
    # No cleanup - keep tables for inspection if needed


def test_generate_invoice_pdf():
    """Test generating a PDF invoice for a transaction"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create test user
    user = User(id=user_id)
    db.add(user)

    # Create test subscription
    subscription = Subscription(user_id=user_id, plan="starter", status="active")
    db.add(subscription)

    # Create test proposal
    proposal = Proposal(
        user_id=user_id,
        proposal_type="food_order",
        status="approved",
        payload_json=json.dumps({
            "description": "Dinner order from Italian Restaurant",
            "items": [
                {"name": "Margherita Pizza", "price": 15.99},
                {"name": "Caesar Salad", "price": 8.99},
            ],
            "total_price": 24.98,
        }),
    )
    db.add(proposal)
    db.commit()

    # Create test transaction
    transaction = Transaction(
        user_id=user_id,
        proposal_id=proposal.id,
        amount=24.98,
        currency="USD",
        status="succeeded",
        stripe_payment_intent_id=f"pi_test_{uuid.uuid4().hex[:16]}",
        stripe_charge_id=f"ch_test_{uuid.uuid4().hex[:16]}",
        transaction_type="food_order",
        description="Dinner order from Italian Restaurant",
    )
    db.add(transaction)
    db.commit()

    # Generate invoice
    invoice_path = InvoiceService.generate_invoice_pdf(db, transaction.id)

    # Verify PDF was created
    assert invoice_path is not None
    assert os.path.exists(invoice_path)
    assert invoice_path.endswith(".pdf")
    assert f"invoice_{transaction.id}" in invoice_path

    # Verify file is not empty
    assert os.path.getsize(invoice_path) > 0

    # Clean up
    if os.path.exists(invoice_path):
        os.remove(invoice_path)
    db.close()


def test_generate_invoice_with_refund():
    """Test generating an invoice for a transaction with refund"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create test user
    user = User(id=user_id)
    db.add(user)

    # Create test transaction with refund
    transaction = Transaction(
        user_id=user_id,
        amount=100.0,
        currency="USD",
        status="refunded",
        stripe_payment_intent_id=f"pi_test_refund_{uuid.uuid4().hex[:12]}",
        transaction_type="flight_booking",
        description="Flight booking - NYC to LAX",
        refund_amount=50.0,
        refund_reason="Partial cancellation",
        refunded_at=datetime.utcnow(),
    )
    db.add(transaction)
    db.commit()

    # Generate invoice
    invoice_path = InvoiceService.generate_invoice_pdf(db, transaction.id)

    # Verify PDF was created
    assert invoice_path is not None
    assert os.path.exists(invoice_path)

    # Clean up
    if os.path.exists(invoice_path):
        os.remove(invoice_path)
    db.close()


def test_get_invoice_path():
    """Test retrieving invoice path from transaction"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create test user
    user = User(id=user_id)
    db.add(user)

    # Create test transaction with invoice path
    transaction = Transaction(
        user_id=user_id,
        amount=50.0,
        currency="USD",
        status="succeeded",
        transaction_type="retail_purchase",
        invoice_pdf_path="/path/to/invoice.pdf",
    )
    db.add(transaction)
    db.commit()

    # Get invoice path
    invoice_path = InvoiceService.get_invoice_path(db, transaction.id)

    assert invoice_path == "/path/to/invoice.pdf"

    db.close()


def test_update_invoice_path():
    """Test updating invoice path in transaction"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create test user
    user = User(id=user_id)
    db.add(user)

    # Create test transaction
    transaction = Transaction(
        user_id=user_id,
        amount=75.0,
        currency="USD",
        status="succeeded",
        transaction_type="hotel_booking",
    )
    db.add(transaction)
    db.commit()

    # Update invoice path
    new_path = "/invoices/test_invoice.pdf"
    result = InvoiceService.update_invoice_path(db, transaction.id, new_path)

    assert result is True

    # Verify path was updated
    db.refresh(transaction)
    assert transaction.invoice_pdf_path == new_path

    db.close()


def test_generate_invoice_transaction_not_found():
    """Test generating invoice for non-existent transaction"""
    db = SessionLocal()

    with pytest.raises(ValueError, match="Transaction .* not found"):
        InvoiceService.generate_invoice_pdf(db, 999999)

    db.close()


def test_generate_invoice_no_user():
    """Test generating invoice when user is missing"""
    db = SessionLocal()

    # Create transaction without valid user
    transaction = Transaction(
        user_id="nonexistent_user",
        amount=50.0,
        currency="USD",
        status="succeeded",
        transaction_type="food_order",
    )
    db.add(transaction)
    db.commit()

    with pytest.raises(ValueError, match="User .* not found"):
        InvoiceService.generate_invoice_pdf(db, transaction.id)

    db.close()
