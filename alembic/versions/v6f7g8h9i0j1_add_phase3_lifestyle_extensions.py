"""add phase 3 lifestyle extensions

Revision ID: v6f7g8h9i0j1
Revises: u5f6a7b8c9d0
Create Date: 2026-02-11
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "v6f7g8h9i0j1"
down_revision = "u5f6a7b8c9d0"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "gift_retailer_allowlist",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=True, index=True),
        sa.Column("domain", sa.String(), nullable=False, index=True),
        sa.Column("status", sa.String(), nullable=False, server_default="allowed", index=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "domain", name="uq_gift_retailer_user_domain"),
    )

    op.create_table(
        "gift_orders",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("gift_idea_id", sa.Integer(), sa.ForeignKey("gift_ideas.id"), nullable=True, index=True),
        sa.Column("occasion_id", sa.Integer(), sa.ForeignKey("gift_occasions.id"), nullable=True, index=True),
        sa.Column("proposal_id", sa.Integer(), sa.ForeignKey("proposals.id"), nullable=True, index=True),
        sa.Column("transaction_id", sa.Integer(), sa.ForeignKey("transactions.id"), nullable=True, index=True),
        sa.Column("payment_method_id", sa.Integer(), sa.ForeignKey("payment_methods.id"), nullable=True),
        sa.Column("retailer_domain", sa.String(), nullable=True, index=True),
        sa.Column("product_url", sa.String(), nullable=True),
        sa.Column("product_title", sa.String(), nullable=True),
        sa.Column("quantity", sa.Integer(), nullable=False, server_default="1"),
        sa.Column("unit_price", sa.Float(), nullable=True),
        sa.Column("total_price", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="pending_approval", index=True),
        sa.Column("shipping_address_json", sa.Text(), nullable=True),
        sa.Column("tracking_number", sa.String(), nullable=True, index=True),
        sa.Column("tracking_url", sa.String(), nullable=True),
        sa.Column("return_window_end", sa.DateTime(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "gift_order_events",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("gift_order_id", sa.Integer(), sa.ForeignKey("gift_orders.id"), nullable=False, index=True),
        sa.Column("status", sa.String(), nullable=False, index=True),
        sa.Column("message", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("occurred_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "entertainment_events",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("title", sa.String(), nullable=False, index=True),
        sa.Column("event_type", sa.String(), nullable=True, index=True),
        sa.Column("venue", sa.String(), nullable=True),
        sa.Column("location", sa.String(), nullable=True, index=True),
        sa.Column("starts_at", sa.DateTime(), nullable=True, index=True),
        sa.Column("ends_at", sa.DateTime(), nullable=True),
        sa.Column("external_url", sa.String(), nullable=True),
        sa.Column("provider", sa.String(), nullable=True, index=True),
        sa.Column("provider_event_id", sa.String(), nullable=True, index=True),
        sa.Column("price_min", sa.Float(), nullable=True),
        sa.Column("price_max", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="interested", index=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "external_url", name="uq_entertainment_event_user_url"),
    )

    op.create_table(
        "entertainment_event_bookings",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("event_id", sa.Integer(), sa.ForeignKey("entertainment_events.id"), nullable=True, index=True),
        sa.Column("proposal_id", sa.Integer(), sa.ForeignKey("proposals.id"), nullable=True, index=True),
        sa.Column("transaction_id", sa.Integer(), sa.ForeignKey("transactions.id"), nullable=True, index=True),
        sa.Column("quantity", sa.Integer(), nullable=False, server_default="1"),
        sa.Column("total_price", sa.Float(), nullable=True),
        sa.Column("currency", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="pending_approval", index=True),
        sa.Column("ticket_delivery", sa.String(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "language_goals",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("language", sa.String(), nullable=False, index=True),
        sa.Column("daily_minutes", sa.Integer(), nullable=False, server_default="15"),
        sa.Column("weekly_sessions", sa.Integer(), nullable=False, server_default="3"),
        sa.Column("target_level", sa.String(), nullable=True),
        sa.Column("active", sa.Boolean(), nullable=False, server_default=sa.text("true")),
        sa.Column("start_date", sa.Date(), nullable=True),
        sa.Column("end_date", sa.Date(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "language", name="uq_language_goal_user_language"),
    )

    op.create_table(
        "language_practice_sessions",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("language", sa.String(), nullable=False, index=True),
        sa.Column("session_type", sa.String(), nullable=True, index=True),
        sa.Column("duration_minutes", sa.Integer(), nullable=True),
        sa.Column("accuracy_score", sa.Float(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("occurred_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )

    op.create_table(
        "learning_resources",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("topic", sa.String(), nullable=True, index=True),
        sa.Column("title", sa.String(), nullable=False, index=True),
        sa.Column("url", sa.String(), nullable=True),
        sa.Column("source", sa.String(), nullable=True, index=True),
        sa.Column("resource_type", sa.String(), nullable=True, index=True),
        sa.Column("difficulty", sa.String(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="planned", index=True),
        sa.Column("tags_json", sa.Text(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.UniqueConstraint("user_id", "url", name="uq_learning_resource_user_url"),
    )

    op.create_table(
        "learning_schedules",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False, index=True),
        sa.Column("resource_id", sa.Integer(), sa.ForeignKey("learning_resources.id"), nullable=True, index=True),
        sa.Column("scheduled_for", sa.DateTime(), nullable=True, index=True),
        sa.Column("duration_minutes", sa.Integer(), nullable=True),
        sa.Column("status", sa.String(), nullable=False, server_default="scheduled", index=True),
        sa.Column("notes", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.text("CURRENT_TIMESTAMP"), index=True),
    )


def downgrade():
    op.drop_table("learning_schedules")
    op.drop_table("learning_resources")
    op.drop_table("language_practice_sessions")
    op.drop_table("language_goals")
    op.drop_table("entertainment_event_bookings")
    op.drop_table("entertainment_events")
    op.drop_table("gift_order_events")
    op.drop_table("gift_orders")
    op.drop_table("gift_retailer_allowlist")
