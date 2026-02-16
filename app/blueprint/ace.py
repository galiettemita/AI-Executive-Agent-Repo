from __future__ import annotations

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


LOW_RISK_HINTS = ("summarize", "draft", "analyze", "explain", "outline", "plan")
MEDIUM_RISK_HINTS = ("send", "book", "schedule", "invite", "buy", "purchase")
HIGH_RISK_HINTS = ("pay", "wire", "transfer", "delete", "cancel", "publish")


def build_dual_memory_signals(db: Session, *, user_id: str) -> dict[str, Any]:
    """
    Pull lightweight ACE priors from both memory paths:
    - Episodic path: feedback_signals (recent approvals/corrections/outcomes)
    - Durable path: behavioral_rules + AGENTS.md directives
    """
    episodic_positive = 0
    episodic_negative = 0
    durable_strict_rules = 0
    durable_relaxed_rules = 0

    try:
        row = db.execute(
            text(
                """
                select
                  sum(case when signal_type in ('praise', 'approved', 'outcome_success') then 1 else 0 end) as pos_count,
                  sum(case when signal_type in ('correction', 'override', 'edit', 'complaint', 'outcome_failed') then 1 else 0 end) as neg_count
                from feedback_signals
                where user_id = :user_id
                """
            ),
            {"user_id": user_id},
        ).mappings().first()
        if row:
            episodic_positive = int(row.get("pos_count") or 0)
            episodic_negative = int(row.get("neg_count") or 0)
    except Exception:
        episodic_positive = 0
        episodic_negative = 0

    try:
        rows = db.execute(
            text(
                """
                select rule_key, rule_value
                from behavioral_rules
                where user_id = :user_id
                order by created_at desc
                limit 200
                """
            ),
            {"user_id": user_id},
        ).mappings().all()
        for row in rows:
            key = str(row.get("rule_key") or "").lower()
            value = str(row.get("rule_value") or "").lower()
            if any(k in key or k in value for k in ("always require approval", "never auto-send", "confirm first")):
                durable_strict_rules += 1
            if any(k in key or k in value for k in ("auto send", "allow autonomy", "skip approval")):
                durable_relaxed_rules += 1
    except Exception:
        pass

    return {
        "episodic_positive": episodic_positive,
        "episodic_negative": episodic_negative,
        "durable_strict_rules": durable_strict_rules,
        "durable_relaxed_rules": durable_relaxed_rules,
    }


def _base_risk_from_text(text: str) -> tuple[str, float]:
    t = (text or "").lower()
    risk_level = "none"
    autonomy_score = 0.15

    if any(k in t for k in LOW_RISK_HINTS):
        risk_level = "low"
        autonomy_score = 0.35
    if any(k in t for k in MEDIUM_RISK_HINTS):
        risk_level = "medium"
        autonomy_score = 0.62
    if any(k in t for k in HIGH_RISK_HINTS):
        risk_level = "high"
        autonomy_score = 0.9

    if any(k in t for k in ("urgent", "asap", "immediately")):
        autonomy_score = min(1.0, autonomy_score + 0.08)

    return risk_level, autonomy_score


def _derive_beta_prior(
    *,
    agents_content: str,
    dual_memory_signals: dict[str, Any] | None,
) -> tuple[float, float]:
    # alpha = autonomy confidence, beta = approval/guardrail confidence
    alpha = 2.0
    beta = 2.0

    signals = dual_memory_signals or {}
    alpha += float(signals.get("episodic_positive") or 0) * 0.35
    beta += float(signals.get("episodic_negative") or 0) * 0.65
    beta += float(signals.get("durable_strict_rules") or 0) * 0.45
    alpha += float(signals.get("durable_relaxed_rules") or 0) * 0.25

    agents = (agents_content or "").lower()
    if agents:
        if "always require approval" in agents:
            beta += 2.0
        if "never auto-send email" in agents:
            beta += 1.8
        if "default to draft/preview" in agents:
            beta += 1.0
        if "allow autonomous execution" in agents:
            alpha += 1.2

    return max(0.1, alpha), max(0.1, beta)


def classify_action(
    text: str,
    *,
    agents_content: str | None = None,
    dual_memory_signals: dict[str, Any] | None = None,
) -> dict[str, Any]:
    risk_level, autonomy_score = _base_risk_from_text(text)
    alpha, beta = _derive_beta_prior(
        agents_content=agents_content or "",
        dual_memory_signals=dual_memory_signals,
    )
    approval_probability = beta / (alpha + beta)

    agents = (agents_content or "").lower()
    if agents:
        if "always require approval" in agents and risk_level in {"low", "medium", "high"}:
            autonomy_score = max(autonomy_score, 0.7)
            risk_level = "medium" if risk_level == "low" else risk_level
        if "never auto-send email" in agents and "send" in (text or "").lower():
            risk_level = "high"
            autonomy_score = max(autonomy_score, 0.85)

    requires_approval = risk_level in {"medium", "high"} or approval_probability >= 0.55

    action_type = "informational"
    if risk_level == "medium":
        action_type = "transactional"
    elif risk_level == "high":
        action_type = "high_stakes"

    return {
        "action_type": action_type,
        "risk_level": risk_level,
        "autonomy_score": round(autonomy_score, 3),
        "requires_approval": requires_approval,
        "approval_probability": round(approval_probability, 3),
        "beta_prior_alpha": round(alpha, 3),
        "beta_prior_beta": round(beta, 3),
        "dual_memory_signals": dual_memory_signals or {},
    }

