from __future__ import annotations

import json
import logging
from typing import Any

import requests
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.provisioning_pipeline import ensure_provisioning_tables

logger = logging.getLogger(__name__)


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


def _parse_list(value: Any) -> list[str]:
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    if isinstance(value, str) and value.strip():
        raw = value.strip()
        if raw.startswith("[") and raw.endswith("]"):
            try:
                parsed = json.loads(raw)
                if isinstance(parsed, list):
                    return [str(item).strip() for item in parsed if str(item).strip()]
            except Exception:
                pass
        return [part.strip() for part in raw.replace("|", ",").split(",") if part.strip()]
    return []


def _parse_dict(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    if isinstance(value, str) and value.strip():
        try:
            parsed = json.loads(value)
            if isinstance(parsed, dict):
                return parsed
        except Exception:
            return {}
    return {}


def _normalize_auth_type(value: Any) -> str:
    raw = str(value or "oauth2").strip().lower()
    if raw in {"oauth", "oauth2", "oauth2_consolidated"}:
        return raw if raw != "oauth" else "oauth2"
    if raw in {"api_key", "pre_provisioned", "plaid_link", "tesla_sso"}:
        return raw
    return "oauth2"


def _normalize_entry(raw: dict[str, Any]) -> dict[str, Any] | None:
    server_id = str(raw.get("server_id") or raw.get("id") or "").strip().lower()
    if not server_id:
        return None
    approved = raw.get("approved")
    status = str(raw.get("status") or "").strip().lower()
    if not status:
        status = "active" if approved is not False else "pending"
    return {
        "server_id": server_id,
        "display_name": str(raw.get("display_name") or raw.get("name") or server_id).strip(),
        "description": str(raw.get("description") or raw.get("summary") or "MCP server capability").strip(),
        "auth_type": _normalize_auth_type(raw.get("auth_type")),
        "min_plan": str(raw.get("min_plan") or raw.get("plan") or "professional").strip().lower(),
        "setup_seconds": max(10, int(raw.get("setup_seconds") or 30)),
        "capabilities": _parse_list(raw.get("capabilities")),
        "keywords": _parse_list(raw.get("keywords")),
        "hosting_model": str(raw.get("hosting_model") or "").strip().lower(),
        "oauth_config": _parse_dict(raw.get("oauth_config")),
        "container_image": str(raw.get("container_image") or "").strip(),
        "source": str(raw.get("source") or "remote").strip().lower() or "remote",
        "signature": str(raw.get("signature") or "").strip() or None,
        "status": status,
    }


def _extract_entries(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        rows = payload
    elif isinstance(payload, dict):
        rows = payload.get("items") or payload.get("servers") or payload.get("results") or payload.get("entries") or []
    else:
        rows = []
    out: list[dict[str, Any]] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        entry = _normalize_entry(row)
        if entry is not None:
            out.append(entry)
    return out


def _catalog_headers() -> dict[str, str]:
    headers: dict[str, str] = {"Accept": "application/json"}
    api_key = str(settings.REMOTE_CATALOG_API_KEY or "").strip()
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
        headers["X-API-Key"] = api_key
    return headers


def _query_remote_catalog(url: str, *, params: dict[str, Any]) -> list[dict[str, Any]]:
    timeout = max(2, int(settings.REMOTE_CATALOG_TIMEOUT_SECONDS or 8))
    headers = _catalog_headers()
    response = requests.get(url, params=params, headers=headers, timeout=timeout)
    response.raise_for_status()
    data = response.json()
    return _extract_entries(data)


def search_remote_catalog(
    *,
    capability: str,
    limit: int = 10,
) -> list[dict[str, Any]]:
    base = str(settings.REMOTE_CATALOG_API_URL or "").strip().rstrip("/")
    if not base:
        return []

    q = str(capability or "").strip()
    if not q:
        return []

    safe_limit = max(1, min(25, int(limit or 10)))
    urls = [f"{base}/search", f"{base}/catalog/search", base]
    for url in urls:
        try:
            rows = _query_remote_catalog(
                url,
                params={
                    "capability": q,
                    "q": q,
                    "query": q,
                    "limit": safe_limit,
                },
            )
            if rows:
                deduped: dict[str, dict[str, Any]] = {}
                for row in rows:
                    deduped[str(row.get("server_id") or "").strip().lower()] = row
                return list(deduped.values())[:safe_limit]
        except Exception:
            logger.warning("remote_catalog_search_failed url=%s", url, exc_info=True)
            continue
    return []


def fetch_remote_catalog_snapshot() -> list[dict[str, Any]]:
    base = str(settings.REMOTE_CATALOG_API_URL or "").strip().rstrip("/")
    if not base:
        return []
    urls = [f"{base}/catalog", f"{base}/entries", base]
    for url in urls:
        try:
            rows = _query_remote_catalog(url, params={})
            if rows:
                deduped: dict[str, dict[str, Any]] = {}
                for row in rows:
                    deduped[str(row.get("server_id") or "").strip().lower()] = row
                return list(deduped.values())
        except Exception:
            logger.warning("remote_catalog_snapshot_fetch_failed url=%s", url, exc_info=True)
            continue
    return []


def upsert_remote_entries(
    db: Session,
    *,
    entries: list[dict[str, Any]],
    mark_missing_as_deprecated: bool,
) -> dict[str, Any]:
    ensure_provisioning_tables(db)
    if not _table_exists(db, "server_catalog"):
        return {"ok": False, "upserted": 0, "deprecated": 0, "reason": "server_catalog_missing"}

    dialect = db.bind.dialect.name if db.bind is not None else ""
    seen: set[str] = set()
    upserted = 0

    for raw in entries:
        entry = _normalize_entry(raw if isinstance(raw, dict) else {})
        if entry is None:
            continue
        server_id = str(entry["server_id"])
        seen.add(server_id)
        params = {
            "server_id": server_id,
            "display_name": entry["display_name"],
            "description": entry["description"],
            "auth_type": entry["auth_type"],
            "min_plan": entry["min_plan"],
            "setup_seconds": int(entry["setup_seconds"]),
            "capabilities": json.dumps(entry["capabilities"], ensure_ascii=False),
            "keywords": json.dumps(entry["keywords"], ensure_ascii=False),
            "hosting_model": entry["hosting_model"],
            "oauth_config": json.dumps(entry["oauth_config"], ensure_ascii=False),
            "container_image": entry["container_image"],
            "source": "remote",
            "signature": entry["signature"],
            "status": entry["status"],
        }
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    insert into server_catalog (
                      server_id, display_name, description, auth_type, min_plan, setup_seconds,
                      capabilities, keywords, hosting_model, oauth_config, container_image, source, signature, status
                    ) values (
                      :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds,
                      :capabilities, :keywords, :hosting_model, :oauth_config, :container_image, :source, :signature, :status
                    )
                    on conflict(server_id) do update set
                      display_name = excluded.display_name,
                      description = excluded.description,
                      auth_type = excluded.auth_type,
                      min_plan = excluded.min_plan,
                      setup_seconds = excluded.setup_seconds,
                      capabilities = excluded.capabilities,
                      keywords = excluded.keywords,
                      hosting_model = excluded.hosting_model,
                      oauth_config = excluded.oauth_config,
                      container_image = excluded.container_image,
                      source = excluded.source,
                      signature = excluded.signature,
                      status = excluded.status,
                      updated_at = current_timestamp
                    """
                ),
                params,
            )
        else:
            db.execute(
                text(
                    """
                    insert into server_catalog (
                      server_id, display_name, description, auth_type, min_plan, setup_seconds,
                      capabilities, keywords, hosting_model, oauth_config, container_image, source, signature, status
                    ) values (
                      :server_id, :display_name, :description, :auth_type, :min_plan, :setup_seconds,
                      cast(:capabilities as jsonb), cast(:keywords as jsonb), :hosting_model, cast(:oauth_config as jsonb),
                      :container_image, :source, :signature, :status
                    )
                    on conflict (server_id) do update set
                      display_name = excluded.display_name,
                      description = excluded.description,
                      auth_type = excluded.auth_type,
                      min_plan = excluded.min_plan,
                      setup_seconds = excluded.setup_seconds,
                      capabilities = excluded.capabilities,
                      keywords = excluded.keywords,
                      hosting_model = excluded.hosting_model,
                      oauth_config = excluded.oauth_config,
                      container_image = excluded.container_image,
                      source = excluded.source,
                      signature = excluded.signature,
                      status = excluded.status,
                      updated_at = now()
                    """
                ),
                params,
            )
        upserted += 1

    deprecated = 0
    if mark_missing_as_deprecated:
        rows = db.execute(
            text("select server_id from server_catalog where source = 'remote' and coalesce(status, 'active') <> 'deprecated'")
        ).all()
        for row in rows:
            server_id = str((row or [None])[0] or "").strip().lower()
            if not server_id or server_id in seen:
                continue
            if dialect == "sqlite":
                db.execute(
                    text(
                        "update server_catalog "
                        "set status = 'deprecated', updated_at = current_timestamp "
                        "where server_id = :server_id"
                    ),
                    {"server_id": server_id},
                )
            else:
                db.execute(
                    text(
                        "update server_catalog "
                        "set status = 'deprecated', updated_at = now() "
                        "where server_id = :server_id"
                    ),
                    {"server_id": server_id},
                )
            deprecated += 1

    db.commit()
    return {"ok": True, "upserted": upserted, "deprecated": deprecated, "seen_count": len(seen)}


def sync_remote_catalog(db: Session) -> dict[str, Any]:
    if not settings.PROVISIONING_REMOTE_SYNC_ENABLED:
        return {"ok": False, "reason": "remote_sync_disabled"}
    if not str(settings.REMOTE_CATALOG_API_URL or "").strip():
        return {"ok": False, "reason": "remote_catalog_url_missing"}
    entries = fetch_remote_catalog_snapshot()
    if not entries:
        return {"ok": False, "reason": "remote_catalog_empty"}
    result = upsert_remote_entries(db, entries=entries, mark_missing_as_deprecated=True)
    return {"ok": bool(result.get("ok")), "entries_fetched": len(entries), **result}
