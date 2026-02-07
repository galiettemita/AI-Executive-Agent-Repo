# backend/app/services/embeddings.py

from __future__ import annotations

from typing import List

from openai import OpenAI

from app.core.config import settings


client = OpenAI(api_key=settings.OPENAI_API_KEY)


def embed_texts(texts: List[str]) -> List[List[float]]:
    if not texts:
        return []
    model = getattr(settings, "OPENAI_EMBEDDING_MODEL", None) or "text-embedding-3-small"
    resp = client.embeddings.create(
        model=model,
        input=texts,
    )
    return [item.embedding for item in resp.data]
