# app/services/wardrobe_intelligence.py

from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, time, timezone
from typing import Any, Dict, List, Optional
from zoneinfo import ZoneInfo

from openai import OpenAI
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.calendar_router import list_events_in_range
from app.services.profile_service import get_profile
from app.services.preferences import get_preferences
from app.services.weather_service import WeatherError, parse_date, resolve_location, get_daily_weather
from app.services.wardrobe_service import list_wardrobe_items, serialize_item
from app.services.discover_provider import discover_search, DiscoverNotConfiguredError


_client = OpenAI(api_key=settings.OPENAI_API_KEY)


FORMAL_KEYWORDS = [
    "interview",
    "wedding",
    "gala",
    "ceremony",
    "presentation",
    "client",
    "board",
    "pitch",
    "investor",
]

WORKOUT_KEYWORDS = ["gym", "workout", "run", "yoga", "training", "spin", "pilates"]
TRAVEL_KEYWORDS = ["flight", "airport", "travel", "trip", "hotel"]
CASUAL_KEYWORDS = ["brunch", "coffee", "hangout", "party", "dinner", "date"]


@dataclass
class WardrobeContext:
    date: str
    timezone: str
    weather: Optional[Dict[str, Any]]
    events: List[Dict[str, Any]]
    event_tags: List[str]
    prefs: Dict[str, Any]


def build_context(
    db: Session,
    user_id: str,
    date_str: Optional[str] = None,
    location: Optional[str] = None,
    calendar_provider: Optional[str] = None,
) -> WardrobeContext:
    prefs = get_preferences(db, user_id)
    profile = get_profile(db, user_id) or {}

    tz_name = profile.get("timezone") or prefs.get("timezone") or "UTC"
    try:
        tz = ZoneInfo(tz_name)
    except Exception:
        tz = timezone.utc
        tz_name = "UTC"

    forecast_date = parse_date(date_str)
    weather = None
    try:
        loc = resolve_location(profile, override_location=location)
        weather = get_daily_weather(loc, forecast_date)
    except WeatherError:
        weather = None

    start = datetime.combine(forecast_date, time.min).replace(tzinfo=tz).astimezone(timezone.utc)
    end = datetime.combine(forecast_date, time.max).replace(tzinfo=tz).astimezone(timezone.utc)

    try:
        events = list_events_in_range(
            db=db,
            user_id=user_id,
            start_utc=start,
            end_utc=end,
            provider=calendar_provider,
            max_results=10,
        )
    except Exception:
        events = []

    event_tags = _infer_event_tags(events)

    return WardrobeContext(
        date=forecast_date.isoformat(),
        timezone=tz_name,
        weather=weather,
        events=events,
        event_tags=event_tags,
        prefs=prefs,
    )


def _infer_event_tags(events: List[Dict[str, Any]]) -> List[str]:
    tags: set[str] = set()
    for ev in events:
        summary = (ev.get("summary") or ev.get("title") or "").lower()
        if any(k in summary for k in FORMAL_KEYWORDS):
            tags.add("formal")
        if any(k in summary for k in WORKOUT_KEYWORDS):
            tags.add("workout")
        if any(k in summary for k in TRAVEL_KEYWORDS):
            tags.add("travel")
        if any(k in summary for k in CASUAL_KEYWORDS):
            tags.add("casual")
        if "meeting" in summary or "call" in summary:
            tags.add("business")
    if not tags:
        tags.add("casual")
    return sorted(tags)


def suggest_outfits(
    db: Session,
    user_id: str,
    context: WardrobeContext,
    max_suggestions: int = 3,
) -> List[Dict[str, Any]]:
    items = list_wardrobe_items(db, user_id, limit=200)
    if not items:
        return [
            {
                "title": "Add wardrobe items",
                "notes": "Upload a few wardrobe items to get tailored outfit suggestions.",
                "items": [],
            }
        ]

    if settings.WARDROBE_LLM_ENABLED == "1" and settings.OPENAI_API_KEY:
        suggestions = _llm_suggest_outfits(items, context, max_suggestions)
        if suggestions:
            return suggestions

    return _fallback_outfits(items, context, max_suggestions)


def _llm_suggest_outfits(
    items: List[Any],
    context: WardrobeContext,
    max_suggestions: int,
) -> List[Dict[str, Any]]:
    wardrobe_items = [
        {
            "id": i.id,
            "name": i.name,
            "category": i.category,
            "subcategory": i.subcategory,
            "color": i.color,
            "season": i.season,
            "tags": i.tags_json,
        }
        for i in items
    ]
    system = (
        "You are a wardrobe stylist. Use ONLY the provided wardrobe items to suggest outfits. "
        "Return strict JSON in this format: "
        '{"suggestions":[{"title":"string","notes":"string","items":[{"id":1,"name":"string"}]}]}.'
    )
    user = {
        "wardrobe_items": wardrobe_items,
        "weather": context.weather,
        "event_tags": context.event_tags,
        "events": context.events[:5],
        "preferences": context.prefs,
        "max_suggestions": max_suggestions,
    }
    try:
        resp = _client.responses.create(
            model=settings.OPENAI_MODEL,
            input=[
                {"role": "system", "content": system},
                {"role": "user", "content": json.dumps(user)},
            ],
            temperature=0.2,
        )
        raw = resp.output_text.strip()
        payload = json.loads(raw)
        suggestions = payload.get("suggestions") if isinstance(payload, dict) else None
        if not isinstance(suggestions, list):
            return []
        out = []
        for suggestion in suggestions[:max_suggestions]:
            if not isinstance(suggestion, dict):
                continue
            out.append(
                {
                    "title": suggestion.get("title") or "Outfit",
                    "notes": suggestion.get("notes"),
                    "items": suggestion.get("items") or [],
                }
            )
        return out
    except Exception:
        return []


