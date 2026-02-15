import os
import sys
from pathlib import Path

import psycopg


def normalize_url(raw: str) -> str:
    return raw.replace("postgresql+asyncpg://", "postgresql://", 1)


def main() -> int:
    raw = os.environ.get("DATABASE_URL")
    if not raw:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    sql_path = Path(__file__).with_name("pgvector_store.sql")
    sql = sql_path.read_text()
    statements = [stmt.strip() for stmt in sql.split(";") if stmt.strip()]

    with psycopg.connect(normalize_url(raw)) as conn:
        conn.autocommit = True
        with conn.cursor() as cur:
            for stmt in statements:
                cur.execute(stmt)
    print(f"pgvector store applied ({len(statements)} statements)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
