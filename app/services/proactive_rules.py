from __future__ import annotations

import json
import logging
from datetime import datetime, timedelta
from typing import Any, Dict, Optional

from sqlalchemy.orm import Session

from app.db.models import ProactiveRule, ProactiveRuleRun, NotificationQueue
from app.services.preferences import get_preferences, is_onboarding_complete
from app.services.proposals import create_proposal_with_link

logger = logging.getLogger(__name__)


def _parse_json(text: str | None, default: Any) -> Any:
    if not text:
        return default
    try:
        return json.loads(text)
    except Exception:
        return default


def _to_json(value: Any) -> str:
    return json.dumps(value or {}, ensure_ascii=False)


def _parse_dt(value: Optional[str]) -> Optional[datetime]:
    if not value:
        return None
    try:
        if value.endswith("Z"):
            value = value.replace("Z", "+00:00")
        return datetime.fromisoformat(value)
    except Exception:
        return None


def _compute_next_run(trigger_type: str, trigger_config: Dict[str, Any], last_run_at: Optional[datetime]) -> Optional[datetime]:
    now = datetime.utcnow()
    trigger_type = (trigger_type or "").lower()

    if trigger_type == "interval":
        interval = int(trigger_config.get("interval_minutes") or 60)
        if last_run_at:
            return last_run_at + timedelta(minutes=interval)
        start_at = _parse_dt(trigger_config.get("start_at"))
        if start_at and start_at > now:
            return start_at
        return now

    if trigger_type == "daily":
        hour = int(trigger_config.get("hour") or 9)
        minute = int(trigger_config.get("minute") or 0)
        candidate = now.replace(hour=hour, minute=minute, second=0, microsecond=0)
        if candidate <= now:
            candidate = candidate + timedelta(days=1)
        return candidate

    if trigger_type == "once":
        return _parse_dt(trigger_config.get("run_at"))

    return None


def create_rule(
    db: Session,
    user_id: str,
    name: str,
    trigger_type: str,
    trigger_config: Dict[str, Any],
    action_type: str,
    action_payload: Dict[str, Any],
    conditions: Optional[Dict[str, Any]] = None,
    is_active: bool = True,
) -> ProactiveRule:
    trigger_config = trigger_config or {}
    conditions = conditions or {}
    action_payload = action_payload or {}

    rule = ProactiveRule(
        user_id=user_id,
        name=name,
        is_active=is_active,
        trigger_type=trigger_type,
        trigger_config_json=_to_json(trigger_config),
        conditions_json=_to_json(conditions),
        action_type=action_type,
        action_payload_json=_to_json(action_payload),
    )
    rule.next_run_at = _compute_next_run(trigger_type, trigger_config, None)

    db.add(rule)
    db.commit()
    db.refresh(rule)
    return rule


def list_rules(db: Session, user_id: str) -> list[ProactiveRule]:
    return (
        db.query(ProactiveRule)
        .filter(ProactiveRule.user_id == user_id)
        .order_by(ProactiveRule.created_at.desc())
        .all()
    )


def get_rule(db: Session, rule_id: int, user_id: Optional[str] = None) -> Optional[ProactiveRule]:
    q = db.query(ProactiveRule).filter(ProactiveRule.id == rule_id)
    if user_id:
        q = q.filter(ProactiveRule.user_id == user_id)
    return q.one_or_none()


def update_rule(db: Session, rule: ProactiveRule, patch: Dict[str, Any]) -> ProactiveRule:
    if "name" in patch:
        rule.name = patch["name"]
    if "is_active" in patch:
        rule.is_active = bool(patch["is_active"])
    if "trigger_type" in patch:
        rule.trigger_type = patch["trigger_type"]
    if "trigger_config" in patch:
        rule.trigger_config_json = _to_json(patch["trigger_config"]) if patch["trigger_config"] is not None else None
    if "conditions" in patch:
        rule.conditions_json = _to_json(patch["conditions"]) if patch["conditions"] is not None else None
    if "action_type" in patch:
        rule.action_type = patch["action_type"]
    if "action_payload" in patch:
        rule.action_payload_json = _to_json(patch["action_payload"]) if patch["action_payload"] is not None else None

    trigger_config = _parse_json(rule.trigger_config_json, {})
    rule.next_run_at = _compute_next_run(rule.trigger_type, trigger_config, rule.last_run_at)

    db.commit()
    db.refresh(rule)
    return rule


def _log_run(db: Session, rule: ProactiveRule, status: str, reason: str | None = None) -> None:
    db.add(
        ProactiveRuleRun(
            rule_id=rule.id,
            user_id=rule.user_id,
            status=status,
            reason=reason,
        )
    )
    db.commit()


