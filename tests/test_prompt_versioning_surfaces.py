from __future__ import annotations

import json
import uuid

from sqlalchemy import text

from app.blueprint.context_compiler import compile_context_messages, compile_tool_schemas
from app.blueprint.contracts import LLMProvider, LLMResponse, TokenUsage
from app.blueprint.knowledge_files import ensure_default_knowledge_files
from app.db.database import SessionLocal
from app.services import content_safety, quality_eval
from app.services.prompt_versions import create_prompt_version, ensure_prompt_versions_table


def _ensure_knowledge_files_table(db) -> None:
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
              version integer,
              metadata text,
              created_at text,
              updated_at text
            )
            """
        )
    )
    db.commit()


def test_knowledge_template_prompt_version_is_applied() -> None:
    user_id = f"pv-knowledge-{uuid.uuid4().hex[:8]}"
    db = SessionLocal()
    try:
        _ensure_knowledge_files_table(db)
        ensure_prompt_versions_table(db)
        create_prompt_version(
            db,
            prompt_group="knowledge_template:SOUL.md",
            version_label="v-knowledge-soul",
            content="# SOUL.md\n## Character\n- Calm, direct, and highly actionable.\n",
            status="active",
            rollout_percentage=100,
        )

        inserted = ensure_default_knowledge_files(
            db,
            user_id=user_id,
            display_name="Prompt User",
            timezone="UTC",
        )
        assert "SOUL.md" in inserted

        row = db.execute(
            text(
                "select content from knowledge_files "
                "where user_id = :user_id and file_path = 'SOUL.md' "
                "order by version desc limit 1"
            ),
            {"user_id": user_id},
        ).mappings().first()
    finally:
        db.close()

    assert row is not None
    assert "calm, direct, and highly actionable" in str(row.get("content") or "").lower()


def test_context_compiler_template_prompt_version_is_applied() -> None:
    db = SessionLocal()
    try:
        ensure_prompt_versions_table(db)
        create_prompt_version(
            db,
            prompt_group="context_compiler_template",
            version_label=f"v-context-{uuid.uuid4().hex[:6]}",
            content="[[CTX]]\n{base_system_prompt}\n[[KNOW]]\n{knowledge_blocks}\n[[END]]",
            status="active",
            rollout_percentage=100,
        )
    finally:
        db.close()

    messages, _injected, _chunks = compile_context_messages(
        base_system_prompt="BASE SYSTEM PROMPT",
        user_id=None,
        user_text="hello",
        history_messages=None,
        tier=1,
    )
    system_text = str(messages[0].get("content") or "")
    assert system_text.startswith("[[CTX]]")
    assert "BASE SYSTEM PROMPT" in system_text
    assert "[[END]]" in system_text


def test_tool_description_prompt_version_is_applied() -> None:
    db = SessionLocal()
    try:
        ensure_prompt_versions_table(db)
        create_prompt_version(
            db,
            prompt_group="tool_description:web.search",
            version_label=f"v-tool-desc-{uuid.uuid4().hex[:6]}",
            content="Custom web search description for rollout verification.",
            status="active",
            rollout_percentage=100,
        )
    finally:
        db.close()

    schemas = compile_tool_schemas(tier=2, user_id=f"pv-tools-{uuid.uuid4().hex[:6]}")
    web_schema = next(
        schema for schema in schemas if str(((schema.get("function") or {}).get("name") or "")) == "web_search"
    )
    description = str(((web_schema.get("function") or {}).get("description") or ""))
    assert description == "Custom web search description for rollout verification."


def test_evaluator_prompt_groups_are_explicit(monkeypatch) -> None:
    seen_groups: list[str] = []

    class _DummyRouter:
        def call(self, req):
            seen_groups.append(str(req.prompt_group or ""))
            if "safety" in str(req.prompt_group or ""):
                content = json.dumps(
                    {
                        "risk_score": 0.1,
                        "categories": [],
                        "reason": "ok",
                        "flagged": False,
                    }
                )
            else:
                content = json.dumps(
                    {
                        "coherence": 4.1,
                        "helpfulness": 4.0,
                        "safety": 4.6,
                        "tool_usage": 3.8,
                    }
                )
            return LLMResponse(
                provider=LLMProvider.OPENAI,
                model="gpt-4o-mini",
                content=content,
                usage=TokenUsage(input_tokens=10, output_tokens=10, total_tokens=20),
            )

    monkeypatch.setattr(quality_eval, "get_llm_router", lambda: _DummyRouter())
    monkeypatch.setattr(content_safety, "get_llm_router", lambda: _DummyRouter())

    _ = quality_eval._llm_scores(
        user_text="Summarize my plan",
        assistant_text="Here is the summary and next steps.",
        used_tools=True,
    )
    _ = content_safety._llm_classify("please summarize this safely")

    assert "evaluator_prompt_quality" in seen_groups
    assert "evaluator_prompt_safety" in seen_groups
