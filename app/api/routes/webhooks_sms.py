# backend/app/api/routes/webhooks_sms.py

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session
from twilio.request_validator import RequestValidator

from app.core.config import settings
from app.api.deps import get_db
from app.services.messaging_service import record_delivery_status
from app.middleware.rate_limiter import rate_limit_webhook

router = APIRouter(prefix="/webhooks/sms", tags=["webhooks"])


def _validate_twilio(request: Request, params: dict) -> bool:
    if settings.ENFORCE_WEBHOOK_SIGNATURES != "1":
        return True
    if not settings.TWILIO_AUTH_TOKEN:
        return False
    signature = request.headers.get("X-Twilio-Signature", "")
    validator = RequestValidator(settings.TWILIO_AUTH_TOKEN)
    return validator.validate(str(request.url), params, signature)


@rate_limit_webhook()
@router.post("/status")
async def sms_status_webhook(request: Request, db: Session = Depends(get_db)):
    form = await request.form()
    params = dict(form)
    if not _validate_twilio(request, params):
        raise HTTPException(status_code=403, detail="Invalid Twilio signature")

    message_sid = params.get("MessageSid") or params.get("SmsSid")
    status = params.get("MessageStatus") or params.get("SmsStatus") or ""

    if message_sid:
        record_delivery_status(
            db,
            provider="twilio",
            provider_message_id=message_sid,
            provider_status=status,
            payload=params,
        )

    return {"ok": True}
