from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.services.plaid_connector import (
    PlaidNotConfiguredError,
    create_link_token,
    exchange_public_token,
    list_accounts,
    list_transactions,
)


router = APIRouter(prefix="/api/v1/plaid", tags=["plaid"])


class PlaidLinkTokenRequest(BaseModel):
    user_id: str
    stage: str = "staging"


class PlaidExchangeTokenRequest(BaseModel):
    user_id: str
    public_token: str
    stage: str = "staging"
    approved: bool = False


@router.post("/link-token")
def plaid_link_token(payload: PlaidLinkTokenRequest):
    stage = "prod" if payload.stage.strip().lower() == "prod" else "staging"
    if stage == "prod":
        raise HTTPException(status_code=400, detail="Plaid production is disabled during Phase 3")
    try:
        token = create_link_token(user_id=payload.user_id, stage=stage)
        return {"ok": True, "link_token": token.get("link_token"), "expiration": token.get("expiration")}
    except PlaidNotConfiguredError as exc:
        raise HTTPException(status_code=503, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Failed to create link token: {exc}")


@router.post("/exchange-public-token")
def plaid_exchange(payload: PlaidExchangeTokenRequest, db: Session = Depends(get_db)):
    stage = "prod" if payload.stage.strip().lower() == "prod" else "staging"
    if stage == "prod":
        raise HTTPException(status_code=400, detail="Plaid production is disabled during Phase 3")
    if not payload.approved:
        raise HTTPException(
            status_code=428,
            detail={
                "message": "Plaid public-token exchange requires explicit approval",
                "risk_level": "high",
                "required_action": "Retry with approved=true after user confirmation",
                "operation": "plaid.exchange_public_token",
            },
        )
    try:
        result = exchange_public_token(
            db=db,
            user_id=payload.user_id,
            public_token=payload.public_token,
            stage=stage,
        )
        return {"ok": True, **result}
    except PlaidNotConfiguredError as exc:
        raise HTTPException(status_code=503, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Failed to exchange public token: {exc}")


@router.get("/accounts")
def plaid_accounts(user_id: str, db: Session = Depends(get_db)):
    try:
        data = list_accounts(db=db, user_id=user_id, stage="staging")
        return {"ok": True, "accounts": data.get("accounts") or []}
    except PlaidNotConfiguredError as exc:
        raise HTTPException(status_code=503, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Failed to load accounts: {exc}")


@router.get("/transactions")
def plaid_transactions(user_id: str, start_date: date, end_date: date, db: Session = Depends(get_db)):
    try:
        data = list_transactions(db=db, user_id=user_id, start_date=start_date, end_date=end_date, stage="staging")
        return {"ok": True, "transactions": data.get("transactions") or []}
    except PlaidNotConfiguredError as exc:
        raise HTTPException(status_code=503, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Failed to load transactions: {exc}")
