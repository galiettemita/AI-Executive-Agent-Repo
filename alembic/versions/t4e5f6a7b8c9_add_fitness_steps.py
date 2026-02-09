"""add fitness step logs

Revision ID: t4e5f6a7b8c9
Revises: s3d4e5f6a7b8
Create Date: 2026-02-09
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "t4e5f6a7b8c9"
down_revision = "s3d4e5f6a7b8"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "fitness_step_logs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("step_date", sa.Date(), nullable=False, index=True),
        sa.Column("steps", sa.Integer(), nullable=False, server_default=sa.text("0")),
        sa.Column("source", sa.String(), nullable=False, server_default="fitbit", index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "step_date", name="uq_fitness_steps_user_date"),
    )


def downgrade():
    op.drop_table("fitness_step_logs")
