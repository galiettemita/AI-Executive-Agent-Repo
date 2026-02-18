from __future__ import annotations

from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.vector_store import get_vector_store
from app.services.embeddings import embed_texts


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _latest_knowledge_rows(db: Session, *, user_id: str) -> list[dict[str, Any]]:
    rows = db.execute(
        text(
            """
            select file_path, version, content, coalesce(updated_at, created_at) as freshness
            from knowledge_files
            where user_id = :user_id
            order by file_path asc, version desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    latest_by_file: dict[str, dict[str, Any]] = {}
    for row in rows:
        file_path = str(row.get("file_path") or "").strip()
        if not file_path or file_path in latest_by_file:
            continue
        latest_by_file[file_path] = dict(row)
    return [latest_by_file[key] for key in sorted(latest_by_file.keys())]


def run_embedding_reembed_audit(db: Session, *, user_id: str) -> dict[str, Any]:
    if not _table_exists(db, "knowledge_files"):
        return {"ok": False, "user_id": user_id, "reason": "knowledge_files_missing"}

    rows = _latest_knowledge_rows(db, user_id=user_id)
    if not rows:
        return {"ok": True, "user_id": user_id, "embedded": 0, "skipped": 0}

    ids: list[str] = []
    texts: list[str] = []
    metadata: list[dict[str, Any]] = []
    skipped = 0
    for row in rows:
        content = str(row.get("content") or "").strip()
        if not content:
            skipped += 1
            continue
        file_path = str(row.get("file_path") or "")
        version = int(row.get("version") or 1)
        ids.append(f"knowledge:{user_id}:{file_path}:{version}")
        texts.append(content[:10000])
        metadata.append(
            {
                "user_id": user_id,
                "file_path": file_path,
                "version": version,
                "freshness": str(row.get("freshness") or ""),
                "embedded_at": datetime.utcnow().isoformat(),
                "model": str(getattr(settings, "OPENAI_EMBEDDING_MODEL", "") or "text-embedding-3-small"),
            }
        )

    if not texts:
        return {"ok": True, "user_id": user_id, "embedded": 0, "skipped": skipped}

    vectors = embed_texts(texts)
    try:
        vector_store = get_vector_store()
    except Exception as exc:
        return {
            "ok": False,
            "user_id": user_id,
            "embedded": 0,
            "skipped": skipped,
            "reason": f"vector_store_not_configured: {exc}",
        }
    vector_store.upsert(ids=ids, vectors=vectors, metadata=metadata, namespace=f"knowledge:{user_id}")
    return {
        "ok": True,
        "user_id": user_id,
        "embedded": len(ids),
        "skipped": skipped,
    }


def run_embedding_reembed_audit_all_users(db: Session) -> dict[str, Any]:
    if not _table_exists(db, "knowledge_files"):
        return {"ok": False, "reason": "knowledge_files_missing", "processed": 0, "results": []}
    rows = db.execute(
        text(
            """
            select distinct user_id
            from knowledge_files
            where user_id is not null
            """
        )
    ).mappings().all()
    users = sorted({str(row.get("user_id") or "").strip() for row in rows if str(row.get("user_id") or "").strip()})
    results: list[dict[str, Any]] = []
    for user_id in users:
        try:
            results.append(run_embedding_reembed_audit(db, user_id=user_id))
        except Exception as exc:
            results.append({"ok": False, "user_id": user_id, "error": str(exc)})
    return {"ok": True, "processed": len(users), "results": results}
