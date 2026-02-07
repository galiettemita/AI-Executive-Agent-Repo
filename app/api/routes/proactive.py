from __future__ import annotations

from typing import Any, Dict, Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.proactive_rules import create_rule, list_rules, get_rule, update_rule, run_rule

router = APIRouter(prefix="/proactive", tags=["proactive"])


class CreateRuleRequest(BaseModel):
    user_id: str
    name: str
    trigger_type: str
    trigger_config: Dict[str, Any] = {}
    action_type: str
    action_payload: Dict[str, Any] = {}
    conditions: Dict[str, Any] = {}
    is_active: bool = True


class UpdateRuleRequest(BaseModel):
    name: Optional[str] = None
    trigger_type: Optional[str] = None
    trigger_config: Optional[Dict[str, Any]] = None
    action_type: Optional[str] = None
    action_payload: Optional[Dict[str, Any]] = None
    conditions: Optional[Dict[str, Any]] = None
    is_active: Optional[bool] = None


class RunRuleRequest(BaseModel):
    force: bool = False


def _parse_json(text: str | None):
    if not text:
        return {}
    try:
        import json
        return json.loads(text)
    except Exception:
        return {}


def _serialize_rule(rule) -> dict:
    return {
        "id": rule.id,
        "user_id": rule.user_id,
        "name": rule.name,
        "is_active": rule.is_active,
        "trigger_type": rule.trigger_type,
        "trigger_config": _parse_json(rule.trigger_config_json),
        "conditions": _parse_json(rule.conditions_json),
        "action_type": rule.action_type,
        "action_payload": _parse_json(rule.action_payload_json),
        "last_run_at": rule.last_run_at.isoformat() if rule.last_run_at else None,
        "next_run_at": rule.next_run_at.isoformat() if rule.next_run_at else None,
        "created_at": rule.created_at.isoformat() if rule.created_at else None,
    }


@router.get("/rules")
def get_rules(user_id: str, db: Session = Depends(get_db)):
    rules = list_rules(db, user_id)
    return {"items": [_serialize_rule(r) for r in rules]}


@router.post("/rules")
def create_rule_endpoint(request: CreateRuleRequest, db: Session = Depends(get_db)):
    rule = create_rule(
        db,
        user_id=request.user_id,
        name=request.name,
        trigger_type=request.trigger_type,
        trigger_config=request.trigger_config,
        action_type=request.action_type,
        action_payload=request.action_payload,
        conditions=request.conditions,
        is_active=request.is_active,
    )
    return {"ok": True, "rule": _serialize_rule(rule)}


@router.patch("/rules/{rule_id}")
def update_rule_endpoint(rule_id: int, request: UpdateRuleRequest, db: Session = Depends(get_db)):
    rule = get_rule(db, rule_id)
    if not rule:
        raise HTTPException(status_code=404, detail="Rule not found")
    rule = update_rule(db, rule, request.model_dump(exclude_unset=True))
    return {"ok": True, "rule": _serialize_rule(rule)}


@router.post("/rules/{rule_id}/run")
def run_rule_endpoint(rule_id: int, request: RunRuleRequest, db: Session = Depends(get_db)):
    rule = get_rule(db, rule_id)
    if not rule:
        raise HTTPException(status_code=404, detail="Rule not found")
    result = run_rule(db, rule, force=request.force)
    return {"ok": True, "result": result}
