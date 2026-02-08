# backend/app/api/routes/admin_beta.py

from __future__ import annotations

from typing import Optional, List

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.middleware.rate_limiter import rate_limit_user
from app.services.beta_service import (
    list_beta_testers,
    upsert_beta_tester,
    upsert_beta_testers_bulk,
    summarize_beta_testers,
    delete_beta_tester,
)

router = APIRouter(prefix="/admin/beta", tags=["admin"])


class BetaTesterCreate(BaseModel):
    user_id: str
    email: Optional[str] = None
    status: Optional[str] = "active"
    notes: Optional[str] = None


class BetaTesterBulkItem(BaseModel):
    user_id: str
    email: Optional[str] = None
    status: Optional[str] = "active"
    notes: Optional[str] = None


class BetaTesterBulkRequest(BaseModel):
    testers: List[BetaTesterBulkItem]


@rate_limit_user()
@router.post("/testers")
def add_beta_tester(request: Request, payload: BetaTesterCreate, db: Session = Depends(get_db)):
    tester = upsert_beta_tester(
        db=db,
        user_id=payload.user_id,
        email=payload.email,
        status=payload.status or "active",
        notes=payload.notes,
    )
    return {
        "ok": True,
        "tester": {
            "id": tester.id,
            "user_id": tester.user_id,
            "email": tester.email,
            "status": tester.status,
            "notes": tester.notes,
            "created_at": tester.created_at.isoformat() if tester.created_at else None,
            "updated_at": tester.updated_at.isoformat() if tester.updated_at else None,
        },
    }


@rate_limit_user()
@router.get("/testers")
def list_beta_testers_endpoint(
    request: Request,
    status: Optional[str] = None,
    limit: int = 200,
    db: Session = Depends(get_db),
):
    rows = list_beta_testers(db, status=status, limit=limit)
    return {
        "items": [
            {
                "id": r.id,
                "user_id": r.user_id,
                "email": r.email,
                "status": r.status,
                "notes": r.notes,
                "created_at": r.created_at.isoformat() if r.created_at else None,
                "updated_at": r.updated_at.isoformat() if r.updated_at else None,
            }
            for r in rows
        ]
    }


@rate_limit_user()
@router.post("/testers/bulk")
def bulk_add_beta_testers(
    request: Request,
    payload: BetaTesterBulkRequest,
    db: Session = Depends(get_db),
):
    items = [item.model_dump() for item in payload.testers]
    count = upsert_beta_testers_bulk(db, items)
    return {"ok": True, "count": count}


@rate_limit_user()
@router.get("/summary")
def beta_tester_summary(request: Request, db: Session = Depends(get_db)):
    return {"ok": True, "summary": summarize_beta_testers(db)}


@rate_limit_user()
@router.delete("/testers/{tester_id}")
def remove_beta_tester(request: Request, tester_id: int, db: Session = Depends(get_db)):
    ok = delete_beta_tester(db, tester_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Beta tester not found")
    return {"ok": True}
