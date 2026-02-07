"""add contacts, outbound messages, audit logs

Revision ID: h2e9c4f1d7aa
Revises: g1c4b2d9e0aa
Create Date: 2026-02-07
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = "h2e9c4f1d7aa"
down_revision = "g1c4b2d9e0aa"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "audit_logs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=True),
        sa.Column("actor_type", sa.String(), nullable=False, server_default="user", index=True),
        sa.Column("action", sa.String(), nullable=False, index=True),
        sa.Column("resource_type", sa.String(), nullable=True, index=True),
        sa.Column("resource_id", sa.String(), nullable=True, index=True),
        sa.Column("method", sa.String(), nullable=True),
        sa.Column("path", sa.String(), nullable=True, index=True),
        sa.Column("status_code", sa.Integer(), nullable=True),
        sa.Column("ip_address", sa.String(), nullable=True),
        sa.Column("user_agent", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "contacts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("name", sa.String(), nullable=True, index=True),
        sa.Column("phone", sa.String(), nullable=True),
        sa.Column("email", sa.String(), nullable=True),
        sa.Column("normalized_phone", sa.String(), nullable=True, index=True),
        sa.Column("normalized_email", sa.String(), nullable=True, index=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "normalized_phone", name="uq_contacts_user_phone"),
        sa.UniqueConstraint("user_id", "normalized_email", name="uq_contacts_user_email"),
    )

    op.create_table(
        "outbound_messages",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), index=True, nullable=False),
        sa.Column("contact_id", sa.Integer(), sa.ForeignKey("contacts.id"), index=True, nullable=True),
        sa.Column("channel", sa.String(), nullable=False, index=True),
        sa.Column("to_address", sa.String(), nullable=False, index=True),
        sa.Column("body", sa.Text(), nullable=False),
        sa.Column("status", sa.String(), nullable=False, server_default="queued", index=True),
        sa.Column("provider_message_id", sa.String(), nullable=True),
        sa.Column("error_message", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("sent_at", sa.DateTime(), nullable=True, index=True),
    )


def downgrade():
    op.drop_table("outbound_messages")
    op.drop_table("contacts")
    op.drop_table("audit_logs")
