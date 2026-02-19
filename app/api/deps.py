# backend/app/api/deps.py
"""
Shared dependencies (FastAPI).
Avoid repeating "get or create user" and "get_db" in every route.
"""

from typing import Generator

from sqlalchemy.orm import Session

from app.db.database import SessionLocal  # <-- correct source
from app.db.models import User
from app.db.user_compat import ensure_user_row


def get_or_create_user(db: Session, user_id: str) -> User:
    uid = str(user_id or "").strip()
    if not uid:
        raise ValueError("user_id is required")
    ensure_user_row(db, uid)
    # Most call-sites only use this as an existence guard.
    return User(id=uid)


def get_db() -> Generator[Session, None, None]:
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
