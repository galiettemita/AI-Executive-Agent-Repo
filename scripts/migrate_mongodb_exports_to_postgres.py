#!/usr/bin/env python3
"""
Import MongoDB export files (JSON/JSONL) into matching Postgres tables.

Usage:
  python scripts/migrate_mongodb_exports_to_postgres.py --export-dir ./mongo-export

Rules:
- Each file name maps to a table name (e.g. users.json -> users).
- Only columns that already exist in the target table are inserted.
- `_id` is mapped to `id` when possible.
- Dict/list values are JSON-encoded for non-JSON columns.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from typing import Iterable

import psycopg


def _normalize_dsn(raw: str) -> str:
    return raw.replace("postgresql+asyncpg://", "postgresql://", 1)


def _iter_docs(path: Path) -> Iterable[dict]:
    text = path.read_text(encoding="utf-8").strip()
    if not text:
        return []
    if text.startswith("["):
        payload = json.loads(text)
        return (doc for doc in payload if isinstance(doc, dict))
    return (
        json.loads(line)
        for line in text.splitlines()
        if line.strip()
    )


def _column_map(cur: psycopg.Cursor, table: str) -> dict[str, str]:
    cur.execute(
        """
        SELECT column_name, data_type
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = %s
        """,
        (table,),
    )
    return {row[0]: row[1] for row in cur.fetchall()}


def _table_exists(cur: psycopg.Cursor, table: str) -> bool:
    cur.execute("SELECT to_regclass(%s)", (table,))
    row = cur.fetchone()
    return bool(row and row[0])


def _coerce(value, data_type: str):
    if value is None:
        return None
    if isinstance(value, (dict, list)):
        # Keep JSON types as JSON; fallback to text.
        if "json" in data_type.lower():
            return json.dumps(value)
        return json.dumps(value, ensure_ascii=False)
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value
    return str(value)


def _prepare_row(doc: dict, columns: dict[str, str]) -> dict:
    row = dict(doc)
    if "_id" in row and "id" in columns and "id" not in row:
        row["id"] = row.pop("_id")
    elif "_id" in row:
        row.pop("_id")

    cleaned = {}
    for key, value in row.items():
        if key not in columns:
            continue
        cleaned[key] = _coerce(value, columns[key])
    return cleaned


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--export-dir", required=True, help="Directory containing collection export files")
    parser.add_argument(
        "--dsn",
        default=os.getenv("DATABASE_URL", ""),
        help="Postgres DSN (defaults to DATABASE_URL)",
    )
    args = parser.parse_args()

    export_dir = Path(args.export_dir)
    if not export_dir.exists() or not export_dir.is_dir():
        raise SystemExit(f"Export directory not found: {export_dir}")

    if not args.dsn:
        raise SystemExit("Missing DSN. Set --dsn or DATABASE_URL.")

    dsn = _normalize_dsn(args.dsn)
    files = sorted([p for p in export_dir.iterdir() if p.suffix.lower() in {".json", ".jsonl", ".ndjson"}])
    if not files:
        print("No export files found.")
        return 0

    total_inserted = 0
    with psycopg.connect(dsn) as conn:
        with conn.cursor() as cur:
            for path in files:
                table = path.stem
                if not _table_exists(cur, table):
                    print(f"skip {path.name}: table '{table}' not found")
                    continue
                columns = _column_map(cur, table)
                if not columns:
                    print(f"skip {path.name}: no columns discovered")
                    continue

                inserted = 0
                for doc in _iter_docs(path):
                    row = _prepare_row(doc, columns)
                    if not row:
                        continue
                    keys = list(row.keys())
                    placeholders = ", ".join(["%s"] * len(keys))
                    cols = ", ".join(keys)
                    values = [row[k] for k in keys]
                    sql = f"INSERT INTO {table} ({cols}) VALUES ({placeholders})"
                    if "id" in keys:
                        sql += " ON CONFLICT (id) DO NOTHING"
                    try:
                        cur.execute(sql, values)
                        inserted += 1
                    except Exception as exc:
                        conn.rollback()
                        print(f"warn {path.name}: failed row insert ({exc.__class__.__name__})")
                        continue
                total_inserted += inserted
                print(f"{path.name}: inserted {inserted} rows into {table}")
    print(f"Migration complete. Total rows inserted: {total_inserted}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
