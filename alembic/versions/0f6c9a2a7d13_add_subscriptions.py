"""add subscriptions table

Revision ID: 0f6c9a2a7d13
Revises: 5b8d2d4a1f21
Create Date: 2026-02-03 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "0f6c9a2a7d13"
down_revision = "5b8d2d4a1f21"
branch_labels = None
depends_on = None


def _table_exists(conn, table_name: str) -> bool:
    dialect = conn.dialect.name
    if dialect == "sqlite":
        row = conn.execute(
            sa.text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    row = conn.execute(
        sa.text(
            "select 1 from information_schema.tables "
            "where table_schema = current_schema() and table_name = :name"
        ),
        {"name": table_name},
    ).first()
    return bool(row)


def upgrade() -> None:
    conn = op.get_bind()
    if not _table_exists(conn, "subscriptions"):
        op.create_table(
            "subscriptions",
            sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), primary_key=True),
            sa.Column("plan", sa.String(), nullable=False, server_default="free"),
            sa.Column("status", sa.String(), nullable=False, server_default="active"),
            sa.Column("provider", sa.String(), nullable=True),
            sa.Column("provider_customer_id", sa.String(), nullable=True),
            sa.Column("provider_subscription_id", sa.String(), nullable=True),
            sa.Column("current_period_end", sa.DateTime(), nullable=True),
            sa.Column("updated_at", sa.DateTime(), nullable=True),
        )


def downgrade() -> None:
    conn = op.get_bind()
    if _table_exists(conn, "subscriptions"):
        op.drop_table("subscriptions")
