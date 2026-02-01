#database connection + session setup

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, DeclarativeBase
from app.core.config import settings

def _make_engine():
    # SQLite needs this flag for multithreading (FastAPI dev server)
    if settings.DATABASE_URL.startswith("sqlite"):
        return create_engine(settings.DATABASE_URL, connect_args={"check_same_thread": False})
    return create_engine(settings.DATABASE_URL)

engine = _make_engine()

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

class Base(DeclarativeBase):
    pass

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
