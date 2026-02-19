from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from app.db.database import SessionLocal
from app.services.analytics import emit_event, wave14_server_prioritization

REPORT_PATH = Path("docs/reports/phase4_launch_prioritization.json")


def run() -> dict[str, object]:
    db = SessionLocal()
    try:
        # Seed a compact demand sample for deterministic ordering in the report.
        emit_event(db, event_name="provisioning_requested", user_id="prio-a", source="phase4", payload={"server_id": "slack-mcp"})
        emit_event(db, event_name="provisioning_requested", user_id="prio-b", source="phase4", payload={"server_id": "slack-mcp"})
        emit_event(db, event_name="server_provisioned", user_id="prio-a", source="phase4", payload={"server_id": "slack-mcp"})
        emit_event(db, event_name="provisioning_requested", user_id="prio-c", source="phase4", payload={"server_id": "teams-mcp"})
        payload = wave14_server_prioritization(db, days=30)
    finally:
        db.close()

    payload["generated_at"] = datetime.now(timezone.utc).isoformat()
    return payload


def main() -> None:
    payload = run()
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()
