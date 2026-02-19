from __future__ import annotations

import uuid

from sqlalchemy import text

from app.db.database import SessionLocal
from app.db.user_compat import ensure_fk_parent_row, user_exists


def test_ensure_fk_parent_row_creates_user_for_phone_verifications():
    db = SessionLocal()
    try:
        uid = f"user_{uuid.uuid4().hex[:10]}"
        assert user_exists(db, uid) is False
        ok = ensure_fk_parent_row(
            db,
            child_table="phone_verifications",
            fk_column="user_id",
            user_id=uid,
        )
        assert ok is True
        assert user_exists(db, uid) is True
    finally:
        db.close()


def test_ensure_fk_parent_row_creates_accounts_parent_when_fk_targets_accounts():
    db = SessionLocal()
    try:
        child_table = "channel_connections_accounts_fk_test"
        db.execute(
            text(
                """
                create table if not exists accounts (
                  id text primary key,
                  created_at text
                )
                """
            )
        )
        db.execute(
            text(
                """
                create table if not exists channel_connections_accounts_fk_test (
                  id text primary key,
                  user_id text references accounts(id),
                  channel text,
                  channel_identifier text
                )
                """
            )
        )
        db.commit()

        uid = f"user_{uuid.uuid4().hex[:10]}"
        row = db.execute(text("select id from accounts where id = :id"), {"id": uid}).first()
        assert row is None

        ok = ensure_fk_parent_row(
            db,
            child_table=child_table,
            fk_column="user_id",
            user_id=uid,
        )
        assert ok is True
        row = db.execute(text("select id from accounts where id = :id"), {"id": uid}).first()
        assert row is not None
    finally:
        db.close()
