"""add usage table

Revision ID: 1f3b6d9a2c14
Revises: 0f6c9a2a7d13
Create Date: 2026-02-03 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "1f3b6d9a2c14"
down_revision = "0f6c9a2a7d13"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "usage",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("period", sa.String(), nullable=False),
        sa.Column("messages_count", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("tokens_count", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("proposals_count", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
    )
    op.create_index("ix_usage_user_id", "usage", ["user_id"], unique=False)
    op.create_index("ix_usage_period", "usage", ["period"], unique=False)


def downgrade() -> None:
    op.drop_index("ix_usage_period", table_name="usage")
    op.drop_index("ix_usage_user_id", table_name="usage")
    op.drop_table("usage")
