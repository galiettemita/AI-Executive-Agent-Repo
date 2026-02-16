from __future__ import annotations

from fastapi import APIRouter, Depends, Query, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.api.routes import webhooks_whatsapp


router = APIRouter(prefix="/webhook", tags=["webhooks"])


@router.get("/whatsapp")
def verify_whatsapp_alias(
    hub_mode: str = Query("", alias="hub.mode"),
    hub_challenge: str = Query("", alias="hub.challenge"),
    hub_verify_token: str = Query("", alias="hub.verify_token"),
):
    return webhooks_whatsapp.verify_webhook(
        hub_mode=hub_mode,
        hub_challenge=hub_challenge,
        hub_verify_token=hub_verify_token,
    )


@router.post("/whatsapp")
async def receive_whatsapp_alias(request: Request, db: Session = Depends(get_db)):
    return await webhooks_whatsapp.receive(request=request, db=db)
