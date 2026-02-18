from __future__ import annotations

import re
from datetime import datetime
from typing import Any

from sqlalchemy.orm import Session

from app.blueprint.knowledge_files import list_knowledge_files, put_knowledge_file_version

_KV_LINE = re.compile(r"^\s*[-*]?\s*([A-Za-z0-9 _/-]{2,80})\s*:\s*(.+)\s*$")


def _extract_key_values(content: str, *, file_path: str) -> list[tuple[str, str, str]]:
    out: list[tuple[str, str, str]] = []
    for raw in str(content or "").splitlines():
        m = _KV_LINE.match(raw)
        if not m:
            continue
        key = m.group(1).strip().lower()
        value = m.group(2).strip()
        out.append((key, value, file_path))
    return out


def run_nightly_consolidation(db: Session, *, user_id: str) -> dict[str, Any]:
    files = list_knowledge_files(db, user_id=user_id)
    by_key: dict[str, list[tuple[str, str]]] = {}
    for item in files:
        file_path = str(item.get("file_path") or "")
        content = str(item.get("content") or "")
        for key, value, src in _extract_key_values(content, file_path=file_path):
            by_key.setdefault(key, []).append((value, src))

    contradictions: list[dict[str, Any]] = []
    for key, values in by_key.items():
        uniq = {}
        for value, src in values:
            uniq.setdefault(value.lower(), set()).add(src)
        if len(uniq) > 1:
            contradictions.append(
                {
                    "key": key,
                    "values": [
                        {"value": val, "sources": sorted(list(srcs))}
                        for val, srcs in uniq.items()
                    ],
                }
            )

    merged_lines = [
        "# CONSOLIDATION.md",
        f"_Generated: {datetime.utcnow().isoformat()}_",
        "",
        "## Contradictions",
    ]
    if contradictions:
        for item in contradictions:
            merged_lines.append(f"- {item['key']}")
            for value in item["values"]:
                merged_lines.append(f"  - {value['value']} (sources: {', '.join(value['sources'])})")
    else:
        merged_lines.append("- No contradictions detected.")

    merged_lines.extend(["", "## Consolidation Actions", "- Keep latest source of truth where confidence is higher.", "- Ask user for clarification on conflicts."])
    content = "\n".join(merged_lines).strip()
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="CONSOLIDATION.md",
        content=content,
        metadata={"source": "nightly_consolidation", "contradiction_count": len(contradictions)},
    )
    return {"ok": True, "contradictions": contradictions, "contradiction_count": len(contradictions)}


def run_weekly_self_review(db: Session, *, user_id: str) -> dict[str, Any]:
    files = list_knowledge_files(db, user_id=user_id)
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
    present = {str(item.get("file_path") or "") for item in files}
    missing = [name for name in required if name not in present]
    stale: list[str] = []
    now = datetime.utcnow()
    for item in files:
        updated = item.get("updated_at")
        if isinstance(updated, datetime):
            if (now - updated).days >= 14:
                stale.append(str(item.get("file_path") or ""))

    questions: list[str] = []
    if missing:
        questions.append("Which missing knowledge files should be prioritized this week?")
    if stale:
        questions.append("Should I refresh stale files based on your recent behavior?")
    questions.extend(
        [
            "What high-value workflow should I automate next?",
            "Which teammate interactions are most important this week?",
        ]
    )

    lines = [
        "# SELF_REVIEW.md",
        f"_Generated: {now.isoformat()}_",
        "",
        "## Missing Files",
    ]
    if missing:
        lines.extend([f"- {name}" for name in missing])
    else:
        lines.append("- None")
    lines.extend(["", "## Stale Files (>14 days)"])
    stale_unique = sorted(set(stale))
    if stale_unique:
        lines.extend([f"- {name}" for name in stale_unique])
    else:
        lines.append("- None")
    lines.extend(["", "## Suggested Questions"])
    lines.extend([f"- {q}" for q in questions])
    content = "\n".join(lines).strip()
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="SELF_REVIEW.md",
        content=content,
        metadata={"source": "weekly_self_review", "missing_count": len(missing), "stale_count": len(set(stale))},
    )
    return {"ok": True, "missing_files": missing, "stale_files": stale_unique, "questions": questions}
