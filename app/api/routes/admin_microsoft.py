# backend/app/api/routes/admin_microsoft.py

from __future__ import annotations

import logging
from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.microsoft_oauth import (
    build_microsoft_auth_url,
    exchange_code_and_store_tokens,
    get_microsoft_connection_status,
    disconnect_microsoft,
)

router = APIRouter(prefix="/admin/microsoft", tags=["admin"])
logger = logging.getLogger(__name__)


@router.get("/connect")
def microsoft_connect(
    user_id: str = Query(..., description="For WhatsApp MVP, pass the phone number as user_id."),
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    url = build_microsoft_auth_url(user_id=user_id)
    return {"auth_url": url}


@router.get("/callback")
def microsoft_callback(
    code: str,
    state: str,
    db: Session = Depends(get_db),
):
    try:
        user_id = exchange_code_and_store_tokens(db=db, code=code, state=state)
    except Exception as e:
        logger.error("MICROSOFT CALLBACK ERROR: %s", repr(e))
        raise HTTPException(status_code=400, detail=str(e))
    return {
        "ok": True,
        "message": "Connected! You can now manage Outlook email and calendar.",
        "user_id": user_id,
    }


@router.get("/status")
def microsoft_status(
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    return get_microsoft_connection_status(db=db, user_id=user_id)


@router.post("/disconnect")
def microsoft_disconnect(
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    disconnect_microsoft(db=db, user_id=user_id)
    return {"ok": True}
