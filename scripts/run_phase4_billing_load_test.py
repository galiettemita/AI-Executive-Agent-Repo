from __future__ import annotations

import concurrent.futures
import json
from datetime import datetime, timezone
from pathlib import Path

from app.db.database import SessionLocal
from app.services.billing_middleware import enforce_billing_for_inbound_message

REPORT_PATH = Path("docs/reports/phase4_billing_load_test.json")


def _single_check(user_id: str) -> dict[str, object]:
    db = SessionLocal()
    try:
        decision = enforce_billing_for_inbound_message(db, user_id)
        return {
            "allowed": bool(decision.allowed),
            "plan": str(decision.plan),
            "status": str(decision.status),
            "block_reason": str((decision.block.reason if decision.block else "") or ""),
        }
    except Exception as exc:  # pragma: no cover - script mode
        return {"allowed": False, "plan": "unknown", "status": "error", "block_reason": str(exc)}
    finally:
        db.close()


def run(*, users: int = 400, checks_per_user: int = 5, max_workers: int = 32) -> dict[str, object]:
    work = []
    for idx in range(users):
        for attempt in range(checks_per_user):
            user_id = f"phase4-billing-user-{idx}-{attempt}"
            work.append(user_id)

    allowed = 0
    blocked = 0
    by_reason: dict[str, int] = {}

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as pool:
        futures = [pool.submit(_single_check, user_id) for user_id in work]
        for fut in concurrent.futures.as_completed(futures):
            row = fut.result()
            if bool(row.get("allowed")):
                allowed += 1
            else:
                blocked += 1
                reason = str(row.get("block_reason") or "unknown")
                by_reason[reason] = int(by_reason.get(reason) or 0) + 1

    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "users": users,
        "checks_per_user": checks_per_user,
        "total_checks": len(work),
        "allowed": allowed,
        "blocked": blocked,
        "blocked_by_reason": by_reason,
        "ok": allowed > 0,
    }


def main() -> None:
    payload = run()
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()
