"""add message receipts and voice error tracking

Revision ID: j4c9e1f2a7b3
Revises: i3b7a1d2c4ef
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "j4c9e1f2a7b3"
down_revision = "i3b7a1d2c4ef"
branch_labels = None
depends_on = None


def upgrade():
    op.add_column("outbound_messages", sa.Column("provider", sa.String(), nullable=True))
    op.add_column("outbound_messages", sa.Column("provider_status", sa.String(), nullable=True))
    op.add_column("outbound_messages", sa.Column("delivered_at", sa.DateTime(), nullable=True))
    op.add_column("outbound_messages", sa.Column("failed_at", sa.DateTime(), nullable=True))
    op.add_column("outbound_messages", sa.Column("last_status_at", sa.DateTime(), nullable=True))
    op.create_index("ix_outbound_messages_provider", "outbound_messages", ["provider"])
    op.create_index("ix_outbound_messages_provider_status", "outbound_messages", ["provider_status"])
    op.create_index("ix_outbound_messages_delivered_at", "outbound_messages", ["delivered_at"])
    op.create_index("ix_outbound_messages_failed_at", "outbound_messages", ["failed_at"])
    op.create_index("ix_outbound_messages_last_status_at", "outbound_messages", ["last_status_at"])

    op.create_table(
        "outbound_message_events",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("message_id", sa.Integer(), sa.ForeignKey("outbound_messages.id"), nullable=True, index=True),
        sa.Column("provider", sa.String(), nullable=True, index=True),
        sa.Column("event_type", sa.String(), nullable=False, index=True),
        sa.Column("payload_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.add_column("voice_calls", sa.Column("error_message", sa.Text(), nullable=True))


def downgrade():
    op.drop_column("voice_calls", "error_message")
    op.drop_table("outbound_message_events")
    op.drop_index("ix_outbound_messages_last_status_at", table_name="outbound_messages")
    op.drop_index("ix_outbound_messages_failed_at", table_name="outbound_messages")
    op.drop_index("ix_outbound_messages_delivered_at", table_name="outbound_messages")
    op.drop_index("ix_outbound_messages_provider_status", table_name="outbound_messages")
    op.drop_index("ix_outbound_messages_provider", table_name="outbound_messages")
    op.drop_column("outbound_messages", "last_status_at")
    op.drop_column("outbound_messages", "failed_at")
    op.drop_column("outbound_messages", "delivered_at")
    op.drop_column("outbound_messages", "provider_status")
    op.drop_column("outbound_messages", "provider")
