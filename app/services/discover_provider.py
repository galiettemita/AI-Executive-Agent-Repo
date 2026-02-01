# implement the real provider call
""" Uses SerpAPI’s /search.json endpoint and passes an engine and query params.

Only returns metadata from search results (no HTML scraping). """
from __future__ import annotations

from typing import Any
import httpx

from app.core.config import settings
from app.schemas.discover import DiscoverResult
from urllib.parse import urlparse

BLOCKED_DOMAINS = {
    "youtube.com", "www.youtube.com",
    "reddit.com", "www.reddit.com",
    "wikipedia.org", "en.wikipedia.org",
    "pinterest.com", "www.pinterest.com",
    "tiktok.com", "www.tiktok.com",
    "instagram.com", "www.instagram.com",
    "facebook.com", "www.facebook.com",     "businessinsider.com", "www.businessinsider.com",
    "rtings.com", "www.rtings.com",
    "cnn.com", "www.cnn.com",
    "tomsguide.com", "www.tomsguide.com",
    "tech.yahoo.com", "www.tech.yahoo.com", "yahoo.com", "www.yahoo.com",
    "cnet.com", "www.cnet.com",
    "theverge.com", "www.theverge.com",
    "nytimes.com", "www.nytimes.com",
    "wsj.com", "www.wsj.com",
    "forbes.com", "www.forbes.com",
    "pcmag.com", "www.pcmag.com",

}

def get_domain(url: str) -> str:
    return (urlparse(url).netloc or "").lower()

def is_blocked_domain(domain: str) -> bool:
    # block exact match and subdomains of blocked sites
    for b in BLOCKED_DOMAINS:
        if domain == b or domain.endswith("." + b):
            return True
    return False




SERPAPI_ENDPOINT = "https://serpapi.com/search.json"

class DiscoverNotConfiguredError(RuntimeError):
    pass

async def discover_search(query: str, *, max_results: int = 8) -> list[DiscoverResult]:
    """
    Calls SerpAPI to get search results. We only return title/url/snippet.
    No page scraping, no checkout scraping.
    """
    if not settings.SERPAPI_API_KEY:
        raise DiscoverNotConfiguredError("SERPAPI_API_KEY is not set.")

    params = {
        "engine": settings.SERPAPI_ENGINE,  # e.g., "google"
        "q": query,
        "api_key": settings.SERPAPI_API_KEY,
        "gl": settings.SERPAPI_GL,
        "hl": settings.SERPAPI_HL,
        "num": min(max_results, 10),  # keep small; Serp responses vary
    }

    timeout = httpx.Timeout(15.0, connect=10.0)
    async with httpx.AsyncClient(timeout=timeout) as client:
        resp = await client.get(SERPAPI_ENDPOINT, params=params)
        resp.raise_for_status()
        data: dict[str, Any] = resp.json()

    # SerpAPI typically returns organic results under "organic_results"
    organic = data.get("organic_results", []) or []
    results: list[DiscoverResult] = []

    for r in organic[:max_results]:
        title = r.get("title") or ""
        url = r.get("link") or ""
        snippet = r.get("snippet")

        domain = get_domain(url)
        if not title or not url:
            continue

        if not domain or is_blocked_domain(domain):
            continue

        results.append(
            DiscoverResult(
                title=title,
                url=url,
                snippet=snippet,
                source="serpapi",
                retailer_domain=domain,
            )
        )


    return results
