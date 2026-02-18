from __future__ import annotations

from logging.config import fileConfig
import os

from alembic import context
from sqlalchemy import engine_from_config, pool

config = context.config

if config.config_file_name is not None:
    fileConfig(config.config_file_name)

# IMPORTANT: import Base + models so metadata is populated
from app.db.database import Base  # noqa: E402
import app.db.models  # noqa: F401,E402

target_metadata = Base.metadata


def _normalize_sqlalchemy_url(url: str) -> str:
    # ECS secrets currently provide asyncpg DSNs; Alembic uses a sync SQLAlchemy engine.
    if url.startswith("postgresql+asyncpg://"):
        return url.replace("postgresql+asyncpg://", "postgresql+psycopg://", 1)
    # Some platforms still use the deprecated postgres:// scheme.
    if url.startswith("postgres://"):
        return url.replace("postgres://", "postgresql+psycopg://", 1)
    # Ensure a driver we actually ship (psycopg) is selected by SQLAlchemy.
    if url.startswith("postgresql://"):
        return url.replace("postgresql://", "postgresql+psycopg://", 1)
    return url


def get_url() -> str:
    # Prefer env var, fallback to alembic.ini value
    raw = os.getenv("DATABASE_URL") or config.get_main_option("sqlalchemy.url") or ""
    return _normalize_sqlalchemy_url(raw)


def run_migrations_offline() -> None:
    url = get_url()
    context.configure(
        url=url,
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
        compare_type=True,
    )

    with context.begin_transaction():
        context.run_migrations()


def run_migrations_online() -> None:
    configuration = config.get_section(config.config_ini_section) or {}
    configuration["sqlalchemy.url"] = get_url()

    connectable = engine_from_config(
        configuration,
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
        future=True,
    )

    with connectable.connect() as connection:
        context.configure(
            connection=connection,
            target_metadata=target_metadata,
            compare_type=True,
        )

        with context.begin_transaction():
            context.run_migrations()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
