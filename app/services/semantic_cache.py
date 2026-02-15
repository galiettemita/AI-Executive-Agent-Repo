from __future__ import annotations

import hashlib
import json
import logging
import uuid
from datetime import datetime, timedelta, timezone
from typing import Optional

import psycopg

from app.core.config import settings
from app.services.embeddings import embed_texts

logger = logging.getLogger(__name__)

_SEM_CACHE_NAMESPACE_PREFIX = "semantic_cache"


def _truthy(value: object) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    return str(value).strip().lower() in {"1", "true", "yes", "on"}


def _normalize_dsn() -> str:
    dsn = settings.PGVECTOR_DSN or settings.DATABASE_URL or ""
    return dsn.replace("postgresql+asyncpg://", "postgresql://", 1)


def _vector_literal(vector: list[float]) -> str:
    return "[" + ",".join(f"{v:.6f}" for v in vector) + "]"


def _context_hash(context: dict | None) -> str:
    payload = json.dumps(context or {}, sort_keys=True, ensure_ascii=False)
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def _namespace(user_id: str, model: str, tier: int) -> str:
    return f"{_SEM_CACHE_NAMESPACE_PREFIX}:{user_id}:{model}:{tier}"


def _table_exists(cur: psycopg.Cursor, table_name: str) -> bool:
    cur.execute("SELECT to_regclass(%s)", (table_name,))
    row = cur.fetchone()
    return bool(row and row[0])


def _is_uuid(value: str) -> bool:
    try:
        uuid.UUID(value)
        return True
    except Exception:
        return False


def _query_embedding(query_text: str) -> list[float]:
    return embed_texts([query_text])[0]


def get_cached_response(
    *,
    user_id: str,
    query_text: str,
    model: str,
    tier: int,
    context: dict | None = None,
) -> Optional[str]:
    if not _truthy(settings.SEMANTIC_CACHE_ENABLED):
        return None
    if len((query_text or "").strip()) < max(1, settings.SEMANTIC_CACHE_MIN_QUERY_CHARS):
        return None

    dsn = _normalize_dsn()
    if not dsn:
        return None

    try:
        embedding = _query_embedding(query_text)
        vec = _vector_literal(embedding)
        now = datetime.now(timezone.utc)
        min_score = float(settings.SEMANTIC_CACHE_MIN_SIMILARITY)
        c_hash = _context_hash(context)

        with psycopg.connect(dsn) as conn:
            with conn.cursor() as cur:
                # Preferred blueprint table.
                if _table_exists(cur, "semantic_cache") and _is_uuid(user_id):
                    cur.execute(
                        """
                        SELECT id, response, 1 - (query_embedding <=> (%s)::vector) AS score
                        FROM semantic_cache
                        WHERE user_id = %s::uuid
                          AND model = %s
                          AND tier = %s
                          AND expires_at > now()
                          AND ((response->>'context_hash') IS NULL OR response->>'context_hash' = %s)
                        ORDER BY query_embedding <=> (%s)::vector
                        LIMIT 1
                        """,
                        (vec, user_id, model, tier, c_hash, vec),
                    )
                    row = cur.fetchone()
                    if row:
                        cache_id, response_json, score = row
                        if score is not None and float(score) >= min_score:
                            cur.execute(
                                "UPDATE semantic_cache SET hit_count = hit_count + 1 WHERE id = %s",
                                (cache_id,),
                            )
                            text = (response_json or {}).get("assistant_message")
                            return text if isinstance(text, str) else None

                # Fallback table used by pgvector helpers.
                if _table_exists(cur, "vector_store_items"):
                    cur.execute(
                        """
                        SELECT id, metadata, 1 - (embedding <=> (%s)::vector) AS score
                        FROM vector_store_items
                        WHERE namespace = %s
                          AND metadata @> (%s)::jsonb
                        ORDER BY embedding <=> (%s)::vector
                        LIMIT 5
                        """,
                        (
                            vec,
                            _namespace(user_id, model, tier),
                            json.dumps({"context_hash": c_hash}),
                            vec,
                        ),
                    )
                    rows = cur.fetchall()
                    for cache_id, meta, score in rows:
                        try:
                            metadata = meta if isinstance(meta, dict) else json.loads(meta or "{}")
                        except Exception:
                            metadata = {}
                        expires_at = metadata.get("expires_at")
                        if not expires_at:
                            continue
                        try:
                            expiry = datetime.fromisoformat(str(expires_at).replace("Z", "+00:00"))
                        except ValueError:
                            continue
                        if expiry <= now:
                            continue
                        if score is None or float(score) < min_score:
                            continue

                        metadata["hit_count"] = int(metadata.get("hit_count", 0)) + 1
                        cur.execute(
                            """
                            UPDATE vector_store_items
                            SET metadata = (%s)::jsonb
                            WHERE id = %s AND namespace = %s
                            """,
                            (json.dumps(metadata), cache_id, _namespace(user_id, model, tier)),
                        )
                        text = metadata.get("assistant_message")
                        return text if isinstance(text, str) else None
    except Exception as exc:
        logger.warning("Semantic cache lookup failed: %s", exc)
    return None


def put_cached_response(
    *,
    user_id: str,
    query_text: str,
    assistant_message: str,
    model: str,
    tier: int,
    context: dict | None = None,
) -> None:
    if not _truthy(settings.SEMANTIC_CACHE_ENABLED):
        return
    if not query_text or not assistant_message:
        return

    dsn = _normalize_dsn()
    if not dsn:
        return

    try:
        embedding = _query_embedding(query_text)
        vec = _vector_literal(embedding)
        ttl_seconds = max(60, int(settings.SEMANTIC_CACHE_TTL_SECONDS))
        expires_at = datetime.now(timezone.utc) + timedelta(seconds=ttl_seconds)
        c_hash = _context_hash(context)
        cache_payload = {
            "assistant_message": assistant_message,
            "context_hash": c_hash,
            "query_text": query_text,
            "expires_at": expires_at.isoformat(),
            "hit_count": 0,
        }

        with psycopg.connect(dsn) as conn:
            with conn.cursor() as cur:
                if _table_exists(cur, "semantic_cache") and _is_uuid(user_id):
                    cur.execute(
                        """
                        INSERT INTO semantic_cache (user_id, query_embedding, query_text, response, model, tier, expires_at)
                        VALUES (%s::uuid, (%s)::vector, %s, (%s)::jsonb, %s, %s, %s)
                        """,
                        (
                            user_id,
                            vec,
                            query_text,
                            json.dumps(cache_payload),
                            model,
                            tier,
                            expires_at,
                        ),
                    )
                    return

                if _table_exists(cur, "vector_store_items"):
                    cache_id = hashlib.sha256(
                        f"{user_id}|{model}|{tier}|{c_hash}|{query_text}".encode("utf-8")
                    ).hexdigest()
                    cur.execute(
                        """
                        INSERT INTO vector_store_items (id, namespace, embedding, metadata)
                        VALUES (%s, %s, (%s)::vector, (%s)::jsonb)
                        ON CONFLICT (id, namespace) DO UPDATE
                          SET embedding = EXCLUDED.embedding,
                              metadata = EXCLUDED.metadata
                        """,
                        (
                            cache_id,
                            _namespace(user_id, model, tier),
                            vec,
                            json.dumps(cache_payload),
                        ),
                    )
    except Exception as exc:
        logger.warning("Semantic cache write failed: %s", exc)
