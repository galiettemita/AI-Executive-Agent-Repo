from __future__ import annotations

import hashlib
import hmac
import json
import re
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


@dataclass(frozen=True)
class CatalogEntry:
    server_id: str
    description: str
    auth_type: str
    display_name: str = ""
    setup_seconds: int = 30
    min_plan: str = "free"
    capabilities: tuple[str, ...] = ()
    keywords: tuple[str, ...] = ()
    hosting_model: str = ""
    container_image: str = ""
    source: str = "local"
    signature: str | None = None
    status: str = "active"


_PLAN_RANK = {
    "free": 0,
    "trial": 0,
    "starter": 1,
    "personal": 1,
    "plus": 2,
    "professional": 3,
    "pro": 3,
}

_WAVE1_DEFAULT_CATALOG: tuple[CatalogEntry, ...] = (
    CatalogEntry(
        server_id="google-calendar-mcp",
        description="Calendar read/write operations for scheduling and conflict handling.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("calendar", "schedule", "events", "meeting"),
        keywords=("wave1", "calendar", "google"),
    ),
    CatalogEntry(
        server_id="google-drive-mcp",
        description="Drive file discovery and retrieval for context injection.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("files", "drive", "documents", "storage"),
        keywords=("wave1", "drive", "google"),
    ),
    CatalogEntry(
        server_id="gmail-mcp",
        description="Gmail read/search/send tool surface.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("email", "gmail", "inbox", "mail"),
        keywords=("wave1", "gmail", "google"),
    ),
    CatalogEntry(
        server_id="notion-mcp",
        description="Notion search/read/write operations for workspace docs.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("notes", "docs", "wiki", "knowledge"),
        keywords=("wave1", "notion", "knowledge"),
    ),
    CatalogEntry(
        server_id="todoist-mcp",
        description="Task capture and task-state synchronization.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("tasks", "todo", "reminders", "project"),
        keywords=("wave1", "todoist", "tasks"),
    ),
    CatalogEntry(
        server_id="brave-search-mcp",
        description="Web search enrichment for research and grounding.",
        auth_type="api_key",
        setup_seconds=30,
        min_plan="free",
        capabilities=("search", "web", "research", "news"),
        keywords=("wave1", "search", "brave"),
    ),
    CatalogEntry(
        server_id="github-mcp",
        description="Repository/issue/PR operations for engineering workflows.",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("github", "code", "issues", "pull requests"),
        keywords=("wave1", "github", "engineering"),
    ),
    CatalogEntry(
        server_id="apple-reminders-mcp",
        description="Apple Reminders list/read/write/complete operations.",
        auth_type="pre_provisioned",
        setup_seconds=30,
        min_plan="free",
        capabilities=("reminders", "tasks", "checklist", "apple"),
        keywords=("wave1", "apple", "reminders"),
    ),
)

_EXTRA_CATALOG: tuple[CatalogEntry, ...] = (
    CatalogEntry(
        server_id="duffel-mcp",
        description="Flight search and booking",
        auth_type="api_key",
        setup_seconds=30,
        min_plan="free",
        capabilities=("flight", "flights", "airfare", "travel booking", "book flight"),
        keywords=("duffel", "trip", "airport", "ticket"),
    ),
    CatalogEntry(
        server_id="zoom-mcp",
        description="Meeting management, recordings, transcripts",
        auth_type="oauth",
        setup_seconds=30,
        min_plan="free",
        capabilities=("zoom", "meeting", "recording", "transcript"),
        keywords=("video", "call", "conference"),
    ),
    CatalogEntry(
        server_id="plaid-mcp",
        description="Bank balances and transactions",
        auth_type="plaid_link",
        setup_seconds=60,
        min_plan="professional",
        capabilities=("bank", "transactions", "spending", "finance"),
        keywords=("plaid", "balance", "account"),
    ),
    CatalogEntry(
        server_id="tesla-mcp",
        description="Vehicle control, charge status, climate",
        auth_type="tesla_sso",
        setup_seconds=45,
        min_plan="professional",
        capabilities=("tesla", "vehicle", "charging", "climate"),
        keywords=("car", "ev"),
    ),
)

_CAPABILITY_HINTS: dict[str, str] = {
    "flight": "duffel-mcp",
    "flights": "duffel-mcp",
    "airfare": "duffel-mcp",
    "book": "duffel-mcp",
    "meeting": "zoom-mcp",
    "zoom": "zoom-mcp",
    "bank": "plaid-mcp",
    "transaction": "plaid-mcp",
    "transactions": "plaid-mcp",
    "tesla": "tesla-mcp",
}


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _plan_rank(plan: str | None) -> int:
    return _PLAN_RANK.get(str(plan or "free").strip().lower(), 0)


