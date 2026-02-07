# tests/test_assets_service.py

import uuid

from app.db.database import SessionLocal
from app.services.assets_service import (
    save_file_asset,
    save_photo_asset,
    list_file_assets,
    list_photo_assets,
    search_file_assets,
    search_photo_assets,
)


def test_file_asset_crud():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    asset = save_file_asset(
        db=db,
        user_id=user_id,
        filename="report.pdf",
        content_type="application/pdf",
        data=b"pdf-bytes",
        tags=["finance", "q1"],
    )
    assert asset.id is not None

    listed = list_file_assets(db, user_id, limit=10)
    assert any(a.id == asset.id for a in listed)

    results = search_file_assets(db, user_id, "report", limit=10)
    assert any(a.id == asset.id for a in results)

    db.close()


def test_photo_asset_crud():
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    asset = save_photo_asset(
        db=db,
        user_id=user_id,
        filename="photo.jpg",
        content_type="image/jpeg",
        data=b"jpeg-bytes",
        tags=["vacation"],
    )
    assert asset.id is not None

    listed = list_photo_assets(db, user_id, limit=10)
    assert any(a.id == asset.id for a in listed)

    results = search_photo_assets(db, user_id, "vacation", limit=10)
    assert any(a.id == asset.id for a in results)

    db.close()
