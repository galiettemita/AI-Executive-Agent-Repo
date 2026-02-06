# app/api/routes/gdpr.py
"""
GDPR Compliance API Endpoints

Provides endpoints for:
- POST /gdpr/delete - Delete user data (Right to be Forgotten)
- POST /gdpr/delete/preview - Preview what would be deleted (dry run)
- GET /gdpr/export - Export user data (Data Portability)
- GET /gdpr/summary - Get summary of user data

All endpoints require authentication via user_id.
"""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.database import SessionLocal
from app.services.gdpr_service import (
    delete_user_data,
    export_user_data,
    get_user_data_summary,
)

router = APIRouter(prefix="/gdpr", tags=["gdpr"])


def get_db():
    """Database session dependency."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


class DeletionRequest(BaseModel):
    """Request model for data deletion."""

    user_id: str
    confirmation: str  # Must be "DELETE MY DATA" for safety


class DeletionResponse(BaseModel):
    """Response model for data deletion."""

    success: bool
    user_id: str
    dry_run: bool
    deleted_at: Optional[str] = None
    tables: dict
    errors: list


class ExportResponse(BaseModel):
    """Response model for data export."""

    user_id: str
    exported_at: str
    data: dict


class SummaryResponse(BaseModel):
    """Response model for data summary."""

    user_id: str
    record_counts: dict
    total_records: int


@router.post("/delete/preview", response_model=DeletionResponse)
async def preview_deletion(
    user_id: str = Query(..., description="User ID to preview deletion for"),
    db: Session = Depends(get_db),
):
    """
    Preview what data would be deleted for a user.

    This is a dry-run that shows what would be deleted without
    actually deleting anything. Use this before actual deletion.
    """
    result = delete_user_data(
        db=db,
        user_id=user_id,
        dry_run=True,
        keep_anonymized_transactions=True,
    )

    return DeletionResponse(
        success=result["success"],
        user_id=result["user_id"],
        dry_run=result["dry_run"],
        deleted_at=result["deleted_at"],
        tables=result["tables"],
        errors=result["errors"],
    )


@router.post("/delete", response_model=DeletionResponse)
async def delete_data(
    request: DeletionRequest,
    db: Session = Depends(get_db),
):
    """
    Delete all user data (Right to be Forgotten).

    GDPR Article 17 compliant data deletion.

    Requires confirmation string "DELETE MY DATA" to prevent accidental deletion.

    Note: Financial transaction records are anonymized rather than deleted
    to comply with financial record-keeping requirements.
    """
    # Safety check: require explicit confirmation
    if request.confirmation != "DELETE MY DATA":
        raise HTTPException(
            status_code=400,
            detail="Confirmation must be 'DELETE MY DATA' to proceed with deletion.",
        )

    result = delete_user_data(
        db=db,
        user_id=request.user_id,
        dry_run=False,
        keep_anonymized_transactions=True,
    )

    if not result["success"]:
        raise HTTPException(
            status_code=500,
            detail=f"Deletion failed: {', '.join(result['errors'])}",
        )

    return DeletionResponse(
        success=result["success"],
        user_id=result["user_id"],
        dry_run=result["dry_run"],
        deleted_at=result["deleted_at"],
        tables=result["tables"],
        errors=result["errors"],
    )


@router.get("/export", response_model=ExportResponse)
async def export_data(
    user_id: str = Query(..., description="User ID to export data for"),
    db: Session = Depends(get_db),
):
    """
    Export all user data (Data Portability).

    GDPR Article 20 compliant data export.

    Returns all user data in a structured JSON format.
    """
    result = export_user_data(db=db, user_id=user_id)

    return ExportResponse(
        user_id=result["user_id"],
        exported_at=result["exported_at"],
        data=result["data"],
    )


@router.get("/summary", response_model=SummaryResponse)
async def get_summary(
    user_id: str = Query(..., description="User ID to get summary for"),
    db: Session = Depends(get_db),
):
    """
    Get a summary of user data counts.

    Returns the number of records in each table for the specified user.
    """
    counts = get_user_data_summary(db=db, user_id=user_id)
    total = sum(counts.values())

    return SummaryResponse(
        user_id=user_id,
        record_counts=counts,
        total_records=total,
    )
