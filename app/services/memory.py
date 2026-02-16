# backend/app/services/memory.py

from datetime import datetime
import logging
from app.services.llm_client import OpenAIProxy as OpenAI
from sqlalchemy.orm import Session

from app.db.models import MemoryNote
from app.core.config import settings
from app.core.vector_store import get_vector_store
from app.services.embeddings import embed_texts

client = OpenAI(api_key=settings.OPENAI_API_KEY)
logger = logging.getLogger(__name__)
_MEMORY_NAMESPACE = "memory_notes"


def get_user_memory(db: Session, user_id: str) -> str:
    note = db.query(MemoryNote).filter(MemoryNote.user_id == user_id).first()
    return (note.summary or "").strip() if note else ""


def upsert_user_memory(db: Session, user_id: str, new_summary: str) -> None:
    note = db.query(MemoryNote).filter(MemoryNote.user_id == user_id).first()
    if not note:
        note = MemoryNote(user_id=user_id, summary=new_summary, updated_at=datetime.utcnow())
        db.add(note)
    else:
        note.summary = new_summary
        note.updated_at = datetime.utcnow()

    db.commit()
    _sync_memory_embedding(user_id=user_id, summary=new_summary)


def _sync_memory_embedding(*, user_id: str, summary: str) -> None:
    summary = (summary or "").strip()
    if not summary:
        return
    try:
        vector = embed_texts([summary])[0]
        get_vector_store().upsert(
            ids=[user_id],
            vectors=[vector],
            metadata=[{"user_id": user_id, "summary": summary, "updated_at": datetime.utcnow().isoformat()}],
            namespace=_MEMORY_NAMESPACE,
        )
    except Exception as exc:
        logger.warning("Memory embedding sync failed: %s", exc)


def get_user_memory_context(db: Session, user_id: str, query_text: str) -> str:
    summary = get_user_memory(db, user_id)
    if not summary:
        return ""

    query_text = (query_text or "").strip()
    if not query_text:
        return summary

    try:
        query_vec = embed_texts([query_text])[0]
        rows = get_vector_store().query(
            vector=query_vec,
            top_k=1,
            filter={"user_id": user_id},
            namespace=_MEMORY_NAMESPACE,
        )
        if rows and rows[0].get("score") is not None and float(rows[0]["score"]) >= 0.65:
            meta = rows[0].get("metadata") or {}
            cached_summary = str(meta.get("summary") or "").strip()
            if cached_summary:
                return cached_summary
    except Exception as exc:
        logger.warning("Memory vector lookup failed: %s", exc)

    return summary


def update_memory_from_turn(
    db: Session,
    user_id: str,
    user_message: str,
    assistant_message: str,
) -> None:
    """
    Keeps a rolling memory summary per user.
    This is intentionally short and stable (preferences, ongoing tasks, important context).
    """
    old = get_user_memory(db, user_id)

    prompt = f"""
You maintain a SHORT rolling memory for a personal assistant.
Update the memory using the new conversation turn.

Rules:
- Keep under 1200 characters.
- Store only stable preferences, ongoing tasks, important people/things, and commitments.
- Do NOT store secrets (passwords, auth codes), payment card numbers, or highly sensitive info.
- If something becomes irrelevant, remove it.

OLD MEMORY:
{old}

NEW TURN:
User: {user_message}
Assistant: {assistant_message}

Return ONLY the updated memory text.
""".strip()

    resp = client.responses.create(
        model=settings.OPENAI_MODEL,
        input=[{"role": "user", "content": prompt}],
    )

    new_mem = (resp.output_text or "").strip()
    upsert_user_memory(db, user_id, new_mem)
