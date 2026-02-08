import uuid

from app.core.config import settings
from app.db.database import SessionLocal
from app.services.assets_service import save_file_asset, save_photo_asset
import app.services.file_semantic_search as file_semantic_search
import app.services.photo_semantic_search as photo_semantic_search


class StubVectorStore:
    def __init__(self, matches):
        self._matches = matches

    def upsert(self, ids, vectors, metadata=None, namespace=None):
        return None

    def query(self, vector, top_k=10, filter=None, namespace=None):
        return self._matches


def test_file_semantic_search(monkeypatch):
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    asset = save_file_asset(
        db=db,
        user_id=user_id,
        filename="report.txt",
        content_type="text/plain",
        data=b"financial report",
        tags=["finance"],
    )

    monkeypatch.setattr(settings, "FILE_EMBEDDINGS_ENABLED", "1", raising=False)
    monkeypatch.setattr(file_semantic_search, "embed_texts", lambda texts: [[0.1, 0.2, 0.3]])
    monkeypatch.setattr(
        file_semantic_search,
        "get_vector_store",
        lambda: StubVectorStore([
            {"id": f"file:{asset.id}", "score": 0.91, "metadata": {"asset_id": asset.id, "user_id": user_id, "asset_type": "file"}}
        ]),
    )

    results = file_semantic_search.semantic_search_files(db, user_id, "report", top_k=5)
    assert results
    assert results[0]["asset"].id == asset.id

    db.close()


def test_photo_semantic_search(monkeypatch):
    db = SessionLocal()
    user_id = f"user_{uuid.uuid4().hex[:8]}"

    asset = save_photo_asset(
        db=db,
        user_id=user_id,
        filename="trip.jpg",
        content_type="image/jpeg",
        data=b"jpeg-bytes",
        tags=["vacation"],
    )

    monkeypatch.setattr(settings, "PHOTO_EMBEDDINGS_ENABLED", "1", raising=False)
    monkeypatch.setattr(photo_semantic_search, "embed_texts", lambda texts: [[0.2, 0.1, 0.4]])
    monkeypatch.setattr(
        photo_semantic_search,
        "get_vector_store",
        lambda: StubVectorStore([
            {"id": f"photo:{asset.id}", "score": 0.88, "metadata": {"asset_id": asset.id, "user_id": user_id, "asset_type": "photo"}}
        ]),
    )

    results = photo_semantic_search.semantic_search_photos(db, user_id, "beach", top_k=5)
    assert results
    assert results[0]["asset"].id == asset.id

    db.close()
