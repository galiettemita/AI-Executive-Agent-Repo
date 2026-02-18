from __future__ import annotations

import hmac
import json
import time
from datetime import datetime, timezone
from typing import Any

import httpx
import jwt
from fastapi import APIRouter, Depends, Request
from fastapi.responses import JSONResponse, Response
from googleapiclient.discovery import build
from sqlalchemy.orm import Session

from app.blueprint.mcp.custom.apple_reminders.handlers import handle_tool_call as handle_apple_tool_call
from app.core.config import settings
from app.db.database import get_db
from app.services.calendar_router import delete_event as delete_calendar_event
from app.services.google_calendar import (
    create_calendar_event,
    list_events_in_range as list_calendar_events_in_range,
    list_upcoming_events,
    update_calendar_event,
)
from app.services.google_gmail import get_gmail_message, search_gmail_messages, send_email
from app.services.google_oauth import get_valid_google_credentials

router = APIRouter(tags=["mcp-wave1-host"])


SERVER_TOOLS: dict[str, list[dict[str, Any]]] = {
    "google-calendar-mcp": [
        {
            "name": "calendar.list",
            "description": "List Google Calendar events in a range.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "start_iso": {"type": "string"},
                    "end_iso": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
        {
            "name": "calendar.create",
            "description": "Create a Google Calendar event.",
            "inputSchema": {
                "type": "object",
                "required": ["title", "start_iso", "end_iso"],
                "properties": {
                    "title": {"type": "string"},
                    "start_iso": {"type": "string"},
                    "end_iso": {"type": "string"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                },
            },
        },
        {
            "name": "calendar.update",
            "description": "Update a Google Calendar event.",
            "inputSchema": {
                "type": "object",
                "required": ["event_id"],
                "properties": {
                    "event_id": {"type": "string"},
                    "title": {"type": "string"},
                    "start_iso": {"type": "string"},
                    "end_iso": {"type": "string"},
                    "description": {"type": "string"},
                    "location": {"type": "string"},
                },
            },
        },
        {
            "name": "calendar.delete",
            "description": "Delete a Google Calendar event.",
            "inputSchema": {
                "type": "object",
                "required": ["event_id"],
                "properties": {
                    "event_id": {"type": "string"},
                },
            },
        },
    ],
    "google-drive-mcp": [
        {
            "name": "drive.search",
            "description": "Search files in Google Drive.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
        {
            "name": "drive.get_file",
            "description": "Get Google Drive file metadata by id.",
            "inputSchema": {
                "type": "object",
                "required": ["file_id"],
                "properties": {
                    "file_id": {"type": "string"},
                },
            },
        },
        {
            "name": "drive.list_recent",
            "description": "List recently modified Google Drive files.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
    ],
    "gmail-mcp": [
        {
            "name": "gmail.search",
            "description": "Search Gmail messages.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 50},
                    "include_body": {"type": "boolean"},
                },
            },
        },
        {
            "name": "gmail.get_message",
            "description": "Get a Gmail message by id.",
            "inputSchema": {
                "type": "object",
                "required": ["message_id"],
                "properties": {
                    "message_id": {"type": "string"},
                    "include_body": {"type": "boolean"},
                },
            },
        },
        {
            "name": "gmail.send",
            "description": "Send an email via Gmail.",
            "inputSchema": {
                "type": "object",
                "required": ["to_email", "subject", "body_text"],
                "properties": {
                    "to_email": {"type": "string"},
                    "subject": {"type": "string"},
                    "body_text": {"type": "string"},
                    "cc": {"type": "string"},
                    "bcc": {"type": "string"},
                },
            },
        },
    ],
    "notion-mcp": [
        {
            "name": "notion.search",
            "description": "Search Notion pages/databases.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
        {
            "name": "notion.get_page",
            "description": "Get Notion page details.",
            "inputSchema": {
                "type": "object",
                "required": ["page_id"],
                "properties": {"page_id": {"type": "string"}},
            },
        },
        {
            "name": "notion.update_page",
            "description": "Update Notion page properties.",
            "inputSchema": {
                "type": "object",
                "required": ["page_id", "properties"],
                "properties": {
                    "page_id": {"type": "string"},
                    "properties": {"type": "object"},
                },
            },
        },
    ],
    "todoist-mcp": [
        {
            "name": "todoist.list_tasks",
            "description": "List Todoist tasks.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 200},
                },
            },
        },
        {
            "name": "todoist.create_task",
            "description": "Create a Todoist task.",
            "inputSchema": {
                "type": "object",
                "required": ["content"],
                "properties": {
                    "content": {"type": "string"},
                    "description": {"type": "string"},
                    "due_string": {"type": "string"},
                    "due_date": {"type": "string"},
                    "project_id": {"type": "string"},
                    "priority": {"type": "integer"},
                },
            },
        },
        {
            "name": "todoist.complete_task",
            "description": "Mark a Todoist task as completed.",
            "inputSchema": {
                "type": "object",
                "required": ["task_id"],
                "properties": {
                    "task_id": {"type": "string"},
                },
            },
        },
    ],
    "brave-search-mcp": [
        {
            "name": "brave.search",
            "description": "Run Brave web search.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "count": {"type": "integer", "minimum": 1, "maximum": 20},
                },
            },
        },
        {
            "name": "brave.news",
            "description": "Run Brave news search.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "count": {"type": "integer", "minimum": 1, "maximum": 20},
                },
            },
        },
        {
            "name": "brave.images",
            "description": "Run Brave image search.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "count": {"type": "integer", "minimum": 1, "maximum": 20},
                },
            },
        },
    ],
    "github-mcp": [
        {
            "name": "github.list_repos",
            "description": "List repositories available to the GitHub app installation.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
        {
            "name": "github.search_issues",
            "description": "Search GitHub issues.",
            "inputSchema": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "minimum": 1, "maximum": 100},
                },
            },
        },
        {
            "name": "github.create_issue",
            "description": "Create a GitHub issue in a repository.",
            "inputSchema": {
                "type": "object",
                "required": ["owner", "repo", "title"],
                "properties": {
                    "owner": {"type": "string"},
                    "repo": {"type": "string"},
                    "title": {"type": "string"},
                    "body": {"type": "string"},
                },
            },
        },
    ],
    "apple-reminders-mcp": [
        {
            "name": "reminders.list",
            "description": "List reminders.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "completed": {"type": "boolean"},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 200},
                },
            },
        },
        {
            "name": "reminders.create",
            "description": "Create a reminder.",
            "inputSchema": {
                "type": "object",
                "required": ["title"],
                "properties": {
                    "title": {"type": "string"},
                    "notes": {"type": "string"},
                    "due_at": {"type": "string"},
                },
            },
        },
        {
            "name": "reminders.complete",
            "description": "Complete a reminder.",
            "inputSchema": {
                "type": "object",
                "required": ["reminder_id"],
                "properties": {
                    "reminder_id": {"type": "string"},
                },
            },
        },
    ],
}

