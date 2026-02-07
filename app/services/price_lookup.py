import re
import httpx
from typing import Optional
from app.core.config import settings

SERPAPI_GL = settings.SERPAPI_GL
SERPAPI_HL = settings.SERPAPI_HL
SERPAPI_API_KEY = settings.SERPAPI_API_KEY


class PriceLookupError(Exception):
    pass


def _parse_price(value: str) -> Optional[float]:
    """
    Parses strings like '$199.99', 'From $129', '129.00'
    """
    if not value:
        return None
    match = re.search(r"(\d+(?:\.\d{1,2})?)", value.replace(",", ""))
    if not match:
        return None
    return float(match.group(1))


def _normalize_key(s: str) -> str:
    s = s.strip().lower()
    s = re.sub(r"\s+", " ", s)
    s = re.sub(r"[^a-z0-9 :\-]", "", s)
    return s


def _looks_like_upc(s: str) -> bool:
    s = s.strip()
    return bool(re.fullmatch(r"\d{12,14}", s))


def _to_float(x) -> Optional[float]:
    try:
        if x is None:
            return None
        return float(x)
    except Exception:
        return None


def _to_int(x) -> Optional[int]:
    try:
        if x is None:
            return None
        return int(float(x))
    except Exception:
        return None


def _is_google_url(url: str) -> bool:
    u = url.lower()
    return ("google.com" in u) or ("googleusercontent.com" in u)


def _pick_offer_url(item: dict) -> Optional[str]:
    """
    Prefer direct merchant/product links.
    Reject anything that looks like a Google redirect / Google Shopping page.
    """
    url = (
        item.get("merchant_link")
        or item.get("merchant_url")
        or item.get("product_link")
        or item.get("tracking_link")
        or item.get("link")
        or item.get("shopping_result_link")
    )

    if not isinstance(url, str) or not url.strip():
        return None

    # Hard reject google URLs (you said: NO google fallback)
    if _is_google_url(url):
        return None

    # Also reject the classic shopping redirect format if it slips through
    if "ibp=oshop" in url:
        return None

    return url


def _extract_retailer(item: dict) -> str:
    return (
        item.get("source")
        or item.get("seller")
        or item.get("merchant")
        or item.get("store")
        or ""
    )


def _extract_condition(item: dict) -> Optional[str]:
    c = item.get("condition") or item.get("item_condition")
    if isinstance(c, str):
        c = c.strip()
        return c or None
    return None


def _extract_description(item: dict) -> Optional[str]:
    d = item.get("snippet") or item.get("description")
    if isinstance(d, str):
        d = d.strip()
        return d or None
    return None


def _extract_title(item: dict) -> Optional[str]:
    t = item.get("title") or item.get("name")
    if isinstance(t, str):
        t = t.strip()
        return t or None
    return None


def _is_blocked_retailer(retailer_lower: str) -> bool:
    blocked = (
        "ebay",
        "mercari",
        "poshmark",
        "depop",
        "facebook marketplace",
        "offerup",
        "aliexpress",
        "temu",
        "shein",
    )
    return any(b in retailer_lower for b in blocked)


def _seller_type_from(retailer_lower: str, condition: Optional[str]) -> str:
    if condition and any(x in condition.lower() for x in ("used", "pre-owned", "refurb", "restored")):
        return "marketplace"
    if any(x in retailer_lower for x in ("marketplace", "reseller", "used", "pre-owned")):
        return "marketplace"
    return "retailer"


def _score_offer(offer: dict) -> tuple:
    # prefer retailer + new, then lowest price
    price = offer.get("price") or 10**9
    seller_type = offer.get("seller_type") or "marketplace"
    condition = (offer.get("condition") or "").lower()

    is_retailer = 0 if seller_type == "retailer" else 1
    is_new = 0 if ("new" in condition or condition == "") else 1

    return (is_retailer, is_new, price)


async def lookup_price_google_shopping(query: str):
    """
    Returns:
      best_price: Optional[float]
      currency: str
      retailer: Optional[str]
      offer_url: Optional[str]
      best_meta: dict (product details)
      offers: list[dict] (top offers for history/AI)
    """
    if not SERPAPI_API_KEY:
        raise PriceLookupError("Missing SERPAPI_API_KEY")

    q_raw = query.strip()
    is_upc = _looks_like_upc(q_raw)
    product_key = f"upc:{q_raw}" if is_upc else f"q:{_normalize_key(q_raw)}"

    params = {
        "engine": "google_shopping",
        "q": q_raw,
        "hl": SERPAPI_HL,
        "gl": SERPAPI_GL,
        "api_key": SERPAPI_API_KEY,
    }

    async with httpx.AsyncClient(timeout=20) as client:
        r = await client.get("https://serpapi.com/search.json", params=params)
        if r.status_code != 200:
            raise PriceLookupError(f"SerpAPI HTTP {r.status_code}: {r.text}")
        data = r.json()

    shopping_results = data.get("shopping_results") or []

    offers: list[dict] = []
    for item in shopping_results:
        # price
        price = item.get("extracted_price")
        if isinstance(price, (int, float)):
            price_val = float(price)
        else:
            price_str = item.get("price")
            price_val = _parse_price(price_str) if isinstance(price_str, str) else None

        if price_val is None:
            continue

        # filter: obvious junk pricing
        if price_val < 20:
            continue

        retailer_val = _extract_retailer(item)
        retailer_lower = str(retailer_val).strip().lower()

        if not retailer_lower:
            retailer_val = None
            retailer_lower = ""


        # filter: blocked sellers (contains)
        if retailer_lower and _is_blocked_retailer(retailer_lower):
            continue


        # Prefer a DIRECT (non-google) link, but allow missing link for price tracking
        offer_url_val = _pick_offer_url(item)
        # If no direct link is available, keep the offer but set offer_url to None


        title = _extract_title(item)
        description = _extract_description(item)

        rating = _to_float(item.get("rating"))
        reviews_count = _to_int(item.get("reviews")) or _to_int(item.get("reviews_count"))

        condition = _extract_condition(item)
        seller_type = _seller_type_from(retailer_lower, condition)

        offers.append(
            {
                "price": price_val,
                "currency": "USD",
                "retailer": str(retailer_val) if retailer_val else None,
                "offer_url": offer_url_val,
                "product_key": product_key,
                "title": title,
                "description": description,
                "rating": rating,
                "reviews_count": reviews_count,
                "condition": condition,
                "seller_type": seller_type,
            }
        )

    # Choose best offer (not strictly cheapest; prefers retailer/new)
    best_price = None
    best_retailer = None
    best_offer_url = None
    best_meta = {
        "product_key": product_key,
        "title": None,
        "description": None,
        "rating": None,
        "reviews_count": None,
        "condition": None,
        "seller_type": None,
    }

    offers_sorted = sorted(offers, key=_score_offer)
    if offers_sorted:
        best = offers_sorted[0]
        best_price = best["price"]
        best_retailer = best["retailer"]
        best_offer_url = best["offer_url"]
        best_meta = {
            "product_key": best.get("product_key"),
            "title": best.get("title"),
            "description": best.get("description"),
            "rating": best.get("rating"),
            "reviews_count": best.get("reviews_count"),
            "condition": best.get("condition"),
            "seller_type": best.get("seller_type"),
        }

    return best_price, "USD", best_retailer, best_offer_url, best_meta, offers_sorted[:10]
