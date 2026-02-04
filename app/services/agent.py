import os
import json
import asyncio
from typing import Any, Dict, List, Optional
from urllib.parse import urlparse
from openai import OpenAI
from sqlalchemy.orm import Session
from datetime import datetime, timedelta
from sqlalchemy import func
from app.db.models import WatchItem, WatchOffer, NotificationQueue
from app.services.discover_provider import discover_search

client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))


def _is_google_link(url: Optional[str]) -> bool:
    if not url:
        return False
    u = url.lower()
    return "google.com" in u or "googleusercontent.com" in u


def _direct_buy_link(url: Optional[str]) -> Optional[str]:
    # NO google fallback
    if not url or _is_google_link(url):
        return None
    return url


def _watch_list_payload(db: Session, user_id: str) -> Dict[str, Any]:
    items = (
        db.query(WatchItem)
        .filter(WatchItem.user_id == user_id)
        .order_by(WatchItem.id.desc())
        .all()
    )
    return {
        "items": [
        {
                "id": i.id,
                "dedupe_key": (i.product_key or i.upc or i.url or str(i.id)),
                "url": i.url,
                "title_hint": i.title_hint,
                "product_key": getattr(i, "product_key", None),
                "upc": getattr(i, "upc", None),
                "desired_price": i.desired_price,
                "currency": i.currency,
                "last_seen_price": i.last_seen_price,
                "best_price": i.best_price,
                "best_retailer": i.best_retailer,
                "best_offer_url": _direct_buy_link(i.best_offer_url),
                "best_title": getattr(i, "best_title", None),
                "best_description": getattr(i, "best_description", None),
                "best_rating": getattr(i, "best_rating", None),
                "best_reviews_count": getattr(i, "best_reviews_count", None),
                "best_condition": getattr(i, "best_condition", None),
                "best_seller_type": getattr(i, "best_seller_type", None),
                "last_checked_at": i.last_checked_at.isoformat() if i.last_checked_at else None,
            }
            for i in items
        ]
    }


def _offers_for_items_payload(db: Session, user_id: str, item_ids: List[int], limit_each: int = 5) -> Dict[str, Any]:
    out: Dict[str, Any] = {"offers_by_item_id": {}}
    for item_id in item_ids:
        offers = (
            db.query(WatchOffer)
            .filter(WatchOffer.user_id == user_id, WatchOffer.watch_item_id == item_id)
            .order_by(WatchOffer.fetched_at.desc())
            .limit(limit_each)
            .all()
        )
        out["offers_by_item_id"][str(item_id)] = [
            {
                "price": o.price,
                "currency": o.currency,
                "retailer": o.retailer,
                "offer_url": _direct_buy_link(o.offer_url),
                "title": o.title,
                "description": o.description,
                "rating": o.rating,
                "reviews_count": o.reviews_count,
                "condition": o.condition,
                "seller_type": o.seller_type,
                "fetched_at": o.fetched_at.isoformat() if o.fetched_at else None,
            }
            for o in offers
        ]
    return out


def _notifications_payload(db: Session, user_id: str) -> Dict[str, Any]:
    rows = (
        db.query(NotificationQueue)
        .filter(NotificationQueue.user_id == user_id, NotificationQueue.sent_at.is_(None))
        .order_by(NotificationQueue.created_at.asc())
        .all()
    )
    return {
        "items": [
            {
                "id": n.id,
                "event_type": n.event_type,
                "title": n.title,
                "message": n.message,
                "deep_link_url": n.deep_link_url,
                "prev_price": n.prev_price,
                "new_price": n.new_price,
                "currency": n.currency,
                "created_at": n.created_at.isoformat() if n.created_at else None,
            }
            for n in rows
        ]
    }


def get_cheapest_offer_today(db: Session, user_id: str) -> Optional[dict]:
    latest = (
        db.query(WatchOffer.fetched_at)
        .filter(WatchOffer.user_id == user_id)
        .order_by(WatchOffer.fetched_at.desc())
        .first()
    )
    if not latest or not latest[0]:
        return None

    latest_ts = latest[0]

    offer = (
        db.query(WatchOffer)
        .filter(WatchOffer.user_id == user_id, WatchOffer.fetched_at == latest_ts)
        .order_by(WatchOffer.price.asc())
        .first()
    )
    if not offer:
        return None

    return {
        "title": offer.title or "Unknown item",
        "price": offer.price,
        "currency": offer.currency,
        "retailer": offer.retailer,
        "offer_url": _direct_buy_link(offer.offer_url),
        "rating": offer.rating,
        "reviews_count": offer.reviews_count,
        "condition": offer.condition,
        "seller_type": offer.seller_type,
    }


