from __future__ import annotations

from datetime import datetime, timedelta
from typing import Any

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.blueprint.knowledge_files import ensure_default_knowledge_files, list_knowledge_files
from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version
from app.blueprint.preferences_learning import record_feedback_signal
from app.blueprint.profiling import (
    ensure_phase1_profiling_sessions,
    get_next_profile_question,
    record_profile_answer,
)
from app.blueprint.team import get_or_refresh_team_view, refresh_team_knowledge_file
from app.blueprint.proactive import create_proactive_trigger, eligible_due_triggers, mark_trigger_fired, render_proactive_message
from app.blueprint.contracts import WorkflowDefinition
from app.blueprint.research import (
    create_research_job,
    delete_research_job,
    get_research_job,
    list_research_jobs,
    run_research_job,
    update_research_job,
)
from app.blueprint.workflows import (
    create_workflow,
    delete_workflow,
    dry_run_workflow,
    get_workflow,
    list_workflows,
    parse_nl_workflow_definition,
    update_workflow,
)


router = APIRouter(prefix="/api/v1", tags=["behavioral-v1"])


class OnboardingStartRequest(BaseModel):
    user_id: str
    display_name: str = "User"
    timezone: str = "America/New_York"


class ProfilingAnswerRequest(BaseModel):
    session_id: str
    answer: str


class MemorySearchRequest(BaseModel):
    user_id: str
    query: str
    top_k: int = 5


class FeedbackRequest(BaseModel):
    user_id: str
    signal_type: str
    original_output: str | None = None
    corrected_output: str | None = None
    context: dict[str, Any] = {}


class RuleUpdateRequest(BaseModel):
    rule_value: str | None = None
    confidence: float | None = None
    category: str | None = None


class DelegationCreateRequest(BaseModel):
    user_id: str
    delegate_name: str
    delegate_contact: str | None = None
    task_description: str
    due_at: datetime | None = None
    priority: str = "medium"
    source: str = "manual"


class DelegationUpdateRequest(BaseModel):
    status: str | None = None
    result_summary: str | None = None
    due_at: datetime | None = None
    reminder_sent_at: datetime | None = None


class ProactiveBootstrapRequest(BaseModel):
    user_id: str
    timezone: str = "America/New_York"


class GoalCreateRequest(BaseModel):
    user_id: str
    title: str
    description: str | None = None
    timeframe: str = "this_month"
    status: str = "active"
    target_date: datetime | None = None


class GoalUpdateRequest(BaseModel):
    title: str | None = None
    description: str | None = None
    timeframe: str | None = None
    status: str | None = None
    progress_pct: float | None = None
    target_date: datetime | None = None


class WorkflowCreateRequest(BaseModel):
    user_id: str
    definition: WorkflowDefinition | None = None
    natural_language: str | None = None
    status: str = "active"


class WorkflowUpdateRequest(BaseModel):
    definition: WorkflowDefinition | None = None
    natural_language: str | None = None
    status: str | None = None


class WorkflowTestRequest(BaseModel):
    sample_inputs: dict[str, Any] = Field(default_factory=dict)


class ResearchCreateRequest(BaseModel):
    user_id: str
    title: str
    query: str
    sources: list[str] = Field(default_factory=list)
    schedule: str | None = None
    status: str = "active"
    delivery_channel: str | None = "whatsapp"
    delivery_format: str = "summary"
    max_cost_per_run: float = 0.5


class ResearchUpdateRequest(BaseModel):
    title: str | None = None
    query: str | None = None
    sources: list[str] | None = None
    schedule: str | None = None
    status: str | None = None
    delivery_channel: str | None = None
    delivery_format: str | None = None
    max_cost_per_run: float | None = None
    next_run_at: datetime | None = None


