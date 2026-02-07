# backend/app/services/assets_service.py

from __future__ import annotations

import json
import os
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4

from sqlalchemy.orm import Session

from app.core.storage import get_storage
from app.db.models import FileAsset, PhotoAsset, User


def _ensure_user(db: Session, user_id: str) -> None:
    user = db.get(User, user_id)
    if user is None:
        db.add(User(id=user_id))
        db.commit()


def _sanitize_filename(filename: str) -> str:
    if not filename:
        return "upload.bin"
    base = os.path.basename(filename)
    return base.replace("..", "_")


def _build_storage_key(user_id: str, kind: str, filename: str) -> str:
    safe = _sanitize_filename(filename)
    return f"{user_id}/{kind}/{uuid4().hex}_{safe}"


def _serialize_tags(tags: Optional[List[str]]) -> str:
    if not tags:
        return "[]"
    return json.dumps([t.strip() for t in tags if t and t.strip()], ensure_ascii=False)


def _serialize_metadata(metadata: Optional[Dict[str, Any]]) -> str:
    if not metadata:
        return "{}"
    return json.dumps(metadata, ensure_ascii=False)


def save_file_asset(
    db: Session,
    user_id: str,
    filename: str,
    content_type: Optional[str],
    data: bytes,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> FileAsset:
    _ensure_user(db, user_id)
    storage = get_storage()
    key = _build_storage_key(user_id, "files", filename)
    storage.put_bytes(key, data, content_type=content_type)

    asset = FileAsset(
        user_id=user_id,
        filename=_sanitize_filename(filename),
        content_type=content_type,
        size_bytes=len(data) if data else 0,
        storage_key=key,
        tags_json=_serialize_tags(tags),
        metadata_json=_serialize_metadata(metadata),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(asset)
    db.commit()
    db.refresh(asset)
    return asset


def save_photo_asset(
    db: Session,
    user_id: str,
    filename: str,
    content_type: Optional[str],
    data: bytes,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> PhotoAsset:
    _ensure_user(db, user_id)
    storage = get_storage()
    key = _build_storage_key(user_id, "photos", filename)
    storage.put_bytes(key, data, content_type=content_type)

    asset = PhotoAsset(
        user_id=user_id,
        filename=_sanitize_filename(filename),
        content_type=content_type,
        size_bytes=len(data) if data else 0,
        storage_key=key,
        tags_json=_serialize_tags(tags),
        metadata_json=_serialize_metadata(metadata),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(asset)
    db.commit()
    db.refresh(asset)
    return asset


def list_file_assets(db: Session, user_id: str, limit: int = 50) -> List[FileAsset]:
    return (
        db.query(FileAsset)
        .filter(FileAsset.user_id == user_id)
        .order_by(FileAsset.created_at.desc())
        .limit(limit)
        .all()
    )


def list_photo_assets(db: Session, user_id: str, limit: int = 50) -> List[PhotoAsset]:
    return (
        db.query(PhotoAsset)
        .filter(PhotoAsset.user_id == user_id)
        .order_by(PhotoAsset.created_at.desc())
        .limit(limit)
        .all()
    )


def search_file_assets(db: Session, user_id: str, query: str, limit: int = 20) -> List[FileAsset]:
    q = f"%{query}%"
    return (
        db.query(FileAsset)
        .filter(FileAsset.user_id == user_id)
        .filter((FileAsset.filename.ilike(q)) | (FileAsset.tags_json.ilike(q)))  # type: ignore
        .order_by(FileAsset.created_at.desc())
        .limit(limit)
        .all()
    )


def search_photo_assets(db: Session, user_id: str, query: str, limit: int = 20) -> List[PhotoAsset]:
    q = f"%{query}%"
    return (
        db.query(PhotoAsset)
        .filter(PhotoAsset.user_id == user_id)
        .filter((PhotoAsset.filename.ilike(q)) | (PhotoAsset.tags_json.ilike(q)))  # type: ignore
        .order_by(PhotoAsset.created_at.desc())
        .limit(limit)
        .all()
    )


def get_file_asset(db: Session, user_id: str, asset_id: int) -> Optional[FileAsset]:
    return (
        db.query(FileAsset)
        .filter(FileAsset.user_id == user_id, FileAsset.id == asset_id)
        .one_or_none()
    )


def get_photo_asset(db: Session, user_id: str, asset_id: int) -> Optional[PhotoAsset]:
    return (
        db.query(PhotoAsset)
        .filter(PhotoAsset.user_id == user_id, PhotoAsset.id == asset_id)
        .one_or_none()
    )


def get_asset_url(storage_key: str, expires_seconds: int = 3600) -> str:
    storage = get_storage()
    return storage.get_url(storage_key, expires_seconds=expires_seconds)