def _should_use_discover(user_message: str) -> bool:
    m = (user_message or "").lower()
    triggers = (
        "discover", "alternatives", "similar", "instead", "other options", "compare",
        "track", "watch", "monitor", "alert me", "notify me", "price", "on sale", "deal"
    )
    return any(t in m for t in triggers)

def _is_track_intent(user_message: str) -> bool:
    m = (user_message or "").lower()
    triggers = ("track", "watch", "monitor", "alert me", "notify me", "on sale", "price drop", "price drops")
    return any(t in m for t in triggers)

def _pick_best_discover_url(discover_results: list[dict]) -> Optional[str]:
    # We do NOT invent URLs. We only use discover results.
    for r in discover_results or []:
        url = r.get("url")
        if isinstance(url, str) and url.strip():
            return url.strip()
    return None

def _append_track_directive(reply_text: str, *, title_hint: Optional[str], url: str) -> str:
    # Keep it machine-readable on a single line.
    payload = {
        "title_hint": title_hint,
        "url": url,
        "desired_price": None,
        "currency": "USD",
        "product_key": None,
        "upc": None,
    }
    return (reply_text.strip() + "\nTRACK_ITEM " + json.dumps(payload, ensure_ascii=False)).strip()


def _discover_payload(user_message: str, watchlist: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    # Prefer searching for alternatives to a tracked item, not the user's instruction text.
    # Use best_title if present, else title_hint, else url.
    _ = watchlist  # optional; keep for compatibility if unused
    items = (watchlist or {}).get("items") or []
    seed = None
    if items:
        first = items[0]
        seed = first.get("best_title") or first.get("title_hint") or first.get("url")

    q = (user_message or "").strip()
    q = f"{q} buy product page official"
    results = asyncio.run(discover_search(q, max_results=8))
    return {"results": [r.model_dump() for r in results]}

def get_price_change_from_history(db: Session, user_id: str, watch_item_id: int) -> Optional[dict]:
    """
    Uses watch_offers snapshots to compute price change.
    - latest snapshot: cheapest offer in the most recent fetched_at batch
    - previous snapshot: cheapest offer in the previous fetched_at batch
    Returns dict with prev/new/delta and timestamps, or None if not enough data.
    """

    latest_ts_row = (
        db.query(WatchOffer.fetched_at)
        .filter(WatchOffer.user_id == user_id, WatchOffer.watch_item_id == watch_item_id)
        .order_by(WatchOffer.fetched_at.desc())
        .first()
    )
    if not latest_ts_row or not latest_ts_row[0]:
        return None
    latest_ts = latest_ts_row[0]

    prev_ts_row = (
        db.query(WatchOffer.fetched_at)
        .filter(
            WatchOffer.user_id == user_id,
            WatchOffer.watch_item_id == watch_item_id,
            WatchOffer.fetched_at < latest_ts,
        )
        .order_by(WatchOffer.fetched_at.desc())
        .first()
    )
    if not prev_ts_row or not prev_ts_row[0]:
        return None
    prev_ts = prev_ts_row[0]

        # If timestamps are extremely close, treat as same refresh and don't report a change
    if (latest_ts - prev_ts).total_seconds() < 2:
        return None

    latest_best = (
        db.query(WatchOffer)
        .filter(
            WatchOffer.user_id == user_id,
            WatchOffer.watch_item_id == watch_item_id,
            WatchOffer.fetched_at == latest_ts,
        )
        .order_by(WatchOffer.price.asc())
        .first()
    )
    prev_best = (
        db.query(WatchOffer)
        .filter(
            WatchOffer.user_id == user_id,
            WatchOffer.watch_item_id == watch_item_id,
            WatchOffer.fetched_at == prev_ts,
        )
        .order_by(WatchOffer.price.asc())
        .first()
    )

    if not latest_best or not prev_best:
        return None

    prev_price = float(prev_best.price)
    new_price = float(latest_best.price)

    return {
        "watch_item_id": watch_item_id,
        "prev_price": prev_price,
        "new_price": new_price,
        "delta": new_price - prev_price,
        "currency": latest_best.currency or prev_best.currency or "USD",
        "prev_fetched_at": prev_ts.isoformat() if prev_ts else None,
        "new_fetched_at": latest_ts.isoformat() if latest_ts else None,
        "prev_retailer": prev_best.retailer,
        "new_retailer": latest_best.retailer,
        "prev_offer_url": _direct_buy_link(prev_best.offer_url) if "_direct_buy_link" in globals() else prev_best.offer_url,
        "new_offer_url": _direct_buy_link(latest_best.offer_url) if "_direct_buy_link" in globals() else latest_best.offer_url,
        "prev_title": prev_best.title,
        "new_title": latest_best.title,
    }


def _is_bad_track_url(url: str) -> bool:
    u = (url or "").strip().lower()
    if not u.startswith("http"):
        return True

    # block obvious non-product pages
    bad_substrings = (
        "/blog/",
        "/blogs/",
        "/news/",
        "/guide",
        "/guides/",
        "/review",
        "/reviews/",
        "price-history",
        "pricehistory",
        "history",
        "forum",
        "forums",
        "reddit.com",
        "wikipedia.org",
    )
    if any(b in u for b in bad_substrings):
        return True

    # if it has almost no path, it's probably a homepage → not useful for tracking
    p = urlparse(u)
    if not p.netloc:
        return True
    if p.path in ("", "/"):
        return True
    domain = (p.netloc or "").lower()

    # Block common price/affiliate/aggregator domains (not retailer product pages)
    blocked_domains = (
        "appleinsider.com",
        "prices.appleinsider.com",
        "pricewatchpro.com",
        "camelcamelcamel.com",
        "slickdeals.net",
        "dealnews.com",
        "9to5toys.com",
        "theverge.com",
        "cnet.com",
        "forbes.com",
        "nytimes.com",
        "wsj.com",
        "pcmag.com",
        "tomsguide.com",
        "rtings.com",
    )
    if any(domain == b or domain.endswith("." + b) for b in blocked_domains):
        return True

    return False


def _pick_best_discover_url(discover_results: list[dict]) -> Optional[str]:
    for r in discover_results or []:
        url = r.get("url")
        if isinstance(url, str) and url.strip():
            candidate = url.strip()
            if not _is_bad_track_url(candidate):
                return candidate
    return None


def _strip_track_lines(text: str) -> str:
    # Remove any TRACK_ITEM lines the model might produce
    lines = (text or "").splitlines()
    kept = []
    for line in lines:
        if line.strip().startswith("TRACK_ITEM "):
            continue
        kept.append(line)
    return "\n".join(kept).strip()

def _is_sale_tracking_question(user_message: str) -> bool:
    m = (user_message or "").lower()
    triggers = (
        "go on sale",
        "on sale",
        "sale alert",
        "notify me",
        "alert me",
        "watch for a drop",
        "track price",
        "price drop",
        "when it drops",
        "when it gets cheaper",
        "deal",
        "discount",
    )
    return any(t in m for t in triggers)

def run_agent(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
) -> Dict[str, Any]:
    watchlist = _watch_list_payload(db, user_id)
    item_ids = [it["id"] for it in watchlist.get("items", []) if "id" in it]
    offers = _offers_for_items_payload(db, user_id, item_ids, limit_each=5) if item_ids else {"offers_by_item_id": {}}
    notifications = _notifications_payload(db, user_id)
    cheapest_today = get_cheapest_offer_today(db, user_id)
   
    price_changes = {}
    for item_id in item_ids:
        pc = get_price_change_from_history(db, user_id, item_id)
        if pc:
            price_changes[str(item_id)] = pc
        else:
            price_changes[str(item_id)] = {"status": "insufficient_history"}



    discover = {"results": []}
    if _should_use_discover(user_message) or _is_track_intent(user_message):
        discover = _discover_payload(user_message)

    sale_tracking_mode = _is_sale_tracking_question(user_message)

    context = {
        "user_id": user_id,
        "watchlist": watchlist,
        "offers": offers,
        "pending_notifications": notifications,
        "discover": discover,
        "price_changes": price_changes,
        "sale_tracking_mode": sale_tracking_mode,
    }

    # Deterministic tracking: if the user asks to track something,
    # we will include a TRACK_ITEM directive ourselves using discover results.
    # No invented URLs.
    track_url = None
    if _is_track_intent(user_message):
        track_url = _pick_best_discover_url(discover.get("results") or [])


    system = (
        "You are a casual, fast, human-like shopping assistant inside a mobile app.\n"
        "Use the provided JSON context as the ONLY source of truth.\n"
        "DO NOT invent prices, retailers, drops, ratings, review counts, or links.\n"
        "Keep replies SHORT (1-4 sentences) unless the user explicitly asks for details.\n"
        "Do NOT summarize the user's watchlist or list items unless the user asks.\n"
        "\n"
        "TWO MODES:\n"
        "1. TRACKING MODE: User wants to watch/track/monitor prices → use TRACK_ITEM\n"
        "2. PURCHASE MODE: User wants to buy/purchase/order now → use CART_PROPOSAL\n"
        "\n"
        "TRACKING MODE:\n"
        "If the user asks to track something, ask only the minimum clarifying questions needed (model/size/color/retailer preference).\n"
        "For sale tracking: say you will check on refresh/app open (MVP) and alert when price drops or target is hit.\n"
        'TRACK_ITEM {"title_hint":"...","url":"...","desired_price":null,"currency":"USD","product_key":null,"upc":null}\n'
        "Only emit TRACK_ITEM if you have a real URL from context.discover results.\n"
        "\n"
        "PURCHASE MODE:\n"
        "If the user wants to BUY/PURCHASE items NOW, create a shopping cart proposal:\n"
        'CART_PROPOSAL {"items":[{"name":"item","url":"link","price":0.0,"quantity":1,"retailer":"store"}],"estimated_total":0.0,"notes":"any notes"}\n'
        "Only emit CART_PROPOSAL when user confirms they want to purchase.\n"
        "\n"
        "If a direct buy link is null, say 'Direct buy link unavailable' and do not provide a Google Shopping link.\n"
        "Never claim a price changed unless context.price_changes has prev_price and new_price with different timestamps.\n"
        "If those fields are missing or delta is not negative, you MUST say you cannot confirm a drop.\n"
        "\n"
        "Keep replies short and casual.\n"

    )

    if cheapest_today:
        system += (
            "\nCheapest offer in latest snapshot (if offer_url is null, say no direct link):\n"
            f"- title={cheapest_today.get('title')}\n"
            f"- price={cheapest_today.get('price')} {cheapest_today.get('currency')}\n"
            f"- retailer={cheapest_today.get('retailer')}\n"
            f"- offer_url={cheapest_today.get('offer_url')}\n"
        )

    input_messages = [{"role": "system", "content": system}]
    input_messages.extend(history[-20:])
    input_messages.append(
        {
            "role": "user",
            "content": "CONTEXT_JSON:\n" + json.dumps(context, ensure_ascii=False) + "\n\nUSER_MESSAGE:\n" + user_message,
        }
    )

    resp = client.responses.create(
    model=os.getenv("OPENAI_MODEL", "gpt-4.1"),
    input=input_messages,
    )

    reply = resp.output_text.strip()

    # Check if agent wants to create a shopping cart proposal
    if "CART_PROPOSAL " in reply:
        lines = reply.split("\n")
        proposal_line = None
        clean_lines = []

        for line in lines:
            if line.strip().startswith("CART_PROPOSAL "):
                proposal_line = line.strip()
            else:
                clean_lines.append(line)

        reply = "\n".join(clean_lines).strip()

        if proposal_line:
            try:
                proposal_json = proposal_line.replace("CART_PROPOSAL ", "")
                proposal_data = json.loads(proposal_json)

                return {
                    "proposal": {
                        "type": "purchase_cart",
                        "summary": reply or "I've created a shopping cart for you to review.",
                        "payload": proposal_data,
                    }
                }
            except json.JSONDecodeError:
                pass  # If parsing fails, just return the message

    # Handle tracking mode
    reply = _strip_track_lines(reply)

    if track_url:
        title_hint = None
        # Use best guess from discover titles if present
        try:
            first = (discover.get("results") or [])[0]
            if isinstance(first, dict):
                title_hint = first.get("title")
        except Exception:
            title_hint = None

        if not title_hint:
            title_hint = "Tracked item"

        reply = _append_track_directive(reply, title_hint=title_hint, url=track_url)

    return {"assistant_message": reply}
