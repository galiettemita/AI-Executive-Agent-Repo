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

    # Shopping
    if any(k in t for k in ["buy", "purchase", "order", "cart", "price", "deal", "track", "watch", "amazon", "walmart"]):
        return Intent.SHOPPING

    # Food
    if any(k in t for k in ["doordash", "ubereats", "grubhub", "food", "pizza", "pickup", "delivery", "restaurant"]):
        return Intent.FOOD

    # Travel
    if any(k in t for k in ["flight", "hotel", "airline", "itinerary", "trip", "rent a car", "boarding"]):
        return Intent.TRAVEL

    # Creative / design
    if any(k in t for k in ["design", "logo", "flyer", "brand", "caption", "copy", "instagram", "ad", "poster", "website"]):
        return Intent.CREATIVE

    # Wardrobe
    if any(k in t for k in ["outfit", "wardrobe", "what to wear", "style", "shoes", "jacket", "capsule wardrobe"]):
        return Intent.WARDROBE

    return Intent.GENERAL