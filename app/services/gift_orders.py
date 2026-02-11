from __future__ import annotations

import json
from datetime import datetime
from typing import Any, Dict, List, Optional
from urllib.parse import urlparse

from sqlalchemy.orm import Session

from app.db.models import (
    GiftOrder,
    GiftOrderEvent,
    GiftRetailerAllowlist,
    GiftIdea,
    PaymentMethod,
)


def _dump_json(value: Optional[Dict[str, Any]]) -> str:
    if not value:
        return "{}"
    return json.dumps(value, ensure_ascii=False)


def _load_json(value: Optional[str]) -> Dict[str, Any]:
    if not value:
        return {}
    try:
        data = json.loads(value)
        return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def _normalize_domain(value: Optional[str]) -> str:
    if not value:
        return ""
    raw = value.strip().lower()
    if "://" not in raw:
        return raw
    try:
        return (urlparse(raw).netloc or "").lower()
    except Exception:
        return raw


def serialize_retailer(row: GiftRetailerAllowlist) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "domain": row.domain,
        "status": row.status,
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def list_retailers(db: Session, user_id: str, status: Optional[str] = None) -> List[GiftRetailerAllowlist]:
    q = db.query(GiftRetailerAllowlist).filter(
        (GiftRetailerAllowlist.user_id == user_id) | (GiftRetailerAllowlist.user_id.is_(None))
    )
    if status:
        q = q.filter(GiftRetailerAllowlist.status == status)
    return q.order_by(GiftRetailerAllowlist.domain.asc()).all()


