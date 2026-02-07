# tests/test_phase1_execution.py

import pytest
import jwt
import json
import uuid
from datetime import datetime, timedelta
from unittest.mock import patch, MagicMock

from app.db.database import SessionLocal
from app.db.models import (
    User,
    PaymentMethod,
    Transaction,
    Proposal,
    ExecutionLog,
    Booking,
    Subscription,
)
from app.services.stripe_service import StripeService
from app.services.execution_engine import ExecutionEngine
from app.services.intervention_service import InterventionService
from app.core.config import settings


def test_verify_approval_token_valid():
    """Test verifying a valid approval token"""
    import os
    os.environ["JWT_SECRET"] = "test_secret_key"
    settings.JWT_SECRET = "test_secret_key"

    # Create valid token
    payload = {
        "proposal_id": 123,
        "user_id": "test_user",
        "action": "approve",
        "exp": (datetime.utcnow() + timedelta(hours=24)).timestamp(),
    }
    token = jwt.encode(payload, "test_secret_key", algorithm="HS256")

    # Verify
    result = ExecutionEngine.verify_approval_token(token)

    assert result["proposal_id"] == 123
    assert result["user_id"] == "test_user"
    assert result["action"] == "approve"


def test_verify_approval_token_expired():
    """Test verifying an expired approval token"""
    import os
    os.environ["JWT_SECRET"] = "test_secret_key"
    settings.JWT_SECRET = "test_secret_key"

    # Create expired token
    payload = {
        "proposal_id": 123,
        "user_id": "test_user",
        "exp": (datetime.utcnow() - timedelta(hours=1)).timestamp(),
    }
    token = jwt.encode(payload, "test_secret_key", algorithm="HS256")

    # Verify should raise error
    with pytest.raises(ValueError, match="expired"):
        ExecutionEngine.verify_approval_token(token)


def test_check_spending_limit_allowed():
    """Test spending limit check when allowed"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    user = User(id=user_id)
    db.add(user)
    subscription = Subscription(user_id=user_id, plan="starter", status="active")
    db.add(subscription)
    db.commit()

    result = StripeService.check_spending_limit(
        db=db,
        user_id=user_id,
        amount=50.0,
    )

    assert result["allowed"] is True
    db.close()


def test_check_spending_limit_exceeds_max_transaction():
    """Test spending limit check when exceeding max transaction"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    user = User(id=user_id)
    db.add(user)
    subscription = Subscription(user_id=user_id, plan="starter", status="active")
    db.add(subscription)
    db.commit()

    result = StripeService.check_spending_limit(
        db=db,
        user_id=user_id,
        amount=600.0,  # Over $500 limit
    )

    assert result["allowed"] is False
    assert result["limit_type"] == "max_transaction"
    db.close()


def test_execute_proposal_dry_run():
    """Test executing a proposal in dry-run mode"""
    import os
    os.environ["JWT_SECRET"] = "test_secret_key"
    settings.JWT_SECRET = "test_secret_key"

    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    user = User(id=user_id)
    db.add(user)
    subscription = Subscription(user_id=user_id, plan="starter", status="active")
    db.add(subscription)

    pm = PaymentMethod(
        user_id=user_id,
        stripe_payment_method_id="pm_test_123",
        type="card",
        is_default=True,
    )
    db.add(pm)

    proposal = Proposal(
        user_id=user_id,
        proposal_type="food_order",
        status="pending",
        payload_json=json.dumps({"total_price": 25.0}),
    )
    db.add(proposal)
    db.commit()

    payload = {
        "proposal_id": proposal.id,
        "user_id": user_id,
        "action": "approve",
        "exp": (datetime.utcnow() + timedelta(hours=24)).timestamp(),
    }
    token = jwt.encode(payload, "test_secret_key", algorithm="HS256")

    # Avoid intervention flagging in dry-run test
    original_flag = InterventionService.should_flag_for_review
    InterventionService.should_flag_for_review = staticmethod(lambda **kwargs: (False, None))
    try:
        result = ExecutionEngine.execute_proposal(
            db=db,
            proposal_id=proposal.id,
            approval_token=token,
            dry_run=True,
        )
    finally:
        InterventionService.should_flag_for_review = original_flag

    assert result["success"] is True
    assert result["dry_run"] is True
    assert result["amount"] == 25.0

    db.close()
