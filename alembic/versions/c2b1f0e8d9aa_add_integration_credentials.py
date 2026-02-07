"""add integration_credentials table

Revision ID: c2b1f0e8d9aa
Revises: ab97ac091deb
Create Date: 2026-02-07 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "c2b1f0e8d9aa"
down_revision = "ab97ac091deb"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "integration_credentials",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("provider", sa.String(), nullable=False),
        sa.Column("username", sa.String(), nullable=True),
        sa.Column("secret_enc", sa.Text(), nullable=True),
        sa.Column("server_url", sa.String(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
        sa.UniqueConstraint("user_id", "provider", name="uq_integration_credentials_user_provider"),
    )
    op.create_index(
        op.f("ix_integration_credentials_user_id"),
        "integration_credentials",
        ["user_id"],
        unique=False,
    )
    op.create_index(
        op.f("ix_integration_credentials_provider"),
        "integration_credentials",
        ["provider"],
        unique=False,
    )
    op.create_index(
        op.f("ix_integration_credentials_created_at"),
        "integration_credentials",
        ["created_at"],
        unique=False,
    )
    op.create_index(
        op.f("ix_integration_credentials_updated_at"),
        "integration_credentials",
        ["updated_at"],
        unique=False,
    )


def downgrade() -> None:
    op.drop_table("integration_credentials")
