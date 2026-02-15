from __future__ import annotations

import json
import os
import sqlite3
import uuid
from datetime import datetime
from typing import Dict, Tuple

import psycopg


def _normalize_dsn(raw: str) -> str:
    return raw.replace("postgresql+asyncpg://", "postgresql://", 1)


def _uuid_from_legacy(legacy_id: str) -> uuid.UUID:
    try:
        return uuid.UUID(str(legacy_id))
    except Exception:
        return uuid.uuid5(uuid.NAMESPACE_URL, f"legacy-user:{legacy_id}")


def _looks_like_phone(value: str) -> bool:
    value = value.strip()
    return value.startswith("+") and value[1:].isdigit()


def _load_phone_map(path: str | None) -> Dict[str, str]:
    if not path:
        return {}
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return {str(k): str(v) for k, v in data.items()}


def main() -> None:
    sqlite_path = os.environ.get("SQLITE_PATH", "app.db")
    pg_dsn = os.environ.get("DATABASE_URL")
    if not pg_dsn:
        raise SystemExit("DATABASE_URL is required")
    pg_dsn = _normalize_dsn(pg_dsn)

    phone_map = _load_phone_map(os.environ.get("USER_PHONE_MAP_JSON"))
    dry_run = os.environ.get("DRY_RUN", "1") == "1"

    sqlite_conn = sqlite3.connect(sqlite_path)
    sqlite_conn.row_factory = sqlite3.Row

    user_rows = sqlite_conn.execute("SELECT id, created_at FROM users").fetchall()
    convo_rows = sqlite_conn.execute("SELECT id, user_id, created_at FROM conversations").fetchall()
    msg_rows = sqlite_conn.execute(
        "SELECT id, conversation_id, user_id, role, content, created_at FROM chat_messages"
    ).fetchall()

    user_id_map: Dict[str, uuid.UUID] = {}
    convo_id_map: Dict[int, uuid.UUID] = {}

    for row in user_rows:
        legacy_id = str(row["id"])
        user_id_map[legacy_id] = _uuid_from_legacy(legacy_id)

    for row in convo_rows:
        convo_id_map[int(row["id"])] = uuid.uuid4()

    if dry_run:
        print(f"[dry-run] users={len(user_rows)} conversations={len(convo_rows)} messages={len(msg_rows)}")
        return

    with psycopg.connect(pg_dsn) as conn:
        conn.autocommit = True
        with conn.cursor() as cur:
            # Users
            for row in user_rows:
                legacy_id = str(row["id"])
                new_id = user_id_map[legacy_id]
                phone_number = phone_map.get(legacy_id, legacy_id if _looks_like_phone(legacy_id) else None)
                if not phone_number:
                    # Fallback: generate deterministic placeholder
                    phone_number = f"+1000000{str(new_id.int)[:6]}"
                created_at = row["created_at"] or datetime.utcnow().isoformat()
                cur.execute(
                    """
                    INSERT INTO users (id, phone_number, created_at, updated_at)
                    VALUES (%s, %s, %s, %s)
                    ON CONFLICT (id) DO NOTHING
                    """,
                    (str(new_id), phone_number, created_at, created_at),
                )

            # Conversations
            for row in convo_rows:
                legacy_convo_id = int(row["id"])
                new_convo_id = convo_id_map[legacy_convo_id]
                legacy_user_id = str(row["user_id"])
                user_id = user_id_map.get(legacy_user_id)
                if not user_id:
                    continue
                created_at = row["created_at"] or datetime.utcnow().isoformat()
                cur.execute(
                    """
                    INSERT INTO conversations (id, user_id, channel, state, summary, started_at, last_active_at)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (id) DO NOTHING
                    """,
                    (str(new_convo_id), str(user_id), "web", "{}", None, created_at, created_at),
                )

            # Messages
            for row in msg_rows:
                legacy_user_id = str(row["user_id"])
                user_id = user_id_map.get(legacy_user_id)
                convo_uuid = convo_id_map.get(int(row["conversation_id"]))
                if not user_id or not convo_uuid:
                    continue
                direction = "inbound" if row["role"] == "user" else "outbound"
                created_at = row["created_at"] or datetime.utcnow().isoformat()
                cur.execute(
                    """
                    INSERT INTO messages (id, conversation_id, user_id, direction, content, created_at)
                    VALUES (gen_random_uuid(), %s, %s, %s, %s, %s)
                    """,
                    (str(convo_uuid), str(user_id), direction, json.dumps({"text": row["content"] or ""}), created_at),
                )

    print("migration complete")


if __name__ == "__main__":
    main()
