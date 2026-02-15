from __future__ import annotations

from typing import Any
import httpx

from app.core.config import settings


class TavilyNotConfiguredError(RuntimeError):
    pass


async def tavily_search(
    query: str,
    *,
    max_results: int = 5,
    search_depth: str | None = None,
    topic: str = "general",
    include_answer: bool = False,
    include_raw_content: bool = False,
) -> dict[str, Any]:
    if not settings.TAVILY_API_KEY:
        raise TavilyNotConfiguredError("TAVILY_API_KEY is not set.")

    payload = {
        "query": query,
        "max_results": max_results,
        "search_depth": search_depth or settings.TAVILY_SEARCH_DEPTH,
        "topic": topic,
        "include_answer": include_answer,
        "include_raw_content": include_raw_content,
    }

    headers = {
        "Authorization": f"Bearer {settings.TAVILY_API_KEY}",
        "Content-Type": "application/json",
    }

    timeout = httpx.Timeout(20.0, connect=10.0)
    async with httpx.AsyncClient(timeout=timeout) as client:
        resp = await client.post("https://api.tavily.com/search", json=payload, headers=headers)
        resp.raise_for_status()
        return resp.json()
