from __future__ import annotations

from dataclasses import dataclass
from typing import Callable

from app.blueprint.ace import classify_action
from app.blueprint.brain.tier_router import route_tier


@dataclass(frozen=True)
class GoldenScenario:
    scenario_id: str
    prompt: str
    expected_tier: int
    expected_action_type: str
    personalization_anchor: str


def default_golden_scenarios() -> list[GoldenScenario]:
    templates = [
        ("say hi", 0, "informational", "name"),
        ("thanks for the help", 0, "informational", "tone"),
        ("summarize my meetings for today", 1, "informational", "calendar"),
        ("draft a professional follow-up email to my manager", 1, "informational", "email"),
        ("find free slots this week and propose three options", 2, "informational", "calendar"),
        ("search web for latest AI chip export rules", 2, "informational", "research"),
        ("create a task list from my meeting notes", 2, "informational", "tasks"),
        ("find flights and then draft an email update", 3, "transactional", "workflow"),
        ("compare hotel options, then build an itinerary and checklist", 3, "transactional", "travel"),
        ("summarize spend trends and suggest budget actions", 3, "transactional", "finance"),
    ]
    scenarios: list[GoldenScenario] = []
    for idx in range(1, 121):
        prompt, tier, action_type, anchor = templates[(idx - 1) % len(templates)]
        scenarios.append(
            GoldenScenario(
                scenario_id=f"golden_{idx:03d}",
                prompt=f"{prompt} #{idx}",
                expected_tier=tier,
                expected_action_type=action_type,
                personalization_anchor=anchor,
            )
        )
    return scenarios


def personalization_score(*, reply: str, expected_anchor: str) -> float:
    text = (reply or "").lower()
    if not text:
        return 0.0
    anchor = (expected_anchor or "").strip().lower()
    if anchor and anchor in text:
        return 1.0
    if "you" in text or "your" in text:
        return 0.7
    if any(k in text for k in ("next", "plan", "step", "action")):
        return 0.5
    return 0.25


def _default_reply_generator(prompt: str) -> str:
    return f"For your request, next action is to confirm details and execute safely. ({prompt[:24]})"


def run_agentic_eval(
    *,
    reply_generator: Callable[[str], str] | None = None,
) -> dict[str, float | int]:
    scenarios = default_golden_scenarios()
    if not scenarios:
        return {
            "scenario_count": 0,
            "tier_accuracy": 0.0,
            "action_accuracy": 0.0,
            "personalization_avg": 0.0,
            "overall_score": 0.0,
        }

    generate_reply = reply_generator or _default_reply_generator

    tier_correct = 0
    action_correct = 0
    personalization_total = 0.0

    for scenario in scenarios:
        if route_tier(scenario.prompt) == scenario.expected_tier:
            tier_correct += 1

        ace = classify_action(scenario.prompt)
        if str(ace.get("action_type") or "") == scenario.expected_action_type:
            action_correct += 1

        reply = generate_reply(scenario.prompt)
        personalization_total += personalization_score(
            reply=reply,
            expected_anchor=scenario.personalization_anchor,
        )

    total = float(len(scenarios))
    tier_accuracy = round(tier_correct / total, 4)
    action_accuracy = round(action_correct / total, 4)
    personalization_avg = round(personalization_total / total, 4)

    overall = round((tier_accuracy * 0.4) + (action_accuracy * 0.25) + (personalization_avg * 0.35), 4)
    return {
        "scenario_count": len(scenarios),
        "tier_accuracy": tier_accuracy,
        "action_accuracy": action_accuracy,
        "personalization_avg": personalization_avg,
        "overall_score": overall,
    }


def run_golden_route_eval() -> dict[str, float]:
    result = run_agentic_eval()
    return {
        "accuracy": float(result.get("tier_accuracy") or 0.0),
        "overall_score": float(result.get("overall_score") or 0.0),
    }
