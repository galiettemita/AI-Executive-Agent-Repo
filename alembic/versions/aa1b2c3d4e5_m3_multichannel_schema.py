"""m3 multi-channel schema fields

Revision ID: aa1b2c3d4e5
Revises: z0a1b2c3d4e5
Create Date: 2026-02-18 00:00:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "aa1b2c3d4e5"
down_revision = "z0a1b2c3d4e5"
branch_labels = None
depends_on = None


def _table_exists(conn, table_name: str) -> bool:
    dialect = conn.dialect.name
    if dialect == "sqlite":
        row = conn.execute(
            sa.text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    row = conn.execute(
        sa.text(
            "select 1 from information_schema.tables "
            "where table_schema = current_schema() and table_name = :name"
        ),
        {"name": table_name},
    ).first()
    return bool(row)


def _column_exists(conn, table_name: str, column_name: str) -> bool:
    dialect = conn.dialect.name
    if dialect == "sqlite":
        rows = conn.execute(sa.text(f"PRAGMA table_info({table_name})")).mappings().all()
        return any(str(r.get("name")) == column_name for r in rows)
    row = conn.execute(
        sa.text(
            "select 1 from information_schema.columns "
            "where table_schema = current_schema() and table_name = :table and column_name = :column"
        ),
        {"table": table_name, "column": column_name},
    ).first()
    return bool(row)


def upgrade() -> None:
    conn = op.get_bind()

    if _table_exists(conn, "conversations"):
        with op.batch_alter_table("conversations") as batch:
            if not _column_exists(conn, "conversations", "active_channel"):
                batch.add_column(sa.Column("active_channel", sa.String(), nullable=True))
            if not _column_exists(conn, "conversations", "channels_used"):
                batch.add_column(sa.Column("channels_used", sa.Text(), nullable=True))

    # Legacy schema stores user-visible messages in chat_messages.
    if _table_exists(conn, "chat_messages"):
        with op.batch_alter_table("chat_messages") as batch:
            if not _column_exists(conn, "chat_messages", "channel"):
                batch.add_column(sa.Column("channel", sa.String(), nullable=True))
            if not _column_exists(conn, "chat_messages", "channel_message_id"):
                batch.add_column(sa.Column("channel_message_id", sa.String(), nullable=True))

    # Blueprint v5 schema stores messages in `messages`.
    if _table_exists(conn, "messages"):
        with op.batch_alter_table("messages") as batch:
            if not _column_exists(conn, "messages", "channel_message_id"):
                batch.add_column(sa.Column("channel_message_id", sa.String(), nullable=True))


def downgrade() -> None:
    conn = op.get_bind()

    if _table_exists(conn, "messages"):
        with op.batch_alter_table("messages") as batch:
            if _column_exists(conn, "messages", "channel_message_id"):
                batch.drop_column("channel_message_id")

    if _table_exists(conn, "chat_messages"):
        with op.batch_alter_table("chat_messages") as batch:
            if _column_exists(conn, "chat_messages", "channel_message_id"):
                batch.drop_column("channel_message_id")
            if _column_exists(conn, "chat_messages", "channel"):
                batch.drop_column("channel")

    if _table_exists(conn, "conversations"):
        with op.batch_alter_table("conversations") as batch:
            if _column_exists(conn, "conversations", "channels_used"):
                batch.drop_column("channels_used")
            if _column_exists(conn, "conversations", "active_channel"):
                batch.drop_column("active_channel")
