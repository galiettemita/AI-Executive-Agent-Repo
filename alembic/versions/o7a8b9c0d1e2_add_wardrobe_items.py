"""add wardrobe items

Revision ID: o7a8b9c0d1e2
Revises: n6d2f8c4a1b7
Create Date: 2026-02-08
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "o7a8b9c0d1e2"
down_revision = "n6d2f8c4a1b7"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "wardrobe_items",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("name", sa.String(), nullable=False, index=True),
        sa.Column("category", sa.String(), nullable=True, index=True),
        sa.Column("subcategory", sa.String(), nullable=True, index=True),
        sa.Column("brand", sa.String(), nullable=True, index=True),
        sa.Column("color", sa.String(), nullable=True, index=True),
        sa.Column("size", sa.String(), nullable=True),
        sa.Column("material", sa.String(), nullable=True),
        sa.Column("season", sa.String(), nullable=True, index=True),
        sa.Column("condition", sa.String(), nullable=True),
        sa.Column("purchase_date", sa.DateTime(), nullable=True),
        sa.Column("price", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "wardrobe_item_photos",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("wardrobe_item_id", sa.Integer(), sa.ForeignKey("wardrobe_items.id"), nullable=False, index=True),
        sa.Column("photo_asset_id", sa.Integer(), sa.ForeignKey("photo_assets.id"), nullable=False, index=True),
        sa.Column("is_primary", sa.Boolean(), nullable=False, server_default=sa.text("false"), index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("wardrobe_item_id", "photo_asset_id", name="uq_wardrobe_item_photo"),
    )


def downgrade():
    op.drop_table("wardrobe_item_photos")
    op.drop_table("wardrobe_items")
