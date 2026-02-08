"""add wardrobe wear tracking

Revision ID: p8a1b2c3d4e5
Revises: o7a8b9c0d1e2
Create Date: 2026-02-08
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "p8a1b2c3d4e5"
down_revision = "o7a8b9c0d1e2"
branch_labels = None
depends_on = None


def upgrade():
    op.add_column("wardrobe_items", sa.Column("wear_count", sa.Integer(), nullable=False, server_default=sa.text("0")))
    op.add_column("wardrobe_items", sa.Column("last_worn_at", sa.DateTime(), nullable=True))
    op.add_column("wardrobe_items", sa.Column("last_rotation_notified_at", sa.DateTime(), nullable=True))

    op.create_index("ix_wardrobe_items_wear_count", "wardrobe_items", ["wear_count"])
    op.create_index("ix_wardrobe_items_last_worn_at", "wardrobe_items", ["last_worn_at"])
    op.create_index("ix_wardrobe_items_last_rotation_notified_at", "wardrobe_items", ["last_rotation_notified_at"])

    op.create_table(
        "wardrobe_wear_events",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("wardrobe_item_id", sa.Integer(), sa.ForeignKey("wardrobe_items.id"), nullable=False, index=True),
        sa.Column("worn_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("source", sa.String(), nullable=False, server_default="manual", index=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("wardrobe_wear_events")
    op.drop_index("ix_wardrobe_items_last_rotation_notified_at", table_name="wardrobe_items")
    op.drop_index("ix_wardrobe_items_last_worn_at", table_name="wardrobe_items")
    op.drop_index("ix_wardrobe_items_wear_count", table_name="wardrobe_items")

    op.drop_column("wardrobe_items", "last_rotation_notified_at")
    op.drop_column("wardrobe_items", "last_worn_at")
    op.drop_column("wardrobe_items", "wear_count")
