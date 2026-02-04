# backend/app/services/intent.py

from enum import Enum


class Intent(str, Enum):
    SHOPPING = "shopping"
    TRAVEL = "travel"
    FOOD = "food"
    CREATIVE = "creative"
    WARDROBE = "wardrobe"
    ADMIN = "admin"        # email/calendar/tasks
    GENERAL = "general"


def classify_intent(text: str) -> Intent:
    t = (text or "").lower()

    # Admin
    if any(k in t for k in ["email", "inbox", "calendar", "schedule", "meeting", "reschedule", "appointment"]):
        return Intent.ADMIN

    # Food (check before shopping to catch "order pizza" etc.)
    if any(k in t for k in ["doordash", "ubereats", "grubhub", "food", "pizza", "sushi", "burger", "pickup", "delivery", "restaurant", "hungry", "eat", "meal", "lunch", "dinner", "breakfast"]):
        return Intent.FOOD

    # Shopping
    if any(k in t for k in ["buy", "purchase", "order", "cart", "price", "deal", "track", "watch", "amazon", "walmart"]):
        return Intent.SHOPPING

    # Travel
    if any(k in t for k in ["flight", "hotel", "airline", "itinerary", "trip", "rent a car", "boarding"]):
        return Intent.TRAVEL

    # Creative / design
    if any(k in t for k in ["design", "logo", "flyer", "brand", "caption", "copy", "instagram", "ad", "poster", "website"]):
        return Intent.CREATIVE

    # Wardrobe
    if any(k in t for k in ["outfit", "wardrobe", "what to wear", "what should i wear", "wear to", "style", "shoes", "jacket", "capsule wardrobe", "dress", "clothing"]):
        return Intent.WARDROBE

    return Intent.GENERAL