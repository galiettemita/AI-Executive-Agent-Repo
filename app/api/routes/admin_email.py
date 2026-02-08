from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.integration_credentials import (
    upsert_integration_credential,
    delete_integration_credential,
    get_connection_status,
)
from app.services.imap_email import DEFAULT_PROVIDER_SETTINGS, verify_imap_connection

router = APIRouter(prefix="/admin/email", tags=["admin"])


class EmailConnectRequest(BaseModel):
    user_id: str = Field(..., description="User ID (WhatsApp phone number or internal user ID).")
    provider: str = Field(..., description="Email provider: icloud or yahoo")
    email: str = Field(..., description="Email address / username")
    password: str = Field(..., description="App-specific password")

    imap_host: Optional[str] = None
    imap_port: Optional[int] = None
    imap_ssl: Optional[bool] = True

    smtp_host: Optional[str] = None
    smtp_port: Optional[int] = None
    smtp_ssl: Optional[bool] = False

    smtp_user: Optional[str] = None
    from_email: Optional[str] = None
    mailbox: Optional[str] = None

    verify: bool = Field(False, description="Verify credentials by connecting to IMAP.")


@router.post("/connect")
def email_connect(req: EmailConnectRequest, db: Session = Depends(get_db)):
    provider = (req.provider or "").lower().strip()
    if provider not in {"icloud", "yahoo"}:
        raise HTTPException(status_code=400, detail="Provider must be icloud or yahoo")

    get_or_create_user(db, req.user_id)

    defaults = DEFAULT_PROVIDER_SETTINGS.get(provider, {})
    metadata = {
        "imap_host": req.imap_host or defaults.get("imap_host"),
        "imap_port": req.imap_port or defaults.get("imap_port"),
        "imap_ssl": req.imap_ssl if req.imap_ssl is not None else defaults.get("imap_ssl", True),
        "smtp_host": req.smtp_host or defaults.get("smtp_host"),
        "smtp_port": req.smtp_port or defaults.get("smtp_port"),
        "smtp_ssl": req.smtp_ssl if req.smtp_ssl is not None else defaults.get("smtp_ssl", False),
        "smtp_user": req.smtp_user or req.email,
        "from_email": req.from_email or req.email,
        "mailbox": req.mailbox or "INBOX",
    }

    upsert_integration_credential(
        db=db,
        user_id=req.user_id,
        provider=provider,
        username=req.email,
        secret=req.password,
        server_url=metadata.get("imap_host"),
        metadata=metadata,
    )

    verified = None
    if req.verify:
        try:
            verified = verify_imap_connection(db=db, user_id=req.user_id, provider=provider)
        except Exception as exc:
            raise HTTPException(status_code=400, detail=str(exc))

    return {"ok": True, "verified": verified}


@router.get("/status")
def email_status(user_id: str, provider: str, db: Session = Depends(get_db)):
    provider = (provider or "").lower().strip()
    if provider not in {"icloud", "yahoo"}:
        raise HTTPException(status_code=400, detail="Provider must be icloud or yahoo")
    get_or_create_user(db, user_id)
    return get_connection_status(db=db, user_id=user_id, provider=provider)


@router.post("/disconnect")
def email_disconnect(user_id: str, provider: str, db: Session = Depends(get_db)):
    provider = (provider or "").lower().strip()
    if provider not in {"icloud", "yahoo"}:
        raise HTTPException(status_code=400, detail="Provider must be icloud or yahoo")
    get_or_create_user(db, user_id)
    delete_integration_credential(db=db, user_id=user_id, provider=provider)
    return {"ok": True}
