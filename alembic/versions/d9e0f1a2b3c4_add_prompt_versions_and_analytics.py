"""add prompt versioning and analytics tables

Revision ID: d9e0f1a2b3c4
Revises: c7d8e9f0a1b2
Create Date: 2026-02-19 11:45:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "d9e0f1a2b3c4"
down_revision = "c7d8e9f0a1b2"
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
        rows = conn.execute(sa.text(f"pragma table_info({table_name})")).mappings().all()
        return any(str(row.get("name")) == column_name for row in rows)
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
    dialect = conn.dialect.name

    if not _table_exists(conn, "prompt_versions"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table prompt_versions (
                      id text primary key,
                      prompt_group text not null,
                      version_label text not null,
                      content text not null,
                      status text not null default 'draft',
                      rollout_percentage integer default 0,
                      metadata_json text,
                      created_at datetime default current_timestamp,
                      updated_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table prompt_versions (
                      id text primary key,
                      prompt_group text not null,
                      version_label text not null,
                      content text not null,
                      status text not null default 'draft',
                      rollout_percentage integer default 0,
                      metadata_json jsonb,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(
            sa.text(
                "create index if not exists idx_prompt_versions_group_status "
                "on prompt_versions(prompt_group, status, created_at)"
            )
        )

    if not _table_exists(conn, "analytics_events"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table analytics_events (
                      id text primary key,
                      user_id text,
                      event_name text not null,
                      source text,
                      payload_json text,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table analytics_events (
                      id text primary key,
                      user_id text,
                      event_name text not null,
                      source text,
                      payload_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(sa.text("create index if not exists idx_analytics_events_created on analytics_events(created_at)"))
        op.execute(sa.text("create index if not exists idx_analytics_events_name on analytics_events(event_name, created_at)"))

    if not _table_exists(conn, "analytics_daily"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table analytics_daily (
                      id text primary key,
                      day text not null,
                      dau integer default 0,
                      mau integer default 0,
                      message_volume integer default 0,
                      tool_calls integer default 0,
                      avg_quality_score real default 0,
                      revenue_cents integer default 0,
                      metadata_json text,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table analytics_daily (
                      id text primary key,
                      day date not null,
                      dau integer default 0,
                      mau integer default 0,
                      message_volume integer default 0,
                      tool_calls integer default 0,
                      avg_quality_score double precision default 0,
                      revenue_cents bigint default 0,
                      metadata_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(sa.text("create unique index if not exists uq_analytics_daily_day on analytics_daily(day)"))

    if _table_exists(conn, "eval_results") and not _column_exists(conn, "eval_results", "prompt_version_id"):
        with op.batch_alter_table("eval_results") as batch:
            batch.add_column(sa.Column("prompt_version_id", sa.Text(), nullable=True))


def downgrade() -> None:
    conn = op.get_bind()
    if _table_exists(conn, "analytics_daily"):
        op.execute(sa.text("drop table analytics_daily"))
    if _table_exists(conn, "analytics_events"):
        op.execute(sa.text("drop table analytics_events"))
    if _table_exists(conn, "prompt_versions"):
        op.execute(sa.text("drop table prompt_versions"))
