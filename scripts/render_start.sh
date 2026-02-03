#!/usr/bin/env bash
set -euo pipefail

echo "==> Render start: checking Alembic state"

if [[ ! -f "alembic.ini" ]]; then
  echo "alembic.ini not found in $(pwd)"
  exit 1
fi

# Determine if alembic_version exists and whether DB already has tables.
python - <<'PY'
import os
from sqlalchemy import create_engine, text

url = os.getenv("DATABASE_URL")
if not url:
    raise SystemExit("DATABASE_URL not set")

engine = create_engine(url)
with engine.connect() as conn:
    # Check for alembic_version table
    has_version = conn.execute(
        text("""
        SELECT EXISTS (
            SELECT 1
            FROM information_schema.tables
            WHERE table_schema = 'public' AND table_name = 'alembic_version'
        )
        """)
    ).scalar()

    # Check if any user tables already exist (excluding alembic_version)
    has_tables = conn.execute(
        text("""
        SELECT EXISTS (
            SELECT 1
            FROM information_schema.tables
            WHERE table_schema = 'public'
              AND table_name NOT IN ('alembic_version')
        )
        """)
    ).scalar()

print("ALEMBIC_HAS_VERSION", bool(has_version))
print("DB_HAS_TABLES", bool(has_tables))

with open("/tmp/alembic_state.env", "w") as f:
    f.write(f"ALEMBIC_HAS_VERSION={int(bool(has_version))}\n")
    f.write(f"DB_HAS_TABLES={int(bool(has_tables))}\n")
PY

source /tmp/alembic_state.env

if [[ "${ALEMBIC_HAS_VERSION}" == "0" && "${DB_HAS_TABLES}" == "1" ]]; then
  echo "==> Stamping alembic head (pre-existing tables detected)"
  python -m alembic -c alembic.ini stamp head
fi

echo "==> Running alembic upgrade head"
python -m alembic -c alembic.ini upgrade head

echo "==> Starting app"
exec uvicorn app.main:app --host 0.0.0.0 --port "${PORT:-8000}"
