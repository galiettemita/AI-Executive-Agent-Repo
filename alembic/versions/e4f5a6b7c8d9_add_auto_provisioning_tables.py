"""add auto-provisioning tables

Revision ID: e4f5a6b7c8d9
Revises: d9e0f1a2b3c4
Create Date: 2026-02-19 15:30:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "e4f5a6b7c8d9"
down_revision = "d9e0f1a2b3c4"
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
    dialect = conn.dialect.name

    if not _table_exists(conn, "provisioning_requests"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table provisioning_requests (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      state text not null default 'initiated',
                      trigger text not null default 'capability_gap',
                      auth_type text,
                      reason text,
                      original_task_id text,
                      retry_count integer default 0,
                      state_history text,
                      error_message text,
                      expires_at datetime,
                      created_at datetime default current_timestamp,
                      updated_at datetime default current_timestamp,
                      completed_at datetime
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table provisioning_requests (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      state text not null default 'initiated',
                      trigger text not null default 'capability_gap',
                      auth_type text,
                      reason text,
                      original_task_id text,
                      retry_count integer default 0,
                      state_history jsonb,
                      error_message text,
                      expires_at timestamptz,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now(),
                      completed_at timestamptz
                    )
                    """
                )
            )
        op.execute(
            sa.text(
                "create index if not exists idx_provisioning_requests_user_state "
                "on provisioning_requests(user_id, state, updated_at)"
            )
        )
        op.execute(
            sa.text(
                "create index if not exists idx_provisioning_requests_server_state "
                "on provisioning_requests(server_id, state, updated_at)"
            )
        )

    if not _table_exists(conn, "server_catalog"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table server_catalog (
                      server_id text primary key,
                      display_name text,
                      description text,
                      auth_type text,
                      min_plan text default 'free',
                      setup_seconds integer default 30,
                      capabilities text,
                      keywords text,
                      hosting_model text,
                      oauth_config text,
                      container_image text,
                      source text default 'local',
                      signature text,
                      status text default 'active',
                      updated_at datetime default current_timestamp,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table server_catalog (
                      server_id text primary key,
                      display_name text,
                      description text,
                      auth_type text,
                      min_plan text default 'free',
                      setup_seconds integer default 30,
                      capabilities jsonb,
                      keywords jsonb,
                      hosting_model text,
                      oauth_config jsonb,
                      container_image text,
                      source text default 'local',
                      signature text,
                      status text default 'active',
                      updated_at timestamptz default now(),
                      created_at timestamptz default now()
                    )
                    """
                )
            )

    if not _table_exists(conn, "provisioning_declined"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table provisioning_declined (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      reason text,
                      declined_at datetime default current_timestamp,
                      cooldown_until datetime,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            op.execute(
                sa.text(
                    """
                    create table provisioning_declined (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      reason text,
                      declined_at timestamptz default now(),
                      cooldown_until timestamptz,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(
            sa.text(
                "create index if not exists idx_provisioning_declined_user_server "
                "on provisioning_declined(user_id, server_id, declined_at)"
            )
        )


def downgrade() -> None:
    conn = op.get_bind()
    if _table_exists(conn, "provisioning_declined"):
        op.execute(sa.text("drop table provisioning_declined"))
    if _table_exists(conn, "server_catalog"):
        op.execute(sa.text("drop table server_catalog"))
    if _table_exists(conn, "provisioning_requests"):
        op.execute(sa.text("drop table provisioning_requests"))
