"""add proposals table

Revision ID: 2f9c0c1b4a7e
Revises: 1f3b6d9a2c14
Create Date: 2026-02-03 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "2f9c0c1b4a7e"
down_revision = "1f3b6d9a2c14"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "proposals",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("proposal_type", sa.String(), nullable=False),
        sa.Column("status", sa.String(), nullable=False, server_default="pending"),
        sa.Column("payload_json", sa.Text(), nullable=False, server_default="{}"),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("expires_at", sa.DateTime(), nullable=True),
    )
    op.create_index("ix_proposals_user_id", "proposals", ["user_id"], unique=False)
    op.create_index("ix_proposals_status", "proposals", ["status"], unique=False)
    op.create_index("ix_proposals_proposal_type", "proposals", ["proposal_type"], unique=False)


def downgrade() -> None:
    op.drop_index("ix_proposals_proposal_type", table_name="proposals")
    op.drop_index("ix_proposals_status", table_name="proposals")
    op.drop_index("ix_proposals_user_id", table_name="proposals")
    op.drop_table("proposals")
