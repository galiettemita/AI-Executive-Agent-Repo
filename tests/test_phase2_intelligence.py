from __future__ import annotations

from datetime import datetime

from sqlalchemy import text

from app.blueprint.ace import build_dual_memory_signals, classify_action
from app.blueprint.evals import run_agentic_eval
from app.blueprint.preferences_learning import record_feedback_signal
from app.blueprint.team import build_team_snapshot
from app.blueprint.temporal_orchestration import orchestrate_tier3_plan
from app.api.internal.hands import _validate_side_effect_output
from app.core.config import settings
from app.db.database import SessionLocal


def _ensure_phase2_tables(db) -> None:
    db.execute(
        text(
            """
            create table if not exists feedback_signals (
              id integer primary key autoincrement,
              user_id text,
              signal_type text,
              original_output text,
              corrected_output text,
              context text,
              created_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists behavioral_rules (
              id integer primary key autoincrement,
              user_id text,
              file_path text,
              category text,
              rule_key text,
              rule_value text,
              source text,
              confidence real,
              created_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists contacts (
              id text,
              user_id text,
              name text,
              email text,
              phone text,
              created_at text,
              updated_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists delegations (
              id text,
              user_id text,
              delegate_name text,
              delegate_contact text,
              task_description text,
              status text,
              due_at text,
              created_at text,
              completed_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists profiling_sessions (
              id text,
              user_id text,
              dimension text,
              status text,
              progress_pct real,
              facts_extracted integer,
              created_at text,
              completed_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists knowledge_graph_edges (
              id text,
              user_id text,
              source_node text,
              target_node text,
              relation text,
              weight real,
              metadata text,
              created_at text
            )
            """
        )
    )
    db.commit()


def test_temporal_orchestration_local_plan_when_disabled() -> None:
    original = settings.TEMPORAL_ENABLED
    settings.TEMPORAL_ENABLED = False
    try:
        plan, meta = orchestrate_tier3_plan(
            run_id="run-123",
            user_id="user-123",
            subtasks=[
                "search for candidate vendors",
                "book a follow-up call",
            ],
        )
    finally:
        settings.TEMPORAL_ENABLED = original

    assert meta["engine"] == "local"
    assert len(plan) == 2
    assert plan[0]["tier"] >= 2
    assert plan[1]["tier"] >= 3
    assert all(int(step["retry_limit"]) >= 1 for step in plan)


def test_preference_learning_and_ace_beta_prior_integration() -> None:
    db = SessionLocal()
    try:
        _ensure_phase2_tables(db)
        user_id = "phase2-user-1"
        record_feedback_signal(
            db,
            user_id=user_id,
            signal_type="override",
            original_output="send immediately",
            corrected_output="always ask before sending",
            context={"rule_key": "autonomy.guardrails", "file_path": "AGENTS.md", "category": "correction"},
        )
        record_feedback_signal(
            db,
            user_id=user_id,
            signal_type="complaint",
            original_output="booked too soon",
            corrected_output="confirm booking first",
            context={"rule_key": "autonomy.guardrails", "file_path": "AGENTS.md", "category": "correction"},
        )
        signals = build_dual_memory_signals(db, user_id=user_id)
    finally:
        db.close()

    decision = classify_action(
        "send the update and schedule a meeting",
        agents_content="Always require approval for outbound actions.",
        dual_memory_signals=signals,
    )
    assert decision["requires_approval"] is True
    assert float(decision["approval_probability"]) >= 0.55


def test_team_snapshot_uses_contacts_profiling_and_graph() -> None:
    db = SessionLocal()
    try:
        _ensure_phase2_tables(db)
        now = datetime.utcnow().isoformat()
        user_id = "phase2-user-2"
        db.execute(
            text(
                """
                insert into contacts (user_id, name, email, phone, created_at, updated_at)
                values (:user_id, 'Alex Kim', 'alex@example.com', '+15555550101', :now, :now)
                """
            ),
            {"user_id": user_id, "now": now},
        )
        db.execute(
            text(
                """
                insert into profiling_sessions (id, user_id, dimension, status, progress_pct, facts_extracted, created_at, completed_at)
                values ('p1', :user_id, 'communication_style', 'completed', 100, 4, :now, :now)
                """
            ),
            {"user_id": user_id, "now": now},
        )
        db.execute(
            text(
                """
                insert into knowledge_graph_edges (id, user_id, source_node, target_node, relation, weight, metadata, created_at)
                values ('g1', :user_id, 'person:alex', 'topic:weekly-sync', 'collaborates_on', 0.9, '{}', :now)
                """
            ),
            {"user_id": user_id, "now": now},
        )
        db.commit()
        snapshot = build_team_snapshot(db, user_id=user_id)
    finally:
        db.close()

    assert snapshot["people"]
    assert snapshot["profiling_signals"]
    assert snapshot["interaction_graph"]


def test_side_effect_output_validation_email_send() -> None:
    err = _validate_side_effect_output(
        tool="email.send",
        args={
            "mode": "send",
            "to_email": "ceo@example.com",
            "subject": "Status Update",
        },
        output_payload={
            "status": "sent",
            "recipient": "ceo@example.com",
            "subject": "Status Update",
        },
    )
    assert err is None

    err2 = _validate_side_effect_output(
        tool="email.send",
        args={
            "mode": "send",
            "to_email": "ceo@example.com",
            "subject": "Status Update",
        },
        output_payload={
            "status": "sent",
            "recipient": "wrong@example.com",
            "subject": "Status Update",
        },
    )
    assert "recipient mismatch" in str(err2)


def test_agentic_eval_suite_shape_and_count() -> None:
    result = run_agentic_eval(
        reply_generator=lambda prompt: f"This is your concise plan for {prompt}.",
    )
    assert int(result["scenario_count"]) == 50
    assert 0.0 <= float(result["tier_accuracy"]) <= 1.0
    assert 0.0 <= float(result["action_accuracy"]) <= 1.0
    assert 0.0 <= float(result["personalization_avg"]) <= 1.0
    assert 0.0 <= float(result["overall_score"]) <= 1.0
