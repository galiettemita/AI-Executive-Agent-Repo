"""add smart home tables

Revision ID: f3c2a8b1e9d0
Revises: e1b7d9c2a0f4
Create Date: 2026-02-07 00:00:00.000000
"""

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = "f3c2a8b1e9d0"
down_revision = "e1b7d9c2a0f4"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "smart_home_devices",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("provider", sa.String(), nullable=False),
        sa.Column("provider_device_id", sa.String(), nullable=False),
        sa.Column("name", sa.String(), nullable=False),
        sa.Column("device_type", sa.String(), nullable=True),
        sa.Column("room", sa.String(), nullable=True),
        sa.Column("traits_json", sa.Text(), nullable=True),
        sa.Column("state_json", sa.Text(), nullable=True),
        sa.Column("online", sa.Boolean(), nullable=False, server_default=sa.true()),
        sa.Column("last_state_at", sa.DateTime(), nullable=True),
        sa.Column("last_seen_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
        sa.UniqueConstraint("user_id", "provider", "provider_device_id", name="uq_smart_home_device_provider"),
    )
    op.create_index(op.f("ix_smart_home_devices_user_id"), "smart_home_devices", ["user_id"], unique=False)
    op.create_index(op.f("ix_smart_home_devices_provider"), "smart_home_devices", ["provider"], unique=False)
    op.create_index(op.f("ix_smart_home_devices_provider_device_id"), "smart_home_devices", ["provider_device_id"], unique=False)
    op.create_index(op.f("ix_smart_home_devices_name"), "smart_home_devices", ["name"], unique=False)
    op.create_index(op.f("ix_smart_home_devices_created_at"), "smart_home_devices", ["created_at"], unique=False)
    op.create_index(op.f("ix_smart_home_devices_updated_at"), "smart_home_devices", ["updated_at"], unique=False)

    op.create_table(
        "smart_home_scenes",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("name", sa.String(), nullable=False),
        sa.Column("description", sa.String(), nullable=True),
        sa.Column("actions_json", sa.Text(), nullable=False, server_default="[]"),
        sa.Column("created_at", sa.DateTime(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), nullable=True),
    )
    op.create_index(op.f("ix_smart_home_scenes_user_id"), "smart_home_scenes", ["user_id"], unique=False)
    op.create_index(op.f("ix_smart_home_scenes_name"), "smart_home_scenes", ["name"], unique=False)
    op.create_index(op.f("ix_smart_home_scenes_created_at"), "smart_home_scenes", ["created_at"], unique=False)
    op.create_index(op.f("ix_smart_home_scenes_updated_at"), "smart_home_scenes", ["updated_at"], unique=False)

    op.create_table(
        "smart_home_action_logs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("device_id", sa.Integer(), sa.ForeignKey("smart_home_devices.id"), nullable=True),
        sa.Column("scene_id", sa.Integer(), sa.ForeignKey("smart_home_scenes.id"), nullable=True),
        sa.Column("action_type", sa.String(), nullable=False),
        sa.Column("status", sa.String(), nullable=False),
        sa.Column("request_json", sa.Text(), nullable=True),
        sa.Column("response_json", sa.Text(), nullable=True),
        sa.Column("error_message", sa.Text(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
    )
    op.create_index(op.f("ix_smart_home_action_logs_user_id"), "smart_home_action_logs", ["user_id"], unique=False)
    op.create_index(op.f("ix_smart_home_action_logs_action_type"), "smart_home_action_logs", ["action_type"], unique=False)
    op.create_index(op.f("ix_smart_home_action_logs_status"), "smart_home_action_logs", ["status"], unique=False)
    op.create_index(op.f("ix_smart_home_action_logs_created_at"), "smart_home_action_logs", ["created_at"], unique=False)

    op.create_table(
        "smart_home_energy_alerts",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("provider", sa.String(), nullable=False),
        sa.Column("entity_id", sa.String(), nullable=False),
        sa.Column("comparison", sa.String(), nullable=False, server_default="gt"),
        sa.Column("threshold_value", sa.Float(), nullable=False, server_default="0"),
        sa.Column("unit", sa.String(), nullable=True),
        sa.Column("last_triggered_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False),
    )
    op.create_index(op.f("ix_smart_home_energy_alerts_user_id"), "smart_home_energy_alerts", ["user_id"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_alerts_provider"), "smart_home_energy_alerts", ["provider"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_alerts_entity_id"), "smart_home_energy_alerts", ["entity_id"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_alerts_created_at"), "smart_home_energy_alerts", ["created_at"], unique=False)

    op.create_table(
        "smart_home_energy_readings",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("user_id", sa.String(), sa.ForeignKey("users.id"), nullable=False),
        sa.Column("provider", sa.String(), nullable=False),
        sa.Column("entity_id", sa.String(), nullable=False),
        sa.Column("reading_time", sa.DateTime(), nullable=False),
        sa.Column("value", sa.Float(), nullable=False, server_default="0"),
        sa.Column("unit", sa.String(), nullable=True),
        sa.Column("metadata_json", sa.Text(), nullable=True),
    )
    op.create_index(op.f("ix_smart_home_energy_readings_user_id"), "smart_home_energy_readings", ["user_id"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_readings_provider"), "smart_home_energy_readings", ["provider"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_readings_entity_id"), "smart_home_energy_readings", ["entity_id"], unique=False)
    op.create_index(op.f("ix_smart_home_energy_readings_reading_time"), "smart_home_energy_readings", ["reading_time"], unique=False)


def downgrade() -> None:
    op.drop_table("smart_home_energy_readings")
    op.drop_table("smart_home_energy_alerts")
    op.drop_table("smart_home_action_logs")
    op.drop_table("smart_home_scenes")
    op.drop_table("smart_home_devices")
