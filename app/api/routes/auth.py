from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.services.pairing_service import pair_with_code

router = APIRouter(prefix="/auth", tags=["auth"])


class PairRequest(BaseModel):
    code: str


@router.post("/pair")
def pair_device(payload: PairRequest, db: Session = Depends(get_db)):
    result = pair_with_code(db, payload.code)
    if not result:
        raise HTTPException(status_code=400, detail="Invalid or expired pairing code")
    return result
