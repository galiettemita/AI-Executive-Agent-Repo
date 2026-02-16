from __future__ import annotations

from typing import Any

from sqlalchemy import text

from app.db.database import SessionLocal
from app.blueprint.tools import get_tool_registry


KNOWLEDGE_PRIORITY = [
    "IDENTITY.md",
    "SOUL.md",
    "AGENTS.md",
    "USER.md",
    "HEARTBEAT.md",
    "TOOLS.md",
    "TEAM.md",
    "WORKFLOWS.md",
    "MEMORY.md",
]


def _approx_tokens(text_value: str) -> int:
    return max(1, len((text_value or "").strip()) // 4)


def _latest_knowledge_by_file(user_id: str) -> dict[str, str]:
    db = SessionLocal()
    try:
        rows = db.execute(
            text(
                """
                select file_path, content, version
                from knowledge_files
                where user_id = :user_id
                order by file_path asc, version desc
                """
            ),
            {"user_id": user_id},
        ).mappings().all()
    except Exception:
        return {}
    finally:
        try:
            db.close()
        except Exception:
            pass

    seen: set[str] = set()
    out: dict[str, str] = {}
    for row in rows:
        file_path = str(row.get("file_path") or "").strip()
        if not file_path or file_path in seen:
            continue
        seen.add(file_path)
        content = str(row.get("content") or "").strip()
        if content:
            out[file_path] = content
    return out


def compile_context_messages(
    *,
    base_system_prompt: str,
    user_id: str | None,
    user_text: str,
    history_messages: list[dict[str, Any]] | None,
    tier: int,
) -> tuple[list[dict[str, Any]], list[str], list[dict[str, str]]]:
    """
    Phase-1 Context Compiler:
    - Build a dynamic system prompt
    - Inject highest-priority knowledge files within a token budget
    """
    history = history_messages or []
    max_context_tokens = 1000 if tier <= 1 else 1800

    injected_files: list[str] = []
    system_parts: list[str] = [base_system_prompt.strip()]
    used_tokens = _approx_tokens(system_parts[0])
    context_chunks: list[dict[str, str]] = [
        {
            "role": "system",
            "source": "base_system_prompt",
            "provenance": "system",
        }
    ]

    if user_id:
        latest = _latest_knowledge_by_file(user_id)
        for file_path in KNOWLEDGE_PRIORITY:
            content = latest.get(file_path)
            if not content:
                continue
            block = f"[{file_path}]\n{content}"
            block_tokens = _approx_tokens(block)
            if used_tokens + block_tokens > max_context_tokens:
                continue
            system_parts.append(block)
            injected_files.append(file_path)
            used_tokens += block_tokens
            context_chunks.append(
                {
                    "role": "system",
                    "source": file_path,
                    "provenance": "knowledge_file",
                }
            )

    system_text = "\n\n".join(system_parts).strip()
    messages: list[dict[str, Any]] = [{"role": "system", "content": system_text}]
    if history:
        for item in history:
            role = str(item.get("role") or "user")
            content = str(item.get("content") or "")
            if not content.strip():
                continue
            messages.append({"role": role, "content": content})
            provenance = str(item.get("provenance") or ("user_direct" if role == "user" else "assistant_output"))
            source = str(item.get("source") or "conversation_history")
            context_chunks.append({"role": role, "source": source, "provenance": provenance})

    messages.append({"role": "user", "content": user_text})
    context_chunks.append({"role": "user", "source": "latest_user_message", "provenance": "user_direct"})
    return messages, injected_files, context_chunks


def compile_tool_schemas(*, tier: int, tags: list[str] | None = None) -> list[dict[str, Any]]:
    """
    Build tool schemas for the active reasoning tier from the shared registry.
    """
    registry = get_tool_registry()
    return registry.list_llm_tool_schemas(tier=tier, tags=tags)