_GITHUB_TOKEN_CACHE: dict[str, Any] = {"token": None, "expires_at": 0.0}


def _ok(payload_id: str | int | None, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": payload_id, "result": result}


def _error(payload_id: str | int | None, message: str, code: int = -32000) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": payload_id, "error": {"code": code, "message": message}}


def _text_result(payload: Any) -> dict[str, Any]:
    return {"content": [{"type": "text", "text": json.dumps(payload, ensure_ascii=False, default=str)}]}


def _parse_iso_utc(raw: Any) -> datetime | None:
    if raw is None:
        return None
    value = str(raw).strip()
    if not value:
        return None
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"
    dt = datetime.fromisoformat(value)
    if dt.tzinfo is None:
        return dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def _require_user_id(arguments: dict[str, Any]) -> str:
    user_id = arguments.get("_eo_user_id") or arguments.get("user_id")
    user_id = str(user_id or "").strip()
    if not user_id:
        raise ValueError("_eo_user_id is required for Wave 1 Google tools")
    return user_id


def _require_secret(value: str | None, name: str) -> str:
    if value and str(value).strip():
        return str(value).strip()
    raise RuntimeError(f"Missing required secret: {name}")


def _github_installation_token() -> str:
    now = time.time()
    cached = str(_GITHUB_TOKEN_CACHE.get("token") or "").strip()
    cached_exp = float(_GITHUB_TOKEN_CACHE.get("expires_at") or 0)
    if cached and now < cached_exp - 60:
        return cached

    app_id = _require_secret(settings.GITHUB_APP_ID, "GITHUB_APP_ID")
    private_key = _require_secret(settings.GITHUB_APP_PRIVATE_KEY, "GITHUB_APP_PRIVATE_KEY").replace("\\n", "\n")
    installation_id = _require_secret(settings.GITHUB_INSTALLATION_ID, "GITHUB_INSTALLATION_ID")

    issued_at = int(now) - 60
    payload = {"iat": issued_at, "exp": issued_at + 540, "iss": app_id}
    app_jwt = jwt.encode(payload, private_key, algorithm="RS256")

    with httpx.Client(timeout=20.0) as client:
        resp = client.post(
            f"https://api.github.com/app/installations/{installation_id}/access_tokens",
            headers={
                "Authorization": f"Bearer {app_jwt}",
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        if resp.status_code >= 300:
            raise RuntimeError(f"GitHub token exchange failed ({resp.status_code}): {resp.text[:300]}")
        data = resp.json()

    token = str(data.get("token") or "").strip()
    if not token:
        raise RuntimeError("GitHub installation token is empty")

    expires_at_raw = str(data.get("expires_at") or "").strip()
    expires_at = now + 300
    if expires_at_raw:
        exp = _parse_iso_utc(expires_at_raw)
        if exp:
            expires_at = exp.timestamp()

    _GITHUB_TOKEN_CACHE["token"] = token
    _GITHUB_TOKEN_CACHE["expires_at"] = expires_at
    return token


def _google_drive_service(db: Session, user_id: str):
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")
    return build("drive", "v3", credentials=creds)


def _dispatch_google_calendar(db: Session, tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    user_id = _require_user_id(arguments)

    if tool_name == "calendar.list":
        max_results = int(arguments.get("max_results") or arguments.get("limit") or 10)
        start_utc = _parse_iso_utc(arguments.get("start_iso") or arguments.get("start"))
        end_utc = _parse_iso_utc(arguments.get("end_iso") or arguments.get("end"))
        if start_utc and end_utc:
            items = list_calendar_events_in_range(
                db=db,
                user_id=user_id,
                start_utc=start_utc,
                end_utc=end_utc,
                max_results=max_results,
            )
        else:
            items = list_upcoming_events(db=db, user_id=user_id, max_results=max_results)
        return _text_result({"items": items, "count": len(items)})

    if tool_name == "calendar.create":
        title = str(arguments.get("title") or "").strip()
        if not title:
            raise ValueError("title is required")
        start_utc = _parse_iso_utc(arguments.get("start_iso"))
        end_utc = _parse_iso_utc(arguments.get("end_iso"))
        if not start_utc or not end_utc:
            raise ValueError("start_iso and end_iso are required")
        event = create_calendar_event(
            db=db,
            user_id=user_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=arguments.get("description"),
            location=arguments.get("location"),
        )
        return _text_result({"event": event})

    if tool_name == "calendar.update":
        event_id = str(arguments.get("event_id") or "").strip()
        if not event_id:
            raise ValueError("event_id is required")
        event = update_calendar_event(
            db=db,
            user_id=user_id,
            event_id=event_id,
            title=arguments.get("title"),
            start_utc=_parse_iso_utc(arguments.get("start_iso")),
            end_utc=_parse_iso_utc(arguments.get("end_iso")),
            description=arguments.get("description"),
            location=arguments.get("location"),
        )
        return _text_result({"event": event})

    if tool_name == "calendar.delete":
        event_id = str(arguments.get("event_id") or "").strip()
        if not event_id:
            raise ValueError("event_id is required")
        result = delete_calendar_event(db=db, user_id=user_id, event_id=event_id, provider="google")
        return _text_result(result)

    raise ValueError(f"Unsupported tool for google-calendar-mcp: {tool_name}")


def _dispatch_google_drive(db: Session, tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    user_id = _require_user_id(arguments)
    service = _google_drive_service(db=db, user_id=user_id)

    if tool_name == "drive.search":
        query = str(arguments.get("query") or "").strip()
        max_results = int(arguments.get("max_results") or 10)
        q = query if query else "trashed = false"
        if "trashed" not in q.lower():
            q = f"({q}) and trashed = false"
        payload = (
            service.files()
            .list(
                q=q,
                pageSize=max_results,
                fields="files(id,name,mimeType,modifiedTime,webViewLink,size,owners(displayName,emailAddress))",
                orderBy="modifiedTime desc",
            )
            .execute()
        )
        files = payload.get("files") or []
        return _text_result({"files": files, "count": len(files)})

    if tool_name == "drive.get_file":
        file_id = str(arguments.get("file_id") or "").strip()
        if not file_id:
            raise ValueError("file_id is required")
        file_data = (
            service.files()
            .get(
                fileId=file_id,
                fields="id,name,mimeType,modifiedTime,webViewLink,size,owners(displayName,emailAddress)",
            )
            .execute()
        )
        return _text_result({"file": file_data})

    if tool_name == "drive.list_recent":
        max_results = int(arguments.get("max_results") or 10)
        payload = (
            service.files()
            .list(
                q="trashed = false",
                pageSize=max_results,
                fields="files(id,name,mimeType,modifiedTime,webViewLink,size,owners(displayName,emailAddress))",
                orderBy="modifiedTime desc",
            )
            .execute()
        )
        files = payload.get("files") or []
        return _text_result({"files": files, "count": len(files)})

    raise ValueError(f"Unsupported tool for google-drive-mcp: {tool_name}")


def _dispatch_gmail(db: Session, tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    user_id = _require_user_id(arguments)

    if tool_name == "gmail.search":
        query = str(arguments.get("query") or "").strip()
        if not query:
            raise ValueError("query is required")
        max_results = int(arguments.get("max_results") or 10)
        include_body = bool(arguments.get("include_body") or False)
        items = search_gmail_messages(
            db=db,
            user_id=user_id,
            query=query,
            max_results=max_results,
            include_body=include_body,
        )
        return _text_result({"messages": items, "count": len(items)})

    if tool_name == "gmail.get_message":
        message_id = str(arguments.get("message_id") or "").strip()
        if not message_id:
            raise ValueError("message_id is required")
        include_body = bool(arguments.get("include_body") if "include_body" in arguments else True)
        item = get_gmail_message(db=db, user_id=user_id, message_id=message_id, include_body=include_body)
        return _text_result({"message": item})

    if tool_name == "gmail.send":
        to_email = str(arguments.get("to_email") or "").strip()
        subject = str(arguments.get("subject") or "").strip()
        body_text = str(arguments.get("body_text") or "").strip()
        if not to_email or not subject or not body_text:
            raise ValueError("to_email, subject, and body_text are required")
        sent = send_email(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=arguments.get("cc"),
            bcc=arguments.get("bcc"),
        )
        return _text_result({"sent": sent})

    raise ValueError(f"Unsupported tool for gmail-mcp: {tool_name}")


def _dispatch_notion(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    token = _require_secret(settings.NOTION_API_KEY, "NOTION_API_KEY")
    headers = {
        "Authorization": f"Bearer {token}",
        "Notion-Version": "2022-06-28",
        "Content-Type": "application/json",
    }

    with httpx.Client(timeout=20.0) as client:
        if tool_name == "notion.search":
            query = str(arguments.get("query") or "").strip()
            if not query:
                raise ValueError("query is required")
            max_results = int(arguments.get("max_results") or 10)
            resp = client.post(
                "https://api.notion.com/v1/search",
                headers=headers,
                json={"query": query, "page_size": max_results},
            )
            if resp.status_code >= 300:
                raise RuntimeError(f"Notion search failed ({resp.status_code}): {resp.text[:300]}")
            data = resp.json()
            return _text_result({"results": data.get("results") or [], "has_more": bool(data.get("has_more"))})

        if tool_name == "notion.get_page":
            page_id = str(arguments.get("page_id") or "").strip()
            if not page_id:
                raise ValueError("page_id is required")
            resp = client.get(f"https://api.notion.com/v1/pages/{page_id}", headers=headers)
            if resp.status_code >= 300:
                raise RuntimeError(f"Notion get_page failed ({resp.status_code}): {resp.text[:300]}")
            return _text_result({"page": resp.json()})

        if tool_name == "notion.update_page":
            page_id = str(arguments.get("page_id") or "").strip()
            properties = arguments.get("properties") if isinstance(arguments.get("properties"), dict) else None
            if not page_id or properties is None:
                raise ValueError("page_id and properties are required")
            resp = client.patch(
                f"https://api.notion.com/v1/pages/{page_id}",
                headers=headers,
                json={"properties": properties},
            )
            if resp.status_code >= 300:
                raise RuntimeError(f"Notion update_page failed ({resp.status_code}): {resp.text[:300]}")
            return _text_result({"page": resp.json()})

    raise ValueError(f"Unsupported tool for notion-mcp: {tool_name}")


def _dispatch_todoist(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    token = _require_secret(settings.TODOIST_API_TOKEN, "TODOIST_API_TOKEN")
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    with httpx.Client(timeout=20.0) as client:
        if tool_name == "todoist.list_tasks":
            max_results = int(arguments.get("max_results") or 20)
            resp = client.get("https://api.todoist.com/rest/v2/tasks", headers=headers, params={"limit": max_results})
            if resp.status_code >= 300:
                raise RuntimeError(f"Todoist list_tasks failed ({resp.status_code}): {resp.text[:300]}")
            tasks = resp.json() if isinstance(resp.json(), list) else []
            return _text_result({"tasks": tasks, "count": len(tasks)})

        if tool_name == "todoist.create_task":
            content = str(arguments.get("content") or "").strip()
            if not content:
                raise ValueError("content is required")
            payload: dict[str, Any] = {"content": content}
            for key in ("description", "due_string", "due_date", "project_id", "priority"):
                if arguments.get(key) not in (None, ""):
                    payload[key] = arguments.get(key)
            resp = client.post("https://api.todoist.com/rest/v2/tasks", headers=headers, json=payload)
            if resp.status_code >= 300:
                raise RuntimeError(f"Todoist create_task failed ({resp.status_code}): {resp.text[:300]}")
            return _text_result({"task": resp.json()})

        if tool_name == "todoist.complete_task":
            task_id = str(arguments.get("task_id") or "").strip()
            if not task_id:
                raise ValueError("task_id is required")
            resp = client.post(f"https://api.todoist.com/api/v1/tasks/{task_id}/close", headers=headers)
            if resp.status_code not in (200, 204):
                raise RuntimeError(f"Todoist complete_task failed ({resp.status_code}): {resp.text[:300]}")
            return _text_result({"task_id": task_id, "completed": True})

    raise ValueError(f"Unsupported tool for todoist-mcp: {tool_name}")


def _dispatch_brave(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    token = _require_secret(settings.BRAVE_API_KEY, "BRAVE_API_KEY")
    headers = {"X-Subscription-Token": token}
    query = str(arguments.get("query") or "").strip()
    if not query:
        raise ValueError("query is required")
    count = int(arguments.get("count") or 5)

    endpoint = {
        "brave.search": "https://api.search.brave.com/res/v1/web/search",
        "brave.news": "https://api.search.brave.com/res/v1/news/search",
        "brave.images": "https://api.search.brave.com/res/v1/images/search",
    }.get(tool_name)
    if not endpoint:
        raise ValueError(f"Unsupported tool for brave-search-mcp: {tool_name}")

    with httpx.Client(timeout=20.0) as client:
        resp = client.get(endpoint, headers=headers, params={"q": query, "count": count})
        if resp.status_code >= 300:
            raise RuntimeError(f"Brave {tool_name} failed ({resp.status_code}): {resp.text[:300]}")
        data = resp.json()

    if tool_name == "brave.search":
        rows = ((data.get("web") or {}).get("results") or [])
    else:
        rows = data.get("results") or []
    return _text_result({"results": rows, "count": len(rows)})


def _dispatch_github(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    token = _github_installation_token()
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }

    with httpx.Client(timeout=20.0) as client:
        if tool_name == "github.list_repos":
            max_results = int(arguments.get("max_results") or 30)
            resp = client.get(
                "https://api.github.com/installation/repositories",
                headers=headers,
                params={"per_page": max_results},
            )
            if resp.status_code >= 300:
                raise RuntimeError(f"GitHub list_repos failed ({resp.status_code}): {resp.text[:300]}")
            data = resp.json()
            repos = data.get("repositories") or []
            return _text_result({"repositories": repos, "count": len(repos)})

        if tool_name == "github.search_issues":
            query = str(arguments.get("query") or "").strip()
            if not query:
                raise ValueError("query is required")
            max_results = int(arguments.get("max_results") or 20)
            resp = client.get(
                "https://api.github.com/search/issues",
                headers=headers,
                params={"q": query, "per_page": max_results},
            )
            if resp.status_code >= 300:
                raise RuntimeError(f"GitHub search_issues failed ({resp.status_code}): {resp.text[:300]}")
            data = resp.json()
            return _text_result({"items": data.get("items") or [], "total_count": int(data.get("total_count") or 0)})

        if tool_name == "github.create_issue":
            owner = str(arguments.get("owner") or "").strip()
            repo = str(arguments.get("repo") or "").strip()
            title = str(arguments.get("title") or "").strip()
            if not owner or not repo or not title:
                raise ValueError("owner, repo, and title are required")
            payload = {"title": title}
            if arguments.get("body"):
                payload["body"] = str(arguments.get("body"))
            resp = client.post(
                f"https://api.github.com/repos/{owner}/{repo}/issues",
                headers=headers,
                json=payload,
            )
            if resp.status_code >= 300:
                raise RuntimeError(f"GitHub create_issue failed ({resp.status_code}): {resp.text[:300]}")
            return _text_result({"issue": resp.json()})

    raise ValueError(f"Unsupported tool for github-mcp: {tool_name}")


def _dispatch_apple(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    # Reuse the existing custom Apple reminders handlers.
    return handle_apple_tool_call(name=tool_name, arguments=arguments)


def _dispatch_tool(server_id: str, tool_name: str, arguments: dict[str, Any], db: Session) -> dict[str, Any]:
    if server_id == "google-calendar-mcp":
        return _dispatch_google_calendar(db, tool_name, arguments)
    if server_id == "google-drive-mcp":
        return _dispatch_google_drive(db, tool_name, arguments)
    if server_id == "gmail-mcp":
        return _dispatch_gmail(db, tool_name, arguments)
    if server_id == "notion-mcp":
        return _dispatch_notion(tool_name, arguments)
    if server_id == "todoist-mcp":
        return _dispatch_todoist(tool_name, arguments)
    if server_id == "brave-search-mcp":
        return _dispatch_brave(tool_name, arguments)
    if server_id == "github-mcp":
        return _dispatch_github(tool_name, arguments)
    if server_id == "apple-reminders-mcp":
        return _dispatch_apple(tool_name, arguments)
    raise ValueError(f"Unknown Wave 1 server_id: {server_id}")


@router.post("/mcp/wave1/{server_id}")
async def wave1_mcp_host(server_id: str, request: Request, db: Session = Depends(get_db)):
    configured_token = str(settings.MCP_HOST_TOKEN or "").strip()
    if configured_token:
        header_token = str(request.headers.get("X-MCP-Host-Token") or "")
        if not hmac.compare_digest(header_token, configured_token):
            return JSONResponse(_error(None, "Unauthorized", code=-32001), status_code=401)

    if server_id not in SERVER_TOOLS:
        return JSONResponse(_error(None, f"Unknown server_id: {server_id}", code=-32601), status_code=404)

    try:
        payload = await request.json()
    except Exception:
        return JSONResponse(_error(None, "Invalid JSON payload", code=-32700), status_code=400)

    if not isinstance(payload, dict):
        return JSONResponse(_error(None, "JSON-RPC payload must be an object", code=-32600), status_code=400)

    method = str(payload.get("method") or "").strip()
    payload_id = payload.get("id")
    params = payload.get("params") if isinstance(payload.get("params"), dict) else {}

    # Notification payloads don't need a response body.
    if payload_id is None:
        return Response(status_code=204)

    if method == "initialize":
        return JSONResponse(
            _ok(
                payload_id,
                {
                    "protocolVersion": "2025-03-26",
                    "serverInfo": {"name": f"executive-os-{server_id}", "version": "0.1.0"},
                    "capabilities": {"tools": {}, "resources": {"subscribe": False}, "prompts": {}},
                },
            ),
            headers={"Mcp-Session-Id": f"wave1-{server_id}"},
        )

    if method == "tools/list":
        return JSONResponse(_ok(payload_id, {"tools": SERVER_TOOLS[server_id]}))

    if method == "resources/list":
        return JSONResponse(_ok(payload_id, {"resources": []}))

    if method == "prompts/list":
        return JSONResponse(_ok(payload_id, {"prompts": []}))

    if method == "ping":
        return JSONResponse(_ok(payload_id, {}))

    if method in {"tools/call", "tool/call"}:
        tool_name = str(params.get("name") or "").strip()
        arguments = params.get("arguments") if isinstance(params.get("arguments"), dict) else {}
        try:
            result = _dispatch_tool(server_id, tool_name, arguments, db)
            return JSONResponse(_ok(payload_id, result))
        except Exception as exc:
            return JSONResponse(_error(payload_id, str(exc), code=-32000), status_code=200)

    return JSONResponse(_error(payload_id, f"Unsupported method: {method}", code=-32601), status_code=200)
