from __future__ import annotations

import json
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


PROFILE_DIMENSIONS_V1 = [
    "identity",
    "operations",
    "goals",
    "communication",
]

QUESTION_BANK: dict[str, list[str]] = {
    "identity": [
        "What should I call you, and what timezone do you operate in most often?",
        "What kind of role or responsibilities do you currently have?",
    ],
    "operations": [
        "What does an ideal weekday schedule look like for you?",
        "How much meeting buffer time do you prefer between calls?",
    ],
    "goals": [
        "What is the most important thing you want to accomplish this month?",
        "What recurring blocker slows you down most?",
    ],
    "communication": [
        "Do you prefer short direct updates or detailed briefings by default?",
        "When should I proactively ping you versus waiting for you to ask?",
    ],
}


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


def ensure_phase1_profiling_sessions(db: Session, user_id: str) -> None:
    if not _table_exists(db, "profiling_sessions"):
        return

    for dim in PROFILE_DIMENSIONS_V1:
        row = db.execute(
            text(
                "select id from profiling_sessions where user_id = :user_id and dimension = :dimension limit 1"
            ),
            {"user_id": user_id, "dimension": dim},
        ).mappings().first()
        if row:
            continue
        db.execute(
            text(
                """
                insert into profiling_sessions (
                    user_id, dimension, layer, status, questions_asked, facts_extracted, progress_pct, created_at
                ) values (
                    :user_id, :dimension, 'brain', 'pending',
                    0, 0, 0, now()
                )
                """
            ),
            {"user_id": user_id, "dimension": dim},
        )
    db.commit()


def get_next_profile_question(db: Session, user_id: str) -> dict[str, Any] | None:
    if not _table_exists(db, "profiling_sessions"):
        return None
    rows = db.execute(
        text(
            """
            select id, dimension, status, questions_asked, progress_pct
            from profiling_sessions
            where user_id = :user_id
            order by created_at asc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()

    for row in rows:
        dim = row.get("dimension")
        asked = int(row.get("questions_asked") or 0)
        questions = QUESTION_BANK.get(dim) or []
        if asked < len(questions):
            return {
                "session_id": str(row.get("id")),
                "dimension": dim,
                "question": questions[asked],
                "progress_pct": float(row.get("progress_pct") or 0),
            }
    return None


def _extract_simple_facts(dimension: str, answer: str) -> dict[str, Any]:
    answer = (answer or "").strip()
    if not answer:
        return {}
    if dimension == "identity":
        return {"identity_notes": answer[:500]}
    if dimension == "operations":
        return {"operations_notes": answer[:500]}
    if dimension == "goals":
        return {"goal_notes": answer[:500]}
    if dimension == "communication":
        return {"communication_notes": answer[:500]}
    return {"note": answer[:500]}


def record_profile_answer(db: Session, *, session_id: str, answer: str) -> dict[str, Any]:
    if not _table_exists(db, "profiling_sessions"):
        raise ValueError("profiling_sessions table not found")

    session = db.execute(
        text(
            "select id, user_id, dimension, questions_asked from profiling_sessions where id = :id"
        ),
        {"id": session_id},
    ).mappings().first()
    if not session:
        raise ValueError("Profiling session not found")

    dimension = str(session.get("dimension") or "")
    asked = int(session.get("questions_asked") or 0)
    questions = QUESTION_BANK.get(dimension) or []
    next_asked = asked + 1
    done = next_asked >= len(questions)

    facts = _extract_simple_facts(dimension, answer)

    db.execute(
        text(
            """
            update profiling_sessions
            set
              status = :status,
              questions_asked = :questions_asked,
              facts_extracted = facts_extracted + :facts_inc,
              progress_pct = :progress_pct,
              completed_at = case when :done then now() else completed_at end
            where id = :id
            """
        ),
        {
            "id": session_id,
            "status": "completed" if done else "active",
            "questions_asked": next_asked,
            "facts_inc": 1 if facts else 0,
            "progress_pct": 100.0 if done else round((next_asked / max(1, len(questions))) * 100, 2),
            "done": done,
        },
    )

    if facts and _table_exists(db, "feedback_signals"):
        db.execute(
            text(
                """
                insert into feedback_signals (user_id, signal_type, original_output, corrected_output, context, created_at)
                values (:user_id, 'edit', :original, :corrected, :context, now())
                """
            ),
            {
                "user_id": str(session.get("user_id")),
                "original": "profiling_answer",
                "corrected": answer[:1000],
                "context": json.dumps({"dimension": dimension, "facts": facts}, ensure_ascii=False),
            },
        )

    db.commit()
    return {"dimension": dimension, "facts": facts, "completed": done}