def _should_run(db: Session, rule: ProactiveRule) -> tuple[bool, str | None]:
    now = datetime.utcnow()
    if not rule.is_active:
        return False, "inactive"

    if rule.next_run_at and rule.next_run_at > now:
        return False, "not_due"

    conditions = _parse_json(rule.conditions_json, {})
    prefs = get_preferences(db, rule.user_id)

    if conditions.get("require_onboarding_complete") and not is_onboarding_complete(prefs):
        return False, "onboarding_incomplete"

    if conditions.get("require_phone_verified") and not prefs.get("phone_verified"):
        return False, "phone_not_verified"

    min_gap = conditions.get("min_gap_minutes")
    if min_gap and rule.last_run_at:
        gap = (now - rule.last_run_at).total_seconds() / 60
        if gap < float(min_gap):
            return False, "min_gap_not_met"

    return True, None


def _execute_action(db: Session, rule: ProactiveRule) -> dict:
    payload = _parse_json(rule.action_payload_json, {})

    if rule.action_type == "notify":
        title = payload.get("title") or rule.name
        message = payload.get("message") or ""
        deep_link_url = payload.get("deep_link_url")

        db.add(
            NotificationQueue(
                user_id=rule.user_id,
                watch_item_id=None,
                event_type="proactive",
                title=title,
                message=message,
                deep_link_url=deep_link_url,
                created_at=datetime.utcnow(),
            )
        )
        db.commit()
        return {"status": "notified"}

    if rule.action_type == "create_proposal":
        proposal_type = payload.get("proposal_type") or "generic"
        proposal_payload = payload.get("payload") or {}
        summary = payload.get("summary") or "Proposal created"
        created = create_proposal_with_link(
            db,
            user_id=rule.user_id,
            proposal_type=proposal_type,
            payload=proposal_payload,
        )
        db.add(
            NotificationQueue(
                user_id=rule.user_id,
                watch_item_id=None,
                event_type="proactive",
                title=summary,
                message=f"Approve here: {created['approval_url']}",
                deep_link_url=created["approval_url"],
                created_at=datetime.utcnow(),
            )
        )
        db.commit()
        return {"status": "proposal_created", "proposal_id": created.get("proposal_id")}

    if rule.action_type == "voice_call_proposal":
        proposal_payload = payload or {}
        created = create_proposal_with_link(
            db,
            user_id=rule.user_id,
            proposal_type="voice_call",
            payload=proposal_payload,
        )
        summary = payload.get("summary") or "Voice call ready for approval"
        db.add(
            NotificationQueue(
                user_id=rule.user_id,
                watch_item_id=None,
                event_type="proactive",
                title=summary,
                message=f"Approve the call: {created['approval_url']}",
                deep_link_url=created["approval_url"],
                created_at=datetime.utcnow(),
            )
        )
        db.commit()
        return {"status": "voice_call_proposal", "proposal_id": created.get("proposal_id")}

    raise ValueError(f"Unknown action_type: {rule.action_type}")


def run_rule(db: Session, rule: ProactiveRule, force: bool = False) -> dict:
    should_run, reason = _should_run(db, rule)
    if not should_run and not force:
        _log_run(db, rule, "skipped", reason)
        return {"status": "skipped", "reason": reason}

    try:
        result = _execute_action(db, rule)
        if "status" in result:
            result["action_status"] = result.pop("status")
        rule.last_run_at = datetime.utcnow()
        trigger_config = _parse_json(rule.trigger_config_json, {})
        rule.next_run_at = _compute_next_run(rule.trigger_type, trigger_config, rule.last_run_at)
        if rule.trigger_type == "once":
            rule.is_active = False
        db.commit()
        _log_run(db, rule, "ok", None)
        return {"status": "ok", **result}
    except Exception as exc:
        logger.error("Proactive rule %s failed: %s", rule.id, exc)
        _log_run(db, rule, "failed", str(exc))
        raise


def run_due_rules(db: Session) -> dict:
    now = datetime.utcnow()
    rules = (
        db.query(ProactiveRule)
        .filter(ProactiveRule.is_active == True)
        .filter(ProactiveRule.next_run_at.isnot(None))
        .filter(ProactiveRule.next_run_at <= now)
        .order_by(ProactiveRule.next_run_at.asc())
        .all()
    )

    ran = 0
    skipped = 0
    failed = 0

    for rule in rules:
        try:
            result = run_rule(db, rule)
            if result.get("status") == "skipped":
                skipped += 1
            else:
                ran += 1
        except Exception:
            failed += 1

    return {"ran": ran, "skipped": skipped, "failed": failed}
