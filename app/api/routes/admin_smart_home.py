# backend/app/api/routes/admin_smart_home.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.integration_credentials import (
    upsert_integration_credential,
    delete_integration_credential,
    get_connection_status,
)
from app.services.smart_home.providers.registry import get_provider

router = APIRouter(prefix="/admin/smart_home", tags=["admin"])


class SmartHomeConnectRequest(BaseModel):
    user_id: str
    provider: str = "home_assistant"
    base_url: str
    token: str
    verify: bool = True


@router.post("/connect")
def connect_smart_home(
    payload: SmartHomeConnectRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    upsert_integration_credential(
        db=db,
        user_id=payload.user_id,
        provider=payload.provider,
        username=None,
        secret=payload.token,
        server_url=payload.base_url,
        metadata=None,
    )

    if payload.verify:
        try:
            provider = get_provider(db, payload.provider)
            provider.discover_devices(payload.user_id)
        except Exception as e:
            raise HTTPException(status_code=400, detail=str(e))

    return {"ok": True}


@router.get("/status")
def smart_home_status(
    user_id: str,
    provider: str = "home_assistant",
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    return get_connection_status(db=db, user_id=user_id, provider=provider)


@router.post("/disconnect")
def smart_home_disconnect(
    user_id: str,
    provider: str = "home_assistant",
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    delete_integration_credential(db=db, user_id=user_id, provider=provider)
    return {"ok": True}
