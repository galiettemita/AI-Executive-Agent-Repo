from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

from app.db.database import SessionLocal
from app.services.prompt_versions import create_prompt_version, ensure_prompt_versions_table, rollback_prompt, select_prompt_version

REPORT_PATH = Path("docs/reports/phase4_prompt_ab_eval.md")


def run() -> dict[str, object]:
    db = SessionLocal()
    try:
        ensure_prompt_versions_table(db)
        prompt_group = "system_prompt"
        control_id = create_prompt_version(
            db,
            prompt_group=prompt_group,
            version_label="phase4-control",
            content="You are Executive OS. Be clear and concise.",
            status="active",
            rollout_percentage=100,
            metadata={"phase": "phase4", "variant": "control"},
        )
        candidate_id = create_prompt_version(
            db,
            prompt_group=prompt_group,
            version_label="phase4-candidate",
            content="You are Executive OS. Prefer explicit tool-planning and compact execution summaries.",
            status="canary",
            rollout_percentage=20,
            metadata={"phase": "phase4", "variant": "candidate"},
        )

        sample_users = [f"ab-user-{i}" for i in range(1, 51)]
        assigned_candidate = 0
        for user_id in sample_users:
            selected = select_prompt_version(
                db,
                user_id=user_id,
                prompt_group=prompt_group,
                default_content="fallback",
            )
            if str(selected.get("prompt_version_id") or "") == candidate_id:
                assigned_candidate += 1

        rolled_to = rollback_prompt(db, prompt_group=prompt_group, target_version_id=control_id)
    finally:
        db.close()

    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "prompt_group": "system_prompt",
        "control_id": control_id,
        "candidate_id": candidate_id,
        "sample_users": len(sample_users),
        "candidate_assigned": assigned_candidate,
        "candidate_assignment_rate": round((assigned_candidate / max(1, len(sample_users))) * 100.0, 2),
        "rollback_target": rolled_to,
        "ok": bool(rolled_to == control_id),
    }


def main() -> None:
    payload = run()
    lines = [
        "# Phase 4 Prompt A/B Eval",
        "",
        f"Generated at: {payload['generated_at']}",
        f"Prompt group: `{payload['prompt_group']}`",
        f"Control version: `{payload['control_id']}`",
        f"Candidate version: `{payload['candidate_id']}`",
        f"Sample users: {payload['sample_users']}",
        f"Candidate assignments: {payload['candidate_assigned']} ({payload['candidate_assignment_rate']}%)",
        f"Rollback target: `{payload['rollback_target']}`",
        f"Status: {'PASS' if payload['ok'] else 'FAIL'}",
        "",
        "This validates canary assignment and rollback mechanics for prompt versioning.",
    ]
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()
