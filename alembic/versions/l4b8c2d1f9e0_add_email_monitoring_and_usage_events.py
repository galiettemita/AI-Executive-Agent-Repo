"""add email monitoring and usage events

Revision ID: l4b8c2d1f9e0
Revises: k2d5f8a7b9c1
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "l4b8c2d1f9e0"
down_revision = "k2d5f8a7b9c1"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "email_monitor_configs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("provider", sa.String(), nullable=True, index=True),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("true"), index=True),
        sa.Column("keywords_json", sa.Text(), nullable=True),
        sa.Column("sender_allowlist_json", sa.Text(), nullable=True),
        sa.Column("subject_keywords_json", sa.Text(), nullable=True),
        sa.Column("priority_threshold", sa.Integer(), nullable=True),
        sa.Column("use_ai_priority", sa.Boolean(), nullable=False, server_default=sa.text("false")),
        sa.Column("alert_channel", sa.String(), nullable=False, server_default="whatsapp", index=True),
        sa.Column("alert_title", sa.String(), nullable=True),
        sa.Column("window_minutes", sa.Integer(), nullable=False, server_default=sa.text("60")),
        sa.Column("max_results", sa.Integer(), nullable=False, server_default=sa.text("20")),
        sa.Column("last_checked_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "email_alerts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("provider", sa.String(), nullable=True, index=True),
        sa.Column("message_id", sa.String(), nullable=False, index=True),
        sa.Column("subject", sa.String(), nullable=True),
        sa.Column("sender", sa.String(), nullable=True),
        sa.Column("priority", sa.Integer(), nullable=True),
        sa.Column("reason", sa.Text(), nullable=True),
        sa.Column("alert_channel", sa.String(), nullable=False, server_default="whatsapp", index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint(
            "user_id",
            "provider",
            "message_id",
            "alert_channel",
            name="uq_email_alert_user_message",
        ),
    )

    op.create_table(
        "usage_events",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("event_type", sa.String(), nullable=False, index=True),
        sa.Column("source", sa.String(), nullable=True),
        sa.Column("channel", sa.String(), nullable=True),
        sa.Column("provider", sa.String(), nullable=True),
        sa.Column("tokens", sa.Integer(), nullable=True),
        sa.Column("cost_usd", sa.Float(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("usage_events")
    op.drop_table("email_alerts")
    op.drop_table("email_monitor_configs")
