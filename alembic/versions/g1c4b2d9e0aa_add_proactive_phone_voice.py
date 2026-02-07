"""add proactive rules, phone verification, voice scripts

Revision ID: g1c4b2d9e0aa
Revises: f3c2a8b1e9d0
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "g1c4b2d9e0aa"
down_revision = "f3c2a8b1e9d0"
branch_labels = None
depends_on = None


def upgrade():
    # Make watch_item_id nullable for generic notifications
    op.alter_column(
        "notification_queue",
        "watch_item_id",
        existing_type=sa.Integer(),
        nullable=True,
    )

    # Phone verifications
    op.create_table(
        "phone_verifications",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("phone_number", sa.String(), index=True, nullable=False),
        sa.Column("code_hash", sa.String(), nullable=False),
        sa.Column("status", sa.String(), nullable=False, server_default="pending", index=True),
        sa.Column("attempts", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("max_attempts", sa.Integer(), nullable=False, server_default="5"),
        sa.Column("expires_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("verified_at", sa.DateTime(), nullable=True),
        sa.Column("last_sent_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    # Proactive rules
    op.create_table(
        "proactive_rules",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("name", sa.String(), nullable=False, index=True),
        sa.Column("is_active", sa.Boolean(), nullable=False, server_default=sa.text("1")),
        sa.Column("trigger_type", sa.String(), nullable=False, index=True),
        sa.Column("trigger_config_json", sa.Text(), nullable=True),
        sa.Column("conditions_json", sa.Text(), nullable=True),
        sa.Column("action_type", sa.String(), nullable=False, index=True),
        sa.Column("action_payload_json", sa.Text(), nullable=True),
        sa.Column("last_run_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("next_run_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "proactive_rule_runs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("rule_id", sa.Integer(), sa.ForeignKey("proactive_rules.id"), index=True, nullable=False),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("status", sa.String(), nullable=False, index=True),
        sa.Column("reason", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    # Voice call scripts
    op.create_table(
        "voice_call_scripts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("name", sa.String(), nullable=False, index=True),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("script_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    # Voice call columns
    op.add_column(
        "voice_calls",
        sa.Column("proposal_id", sa.Integer(), sa.ForeignKey("proposals.id"), nullable=True, index=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("script_id", sa.Integer(), sa.ForeignKey("voice_call_scripts.id"), nullable=True, index=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("script_json", sa.Text(), nullable=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("answered_at", sa.DateTime(), nullable=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("ended_at", sa.DateTime(), nullable=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("outcome_status", sa.String(), nullable=True, index=True),
    )
    op.add_column(
        "voice_calls",
        sa.Column("outcome_notes", sa.Text(), nullable=True),
    )


def downgrade():
    op.drop_column("voice_calls", "outcome_notes")
    op.drop_column("voice_calls", "outcome_status")
    op.drop_column("voice_calls", "ended_at")
    op.drop_column("voice_calls", "answered_at")
    op.drop_column("voice_calls", "script_json")
    op.drop_column("voice_calls", "script_id")
    op.drop_column("voice_calls", "proposal_id")

    op.drop_table("voice_call_scripts")
    op.drop_table("proactive_rule_runs")
    op.drop_table("proactive_rules")
    op.drop_table("phone_verifications")

    op.alter_column(
        "notification_queue",
        "watch_item_id",
        existing_type=sa.Integer(),
        nullable=False,
    )
