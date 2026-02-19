"""add eval results, user feedback, scheduled notifications tables

Revision ID: c7d8e9f0a1b2
Revises: b1c2d3e4f5a6
Create Date: 2026-02-19 11:05:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "c7d8e9f0a1b2"
down_revision = "b1c2d3e4f5a6"
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

    if not _table_exists(conn, "eval_results"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table eval_results (
                      id text primary key,
                      user_id text,
                      conversation_id text,
                      run_id text,
                      message_id text,
                      coherence_score real not null,
                      helpfulness_score real not null,
                      safety_score real not null,
                      tool_usage_score real not null,
                      overall_score real not null,
                      evaluator_provider text,
                      prompt_version_id text,
                      source text not null default 'live',
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
                    create table eval_results (
                      id text primary key,
                      user_id text,
                      conversation_id text,
                      run_id text,
                      message_id text,
                      coherence_score double precision not null,
                      helpfulness_score double precision not null,
                      safety_score double precision not null,
                      tool_usage_score double precision not null,
                      overall_score double precision not null,
                      evaluator_provider text,
                      prompt_version_id text,
                      source text not null default 'live',
                      metadata_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(sa.text("create index if not exists idx_eval_results_created_at on eval_results(created_at)"))
        op.execute(sa.text("create index if not exists idx_eval_results_user_id on eval_results(user_id)"))

    if not _table_exists(conn, "user_feedback"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table user_feedback (
                      id text primary key,
                      user_id text,
                      message_id text,
                      run_id text,
                      feedback text not null,
                      comment text,
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
                    create table user_feedback (
                      id text primary key,
                      user_id text,
                      message_id text,
                      run_id text,
                      feedback text not null,
                      comment text,
                      metadata_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(sa.text("create index if not exists idx_user_feedback_created_at on user_feedback(created_at)"))
        op.execute(sa.text("create index if not exists idx_user_feedback_user_id on user_feedback(user_id)"))

    if not _table_exists(conn, "scheduled_notifications"):
        if dialect == "sqlite":
            op.execute(
                sa.text(
                    """
                    create table scheduled_notifications (
                      id text primary key,
                      user_id text not null,
                      notification_type text not null,
                      channel text,
                      timezone text,
                      payload_json text,
                      scheduled_for datetime not null,
                      status text not null default 'scheduled',
                      sent_at datetime,
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
                    create table scheduled_notifications (
                      id text primary key,
                      user_id text not null,
                      notification_type text not null,
                      channel text,
                      timezone text,
                      payload_json jsonb,
                      scheduled_for timestamptz not null,
                      status text not null default 'scheduled',
                      sent_at timestamptz,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now()
                    )
                    """
                )
            )
        op.execute(
            sa.text(
                "create index if not exists idx_scheduled_notifications_due "
                "on scheduled_notifications(status, scheduled_for)"
            )
        )
        op.execute(
            sa.text(
                "create index if not exists idx_scheduled_notifications_user "
                "on scheduled_notifications(user_id, status)"
            )
        )


def downgrade() -> None:
    conn = op.get_bind()
    if _table_exists(conn, "scheduled_notifications"):
        op.execute(sa.text("drop table scheduled_notifications"))
    if _table_exists(conn, "user_feedback"):
        op.execute(sa.text("drop table user_feedback"))
    if _table_exists(conn, "eval_results"):
        op.execute(sa.text("drop table eval_results"))
