"""add beta testers

Revision ID: i3b7a1d2c4ef
Revises: h2e9c4f1d7aa
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "i3b7a1d2c4ef"
down_revision = "h2e9c4f1d7aa"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "beta_testers",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), nullable=False, unique=True, index=True),
        sa.Column("email", sa.String(), nullable=True, index=True),
        sa.Column("status", sa.String(), nullable=False, server_default="active", index=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("beta_testers")
