"""add device pairing codes

Revision ID: m5a1c7d9b2e4
Revises: l4b8c2d1f9e0
Create Date: 2026-02-08
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "m5a1c7d9b2e4"
down_revision = "l4b8c2d1f9e0"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "device_pairing_codes",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("code", sa.String(), nullable=False, unique=True, index=True),
        sa.Column("expires_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("used_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("device_pairing_codes")
