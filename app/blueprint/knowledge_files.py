from __future__ import annotations

import hashlib
import json
import logging
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.redis import cache_delete, cache_get_json, cache_set_json
from app.core.storage import S3Storage


logger = logging.getLogger(__name__)


DEFAULT_KNOWLEDGE_TEMPLATES: dict[str, dict[str, str]] = {
    "USER.md": {
        "layer": "brain",
        "content": """# USER.md\n## BASICS\n- Name: {display_name}\n- Timezone: {timezone}\n\n## WORK\n- Role:\n- Company:\n- Key goals:\n\n## PEOPLE\n- Key stakeholders:\n\n## OPERATIONS\n- Daily patterns:\n- Meeting preferences:\n\n## FRICTION\n- Repeating blockers:\n""",
    },
    "SOUL.md": {
        "layer": "brain",
        "content": """# SOUL.md\n## Character\n- Clear, concise, proactive.\n\n## Voice\n- Confident and brief by default.\n\n## Boundaries\n- Ask before high-risk actions.\n""",
    },
    "IDENTITY.md": {
        "layer": "brain",
        "content": """# IDENTITY.md\n## Name\n- Executive OS\n\n## Greeting\n- Keep greetings short and action-oriented.\n""",
    },
    "AGENTS.md": {
        "layer": "dna",
        "content": """# AGENTS.md\n## Autonomy\n- Default to draft/preview for medium+ risk actions.\n\n## Delegation Preferences\n- Prefer explicit deadlines and owner assignment.\n""",
    },
    "MEMORY.md": {
        "layer": "dna",
        "content": """# MEMORY.md\n## Retention Rules\n- Keep durable preferences and stable facts.\n- Expire stale episodic context.\n""",
    },
    "HEARTBEAT.md": {
        "layer": "brain",
        "content": """# HEARTBEAT.md\n## This Week\n-\n\n## This Month\n-\n\n## Blocked Items\n-\n\n## Delegation Tracker\n-\n""",
    },
    "TOOLS.md": {
        "layer": "bones",
        "content": """# TOOLS.md\n## Connected Services\n-\n\n## Tool Preferences\n-\n\n## Cost Limits\n-\n""",
    },
    "TEAM.md": {
        "layer": "brain",
        "content": """# TEAM.md\n## Direct Reports\n-\n\n## Key Stakeholders\n-\n\n## Communication Preferences\n-\n\n## Delegation Track Record\n-\n""",
    },
    "WORKFLOWS.md": {
        "layer": "dna",
        "content": """# WORKFLOWS.md\n## Active Workflows\n-\n\n## Workflow Templates\n-\n\n## Trigger Definitions\n-\n\n## Error Handling Preferences\n-\n""",
    },
}

HOT_CACHE_FILES = {"IDENTITY.md", "SOUL.md", "AGENTS.md"}


