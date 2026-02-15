#database connection + session setup

import logging

from sqlalchemy import create_engine, text, event
from sqlalchemy.orm import sessionmaker, DeclarativeBase, Session
from app.core.config import settings
from app.core.log_context import get_user_id

logger = logging.getLogger(__name__)
RLS_FALLBACK_USER_ID = "00000000-0000-0000-0000-000000000000"

def _normalize_database_url(url: str) -> str:
    # ECS secrets currently provide asyncpg DSNs; this code path uses sync SQLAlchemy sessions.
    if url.startswith("postgresql+asyncpg://"):
        return url.replace("postgresql+asyncpg://", "postgresql+psycopg://", 1)
    return url

def _make_engine():
    db_url = _normalize_database_url(settings.DATABASE_URL)
    # SQLite needs this flag for multithreading (FastAPI dev server)
    if db_url.startswith("sqlite"):
        return create_engine(db_url, connect_args={"check_same_thread": False})
    return create_engine(db_url)

engine = _make_engine()

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

class Base(DeclarativeBase):
    pass

@event.listens_for(Session, "after_begin")
def _set_app_user_id(session, transaction, connection):  # noqa: ARG001
    if settings.DATABASE_URL.startswith("sqlite"):
        return
    user_id = get_user_id() or RLS_FALLBACK_USER_ID
    try:
        connection.execute(
            text("select set_config('app.user_id', :user_id, true)"),
            {"user_id": user_id},
        )
    except Exception:
        logger.exception("Failed to set app.user_id on DB session")

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
