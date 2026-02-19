"""add moderation queue table for abuse prevention

Revision ID: b1c2d3e4f5a6
Revises: aa1b2c3d4e5
Create Date: 2026-02-19 10:15:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "b1c2d3e4f5a6"
down_revision = "aa1b2c3d4e5"
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


def upgrade() -> None:
    conn = op.get_bind()
    if _table_exists(conn, "moderation_queue"):
        return

    dialect = conn.dialect.name
    if dialect == "sqlite":
        op.execute(
            sa.text(
                """
                create table moderation_queue (
                  id text primary key,
                  user_id text,
                  run_id text,
                  message_direction text not null,
                  channel text,
                  content_excerpt text,
                  categories text,
                  risk_score real default 0,
                  classifier text,
                  status text not null default 'pending',
                  source text not null default 'safety_classifier',
                  metadata_json text,
                  created_at datetime default current_timestamp,
                  resolved_at datetime,
                  resolver_id text,
                  resolution_notes text
                )
                """
            )
        )
    else:
        op.execute(
            sa.text(
                """
                create table moderation_queue (
                  id text primary key,
                  user_id text,
                  run_id text,
                  message_direction text not null,
                  channel text,
                  content_excerpt text,
                  categories jsonb,
                  risk_score double precision default 0,
                  classifier text,
                  status text not null default 'pending',
                  source text not null default 'safety_classifier',
                  metadata_json jsonb,
                  created_at timestamptz not null default now(),
                  resolved_at timestamptz,
                  resolver_id text,
                  resolution_notes text
                )
                """
            )
        )

    op.execute(sa.text("create index if not exists idx_moderation_queue_user_status on moderation_queue(user_id, status)"))
    op.execute(sa.text("create index if not exists idx_moderation_queue_created on moderation_queue(created_at)"))


def downgrade() -> None:
    conn = op.get_bind()
    if not _table_exists(conn, "moderation_queue"):
        return
    op.execute(sa.text("drop table moderation_queue"))