def _plan_allowed(*, plan: str | None, min_plan: str | None) -> bool:
    return _plan_rank(plan) >= _plan_rank(min_plan)


def _resolve_user_plan(db: Session, *, user_id: str | None) -> str:
    if not user_id:
        return "free"
    if not _table_exists(db, "subscriptions"):
        return "free"
    try:
        row = db.execute(
            text(
                "select plan, status from subscriptions "
                "where user_id = :user_id order by updated_at desc limit 1"
            ),
            {"user_id": user_id},
        ).mappings().first()
    except Exception:
        return "free"
    if not row:
        return "free"
    status = str(row.get("status") or "active").strip().lower()
    plan = str(row.get("plan") or "free").strip().lower()
    if status in {"active", "trialing", "pending"}:
        return plan
    return "free"


def _safe_parse_list(value: Any) -> tuple[str, ...]:
    if isinstance(value, (list, tuple)):
        return tuple(str(item).strip() for item in value if str(item).strip())
    if isinstance(value, str):
        raw = value.strip()
        if not raw:
            return ()
        if raw.startswith("[") and raw.endswith("]"):
            # Best-effort parse for JSON-like arrays without importing heavy libs.
            tokens = re.findall(r'"([^"]+)"|\'([^\']+)\'', raw)
            out: list[str] = []
            for pair in tokens:
                candidate = pair[0] or pair[1]
                if candidate:
                    out.append(candidate.strip())
            if out:
                return tuple(out)
        return tuple(token.strip() for token in re.split(r"[,\n|]+", raw) if token.strip())
    return ()


def _canonical_catalog_payload(entry: dict[str, Any]) -> str:
    payload = {
        "server_id": str(entry.get("server_id") or "").strip().lower(),
        "display_name": str(entry.get("display_name") or "").strip(),
        "description": str(entry.get("description") or "").strip(),
        "auth_type": str(entry.get("auth_type") or "oauth").strip().lower(),
        "min_plan": str(entry.get("min_plan") or "free").strip().lower(),
        "setup_seconds": int(entry.get("setup_seconds") or 30),
        "capabilities": sorted({str(v).strip().lower() for v in (entry.get("capabilities") or []) if str(v).strip()}),
        "keywords": sorted({str(v).strip().lower() for v in (entry.get("keywords") or []) if str(v).strip()}),
        "hosting_model": str(entry.get("hosting_model") or "").strip().lower(),
        "container_image": str(entry.get("container_image") or "").strip().lower(),
        "source": str(entry.get("source") or "local").strip().lower(),
        "status": str(entry.get("status") or "active").strip().lower(),
    }
    return json.dumps(payload, sort_keys=True, ensure_ascii=False, separators=(",", ":"))


def compute_catalog_signature(entry: dict[str, Any], *, secret: str) -> str:
    key = str(secret or "").encode("utf-8")
    body = _canonical_catalog_payload(entry).encode("utf-8")
    return hmac.new(key, body, hashlib.sha256).hexdigest()


def verify_catalog_entry_signature(entry: dict[str, Any], *, secret: str) -> bool:
    provided = str(entry.get("signature") or "").strip().lower()
    if not provided:
        return False
    expected = compute_catalog_signature(entry, secret=secret).lower()
    return hmac.compare_digest(provided, expected)


def is_container_image_allowed(image: str, *, allowed_prefixes: list[str]) -> bool:
    normalized = str(image or "").strip().lower()
    if not normalized:
        return True
    if not allowed_prefixes:
        return True
    return any(normalized.startswith(prefix.lower()) for prefix in allowed_prefixes)


def _default_catalog_entries() -> list[CatalogEntry]:
    items: list[CatalogEntry] = list(_WAVE1_DEFAULT_CATALOG)
    items.extend(_EXTRA_CATALOG)
    return items


def _catalog_from_table(db: Session) -> list[CatalogEntry]:
    if not _table_exists(db, "server_catalog"):
        return []
    try:
        rows = db.execute(text("select * from server_catalog order by server_id asc")).mappings().all()
    except Exception:
        return []
    out: list[CatalogEntry] = []
    for row in rows:
        server_id = str(row.get("server_id") or row.get("id") or "").strip()
        if not server_id:
            continue
        status = str(row.get("status") or "active").strip().lower()
        if status and status not in {"active", "approved", "ready"}:
            continue
        display_name = str(row.get("display_name") or server_id).strip() or server_id
        description = str(row.get("description") or row.get("summary") or "").strip() or "MCP server capability"
        auth_type = str(row.get("auth_type") or row.get("auth") or "oauth").strip().lower()
        setup_seconds = int(row.get("setup_seconds") or row.get("setup_time_seconds") or 30)
        min_plan = str(row.get("min_plan") or row.get("required_plan") or "free").strip().lower()
        capabilities = _safe_parse_list(row.get("capabilities") or row.get("capabilities_json") or row.get("capability_tags"))
        keywords = _safe_parse_list(row.get("keywords") or row.get("tags"))
        hosting_model = str(row.get("hosting_model") or "").strip().lower()
        container_image = str(row.get("container_image") or "").strip()
        source = str(row.get("source") or "local").strip().lower() or "local"
        signature = str(row.get("signature") or "").strip() or None
        out.append(
            CatalogEntry(
                server_id=server_id,
                display_name=display_name,
                description=description,
                auth_type=auth_type or "oauth",
                setup_seconds=max(10, setup_seconds),
                min_plan=min_plan or "free",
                capabilities=capabilities,
                keywords=keywords,
                hosting_model=hosting_model,
                container_image=container_image,
                source=source,
                signature=signature,
                status=status or "active",
            )
        )
    return out


