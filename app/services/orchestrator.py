# backend/app/services/orchestrator.py

import logging
import time
from contextlib import nullcontext
from typing import Any, Dict, List
from sqlalchemy.orm import Session
from app.services.admin_handler import handle_admin
from app.services.intent import classify_intent, Intent
from app.services.memory import get_user_memory_context
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
from app.services.smart_home_agent import run_smart_home_agent
from app.core.config import settings

logger = logging.getLogger(__name__)


def _infer_tier(intent: Intent, user_message: str) -> int:
    text = (user_message or "").lower()

    # T3: explicit complex planning language.
    if any(
        token in text
        for token in (
            "multi-step",
            "step by step",
            "plan this out",
            "strategy",
            "roadmap",
            "analyze deeply",
        )
    ):
        return 3

    # T2: higher-complexity verticals in the current implementation.
    if intent in {Intent.ADMIN, Intent.TRAVEL, Intent.SMART_HOME}:
        return 2

    # T0: very short general prompts.
    if intent == Intent.GENERAL and len(text.split()) <= 6:
        return 0

    # T1: default.
    return 1


def infer_tier_for_message(user_message: str) -> tuple[Intent, int]:
    intent = classify_intent(user_message)
    return intent, _infer_tier(intent, user_message)


def _set_span_attr(span: Any, key: str, value: Any) -> None:
    if span is None or value is None:
        return
    try:
        span.set_attribute(key, value)
    except Exception:
        # Never let telemetry break runtime behavior.
        return


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
    intent, tier = infer_tier_for_message(user_message)

    tracer_cm = nullcontext(None)
    otel_status = None
    otel_status_code = None
    if settings.OTEL_ENABLED:
        try:
            from opentelemetry import trace
            from opentelemetry.trace import Status, StatusCode

            tracer_cm = trace.get_tracer(__name__).start_as_current_span("orchestrator.run")
            otel_status = Status
            otel_status_code = StatusCode
        except Exception:
            tracer_cm = nullcontext(None)

    started_at = time.perf_counter()
    outcome = "success"

    with tracer_cm as span:
        _set_span_attr(span, "exec.intent", intent.value)
        _set_span_attr(span, "exec.tier", tier)
        _set_span_attr(span, "exec.user_id", user_id)
        _set_span_attr(span, "exec.message_length", len(user_message or ""))

        try:
            prefs = get_preferences(db, user_id)
            if settings.REQUIRE_PHONE_VERIFICATION == "1" and not prefs.get("phone_verified"):
                outcome = "blocked_phone_verification"
                return (
                    "Please verify your phone number to continue. "
                    "Use POST /onboarding/phone/start to request a code, then /onboarding/phone/verify to confirm."
                )
            if not is_onboarding_complete(prefs):
                reply, updated = handle_onboarding_step(user_message, prefs)
                if reply:
                    update_preferences(db, user_id, updated)
                    outcome = "onboarding_step"
                    return reply

            # Check if wardrobe onboarding is in progress (step started but not complete)
            if prefs.get("wardrobe_onboarding_step") and not is_wardrobe_onboarding_complete(prefs):
                reply, updated = handle_wardrobe_onboarding_step(user_message, prefs)
                if reply:
                    update_preferences(db, user_id, updated)
                    outcome = "wardrobe_onboarding_step"
                    return reply

            entitlements = get_entitlements(db, user_id)
            usage = get_usage(db, user_id)
            limits = get_plan_limits(entitlements)
            memory = get_user_memory_context(db, user_id, user_message)

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
                outcome = "blocked_usage_limit"
                return limit_prompt(user_id)

            # Entitlements gating (default policy)
            premium_intents = {Intent.ADMIN, Intent.SHOPPING, Intent.FOOD, Intent.TRAVEL}
            if intent in premium_intents and not is_premium_user(entitlements):
                outcome = "blocked_upgrade_required"
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
                        outcome = "shopping_proposal"
                        return f"{summary}\nApprove: {created['approval_url']}"
                    outcome = "shopping_response"
                    return out.get("assistant_message", "")
                outcome = "shopping_response"
                return str(out)

            # Creative mode (design, content, styling)
            if intent == Intent.CREATIVE:
                out = run_creative_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message, preferences=prefs)
                if isinstance(out, dict):
                    outcome = "creative_response"
                    return out.get("assistant_message", "")
                outcome = "creative_response"
                return str(out)

            # Wardrobe mode (outfit suggestions, style advice, shopping lists)
            if intent == Intent.WARDROBE:
                # Check if wardrobe onboarding is complete
                if not is_wardrobe_onboarding_complete(prefs):
                    reply, updated = handle_wardrobe_onboarding_step(user_message, prefs)
                    if reply:
                        update_preferences(db, user_id, updated)
                        outcome = "wardrobe_onboarding_step"
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
                        outcome = "wardrobe_proposal"
                        return f"{summary}\n\nApprove: {created['approval_url']}"
                    outcome = "wardrobe_response"
                    return out.get("assistant_message", "")
                outcome = "wardrobe_response"
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
                        outcome = "travel_proposal"
                        return f"{summary}\n\nApprove: {created['approval_url']}"
                    outcome = "travel_response"
                    return out.get("assistant_message", "")
                outcome = "travel_response"
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
                        outcome = "food_proposal"
                        return f"{summary}\n\nApprove: {created['approval_url']}"
                    outcome = "food_response"
                    return out.get("assistant_message", "")
                outcome = "food_response"
                return str(out)

            if intent == Intent.ADMIN:
                outcome = "admin_response"
                return handle_admin(db=db, user_id=user_id, history=injected_history, user_message=user_message)

            if intent == Intent.SMART_HOME:
                if settings.ENABLE_SMART_HOME != "1":
                    outcome = "smart_home_disabled"
                    return "Smart home integration is currently disabled."
                outcome = "smart_home_response"
                return run_smart_home_agent(db=db, user_id=user_id, history=injected_history, user_message=user_message)

            # Fallback
            outcome = "fallback_response"
            return "Okay — tell me what you want to do, and any constraints (budget, timing, preferences)."
        except Exception as exc:
            outcome = "error"
            _set_span_attr(span, "exec.error_type", exc.__class__.__name__)
            if span is not None and otel_status is not None and otel_status_code is not None:
                try:
                    span.record_exception(exc)
                    span.set_status(otel_status(otel_status_code.ERROR, str(exc)))
                except Exception:
                    pass
            raise
        finally:
            latency_ms = (time.perf_counter() - started_at) * 1000
            _set_span_attr(span, "exec.outcome", outcome)
            _set_span_attr(span, "exec.latency_ms", round(latency_ms, 2))
            logger.info(
                "orchestrator_run intent=%s tier=%s outcome=%s latency_ms=%.2f",
                intent.value,
                tier,
                outcome,
                latency_ms,
            )
