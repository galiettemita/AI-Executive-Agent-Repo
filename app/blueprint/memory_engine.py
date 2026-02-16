from __future__ import annotations

import json
import uuid
from datetime import datetime

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


def _append_memory_md(db: Session, *, user_id: str, user_text: str, assistant_text: str) -> None:
    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="MEMORY.md")
    header = "# MEMORY.md\n## Retention Rules\n- Keep durable preferences and stable facts.\n- Expire stale episodic context.\n\n## Recent Episodes"
    existing = str((latest or {}).get("content") or "").strip() or header
    line = f"- {datetime.utcnow().isoformat()}: user=\"{user_text[:120]}\" | assistant=\"{assistant_text[:120]}\""
    content = f"{existing}\n{line}".strip()
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="MEMORY.md",
        content=content,
        metadata={"source": "memory_dual_path"},
    )


def record_memory_dual_path(
    db: Session,
    *,
    user_id: str,
    run_id: str | None,
    user_text: str,
    assistant_text: str,
) -> None:
    if _table_exists(db, "memories"):
        db.execute(
            text(
                """
                insert into memories (
                    id, user_id, run_id, memory_type, content, sensitivity, confidence, source, created_at
                ) values (
                    :id, :user_id, :run_id, 'episode', :content,
                    'private', :confidence, 'conversation_turn', now()
                )
                """
            ),
            {
                "id": str(uuid.uuid4()),
                "user_id": user_id,
                "run_id": run_id,
                "content": f"User: {user_text[:500]}\nAssistant: {assistant_text[:500]}",
                "confidence": 0.8,
            },
        )

    _append_memory_md(db, user_id=user_id, user_text=user_text, assistant_text=assistant_text)

    # Lightweight graph edge for Team awareness / memory graph growth.
    if _table_exists(db, "knowledge_graph_edges"):
        db.execute(
            text(
                """
                insert into knowledge_graph_edges (
                    id, user_id, source_node, target_node, relation, weight, metadata, created_at
                ) values (
                    :id, :user_id, :source_node, :target_node, :relation, :weight, :metadata, now()
                )
                """
            ),
            {
                "id": str(uuid.uuid4()),
                "user_id": user_id,
                "source_node": "conversation:user",
                "target_node": "conversation:assistant",
                "relation": "responded_to",
                "weight": 0.5,
                "metadata": json.dumps({"run_id": run_id, "snippet": user_text[:80]}, ensure_ascii=False),
            },
        )

    db.commit()
