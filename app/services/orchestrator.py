# backend/app/services/orchestrator.py

from typing import Dict, List
from sqlalchemy.orm import Session
from app.services.admin_handler import handle_admin
from app.services.intent import classify_intent, Intent
from app.services.memory import get_user_memory
from app.services.preferences import (
    get_preferences,
    handle_onboarding_step,
    is_onboarding_complete,
    update_preferences,
)
from app.services.subscriptions import (
    get_entitlements,
    get_plan_limits,
    is_premium_user,
    limit_prompt,
    upgrade_prompt,
)
from app.services.usage import get_usage
from app.services.proposals import create_proposal_with_link
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
    prefs = get_preferences(db, user_id)
    if not is_onboarding_complete(prefs):
        reply, updated = handle_onboarding_step(user_message, prefs)
        if reply:
            update_preferences(db, user_id, updated)
            return reply

    entitlements = get_entitlements(db, user_id)
    usage = get_usage(db, user_id)
    limits = get_plan_limits(entitlements)
    intent = classify_intent(user_message)
    memory = get_user_memory(db, user_id)

    # Lightweight “context injection” for any downstream skill:
    # We prepend a synthetic system message into history.
    injected_history = history[:]
    if prefs:
        injected_history = [{"role": "system", "content": f"USER_PREFERENCES:\n{prefs}"}] + injected_history
    if entitlements:
        injected_history = [{"role": "system", "content": f"USER_ENTITLEMENTS:\n{entitlements}"}] + injected_history
    if memory:
        injected_history = [{"role": "system", "content": f"USER_MEMORY:\n{memory}"}] + injected_history

    # Usage limits (monthly)
    if usage.messages_count >= limits["messages"]:
        return limit_prompt(user_id)

    # Entitlements gating (default policy)
    premium_intents = {Intent.ADMIN, Intent.SHOPPING, Intent.FOOD, Intent.TRAVEL}
    if intent in premium_intents and not is_premium_user(entitlements):
        return upgrade_prompt(user_id)

    # ROUTING
    if intent == Intent.SHOPPING:
        # You already built a shopping/watchlist agent
        out = run_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message)
        if isinstance(out, dict):
            if "proposal" in out:
                proposal = out.get("proposal") or {}
                created = create_proposal_with_link(
                    db,
                    user_id=user_id,
                    proposal_type=proposal.get("type", "generic"),
                    payload=proposal.get("payload", {}),
                )
                summary = proposal.get("summary", "I created a proposal for you.")
                return f"{summary}\nApprove: {created['approval_url']}"
            return out.get("assistant_message", "")
        return str(out)

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
