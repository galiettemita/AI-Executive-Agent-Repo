from __future__ import annotations

from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version


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


def build_team_snapshot(db: Session, *, user_id: str, max_people: int = 12) -> dict[str, Any]:
    people: list[dict[str, Any]] = []
    if _table_exists(db, "contacts"):
        rows = db.execute(
            text(
                """
                select id, name, email, phone, coalesce(updated_at, created_at) as freshness
                from contacts
                where user_id = :user_id
                order by freshness desc
                limit :max_people
                """
            ),
            {"user_id": user_id, "max_people": int(max_people)},
        ).mappings().all()
        for row in rows:
            people.append(
                {
                    "id": str(row.get("id") or ""),
                    "name": str(row.get("name") or "Unknown"),
                    "email": row.get("email"),
                    "phone": row.get("phone"),
                    "source": "contacts",
                }
            )

    delegation_summary: list[dict[str, Any]] = []
    if _table_exists(db, "delegations"):
        rows = db.execute(
            text(
                """
                select delegate_name, status, task_description, due_at, completed_at
                from delegations
                where user_id = :user_id
                order by created_at desc
                limit 25
                """
            ),
            {"user_id": user_id},
        ).mappings().all()
        for row in rows:
            delegation_summary.append(
                {
                    "delegate_name": str(row.get("delegate_name") or "Unknown"),
                    "status": str(row.get("status") or "pending"),
                    "task_description": str(row.get("task_description") or ""),
                    "due_at": row.get("due_at").isoformat() if isinstance(row.get("due_at"), datetime) else row.get("due_at"),
                    "completed_at": row.get("completed_at").isoformat()
                    if isinstance(row.get("completed_at"), datetime)
                    else row.get("completed_at"),
                }
            )

    profiling_signals: list[dict[str, Any]] = []
    if _table_exists(db, "profiling_sessions"):
        rows = db.execute(
            text(
                """
                select dimension, status, progress_pct, facts_extracted, completed_at
                from profiling_sessions
                where user_id = :user_id
                order by coalesce(completed_at, created_at) desc
                limit 20
                """
            ),
            {"user_id": user_id},
        ).mappings().all()
        for row in rows:
            profiling_signals.append(
                {
                    "dimension": str(row.get("dimension") or ""),
                    "status": str(row.get("status") or "pending"),
                    "progress_pct": float(row.get("progress_pct") or 0.0),
                    "facts_extracted": int(row.get("facts_extracted") or 0),
                    "completed_at": row.get("completed_at").isoformat()
                    if isinstance(row.get("completed_at"), datetime)
                    else row.get("completed_at"),
                }
            )

    interaction_graph: list[dict[str, Any]] = []
    if _table_exists(db, "knowledge_graph_edges"):
        rows = db.execute(
            text(
                """
                select source_node, target_node, relation, weight
                from knowledge_graph_edges
                where user_id = :user_id
                order by created_at desc
                limit 50
                """
            ),
            {"user_id": user_id},
        ).mappings().all()
        for row in rows:
            relation = str(row.get("relation") or "")
            if not relation:
                continue
            interaction_graph.append(
                {
                    "source_node": str(row.get("source_node") or ""),
                    "target_node": str(row.get("target_node") or ""),
                    "relation": relation,
                    "weight": float(row.get("weight") or 0.0),
                }
            )

    return {
        "people": people,
        "delegations": delegation_summary,
        "profiling_signals": profiling_signals,
        "interaction_graph": interaction_graph,
        "generated_at": datetime.utcnow().isoformat(),
    }


def _to_team_markdown(snapshot: dict[str, Any]) -> str:
    people = snapshot.get("people") or []
    delegations = snapshot.get("delegations") or []
    profiling_signals = snapshot.get("profiling_signals") or []
    interaction_graph = snapshot.get("interaction_graph") or []
    lines: list[str] = [
        "# TEAM.md",
        "## Team Directory",
    ]
    if people:
        for p in people:
            lines.append(
                f"- {p.get('name')} | email: {p.get('email') or '-'} | phone: {p.get('phone') or '-'}"
            )
    else:
        lines.append("- No contacts synced yet.")

    lines.append("")
    lines.append("## Delegation Track Record")
    if delegations:
        for d in delegations[:20]:
            lines.append(
                f"- {d.get('delegate_name')}: {d.get('status')} — {d.get('task_description')[:140]}"
            )
    else:
        lines.append("- No delegations recorded yet.")
    lines.append("")
    lines.append("## Profiling Signals")
    if profiling_signals:
        for p in profiling_signals[:12]:
            lines.append(
                f"- {p.get('dimension')}: {p.get('status')} | progress={p.get('progress_pct')} | facts={p.get('facts_extracted')}"
            )
    else:
        lines.append("- No profiling sessions completed yet.")
    lines.append("")
    lines.append("## Interaction Graph Hints")
    if interaction_graph:
        for edge in interaction_graph[:15]:
            lines.append(
                f"- {edge.get('source_node')} --[{edge.get('relation')}:{edge.get('weight')}]--> {edge.get('target_node')}"
            )
    else:
        lines.append("- No graph edges yet.")
    lines.append("")
    lines.append(f"_Last refreshed: {snapshot.get('generated_at')}_")
    return "\n".join(lines).strip()


def refresh_team_knowledge_file(db: Session, *, user_id: str) -> dict[str, Any]:
    snapshot = build_team_snapshot(db, user_id=user_id)
    content = _to_team_markdown(snapshot)
    latest = put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="TEAM.md",
        content=content,
        metadata={"source": "team_awareness_engine", "people_count": len(snapshot.get("people") or [])},
    )
    return {"snapshot": snapshot, "knowledge_file": latest}


def get_or_refresh_team_view(db: Session, *, user_id: str) -> dict[str, Any]:
    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="TEAM.md")
    if latest:
        return {"knowledge_file": latest, "snapshot": build_team_snapshot(db, user_id=user_id)}
    return refresh_team_knowledge_file(db, user_id=user_id)
