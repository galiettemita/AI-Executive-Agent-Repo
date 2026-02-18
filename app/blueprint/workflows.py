from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import Any
from uuid import uuid4

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.contracts import WorkflowAction, WorkflowCondition, WorkflowDefinition, WorkflowTrigger
from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables where table_schema = current_schema() and table_name = :name"
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


def _loads_json(value: Any, default: Any) -> Any:
    if isinstance(value, (list, dict)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except Exception:
            return default
    return default


def parse_nl_workflow_definition(text_value: str) -> WorkflowDefinition:
    raw = str(text_value or "").strip()
    if not raw:
        raise ValueError("natural_language is required")

    lines = [line.strip(" -\t") for line in raw.splitlines() if line.strip()]
    title = lines[0][:120] if lines else "Custom Workflow"
    lower = raw.lower()
    trigger_type = "manual"
    trigger_config: dict[str, Any] = {}
    if "daily" in lower:
        trigger_type = "schedule"
        trigger_config = {"cron": "0 9 * * *", "timezone": "UTC"}
    elif "weekly" in lower:
        trigger_type = "schedule"
        trigger_config = {"cron": "0 9 * * 1", "timezone": "UTC"}
    elif "when" in lower or "if" in lower:
        trigger_type = "condition"
        trigger_config = {"expression": raw[:240]}
    elif "on " in lower and ("message" in lower or "email" in lower or "slack" in lower):
        trigger_type = "event"
        trigger_config = {"event": "message_received"}

    actions: list[WorkflowAction] = []
    action_lines = lines[1:] if len(lines) > 1 else [raw]
    step = 1
    for line in action_lines[:8]:
        action_tool = "web.search"
        if "email" in line.lower():
            action_tool = "email.send"
        elif "calendar" in line.lower():
            action_tool = "calendar.create"
        elif "slack" in line.lower():
            action_tool = "slack.messages.send"
        elif "research" in line.lower():
            action_tool = "tavily.search"

        actions.append(
            WorkflowAction(
                step=step,
                tool_name=action_tool,
                arguments_template={"instruction": line},
                on_failure="continue" if step < 3 else "stop",
                approval_required=action_tool in {"email.send", "calendar.create", "slack.messages.send"},
            )
        )
        step += 1

    if not actions:
        actions.append(
            WorkflowAction(
                step=1,
                tool_name="web.search",
                arguments_template={"instruction": raw},
                on_failure="stop",
                approval_required=False,
            )
        )

    return WorkflowDefinition(
        name=title,
        description=raw[:500],
        trigger=WorkflowTrigger(type=trigger_type, config=trigger_config),
        actions=actions,
        conditions=[WorkflowCondition(field="user_active", operator="==", value="true")] if trigger_type == "condition" else None,
    )


def _refresh_workflows_knowledge_file(db: Session, *, user_id: str) -> None:
    rows = db.execute(
        text(
            """
            select id, name, description, trigger_type, status, next_run_at, updated_at
            from workflows
            where user_id = :user_id
            order by updated_at desc
            limit 50
            """
        ),
        {"user_id": user_id},
    ).mappings().all()

    lines = [
        "# WORKFLOWS.md",
        "## Active Workflows",
    ]
    if rows:
        for row in rows:
            next_run = row.get("next_run_at")
            next_text = next_run.isoformat() if isinstance(next_run, datetime) else str(next_run or "-")
            lines.append(
                f"- {row.get('name') or 'Untitled'} ({row.get('status')}) "
                f"[trigger={row.get('trigger_type')}, next={next_text}]"
            )
    else:
        lines.append("- No workflows configured.")

    lines.extend(["", "## Workflow Templates", "- Daily briefing workflow", "- Weekly review workflow"])
    lines.extend(["", "## Trigger Definitions", "- schedule", "- event", "- condition", "- manual"])
    lines.extend(["", "## Error Handling Preferences", "- stop on critical side effects", "- continue on read-only failures"])
    content = "\n".join(lines).strip()

    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="WORKFLOWS.md")
    if latest and str(latest.get("content") or "").strip() == content:
        return
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="WORKFLOWS.md",
        content=content,
        metadata={"source": "workflow_engine"},
    )


