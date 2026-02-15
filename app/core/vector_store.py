from __future__ import annotations

from typing import Protocol, Optional
import json
import logging

import psycopg

from app.core.config import settings

logger = logging.getLogger(__name__)


class VectorStore(Protocol):
    def upsert(
        self,
        ids: list[str],
        vectors: list[list[float]],
        metadata: list[dict] | None = None,
        namespace: Optional[str] = None,
    ) -> None: ...

    def query(
        self,
        vector: list[float],
        top_k: int = 10,
        filter: dict | None = None,
        namespace: Optional[str] = None,
    ) -> list[dict]: ...


class NotConfiguredVectorStore:
    def upsert(
        self,
        ids: list[str],
        vectors: list[list[float]],
        metadata: list[dict] | None = None,
        namespace: Optional[str] = None,
    ) -> None:
        raise RuntimeError("Vector store not configured")

    def query(
        self,
        vector: list[float],
        top_k: int = 10,
        filter: dict | None = None,
        namespace: Optional[str] = None,
    ) -> list[dict]:
        raise RuntimeError("Vector store not configured")


def _normalize_dsn() -> str:
    dsn = settings.PGVECTOR_DSN or settings.DATABASE_URL
    if not dsn:
        raise RuntimeError("PGVector requires PGVECTOR_DSN or DATABASE_URL")
    return dsn.replace("postgresql+asyncpg://", "postgresql://", 1)


def _vector_literal(vector: list[float]) -> str:
    return "[" + ",".join(f"{v:.6f}" for v in vector) + "]"


class PgvectorVectorStore:
    def __init__(self):
        self._dsn = _normalize_dsn()

    def upsert(
        self,
        ids: list[str],
        vectors: list[list[float]],
        metadata: list[dict] | None = None,
        namespace: Optional[str] = None,
    ) -> None:
        ns = namespace or "default"
        if not ids or not vectors:
            return
        payload = []
        for idx, vector in enumerate(vectors):
            meta = metadata[idx] if metadata and idx < len(metadata) else {}
            payload.append(
                (
                    ids[idx],
                    ns,
                    _vector_literal(vector),
                    json.dumps(meta or {}),
                )
            )

        sql = (
            "INSERT INTO vector_store_items (id, namespace, embedding, metadata) "
            "VALUES (%s, %s, (%s)::vector, (%s)::jsonb) "
            "ON CONFLICT (id, namespace) DO UPDATE SET "
            "embedding = EXCLUDED.embedding, metadata = EXCLUDED.metadata"
        )
        with psycopg.connect(self._dsn) as conn:
            with conn.cursor() as cur:
                cur.executemany(sql, payload)

    def query(
        self,
        vector: list[float],
        top_k: int = 10,
        filter: dict | None = None,
        namespace: Optional[str] = None,
    ) -> list[dict]:
        ns = namespace or "default"
        vec = _vector_literal(vector)
        where = "namespace = %s"
        params: list[object] = [ns]

        if filter:
            where += " AND metadata @> (%s)::jsonb"
            params.append(json.dumps(filter))

        sql = (
            "SELECT id, 1 - (embedding <=> (%s)::vector) AS score, metadata "
            "FROM vector_store_items "
            f"WHERE {where} "
            "ORDER BY embedding <=> (%s)::vector "
            "LIMIT %s"
        )
        params = [vec, *params, vec, max(1, top_k)]

        with psycopg.connect(self._dsn) as conn:
            with conn.cursor() as cur:
                cur.execute(sql, params)
                rows = cur.fetchall()

        out: list[dict] = []
        for row in rows:
            meta = row[2]
            if isinstance(meta, str):
                try:
                    meta = json.loads(meta)
                except Exception:
                    meta = {}
            out.append({"id": row[0], "score": float(row[1]) if row[1] is not None else None, "metadata": meta or {}})
        return out


def get_vector_store() -> VectorStore:
    backend = (settings.VECTOR_DB_BACKEND or "").lower()
    if not backend:
        if settings.DATABASE_URL and not settings.DATABASE_URL.startswith("sqlite"):
            backend = "pgvector"
        else:
            return NotConfiguredVectorStore()

    if backend == "pgvector":
        return PgvectorVectorStore()

    raise RuntimeError(f"Unknown vector backend: {backend}")
