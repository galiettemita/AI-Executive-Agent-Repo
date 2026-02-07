from __future__ import annotations

import json
import re
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.db.models import Contact


def _to_json(value: Any) -> str:
    return json.dumps(value or {}, ensure_ascii=False)


def _parse_json(text: str | None, default: Any) -> Any:
    if not text:
        return default
    try:
        return json.loads(text)
    except Exception:
        return default


def normalize_email(email: str | None) -> str | None:
    if not email:
        return None
    email = email.strip().lower()
    return email or None


def normalize_phone(phone: str | None) -> str | None:
    if not phone:
        return None
    digits = re.sub(r"\D", "", phone)
    if not digits:
        return None
    # Assume US default if 10 digits
    if len(digits) == 10:
        return f"+1{digits}"
    if phone.strip().startswith("+"):
        return f"+{digits}"
    return f"+{digits}"


def _merge_tags(existing: List[str], incoming: List[str]) -> List[str]:
    merged = list(dict.fromkeys([t for t in existing + incoming if t]))
    return merged


def upsert_contact(
    db: Session,
    user_id: str,
    name: Optional[str] = None,
    phone: Optional[str] = None,
    email: Optional[str] = None,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> Contact:
    normalized_phone = normalize_phone(phone)
    normalized_email = normalize_email(email)

    row: Optional[Contact] = None
    if normalized_phone:
        row = (
            db.query(Contact)
            .filter(Contact.user_id == user_id, Contact.normalized_phone == normalized_phone)
            .one_or_none()
        )
    if row is None and normalized_email:
        row = (
            db.query(Contact)
            .filter(Contact.user_id == user_id, Contact.normalized_email == normalized_email)
            .one_or_none()
        )

    if row is None:
        row = Contact(user_id=user_id)
        db.add(row)

    if name is not None:
        row.name = name
    if phone is not None:
        row.phone = phone
        row.normalized_phone = normalized_phone
    if email is not None:
        row.email = email
        row.normalized_email = normalized_email

    existing_tags = _parse_json(row.tags_json, [])
    if tags:
        row.tags_json = json.dumps(_merge_tags(existing_tags, tags), ensure_ascii=False)

    if metadata:
        existing_meta = _parse_json(row.metadata_json, {})
        existing_meta.update(metadata)
        row.metadata_json = _to_json(existing_meta)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def update_contact(
    db: Session,
    contact: Contact,
    patch: Dict[str, Any],
) -> Contact:
    name = patch.get("name")
    phone = patch.get("phone")
    email = patch.get("email")
    tags = patch.get("tags")
    metadata = patch.get("metadata")

    if name is not None:
        contact.name = name
    if phone is not None:
        contact.phone = phone
        contact.normalized_phone = normalize_phone(phone)
    if email is not None:
        contact.email = email
        contact.normalized_email = normalize_email(email)

    if tags is not None:
        existing_tags = _parse_json(contact.tags_json, [])
        contact.tags_json = json.dumps(_merge_tags(existing_tags, tags), ensure_ascii=False)

    if metadata is not None:
        existing_meta = _parse_json(contact.metadata_json, {})
        existing_meta.update(metadata)
        contact.metadata_json = _to_json(existing_meta)

    contact.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(contact)
    return contact


def get_contact(db: Session, contact_id: int, user_id: Optional[str] = None) -> Optional[Contact]:
    q = db.query(Contact).filter(Contact.id == contact_id)
    if user_id:
        q = q.filter(Contact.user_id == user_id)
    return q.one_or_none()


def list_contacts(db: Session, user_id: str, limit: int = 100) -> List[Contact]:
    return (
        db.query(Contact)
        .filter(Contact.user_id == user_id)
        .order_by(Contact.name.asc().nullslast())
        .limit(limit)
        .all()
    )


def search_contacts(db: Session, user_id: str, query: str, limit: int = 50) -> List[Contact]:
    q = (query or "").strip()
    if not q:
        return []
    like = f"%{q}%"
    return (
        db.query(Contact)
        .filter(Contact.user_id == user_id)
        .filter(
            (Contact.name.ilike(like))
            | (Contact.email.ilike(like))
            | (Contact.phone.ilike(like))
        )
        .order_by(Contact.name.asc().nullslast())
        .limit(limit)
        .all()
    )


def delete_contact(db: Session, contact: Contact) -> None:
    db.delete(contact)
    db.commit()
