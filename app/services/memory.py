# backend/app/services/memory.py

import os
from datetime import datetime
from openai import OpenAI
from sqlalchemy.orm import Session

from app.db.models import MemoryNote

client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))


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
        model=os.getenv("OPENAI_MODEL", "gpt-4.1"),
        input=[{"role": "user", "content": prompt}],
    )

    new_mem = (resp.output_text or "").strip()
    upsert_user_memory(db, user_id, new_mem)