def _fallback_outfits(items: List[Any], context: WardrobeContext, max_suggestions: int) -> List[Dict[str, Any]]:
    groups = {"tops": [], "bottoms": [], "outerwear": [], "shoes": [], "dresses": [], "active": []}
    for item in items:
        label = (item.category or "").lower()
        name = (item.name or "").lower()
        if "dress" in label or "dress" in name:
            groups["dresses"].append(item)
        elif any(k in label for k in ["jacket", "coat", "outerwear"]) or "jacket" in name or "coat" in name:
            groups["outerwear"].append(item)
        elif any(k in label for k in ["shoe", "footwear", "sneaker", "boot"]) or any(k in name for k in ["shoe", "sneaker", "boot"]):
            groups["shoes"].append(item)
        elif any(k in label for k in ["pant", "jean", "bottom", "skirt"]) or any(k in name for k in ["jean", "pant", "trouser", "skirt"]):
            groups["bottoms"].append(item)
        elif any(k in label for k in ["active", "sport", "gym"]) or any(k in name for k in ["active", "gym", "athletic"]):
            groups["active"].append(item)
        else:
            groups["tops"].append(item)

    suggestions: List[Dict[str, Any]] = []
    tags = set(context.event_tags)

    if "workout" in tags and groups["active"]:
        suggestions.append(
            {
                "title": "Workout-ready",
                "notes": "Lightweight activewear for your workout.",
                "items": [serialize_item(groups["active"][0])],
            }
        )

    if "formal" in tags and (groups["dresses"] or groups["outerwear"] or groups["tops"]):
        pick = groups["dresses"][:1] or groups["tops"][:1]
        suggestion_items = [serialize_item(i) for i in pick]
        if groups["outerwear"]:
            suggestion_items.append(serialize_item(groups["outerwear"][0]))
        suggestions.append(
            {
                "title": "Formal look",
                "notes": "Polished pieces for a formal event.",
                "items": suggestion_items,
            }
        )

    if not suggestions:
        base = []
        if groups["tops"]:
            base.append(serialize_item(groups["tops"][0]))
        if groups["bottoms"]:
            base.append(serialize_item(groups["bottoms"][0]))
        if groups["outerwear"]:
            base.append(serialize_item(groups["outerwear"][0]))
        suggestions.append(
            {
                "title": "Everyday outfit",
                "notes": "Balanced look using your staples.",
                "items": base,
            }
        )

    return suggestions[:max_suggestions]


async def shopping_recommendations(
    db: Session,
    user_id: str,
    context: WardrobeContext,
    max_results: Optional[int] = None,
) -> Dict[str, Any]:
    items = list_wardrobe_items(db, user_id, limit=200)
    queries = _build_shopping_queries(items, context)
    if not queries:
        return {"queries": [], "results": []}

    max_results = max_results or settings.WARDROBE_SHOPPING_MAX_RESULTS

    results = []
    for q in queries:
        try:
            found = await discover_search(q, max_results=max_results)
            results.append({"query": q, "results": [r.model_dump() for r in found]})
        except DiscoverNotConfiguredError as exc:
            raise exc
        except Exception:
            results.append({"query": q, "results": []})

    return {"queries": queries, "results": results}


def _build_shopping_queries(items: List[Any], context: WardrobeContext) -> List[str]:
    categories = {(i.category or "").lower() for i in items}
    tags = set(context.event_tags)
    prefs = context.prefs or {}

    vibe = prefs.get("wardrobe_vibe") or prefs.get("taste") or ""
    colors = prefs.get("wardrobe_colors") or ""
    budget = prefs.get("wardrobe_budget") or prefs.get("budget") or ""

    queries: List[str] = []

    def _decorate(query: str) -> str:
        parts = [query]
        if vibe:
            parts.append(vibe)
        if colors:
            parts.append(colors)
        if budget:
            parts.append(f"budget {budget}")
        return " ".join(parts)

    if "formal" in tags and "outerwear" not in categories:
        queries.append(_decorate("blazer or suit jacket"))
    if "workout" in tags and not any("active" in c or "sport" in c for c in categories):
        queries.append(_decorate("activewear set"))

    weather = context.weather or {}
    temp_min = weather.get("temp_c_min")
    temp_max = weather.get("temp_c_max")
    precip = weather.get("precip_mm")

    if isinstance(temp_min, (int, float)) and temp_min <= 5:
        queries.append(_decorate("warm coat"))
    if isinstance(precip, (int, float)) and precip >= 2:
        queries.append(_decorate("waterproof jacket"))
    if not queries:
        queries.append(_decorate("versatile everyday outfit"))
    return queries[:3]