class KnowledgeReviewRequest(BaseModel):
    user_id: str
    focus: str | None = None


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(
            text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _touch_heartbeat_for_delegations(db: Session, *, user_id: str) -> None:
    if not _table_exists(db, "delegations"):
        return
    rows = db.execute(
        text(
            """
            select delegate_name, task_description, status, due_at
            from delegations
            where user_id = :user_id
            order by created_at desc
            limit 12
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    lines = ["# HEARTBEAT.md", "## Delegation Tracker"]
    if rows:
        for row in rows:
            due_raw = row.get("due_at")
            due = due_raw.isoformat() if isinstance(due_raw, datetime) else str(due_raw or "-")
            lines.append(
                f"- {row.get('delegate_name')}: {row.get('status')} — {str(row.get('task_description') or '')[:120]} (due: {due})"
            )
    else:
        lines.append("- No delegations recorded.")
    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    content = "\n".join(lines)
    if latest and str(latest.get("content") or "").strip() == content.strip():
        return
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="HEARTBEAT.md",
        content=content,
        metadata={"source": "delegation_tracker"},
    )


@router.post("/onboarding/start")
def onboarding_start(payload: OnboardingStartRequest, db: Session = Depends(get_db)):
    try:
        inserted = ensure_default_knowledge_files(
            db,
            user_id=payload.user_id,
            display_name=payload.display_name,
            timezone=payload.timezone,
        )
        ensure_phase1_profiling_sessions(db, payload.user_id)
        next_q = get_next_profile_question(db, payload.user_id)
        return {
            "ok": True,
            "knowledge_files_seeded": inserted,
            "next_question": next_q,
        }
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Onboarding bootstrap failed: {exc}")


@router.get("/profiling/next")
def profiling_next(user_id: str, db: Session = Depends(get_db)):
    try:
        ensure_phase1_profiling_sessions(db, user_id)
        next_q = get_next_profile_question(db, user_id)
        return {"ok": True, "next": next_q}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Profiling lookup failed: {exc}")


@router.post("/profiling/answer")
def profiling_answer(payload: ProfilingAnswerRequest, db: Session = Depends(get_db)):
    try:
        result = record_profile_answer(db, session_id=payload.session_id, answer=payload.answer)
        return {"ok": True, "result": result}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Profiling update failed: {exc}")


@router.get("/knowledge/list")
def knowledge_list(user_id: str, db: Session = Depends(get_db)):
    try:
        files = list_knowledge_files(db, user_id=user_id)
        return {"ok": True, "files": files}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Knowledge list failed: {exc}")


@router.post("/memories/search")
def memory_search(payload: MemorySearchRequest, db: Session = Depends(get_db)):
    if not _table_exists(db, "memories"):
        raise HTTPException(status_code=404, detail="memories table not found")
    q = f"%{(payload.query or '').strip().lower()}%"
    rows = db.execute(
        text(
            """
            select id, memory_type, content, confidence, source, created_at
            from memories
            where user_id = :user_id
              and lower(content) like :q
            order by confidence desc, created_at desc
            limit :limit
            """
        ),
        {"user_id": payload.user_id, "q": q, "limit": max(1, min(50, payload.top_k))},
    ).mappings().all()
    out = []
    for row in rows:
        out.append(
            {
                "id": str(row.get("id")),
                "type": row.get("memory_type"),
                "content": row.get("content"),
                "confidence": row.get("confidence"),
                "source": row.get("source"),
                "created_at": row.get("created_at").isoformat() if isinstance(row.get("created_at"), datetime) else row.get("created_at"),
            }
        )
    return {"ok": True, "entries": out}


@router.post("/feedback")
def feedback_ingest(payload: FeedbackRequest, db: Session = Depends(get_db)):
    result = record_feedback_signal(
        db,
        user_id=payload.user_id,
        signal_type=payload.signal_type,
        original_output=payload.original_output,
        corrected_output=payload.corrected_output,
        context=payload.context or {},
    )
    if not result.get("ok"):
        raise HTTPException(status_code=404, detail="feedback_signals table not found")
    return {"ok": True, "learning_applied": bool(result.get("learning_applied"))}


@router.get("/rules")
def list_rules(user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "behavioral_rules"):
        raise HTTPException(status_code=404, detail="behavioral_rules table not found")
    rows = db.execute(
        text(
            """
            select id, file_path, category, rule_key, rule_value, source, confidence, created_at
            from behavioral_rules
            where user_id = :user_id
            order by confidence desc, created_at desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    return {"ok": True, "items": [dict(r) for r in rows]}


@router.put("/rules/{rule_id}")
def update_rule(rule_id: str, payload: RuleUpdateRequest, user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "behavioral_rules"):
        raise HTTPException(status_code=404, detail="behavioral_rules table not found")
    params: dict[str, Any] = {"rule_id": rule_id, "user_id": user_id}
    updates: list[str] = []
    if payload.rule_value is not None:
        updates.append("rule_value = :rule_value")
        params["rule_value"] = payload.rule_value
    if payload.confidence is not None:
        updates.append("confidence = :confidence")
        params["confidence"] = payload.confidence
    if payload.category is not None:
        updates.append("category = :category")
        params["category"] = payload.category
    if not updates:
        return {"ok": True, "updated": 0}
    updated = db.execute(
        text(
            f"update behavioral_rules set {', '.join(updates)} "
            "where id = :rule_id and user_id = :user_id"
        ),
        params,
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Rule not found")
    return {"ok": True, "updated": updated}


@router.delete("/rules/{rule_id}")
def delete_rule(rule_id: str, user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "behavioral_rules"):
        raise HTTPException(status_code=404, detail="behavioral_rules table not found")
    deleted = db.execute(
        text("delete from behavioral_rules where id = :rule_id and user_id = :user_id"),
        {"rule_id": rule_id, "user_id": user_id},
    ).rowcount
    db.commit()
    if not deleted:
        raise HTTPException(status_code=404, detail="Rule not found")
    return {"ok": True, "deleted": deleted}


@router.get("/team")
def team_view(user_id: str, refresh: bool = False, db: Session = Depends(get_db)):
    try:
        if refresh:
            data = refresh_team_knowledge_file(db, user_id=user_id)
        else:
            data = get_or_refresh_team_view(db, user_id=user_id)
        return {"ok": True, **data}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Team view failed: {exc}")


@router.get("/delegations")
def list_delegations(user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "delegations"):
        raise HTTPException(status_code=404, detail="delegations table not found")
    rows = db.execute(
        text(
            """
            select id, delegate_name, delegate_contact, task_description, status, due_at, created_at, completed_at, reminder_sent_at
            from delegations
            where user_id = :user_id
            order by created_at desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    return {"ok": True, "items": [dict(r) for r in rows]}


@router.post("/delegations")
def create_delegation(payload: DelegationCreateRequest, db: Session = Depends(get_db)):
    if not _table_exists(db, "delegations"):
        raise HTTPException(status_code=404, detail="delegations table not found")
    row = db.execute(
        text(
            """
            insert into delegations (
                user_id, delegate_name, delegate_contact, task_description,
                due_at, status, source, created_at
            ) values (
                :user_id, :delegate_name, :delegate_contact, :task_description,
                :due_at, 'pending', :source, now()
            )
            returning id, user_id, delegate_name, delegate_contact, task_description, status, due_at, created_at
            """
        ),
        {
            "user_id": payload.user_id,
            "delegate_name": payload.delegate_name,
            "delegate_contact": payload.delegate_contact,
            "task_description": payload.task_description,
            "due_at": payload.due_at,
            "source": payload.source,
        },
    ).mappings().first()
    db.commit()
    if payload.due_at:
        reminder_time = payload.due_at - timedelta(hours=6)
        if reminder_time <= datetime.utcnow():
            reminder_time = datetime.utcnow() + timedelta(minutes=20)
        create_proactive_trigger(
            db,
            user_id=payload.user_id,
            trigger_type="delegation",
            source="delegation_reminder",
            payload={
                "template": "delegation_reminder_v1",
                "delegation_id": str((row or {}).get("id") or ""),
                "delegate_name": payload.delegate_name,
                "task_description": payload.task_description,
            },
            fire_at=reminder_time,
            delegation_id=str((row or {}).get("id") or "") or None,
        )
    _touch_heartbeat_for_delegations(db, user_id=payload.user_id)
    return {"ok": True, "delegation": dict(row or {})}


@router.get("/delegations/{delegation_id}")
def get_delegation(delegation_id: str, user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "delegations"):
        raise HTTPException(status_code=404, detail="delegations table not found")
    row = db.execute(
        text(
            """
            select id, user_id, delegate_name, delegate_contact, task_description, status, source,
                   due_at, created_at, completed_at, reminder_sent_at, result_summary
            from delegations
            where id = :delegation_id and user_id = :user_id
            """
        ),
        {"delegation_id": delegation_id, "user_id": user_id},
    ).mappings().first()
    if not row:
        raise HTTPException(status_code=404, detail="Delegation not found")
    return {"ok": True, "delegation": dict(row)}


@router.put("/delegations/{delegation_id}")
def update_delegation(
    delegation_id: str,
    payload: DelegationUpdateRequest,
    user_id: str,
    db: Session = Depends(get_db),
):
    if not _table_exists(db, "delegations"):
        raise HTTPException(status_code=404, detail="delegations table not found")
    updates: list[str] = []
    params: dict[str, Any] = {"delegation_id": delegation_id, "user_id": user_id}
    if payload.status is not None:
        updates.append("status = :status")
        params["status"] = payload.status
    if payload.result_summary is not None:
        updates.append("result_summary = :result_summary")
        params["result_summary"] = payload.result_summary
    if payload.due_at is not None:
        updates.append("due_at = :due_at")
        params["due_at"] = payload.due_at
    if payload.reminder_sent_at is not None:
        updates.append("reminder_sent_at = :reminder_sent_at")
        params["reminder_sent_at"] = payload.reminder_sent_at
    if payload.status == "completed":
        updates.append("completed_at = now()")
    if not updates:
        return {"ok": True, "updated": 0}

    updated = db.execute(
        text(
            f"update delegations set {', '.join(updates)} "
            "where id = :delegation_id and user_id = :user_id"
        ),
        params,
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Delegation not found")
    _touch_heartbeat_for_delegations(db, user_id=user_id)
    return {"ok": True, "updated": updated}


@router.post("/proactive/bootstrap")
def bootstrap_proactive(payload: ProactiveBootstrapRequest, db: Session = Depends(get_db)):
    if not _table_exists(db, "proactive_triggers"):
        raise HTTPException(status_code=404, detail="proactive_triggers table not found")

    now = datetime.utcnow()
    created = []
    created.append(
        create_proactive_trigger(
            db,
            user_id=payload.user_id,
            trigger_type="schedule",
            source="morning_briefing",
            payload={"template": "daily_brief_v1", "kind": "morning_brief"},
            fire_at=now + timedelta(minutes=5),
        )
    )
    created.append(
        create_proactive_trigger(
            db,
            user_id=payload.user_id,
            trigger_type="event",
            source="pre_meeting_prep",
            payload={"template": "daily_brief_v1", "kind": "pre_meeting"},
            fire_at=now + timedelta(minutes=15),
        )
    )
    created.append(
        create_proactive_trigger(
            db,
            user_id=payload.user_id,
            trigger_type="pattern",
            source="followup_nudge",
            payload={"template": "followup_nudge_v1", "kind": "followup"},
            fire_at=now + timedelta(minutes=30),
        )
    )
    return {"ok": True, "created": created}


@router.post("/proactive/run")
def run_due_proactive(user_id: str, db: Session = Depends(get_db)):
    due = eligible_due_triggers(db, user_id=user_id)
    fired: list[dict[str, Any]] = []
    for item in due:
        mark_trigger_fired(db, trigger_id=str(item.get("id")))
        fired.append(
            {
                "id": item.get("id"),
                "source": item.get("source"),
                "trigger_type": item.get("trigger_type"),
                "payload": item.get("payload"),
                "message": render_proactive_message(
                    trigger_source=str(item.get("source") or ""),
                    payload=item.get("payload") if isinstance(item.get("payload"), dict) else {},
                ),
            }
        )
    return {"ok": True, "fired": fired, "count": len(fired)}


@router.get("/goals")
def list_goals(user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "goals"):
        raise HTTPException(status_code=404, detail="goals table not found")
    rows = db.execute(
        text(
            """
            select id, title, description, timeframe, status, progress_pct, target_date, created_at, completed_at
            from goals
            where user_id = :user_id
            order by created_at desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    return {"ok": True, "items": [dict(r) for r in rows]}


@router.post("/goals")
def create_goal(payload: GoalCreateRequest, db: Session = Depends(get_db)):
    if not _table_exists(db, "goals"):
        raise HTTPException(status_code=404, detail="goals table not found")
    row = db.execute(
        text(
            """
            insert into goals (
                user_id, title, description, timeframe, status, target_date, progress_pct, created_at
            ) values (
                :user_id, :title, :description, :timeframe, :status, :target_date, 0, now()
            )
            returning id, title, description, timeframe, status, progress_pct, target_date, created_at
            """
        ),
        {
            "user_id": payload.user_id,
            "title": payload.title,
            "description": payload.description,
            "timeframe": payload.timeframe,
            "status": payload.status,
            "target_date": payload.target_date,
        },
    ).mappings().first()
    db.commit()
    return {"ok": True, "goal": dict(row or {})}


@router.put("/goals/{goal_id}")
def update_goal(goal_id: str, payload: GoalUpdateRequest, user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "goals"):
        raise HTTPException(status_code=404, detail="goals table not found")
    params: dict[str, Any] = {"goal_id": goal_id, "user_id": user_id}
    updates: list[str] = []
    if payload.title is not None:
        updates.append("title = :title")
        params["title"] = payload.title
    if payload.description is not None:
        updates.append("description = :description")
        params["description"] = payload.description
    if payload.timeframe is not None:
        updates.append("timeframe = :timeframe")
        params["timeframe"] = payload.timeframe
    if payload.status is not None:
        updates.append("status = :status")
        params["status"] = payload.status
    if payload.progress_pct is not None:
        updates.append("progress_pct = :progress_pct")
        params["progress_pct"] = payload.progress_pct
    if payload.target_date is not None:
        updates.append("target_date = :target_date")
        params["target_date"] = payload.target_date
    if payload.status == "completed":
        updates.append("completed_at = now()")
    if not updates:
        return {"ok": True, "updated": 0}
    updated = db.execute(
        text(
            f"update goals set {', '.join(updates)} "
            "where id = :goal_id and user_id = :user_id"
        ),
        params,
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Goal not found")
    return {"ok": True, "updated": updated}


@router.delete("/goals/{goal_id}")
def delete_goal(goal_id: str, user_id: str, db: Session = Depends(get_db)):
    if not _table_exists(db, "goals"):
        raise HTTPException(status_code=404, detail="goals table not found")
    deleted = db.execute(
        text("delete from goals where id = :goal_id and user_id = :user_id"),
        {"goal_id": goal_id, "user_id": user_id},
    ).rowcount
    db.commit()
    if not deleted:
        raise HTTPException(status_code=404, detail="Goal not found")
    return {"ok": True, "deleted": deleted}


@router.get("/heartbeat")
def get_heartbeat(user_id: str, db: Session = Depends(get_db)):
    item = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    if not item:
        raise HTTPException(status_code=404, detail="HEARTBEAT.md not found")
    return {"ok": True, "heartbeat": item}


@router.post("/knowledge/review")
def knowledge_review(payload: KnowledgeReviewRequest, db: Session = Depends(get_db)):
    files = list_knowledge_files(db, user_id=payload.user_id)
    file_map = {str(item.get("file_path")): item for item in files}
    required = [
        "USER.md",
        "SOUL.md",
        "IDENTITY.md",
        "AGENTS.md",
        "MEMORY.md",
        "HEARTBEAT.md",
        "TOOLS.md",
        "TEAM.md",
        "WORKFLOWS.md",
    ]
    missing = [name for name in required if name not in file_map]
    stale: list[str] = []
    now = datetime.utcnow()
    for name, item in file_map.items():
        updated = item.get("updated_at")
        if isinstance(updated, datetime) and (now - updated).days >= 14:
            stale.append(name)

    questions: list[str] = []
    if "WORKFLOWS.md" in missing:
        questions.append("What repeating tasks should I automate for you this week?")
    if "TEAM.md" in missing:
        questions.append("Who are your top collaborators and preferred communication channels?")
    if stale:
        questions.append("Should I refresh outdated knowledge files based on recent behavior?")
    if payload.focus:
        questions.append(f"What specific updates should I make for {payload.focus}?")

    return {
        "ok": True,
        "review": {
            "missing_files": missing,
            "stale_files": sorted(stale),
            "questions": questions,
            "completeness": {
                "required": len(required),
                "present": len(required) - len(missing),
            },
        },
    }


@router.get("/workflows")
def workflows_list(user_id: str, db: Session = Depends(get_db)):
    try:
        items = list_workflows(db, user_id=user_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    return {"ok": True, "items": items}


@router.post("/workflows")
def workflows_create(payload: WorkflowCreateRequest, db: Session = Depends(get_db)):
    try:
        definition = payload.definition
        if definition is None:
            definition = parse_nl_workflow_definition(payload.natural_language or "")
        item = create_workflow(
            db,
            user_id=payload.user_id,
            definition=definition,
            status=payload.status,
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Workflow create failed: {exc}")
    return {"ok": True, "workflow": item}


@router.get("/workflows/{workflow_id}")
def workflows_get(workflow_id: str, user_id: str, db: Session = Depends(get_db)):
    try:
        item = get_workflow(db, user_id=user_id, workflow_id=workflow_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not item:
        raise HTTPException(status_code=404, detail="Workflow not found")
    return {"ok": True, "workflow": item}


@router.put("/workflows/{workflow_id}")
def workflows_update(workflow_id: str, user_id: str, payload: WorkflowUpdateRequest, db: Session = Depends(get_db)):
    definition = payload.definition
    if definition is None and payload.natural_language:
        try:
            definition = parse_nl_workflow_definition(payload.natural_language)
        except Exception as exc:
            raise HTTPException(status_code=400, detail=f"Invalid natural language workflow: {exc}")
    try:
        item = update_workflow(
            db,
            user_id=user_id,
            workflow_id=workflow_id,
            definition=definition,
            status=payload.status,
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not item:
        raise HTTPException(status_code=404, detail="Workflow not found")
    return {"ok": True, "workflow": item}


@router.delete("/workflows/{workflow_id}")
def workflows_delete(workflow_id: str, user_id: str, db: Session = Depends(get_db)):
    try:
        deleted = delete_workflow(db, user_id=user_id, workflow_id=workflow_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not deleted:
        raise HTTPException(status_code=404, detail="Workflow not found")
    return {"ok": True, "deleted": True}


@router.post("/workflows/{workflow_id}/test")
def workflows_test(workflow_id: str, user_id: str, payload: WorkflowTestRequest, db: Session = Depends(get_db)):
    try:
        result = dry_run_workflow(
            db,
            user_id=user_id,
            workflow_id=workflow_id,
            sample_inputs=payload.sample_inputs or {},
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    return {"ok": True, "result": result}


@router.get("/research")
def research_list(user_id: str, db: Session = Depends(get_db)):
    try:
        items = list_research_jobs(db, user_id=user_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    return {"ok": True, "items": items}


@router.post("/research")
def research_create(payload: ResearchCreateRequest, db: Session = Depends(get_db)):
    try:
        item = create_research_job(
            db,
            user_id=payload.user_id,
            title=payload.title,
            query=payload.query,
            sources=payload.sources,
            schedule=payload.schedule,
            status=payload.status,
            delivery_channel=payload.delivery_channel,
            delivery_format=payload.delivery_format,
            max_cost_per_run=payload.max_cost_per_run,
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Research create failed: {exc}")
    return {"ok": True, "research": item}


@router.get("/research/{research_id}")
def research_get(research_id: str, user_id: str, db: Session = Depends(get_db)):
    try:
        item = get_research_job(db, user_id=user_id, research_id=research_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not item:
        raise HTTPException(status_code=404, detail="Research job not found")
    return {"ok": True, "research": item}


@router.put("/research/{research_id}")
def research_update(research_id: str, user_id: str, payload: ResearchUpdateRequest, db: Session = Depends(get_db)):
    fields = payload.model_dump(exclude_none=True)
    try:
        item = update_research_job(db, user_id=user_id, research_id=research_id, fields=fields)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not item:
        raise HTTPException(status_code=404, detail="Research job not found")
    return {"ok": True, "research": item}


@router.delete("/research/{research_id}")
def research_delete(research_id: str, user_id: str, db: Session = Depends(get_db)):
    try:
        deleted = delete_research_job(db, user_id=user_id, research_id=research_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    if not deleted:
        raise HTTPException(status_code=404, detail="Research job not found")
    return {"ok": True, "deleted": True}


@router.post("/research/{research_id}/run")
def research_run(research_id: str, user_id: str, db: Session = Depends(get_db)):
    try:
        result = run_research_job(db, user_id=user_id, research_id=research_id)
    except RuntimeError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Research run failed: {exc}")
    return {"ok": True, **result}
