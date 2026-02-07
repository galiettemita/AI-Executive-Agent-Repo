"""add file_assets and photo_assets

Revision ID: d4b8c9e3f7a1
Revises: c2b1f0e8d9aa
Create Date: 2026-02-07 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "d4b8c9e3f7a1"
down_revision = "c2b1f0e8d9aa"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "file_assets",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("filename", sa.String(), nullable=False),
        sa.Column("content_type", sa.String(), nullable=True),
        sa.Column("size_bytes", sa.Integer(), nullable=True),
        sa.Column("storage_key", sa.String(), nullable=False),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
        sa.UniqueConstraint("storage_key", name="uq_file_assets_storage_key"),
    )
    op.create_index(op.f("ix_file_assets_user_id"), "file_assets", ["user_id"], unique=False)
    op.create_index(op.f("ix_file_assets_filename"), "file_assets", ["filename"], unique=False)
    op.create_index(op.f("ix_file_assets_storage_key"), "file_assets", ["storage_key"], unique=False)
    op.create_index(op.f("ix_file_assets_created_at"), "file_assets", ["created_at"], unique=False)
    op.create_index(op.f("ix_file_assets_updated_at"), "file_assets", ["updated_at"], unique=False)

    op.create_table(
        "photo_assets",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("filename", sa.String(), nullable=False),
        sa.Column("content_type", sa.String(), nullable=True),
        sa.Column("size_bytes", sa.Integer(), nullable=True),
        sa.Column("storage_key", sa.String(), nullable=False),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
        sa.UniqueConstraint("storage_key", name="uq_photo_assets_storage_key"),
    )
    op.create_index(op.f("ix_photo_assets_user_id"), "photo_assets", ["user_id"], unique=False)
    op.create_index(op.f("ix_photo_assets_filename"), "photo_assets", ["filename"], unique=False)
    op.create_index(op.f("ix_photo_assets_storage_key"), "photo_assets", ["storage_key"], unique=False)
    op.create_index(op.f("ix_photo_assets_created_at"), "photo_assets", ["created_at"], unique=False)
    op.create_index(op.f("ix_photo_assets_updated_at"), "photo_assets", ["updated_at"], unique=False)


def downgrade() -> None:
    op.drop_table("photo_assets")
    op.drop_table("file_assets")