def _render(content: str, *, display_name: str, timezone: str) -> str:
    return content.format(display_name=display_name or "User", timezone=timezone or "America/New_York")


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
        row = db.execute(
            text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _cache_key(user_id: str, file_path: str) -> str:
    return f"bp:knowledge:latest:{user_id}:{file_path}"


def _snapshot_key(user_id: str, file_path: str, version: int) -> str:
    safe_file = (file_path or "unknown").replace("\\", "/").strip("/").replace("/", "__")
    return f"knowledge_snapshots/{user_id}/{safe_file}/v{version}.json"


def _snapshot_bucket() -> str:
    return (settings.S3_KNOWLEDGE_BUCKET or settings.S3_BUCKET or "").strip()


def _write_snapshot(
    *,
    user_id: str,
    file_path: str,
    layer: str,
    version: int,
    content: str,
    metadata: dict[str, Any],
) -> dict[str, str] | None:
    bucket = _snapshot_bucket()
    if not bucket:
        return None
    key = _snapshot_key(user_id, file_path, version)
    payload = {
        "user_id": user_id,
        "file_path": file_path,
        "layer": layer,
        "version": version,
        "content": content,
        "metadata": metadata,
        "created_at": datetime.utcnow().isoformat(),
    }
    try:
        storage = S3Storage(
            bucket=bucket,
            region=settings.S3_REGION or settings.AWS_REGION,
            endpoint_url=settings.S3_ENDPOINT_URL,
        )
        storage.put_bytes(
            key,
            json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            content_type="application/json",
        )
        return {
            "snapshot_bucket": bucket,
            "snapshot_key": key,
            "snapshot_uri": f"s3://{bucket}/{key}",
        }
    except Exception:
        logger.warning(
            "knowledge snapshot write failed user_id=%s file_path=%s version=%s",
            user_id,
            file_path,
            version,
            exc_info=True,
        )
        return None


def ensure_default_knowledge_files(
    db: Session,
    *,
    user_id: str,
    display_name: str = "User",
    timezone: str = "America/New_York",
) -> list[str]:
    """
    Seed v5 knowledge files for a user if they do not exist.
    Returns the file paths that were inserted.
    """
    if not _table_exists(db, "knowledge_files"):
        return []

    inserted: list[str] = []

    for file_path, spec in DEFAULT_KNOWLEDGE_TEMPLATES.items():
        row = db.execute(
            text(
                "select id from knowledge_files where user_id = :user_id and file_path = :file_path order by version desc limit 1"
            ),
            {"user_id": user_id, "file_path": file_path},
        ).mappings().first()
        if row:
            continue

        content = _render(spec["content"], display_name=display_name, timezone=timezone)
        content_hash = hashlib.sha256(content.encode("utf-8")).hexdigest()
        metadata_obj: dict[str, Any] = {}
        snapshot = _write_snapshot(
            user_id=user_id,
            file_path=file_path,
            layer=spec["layer"],
            version=1,
            content=content,
            metadata=metadata_obj,
        )
        if snapshot:
            metadata_obj.update(snapshot)

        db.execute(
            text(
                """
                insert into knowledge_files (
                    user_id, file_path, layer, content, content_hash, token_count, version, metadata, created_at
                ) values (
                    :user_id,
                    :file_path,
                    :layer,
                    :content,
                    :content_hash,
                    :token_count,
                    1,
                    :metadata,
                    :created_at
                )
                """
            ),
            {
                "user_id": user_id,
                "file_path": file_path,
                "layer": spec["layer"],
                "content": content,
                "content_hash": content_hash,
                "token_count": max(1, len(content.split()) * 4 // 3),
                "metadata": json.dumps(metadata_obj, ensure_ascii=False),
                "created_at": datetime.utcnow(),
            },
        )
        inserted.append(file_path)

    if inserted:
        db.commit()
    return inserted


def list_knowledge_files(db: Session, *, user_id: str) -> list[dict[str, Any]]:
    if not _table_exists(db, "knowledge_files"):
        return []

    rows = db.execute(
        text(
            """
            select file_path, layer, content, content_hash, version, token_count, metadata, created_at, updated_at
            from knowledge_files
            where user_id = :user_id
            order by file_path asc, version desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()

    out: list[dict[str, Any]] = []
    for row in rows:
        created = row.get("created_at")
        updated = row.get("updated_at")
        raw_meta = row.get("metadata")
        if isinstance(raw_meta, str):
            try:
                raw_meta = json.loads(raw_meta)
            except Exception:
                raw_meta = {"raw": raw_meta}
        if not isinstance(raw_meta, dict):
            raw_meta = {}
        out.append(
            {
                "file_path": row.get("file_path"),
                "layer": row.get("layer"),
                "content": row.get("content"),
                "content_hash": row.get("content_hash"),
                "version": row.get("version"),
                "token_count": row.get("token_count"),
                "metadata": raw_meta,
                "created_at": created.isoformat() if isinstance(created, datetime) else str(created),
                "updated_at": updated.isoformat() if isinstance(updated, datetime) else str(updated),
            }
        )
    return out


def get_latest_knowledge_file(db: Session, *, user_id: str, file_path: str) -> dict[str, Any] | None:
    if not _table_exists(db, "knowledge_files"):
        return None
    normalized_path = (file_path or "").strip()
    if normalized_path in HOT_CACHE_FILES:
        cached = cache_get_json(_cache_key(user_id, normalized_path))
        if isinstance(cached, dict):
            return cached
    row = db.execute(
        text(
            """
            select file_path, layer, content, content_hash, token_count, version, metadata, created_at
            from knowledge_files
            where user_id = :user_id and file_path = :file_path
            order by version desc
            limit 1
            """
        ),
        {"user_id": user_id, "file_path": normalized_path},
    ).mappings().first()
    if not row:
        return None
    created = row.get("created_at")
    raw_meta = row.get("metadata")
    if isinstance(raw_meta, str):
        try:
            raw_meta = json.loads(raw_meta)
        except Exception:
            raw_meta = {"raw": raw_meta}
    if not isinstance(raw_meta, dict):
        raw_meta = {}
    out = {
        "file_path": row.get("file_path"),
        "layer": row.get("layer"),
        "content": row.get("content"),
        "content_hash": row.get("content_hash"),
        "token_count": row.get("token_count"),
        "version": row.get("version"),
        "metadata": raw_meta,
        "created_at": created.isoformat() if isinstance(created, datetime) else str(created),
    }
    if normalized_path in HOT_CACHE_FILES:
        cache_set_json(_cache_key(user_id, normalized_path), out, ttl_seconds=1800)
    return out


def put_knowledge_file_version(
    db: Session,
    *,
    user_id: str,
    file_path: str,
    content: str,
    metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    if not _table_exists(db, "knowledge_files"):
        raise RuntimeError("knowledge_files table does not exist")

    normalized_path = (file_path or "").strip()
    existing = get_latest_knowledge_file(db, user_id=user_id, file_path=normalized_path)
    next_version = int((existing or {}).get("version") or 0) + 1
    layer = str(
        (existing or {}).get("layer")
        or (DEFAULT_KNOWLEDGE_TEMPLATES.get(normalized_path) or {}).get("layer")
        or "brain"
    )
    content_hash = hashlib.sha256((content or "").encode("utf-8")).hexdigest()
    token_count = max(1, len((content or "").split()) * 4 // 3)
    metadata_obj: dict[str, Any] = dict(metadata or {})
    snapshot = _write_snapshot(
        user_id=user_id,
        file_path=normalized_path,
        layer=layer,
        version=next_version,
        content=content,
        metadata=metadata_obj,
    )
    if snapshot:
        metadata_obj.update(snapshot)

    db.execute(
        text(
            """
            insert into knowledge_files (
                user_id, file_path, layer, content, content_hash, token_count, version, metadata, created_at
            ) values (
                :user_id, :file_path, :layer, :content, :content_hash, :token_count, :version, :metadata, :created_at
            )
            """
        ),
        {
            "user_id": user_id,
            "file_path": normalized_path,
            "layer": layer,
            "content": content,
            "content_hash": content_hash,
            "token_count": token_count,
            "version": next_version,
            "metadata": json.dumps(metadata_obj, ensure_ascii=False),
            "created_at": datetime.utcnow(),
        },
    )
    db.commit()
    if normalized_path in HOT_CACHE_FILES:
        cache_delete(_cache_key(user_id, normalized_path))
    latest = get_latest_knowledge_file(db, user_id=user_id, file_path=normalized_path)
    if not latest:
        raise RuntimeError("failed to read back knowledge file")
    return latest


def knowledge_completeness(files: list[dict[str, Any]]) -> dict[str, Any]:
    required = set(DEFAULT_KNOWLEDGE_TEMPLATES.keys())
    present = {str((item or {}).get("file_path") or "") for item in files}
    present = {p for p in present if p}
    missing = sorted(required - present)
    coverage = round((len(required) - len(missing)) / max(1, len(required)), 4)
    return {
        "coverage": coverage,
        "present_count": len(required) - len(missing),
        "required_count": len(required),
        "missing_files": missing,
    }
