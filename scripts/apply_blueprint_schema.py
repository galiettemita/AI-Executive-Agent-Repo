import os
import sys
from pathlib import Path
from urllib.parse import urlparse

import psycopg


def normalize_url(raw: str) -> str:
    return raw.replace("postgresql+asyncpg://", "postgresql://", 1)


def with_db(parsed, dbname: str) -> str:
    return parsed._replace(path=f"/{dbname}").geturl()


def main() -> int:
    raw = os.environ.get("DATABASE_URL")
    if not raw:
        print("DATABASE_URL is not set", file=sys.stderr)
        return 1

    parsed = urlparse(normalize_url(raw))
    base_url = with_db(parsed, "postgres")
    app_url = with_db(parsed, "executive_os")

    # Create database if it doesn't exist.
    with psycopg.connect(base_url) as conn:
        conn.autocommit = True
        with conn.cursor() as cur:
            cur.execute("SELECT 1 FROM pg_database WHERE datname=%s", ("executive_os",))
            if cur.fetchone() is None:
                cur.execute("CREATE DATABASE executive_os")
                print("created database executive_os")
            else:
                print("database executive_os already exists")

    sql_path = Path(__file__).with_name("blueprint_schema.sql")
    sql = sql_path.read_text()
    statements = [stmt.strip() for stmt in sql.split(";") if stmt.strip()]

    with psycopg.connect(app_url) as conn:
        conn.autocommit = True
        with conn.cursor() as cur:
            for stmt in statements:
                cur.execute(stmt)
    print(f"schema applied ({len(statements)} statements)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
