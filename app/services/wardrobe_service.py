# app/services/wardrobe_service.py

from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy import or_
from sqlalchemy.orm import Session

from app.db.models import WardrobeItem, WardrobeItemPhoto, WardrobeWearEvent, PhotoAsset
from app.services.analytics_service import record_usage_event

logger = logging.getLogger(__name__)


def _serialize_tags(tags: Optional[List[str]]) -> str:
    if tags is None:
        return "[]"
    cleaned = [t.strip() for t in tags if t and t.strip()]
    return json.dumps(cleaned, ensure_ascii=False)


def _serialize_metadata(metadata: Optional[Dict[str, Any]]) -> str:
    if metadata is None:
        return "{}"
    return json.dumps(metadata, ensure_ascii=False)


def _parse_tags(raw: Optional[str]) -> List[str]:
    if not raw:
        return []
    try:
        data = json.loads(raw)
        if isinstance(data, list):
            return [str(item) for item in data]
    except Exception:
        return []
    return []


def _parse_metadata(raw: Optional[str]) -> Dict[str, Any]:
    if not raw:
        return {}
    try:
        data = json.loads(raw)
        if isinstance(data, dict):
            return data
    except Exception:
        return {}
    return {}


def serialize_item(item: WardrobeItem) -> Dict[str, Any]:
    return {
        "id": item.id,
        "user_id": item.user_id,
        "name": item.name,
        "category": item.category,
        "subcategory": item.subcategory,
        "brand": item.brand,
        "color": item.color,
        "size": item.size,
        "material": item.material,
        "season": item.season,
        "condition": item.condition,
        "purchase_date": item.purchase_date.isoformat() if item.purchase_date else None,
        "price": item.price,
        "currency": item.currency,
        "notes": item.notes,
        "tags": _parse_tags(item.tags_json),
        "metadata": _parse_metadata(item.metadata_json),
        "wear_count": item.wear_count,
        "last_worn_at": item.last_worn_at.isoformat() if item.last_worn_at else None,
        "last_rotation_notified_at": item.last_rotation_notified_at.isoformat() if item.last_rotation_notified_at else None,
        "created_at": item.created_at.isoformat() if item.created_at else None,
        "updated_at": item.updated_at.isoformat() if item.updated_at else None,
    }


