from __future__ import annotations

from typing import Protocol, Optional

from app.core.config import settings


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


def get_vector_store() -> VectorStore:
    backend = (settings.VECTOR_DB_BACKEND or "").lower()
    if not backend:
        return NotConfiguredVectorStore()

    if backend == "pinecone":
        if not settings.PINECONE_API_KEY or not settings.PINECONE_INDEX or not settings.PINECONE_ENVIRONMENT:
            raise RuntimeError("Pinecone requires PINECONE_API_KEY, PINECONE_INDEX, and PINECONE_ENVIRONMENT")
        try:
            import pinecone  # type: ignore
        except Exception as e:
            raise RuntimeError("Pinecone client not installed") from e

        class PineconeVectorStore:
            def __init__(self):
                pinecone.init(
                    api_key=settings.PINECONE_API_KEY,
                    environment=settings.PINECONE_ENVIRONMENT,
                )
                self._index = pinecone.Index(settings.PINECONE_INDEX)

            def upsert(
                self,
                ids: list[str],
                vectors: list[list[float]],
                metadata: list[dict] | None = None,
                namespace: Optional[str] = None,
            ) -> None:
                items = []
                for i, vector in enumerate(vectors):
                    meta = metadata[i] if metadata and i < len(metadata) else None
                    if meta is None:
                        items.append((ids[i], vector))
                    else:
                        items.append((ids[i], vector, meta))
                self._index.upsert(items, namespace=namespace)

            def query(
                self,
                vector: list[float],
                top_k: int = 10,
                filter: dict | None = None,
                namespace: Optional[str] = None,
            ) -> list[dict]:
                res = self._index.query(
                    vector=vector,
                    top_k=top_k,
                    include_metadata=True,
                    filter=filter,
                    namespace=namespace,
                )
                matches = getattr(res, "matches", None) or res.get("matches", [])
                out = []
                for m in matches:
                    if isinstance(m, dict):
                        out.append(
                            {
                                "id": m.get("id"),
                                "score": m.get("score"),
                                "metadata": m.get("metadata") or {},
                            }
                        )
                    else:
                        out.append(
                            {
                                "id": getattr(m, "id", None),
                                "score": getattr(m, "score", None),
                                "metadata": getattr(m, "metadata", {}) or {},
                            }
                        )
                return out

        return PineconeVectorStore()

    if backend == "weaviate":
        if not settings.WEAVIATE_URL:
            raise RuntimeError("Weaviate requires WEAVIATE_URL")
        raise RuntimeError("Weaviate backend not wired yet")

    if backend == "pgvector":
        if not settings.PGVECTOR_DSN:
            raise RuntimeError("PGVector requires PGVECTOR_DSN")
        raise RuntimeError("PGVector backend not wired yet")

    raise RuntimeError(f"Unknown vector backend: {backend}")
