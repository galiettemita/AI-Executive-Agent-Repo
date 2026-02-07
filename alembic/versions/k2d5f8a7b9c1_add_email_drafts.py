"""add email drafts

Revision ID: k2d5f8a7b9c1
Revises: j4c9e1f2a7b3
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "k2d5f8a7b9c1"
down_revision = "j4c9e1f2a7b3"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "email_drafts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("provider", sa.String(), nullable=True, index=True),
        sa.Column("to_email", sa.String(), nullable=False, index=True),
        sa.Column("cc", sa.String(), nullable=True),
        sa.Column("bcc", sa.String(), nullable=True),
        sa.Column("subject", sa.String(), nullable=False),
        sa.Column("body_text", sa.Text(), nullable=False),
        sa.Column("source_message_id", sa.String(), nullable=True, index=True),
        sa.Column("provider_draft_id", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="pending", index=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("sent_at", sa.DateTime(), nullable=True, index=True),
    )


def downgrade():
    op.drop_table("email_drafts")
