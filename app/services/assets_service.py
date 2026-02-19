# backend/app/services/assets_service.py

from __future__ import annotations

import json
import logging
import os
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4

from sqlalchemy.orm import Session

from app.core.storage import get_storage
from app.db.models import FileAsset, PhotoAsset
from app.db.user_compat import ensure_user_row
from app.services.analytics_service import record_usage_event
from app.services import file_semantic_search, photo_semantic_search

logger = logging.getLogger(__name__)


def _ensure_user(db: Session, user_id: str) -> None:
    ensure_user_row(db, user_id)


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


def _merge_tags(*tag_sets: Optional[List[str]]) -> List[str]:
    merged: List[str] = []
    seen: set[str] = set()
    for tags in tag_sets:
        if not tags:
            continue
        for tag in tags:
            if not tag:
                continue
            cleaned = tag.strip()
            if not cleaned:
                continue
            key = cleaned.lower()
            if key in seen:
                continue
            seen.add(key)
            merged.append(cleaned)
    return merged


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
    try:
        file_semantic_search.index_file_asset(db, asset, data, tags=tags or None, metadata=metadata)
    except Exception as exc:
        logger.warning("File embedding skipped: %s", exc)
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="file_upload",
            source="files",
            metadata={"filename": asset.filename, "content_type": asset.content_type, "size": asset.size_bytes},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
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

    auto_tags: List[str] = []
    caption = ""
    try:
        auto_tags, caption = photo_semantic_search.generate_photo_tags(
            data=data,
            content_type=content_type,
            filename=filename,
        )
    except Exception as exc:
        logger.warning("Photo auto-tagging failed: %s", exc)

    merged_tags = _merge_tags(tags, auto_tags)
    meta = dict(metadata or {})
    if caption:
        meta["caption"] = caption
    if auto_tags:
        meta["auto_tags"] = auto_tags

    asset = PhotoAsset(
        user_id=user_id,
        filename=_sanitize_filename(filename),
        content_type=content_type,
        size_bytes=len(data) if data else 0,
        storage_key=key,
        tags_json=_serialize_tags(merged_tags),
        metadata_json=_serialize_metadata(meta),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(asset)
    db.commit()
    db.refresh(asset)
    try:
        photo_semantic_search.index_photo_asset(
            db,
            asset,
            tags=merged_tags,
            caption=caption or None,
            metadata=meta,
        )
    except Exception as exc:
        logger.warning("Photo embedding skipped: %s", exc)
    try:
        record_usage_event(
            db,
            user_id=user_id,
            event_type="photo_upload",
            source="photos",
            metadata={"filename": asset.filename, "content_type": asset.content_type, "size": asset.size_bytes},
        )
    except Exception as exc:
        logger.warning("Usage event failed: %s", exc)
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
