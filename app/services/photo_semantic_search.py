from __future__ import annotations

import base64
import json
import logging
from typing import Any, Dict, List, Optional, Tuple

from sqlalchemy.orm import Session
from openai import OpenAI

from app.core.config import settings
from app.core.vector_store import get_vector_store
from app.db.models import PhotoAsset
from app.services.embeddings import embed_texts

logger = logging.getLogger(__name__)

PHOTO_NAMESPACE = "photos"


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


def _fallback_tags_from_filename(filename: Optional[str]) -> List[str]:
    if not filename:
        return []
    base = filename.rsplit("/", 1)[-1]
    base = base.rsplit(".", 1)[0]
    tokens = [t for t in base.replace("_", " ").replace("-", " ").split() if t]
    return _normalize_tags(tokens)


def generate_photo_tags(
    data: bytes,
    content_type: Optional[str],
    filename: Optional[str] = None,
) -> Tuple[List[str], str]:
    if settings.PHOTO_TAGGING_ENABLED != "1":
        return [], ""

    if not settings.OPENAI_API_KEY:
        return _fallback_tags_from_filename(filename), ""

    if not data:
        return _fallback_tags_from_filename(filename), ""

    max_bytes = settings.PHOTO_TAGGING_MAX_BYTES
    if max_bytes and len(data) > max_bytes:
        logger.info("Photo too large for auto-tagging (%d bytes)", len(data))
        return _fallback_tags_from_filename(filename), ""

    ctype = content_type or "image/jpeg"
    try:
        b64 = base64.b64encode(data).decode("utf-8")
    except Exception:
        return _fallback_tags_from_filename(filename), ""

    client = OpenAI(api_key=settings.OPENAI_API_KEY)
    system = (
        "You are a computer vision assistant. "
        "Return ONLY valid JSON with keys: caption (string), tags (list of short tags)."
    )
    user_content = [
        {"type": "text", "text": "Describe the photo and provide 5-15 concise tags."},
        {"type": "image_url", "image_url": {"url": f"data:{ctype};base64,{b64}"}},
    ]

    try:
        resp = client.chat.completions.create(
            model=settings.OPENAI_VISION_MODEL or settings.OPENAI_MODEL,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user_content},
            ],
            temperature=0.2,
        )
        raw = resp.choices[0].message.content or "{}"
        data_obj = json.loads(raw)
        tags = _normalize_tags(data_obj.get("tags") or [])
        caption = data_obj.get("caption") or ""
        return tags, caption
    except Exception as exc:
        logger.warning("Photo tagging failed: %s", exc)
        return _fallback_tags_from_filename(filename), ""


def _build_document(filename: str, tags: List[str], caption: str) -> str:
    parts = [filename]
    if caption:
        parts.append(caption)
    if tags:
        parts.append("Tags: " + ", ".join(tags))
    return "\n".join([p for p in parts if p])


def index_photo_asset(
    db: Session,
    asset: PhotoAsset,
    tags: Optional[List[str]] = None,
    caption: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
) -> bool:
    if settings.PHOTO_EMBEDDINGS_ENABLED != "1":
        return False

    tags_list = _normalize_tags(tags)
    document = _build_document(asset.filename, tags_list, caption or "")
    if not document:
        return False

    try:
        store = get_vector_store()
    except Exception as exc:
        logger.warning("Vector store unavailable for photo embeddings: %s", exc)
        return False

    try:
        vector = embed_texts([document])[0]
        meta = {
            "user_id": asset.user_id,
            "asset_id": asset.id,
            "asset_type": "photo",
            "filename": asset.filename,
            "tags": tags_list,
        }
        if caption:
            meta["caption"] = caption
        if metadata:
            meta.update(metadata)
        store.upsert(
            ids=[f"photo:{asset.id}"],
            vectors=[vector],
            metadata=[meta],
            namespace=PHOTO_NAMESPACE,
        )
        return True
    except Exception as exc:
        logger.warning("Photo embedding failed: %s", exc)
        return False


def semantic_search_photos(
    db: Session,
    user_id: str,
    query: str,
    top_k: int = 10,
) -> List[Dict[str, Any]]:
    if settings.PHOTO_EMBEDDINGS_ENABLED != "1":
        raise RuntimeError("Photo embeddings are disabled")

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
        filter={"user_id": user_id, "asset_type": "photo"},
        namespace=PHOTO_NAMESPACE,
    )

    asset_ids: List[int] = []
    scores: Dict[int, float] = {}
    for item in results:
        meta = item.get("metadata") or {}
        asset_id = meta.get("asset_id")
        if asset_id is None:
            raw_id = item.get("id") or ""
            if isinstance(raw_id, str) and raw_id.startswith("photo:"):
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
        db.query(PhotoAsset)
        .filter(PhotoAsset.user_id == user_id, PhotoAsset.id.in_(asset_ids))
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
