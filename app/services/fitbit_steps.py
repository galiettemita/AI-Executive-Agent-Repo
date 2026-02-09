from __future__ import annotations

from datetime import date, datetime
from typing import Any, Dict, Optional

import httpx
from sqlalchemy.orm import Session

from app.db.models import FitnessStepLog
from app.services.fitbit_oauth import get_valid_fitbit_access_token


def _fitbit_steps_url(step_date: date) -> str:
    date_str = step_date.isoformat()
    return f"https://api.fitbit.com/1/user/-/activities/steps/date/{date_str}/1d.json"


def _parse_steps(payload: Dict[str, Any]) -> int:
    steps_list = payload.get("activities-steps") or []
    if not steps_list:
        return 0
    first = steps_list[0]
    value = first.get("value")
    try:
        return int(float(value)) if value is not None else 0
    except Exception:
        return 0


def fetch_fitbit_steps(access_token: str, step_date: date) -> int:
    headers = {"Authorization": f"Bearer {access_token}"}
    url = _fitbit_steps_url(step_date)
    resp = httpx.get(url, headers=headers, timeout=10.0)
    if resp.status_code >= 400:
        raise RuntimeError(f"Fitbit steps fetch failed: {resp.text}")
    payload = resp.json()
    return _parse_steps(payload)


def get_step_log(db: Session, user_id: str, step_date: date) -> Optional[FitnessStepLog]:
    return (
        db.query(FitnessStepLog)
        .filter(FitnessStepLog.user_id == user_id, FitnessStepLog.step_date == step_date)
        .one_or_none()
    )


def upsert_step_log(db: Session, user_id: str, step_date: date, steps: int, source: str = "fitbit") -> FitnessStepLog:
    row = get_step_log(db, user_id, step_date)
    if row is None:
        row = FitnessStepLog(user_id=user_id, step_date=step_date)
        db.add(row)

    row.steps = steps
    row.source = source
    row.updated_at = datetime.utcnow()
    if not row.created_at:
        row.created_at = datetime.utcnow()

    db.commit()
    db.refresh(row)
    return row


def get_daily_steps(
    db: Session,
    user_id: str,
    step_date: date,
    refresh: bool = False,
) -> Dict[str, Any]:
    row = get_step_log(db, user_id, step_date)
    if row and not refresh:
        return {
            "date": step_date.isoformat(),
            "steps": row.steps,
            "source": row.source,
            "cached": True,
        }

    token = get_valid_fitbit_access_token(db, user_id)
    if not token:
        raise RuntimeError("Fitbit not connected")

    steps = fetch_fitbit_steps(token, step_date)
    row = upsert_step_log(db, user_id, step_date, steps, source="fitbit")
    return {
        "date": step_date.isoformat(),
        "steps": row.steps,
        "source": row.source,
        "cached": False,
    }
