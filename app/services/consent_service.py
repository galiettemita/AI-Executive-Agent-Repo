# backend/app/services/consent_service.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import UserConsent


def grant_consent(
    db: Session,
    user_id: str,
    integration: str,
    metadata: Optional[Dict[str, Any]] = None,
) -> UserConsent:
    row = (
        db.query(UserConsent)
        .filter(UserConsent.user_id == user_id, UserConsent.integration == integration)
        .one_or_none()
    )
    if row is None:
        row = UserConsent(user_id=user_id, integration=integration)
        db.add(row)
    row.granted_at = datetime.utcnow()
    row.revoked_at = None
    row.metadata_json = json.dumps(metadata or {}, ensure_ascii=False)
    db.commit()
    db.refresh(row)
    return row


def revoke_consent(db: Session, user_id: str, integration: str) -> UserConsent:
    row = (
        db.query(UserConsent)
        .filter(UserConsent.user_id == user_id, UserConsent.integration == integration)
        .one_or_none()
    )
    if row is None:
        row = UserConsent(user_id=user_id, integration=integration)
        db.add(row)
    row.revoked_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def has_consent(db: Session, user_id: str, integration: str) -> bool:
    row = (
        db.query(UserConsent)
        .filter(UserConsent.user_id == user_id, UserConsent.integration == integration)
        .one_or_none()
    )
    if not row or not row.granted_at:
        return False
    if row.revoked_at and row.revoked_at > row.granted_at:
        return False
    return True


def require_consent(db: Session, user_id: str, integration: str) -> None:
    if settings.ENV == "dev":
        return
    if not has_consent(db, user_id, integration):
        raise RuntimeError(f"Consent required for {integration}. Ask the user to grant consent.")
