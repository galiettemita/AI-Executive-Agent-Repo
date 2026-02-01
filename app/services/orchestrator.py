# backend/app/services/orchestrator.py

from typing import Dict, List
from sqlalchemy.orm import Session
from app.services.admin_handler import handle_admin
from app.services.intent import classify_intent, Intent
from app.services.memory import get_user_memory
from app.services.agent import run_agent  # your existing shopping-focused agent


def run_orchestrator(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
) -> str:
    """
    Single entry point for the assistant.
    Routes the request to the correct skill based on intent.
    Adds user memory to the context (without replaying entire chat).
    """
    intent = classify_intent(user_message)
    memory = get_user_memory(db, user_id)

    # Lightweight “context injection” for any downstream skill:
    # We prepend a synthetic system message into history.
    injected_history = history[:]
    if memory:
        injected_history = [{"role": "system", "content": f"USER_MEMORY:\n{memory}"}] + injected_history

    # ROUTING
    if intent == Intent.SHOPPING:
        # You already built a shopping/watchlist agent
        return run_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message)

    # Placeholder handlers for other intents (you’ll expand these)
    if intent == Intent.CREATIVE:
        return (
            "Got it — tell me:\n"
            "1) What are we making (logo/flyer/post/etc.)?\n"
            "2) Who is it for?\n"
            "3) What vibe (premium/playful/minimal)?\n"
            "4) Any text that must be included?"
        )

    if intent == Intent.WARDROBE:
        return (
            "I can help. What’s the occasion, your vibe (classic/street/minimal), "
            "and what’s the weather like? Any colors you want to avoid?"
        )

    if intent == Intent.TRAVEL:
        return (
            "Travel mode: Where are you going, what dates, and what matters most "
            "(price, nonstop, airline, baggage, time of day)?"
        )

    if intent == Intent.FOOD:
        return (
            "Food mode: pickup or delivery, budget, dietary preferences, and what are you craving?"
        )

    if intent == Intent.ADMIN:
        return handle_admin(db=db, user_id=user_id, history=injected_history, user_message=user_message)

    # Fallback
    return "Okay — tell me what you want to do, and any constraints (budget, timing, preferences)."