def list_workflows(db: Session, *, user_id: str) -> list[dict[str, Any]]:
    if not _table_exists(db, "workflows"):
        raise RuntimeError("workflows table not found")
    rows = db.execute(
        text(
            """
            select id, user_id, name, description, trigger_type, trigger_config, actions, status,
                   last_run_at, next_run_at, run_count, error_count, last_error, created_at, updated_at
            from workflows
            where user_id = :user_id
            order by updated_at desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    out: list[dict[str, Any]] = []
    for row in rows:
        item = dict(row)
        item["trigger_config"] = _loads_json(item.get("trigger_config"), {})
        item["actions"] = _loads_json(item.get("actions"), [])
        item["last_error"] = _loads_json(item.get("last_error"), None)
        out.append(item)
    return out


def create_workflow(
    db: Session,
    *,
    user_id: str,
    definition: WorkflowDefinition,
    status: str = "active",
) -> dict[str, Any]:
    if not _table_exists(db, "workflows"):
        raise RuntimeError("workflows table not found")
    now = datetime.utcnow()
    next_run_at = now + timedelta(days=1) if definition.trigger.type == "schedule" else None
    row = db.execute(
        text(
            """
            insert into workflows (
                id, user_id, name, description, trigger_type, trigger_config, actions,
                status, next_run_at, run_count, error_count, created_at, updated_at
            ) values (
                :id, :user_id, :name, :description, :trigger_type, :trigger_config,
                :actions, :status, :next_run_at, 0, 0, :now, :now
            )
            returning id, user_id, name, description, trigger_type, trigger_config, actions, status,
                      last_run_at, next_run_at, run_count, error_count, last_error, created_at, updated_at
            """
        ),
        {
            "id": str(uuid4()),
            "user_id": user_id,
            "name": definition.name,
            "description": definition.description,
            "trigger_type": definition.trigger.type.value,
            "trigger_config": json.dumps(definition.trigger.config, ensure_ascii=False),
            "actions": json.dumps([a.model_dump() for a in definition.actions], ensure_ascii=False),
            "status": status,
            "next_run_at": next_run_at,
            "now": now,
        },
    ).mappings().first()
    db.commit()
    _refresh_workflows_knowledge_file(db, user_id=user_id)
    item = dict(row or {})
    item["trigger_config"] = _loads_json(item.get("trigger_config"), {})
    item["actions"] = _loads_json(item.get("actions"), [])
    item["last_error"] = _loads_json(item.get("last_error"), None)
    return item


def get_workflow(db: Session, *, user_id: str, workflow_id: str) -> dict[str, Any] | None:
    if not _table_exists(db, "workflows"):
        raise RuntimeError("workflows table not found")
    row = db.execute(
        text(
            """
            select id, user_id, name, description, trigger_type, trigger_config, actions, status,
                   last_run_at, next_run_at, run_count, error_count, last_error, created_at, updated_at
            from workflows
            where user_id = :user_id and id = :workflow_id
            """
        ),
        {"user_id": user_id, "workflow_id": workflow_id},
    ).mappings().first()
    if not row:
        return None
    item = dict(row)
    item["trigger_config"] = _loads_json(item.get("trigger_config"), {})
    item["actions"] = _loads_json(item.get("actions"), [])
    item["last_error"] = _loads_json(item.get("last_error"), None)
    return item


def update_workflow(
    db: Session,
    *,
    user_id: str,
    workflow_id: str,
    definition: WorkflowDefinition | None = None,
    status: str | None = None,
) -> dict[str, Any] | None:
    if not _table_exists(db, "workflows"):
        raise RuntimeError("workflows table not found")
    updates: list[str] = []
    params: dict[str, Any] = {"workflow_id": workflow_id, "user_id": user_id, "updated_at": datetime.utcnow()}
    if definition is not None:
        updates.extend(
            [
                "name = :name",
                "description = :description",
                "trigger_type = :trigger_type",
                "trigger_config = :trigger_config",
                "actions = :actions",
            ]
        )
        params["name"] = definition.name
        params["description"] = definition.description
        params["trigger_type"] = definition.trigger.type.value
        params["trigger_config"] = json.dumps(definition.trigger.config, ensure_ascii=False)
        params["actions"] = json.dumps([a.model_dump() for a in definition.actions], ensure_ascii=False)
    if status is not None:
        updates.append("status = :status")
        params["status"] = status
    if not updates:
        return get_workflow(db, user_id=user_id, workflow_id=workflow_id)

    updated = db.execute(
        text(
            f"update workflows set {', '.join(updates)}, updated_at = :updated_at "
            "where id = :workflow_id and user_id = :user_id"
        ),
        params,
    ).rowcount
    if not updated:
        db.rollback()
        return None
    db.commit()
    _refresh_workflows_knowledge_file(db, user_id=user_id)
    return get_workflow(db, user_id=user_id, workflow_id=workflow_id)


def delete_workflow(db: Session, *, user_id: str, workflow_id: str) -> bool:
    if not _table_exists(db, "workflows"):
        raise RuntimeError("workflows table not found")
    deleted = db.execute(
        text("delete from workflows where id = :workflow_id and user_id = :user_id"),
        {"workflow_id": workflow_id, "user_id": user_id},
    ).rowcount
    db.commit()
    _refresh_workflows_knowledge_file(db, user_id=user_id)
    return bool(deleted)


def dry_run_workflow(
    db: Session,
    *,
    user_id: str,
    workflow_id: str,
    sample_inputs: dict[str, Any] | None = None,
) -> dict[str, Any]:
    item = get_workflow(db, user_id=user_id, workflow_id=workflow_id)
    if not item:
        raise RuntimeError("Workflow not found")

    actions = item.get("actions") or []
    plan: list[dict[str, Any]] = []
    for idx, action in enumerate(actions, start=1):
        step = action if isinstance(action, dict) else {}
        args = dict(step.get("arguments_template") or {})
        for key, value in (sample_inputs or {}).items():
            args.setdefault(key, value)
        plan.append(
            {
                "step": idx,
                "tool_name": str(step.get("tool_name") or "web.search"),
                "approval_required": bool(step.get("approval_required")),
                "on_failure": str(step.get("on_failure") or "stop"),
                "resolved_arguments": args,
            }
        )

    return {
        "workflow_id": workflow_id,
        "name": item.get("name"),
        "status": item.get("status"),
        "trigger_type": item.get("trigger_type"),
        "steps": plan,
        "dry_run": True,
    }
