# app/services/gift_recommendations.py

from __future__ import annotations

from typing import Any, Dict, List, Optional

from app.services.discover_provider import discover_search, DiscoverNotConfiguredError
from app.services.gift_service import serialize_occasion
from app.db.models import GiftOccasion


async def get_gift_recommendations(
    occasion: Optional[GiftOccasion],
    query: Optional[str],
    max_results: int = 6,
) -> Dict[str, Any]:
    if not query and occasion:
        name = occasion.recipient_name
        occasion_type = occasion.occasion_type or "gift"
        relation = occasion.relationship or ""
        query = f"{occasion_type} gift for {relation} {name}".strip()

    if not query:
        query = "gift ideas"

    results = await discover_search(query, max_results=max_results)
    return {
        "query": query,
        "occasion": serialize_occasion(occasion) if occasion else None,
        "results": [r.model_dump() for r in results],
    }
