from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

REPORT_PATH = Path("docs/reports/phase4_dr_readiness.md")

REQUIRED_PATHS = [
    Path("docs/DR_PLAN.md"),
    Path("docs/DR_OPERATIONAL_RUNBOOKS.md"),
    Path("docs/DR_DRILL_RUNBOOK.md"),
    Path("docs/BACKUP_STRATEGY.md"),
    Path("scripts/dr_restore_drill.py"),
    Path(".github/workflows/dr-restore-drill.yml"),
]


def run() -> dict[str, object]:
    rows = []
    for rel_path in REQUIRED_PATHS:
        exists = rel_path.exists()
        rows.append({"path": str(rel_path), "exists": exists})

    runbook_text = Path("docs/DR_DRILL_RUNBOOK.md").read_text(encoding="utf-8") if Path("docs/DR_DRILL_RUNBOOK.md").exists() else ""
    runbook_text_norm = runbook_text.lower()
    has_required_keywords = all(
        token in runbook_text_norm
        for token in (
            "dr restore drill",
            "runbook",
            "restore",
        )
    )

    ok = all(bool(item.get("exists")) for item in rows) and has_required_keywords
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "rows": rows,
        "runbook_keyword_check": has_required_keywords,
        "ok": ok,
    }


def main() -> None:
    payload = run()
    lines = [
        "# Phase 4 DR Readiness Report",
        "",
        f"Generated at: {payload.get('generated_at')}",
        "",
        "| Artifact | Present |",
        "|---|---:|",
    ]
    for row in payload.get("rows") or []:
        lines.append(f"| `{row.get('path')}` | {'yes' if row.get('exists') else 'no'} |")
    lines.extend(
        [
            "",
            f"Runbook keyword check: {'PASS' if payload.get('runbook_keyword_check') else 'FAIL'}",
            f"Status: {'PASS' if payload.get('ok') else 'FAIL'}",
            "",
            "This verifies DR restore drill automation + runbook artifacts are present and launch-ready in-repo.",
        ]
    )

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()
