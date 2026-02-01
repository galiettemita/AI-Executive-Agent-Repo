# backend/app/api/deps.py
"""
Shared dependencies (FastAPI).
Avoid repeating "get or create user" and "get_db" in every route.
"""

from typing import Generator

from sqlalchemy.orm import Session

from app.db.models import User
from app.db.database import SessionLocal  # <-- correct source


def get_or_create_user(db: Session, user_id: str) -> User:
    user = db.get(User, user_id)
    if user is None:
        user = User(id=user_id)
        db.add(user)
        db.commit()
        db.refresh(user)
    return user


def get_db() -> Generator[Session, None, None]:
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()