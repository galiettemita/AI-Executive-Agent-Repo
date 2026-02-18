"""ops m2: invoices table + legal deletion flag

Revision ID: z0a1b2c3d4e5
Revises: y8z9a0b1c2d3
Create Date: 2026-02-17 00:00:00.000000
"""

from __future__ import annotations

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "z0a1b2c3d4e5"
down_revision = "y8z9a0b1c2d3"
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

    if _table_exists(conn, "users"):
        with op.batch_alter_table("users") as batch:
            batch.add_column(sa.Column("deletion_requested_at", sa.DateTime(), nullable=True))

    if not _table_exists(conn, "invoices"):
        op.create_table(
            "invoices",
            sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
            sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
            sa.Column("provider", sa.String(), nullable=False, server_default="stripe", index=True),
            sa.Column("provider_invoice_id", sa.String(), nullable=False, unique=True, index=True),
            sa.Column("provider_customer_id", sa.String(), nullable=True, index=True),
            sa.Column("provider_subscription_id", sa.String(), nullable=True, index=True),
            sa.Column("status", sa.String(), nullable=False, server_default="open", index=True),
            sa.Column("amount_due", sa.Integer(), nullable=True),
            sa.Column("amount_paid", sa.Integer(), nullable=True),
            sa.Column("currency", sa.String(), nullable=True),
            sa.Column("hosted_invoice_url", sa.String(), nullable=True),
            sa.Column("invoice_pdf_url", sa.String(), nullable=True),
            sa.Column("paid_at", sa.DateTime(), nullable=True, index=True),
            sa.Column("created_at", sa.DateTime(), nullable=True),
            sa.Column("updated_at", sa.DateTime(), nullable=True),
        )


def downgrade() -> None:
    conn = op.get_bind()

    if _table_exists(conn, "invoices"):
        op.drop_table("invoices")

    if _table_exists(conn, "users"):
        with op.batch_alter_table("users") as batch:
            try:
                batch.drop_column("deletion_requested_at")
            except Exception:
                pass