def _declined_server_ids(db: Session, *, user_id: str | None, now_utc: datetime) -> set[str]:
    if not user_id or not _table_exists(db, "provisioning_declined"):
        return set()
    threshold = now_utc - timedelta(days=7)
    queries = (
        (
            "select server_id from provisioning_declined "
            "where user_id = :user_id and declined_at >= :threshold",
            {"user_id": user_id, "threshold": threshold},
        ),
        (
            "select server_id from provisioning_declined "
            "where user_id = :user_id and created_at >= :threshold",
            {"user_id": user_id, "threshold": threshold},
        ),
        (
            "select server_id from provisioning_declined "
            "where user_id = :user_id and cooldown_until > :now_utc",
            {"user_id": user_id, "now_utc": now_utc},
        ),
    )
    for sql, params in queries:
        try:
            rows = db.execute(text(sql), params).all()
            if rows:
                return {str(row[0]).strip() for row in rows if row and row[0]}
        except Exception:
            continue
    return set()


def available_servers_for_user(
    db: Session,
    *,
    user_id: str | None,
    connected_server_ids: set[str] | None = None,
) -> list[dict[str, Any]]:
    now_utc = datetime.now(timezone.utc)
    plan = _resolve_user_plan(db, user_id=user_id)
    connected = {str(item).strip() for item in (connected_server_ids or set()) if str(item).strip()}
    declined = _declined_server_ids(db, user_id=user_id, now_utc=now_utc)

    catalog = _catalog_from_table(db)
    if not catalog:
        catalog = _default_catalog_entries()

    items: list[dict[str, Any]] = []
    for entry in catalog:
        if entry.server_id in connected:
            continue
        if entry.server_id in declined:
            continue
        if not _plan_allowed(plan=plan, min_plan=entry.min_plan):
            continue
        items.append(
            {
                "server_id": entry.server_id,
                "display_name": entry.display_name or entry.server_id,
                "description": entry.description,
                "auth_type": entry.auth_type,
                "setup_seconds": entry.setup_seconds,
                "min_plan": entry.min_plan,
                "capabilities": list(entry.capabilities),
                "keywords": list(entry.keywords),
                "hosting_model": entry.hosting_model,
                "container_image": entry.container_image,
                "source": entry.source,
                "signature": entry.signature,
                "status": entry.status,
            }
        )
    items.sort(key=lambda item: str(item.get("server_id") or ""))
    return items


def get_catalog_entry(db: Session, *, server_id: str) -> dict[str, Any] | None:
    target = str(server_id or "").strip().lower()
    if not target:
        return None
    catalog = _catalog_from_table(db)
    if not catalog:
        catalog = _default_catalog_entries()
    for entry in catalog:
        if str(entry.server_id or "").strip().lower() != target:
            continue
        return {
            "server_id": entry.server_id,
            "display_name": entry.display_name or entry.server_id,
            "description": entry.description,
            "auth_type": entry.auth_type,
            "setup_seconds": entry.setup_seconds,
            "min_plan": entry.min_plan,
            "capabilities": list(entry.capabilities),
            "keywords": list(entry.keywords),
            "hosting_model": entry.hosting_model,
            "container_image": entry.container_image,
            "source": entry.source,
            "signature": entry.signature,
            "status": entry.status,
        }
    return None


def all_catalog_entries(db: Session) -> list[dict[str, Any]]:
    catalog = _catalog_from_table(db)
    if not catalog:
        catalog = _default_catalog_entries()
    out: list[dict[str, Any]] = []
    for entry in catalog:
        out.append(
            {
                "server_id": entry.server_id,
                "display_name": entry.display_name or entry.server_id,
                "description": entry.description,
                "auth_type": entry.auth_type,
                "setup_seconds": entry.setup_seconds,
                "min_plan": entry.min_plan,
                "capabilities": list(entry.capabilities),
                "keywords": list(entry.keywords),
                "hosting_model": entry.hosting_model,
                "container_image": entry.container_image,
                "source": entry.source,
                "signature": entry.signature,
                "status": entry.status,
            }
        )
    return out


