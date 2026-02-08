"""add gift management

Revision ID: q1b2c3d4e5f6
Revises: p8a1b2c3d4e5
Create Date: 2026-02-08
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "q1b2c3d4e5f6"
down_revision = "p8a1b2c3d4e5"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "gift_occasions",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("recipient_name", sa.String(), nullable=False, index=True),
        sa.Column("relationship", sa.String(), nullable=True, index=True),
        sa.Column("occasion_type", sa.String(), nullable=True, index=True),
        sa.Column("occasion_date", sa.Date(), nullable=True, index=True),
        sa.Column("recurrence", sa.String(), nullable=True, index=True),
        sa.Column("reminder_days_before", sa.Integer(), nullable=False, server_default=sa.text("14")),
        sa.Column("last_reminder_sent_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("budget", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("preferences_json", sa.Text(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "gift_ideas",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("occasion_id", sa.Integer(), sa.ForeignKey("gift_occasions.id"), nullable=True, index=True),
        sa.Column("title", sa.String(), nullable=False, index=True),
        sa.Column("description", sa.Text(), nullable=True),
        sa.Column("link_url", sa.String(), nullable=True),
        sa.Column("price", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="idea", index=True),
        sa.Column("source", sa.String(), nullable=True, index=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "gift_thank_you_drafts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("occasion_id", sa.Integer(), sa.ForeignKey("gift_occasions.id"), nullable=True, index=True),
        sa.Column("gift_idea_id", sa.Integer(), sa.ForeignKey("gift_ideas.id"), nullable=True, index=True),
        sa.Column("message", sa.Text(), nullable=False),
        sa.Column("status", sa.String(), nullable=False, server_default="draft", index=True),
        sa.Column("sent_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("gift_thank_you_drafts")
    op.drop_table("gift_ideas")
    op.drop_table("gift_occasions")
