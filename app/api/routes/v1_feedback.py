from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.services.user_feedback import record_user_feedback

router = APIRouter(prefix="/api/v1/feedback", tags=["feedback-v1"])


class FeedbackCreateRequest(BaseModel):
    user_id: str
    feedback: str = Field(description="up/down")
    message_id: str | None = None
    run_id: str | None = None
    comment: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


@router.post("/response")
def create_feedback(payload: FeedbackCreateRequest, db: Session = Depends(get_db)):
    try:
        result = record_user_feedback(
            db,
            user_id=payload.user_id,
            message_id=payload.message_id,
            run_id=payload.run_id,
            feedback=payload.feedback,
            comment=payload.comment,
            metadata=payload.metadata,
        )
        return result
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
