"""add entertainment tables

Revision ID: u5f6a7b8c9d0
Revises: t4e5f6a7b8c9
Create Date: 2026-02-09
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "u5f6a7b8c9d0"
down_revision = "t4e5f6a7b8c9"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "entertainment_items",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("title", sa.String(), nullable=False, index=True),
        sa.Column("content_type", sa.String(), nullable=False, index=True),
        sa.Column("status", sa.String(), nullable=False, server_default="planned", index=True),
        sa.Column("rating", sa.Float(), nullable=True),
        sa.Column("external_url", sa.String(), nullable=True),
        sa.Column("source", sa.String(), nullable=True, index=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("last_consumed_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "external_url", name="uq_entertainment_user_url"),
    )

    op.create_table(
        "entertainment_consumption",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("item_id", sa.Integer(), sa.ForeignKey("entertainment_items.id"), nullable=False, index=True),
        sa.Column("event_type", sa.String(), nullable=False, server_default="watched", index=True),
        sa.Column("duration_minutes", sa.Integer(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("occurred_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("entertainment_consumption")
    op.drop_table("entertainment_items")
