#!/usr/bin/env python3
"""
Backfill Pinecone export data into Postgres pgvector table `vector_store_items`.

Expected input format (JSON array or JSONL):
{
  "id": "doc-1",
  "values": [ ... ],            # or "embedding"
  "metadata": { ... },
  "namespace": "optional"
}
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

import psycopg


def _normalize_dsn(raw: str) -> str:
    return raw.replace("postgresql+asyncpg://", "postgresql://", 1)


def _vector_literal(vector: list[float]) -> str:
    return "[" + ",".join(f"{v:.6f}" for v in vector) + "]"


def _iter_rows(path: Path):
    text = path.read_text(encoding="utf-8").strip()
    if not text:
        return
    if text.startswith("["):
        payload = json.loads(text)
        for row in payload:
            yield row
        return
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        yield json.loads(line)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, help="Path to Pinecone export json/jsonl file")
    parser.add_argument("--namespace-prefix", default="", help="Prefix added to source namespace")
    parser.add_argument(
        "--dsn",
        default=os.getenv("PGVECTOR_DSN") or os.getenv("DATABASE_URL", ""),
        help="Postgres DSN (defaults to PGVECTOR_DSN or DATABASE_URL)",
    )
    args = parser.parse_args()

    if not args.dsn:
        raise SystemExit("Missing DSN. Set --dsn or PGVECTOR_DSN or DATABASE_URL.")

    src = Path(args.input)
    if not src.exists():
        raise SystemExit(f"Input file not found: {src}")

    dsn = _normalize_dsn(args.dsn)
    inserted = 0

    sql = (
        "INSERT INTO vector_store_items (id, namespace, embedding, metadata) "
        "VALUES (%s, %s, (%s)::vector, (%s)::jsonb) "
        "ON CONFLICT (id, namespace) DO UPDATE "
        "SET embedding = EXCLUDED.embedding, metadata = EXCLUDED.metadata"
    )

    with psycopg.connect(dsn) as conn:
        with conn.cursor() as cur:
            for raw in _iter_rows(src):
                item_id = str(raw.get("id") or "").strip()
                vector = raw.get("values") or raw.get("embedding")
                metadata = raw.get("metadata") or {}
                namespace = str(raw.get("namespace") or "pinecone-backfill")
                if args.namespace_prefix:
                    namespace = f"{args.namespace_prefix}{namespace}"
                if not item_id or not isinstance(vector, list) or not vector:
                    continue
                cur.execute(
                    sql,
                    (
                        item_id,
                        namespace,
                        _vector_literal([float(v) for v in vector]),
                        json.dumps(metadata),
                    ),
                )
                inserted += 1

    print(f"Backfill complete. Upserted rows: {inserted}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
