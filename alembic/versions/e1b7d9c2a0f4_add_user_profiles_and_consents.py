"""add user_profiles and user_consents

Revision ID: e1b7d9c2a0f4
Revises: d4b8c9e3f7a1
Create Date: 2026-02-07 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "e1b7d9c2a0f4"
down_revision = "d4b8c9e3f7a1"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "user_profiles",
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), primary_key=True),
        sa.Column("data_json", sa.Text(), nullable=False, server_default="{}"),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
    )
    op.create_index(op.f("ix_user_profiles_created_at"), "user_profiles", ["created_at"], unique=False)
    op.create_index(op.f("ix_user_profiles_updated_at"), "user_profiles", ["updated_at"], unique=False)

    op.create_table(
        "user_consents",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("integration", sa.String(), nullable=False),
        sa.Column("granted_at", sa.DateTime(), nullable=True),
        sa.Column("revoked_at", sa.DateTime(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
    )
    op.create_index(op.f("ix_user_consents_user_id"), "user_consents", ["user_id"], unique=False)
    op.create_index(op.f("ix_user_consents_integration"), "user_consents", ["integration"], unique=False)


def downgrade() -> None:
    op.drop_table("user_consents")
    op.drop_table("user_profiles")
