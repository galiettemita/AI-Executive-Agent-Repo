from __future__ import annotations

from typing import Any

import httpx
from sqlalchemy.orm import Session

from app.services.microsoft_oauth import get_valid_microsoft_access_token


GRAPH_BASE = "https://graph.microsoft.com/v1.0"


def _headers(access_token: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {access_token}",
        "ConsistencyLevel": "eventual",
    }


def search_microsoft_contacts(
    db: Session,
    *,
    user_id: str,
    query: str,
    max_results: int = 10,
) -> list[dict[str, Any]]:
    token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    q = (query or "").strip()
    if not q:
        return []

    params = {
        "$search": f"\"{q}\"",
        "$top": max(1, min(50, int(max_results))),
    }
    with httpx.Client(timeout=15.0) as client:
        resp = client.get(f"{GRAPH_BASE}/me/people", headers=_headers(token), params=params)
        resp.raise_for_status()
        items = (resp.json() or {}).get("value") or []

    out: list[dict[str, Any]] = []
    for row in items:
        emails = [e.get("address") for e in (row.get("scoredEmailAddresses") or []) if isinstance(e, dict)]
        phones = [str(p) for p in (row.get("phones") or []) if p]
        out.append(
            {
                "id": row.get("id"),
                "display_name": row.get("displayName"),
                "given_name": row.get("givenName"),
                "surname": row.get("surname"),
                "emails": [e for e in emails if e],
                "phones": phones,
                "job_title": row.get("jobTitle"),
                "department": row.get("department"),
                "provider": "microsoft",
            }
        )
    return out
