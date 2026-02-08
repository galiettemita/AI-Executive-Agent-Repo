from __future__ import annotations

import io
import logging
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.vector_store import get_vector_store
from app.db.models import FileAsset
from app.services.embeddings import embed_texts

logger = logging.getLogger(__name__)

FILE_NAMESPACE = "files"


def _truncate(text: str, limit: int) -> str:
    if not text:
        return ""
    return text[:limit]


def extract_text_from_file(
    filename: str,
    content_type: Optional[str],
    data: bytes,
) -> str:
    if not data:
        return ""

    ctype = (content_type or "").lower()
    name = (filename or "").lower()

    def _decode_text(blob: bytes) -> str:
        try:
            return blob.decode("utf-8", errors="ignore")
        except Exception:
            return ""

    if ctype.startswith("text/") or name.endswith((".txt", ".md", ".csv", ".log", ".json")):
        return _decode_text(data)

    if ctype in {"application/pdf", "application/x-pdf"} or name.endswith(".pdf"):
        try:
            from PyPDF2 import PdfReader  # type: ignore
        except Exception:
            logger.info("PyPDF2 not installed; skipping PDF text extraction")
            return ""
        try:
            reader = PdfReader(io.BytesIO(data))
            text_parts = []
            for page in reader.pages:
                text_parts.append(page.extract_text() or "")
            return "\n".join(text_parts)
        except Exception as exc:
            logger.warning("Failed to extract PDF text: %s", exc)
            return ""

    return ""


def _normalize_tags(tags: Optional[List[str]]) -> List[str]:
    if not tags:
        return []
    seen: set[str] = set()
    out: List[str] = []
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
        out.append(cleaned)
    return out


def _build_document(filename: str, tags: List[str], text: str) -> str:
    parts = [filename]
    if tags:
        parts.append("Tags: " + ", ".join(tags))
    if text:
        parts.append(text)
    combined = "\n".join([p for p in parts if p])
    limit = settings.FILE_EMBEDDINGS_MAX_CHARS
    return _truncate(combined, limit)


def index_file_asset(
    db: Session,
    asset: FileAsset,
    data: bytes,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> bool:
    if settings.FILE_EMBEDDINGS_ENABLED != "1":
        return False

    tags_list = _normalize_tags(tags)
    text = extract_text_from_file(asset.filename, asset.content_type, data)
    document = _build_document(asset.filename, tags_list, text)

    if not document:
        return False

    try:
        store = get_vector_store()
    except Exception as exc:
        logger.warning("Vector store unavailable for file embeddings: %s", exc)
        return False

    try:
        vector = embed_texts([document])[0]
        meta = {
            "user_id": asset.user_id,
            "asset_id": asset.id,
            "asset_type": "file",
            "filename": asset.filename,
            "tags": tags_list,
        }
        if metadata:
            meta.update(metadata)
        store.upsert(
            ids=[f"file:{asset.id}"],
            vectors=[vector],
            metadata=[meta],
            namespace=FILE_NAMESPACE,
        )
        return True
    except Exception as exc:
        logger.warning("File embedding failed: %s", exc)
        return False


def semantic_search_files(
    db: Session,
    user_id: str,
    query: str,
    top_k: int = 10,
) -> List[Dict[str, Any]]:
    if settings.FILE_EMBEDDINGS_ENABLED != "1":
        raise RuntimeError("File embeddings are disabled")

    if not query:
        return []

    try:
        store = get_vector_store()
    except Exception as exc:
        raise RuntimeError(f"Vector store unavailable: {exc}")

    vector = embed_texts([query])[0]
    results = store.query(
        vector=vector,
        top_k=top_k,
        filter={"user_id": user_id, "asset_type": "file"},
        namespace=FILE_NAMESPACE,
    )

    asset_ids: List[int] = []
    scores: Dict[int, float] = {}
    for item in results:
        meta = item.get("metadata") or {}
        asset_id = meta.get("asset_id")
        if asset_id is None:
            raw_id = item.get("id") or ""
            if isinstance(raw_id, str) and raw_id.startswith("file:"):
                try:
                    asset_id = int(raw_id.split(":", 1)[1])
                except Exception:
                    asset_id = None
        if isinstance(asset_id, int):
            asset_ids.append(asset_id)
            score = item.get("score")
            if isinstance(score, (int, float)):
                scores[asset_id] = float(score)

    if not asset_ids:
        return []

    assets = (
        db.query(FileAsset)
        .filter(FileAsset.user_id == user_id, FileAsset.id.in_(asset_ids))
        .all()
    )
    assets_map = {a.id: a for a in assets}

    ordered = []
    for asset_id in asset_ids:
        asset = assets_map.get(asset_id)
        if not asset:
            continue
        ordered.append({"asset": asset, "score": scores.get(asset_id)})
    return ordered