def create_wardrobe_item(
    db: Session,
    user_id: str,
    name: str,
    category: Optional[str] = None,
    subcategory: Optional[str] = None,
    brand: Optional[str] = None,
    color: Optional[str] = None,
    size: Optional[str] = None,
    material: Optional[str] = None,
    season: Optional[str] = None,
    condition: Optional[str] = None,
    purchase_date: Optional[datetime] = None,
    price: Optional[float] = None,
    currency: Optional[str] = None,
    notes: Optional[str] = None,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> WardrobeItem:
    item = WardrobeItem(
        user_id=user_id,
        name=name,
        category=category,
        subcategory=subcategory,
        brand=brand,
        color=color,
        size=size,
        material=material,
        season=season,
        condition=condition,
        purchase_date=purchase_date,
        price=price,
        currency=currency,
        notes=notes,
        tags_json=_serialize_tags(tags),
        metadata_json=_serialize_metadata(metadata),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(item)
    db.commit()
    db.refresh(item)
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="wardrobe_item_create",
            source="wardrobe",
            metadata={"item_id": item.id, "name": name},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
    return item


def list_wardrobe_items(
    db: Session,
    user_id: str,
    limit: int = 50,
    category: Optional[str] = None,
    season: Optional[str] = None,
    brand: Optional[str] = None,
    query: Optional[str] = None,
) -> List[WardrobeItem]:
    q = db.query(WardrobeItem).filter(WardrobeItem.user_id == user_id)
    if category:
        q = q.filter(WardrobeItem.category == category)
    if season:
        q = q.filter(WardrobeItem.season == season)
    if brand:
        q = q.filter(WardrobeItem.brand == brand)
    if query:
        like = f"%{query}%"
        q = q.filter(
            or_(
                WardrobeItem.name.ilike(like),  # type: ignore
                WardrobeItem.category.ilike(like),  # type: ignore
                WardrobeItem.subcategory.ilike(like),  # type: ignore
                WardrobeItem.brand.ilike(like),  # type: ignore
                WardrobeItem.color.ilike(like),  # type: ignore
                WardrobeItem.tags_json.ilike(like),  # type: ignore
            )
        )
    return q.order_by(WardrobeItem.created_at.desc()).limit(limit).all()


def get_wardrobe_item(db: Session, user_id: str, item_id: int) -> Optional[WardrobeItem]:
    return (
        db.query(WardrobeItem)
        .filter(WardrobeItem.user_id == user_id, WardrobeItem.id == item_id)
        .one_or_none()
    )


def update_wardrobe_item(
    db: Session,
    user_id: str,
    item_id: int,
    **fields: Any,
) -> Optional[WardrobeItem]:
    item = get_wardrobe_item(db, user_id, item_id)
    if not item:
        return None

    for key, value in fields.items():
        if key == "tags":
            if value is not None:
                item.tags_json = _serialize_tags(value)
            continue
        if key == "metadata":
            if value is not None:
                item.metadata_json = _serialize_metadata(value)
            continue
        if hasattr(item, key) and value is not None:
            setattr(item, key, value)

    item.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(item)
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="wardrobe_item_update",
            source="wardrobe",
            metadata={"item_id": item.id},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
    return item


def delete_wardrobe_item(db: Session, user_id: str, item_id: int) -> bool:
    item = get_wardrobe_item(db, user_id, item_id)
    if not item:
        return False
    db.query(WardrobeWearEvent).filter(WardrobeWearEvent.wardrobe_item_id == item_id).delete()
    db.query(WardrobeItemPhoto).filter(WardrobeItemPhoto.wardrobe_item_id == item_id).delete()
    db.delete(item)
    db.commit()
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="wardrobe_item_delete",
            source="wardrobe",
            metadata={"item_id": item_id},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
    return True


def attach_photo_to_item(
    db: Session,
    user_id: str,
    item_id: int,
    photo_asset_id: int,
    is_primary: bool = False,
) -> bool:
    item = get_wardrobe_item(db, user_id, item_id)
    if not item:
        return False
    photo = (
        db.query(PhotoAsset)
        .filter(PhotoAsset.user_id == user_id, PhotoAsset.id == photo_asset_id)
        .one_or_none()
    )
    if not photo:
        return False
    existing = (
        db.query(WardrobeItemPhoto)
        .filter(
            WardrobeItemPhoto.wardrobe_item_id == item_id,
            WardrobeItemPhoto.photo_asset_id == photo_asset_id,
        )
        .one_or_none()
    )
    if existing:
        if is_primary and not existing.is_primary:
            _set_primary_photo(db, item_id, photo_asset_id)
        return True

    if is_primary:
        _clear_primary_photo(db, item_id)

    link = WardrobeItemPhoto(
        wardrobe_item_id=item_id,
        photo_asset_id=photo_asset_id,
        is_primary=is_primary,
        created_at=datetime.utcnow(),
    )
    db.add(link)
    db.commit()
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="wardrobe_photo_attach",
            source="wardrobe",
            metadata={"item_id": item_id, "photo_asset_id": photo_asset_id},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
    return True


def detach_photo_from_item(db: Session, user_id: str, item_id: int, photo_asset_id: int) -> bool:
    item = get_wardrobe_item(db, user_id, item_id)
    if not item:
        return False
    link = (
        db.query(WardrobeItemPhoto)
        .filter(
            WardrobeItemPhoto.wardrobe_item_id == item_id,
            WardrobeItemPhoto.photo_asset_id == photo_asset_id,
        )
        .one_or_none()
    )
    if not link:
        return False
    db.delete(link)
    db.commit()
    return True


def list_item_photos(db: Session, user_id: str, item_id: int) -> List[Dict[str, Any]]:
    rows = (
        db.query(PhotoAsset, WardrobeItemPhoto)
        .join(WardrobeItemPhoto, WardrobeItemPhoto.photo_asset_id == PhotoAsset.id)
        .filter(
            WardrobeItemPhoto.wardrobe_item_id == item_id,
            PhotoAsset.user_id == user_id,
        )
        .order_by(WardrobeItemPhoto.is_primary.desc(), WardrobeItemPhoto.created_at.desc())
        .all()
    )
    return [
        {
            "id": asset.id,
            "filename": asset.filename,
            "content_type": asset.content_type,
            "size_bytes": asset.size_bytes,
            "tags": asset.tags_json,
            "is_primary": link.is_primary,
            "created_at": asset.created_at.isoformat() if asset.created_at else None,
        }
        for asset, link in rows
    ]


def _clear_primary_photo(db: Session, item_id: int) -> None:
    db.query(WardrobeItemPhoto).filter(WardrobeItemPhoto.wardrobe_item_id == item_id).update(
        {WardrobeItemPhoto.is_primary: False}
    )
    db.commit()


def _set_primary_photo(db: Session, item_id: int, photo_asset_id: int) -> None:
    _clear_primary_photo(db, item_id)
    db.query(WardrobeItemPhoto).filter(
        WardrobeItemPhoto.wardrobe_item_id == item_id,
        WardrobeItemPhoto.photo_asset_id == photo_asset_id,
    ).update({WardrobeItemPhoto.is_primary: True})
    db.commit()
