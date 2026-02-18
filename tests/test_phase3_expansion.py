from __future__ import annotations

from datetime import datetime, timedelta

from sqlalchemy import text

from app.blueprint.knowledge_review import run_nightly_consolidation, run_weekly_self_review
from app.blueprint.mcp.mock_servers import build_echo_manifest
from app.blueprint.mcp.registry import MCPServerRegistry
from app.blueprint.research import create_research_job, run_research_job
from app.blueprint.workflows import create_workflow, dry_run_workflow, parse_nl_workflow_definition
from app.db.database import SessionLocal


def _ensure_phase3_tables(db) -> None:
    db.execute(
        text(
            """
            create table if not exists knowledge_files (
              id text,
              user_id text,
              file_path text,
              layer text,
              content text,
              content_hash text,
              token_count integer,
              metadata text,
              version integer,
              created_at text,
              updated_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists workflows (
              id text primary key,
              user_id text,
              name text,
              description text,
              trigger_type text,
              trigger_config text,
              actions text,
              status text,
              last_run_at text,
              next_run_at text,
              run_count integer,
              error_count integer,
              last_error text,
              created_at text,
              updated_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists research_jobs (
              id text primary key,
              user_id text,
              title text,
              query text,
              sources text,
              schedule text,
              status text,
              last_run_at text,
              next_run_at text,
              findings text,
              delivery_channel text,
              delivery_format text,
              max_cost_per_run real,
              total_cost real,
              created_at text,
              updated_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists mcp_servers (
              id integer primary key autoincrement,
              server_id text unique,
              display_name text,
              description text,
              transport_json text,
              tags_json text,
              expected_tools_json text,
              expected_resources_json text,
              expected_prompts_json text,
              tools_json text,
              resources_json text,
              prompts_json text,
              state text,
              rate_limit_per_min integer,
              daily_budget_cents integer,
              health_status text,
              consecutive_failures integer,
              total_calls integer,
              total_errors integer,
              total_cost_cents real,
              last_health_check_at text,
              updated_at text,
              created_at text
            )
            """
        )
    )
    db.execute(
        text(
            """
            create table if not exists mcp_user_servers (
              id integer primary key autoincrement,
              user_id text,
              server_id text,
              is_enabled integer,
              custom_config_json text,
              daily_budget_override real,
              created_at text,
              unique(user_id, server_id)
            )
            """
        )
    )
    db.commit()


def test_workflow_nl_create_and_dry_run() -> None:
    db = SessionLocal()
    try:
        _ensure_phase3_tables(db)
        user_id = "phase3-user-workflow"
        db.execute(
            text(
                """
                insert into knowledge_files (id, user_id, file_path, layer, content, content_hash, token_count, metadata, version, created_at, updated_at)
                values ('kf-1', :user_id, 'WORKFLOWS.md', 'dna', '# WORKFLOWS.md\\n-', 'hash', 1, '{}', 1, :now, :now)
                """
            ),
            {"user_id": user_id, "now": datetime.utcnow().isoformat()},
        )
        db.commit()
        definition = parse_nl_workflow_definition(
            "Daily investor update\nSearch market news\nSend email summary"
        )
        created = create_workflow(db, user_id=user_id, definition=definition, status="active")
        dry = dry_run_workflow(db, user_id=user_id, workflow_id=str(created["id"]), sample_inputs={"topic": "markets"})
    finally:
        db.close()

    assert str(created["name"]).lower().startswith("daily investor")
    assert dry["dry_run"] is True
    assert len(dry["steps"]) >= 1


def test_research_job_run_with_stubbed_tavily() -> None:
    import app.blueprint.research as research_mod

    async def _fake_search(query: str, **_: object):
        return {
            "answer": f"summary for {query}",
            "results": [
                {"title": "A", "url": "https://example.com/a", "content": "alpha", "score": 0.9},
                {"title": "B", "url": "https://example.com/b", "content": "beta", "score": 0.7},
            ],
        }

    original = research_mod.tavily_search
    research_mod.tavily_search = _fake_search
    db = SessionLocal()
    try:
        _ensure_phase3_tables(db)
        user_id = "phase3-user-research"
        db.execute(
            text(
                """
                insert into knowledge_files (id, user_id, file_path, layer, content, content_hash, token_count, metadata, version, created_at, updated_at)
                values ('kf-2', :user_id, 'HEARTBEAT.md', 'brain', '# HEARTBEAT.md\\n## Delegation Tracker\\n-', 'hash', 1, '{}', 1, :now, :now)
                """
            ),
            {"user_id": user_id, "now": datetime.utcnow().isoformat()},
        )
        db.commit()
        created = create_research_job(
            db,
            user_id=user_id,
            title="Competitor watch",
            query="top AI executive assistants",
            sources=["web"],
            schedule="daily",
        )
        result = run_research_job(db, user_id=user_id, research_id=str(created["id"]))
    finally:
        research_mod.tavily_search = original
        db.close()

    assert result["findings_count"] == 2
    assert "summary for" in result["summary"]


def test_knowledge_consolidation_and_self_review() -> None:
    db = SessionLocal()
    try:
        _ensure_phase3_tables(db)
        user_id = "phase3-user-review"
        now = datetime.utcnow().isoformat()
        old = (datetime.utcnow() - timedelta(days=30)).isoformat()
        db.execute(
            text(
                """
                insert into knowledge_files (id, user_id, file_path, layer, content, content_hash, token_count, metadata, version, created_at, updated_at)
                values
                ('k1', :user_id, 'USER.md', 'brain', '- timezone: PST', 'hash1', 1, '{}', 1, :now, :old),
                ('k2', :user_id, 'MEMORY.md', 'dna', '- timezone: EST', 'hash2', 1, '{}', 1, :now, :old),
                ('k3', :user_id, 'HEARTBEAT.md', 'brain', '# HEARTBEAT.md', 'hash3', 1, '{}', 1, :now, :now)
                """
            ),
            {"user_id": user_id, "now": now, "old": old},
        )
        db.commit()
        consolidation = run_nightly_consolidation(db, user_id=user_id)
        review = run_weekly_self_review(db, user_id=user_id)
    finally:
        db.close()

    assert consolidation["ok"] is True
    assert consolidation["contradiction_count"] >= 1
    assert review["ok"] is True
    assert review["missing_files"]


def test_mcp_registry_sqlite_manifest_roundtrip() -> None:
    db = SessionLocal()
    try:
        _ensure_phase3_tables(db)
        registry = MCPServerRegistry()
        manifest = build_echo_manifest(server_id="mcp-echo-phase3")
        config = registry.upsert_server(db, manifest)
        registry.bind_user_server(db, user_id="phase3-user", server_id=config.server_id)
        listed = registry.list_servers(db, user_id="phase3-user")
    finally:
        db.close()

    assert config.server_id == "mcp-echo-phase3"
    assert listed
