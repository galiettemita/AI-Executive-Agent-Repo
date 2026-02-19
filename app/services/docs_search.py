from __future__ import annotations

import logging
from typing import Any

import httpx
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.google_oauth import get_valid_google_credentials

logger = logging.getLogger(__name__)


def search_google_drive_docs(
    db: Session,
    *,
    user_id: str,
    query: str,
    max_results: int = 6,
) -> list[dict[str, Any]]:
    credentials = get_valid_google_credentials(db, user_id)
    if not credentials or not credentials.token:
        return []

    q = str(query or "").replace("'", "\\'")
    params = {
        "q": f"name contains '{q}' and trashed=false",
        "fields": "files(id,name,mimeType,webViewLink,modifiedTime,owners(displayName,emailAddress))",
        "pageSize": max(1, min(25, int(max_results))),
        "includeItemsFromAllDrives": "true",
        "supportsAllDrives": "true",
        "orderBy": "modifiedTime desc",
    }
    headers = {"Authorization": f"Bearer {credentials.token}"}
    with httpx.Client(timeout=25) as client:
        resp = client.get("https://www.googleapis.com/drive/v3/files", params=params, headers=headers)
        resp.raise_for_status()
        payload = resp.json()

    files = payload.get("files") if isinstance(payload, dict) else []
    if not isinstance(files, list):
        return []

    results: list[dict[str, Any]] = []
    for item in files:
        if not isinstance(item, dict):
            continue
        mime_type = str(item.get("mimeType") or "")
        if "google-apps" not in mime_type and "document" not in mime_type:
            # Keep search focused on docs/sheets/slides.
            continue
        results.append(
            {
                "provider": "google_drive",
                "id": str(item.get("id") or ""),
                "title": str(item.get("name") or ""),
                "mime_type": mime_type,
                "url": str(item.get("webViewLink") or ""),
                "updated_at": item.get("modifiedTime"),
                "owner": ((item.get("owners") or [{}])[0] or {}).get("displayName"),
            }
        )
    return results[: max(1, min(25, int(max_results)))]


def search_notion_docs(
    *,
    query: str,
    max_results: int = 6,
) -> list[dict[str, Any]]:
    notion_key = str(settings.NOTION_API_KEY or "").strip()
    if not notion_key:
        return []

    payload = {"query": str(query or "").strip(), "page_size": max(1, min(25, int(max_results)))}
    headers = {
        "Authorization": f"Bearer {notion_key}",
        "Notion-Version": "2022-06-28",
        "Content-Type": "application/json",
    }
    with httpx.Client(timeout=25) as client:
        resp = client.post("https://api.notion.com/v1/search", headers=headers, json=payload)
        resp.raise_for_status()
        body = resp.json()

    rows = body.get("results") if isinstance(body, dict) else []
    if not isinstance(rows, list):
        return []

    out: list[dict[str, Any]] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        if str(row.get("object") or "") != "page":
            continue
        title = "Untitled"
        props = row.get("properties") if isinstance(row.get("properties"), dict) else {}
        for prop in props.values():
            if not isinstance(prop, dict):
                continue
            if str(prop.get("type") or "") == "title":
                title_items = prop.get("title") if isinstance(prop.get("title"), list) else []
                text_value = "".join(str((item or {}).get("plain_text") or "") for item in title_items if isinstance(item, dict)).strip()
                if text_value:
                    title = text_value
                    break
        out.append(
            {
                "provider": "notion",
                "id": str(row.get("id") or ""),
                "title": title,
                "url": str(row.get("url") or ""),
                "updated_at": row.get("last_edited_time"),
            }
        )
    return out[: max(1, min(25, int(max_results)))]


def search_connected_docs(
    db: Session,
    *,
    user_id: str,
    query: str,
    max_results: int = 8,
) -> dict[str, Any]:
    query_value = str(query or "").strip()
    if not query_value:
        raise ValueError("query is required")

    per_provider = max(1, min(25, int(max_results)))
    google_results: list[dict[str, Any]] = []
    notion_results: list[dict[str, Any]] = []

    try:
        google_results = search_google_drive_docs(
            db,
            user_id=user_id,
            query=query_value,
            max_results=per_provider,
        )
    except Exception as exc:
        logger.warning("google docs search failed: %s", exc)

    try:
        notion_results = search_notion_docs(
            query=query_value,
            max_results=per_provider,
        )
    except Exception as exc:
        logger.warning("notion docs search failed: %s", exc)

    merged = (google_results + notion_results)[: max(1, min(50, int(max_results)))]
    return {
        "query": query_value,
        "results": merged,
        "providers": {
            "google_drive": {
                "configured": bool(settings.GOOGLE_CLIENT_ID and settings.GOOGLE_CLIENT_SECRET),
                "hits": len(google_results),
            },
            "notion": {
                "configured": bool(settings.NOTION_API_KEY),
                "hits": len(notion_results),
            },
        },
    }
