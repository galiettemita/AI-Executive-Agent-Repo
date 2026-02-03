# backend/app/api/routes/billing.py

from fastapi import APIRouter

router = APIRouter(prefix="/billing", tags=["billing"])


@router.get("/success")
def billing_success():
    return {"ok": True, "status": "paid"}


@router.get("/cancel")
def billing_cancel():
    return {"ok": True, "status": "canceled"}
