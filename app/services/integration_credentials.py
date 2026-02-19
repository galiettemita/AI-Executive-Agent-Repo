# backend/app/services/integration_credentials.py

from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session

from app.db.models import IntegrationCredential
from app.db.user_compat import ensure_user_row
from app.services.encryption_service import get_encryption_service


def _ensure_user(db: Session, user_id: str) -> None:
    ensure_user_row(db, user_id)


def upsert_integration_credential(
    db: Session,
    user_id: str,
    provider: str,
    username: Optional[str],
    secret: Optional[str],
    server_url: Optional[str],
    metadata: Optional[Dict[str, Any]] = None,
) -> IntegrationCredential:
    _ensure_user(db, user_id)

    row = (
        db.query(IntegrationCredential)
        .filter(IntegrationCredential.user_id == user_id, IntegrationCredential.provider == provider)
        .one_or_none()
    )
    if row is None:
        row = IntegrationCredential(user_id=user_id, provider=provider)
        db.add(row)

    crypto = get_encryption_service()
    row.username = username
    row.secret_enc = crypto.encrypt(secret) if secret else None
    row.server_url = server_url
    row.metadata_json = json.dumps(metadata or {}, ensure_ascii=False)
    row.updated_at = datetime.utcnow()

    db.commit()
    db.refresh(row)
    return row


def get_integration_credential(
    db: Session,
    user_id: str,
    provider: str,
) -> Optional[IntegrationCredential]:
    return (
        db.query(IntegrationCredential)
        .filter(IntegrationCredential.user_id == user_id, IntegrationCredential.provider == provider)
        .one_or_none()
    )


def delete_integration_credential(db: Session, user_id: str, provider: str) -> None:
    row = get_integration_credential(db, user_id, provider)
    if not row:
        return
    db.delete(row)
    db.commit()


def get_decrypted_secret(row: IntegrationCredential) -> Optional[str]:
    if not row or not row.secret_enc:
        return None
    crypto = get_encryption_service()
    return crypto.decrypt(row.secret_enc)


def get_connection_status(db: Session, user_id: str, provider: str) -> Dict[str, Any]:
    row = get_integration_credential(db, user_id, provider)
    if not row:
        return {"connected": False}
    return {
        "connected": True,
        "provider": provider,
        "username": row.username,
        "server_url": row.server_url,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }
