# backend/app/db/session.py
"""
Compatibility module.

Some parts of the app expect database session utilities to live in app.db.session.
So we re-export the canonical objects from app.db.database.
"""

from app.db.database import Base, SessionLocal, engine, get_db  # noqa: F401