def create_retailer(
    db: Session,
    user_id: str,
    domain: str,
    status: str = "allowed",
    notes: Optional[str] = None,
) -> GiftRetailerAllowlist:
    normalized = _normalize_domain(domain)
    row = (
        db.query(GiftRetailerAllowlist)
        .filter(
            GiftRetailerAllowlist.user_id == user_id,
            GiftRetailerAllowlist.domain == normalized,
        )
        .one_or_none()
    )
    if row:
        row.status = status or row.status
        row.notes = notes or row.notes
        row.updated_at = datetime.utcnow()
        db.commit()
        db.refresh(row)
        return row

    row = GiftRetailerAllowlist(
        user_id=user_id,
        domain=normalized,
        status=status or "allowed",
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_retailer(
    db: Session,
    user_id: str,
    retailer_id: int,
    status: Optional[str] = None,
    notes: Optional[str] = None,
) -> Optional[GiftRetailerAllowlist]:
    row = (
        db.query(GiftRetailerAllowlist)
        .filter(GiftRetailerAllowlist.user_id == user_id, GiftRetailerAllowlist.id == retailer_id)
        .one_or_none()
    )
    if not row:
        return None

    if status is not None:
        row.status = status
    if notes is not None:
        row.notes = notes
    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_retailer(db: Session, user_id: str, retailer_id: int) -> bool:
    row = (
        db.query(GiftRetailerAllowlist)
        .filter(GiftRetailerAllowlist.user_id == user_id, GiftRetailerAllowlist.id == retailer_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def is_retailer_allowed(db: Session, user_id: str, domain: str) -> bool:
    normalized = _normalize_domain(domain)
    if not normalized:
        return False

    blocked = (
        db.query(GiftRetailerAllowlist)
        .filter(
            GiftRetailerAllowlist.domain == normalized,
            GiftRetailerAllowlist.status == "blocked",
            (GiftRetailerAllowlist.user_id == user_id) | (GiftRetailerAllowlist.user_id.is_(None)),
        )
        .first()
    )
    if blocked:
        return False

    allowed = (
        db.query(GiftRetailerAllowlist)
        .filter(
            GiftRetailerAllowlist.domain == normalized,
            GiftRetailerAllowlist.status == "allowed",
            (GiftRetailerAllowlist.user_id == user_id) | (GiftRetailerAllowlist.user_id.is_(None)),
        )
        .first()
    )
    return bool(allowed)


def _resolve_idea(db: Session, user_id: str, gift_idea_id: Optional[int]) -> Optional[GiftIdea]:
    if not gift_idea_id:
        return None
    return (
        db.query(GiftIdea)
        .filter(GiftIdea.user_id == user_id, GiftIdea.id == gift_idea_id)
        .one_or_none()
    )


def serialize_order(row: GiftOrder) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "gift_idea_id": row.gift_idea_id,
        "occasion_id": row.occasion_id,
        "proposal_id": row.proposal_id,
        "transaction_id": row.transaction_id,
        "payment_method_id": row.payment_method_id,
        "retailer_domain": row.retailer_domain,
        "product_url": row.product_url,
        "product_title": row.product_title,
        "quantity": row.quantity,
        "unit_price": row.unit_price,
        "total_price": row.total_price,
        "currency": row.currency,
        "status": row.status,
        "shipping_address": _load_json(row.shipping_address_json),
        "tracking_number": row.tracking_number,
        "tracking_url": row.tracking_url,
        "return_window_end": row.return_window_end.isoformat() if row.return_window_end else None,
        "notes": row.notes,
        "metadata": _load_json(row.metadata_json),
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_order_event(row: GiftOrderEvent) -> Dict[str, Any]:
    return {
        "id": row.id,
        "gift_order_id": row.gift_order_id,
        "status": row.status,
        "message": row.message,
        "metadata": _load_json(row.metadata_json),
        "occurred_at": row.occurred_at.isoformat() if row.occurred_at else None,
    }


def create_order(
    db: Session,
    *,
    user_id: str,
    gift_idea_id: Optional[int] = None,
    occasion_id: Optional[int] = None,
    title: Optional[str] = None,
    product_url: Optional[str] = None,
    retailer_domain: Optional[str] = None,
    quantity: int = 1,
    unit_price: Optional[float] = None,
    total_price: Optional[float] = None,
    currency: Optional[str] = None,
    shipping_address: Optional[Dict[str, Any]] = None,
    notes: Optional[str] = None,
    status: Optional[str] = None,
    payment_method_id: Optional[int] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> GiftOrder:
    idea = _resolve_idea(db, user_id, gift_idea_id)
    if idea:
        title = title or idea.title
        product_url = product_url or idea.link_url
        unit_price = unit_price if unit_price is not None else idea.price
        currency = currency or idea.currency
        if occasion_id is None:
            occasion_id = idea.occasion_id

    domain = retailer_domain or _normalize_domain(product_url)
    if domain:
        retailer_domain = domain

    qty = quantity or 1
    computed_total = None
    if unit_price is not None:
        computed_total = unit_price * qty
    total = total_price if total_price is not None else computed_total

    row = GiftOrder(
        user_id=user_id,
        gift_idea_id=gift_idea_id,
        occasion_id=occasion_id,
        retailer_domain=retailer_domain,
        product_url=product_url,
        product_title=title,
        quantity=qty,
        unit_price=unit_price,
        total_price=total,
        currency=currency,
        status=status or "pending_approval",
        shipping_address_json=_dump_json(shipping_address),
        notes=notes,
        metadata_json=_dump_json(metadata),
        payment_method_id=payment_method_id,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)

    record_order_event(db, row.id, row.status, message="order_created")
    return row


def list_orders(db: Session, user_id: str, status: Optional[str] = None, limit: int = 50) -> List[GiftOrder]:
    q = db.query(GiftOrder).filter(GiftOrder.user_id == user_id)
    if status:
        q = q.filter(GiftOrder.status == status)
    return q.order_by(GiftOrder.updated_at.desc()).limit(limit).all()


def get_order(db: Session, user_id: str, order_id: int) -> Optional[GiftOrder]:
    return (
        db.query(GiftOrder)
        .filter(GiftOrder.user_id == user_id, GiftOrder.id == order_id)
        .one_or_none()
    )


def update_order(
    db: Session,
    user_id: str,
    order_id: int,
    *,
    status: Optional[str] = None,
    tracking_number: Optional[str] = None,
    tracking_url: Optional[str] = None,
    notes: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> Optional[GiftOrder]:
    row = get_order(db, user_id, order_id)
    if not row:
        return None

    if status is not None:
        row.status = status
    if tracking_number is not None:
        row.tracking_number = tracking_number
    if tracking_url is not None:
        row.tracking_url = tracking_url
    if notes is not None:
        row.notes = notes
    if metadata is not None:
        existing = _load_json(row.metadata_json)
        existing.update(metadata)
        row.metadata_json = _dump_json(existing)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)

    if status is not None:
        record_order_event(db, row.id, status, message="status_update")
    return row


def authorize_order(
    db: Session,
    user_id: str,
    order_id: int,
    payment_method_id: Optional[int] = None,
) -> Optional[GiftOrder]:
    row = get_order(db, user_id, order_id)
    if not row:
        return None

    if payment_method_id is not None:
        pm = (
            db.query(PaymentMethod)
            .filter(PaymentMethod.id == payment_method_id, PaymentMethod.user_id == user_id)
            .one_or_none()
        )
        if not pm:
            return None
        row.payment_method_id = pm.id

    row.status = "authorized"
    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    record_order_event(db, row.id, "authorized", message="payment_authorized")
    return row


def record_order_event(
    db: Session,
    order_id: int,
    status: str,
    message: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
    occurred_at: Optional[datetime] = None,
) -> GiftOrderEvent:
    row = GiftOrderEvent(
        gift_order_id=order_id,
        status=status,
        message=message,
        metadata_json=_dump_json(metadata),
        occurred_at=occurred_at or datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def list_order_events(db: Session, order_id: int, limit: int = 50) -> List[GiftOrderEvent]:
    return (
        db.query(GiftOrderEvent)
        .filter(GiftOrderEvent.gift_order_id == order_id)
        .order_by(GiftOrderEvent.occurred_at.desc())
        .limit(limit)
        .all()
    )


def refund_order(
    db: Session,
    user_id: str,
    order_id: int,
    reason: Optional[str] = None,
    amount: Optional[float] = None,
) -> Optional[GiftOrder]:
    row = get_order(db, user_id, order_id)
    if not row:
        return None

    metadata = _load_json(row.metadata_json)
    if reason:
        metadata["refund_reason"] = reason
    if amount is not None:
        metadata["refund_amount"] = amount

    row.metadata_json = _dump_json(metadata)
    row.status = "refunded"
    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)

    record_order_event(db, row.id, "refunded", message=reason)
    return row
