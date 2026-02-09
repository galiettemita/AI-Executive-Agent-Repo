"""add relationship manager

Revision ID: r2c3d4e5f6a7
Revises: q1b2c3d4e5f6
Create Date: 2026-02-08
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "r2c3d4e5f6a7"
down_revision = "q1b2c3d4e5f6"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "relationship_profiles",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("contact_id", sa.Integer(), sa.ForeignKey("contacts.id"), nullable=False, index=True),
        sa.Column("relationship", sa.String(), nullable=True, index=True),
        sa.Column("priority", sa.Integer(), nullable=True),
        sa.Column("cadence_days", sa.Integer(), nullable=False, server_default=sa.text("30")),
        sa.Column("preferred_channel", sa.String(), nullable=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("last_interaction_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("last_inbound_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("last_outbound_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("next_checkin_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "contact_id", name="uq_relationship_profile_contact"),
    )

    op.create_table(
        "relationship_interactions",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("contact_id", sa.Integer(), sa.ForeignKey("contacts.id"), nullable=False, index=True),
        sa.Column("direction", sa.String(), nullable=False, server_default=sa.text("'outbound'"), index=True),
        sa.Column("channel", sa.String(), nullable=True, index=True),
        sa.Column("summary", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("occurred_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("relationship_interactions")
    op.drop_table("relationship_profiles")
