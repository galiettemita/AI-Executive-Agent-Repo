# backend/app/services/orchestrator.py

from typing import Dict, List
from sqlalchemy.orm import Session
from app.services.admin_handler import handle_admin
from app.services.intent import classify_intent, Intent
from app.services.memory import get_user_memory
from app.services.preferences import (
    get_preferences,
    handle_onboarding_step,
    handle_wardrobe_onboarding_step,
    is_onboarding_complete,
    is_wardrobe_onboarding_complete,
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
from app.services.wardrobe_agent import run_wardrobe_agent
from app.services.creative_agent import run_creative_agent
from app.services.food_agent import run_food_agent
from app.services.travel_agent import run_travel_agent


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

    # Check if wardrobe onboarding is in progress (step started but not complete)
    if prefs.get("wardrobe_onboarding_step") and not is_wardrobe_onboarding_complete(prefs):
        reply, updated = handle_wardrobe_onboarding_step(user_message, prefs)
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

    # Creative mode (design, content, styling)
    if intent == Intent.CREATIVE:
        out = run_creative_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message, preferences=prefs)
        if isinstance(out, dict):
            return out.get("assistant_message", "")
        return str(out)

    # Wardrobe mode (outfit suggestions, style advice, shopping lists)
    if intent == Intent.WARDROBE:
        # Check if wardrobe onboarding is complete
        if not is_wardrobe_onboarding_complete(prefs):
            reply, updated = handle_wardrobe_onboarding_step(user_message, prefs)
            if reply:
                update_preferences(db, user_id, updated)
                return reply

        # Run wardrobe agent
        out = run_wardrobe_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message, preferences=prefs)
        if isinstance(out, dict):
            if "proposal" in out:
                proposal = out.get("proposal") or {}
                created = create_proposal_with_link(
                    db,
                    user_id=user_id,
                    proposal_type=proposal.get("type", "wardrobe_shopping_list"),
                    payload=proposal.get("payload", {}),
                )
                summary = proposal.get("summary", "I've created a wardrobe shopping list for you.")
                return f"{summary}\n\nApprove: {created['approval_url']}"
            return out.get("assistant_message", "")
        return str(out)

    # Travel mode (flights, hotels, itineraries)
    if intent == Intent.TRAVEL:
        out = run_travel_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message, preferences=prefs)
        if isinstance(out, dict):
            if "proposal" in out:
                proposal = out.get("proposal") or {}
                created = create_proposal_with_link(
                    db,
                    user_id=user_id,
                    proposal_type=proposal.get("type", "travel_itinerary"),
                    payload=proposal.get("payload", {}),
                )
                summary = proposal.get("summary", "I've created a travel itinerary for you.")
                return f"{summary}\n\nApprove: {created['approval_url']}"
            return out.get("assistant_message", "")
        return str(out)

    # Food mode (restaurant orders, delivery, pickup)
    if intent == Intent.FOOD:
        out = run_food_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message, preferences=prefs)
        if isinstance(out, dict):
            if "proposal" in out:
                proposal = out.get("proposal") or {}
                created = create_proposal_with_link(
                    db,
                    user_id=user_id,
                    proposal_type=proposal.get("type", "food_order"),
                    payload=proposal.get("payload", {}),
                )
                summary = proposal.get("summary", "I've created a food order for you.")
                return f"{summary}\n\nApprove: {created['approval_url']}"
            return out.get("assistant_message", "")
        return str(out)

    if intent == Intent.ADMIN:
        return handle_admin(db=db, user_id=user_id, history=injected_history, user_message=user_message)

    # Fallback
    return "Okay — tell me what you want to do, and any constraints (budget, timing, preferences)."
