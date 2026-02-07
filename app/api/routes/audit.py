from __future__ import annotations

from fastapi import APIRouter, Depends, Request
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import AuditLog
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter(prefix="/audit", tags=["audit"])


@rate_limit_user()
@router.get("")
def list_audit_logs(
    request: Request,
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    rows = (
        db.query(AuditLog)
        .filter(AuditLog.user_id == user_id)
        .order_by(AuditLog.created_at.desc())
        .limit(limit)
        .all()
    )
    return {
        "items": [
            {
                "id": r.id,
                "action": r.action,
                "method": r.method,
                "path": r.path,
                "status_code": r.status_code,
                "created_at": r.created_at.isoformat() if r.created_at else None,
            }
            for r in rows
        ]
    }
