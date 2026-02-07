# backend/app/services/email_semantic_search.py

from __future__ import annotations

from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.core.vector_store import get_vector_store
from app.services.embeddings import embed_texts
from app.services.email_router import list_recent_emails, search_emails


EMAIL_NAMESPACE = "email"


def _build_email_id(provider: str, user_id: str, message_id: str) -> str:
    return f"{provider}:{user_id}:{message_id}"


def index_recent_emails(
    db: Session,
    user_id: str,
    provider: Optional[str] = None,
    max_results: int = 50,
    hours_back: int = 168,
) -> int:
    """
    Fetch recent emails and upsert embeddings into the vector store.
    Returns number of indexed messages.
    """
    emails = list_recent_emails(
        db=db,
        user_id=user_id,
        max_results=max_results,
        hours_back=hours_back,
        unread_only=False,
        provider=provider,
        include_body=True,
    )
    if not emails:
        return 0

    texts = []
    ids = []
    metadata = []
    for msg in emails:
        body = msg.get("body") or msg.get("snippet") or ""
        subject = msg.get("subject") or ""
        sender = msg.get("from") or ""
        text = f"From: {sender}\nSubject: {subject}\n\n{body}".strip()
        if not text:
            continue
        provider_used = msg.get("provider") or provider or "google"
        msg_id = msg.get("id") or ""
        if not msg_id:
            continue
        texts.append(text)
        ids.append(_build_email_id(provider_used, user_id, msg_id))
        metadata.append(
            {
                "user_id": user_id,
                "provider": provider_used,
                "message_id": msg_id,
                "subject": subject,
                "from": sender,
                "date": msg.get("date"),
                "snippet": msg.get("snippet"),
            }
        )

    if not texts:
        return 0

    vectors = embed_texts(texts)
    store = get_vector_store()
    store.upsert(ids=ids, vectors=vectors, metadata=metadata, namespace=EMAIL_NAMESPACE)
    return len(texts)


def semantic_search_emails(
    db: Session,
    user_id: str,
    query: str,
    top_k: int = 5,
    provider: Optional[str] = None,
    fallback_keyword_search: bool = True,
) -> List[Dict[str, Any]]:
    if not query:
        return []

    try:
        vector = embed_texts([query])[0]
        store = get_vector_store()
        filter_obj: Dict[str, Any] = {"user_id": user_id}
        if provider:
            filter_obj["provider"] = provider
        results = store.query(vector=vector, top_k=top_k, filter=filter_obj, namespace=EMAIL_NAMESPACE)
        return results
    except Exception:
        if not fallback_keyword_search:
            raise
        # Fallback to keyword search
        msgs = search_emails(db=db, user_id=user_id, query=query, max_results=top_k, provider=provider)
        return [{"metadata": msg, "score": None} for msg in msgs]
