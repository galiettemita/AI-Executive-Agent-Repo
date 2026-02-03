"""add preferences table

Revision ID: 5b8d2d4a1f21
Revises: 9a26270e1925
Create Date: 2026-02-03 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "5b8d2d4a1f21"
down_revision = "9a26270e1925"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "preferences",
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), primary_key=True),
        sa.Column("data_json", sa.Text(), nullable=False, server_default="{}"),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
    )


def downgrade() -> None:
    op.drop_table("preferences")
