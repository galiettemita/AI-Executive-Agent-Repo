# backend/app/api/routes/proposals.py

from __future__ import annotations

import json
from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import HTMLResponse
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.db.models import Proposal
from app.services.proposal_links import verify_token
from app.services.proposals import get_proposal, update_proposal_status, log_proposal_action


router = APIRouter(prefix="/proposals", tags=["proposals"])


def _require_token(request: Request, proposal_id: int) -> None:
    token = request.query_params.get("token", "")
    data = verify_token(token)
    if not data or int(data.get("proposal_id", -1)) != int(proposal_id):
        raise HTTPException(status_code=403, detail="Invalid or expired token")


def _proposal_payload(row: Proposal) -> dict:
    try:
        payload = json.loads(row.payload_json or "{}")
    except Exception:
        payload = {}
    return {
        "id": row.id,
        "user_id": row.user_id,
        "type": row.proposal_type,
        "status": row.status,
        "payload": payload,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "expires_at": row.expires_at.isoformat() if row.expires_at else None,
    }


@router.get("/{proposal_id}", response_class=HTMLResponse)
def proposal_view(proposal_id: int, request: Request, db: Session = Depends(get_db)):
    _require_token(request, proposal_id)
    row = get_proposal(db, proposal_id)
    if not row:
        raise HTTPException(status_code=404, detail="Proposal not found")
    data = _proposal_payload(row)

    html = f"""
<!doctype html>
<html>
  <head>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>Proposal {proposal_id}</title>
    <style>
      body {{ font-family: Arial, sans-serif; padding: 16px; max-width: 720px; margin: 0 auto; }}
      .card {{ border: 1px solid #ddd; border-radius: 12px; padding: 16px; }}
      .row {{ margin-bottom: 12px; }}
      button {{ padding: 12px 16px; border-radius: 8px; border: none; }}
      .approve {{ background: #16a34a; color: white; }}
      .cancel {{ background: #ef4444; color: white; }}
    </style>
  </head>
  <body>
    <h2>Action Proposal</h2>
    <div class="card">
      <div class="row"><strong>Type:</strong> {data["type"]}</div>
      <div class="row"><strong>Status:</strong> {data["status"]}</div>
      <div class="row"><strong>Payload:</strong> <pre>{json.dumps(data["payload"], indent=2)}</pre></div>
      <form method="post" action="/proposals/{proposal_id}/approve?token={request.query_params.get("token", "")}">
        <button class="approve" type="submit">Approve</button>
      </form>
      <form method="post" action="/proposals/{proposal_id}/cancel?token={request.query_params.get("token", "")}" style="margin-top:8px;">
        <button class="cancel" type="submit">Cancel</button>
      </form>
    </div>
  </body>
</html>
"""
    return HTMLResponse(content=html)


@router.post("/{proposal_id}/approve")
def proposal_approve(proposal_id: int, request: Request, db: Session = Depends(get_db)):
    _require_token(request, proposal_id)
    row = get_proposal(db, proposal_id)
    if not row:
        raise HTTPException(status_code=404, detail="Proposal not found")

    old_status = row.status
    update_proposal_status(db, proposal_id, "approved")

    # Log approval action
    log_proposal_action(
        db,
        proposal_id=proposal_id,
        user_id=row.user_id,
        action="approved",
        old_status=old_status,
        new_status="approved",
        metadata={"ip": request.client.host if request.client else None}
    )

    return {"ok": True, "status": "approved"}


@router.post("/{proposal_id}/cancel")
def proposal_cancel(proposal_id: int, request: Request, db: Session = Depends(get_db)):
    _require_token(request, proposal_id)
    row = get_proposal(db, proposal_id)
    if not row:
        raise HTTPException(status_code=404, detail="Proposal not found")

    old_status = row.status
    update_proposal_status(db, proposal_id, "canceled")

    # Log cancel action
    log_proposal_action(
        db,
        proposal_id=proposal_id,
        user_id=row.user_id,
        action="canceled",
        old_status=old_status,
        new_status="canceled",
        metadata={"ip": request.client.host if request.client else None}
    )

    return {"ok": True, "status": "canceled"}


@router.post("/{proposal_id}/edit")
async def proposal_edit(proposal_id: int, request: Request, db: Session = Depends(get_db)):
    _require_token(request, proposal_id)
    row = get_proposal(db, proposal_id)
    if not row:
        raise HTTPException(status_code=404, detail="Proposal not found")

    old_status = row.status
    old_payload = json.loads(row.payload_json or "{}")
    new_payload = await request.json()

    row.payload_json = json.dumps(new_payload or {}, ensure_ascii=False)
    row.status = "edited"
    db.commit()

    # Log edit action with changes
    log_proposal_action(
        db,
        proposal_id=proposal_id,
        user_id=row.user_id,
        action="edited",
        old_status=old_status,
        new_status="edited",
        changes={"old_payload": old_payload, "new_payload": new_payload},
        metadata={"ip": request.client.host if request.client else None}
    )

    return {"ok": True, "status": "edited"}