def _setup_label(seconds: int) -> str:
    value = max(10, int(seconds or 30))
    if value < 60:
        return f"~{value}s"
    minutes = int(round(value / 60.0))
    return f"~{minutes}m"


def render_available_servers_section(entries: list[dict[str, Any]]) -> str:
    lines = ["## Available Servers (Not Connected)"]
    if not entries:
        lines.append("- None available on your current plan.")
        return "\n".join(lines)
    for item in entries:
        server_id = str(item.get("server_id") or "").strip()
        if not server_id:
            continue
        description = str(item.get("description") or "MCP server capability").strip()
        auth_type = str(item.get("auth_type") or "oauth").strip().lower()
        setup_seconds = int(item.get("setup_seconds") or 30)
        lines.append(
            f"- {server_id}: {description} | auth: {auth_type} | setup: {_setup_label(setup_seconds)}"
        )
    return "\n".join(lines)


def parse_available_servers_section(tools_markdown: str) -> list[dict[str, Any]]:
    content = str(tools_markdown or "")
    if not content.strip():
        return []
    rows = content.splitlines()
    in_section = False
    out: list[dict[str, Any]] = []
    for raw in rows:
        line = raw.strip()
        if line.startswith("## "):
            in_section = line.lower() == "## available servers (not connected)"
            continue
        if not in_section or not line.startswith("- "):
            continue
        body = line[2:].strip()
        if not body or body.lower().startswith("none available"):
            continue
        match = re.match(
            r"^(?P<server_id>[a-zA-Z0-9._-]+)\s*:\s*(?P<description>.*?)\s*\|\s*auth:\s*(?P<auth>[^|]+)\s*\|\s*setup:\s*(?P<setup>.+)$",
            body,
        )
        if not match:
            continue
        server_id = str(match.group("server_id") or "").strip()
        if not server_id:
            continue
        setup_raw = str(match.group("setup") or "").strip().lower()
        setup_seconds = 30
        if setup_raw.endswith("m"):
            cleaned = setup_raw.lstrip("~").rstrip("m").strip()
            try:
                setup_seconds = int(float(cleaned) * 60.0)
            except Exception:
                setup_seconds = 60
        else:
            digits = re.findall(r"\d+", setup_raw)
            if digits:
                setup_seconds = int(digits[0])
        out.append(
            {
                "server_id": server_id,
                "description": str(match.group("description") or "").strip() or "MCP server capability",
                "auth_type": str(match.group("auth") or "oauth").strip().lower(),
                "setup_seconds": max(10, setup_seconds),
                "min_plan": "free",
                "capabilities": [],
                "keywords": [],
            }
        )
    return out


def _entry_score(entry: dict[str, Any], tokens: set[str]) -> int:
    server_id = str(entry.get("server_id") or "").strip().lower()
    description = str(entry.get("description") or "").strip().lower()
    capabilities = " ".join(str(item).lower() for item in (entry.get("capabilities") or []))
    keywords = " ".join(str(item).lower() for item in (entry.get("keywords") or []))
    haystack = " ".join([server_id, description, capabilities, keywords]).strip()
    if not haystack:
        return 0

    score = 0
    for token in tokens:
        if token in haystack:
            score += 1
        hinted = _CAPABILITY_HINTS.get(token)
        if hinted and hinted == server_id:
            score += 4
    return score


def find_server_match(
    capability_text: str,
    *,
    entries: list[dict[str, Any]],
) -> dict[str, Any] | None:
    if not entries:
        return None
    tokens = {token for token in re.findall(r"[a-z0-9]+", str(capability_text or "").lower()) if token}
    if not tokens:
        return None

    # Enrich parsed entries with defaults when known.
    defaults = {item.server_id: item for item in _default_catalog_entries()}
    enriched: list[dict[str, Any]] = []
    for entry in entries:
        server_id = str(entry.get("server_id") or "").strip()
        base = defaults.get(server_id)
        merged = dict(entry)
        if base:
            merged["capabilities"] = list(entry.get("capabilities") or base.capabilities)
            merged["keywords"] = list(entry.get("keywords") or base.keywords)
            merged["description"] = str(entry.get("description") or base.description)
            merged["auth_type"] = str(entry.get("auth_type") or base.auth_type)
            merged["setup_seconds"] = int(entry.get("setup_seconds") or base.setup_seconds)
        enriched.append(merged)

    best: dict[str, Any] | None = None
    best_score = 0
    for entry in enriched:
        score = _entry_score(entry, tokens)
        if score > best_score:
            best = entry
            best_score = score
    if best_score <= 0:
        return None
    return best
