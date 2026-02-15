# implement the real provider call
"""Uses Tavily search API for LLM-optimized search results."""
from __future__ import annotations

from typing import Any

from app.schemas.discover import DiscoverResult
from app.services.tavily_client import tavily_search, TavilyNotConfiguredError
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

class DiscoverNotConfiguredError(RuntimeError):
    pass

async def discover_search(query: str, *, max_results: int = 8) -> list[DiscoverResult]:
    """
    Calls Tavily to get search results. We only return title/url/snippet.
    """
    try:
        data: dict[str, Any] = await tavily_search(query, max_results=min(max_results, 10))
    except TavilyNotConfiguredError as exc:
        raise DiscoverNotConfiguredError(str(exc)) from exc

    organic = data.get("results", []) or []
    results: list[DiscoverResult] = []

    for r in organic[:max_results]:
        title = r.get("title") or ""
        url = r.get("link") or ""
        if not url:
            url = r.get("url") or ""
        snippet = r.get("content") or r.get("snippet")

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
                source="tavily",
                retailer_domain=domain,
            )
        )


    return results
